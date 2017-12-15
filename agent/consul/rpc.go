package consul

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/agent/pool"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/yamux"
)

const (
	// maxQueryTime is used to bound the limit of a blocking query
	maxQueryTime = 600 * time.Second

	// defaultQueryTime is the amount of time we block waiting for a change
	// if no time is specified. Previously we would wait the maxQueryTime.
	defaultQueryTime = 300 * time.Second

	// jitterFraction is a the limit to the amount of jitter we apply
	// to a user specified MaxQueryTime. We divide the specified time by
	// the fraction. So 16 == 6.25% limit of jitter. This same fraction
	// is applied to the RPCHoldTimeout
	jitterFraction = 16

	// Warn if the Raft command is larger than this.
	// If it's over 1MB something is probably being abusive.
	raftWarnSize = 1024 * 1024

	// enqueueLimit caps how long we will wait to enqueue
	// a new Raft command. Something is probably wrong if this
	// value is ever reached. However, it prevents us from blocking
	// the requesting goroutine forever.
	enqueueLimit = 30 * time.Second
)

// listen is used to listen for incoming RPC connections
func (s *Server) listen(listener net.Listener) {
	for {
		// Accept a connection
		conn, err := listener.Accept()
		if err != nil {
			if s.shutdown {
				return
			}
			s.logger.Printf("[ERR] consul.rpc: failed to accept RPC conn: %v", err)
			continue
		}

		go s.handleConn(conn, false)
		metrics.IncrCounter([]string{"consul", "rpc", "accept_conn"}, 1)
		metrics.IncrCounter([]string{"rpc", "accept_conn"}, 1)
	}
}

// logConn is a wrapper around memberlist's LogConn so that we format references
// to "from" addresses in a consistent way. This is just a shorter name.
func logConn(conn net.Conn) string {
	return memberlist.LogConn(conn)
}

// handleConn is used to determine if this is a Raft or
// Consul type RPC connection and invoke the correct handler
func (s *Server) handleConn(conn net.Conn, isTLS bool) {
	// Read a single byte
	buf := make([]byte, 1)
	if _, err := conn.Read(buf); err != nil {
		if err != io.EOF {
			s.logger.Printf("[ERR] consul.rpc: failed to read byte: %v %s", err, logConn(conn))
		}
		conn.Close()
		return
	}
	typ := pool.RPCType(buf[0])

	// Enforce TLS if VerifyIncoming is set
	if s.config.VerifyIncoming && !isTLS && typ != pool.RPCTLS {
		s.logger.Printf("[WARN] consul.rpc: Non-TLS connection attempted with VerifyIncoming set %s", logConn(conn))
		conn.Close()
		return
	}

	// Switch on the byte
	switch typ {
	case pool.RPCConsul:
		s.handleConsulConn(conn)

	case pool.RPCRaft:
		metrics.IncrCounter([]string{"consul", "rpc", "raft_handoff"}, 1)
		metrics.IncrCounter([]string{"rpc", "raft_handoff"}, 1)
		s.raftLayer.Handoff(conn)

	case pool.RPCTLS:
		if s.rpcTLS == nil {
			s.logger.Printf("[WARN] consul.rpc: TLS connection attempted, server not configured for TLS %s", logConn(conn))
			conn.Close()
			return
		}
		conn = tls.Server(conn, s.rpcTLS)
		s.handleConn(conn, true)

	case pool.RPCMultiplexV2:
		s.handleMultiplexV2(conn)

	case pool.RPCSnapshot:
		s.handleSnapshotConn(conn)

	default:
		s.logger.Printf("[ERR] consul.rpc: unrecognized RPC byte: %v %s", typ, logConn(conn))
		conn.Close()
		return
	}
}

// handleMultiplexV2 is used to multiplex a single incoming connection
// using the Yamux multiplexer
func (s *Server) handleMultiplexV2(conn net.Conn) {
	defer conn.Close()
	conf := yamux.DefaultConfig()
	conf.LogOutput = s.config.LogOutput
	server, _ := yamux.Server(conn, conf)
	for {
		sub, err := server.Accept()
		if err != nil {
			if err != io.EOF {
				s.logger.Printf("[ERR] consul.rpc: multiplex conn accept failed: %v %s", err, logConn(conn))
			}
			return
		}
		go s.handleConsulConn(sub)
	}
}

// handleConsulConn is used to service a single Consul RPC connection
func (s *Server) handleConsulConn(conn net.Conn) {
	defer conn.Close()
	rpcCodec := msgpackrpc.NewServerCodec(conn)
	for {
		select {
		case <-s.shutdownCh:
			return
		default:
		}

		if err := s.rpcServer.ServeRequest(rpcCodec); err != nil {
			if err != io.EOF && !strings.Contains(err.Error(), "closed") {
				s.logger.Printf("[ERR] consul.rpc: RPC error: %v %s", err, logConn(conn))
				metrics.IncrCounter([]string{"consul", "rpc", "request_error"}, 1)
				metrics.IncrCounter([]string{"rpc", "request_error"}, 1)
			}
			return
		}
		metrics.IncrCounter([]string{"consul", "rpc", "request"}, 1)
		metrics.IncrCounter([]string{"rpc", "request"}, 1)
	}
}

