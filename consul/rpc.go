package consul

import (
	"github.com/ugorji/go/codec"
	"net"
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

// handleConn is used to service a single RPC connection
func (s *Server) handleConn(conn net.Conn) {
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
