package consul

import (
	"crypto/tls"
	"fmt"
	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/consul/structs"
	"github.com/inconshreveable/muxado"
	"github.com/ugorji/go/codec"
	"io"
	"math/rand"
	"net"
	"strings"
	"time"
)

type RPCType byte

const (
	rpcConsul RPCType = iota
	rpcRaft
	rpcMultiplex
	rpcTLS
)

const (
	maxQueryTime = 600 * time.Second
)

// listen is used to listen for incoming RPC connections
func (s *Server) listen() {
	for {
		// Accept a connection
		conn, err := s.rpcListener.Accept()
		if err != nil {
			if s.shutdown {
				return
			}
			s.logger.Printf("[ERR] consul.rpc: failed to accept RPC conn: %v", err)
			continue
		}

		// Track this client
		s.rpcClientLock.Lock()
		s.rpcClients[conn] = struct{}{}
		s.rpcClientLock.Unlock()

		go s.handleConn(conn, false)
		metrics.IncrCounter([]string{"consul", "rpc", "accept_conn"}, 1)
	}
}

// handleConn is used to determine if this is a Raft or
// Consul type RPC connection and invoke the correct handler
func (s *Server) handleConn(conn net.Conn, isTLS bool) {
	// Read a single byte
	buf := make([]byte, 1)
	if _, err := conn.Read(buf); err != nil {
		s.logger.Printf("[ERR] consul.rpc: failed to read byte: %v", err)
		conn.Close()
		return
	}

	// Enforce TLS if VerifyIncoming is set
	if s.config.VerifyIncoming && !isTLS && RPCType(buf[0]) != rpcTLS {
		s.logger.Printf("[WARN] consul.rpc: Non-TLS connection attempted with VerifyIncoming set")
		conn.Close()
		return
	}

	// Switch on the byte
	switch RPCType(buf[0]) {
	case rpcConsul:
		s.handleConsulConn(conn)

	case rpcRaft:
		metrics.IncrCounter([]string{"consul", "rpc", "raft_handoff"}, 1)
		s.raftLayer.Handoff(conn)

	case rpcMultiplex:
		s.handleMultiplex(conn)

	case rpcTLS:
		if s.rpcTLS == nil {
			s.logger.Printf("[WARN] consul.rpc: TLS connection attempted, server not configured for TLS")
			conn.Close()
			return
		}
		conn = tls.Server(conn, s.rpcTLS)
		s.handleConn(conn, true)

	default:
		s.logger.Printf("[ERR] consul.rpc: unrecognized RPC byte: %v", buf[0])
		conn.Close()
		return
	}
}

// handleMultiplex is used to multiplex a single incoming connection
func (s *Server) handleMultiplex(conn net.Conn) {
	defer conn.Close()
	server := muxado.Server(conn)
	for {
		sub, err := server.Accept()
		if err != nil {
			if !strings.Contains(err.Error(), "closed") {
				s.logger.Printf("[ERR] consul.rpc: multiplex conn accept failed: %v", err)
			}
			return
		}
		go s.handleConsulConn(sub)
	}
}

// handleConsulConn is used to service a single Consul RPC connection
func (s *Server) handleConsulConn(conn net.Conn) {
	defer func() {
		conn.Close()
		s.rpcClientLock.Lock()
		delete(s.rpcClients, conn)
		s.rpcClientLock.Unlock()
	}()

	rpcCodec := codec.GoRpc.ServerCodec(conn, &codec.MsgpackHandle{})
	for !s.shutdown {
		if err := s.rpcServer.ServeRequest(rpcCodec); err != nil {
			if err != io.EOF {
				s.logger.Printf("[ERR] consul.rpc: RPC error: %v (%v)", err, conn)
			}
			return
		}
	}
}

// forward is used to forward to a remote DC or to forward to the local leader
// Returns a bool of if forwarding was performed, as well as any error
func (s *Server) forward(method string, info structs.RPCInfo, args interface{}, reply interface{}) (bool, error) {
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

	// Handle leader forwarding
	if !s.IsLeader() {
		err := s.forwardLeader(method, args, reply)
		return true, err
	}
	return false, nil
}

// forwardLeader is used to forward an RPC call to the leader, or fail if no leader
func (s *Server) forwardLeader(method string, args interface{}, reply interface{}) error {
	leader := s.raft.Leader()
	if leader == nil {
		return structs.ErrNoLeader
	}
	return s.connPool.RPC(leader, method, args, reply)
}

// forwardDC is used to forward an RPC call to a remote DC, or fail if no servers
func (s *Server) forwardDC(method, dc string, args interface{}, reply interface{}) error {
	// Bail if we can't find any servers
	s.remoteLock.RLock()
	servers := s.remoteConsuls[dc]
	if len(servers) == 0 {
		s.remoteLock.RUnlock()
		s.logger.Printf("[WARN] consul.rpc: RPC request for DC '%s', no path found", dc)
		return structs.ErrNoDCPath
	}

	// Select a random addr
	offset := rand.Int31() % int32(len(servers))
	server := servers[offset]
	s.remoteLock.RUnlock()

	// Forward to remote Consul
	metrics.IncrCounter([]string{"consul", "rpc", "cross-dc", dc}, 1)
	return s.connPool.RPC(server, method, args, reply)
}

// raftApply is used to encode a message, run it through raft, and return
// the FSM response along with any errors
func (s *Server) raftApply(t structs.MessageType, msg interface{}) (interface{}, error) {
	buf, err := structs.Encode(t, msg)
	if err != nil {
		return nil, fmt.Errorf("Failed to encode request: %v", err)
	}

	future := s.raft.Apply(buf, 0)
	if err := future.Error(); err != nil {
		return nil, err
	}

	return future.Response(), nil
}

// blockingRPC is used for queries that need to wait for a
// minimum index. This is used to block and wait for changes.
func (s *Server) blockingRPC(b *structs.BlockingQuery, tables MDBTables, run func() (uint64, error)) error {
	var timeout <-chan time.Time
	var notifyCh chan struct{}

	// Fast path non-blocking
	if b.MinQueryIndex == 0 {
		goto RUN_QUERY
	}

	// Sanity check that we have tables to block on
	if len(tables) == 0 {
		panic("no tables to block on")
	}

	// Restrict the max query time
	if b.MaxQueryTime > maxQueryTime {
		b.MaxQueryTime = maxQueryTime
	}

	// Ensure a time limit is set if we have an index
	if b.MinQueryIndex > 0 && b.MaxQueryTime == 0 {
		b.MaxQueryTime = maxQueryTime
	}

	// Setup a query timeout
	if b.MaxQueryTime > 0 {
		timeout = time.After(b.MaxQueryTime)
	}

	// Setup a notification channel for changes
SETUP_NOTIFY:
	if b.MinQueryIndex > 0 {
		notifyCh = make(chan struct{}, 1)
		s.fsm.State().Watch(tables, notifyCh)
	}

	// Run the query function
RUN_QUERY:
	idx, err := run()

	// Check for minimum query time
	if err == nil && idx <= b.MinQueryIndex {
		select {
		case <-notifyCh:
			goto SETUP_NOTIFY
		case <-timeout:
		}
	}
	return err
}