// handleSnapshotConn is used to dispatch snapshot saves and restores, which
// stream so don't use the normal RPC mechanism.
func (s *Server) handleSnapshotConn(conn net.Conn) {
	go func() {
		defer conn.Close()
		if err := s.handleSnapshotRequest(conn); err != nil {
			s.logger.Printf("[ERR] consul.rpc: Snapshot RPC error: %v %s", err, logConn(conn))
		}
	}()
}

// canRetry returns true if the given situation is safe for a retry.
func canRetry(args interface{}, err error) bool {
	// No leader errors are always safe to retry since no state could have
	// been changed.
	if structs.IsErrNoLeader(err) {
		return true
	}

	// Reads are safe to retry for stream errors, such as if a server was
	// being shut down.
	info, ok := args.(structs.RPCInfo)
	if ok && info.IsRead() && lib.IsErrEOF(err) {
		return true
	}

	return false
}

// forward is used to forward to a remote DC or to forward to the local leader
// Returns a bool of if forwarding was performed, as well as any error
func (s *Server) forward(method string, info structs.RPCInfo, args interface{}, reply interface{}) (bool, error) {
	var firstCheck time.Time

	// Handle DC forwarding
	dc := info.RequestDatacenter()
	if dc != s.config.Datacenter {
		err := s.forwardDC(method, dc, args, reply)
		return true, err
	}

	// Check if we can allow a stale read
	if info.IsRead() && info.AllowStaleRead() {
		return false, nil
	}

CHECK_LEADER:
	// Fail fast if we are in the process of leaving
	select {
	case <-s.leaveCh:
		return true, structs.ErrNoLeader
	default:
	}

	// Find the leader
	isLeader, leader := s.getLeader()

	// Handle the case we are the leader
	if isLeader {
		return false, nil
	}

	// Handle the case of a known leader
	rpcErr := structs.ErrNoLeader
	if leader != nil {
		rpcErr = s.connPool.RPC(s.config.Datacenter, leader.Addr,
			leader.Version, method, leader.UseTLS, args, reply)
		if rpcErr != nil && canRetry(info, rpcErr) {
			goto RETRY
		}
		return true, rpcErr
	}

RETRY:
	// Gate the request until there is a leader
	if firstCheck.IsZero() {
		firstCheck = time.Now()
	}
	if time.Since(firstCheck) < s.config.RPCHoldTimeout {
		jitter := lib.RandomStagger(s.config.RPCHoldTimeout / jitterFraction)
		select {
		case <-time.After(jitter):
			goto CHECK_LEADER
		case <-s.leaveCh:
		case <-s.shutdownCh:
		}
	}

	// No leader found and hold time exceeded
	return true, rpcErr
}

// getLeader returns if the current node is the leader, and if not then it
// returns the leader which is potentially nil if the cluster has not yet
// elected a leader.
func (s *Server) getLeader() (bool, *metadata.Server) {
	// Check if we are the leader
	if s.IsLeader() {
		return true, nil
	}

	// Get the leader
	leader := s.raft.Leader()
	if leader == "" {
		return false, nil
	}

	// Lookup the server
	server := s.serverLookup.Server(leader)

	// Server could be nil
	return false, server
}

// forwardDC is used to forward an RPC call to a remote DC, or fail if no servers
func (s *Server) forwardDC(method, dc string, args interface{}, reply interface{}) error {
	manager, server, ok := s.router.FindRoute(dc)
	if !ok {
		s.logger.Printf("[WARN] consul.rpc: RPC request for DC %q, no path found", dc)
		return structs.ErrNoDCPath
	}

	metrics.IncrCounterWithLabels([]string{"consul", "rpc", "cross-dc"}, 1,
		[]metrics.Label{{Name: "datacenter", Value: dc}})
	metrics.IncrCounterWithLabels([]string{"rpc", "cross-dc"}, 1,
		[]metrics.Label{{Name: "datacenter", Value: dc}})
	if err := s.connPool.RPC(dc, server.Addr, server.Version, method, server.UseTLS, args, reply); err != nil {
		manager.NotifyFailedServer(server)
		s.logger.Printf("[ERR] consul: RPC failed to server %s in DC %q: %v", server.Addr, dc, err)
		return err
	}

	return nil
}

// globalRPC is used to forward an RPC request to one server in each datacenter.
// This will only error for RPC-related errors. Otherwise, application-level
// errors can be sent in the response objects.
func (s *Server) globalRPC(method string, args interface{},
	reply structs.CompoundResponse) error {

	// Make a new request into each datacenter
	dcs := s.router.GetDatacenters()

	replies, total := 0, len(dcs)
	errorCh := make(chan error, total)
	respCh := make(chan interface{}, total)

	for _, dc := range dcs {
		go func(dc string) {
			rr := reply.New()
			if err := s.forwardDC(method, dc, args, &rr); err != nil {
				errorCh <- err
				return
			}
			respCh <- rr
		}(dc)
	}

	for replies < total {
		select {
		case err := <-errorCh:
			return err
		case rr := <-respCh:
			reply.Add(rr)
			replies++
		}
	}
	return nil
}

