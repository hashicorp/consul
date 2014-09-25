package consul

import (
	"github.com/hashicorp/consul/consul/structs"
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

func (m *Internal) ListKeys(
	args *structs.KeyringRequest,
	reply *structs.KeyringResponses) error {

	respLAN, err := m.srv.KeyManagerLAN().ListKeys()
	if err != nil {
		return err
	}
	reply.Responses = append(reply.Responses, &structs.KeyringResponse{
		Datacenter: m.srv.config.Datacenter,
		Messages:   respLAN.Messages,
		Keys:       respLAN.Keys,
		NumResp:    respLAN.NumResp,
		NumNodes:   respLAN.NumNodes,
		NumErr:     respLAN.NumErr,
	})

	if !args.Forwarded {
		respWAN, err := m.srv.KeyManagerWAN().ListKeys()
		if err != nil {
			return err
		}
		reply.Responses = append(reply.Responses, &structs.KeyringResponse{
			Datacenter: m.srv.config.Datacenter,
			Messages:   respWAN.Messages,
			Keys:       respWAN.Keys,
			NumResp:    respWAN.NumResp,
			NumNodes:   respWAN.NumNodes,
			NumErr:     respWAN.NumErr,
		})

		// Mark key rotation as being already forwarded, then forward.
		args.Forwarded = true
		return m.srv.forwardAll("Internal.ListKeys", args, reply)
	}

	return nil
}

func (m *Internal) InstallKey(
	args *structs.KeyringRequest,
	reply *structs.KeyringResponses) error {

	respLAN, _ := m.srv.KeyManagerLAN().InstallKey(args.Key)
	reply.Responses = append(reply.Responses, &structs.KeyringResponse{
		Datacenter: m.srv.config.Datacenter,
		Messages:   respLAN.Messages,
		Keys:       respLAN.Keys,
		NumResp:    respLAN.NumResp,
		NumNodes:   respLAN.NumNodes,
		NumErr:     respLAN.NumErr,
	})

	if !args.Forwarded {
		respWAN, _ := m.srv.KeyManagerWAN().InstallKey(args.Key)
		reply.Responses = append(reply.Responses, &structs.KeyringResponse{
			Datacenter: m.srv.config.Datacenter,
			Messages:   respWAN.Messages,
			Keys:       respWAN.Keys,
			NumResp:    respWAN.NumResp,
			NumNodes:   respWAN.NumNodes,
			NumErr:     respWAN.NumErr,
		})

		// Mark key rotation as being already forwarded, then forward.
		args.Forwarded = true
		return m.srv.forwardAll("Internal.InstallKey", args, reply)
	}

	return nil
}

func (m *Internal) UseKey(
	args *structs.KeyringRequest,
	reply *structs.KeyringResponses) error {

	respLAN, _ := m.srv.KeyManagerLAN().UseKey(args.Key)
	reply.Responses = append(reply.Responses, &structs.KeyringResponse{
		Datacenter: m.srv.config.Datacenter,
		Messages:   respLAN.Messages,
		Keys:       respLAN.Keys,
		NumResp:    respLAN.NumResp,
		NumNodes:   respLAN.NumNodes,
		NumErr:     respLAN.NumErr,
	})

	if !args.Forwarded {
		respWAN, _ := m.srv.KeyManagerWAN().UseKey(args.Key)
		reply.Responses = append(reply.Responses, &structs.KeyringResponse{
			Datacenter: m.srv.config.Datacenter,
			Messages:   respWAN.Messages,
			Keys:       respWAN.Keys,
			NumResp:    respWAN.NumResp,
			NumNodes:   respWAN.NumNodes,
			NumErr:     respWAN.NumErr,
		})

		// Mark key rotation as being already forwarded, then forward.
		args.Forwarded = true
		return m.srv.forwardAll("Internal.UseKey", args, reply)
	}

	return nil
}

func (m *Internal) RemoveKey(
	args *structs.KeyringRequest,
	reply *structs.KeyringResponses) error {

	respLAN, _ := m.srv.KeyManagerLAN().RemoveKey(args.Key)
	reply.Responses = append(reply.Responses, &structs.KeyringResponse{
		Datacenter: m.srv.config.Datacenter,
		Messages:   respLAN.Messages,
		Keys:       respLAN.Keys,
		NumResp:    respLAN.NumResp,
		NumNodes:   respLAN.NumNodes,
		NumErr:     respLAN.NumErr,
	})

	if !args.Forwarded {
		respWAN, _ := m.srv.KeyManagerWAN().RemoveKey(args.Key)
		reply.Responses = append(reply.Responses, &structs.KeyringResponse{
			Datacenter: m.srv.config.Datacenter,
			Messages:   respWAN.Messages,
			Keys:       respWAN.Keys,
			NumResp:    respWAN.NumResp,
			NumNodes:   respWAN.NumNodes,
			NumErr:     respWAN.NumErr,
		})

		// Mark key rotation as being already forwarded, then forward.
		args.Forwarded = true
		return m.srv.forwardAll("Internal.RemoveKey", args, reply)
	}

	return nil
}
