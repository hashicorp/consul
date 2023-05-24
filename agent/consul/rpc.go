package consul

import (
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"strings"
	"time"

	"github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"
	"github.com/hashicorp/go-connlimit"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-raftchunking"
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/yamux"
	"google.golang.org/grpc"

	msgpackrpc "github.com/hashicorp/consul-net-rpc/net-rpc-msgpackrpc"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/blockingquery"
	"github.com/hashicorp/consul/agent/consul/rate"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/wanfed"
	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/agent/pool"
	"github.com/hashicorp/consul/agent/rpc/middleware"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/logging"
)

var RPCCounters = []prometheus.CounterDefinition{
	{
		Name: []string{"rpc", "accept_conn"},
		Help: "Increments when a server accepts an RPC connection.",
	},
	{
		Name: []string{"rpc", "raft_handoff"},
		Help: "Increments when a server accepts a Raft-related RPC connection.",
	},
	{
		Name: []string{"rpc", "request_error"},
		Help: "Increments when a server returns an error from an RPC request.",
	},
	{
		Name: []string{"rpc", "request"},
		Help: "Increments when a server receives a Consul-related RPC request.",
	},
	{
		Name: []string{"rpc", "cross-dc"},
		Help: "Increments when a server sends a (potentially blocking) cross datacenter RPC query.",
	},
	{
		Name: []string{"rpc", "query"},
		Help: "Increments when a server receives a read request, indicating the rate of new read queries.",
	},
}

var RPCGauges = []prometheus.GaugeDefinition{
	{
		Name: []string{"rpc", "queries_blocking"},
		Help: "Shows the current number of in-flight blocking queries the server is handling.",
	},
}

var RPCSummaries = []prometheus.SummaryDefinition{
	{
		Name: []string{"rpc", "consistentRead"},
		Help: "Measures the time spent confirming that a consistent read can be performed.",
	},
}

const (
	// Warn if the Raft command is larger than this.
	// If it's over 1MB something is probably being abusive.
	raftWarnSize = 1024 * 1024

	// enqueueLimit caps how long we will wait to enqueue
	// a new Raft command. Something is probably wrong if this
	// value is ever reached. However, it prevents us from blocking
	// the requesting goroutine forever.
	enqueueLimit = 30 * time.Second
)

var ErrChunkingResubmit = errors.New("please resubmit call for rechunking")

// partitionUnsetter is used to describe requests values that can unset their
// EnterpriseMeta.Partition value.
type partitionUnsetter interface {
	// UnsetPartition is used to strip a Partition value from the request before
	// it is forwarded to a remote datacenter. By unsetting the value, the server
	// that handles the request can decide which partition should be used (or do nothing).
	// This ensures that servers that are Partition-enabled (pre-1.11, or non-Enterprise)
	// don't inadvertently cause servers that are not Partition-enabled (<= 1.10 or non-Enterprise)
	// to filter their responses by Partition. In other words, this ensures upgraded servers
	// remain compatible with non-upgraded servers.
	UnsetPartition()
}

func (s *Server) rpcLogger() hclog.Logger {
	return s.loggers.Named(logging.RPC)
}