// raftApply is used to encode a message, run it through raft, and return
// the FSM response along with any errors
func (s *Server) raftApply(t structs.MessageType, msg interface{}) (interface{}, error) {
	buf, err := structs.Encode(t, msg)
	if err != nil {
		return nil, fmt.Errorf("Failed to encode request: %v", err)
	}

	// Warn if the command is very large
	if n := len(buf); n > raftWarnSize {
		s.logger.Printf("[WARN] consul: Attempting to apply large raft entry (%d bytes)", n)
	}

	future := s.raft.Apply(buf, enqueueLimit)
	if err := future.Error(); err != nil {
		return nil, err
	}

	return future.Response(), nil
}

// queryFn is used to perform a query operation. If a re-query is needed, the
// passed-in watch set will be used to block for changes. The passed-in state
// store should be used (vs. calling fsm.State()) since the given state store
// will be correctly watched for changes if the state store is restored from
// a snapshot.
type queryFn func(memdb.WatchSet, *state.Store) error

// blockingQuery is used to process a potentially blocking query operation.
func (s *Server) blockingQuery(queryOpts *structs.QueryOptions, queryMeta *structs.QueryMeta,
	fn queryFn) error {
	var timeout *time.Timer

	// Fast path right to the non-blocking query.
	if queryOpts.MinQueryIndex == 0 {
		goto RUN_QUERY
	}

	// Restrict the max query time, and ensure there is always one.
	if queryOpts.MaxQueryTime > maxQueryTime {
		queryOpts.MaxQueryTime = maxQueryTime
	} else if queryOpts.MaxQueryTime <= 0 {
		queryOpts.MaxQueryTime = defaultQueryTime
	}

	// Apply a small amount of jitter to the request.
	queryOpts.MaxQueryTime += lib.RandomStagger(queryOpts.MaxQueryTime / jitterFraction)

	// Setup a query timeout.
	timeout = time.NewTimer(queryOpts.MaxQueryTime)
	defer timeout.Stop()

RUN_QUERY:
	// Update the query metadata.
	s.setQueryMeta(queryMeta)

	// If the read must be consistent we verify that we are still the leader.
	if queryOpts.RequireConsistent {
		if err := s.consistentRead(); err != nil {
			return err
		}
	}

	// Run the query.
	metrics.IncrCounter([]string{"consul", "rpc", "query"}, 1)
	metrics.IncrCounter([]string{"rpc", "query"}, 1)

	// Operate on a consistent set of state. This makes sure that the
	// abandon channel goes with the state that the caller is using to
	// build watches.
	state := s.fsm.State()

	// We can skip all watch tracking if this isn't a blocking query.
	var ws memdb.WatchSet
	if queryOpts.MinQueryIndex > 0 {
		ws = memdb.NewWatchSet()

		// This channel will be closed if a snapshot is restored and the
		// whole state store is abandoned.
		ws.Add(state.AbandonCh())
	}

	// Block up to the timeout if we didn't see anything fresh.
	err := fn(ws, state)
	if err == nil && queryMeta.Index > 0 && queryMeta.Index <= queryOpts.MinQueryIndex {
		if expired := ws.Watch(timeout.C); !expired {
			// If a restore may have woken us up then bail out from
			// the query immediately. This is slightly race-ey since
			// this might have been interrupted for other reasons,
			// but it's OK to kick it back to the caller in either
			// case.
			select {
			case <-state.AbandonCh():
			default:
				goto RUN_QUERY
			}
		}
	}
	return err
}

// setQueryMeta is used to populate the QueryMeta data for an RPC call
func (s *Server) setQueryMeta(m *structs.QueryMeta) {
	if s.IsLeader() {
		m.LastContact = 0
		m.KnownLeader = true
	} else {
		m.LastContact = time.Since(s.raft.LastContact())
		m.KnownLeader = (s.raft.Leader() != "")
	}
}

// consistentRead is used to ensure we do not perform a stale
// read. This is done by verifying leadership before the read.
func (s *Server) consistentRead() error {
	defer metrics.MeasureSince([]string{"consul", "rpc", "consistentRead"}, time.Now())
	defer metrics.MeasureSince([]string{"rpc", "consistentRead"}, time.Now())
	future := s.raft.VerifyLeader()
	if err := future.Error(); err != nil {
		return err //fail fast if leader verification fails
	}
	// poll consistent read readiness, wait for up to RPCHoldTimeout milliseconds
	if s.isReadyForConsistentReads() {
		return nil
	}
	jitter := lib.RandomStagger(s.config.RPCHoldTimeout / jitterFraction)
	deadline := time.Now().Add(s.config.RPCHoldTimeout)

	for time.Now().Before(deadline) {

		select {
		case <-time.After(jitter):
			// Drop through and check before we loop again.

		case <-s.shutdownCh:
			return fmt.Errorf("shutdown waiting for leader")
		}

		if s.isReadyForConsistentReads() {
			return nil
		}
	}

	return structs.ErrNotReadyForConsistentReads
}
