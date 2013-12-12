package consul

import (
	"net"
)

// Raft endpoint is used to manipulate the Raft subsystem
type Raft struct {
	server *Server
}

func (r *Raft) RemovePeer(args string, reply *struct{}) error {
	peer, err := net.ResolveTCPAddr("tcp", args)
	if err != nil {
		r.server.logger.Printf("[ERR] Failed to parse peer: %v", err)
		return err
	}
	future := r.server.raft.RemovePeer(peer)
	return future.Error()
}