// listen is used to listen for incoming RPC connections
func (s *Server) listen(listener net.Listener) {
	for {
		// Accept a connection
		conn, err := listener.Accept()
		if err != nil {
			if s.shutdown {
				return
			}
			s.rpcLogger().Error("failed to accept RPC conn", "error", err)
			continue
		}

		free, err := s.rpcConnLimiter.Accept(conn)
		if err != nil {
			s.rpcLogger().Error("rejecting RPC conn from because rpc_max_conns_per_client exceeded", "conn", logConn(conn))
			conn.Close()
			continue
		}
		// Wrap conn so it will be auto-freed from conn limiter when it closes.
		conn = connlimit.Wrap(conn, free)

		go s.handleConn(conn, false)
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
	// Limit how long the client can hold the connection open before they send the
	// magic byte (and authenticate when mTLS is enabled). If `isTLS == true` then
	// this also enforces a timeout on how long it takes for the handshake to
	// complete since tls.Conn.Read implicitly calls Handshake().
	if s.config.RPCHandshakeTimeout > 0 {
		conn.SetReadDeadline(time.Now().Add(s.config.RPCHandshakeTimeout))
	}

	if !isTLS && s.tlsConfigurator.MutualTLSCapable() {
		// See if actually this is native TLS multiplexed onto the old
		// "type-byte" system.

		peekedConn, nativeTLS, err := pool.PeekForTLS(conn)
		if err != nil {
			if err != io.EOF {
				s.rpcLogger().Error(
					"failed to read first byte",
					"conn", logConn(conn),
					"error", err,
				)
			}
			conn.Close()
			return
		}

		if nativeTLS {
			s.handleNativeTLS(peekedConn)
			return
		}
		conn = peekedConn
	}

	// Read a single byte
	buf := make([]byte, 1)

	if _, err := conn.Read(buf); err != nil {
		if err != io.EOF {
			s.rpcLogger().Error("failed to read byte",
				"conn", logConn(conn),
				"error", err,
			)
		}
		conn.Close()
		return
	}
	typ := pool.RPCType(buf[0])

	// Reset the deadline as we aren't sure what is expected next - it depends on
	// the protocol.
	if s.config.RPCHandshakeTimeout > 0 {
		conn.SetReadDeadline(time.Time{})
	}

	// Enforce TLS if VerifyIncoming is set
	if s.tlsConfigurator.VerifyIncomingRPC() && !isTLS && typ != pool.RPCTLS && typ != pool.RPCTLSInsecure {
		s.rpcLogger().Warn("Non-TLS connection attempted with VerifyIncoming set", "conn", logConn(conn))
		conn.Close()
		return
	}

	// Switch on the byte
	switch typ {
	case pool.RPCConsul:
		s.handleConsulConn(conn)

	case pool.RPCRaft:
		s.handleRaftRPC(conn)

	case pool.RPCTLS:
		// Don't allow malicious client to create TLS-in-TLS for ever.
		if isTLS {
			s.rpcLogger().Error("TLS connection attempting to establish inner TLS connection", "conn", logConn(conn))
			conn.Close()
			return
		}
		conn = tls.Server(conn, s.tlsConfigurator.IncomingRPCConfig())
		s.handleConn(conn, true)

	case pool.RPCMultiplexV2:
		s.handleMultiplexV2(conn)

	case pool.RPCSnapshot:
		s.handleSnapshotConn(conn)

	case pool.RPCTLSInsecure:
		// Don't allow malicious client to create TLS-in-TLS for ever.
		if isTLS {
			s.rpcLogger().Error("TLS connection attempting to establish inner TLS connection", "conn", logConn(conn))
			conn.Close()
			return
		}
		conn = tls.Server(conn, s.tlsConfigurator.IncomingInsecureRPCConfig())
		s.handleInsecureConn(conn)

	case pool.RPCGRPC:
		s.grpcHandler.Handle(conn)

	default:
		if !s.handleEnterpriseRPCConn(typ, conn, isTLS) {
			s.rpcLogger().Error("unrecognized RPC byte",
				"byte", typ,
				"conn", logConn(conn),
			)
			conn.Close()
		}
	}
}

func (s *Server) handleNativeTLS(conn net.Conn) {
	s.rpcLogger().Trace(
		"detected actual TLS over RPC port",
		"conn", logConn(conn),
	)

	tlscfg := s.tlsConfigurator.IncomingALPNRPCConfig(pool.RPCNextProtos)
	tlsConn := tls.Server(conn, tlscfg)

	// Force the handshake to conclude.
	if err := tlsConn.Handshake(); err != nil {
		s.rpcLogger().Error(
			"TLS handshake failed",
			"conn", logConn(conn),
			"error", err,
		)
		conn.Close()
		return
	}

	// Reset the deadline as we aren't sure what is expected next - it depends on
	// the protocol.
	if s.config.RPCHandshakeTimeout > 0 {
		conn.SetReadDeadline(time.Time{})
	}

	var (
		cs        = tlsConn.ConnectionState()
		sni       = cs.ServerName
		nextProto = cs.NegotiatedProtocol

		transport = s.memberlistTransportWAN
	)

	s.rpcLogger().Trace(
		"accepted nativeTLS RPC",
		"sni", sni,
		"protocol", nextProto,
		"conn", logConn(conn),
	)

	switch nextProto {
	case pool.ALPN_RPCConsul:
		s.handleConsulConn(tlsConn)

	case pool.ALPN_RPCRaft:
		s.handleRaftRPC(tlsConn)

	case pool.ALPN_RPCMultiplexV2:
		s.handleMultiplexV2(tlsConn)

	case pool.ALPN_RPCSnapshot:
		s.handleSnapshotConn(tlsConn)

	case pool.ALPN_RPCGRPC:
		s.grpcHandler.Handle(tlsConn)

	case pool.ALPN_WANGossipPacket:
		if err := s.handleALPN_WANGossipPacketStream(tlsConn); err != nil && err != io.EOF {
			s.rpcLogger().Error(
				"failed to ingest RPC",
				"sni", sni,
				"protocol", nextProto,
				"conn", logConn(conn),
				"error", err,
			)
		}

	case pool.ALPN_WANGossipStream:
		// No need to defer the conn.Close() here, the Ingest methods do that.
		if err := transport.IngestStream(tlsConn); err != nil {
			s.rpcLogger().Error(
				"failed to ingest RPC",
				"sni", sni,
				"protocol", nextProto,
				"conn", logConn(conn),
				"error", err,
			)
		}

	default:
		if !s.handleEnterpriseNativeTLSConn(nextProto, conn) {
			s.rpcLogger().Error(
				"discarding RPC for unknown negotiated protocol",
				"failed to ingest RPC",
				"protocol", nextProto,
				"conn", logConn(conn),
			)
			conn.Close()
		}
	}
}

// handleMultiplexV2 is used to multiplex a single incoming connection
// using the Yamux multiplexer
func (s *Server) handleMultiplexV2(conn net.Conn) {
	defer conn.Close()
	conf := yamux.DefaultConfig()
	// override the default because LogOutput conflicts with Logger
	conf.LogOutput = nil
	// TODO: should this be created once and cached?
	conf.Logger = s.logger.StandardLogger(&hclog.StandardLoggerOptions{InferLevels: true})
	server, _ := yamux.Server(conn, conf)
	for {
		sub, err := server.Accept()
		if err != nil {
			if err != io.EOF {
				s.rpcLogger().Error("multiplex conn accept failed",
					"conn", logConn(conn),
					"error", err,
				)
			}
			return
		}

		// In the beginning only RPC was supposed to be multiplexed
		// with yamux. In order to add the ability to multiplex network
		// area connections, this workaround was added.
		// This code peeks the first byte and checks if it is
		// RPCGossip, in which case this is handled by enterprise code.
		// Otherwise this connection is handled like before by the RPC
		// handler.
		// This wouldn't work if a normal RPC could start with
		// RPCGossip(6). In messagepack a 6 encodes a positive fixint:
		// https://github.com/msgpack/msgpack/blob/master/spec.md.
		// None of the RPCs we are doing starts with that, usually it is
		// a string for datacenter.
		peeked, first, err := pool.PeekFirstByte(sub)
		if err != nil {
			s.rpcLogger().Error("Problem peeking connection", "conn", logConn(sub), "err", err)
			sub.Close()
			return
		}
		sub = peeked
		switch first {
		case byte(pool.RPCGossip):
			buf := make([]byte, 1)
			sub.Read(buf)
			go func() {
				if !s.handleEnterpriseRPCConn(pool.RPCGossip, sub, false) {
					s.rpcLogger().Error("unrecognized RPC byte",
						"byte", pool.RPCGossip,
						"conn", logConn(conn),
					)
					sub.Close()
				}
			}()
		default:
			go s.handleConsulConn(sub)
		}
	}
}

// handleConsulConn is used to service a single Consul RPC connection
func (s *Server) handleConsulConn(conn net.Conn) {
	defer conn.Close()
	rpcCodec := msgpackrpc.NewCodecFromHandle(true, true, conn, structs.MsgpackHandle)
	for {
		select {
		case <-s.shutdownCh:
			return
		default:
		}

		if err := s.rpcServer.ServeRequest(rpcCodec); err != nil {
			//EOF or closed are not considered as errors.
			if err == io.EOF || strings.Contains(err.Error(), "closed") {
				return
			}

			metrics.IncrCounter([]string{"rpc", "request_error"}, 1)
			// When a rate-limiting error is returned, it's already logged, so skip logging.
			if errors.Is(err, rate.ErrRetryLater) || errors.Is(err, rate.ErrRetryElsewhere) {
				return
			}

			s.rpcLogger().Error("RPC error",
				"conn", logConn(conn),
				"error", err,
			)
			return
		}
		metrics.IncrCounter([]string{"rpc", "request"}, 1)
	}
}

// handleInsecureConsulConn is used to service a single Consul INSECURERPC connection
func (s *Server) handleInsecureConn(conn net.Conn) {
	defer conn.Close()
	rpcCodec := msgpackrpc.NewCodecFromHandle(true, true, conn, structs.MsgpackHandle)
	for {
		select {
		case <-s.shutdownCh:
			return
		default:
		}

		if err := s.insecureRPCServer.ServeRequest(rpcCodec); err != nil {
			if err != io.EOF && !strings.Contains(err.Error(), "closed") {
				s.rpcLogger().Error("INSECURERPC error",
					"conn", logConn(conn),
					"error", err,
				)
				metrics.IncrCounter([]string{"rpc", "request_error"}, 1)
			}
			return
		}
		metrics.IncrCounter([]string{"rpc", "request"}, 1)
	}
}

// handleSnapshotConn is used to dispatch snapshot saves and restores, which
// stream so don't use the normal RPC mechanism.
func (s *Server) handleSnapshotConn(conn net.Conn) {
	go func() {
		defer conn.Close()
		if err := s.handleSnapshotRequest(conn); err != nil {
			s.rpcLogger().Error("Snapshot RPC error",
				"conn", logConn(conn),
				"error", err,
			)
		}
	}()
}

func (s *Server) handleRaftRPC(conn net.Conn) {
	if tlsConn, ok := conn.(*tls.Conn); ok {
		err := s.tlsConfigurator.AuthorizeServerConn(s.config.Datacenter, tlsConn)
		if err != nil {
			s.rpcLogger().Warn(err.Error(), "from", conn.RemoteAddr(), "operation", "raft RPC")
			conn.Close()
			return
		}
	}

	metrics.IncrCounter([]string{"rpc", "raft_handoff"}, 1)
	s.raftLayer.Handoff(conn)
}

func (s *Server) handleALPN_WANGossipPacketStream(conn net.Conn) error {
	defer conn.Close()

	transport := s.memberlistTransportWAN
	for {
		select {
		case <-s.shutdownCh:
			return nil
		default:
		}

		// Note: if we need to change this format to have additional header
		// information we can just negotiate a different ALPN protocol instead
		// of needing any sort of version field here.
		prefixLen, err := readUint32(conn, wanfed.GossipPacketMaxIdleTime)
		if err != nil {
			return err
		}

		// Avoid a memory exhaustion DOS vector here by capping how large this
		// packet can be to something reasonable.
		if prefixLen > wanfed.GossipPacketMaxByteSize {
			return fmt.Errorf("gossip packet size %d exceeds threshold of %d", prefixLen, wanfed.GossipPacketMaxByteSize)
		}

		lc := &limitedConn{
			Conn: conn,
			lr:   io.LimitReader(conn, int64(prefixLen)),
		}

		if err := transport.IngestPacket(lc, conn.RemoteAddr(), time.Now(), false); err != nil {
			return err
		}
	}
}

func readUint32(conn net.Conn, timeout time.Duration) (uint32, error) {
	// Since requests are framed we can easily just set a deadline on
	// reading that frame and then disable it for the rest of the body.
	if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return 0, err
	}

	var v uint32
	if err := binary.Read(conn, binary.BigEndian, &v); err != nil {
		return 0, err
	}

	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		return 0, err
	}

	return v, nil
}

