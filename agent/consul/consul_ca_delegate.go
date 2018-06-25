package consul

import (
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
)

// consulCADelegate providers callbacks for the Consul CA provider
// to use the state store for its operations.
type consulCADelegate struct {
	srv *Server
}

func (c *consulCADelegate) State() *state.Store {
	return c.srv.fsm.State()
}

func (c *consulCADelegate) ApplyCARequest(req *structs.CARequest) error {
	resp, err := c.srv.raftApply(structs.ConnectCARequestType, req)
	if err != nil {
		return err
	}
	if respErr, ok := resp.(error); ok {
		return respErr
	}

	return nil
}
