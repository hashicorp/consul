package consul

import (
	"fmt"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	bexpr "github.com/hashicorp/go-bexpr"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/serf/serf"
)

// Internal endpoint is used to query the miscellaneous info that
// does not necessarily fit into the other systems. It is also
// used to hold undocumented APIs that users should not rely on.
type Internal struct {
	srv    *Server
	logger hclog.Logger
}

// NodeInfo is used to retrieve information about a specific node.
func (m *Internal) NodeInfo(args *structs.NodeSpecificRequest,
	reply *structs.IndexedNodeDump) error {
	if done, err := m.srv.forward("Internal.NodeInfo", args, args, reply); done {
		return err
	}

	return m.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, dump, err := state.NodeInfo(ws, args.Node, &args.EnterpriseMeta)
			if err != nil {
				return err
			}

			reply.Index, reply.Dump = index, dump
			return m.srv.filterACL(args.Token, reply)
		})
}

// NodeDump is used to generate information about all of the nodes.
func (m *Internal) NodeDump(args *structs.DCSpecificRequest,
	reply *structs.IndexedNodeDump) error {
	if done, err := m.srv.forward("Internal.NodeDump", args, args, reply); done {
		return err
	}

	filter, err := bexpr.CreateFilter(args.Filter, nil, reply.Dump)
	if err != nil {
		return err
	}

	return m.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, dump, err := state.NodeDump(ws, &args.EnterpriseMeta)
			if err != nil {
				return err
			}

			reply.Index, reply.Dump = index, dump
			if err := m.srv.filterACL(args.Token, reply); err != nil {
				return err
			}

			raw, err := filter.Execute(reply.Dump)
			if err != nil {
				return err
			}

			reply.Dump = raw.(structs.NodeDump)
			return nil
		})
}

func (m *Internal) ServiceDump(args *structs.ServiceDumpRequest, reply *structs.IndexedCheckServiceNodes) error {
	if done, err := m.srv.forward("Internal.ServiceDump", args, args, reply); done {
		return err
	}

	filter, err := bexpr.CreateFilter(args.Filter, nil, reply.Nodes)
	if err != nil {
		return err
	}

	return m.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, nodes, err := state.ServiceDump(ws, args.ServiceKind, args.UseServiceKind, &args.EnterpriseMeta)
			if err != nil {
				return err
			}

			reply.Index, reply.Nodes = index, nodes
			if err := m.srv.filterACL(args.Token, reply); err != nil {
				return err
			}

			raw, err := filter.Execute(reply.Nodes)
			if err != nil {
				return err
			}

			reply.Nodes = raw.(structs.CheckServiceNodes)
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

	// Check ACLs
	rule, err := m.srv.ResolveToken(args.Token)
	if err != nil {
		return err
	}

	if rule != nil && rule.EventWrite(args.Name, nil) != acl.Allow {
		m.logger.Warn("user event blocked by ACLs", "event", args.Name)
		return acl.ErrPermissionDenied
	}

	// Set the query meta data
	m.srv.setQueryMeta(&reply.QueryMeta)

	// Add the consul prefix to the event name
	eventName := userEventName(args.Name)

	// Fire the event on all LAN segments
	segments := m.srv.LANSegments()
	var errs error
	for name, segment := range segments {
		err := segment.UserEvent(eventName, args.Payload, false)
		if err != nil {
			err = fmt.Errorf("error broadcasting event to segment %q: %v", name, err)
			errs = multierror.Append(errs, err)
		}
	}
	return errs
}

// KeyringOperation will query the WAN and LAN gossip keyrings of all nodes.
func (m *Internal) KeyringOperation(
	args *structs.KeyringRequest,
	reply *structs.KeyringResponses) error {

	// Check ACLs
	rule, err := m.srv.ResolveToken(args.Token)
	if err != nil {
		return err
	}
	if rule != nil {
		switch args.Operation {
		case structs.KeyringList:
			if rule.KeyringRead(nil) != acl.Allow {
				return fmt.Errorf("Reading keyring denied by ACLs")
			}
		case structs.KeyringInstall:
			fallthrough
		case structs.KeyringUse:
			fallthrough
		case structs.KeyringRemove:
			if rule.KeyringWrite(nil) != acl.Allow {
				return fmt.Errorf("Modifying keyring denied due to ACLs")
			}
		default:
			panic("Invalid keyring operation")
		}
	}

	// Validate use of local-only
	if args.LocalOnly && args.Operation != structs.KeyringList {
		// Error aggressively to be clear about LocalOnly behavior
		return fmt.Errorf("argument error: LocalOnly can only be used for List operations")
	}

	// args.LocalOnly should always be false for non-GET requests
	if !args.LocalOnly {
		// Only perform WAN keyring querying and RPC forwarding once
		if !args.Forwarded && m.srv.serfWAN != nil {
			args.Forwarded = true
			m.executeKeyringOp(args, reply, true)
			return m.srv.globalRPC("Internal.KeyringOperation", args, reply)
		}
	}

	// Query the LAN keyring of this node's DC
	m.executeKeyringOp(args, reply, false)
	return nil
}

// executeKeyringOp executes the keyring-related operation in the request
// on either the WAN or LAN pools.
func (m *Internal) executeKeyringOp(
	args *structs.KeyringRequest,
	reply *structs.KeyringResponses,
	wan bool) {

	if wan {
		mgr := m.srv.KeyManagerWAN()
		m.executeKeyringOpMgr(mgr, args, reply, wan, "")
	} else {
		segments := m.srv.LANSegments()
		for name, segment := range segments {
			mgr := segment.KeyManager()
			m.executeKeyringOpMgr(mgr, args, reply, wan, name)
		}
	}
}

// executeKeyringOpMgr executes the appropriate keyring-related function based on
// the type of keyring operation in the request. It takes the KeyManager as an
// argument, so it can handle any operation for either LAN or WAN pools.
func (m *Internal) executeKeyringOpMgr(
	mgr *serf.KeyManager,
	args *structs.KeyringRequest,
	reply *structs.KeyringResponses,
	wan bool,
	segment string) {
	var serfResp *serf.KeyResponse
	var err error

	opts := &serf.KeyRequestOptions{RelayFactor: args.RelayFactor}
	switch args.Operation {
	case structs.KeyringList:
		serfResp, err = mgr.ListKeysWithOptions(opts)
	case structs.KeyringInstall:
		serfResp, err = mgr.InstallKeyWithOptions(args.Key, opts)
	case structs.KeyringUse:
		serfResp, err = mgr.UseKeyWithOptions(args.Key, opts)
	case structs.KeyringRemove:
		serfResp, err = mgr.RemoveKeyWithOptions(args.Key, opts)
	}

	errStr := ""
	if err != nil {
		errStr = err.Error()
	}

	reply.Responses = append(reply.Responses, &structs.KeyringResponse{
		WAN:        wan,
		Datacenter: m.srv.config.Datacenter,
		Segment:    segment,
		Messages:   serfResp.Messages,
		Keys:       serfResp.Keys,
		NumNodes:   serfResp.NumNodes,
		Error:      errStr,
	})
}