type limitedConn struct {
	net.Conn
	lr io.Reader
}

func (c *limitedConn) Read(b []byte) (n int, err error) {
	return c.lr.Read(b)
}

func getWaitTime(rpcHoldTimeout time.Duration, retryCount int) time.Duration {
	const backoffMultiplier = 2.0

	rpcHoldTimeoutInMilli := int(rpcHoldTimeout.Milliseconds())
	initialBackoffInMilli := rpcHoldTimeoutInMilli / structs.JitterFraction

	if initialBackoffInMilli < 1 {
		initialBackoffInMilli = 1
	}

	waitTimeInMilli := initialBackoffInMilli * int(math.Pow(backoffMultiplier, float64(retryCount-1)))

	return time.Duration(waitTimeInMilli) * time.Millisecond
}

// canRetry returns true if the request and error indicate that a retry is safe.
func canRetry(info structs.RPCInfo, err error, start time.Time, config *Config, retryableMessages []error) bool {
	if info != nil {
		timedOut, timeoutError := info.HasTimedOut(start, config.RPCHoldTimeout, config.MaxQueryTime, config.DefaultQueryTime)
		if timeoutError != nil {
			return false
		}

		if timedOut {
			return false
		}
	}

	if info == nil && time.Since(start) > config.RPCHoldTimeout {
		// When not RPCInfo, timeout is only RPCHoldTimeout
		return false
	}
	// No leader errors are always safe to retry since no state could have
	// been changed.
	if structs.IsErrNoLeader(err) {
		return true
	}

	for _, m := range retryableMessages {
		if err != nil && strings.Contains(err.Error(), m.Error()) {
			return true
		}
	}

	// Reads are safe to retry for stream errors, such as if a server was
	// being shut down.
	return info != nil && info.IsRead() && lib.IsErrEOF(err)
}

