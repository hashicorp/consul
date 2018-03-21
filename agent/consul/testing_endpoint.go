package consul

import (
	"github.com/hashicorp/consul/agent/structs"
)

// Test is an RPC endpoint that is only available during `go test` when
// `TestEndpoint` is called. This is not and must not ever be available
// during a real running Consul agent, since it this endpoint bypasses
// critical ACL checks.
type Test struct {
	// srv is a pointer back to the server.
	srv *Server
}

// ConnectCASetRoots sets the current CA roots state.
func (s *Test) ConnectCASetRoots(
	args []*structs.CARoot,
	reply *interface{}) error {

	// Get the highest index
	state := s.srv.fsm.State()
	idx, _, err := state.CARoots(nil)
	if err != nil {
		return err
	}

	// Commit
	resp, err := s.srv.raftApply(structs.ConnectCARequestType, &structs.CARequest{
		Op:    structs.CAOpSet,
		Index: idx,
		Roots: args,
	})
	if err != nil {
		s.srv.logger.Printf("[ERR] consul.test: Apply failed %v", err)
		return err
	}
	if respErr, ok := resp.(error); ok {
		return respErr
	}

	return nil
}
