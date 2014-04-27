package consul

import (
	"github.com/hashicorp/consul/consul/structs"
)

// Misc endpoint is used to query the miscellaneous info that
// does not necessarily fit into the other systems. It is also
// used to hold undocumented APIs that users should not rely on.
type Misc struct {
	srv *Server
}

// ChecksInState is used to get all the checks in a given state
func (m *Misc) NodeInfo(args *structs.NodeSpecificRequest,
	reply *structs.IndexedNodeDump) error {
	if done, err := m.srv.forward("Misc.NodeInfo", args, args, reply); done {
		return err
	}

	// Get the state specific checks
	state := m.srv.fsm.State()
	return m.srv.blockingRPC(&args.QueryOptions,
		&reply.QueryMeta,
		state.QueryTables("NodeInfo"),
		func() error {
			reply.Index, reply.Dump = state.NodeInfo(args.Node)
			return nil
		})
}

// ChecksInState is used to get all the checks in a given state
func (m *Misc) NodeDump(args *structs.DCSpecificRequest,
	reply *structs.IndexedNodeDump) error {
	if done, err := m.srv.forward("Misc.NodeDump", args, args, reply); done {
		return err
	}

	// Get the state specific checks
	state := m.srv.fsm.State()
	return m.srv.blockingRPC(&args.QueryOptions,
		&reply.QueryMeta,
		state.QueryTables("NodeDump"),
		func() error {
			reply.Index, reply.Dump = state.NodeDump()
			return nil
		})
}