// ForwardRPC is used to potentially forward an RPC request to a remote DC or
// to the local leader depending upon the request.
//
// Returns a bool of if forwarding was performed, as well as any error. If
// false is returned (with no error) it is assumed that the current server
// should handle the request.
func (s *Server) ForwardRPC(method string, info structs.RPCInfo, reply interface{}) (bool, error) {
	forwardToDC := func(dc string) error {
		return s.forwardDC(method, dc, info, reply)
	}
	forwardToLeader := func(leader *metadata.Server) error {
		return s.connPool.RPC(s.config.Datacenter, leader.ShortName, leader.Addr,
			method, info, reply)
	}
	return s.forwardRPC(info, forwardToDC, forwardToLeader)
}

// ForwardGRPC is used to potentially forward an RPC request to a remote DC or
// to the local leader depending upon the request.
//
// Returns a bool of if forwarding was performed, as well as any error. If
// false is returned (with no error) it is assumed that the current server
// should handle the request.
func (s *Server) ForwardGRPC(connPool GRPCClientConner, info structs.RPCInfo, f func(*grpc.ClientConn) error) (handled bool, err error) {
	forwardToDC := func(dc string) error {
		conn, err := connPool.ClientConn(dc)
		if err != nil {
			return err
		}
		return f(conn)
	}
	forwardToLeader := func(leader *metadata.Server) error {
		conn, err := connPool.ClientConnLeader()
		if err != nil {
			return err
		}
		return f(conn)
	}
	return s.forwardRPC(info, forwardToDC, forwardToLeader)
}

