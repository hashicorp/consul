package consul

import (
	"fmt"
	"github.com/hashicorp/consul/rpc"
	"github.com/ugorji/go/codec"
	"io"
	"math/rand"
	"net"
)

type RPCType byte

const (
	rpcConsul RPCType = iota
	rpcRaft
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
			s.logger.Printf("[ERR] Failed to accept RPC conn: %v", err)
			continue
		}

		// Track this client
		s.rpcClientLock.Lock()
		s.rpcClients[conn] = struct{}{}
		s.rpcClientLock.Unlock()

		go s.handleConn(conn)
	}
}

// handleConn is used to determine if this is a Raft or
// Consul type RPC connection and invoke the correct handler
func (s *Server) handleConn(conn net.Conn) {
	// Read a single byte
	buf := make([]byte, 1)
	if _, err := conn.Read(buf); err != nil {
		s.logger.Printf("[ERR] Failed to read byte: %v", err)
		conn.Close()
		return
	}

	// Switch on the byte
	switch RPCType(buf[0]) {
	case rpcConsul:
		s.handleConsulConn(conn)

	case rpcRaft:
		s.raftLayer.Handoff(conn)

	default:
		s.logger.Printf("[ERR] Unrecognized RPC byte: %v", buf[0])
		conn.Close()
		return
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
				s.logger.Printf("[ERR] RPC error: %v (%v)", err, conn)
			}
			return
		}
	}
}

// forward is used to forward to a remote DC or to forward to the local leader
// Returns a bool of if forwarding was performed, as well as any error
func (s *Server) forward(method, dc string, args interface{}, reply interface{}) (bool, error) {
	// Handle DC forwarding
	if dc != s.config.Datacenter {
		err := s.forwardDC(method, dc, args, reply)
		return true, err
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
		return rpc.ErrNoLeader
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
		return rpc.ErrNoDCPath
	}

	// Select a random addr
	offset := rand.Int31() % int32(len(servers))
	server := servers[offset]
	s.remoteLock.RUnlock()

	// Forward to remote Consul
	return s.connPool.RPC(server, method, args, reply)
}

// raftApply is used to encode a message, run it through raft, and return
// the FSM response along with any errors
func (s *Server) raftApply(t rpc.MessageType, msg interface{}) (interface{}, error) {
	buf, err := rpc.Encode(t, msg)
	if err != nil {
		return nil, fmt.Errorf("Failed to encode request: %v", err)
	}

	future := s.raft.Apply(buf, 0)
	if err := future.Error(); err != nil {
		return nil, err
	}

	return future.Response(), nil
}
