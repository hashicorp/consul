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

func (m *Internal) ingestKeyringResponse(
	resp *serf.KeyResponse,
	reply *structs.KeyringResponses) {

	reply.Responses = append(reply.Responses, &structs.KeyringResponse{
		Datacenter: m.srv.config.Datacenter,
		Messages:   resp.Messages,
		Keys:       resp.Keys,
		NumResp:    resp.NumResp,
		NumNodes:   resp.NumNodes,
		NumErr:     resp.NumErr,
	})
}

// ListKeys will query the WAN and LAN gossip keyrings of all nodes, adding
// results into a collective response as we go.
func (m *Internal) ListKeys(
	args *structs.KeyringRequest,
	reply *structs.KeyringResponses) error {

	respLAN, err := m.srv.KeyManagerLAN().ListKeys()
	if err != nil {
		return err
	}
	m.ingestKeyringResponse(respLAN, reply)

	if !args.Forwarded {
		respWAN, err := m.srv.KeyManagerWAN().ListKeys()
		if err != nil {
			return err
		}
		m.ingestKeyringResponse(respWAN, reply)

		// Mark key rotation as being already forwarded, then forward.
		args.Forwarded = true
		return m.srv.forwardAll("Internal.ListKeys", args, reply)
	}

	return nil
}

// InstallKey broadcasts a new encryption key to all nodes. This involves
// installing a new key on every node across all datacenters.
func (m *Internal) InstallKey(
	args *structs.KeyringRequest,
	reply *structs.KeyringResponses) error {

	respLAN, err := m.srv.KeyManagerLAN().InstallKey(args.Key)
	if err != nil {
		return err
	}
	m.ingestKeyringResponse(respLAN, reply)

	if !args.Forwarded {
		respWAN, err := m.srv.KeyManagerWAN().InstallKey(args.Key)
		if err != nil {
			return err
		}
		m.ingestKeyringResponse(respWAN, reply)

		args.Forwarded = true
		return m.srv.forwardAll("Internal.InstallKey", args, reply)
	}

	return nil
}

// UseKey instructs all nodes to change the key they are using to
// encrypt gossip messages.
func (m *Internal) UseKey(
	args *structs.KeyringRequest,
	reply *structs.KeyringResponses) error {

	respLAN, err := m.srv.KeyManagerLAN().UseKey(args.Key)
	if err != nil {
		return err
	}
	m.ingestKeyringResponse(respLAN, reply)

	if !args.Forwarded {
		respWAN, err := m.srv.KeyManagerWAN().UseKey(args.Key)
		if err != nil {
			return err
		}
		m.ingestKeyringResponse(respWAN, reply)

		args.Forwarded = true
		return m.srv.forwardAll("Internal.UseKey", args, reply)
	}

	return nil
}

// RemoveKey instructs all nodes to drop the specified key from the keyring.
func (m *Internal) RemoveKey(
	args *structs.KeyringRequest,
	reply *structs.KeyringResponses) error {

	respLAN, err := m.srv.KeyManagerLAN().RemoveKey(args.Key)
	if err != nil {
		return err
	}
	m.ingestKeyringResponse(respLAN, reply)

	if !args.Forwarded {
		respWAN, err := m.srv.KeyManagerWAN().RemoveKey(args.Key)
		if err != nil {
			return err
		}
		m.ingestKeyringResponse(respWAN, reply)

		args.Forwarded = true
		return m.srv.forwardAll("Internal.RemoveKey", args, reply)
	}

	return nil
}