// forwardRPC is used to potentially forward an RPC request to a remote DC or
// to the local leader depending upon the request.
//
// If info.RequestDatacenter() does not match the local datacenter, then the
// request will be forwarded to the DC using forwardToDC.
//
// Stale read requests will be handled locally if the current node has an
// initialized raft database, otherwise requests will be forwarded to the local
// leader using forwardToLeader.
//
// Returns a bool of if forwarding was performed, as well as any error. If
// false is returned (with no error) it is assumed that the current server
// should handle the request.
func (s *Server) forwardRPC(
	info structs.RPCInfo,
	forwardToDC func(dc string) error,
	forwardToLeader func(leader *metadata.Server) error,
) (handled bool, err error) {
	// Forward the request to the requested datacenter.
	if handled, err := s.forwardRequestToOtherDatacenter(info, forwardToDC); handled || err != nil {
		return handled, err
	}

	// See if we should let this server handle the read request without
	// shipping the request to the leader.
	if s.canServeReadRequest(info) {
		return false, nil
	}

	return s.forwardRequestToLeader(info, forwardToLeader)
}

// forwardRequestToOtherDatacenter is an implementation detail of forwardRPC.
// See the comment for forwardRPC for more details.
func (s *Server) forwardRequestToOtherDatacenter(info structs.RPCInfo, forwardToDC func(dc string) error) (handled bool, err error) {
	// Handle DC forwarding
	dc := info.RequestDatacenter()
	if dc == "" {
		dc = s.config.Datacenter
	}
	if dc != s.config.Datacenter {
		// Local tokens only work within the current datacenter. Check to see
		// if we are attempting to forward one to a remote datacenter and strip
		// it, falling back on the anonymous token on the other end.
		if token := info.TokenSecret(); token != "" {
			done, ident, err := s.ResolveIdentityFromToken(token)
			if done {
				if err != nil && !acl.IsErrNotFound(err) {
					return false, err
				}
				if ident != nil && ident.IsLocal() {
					// Strip it from the request.
					info.SetTokenSecret("")
					defer info.SetTokenSecret(token)
				}
			}
		}
		// In order to interoperate with servers that can interpret Partition, but
		// may not handle it correctly (eg. 1.10 servers), we need to unset the value.
		// Unsetting the Partition ensures that the server that handles the request
		// uses its Partition, or an empty value (aka doing nothing).
		// For requests that are not Partition-aware, this is a no-op.
		if v, ok := info.(partitionUnsetter); ok {
			v.UnsetPartition()
		}

		return true, forwardToDC(dc)
	}

	return false, nil
}

