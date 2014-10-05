package consul

import (
	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/serf/serf"
)

// Internal endpoint is used to query the miscellaneous info that
// does not necessarily fit into the other systems. It is also
// used to hold undocumented APIs that users should not rely on.
type Internal struct {
	srv *Server
}

// ChecksInState is used to get all the checks in a given state
func (m *Internal) NodeInfo(args *structs.NodeSpecificRequest,
	reply *structs.IndexedNodeDump) error {
	if done, err := m.srv.forward("Internal.NodeInfo", args, args, reply); done {
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
func (m *Internal) NodeDump(args *structs.DCSpecificRequest,
	reply *structs.IndexedNodeDump) error {
	if done, err := m.srv.forward("Internal.NodeDump", args, args, reply); done {
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

// EventFire is a bit of an odd endpoint, but it allows for a cross-DC RPC
// call to fire an event. The primary use case is to enable user events being
// triggered in a remote DC.
func (m *Internal) EventFire(args *structs.EventFireRequest,
	reply *structs.EventFireResponse) error {
	if done, err := m.srv.forward("Internal.EventFire", args, args, reply); done {
		return err
	}

	// Set the query meta data
	m.srv.setQueryMeta(&reply.QueryMeta)

	// Fire the event
	return m.srv.UserEvent(args.Name, args.Payload)
}

// KeyringOperation will query the WAN and LAN gossip keyrings of all nodes,
// adding results into a collective response as we go. It can describe requests
// for all keyring-related operations.
func (m *Internal) KeyringOperation(
	args *structs.KeyringRequest,
	reply *structs.KeyringResponses) error {

	dc := m.srv.config.Datacenter

	respLAN, err := executeKeyringOp(args, m.srv.KeyManagerLAN())
	ingestKeyringResponse(respLAN, reply, dc, false, err)

	if !args.Forwarded {
		respWAN, err := executeKeyringOp(args, m.srv.KeyManagerWAN())
		ingestKeyringResponse(respWAN, reply, dc, true, err)

		args.Forwarded = true
		return m.srv.globalRPC("Internal.KeyringOperation", args, reply)
	}

	return nil
}

// executeKeyringOp executes the appropriate keyring-related function based on
// the type of keyring operation in the request. It takes the KeyManager as an
// argument, so it can handle any operation for either LAN or WAN pools.
func executeKeyringOp(
	args *structs.KeyringRequest,
	mgr *serf.KeyManager) (r *serf.KeyResponse, err error) {

	switch args.Operation {
	case structs.KeyringList:
		r, err = mgr.ListKeys()
	case structs.KeyringInstall:
		r, err = mgr.InstallKey(args.Key)
	case structs.KeyringUse:
		r, err = mgr.UseKey(args.Key)
	case structs.KeyringRemove:
		r, err = mgr.RemoveKey(args.Key)
	}

	return r, err
}

// ingestKeyringResponse is a helper method to pick the relative information
// from a Serf message and stuff it into a KeyringResponse.
func ingestKeyringResponse(
	serfResp *serf.KeyResponse, reply *structs.KeyringResponses,
	dc string, wan bool, err error) {

	errStr := ""
	if err != nil {
		errStr = err.Error()
	}

	reply.Responses = append(reply.Responses, &structs.KeyringResponse{
		WAN:        wan,
		Datacenter: dc,
		Messages:   serfResp.Messages,
		Keys:       serfResp.Keys,
		NumNodes:   serfResp.NumNodes,
		Error:      errStr,
	})
}
