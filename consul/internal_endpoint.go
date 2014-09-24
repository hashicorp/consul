package consul

import (
	"fmt"

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

// TODO(ryanuber): Clean up all of these methods
func (m *Internal) InstallKey(args *structs.KeyringRequest,
	reply *structs.KeyringResponse) error {

	var respLAN, respWAN *serf.KeyResponse
	var err error

	if reply.Messages == nil {
		reply.Messages = make(map[string]string)
	}
	if reply.Keys == nil {
		reply.Keys = make(map[string]int)
	}

	m.srv.setQueryMeta(&reply.QueryMeta)

	// Do a LAN key install. This will be invoked in each DC once the RPC call
	// is forwarded below.
	respLAN, err = m.srv.KeyManagerLAN().InstallKey(args.Key)
	for node, msg := range respLAN.Messages {
		reply.Messages["client."+node+"."+m.srv.config.Datacenter] = msg
	}
	reply.NumResp += respLAN.NumResp
	reply.NumErr += respLAN.NumErr
	reply.NumNodes += respLAN.NumNodes
	if err != nil {
		return fmt.Errorf("failed rotating LAN keyring in %s: %s",
			m.srv.config.Datacenter,
			err)
	}

	if !args.Forwarded {
		// Only perform WAN key rotation once.
		respWAN, err = m.srv.KeyManagerWAN().InstallKey(args.Key)
		if err != nil {
			return err
		}
		for node, msg := range respWAN.Messages {
			reply.Messages["server."+node] = msg
		}
		reply.NumResp += respWAN.NumResp
		reply.NumErr += respWAN.NumErr
		reply.NumNodes += respWAN.NumNodes

		// Mark key rotation as being already forwarded, then forward.
		args.Forwarded = true
		return m.srv.forwardAll("Internal.InstallKey", args, reply)
	}

	return nil
}

func (m *Internal) UseKey(args *structs.KeyringRequest,
	reply *structs.KeyringResponse) error {
	var respLAN, respWAN *serf.KeyResponse
	var err error

	if reply.Messages == nil {
		reply.Messages = make(map[string]string)
	}
	if reply.Keys == nil {
		reply.Keys = make(map[string]int)
	}

	m.srv.setQueryMeta(&reply.QueryMeta)

	// Do a LAN key install. This will be invoked in each DC once the RPC call
	// is forwarded below.
	respLAN, err = m.srv.KeyManagerLAN().UseKey(args.Key)
	for node, msg := range respLAN.Messages {
		reply.Messages["client."+node+"."+m.srv.config.Datacenter] = msg
	}
	reply.NumResp += respLAN.NumResp
	reply.NumErr += respLAN.NumErr
	reply.NumNodes += respLAN.NumNodes
	if err != nil {
		return fmt.Errorf("failed rotating LAN keyring in %s: %s",
			m.srv.config.Datacenter,
			err)
	}

	if !args.Forwarded {
		// Only perform WAN key rotation once.
		respWAN, err = m.srv.KeyManagerWAN().UseKey(args.Key)
		if err != nil {
			return err
		}
		for node, msg := range respWAN.Messages {
			reply.Messages["server."+node] = msg
		}
		reply.NumResp += respWAN.NumResp
		reply.NumErr += respWAN.NumErr
		reply.NumNodes += respWAN.NumNodes

		// Mark key rotation as being already forwarded, then forward.
		args.Forwarded = true
		return m.srv.forwardAll("Internal.UseKey", args, reply)
	}

	return nil
}

func (m *Internal) RemoveKey(args *structs.KeyringRequest,
	reply *structs.KeyringResponse) error {
	var respLAN, respWAN *serf.KeyResponse
	var err error

	if reply.Messages == nil {
		reply.Messages = make(map[string]string)
	}
	if reply.Keys == nil {
		reply.Keys = make(map[string]int)
	}

	m.srv.setQueryMeta(&reply.QueryMeta)

	// Do a LAN key install. This will be invoked in each DC once the RPC call
	// is forwarded below.
	respLAN, err = m.srv.KeyManagerLAN().RemoveKey(args.Key)
	for node, msg := range respLAN.Messages {
		reply.Messages["client."+node+"."+m.srv.config.Datacenter] = msg
	}
	reply.NumResp += respLAN.NumResp
	reply.NumErr += respLAN.NumErr
	reply.NumNodes += respLAN.NumNodes
	if err != nil {
		return fmt.Errorf("failed rotating LAN keyring in %s: %s",
			m.srv.config.Datacenter,
			err)
	}

	if !args.Forwarded {
		// Only perform WAN key rotation once.
		respWAN, err = m.srv.KeyManagerWAN().RemoveKey(args.Key)
		if err != nil {
			return err
		}
		for node, msg := range respWAN.Messages {
			reply.Messages["server."+node] = msg
		}
		reply.NumResp += respWAN.NumResp
		reply.NumErr += respWAN.NumErr
		reply.NumNodes += respWAN.NumNodes

		// Mark key rotation as being already forwarded, then forward.
		args.Forwarded = true
		return m.srv.forwardAll("Internal.RemoveKey", args, reply)
	}

	return nil
}

func (m *Internal) ListKeys(args *structs.KeyringRequest,
	reply *structs.KeyringResponse) error {
	var respLAN, respWAN *serf.KeyResponse
	var err error

	if reply.Messages == nil {
		reply.Messages = make(map[string]string)
	}
	if reply.Keys == nil {
		reply.Keys = make(map[string]int)
	}

	m.srv.setQueryMeta(&reply.QueryMeta)

	// Do a LAN key install. This will be invoked in each DC once the RPC call
	// is forwarded below.
	respLAN, err = m.srv.KeyManagerLAN().ListKeys()
	for node, msg := range respLAN.Messages {
		reply.Messages["client."+node+"."+m.srv.config.Datacenter] = msg
	}
	reply.NumResp += respLAN.NumResp
	reply.NumErr += respLAN.NumErr
	reply.NumNodes += respLAN.NumNodes
	if err != nil {
		return fmt.Errorf("failed rotating LAN keyring in %s: %s",
			m.srv.config.Datacenter,
			err)
	}

	if !args.Forwarded {
		// Only perform WAN key rotation once.
		respWAN, err = m.srv.KeyManagerWAN().ListKeys()
		if err != nil {
			return err
		}
		for node, msg := range respWAN.Messages {
			reply.Messages["server."+node] = msg
		}
		reply.NumResp += respWAN.NumResp
		reply.NumErr += respWAN.NumErr
		reply.NumNodes += respWAN.NumNodes

		// Mark key rotation as being already forwarded, then forward.
		args.Forwarded = true
		return m.srv.forwardAll("Internal.ListKeys", args, reply)
	}

	return nil
}