// canServeReadRequest determines if the request is a stale read request and
// the current node can safely process that request.
func (s *Server) canServeReadRequest(info structs.RPCInfo) bool {
	// Check if we can allow a stale read, ensure our local DB is initialized
	return info.IsRead() && info.AllowStaleRead() && !s.raft.LastContact().IsZero()
}

// forwardRequestToLeader is an implementation detail of forwardRPC.
// See the comment for forwardRPC for more details.
func (s *Server) forwardRequestToLeader(info structs.RPCInfo, forwardToLeader func(leader *metadata.Server) error) (handled bool, err error) {
	firstCheck := time.Now()
	retryCount := 0
	previousJitter := time.Duration(0)
CHECK_LEADER:
	retryCount++
	// Fail fast if we are in the process of leaving
	select {
	case <-s.leaveCh:
		return true, structs.ErrNoLeader
	default:
	}

	// Find the leader
	isLeader, leader, rpcErr := s.getLeader()

	// Handle the case we are the leader
	if isLeader {
		return false, nil
	}

	// Handle the case of a known leader
	if leader != nil {
		rpcErr = forwardToLeader(leader)
		if rpcErr == nil {
			return true, nil
		}
	}

	retryableMessages := []error{
		// If we are chunking and it doesn't seem to have completed, try again.
		ErrChunkingResubmit,

		rate.ErrRetryLater,
	}

	if retry := canRetry(info, rpcErr, firstCheck, s.config, retryableMessages); retry {
		// Gate the request until there is a leader
		jitter := lib.RandomStaggerWithRange(previousJitter, getWaitTime(s.config.RPCHoldTimeout, retryCount))
		previousJitter = jitter

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
// elected a leader. In the case of not having a leader elected yet
// then a NoClusterLeader error gets returned. In the case of Raft having
// a leader but out internal tracking failing to find the leader we
// return a LeaderNotTracked error. Therefore if the err is nil AND
// the bool is false then the Server will be non-nil
func (s *Server) getLeader() (bool, *metadata.Server, error) {
	// Check if we are the leader
	if s.IsLeader() {
		return true, nil, nil
	}

	// Get the leader
	leader := s.raft.Leader()
	if leader == "" {
		return false, nil, structs.ErrNoLeader
	}

	// Lookup the server
	server := s.serverLookup.Server(leader)

	// if server is nil this indicates that while we have a Raft leader
	// something has caused that node to be considered unhealthy which
	// cascades into its removal from the serverLookup struct. In this case
	// we should not report no cluster leader but instead report a different
	// error so as not to confuse our users as to the what the root cause of
	// an issue might be.
	if server == nil {
		s.logger.Warn("Raft has a leader but other tracking of the node would indicate that the node is unhealthy or does not exist. The network may be misconfigured.", "leader", leader)
		return false, nil, structs.ErrLeaderNotTracked
	}

	return false, server, nil
}

// forwardDC is used to forward an RPC call to a remote DC, or fail if no servers
func (s *Server) forwardDC(method, dc string, args interface{}, reply interface{}) error {
	manager, server, ok := s.router.FindRoute(dc)
	if !ok {
		if s.router.HasDatacenter(dc) {
			s.rpcLogger().Warn("RPC request to DC is currently failing as no server can be reached", "datacenter", dc)
			return structs.ErrDCNotAvailable
		}
		s.rpcLogger().Warn("RPC request for DC is currently failing as no path was found",
			"datacenter", dc,
			"method", method,
		)
		return structs.ErrNoDCPath
	}

	metrics.IncrCounterWithLabels([]string{"rpc", "cross-dc"}, 1,
		[]metrics.Label{{Name: "datacenter", Value: dc}})
	if err := s.connPool.RPC(dc, server.ShortName, server.Addr, method, args, reply); err != nil {
		manager.NotifyFailedServer(server)
		s.rpcLogger().Error("RPC failed to server in DC",
			"server", server.Addr,
			"datacenter", dc,
			"method", method,
			"error", err,
		)
		return err
	}

	return nil
}

// keyringRPCs is used to forward an RPC request to a server in each dc. This
// will only error for RPC-related errors. Otherwise, application-level errors
// can be sent in the response objects.
func (s *Server) keyringRPCs(method string, args interface{}, dcs []string) (*structs.KeyringResponses, error) {

	errorCh := make(chan error, len(dcs))
	respCh := make(chan *structs.KeyringResponses, len(dcs))

	for _, dc := range dcs {
		go func(dc string) {
			rr := &structs.KeyringResponses{}
			if err := s.forwardDC(method, dc, args, &rr); err != nil {
				errorCh <- err
				return
			}
			respCh <- rr
		}(dc)
	}

	responses := &structs.KeyringResponses{}
	for i := 0; i < len(dcs); i++ {
		select {
		case err := <-errorCh:
			return nil, err
		case rr := <-respCh:
			responses.Add(rr)
		}
	}
	return responses, nil
}

type raftEncoder func(structs.MessageType, interface{}) ([]byte, error)

// leaderRaftApply is used by the leader to persist data to Raft for internal cluster management activities.
// This method MUST not be called from RPC endpoints, since it would result in duplicated RPC metrics.
func (s *Server) leaderRaftApply(method string, t structs.MessageType, msg interface{}) (interface{}, error) {
	start := time.Now()

	resp, err := s.raftApplyMsgpack(t, msg)
	s.rpcRecorder.Record(method, middleware.RPCTypeInternal, start, &msg, err != nil)

	return resp, err
}

// raftApplyMsgpack encodes the msg using msgpack and calls raft.Apply. See
// raftApplyWithEncoder.
// Deprecated: use raftApplyMsgpack
func (s *Server) raftApply(t structs.MessageType, msg interface{}) (interface{}, error) {
	return s.raftApplyMsgpack(t, msg)
}

// raftApplyMsgpack encodes the msg using msgpack and calls raft.Apply. See
// raftApplyWithEncoder.
func (s *Server) raftApplyMsgpack(t structs.MessageType, msg interface{}) (interface{}, error) {
	return s.raftApplyWithEncoder(t, msg, structs.Encode)
}

// raftApplyProtobuf encodes the msg using protobuf and calls raft.Apply. See
// raftApplyWithEncoder.
func (s *Server) raftApplyProtobuf(t structs.MessageType, msg interface{}) (interface{}, error) {
	return s.raftApplyWithEncoder(t, msg, structs.EncodeProtoInterface)
}

// raftApplyWithEncoder encodes a message, and then calls raft.Apply with the
// encoded message. Returns the FSM response along with any errors. If the
// FSM.Apply response is an error it will be returned as the error return
// value with a nil response.
func (s *Server) raftApplyWithEncoder(
	t structs.MessageType,
	msg interface{},
	encoder raftEncoder,
) (response interface{}, err error) {
	if encoder == nil {
		return nil, fmt.Errorf("Failed to encode request: nil encoder")
	}
	buf, err := encoder(t, msg)
	if err != nil {
		return nil, fmt.Errorf("Failed to encode request: %v", err)
	}

	// Warn if the command is very large
	if n := len(buf); n > raftWarnSize {
		s.rpcLogger().Warn("Attempting to apply large raft entry", "size_in_bytes", n)
	}

	var chunked bool
	var future raft.ApplyFuture
	switch {
	case len(buf) <= raft.SuggestedMaxDataSize || t != structs.KVSRequestType:
		future = s.raft.Apply(buf, enqueueLimit)
	default:
		chunked = true
		future = raftchunking.ChunkingApply(buf, nil, enqueueLimit, s.raft.ApplyLog)
	}

	if err := future.Error(); err != nil {
		return nil, err
	}

	resp := future.Response()

	if chunked {
		// In this case we didn't apply all chunks successfully, possibly due
		// to a term change; resubmit
		if resp == nil {
			return nil, ErrChunkingResubmit
		}
		// We expect that this conversion should always work
		chunkedSuccess, ok := resp.(raftchunking.ChunkingSuccess)
		if !ok {
			return nil, errors.New("unknown type of response back from chunking FSM")
		}
		resp = chunkedSuccess.Response
	}

	if err, ok := resp.(error); ok {
		return nil, err
	}
	return resp, nil
}

// queryFn is used to perform a query operation. See Server.blockingQuery for
// the requirements of this function.
type queryFn func(memdb.WatchSet, *state.Store) error

// blockingQueryOptions are options used by Server.blockingQuery to modify the
// behaviour of the query operation, or to populate response metadata.
type blockingQueryOptions interface {
	GetToken() string
	GetMinQueryIndex() uint64
	GetMaxQueryTime() (time.Duration, error)
	GetRequireConsistent() bool
}

// blockingQueryResponseMeta is an interface used to populate the response struct
// with metadata about the query and the state of the server.
type blockingQueryResponseMeta interface {
	SetLastContact(time.Duration)
	SetKnownLeader(bool)
	GetIndex() uint64
	SetIndex(uint64)
	SetResultsFilteredByACLs(bool)
}

// blockingQuery is a passthrough to blockingquery.Query that keeps API
// compatibility with Server. That has RPC and FSM machinery mixed in the same consul
// package.
func (s *Server) blockingQuery(
	requestOpts blockingquery.RequestOptions,
	responseMeta blockingquery.ResponseMeta,
	query blockingquery.QueryFn,
) error {
	return blockingquery.Query(s, requestOpts, responseMeta, query)
}

var (
	errNotFound   = blockingquery.ErrNotFound
	errNotChanged = blockingquery.ErrNotChanged
)

// SetQueryMeta is used to populate the QueryMeta data for an RPC call
//
// Note: This method must be called *after* filtering query results with ACLs.
func (s *Server) SetQueryMeta(m blockingquery.ResponseMeta, token string) {
	if s.IsLeader() {
		m.SetLastContact(0)
		m.SetKnownLeader(true)
	} else {
		m.SetLastContact(time.Since(s.raft.LastContact()))
		m.SetKnownLeader(s.raft.Leader() != "")
	}
	maskResultsFilteredByACLs(token, m)

	// Always set a non-zero QueryMeta.Index. Generally we expect the
	// QueryMeta.Index to be set to structs.RaftIndex.ModifyIndex. If the query
	// returned no results we expect it to be set to the max index of the table,
	// however we can't guarantee this always happens.
	// To prevent a client from accidentally performing many non-blocking queries
	// (which causes lots of unnecessary load), we always set a default value of 1.
	// This is sufficient to prevent the unnecessary load in most cases.
	if m.GetIndex() < 1 {
		m.SetIndex(1)
	}
}

// consistentRead is used to ensure we do not perform a stale
// read. This is done by verifying leadership before the read.
func (s *Server) ConsistentRead() error {
	defer metrics.MeasureSince([]string{"rpc", "consistentRead"}, time.Now())
	future := s.raft.VerifyLeader()
	if err := future.Error(); err != nil {
		return err // fail fast if leader verification fails
	}
	// poll consistent read readiness, wait for up to RPCHoldTimeout milliseconds
	if s.isReadyForConsistentReads() {
		return nil
	}
	jitter := lib.RandomStagger(s.config.RPCHoldTimeout / structs.JitterFraction)
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

// RPCQueryTimeout calculates the timeout for the query, ensures it is
// constrained to the configured limit, and adds jitter to prevent multiple
// blocking queries from all timing out at the same time.
func (s *Server) RPCQueryTimeout(queryTimeout time.Duration) time.Duration {
	// Restrict the max query time, and ensure there is always one.
	if queryTimeout > s.config.MaxQueryTime {
		queryTimeout = s.config.MaxQueryTime
	} else if queryTimeout <= 0 {
		queryTimeout = s.config.DefaultQueryTime
	}

	// Apply a small amount of jitter to the request.
	queryTimeout += lib.RandomStagger(queryTimeout / structs.JitterFraction)
	return queryTimeout
}

// maskResultsFilteredByACLs blanks out the ResultsFilteredByACLs flag if the
// request is unauthenticated, to limit information leaking.
//
// Endpoints that support bexpr filtering could be used in combination with
// this flag/header to discover the existence of resources to which the user
// does not have access, therefore we only expose it when the user presents
// a valid ACL token. This doesn't completely remove the risk (by nature the
// purpose of this flag is to let the user know there are resources they can
// not access) but it prevents completely unauthenticated users from doing so.
//
// Notes:
//
//   - The definition of "unauthenticated" here is incomplete, as it doesn't
//     account for the fact that operators can modify the anonymous token with
//     custom policies, or set namespace default policies. As these scenarios
//     are less common and this flag is a best-effort UX improvement, we think
//     the trade-off for reduced complexity is acceptable.
//
//   - This method assumes that the given token has already been validated (and
//     will only check whether it is blank or not). It's a safe assumption because
//     ResultsFilteredByACLs is only set to try when applying the already-resolved
//     token's policies.
func maskResultsFilteredByACLs(token string, meta blockingQueryResponseMeta) {
	if token == "" {
		meta.SetResultsFilteredByACLs(false)
	}
}
