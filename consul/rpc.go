package consul

import (
	"github.com/ugorji/go/codec"
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
			s.logger.Printf("[ERR] RPC error: %v (%v)", err, conn)
			return
		}
	}
}
