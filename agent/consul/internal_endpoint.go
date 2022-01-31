package consul

import (
	"fmt"

	bexpr "github.com/hashicorp/go-bexpr"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/serf/serf"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
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
	if done, err := m.srv.ForwardRPC("Internal.NodeInfo", args, reply); done {
		return err
	}

	_, err := m.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, nil)
	if err != nil {
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
	if done, err := m.srv.ForwardRPC("Internal.NodeDump", args, reply); done {
		return err
	}

	_, err := m.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, nil)
	if err != nil {
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

			raw, err := filter.Execute(reply.Dump)
			if err != nil {
				return err
			}
			reply.Dump = raw.(structs.NodeDump)

			// Note: we filter the results with ACLs *after* applying the user-supplied
			// bexpr filter, to ensure QueryMeta.ResultsFilteredByACLs does not include
			// results that would be filtered out even if the user did have permission.
			if err := m.srv.filterACL(args.Token, reply); err != nil {
				return err
			}

			return nil
		})
}

func (m *Internal) ServiceDump(args *structs.ServiceDumpRequest, reply *structs.IndexedNodesWithGateways) error {
	if done, err := m.srv.ForwardRPC("Internal.ServiceDump", args, reply); done {
		return err
	}

	_, err := m.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, nil)
	if err != nil {
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
			// Get, store, and filter nodes
			maxIdx, nodes, err := state.ServiceDump(ws, args.ServiceKind, args.UseServiceKind, &args.EnterpriseMeta)
			if err != nil {
				return err
			}
			reply.Nodes = nodes

			// Get, store, and filter gateway services
			idx, gatewayServices, err := state.DumpGatewayServices(ws)
			if err != nil {
				return err
			}
			reply.Gateways = gatewayServices

			if idx > maxIdx {
				maxIdx = idx
			}
			reply.Index = maxIdx

			raw, err := filter.Execute(reply.Nodes)
			if err != nil {
				return err
			}
			reply.Nodes = raw.(structs.CheckServiceNodes)

			// Note: we filter the results with ACLs *after* applying the user-supplied
			// bexpr filter, to ensure QueryMeta.ResultsFilteredByACLs does not include
			// results that would be filtered out even if the user did have permission.
			if err := m.srv.filterACL(args.Token, reply); err != nil {
				return err
			}

			return nil
		})
}

func (m *Internal) ServiceTopology(args *structs.ServiceSpecificRequest, reply *structs.IndexedServiceTopology) error {
	if done, err := m.srv.ForwardRPC("Internal.ServiceTopology", args, reply); done {
		return err
	}
	if args.ServiceName == "" {
		return fmt.Errorf("Must provide a service name")
	}

	var authzContext acl.AuthorizerContext
	authz, err := m.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, &authzContext)
	if err != nil {
		return err
	}
	if err := m.srv.validateEnterpriseRequest(&args.EnterpriseMeta, false); err != nil {
		return err
	}
	if authz.ServiceRead(args.ServiceName, &authzContext) != acl.Allow {
		return acl.ErrPermissionDenied
	}

	return m.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			defaultAllow := authz.IntentionDefaultAllow(nil)

			index, topology, err := state.ServiceTopology(ws, args.Datacenter, args.ServiceName, args.ServiceKind, defaultAllow, &args.EnterpriseMeta)
			if err != nil {
				return err
			}

			reply.Index = index
			reply.ServiceTopology = topology

			if err := m.srv.filterACL(args.Token, reply); err != nil {
				return err
			}
			return nil
		})
}

// IntentionUpstreams returns the upstreams of a service. Upstreams are inferred from intentions.
// If intentions allow a connection from the target to some candidate service, the candidate service is considered
// an upstream of the target.
func (m *Internal) IntentionUpstreams(args *structs.ServiceSpecificRequest, reply *structs.IndexedServiceList) error {
	// Exit early if Connect hasn't been enabled.
	if !m.srv.config.ConnectEnabled {
		return ErrConnectNotEnabled
	}
	if args.ServiceName == "" {
		return fmt.Errorf("Must provide a service name")
	}
	if done, err := m.srv.ForwardRPC("Internal.IntentionUpstreams", args, reply); done {
		return err
	}

	authz, err := m.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, nil)
	if err != nil {
		return err
	}
	if err := m.srv.validateEnterpriseRequest(&args.EnterpriseMeta, false); err != nil {
		return err
	}

	return m.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			defaultDecision := authz.IntentionDefaultAllow(nil)

			sn := structs.NewServiceName(args.ServiceName, &args.EnterpriseMeta)
			index, services, err := state.IntentionTopology(ws, sn, false, defaultDecision)
			if err != nil {
				return err
			}

			reply.Index, reply.Services = index, services
			m.srv.filterACLWithAuthorizer(authz, reply)
			return nil
		})
}

// GatewayServiceNodes returns all the nodes for services associated with a gateway along with their gateway config
func (m *Internal) GatewayServiceDump(args *structs.ServiceSpecificRequest, reply *structs.IndexedServiceDump) error {
	if done, err := m.srv.ForwardRPC("Internal.GatewayServiceDump", args, reply); done {
		return err
	}

	// Verify the arguments
	if args.ServiceName == "" {
		return fmt.Errorf("Must provide gateway name")
	}

	var authzContext acl.AuthorizerContext
	authz, err := m.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, &authzContext)
	if err != nil {
		return err
	}

	if err := m.srv.validateEnterpriseRequest(&args.EnterpriseMeta, false); err != nil {
		return err
	}

	// We need read access to the gateway we're trying to find services for, so check that first.
	if authz.ServiceRead(args.ServiceName, &authzContext) != acl.Allow {
		return acl.ErrPermissionDenied
	}

	err = m.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			var maxIdx uint64
			idx, gatewayServices, err := state.GatewayServices(ws, args.ServiceName, &args.EnterpriseMeta)
			if err != nil {
				return err
			}
			if idx > maxIdx {
				maxIdx = idx
			}

			// Loop over the gateway <-> serviceName mappings and fetch all service instances for each
			var result structs.ServiceDump
			for _, gs := range gatewayServices {
				idx, instances, err := state.CheckServiceNodes(ws, gs.Service.Name, &gs.Service.EnterpriseMeta)
				if err != nil {
					return err
				}
				if idx > maxIdx {
					maxIdx = idx
				}
				for _, n := range instances {
					svc := structs.ServiceInfo{
						Node:           n.Node,
						Service:        n.Service,
						Checks:         n.Checks,
						GatewayService: gs,
					}
					result = append(result, &svc)
				}

				// Ensure we store the gateway <-> service mapping even if there are no instances of the service
				if len(instances) == 0 {
					svc := structs.ServiceInfo{
						GatewayService: gs,
					}
					result = append(result, &svc)
				}
			}
			reply.Index, reply.Dump = maxIdx, result

			if err := m.srv.filterACL(args.Token, reply); err != nil {
				return err
			}
			return nil
		})

	return err
}

// Match returns the set of intentions that match the given source/destination.
func (m *Internal) GatewayIntentions(args *structs.IntentionQueryRequest, reply *structs.IndexedIntentions) error {
	// Forward if necessary
	if done, err := m.srv.ForwardRPC("Internal.GatewayIntentions", args, reply); done {
		return err
	}

	if len(args.Match.Entries) > 1 {
		return fmt.Errorf("Expected 1 gateway name, got %d", len(args.Match.Entries))
	}

	// Get the ACL token for the request for the checks below.
	var entMeta structs.EnterpriseMeta
	var authzContext acl.AuthorizerContext

	authz, err := m.srv.ResolveTokenAndDefaultMeta(args.Token, &entMeta, &authzContext)
	if err != nil {
		return err
	}

	if args.Match.Entries[0].Namespace == "" {
		args.Match.Entries[0].Namespace = entMeta.NamespaceOrDefault()
	}
	if err := m.srv.validateEnterpriseIntentionNamespace(args.Match.Entries[0].Namespace, true); err != nil {
		return fmt.Errorf("Invalid match entry namespace %q: %v", args.Match.Entries[0].Namespace, err)
	}

	// We need read access to the gateway we're trying to find intentions for, so check that first.
	if authz.ServiceRead(args.Match.Entries[0].Name, &authzContext) != acl.Allow {
		return acl.ErrPermissionDenied
	}

	return m.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			var maxIdx uint64
			idx, gatewayServices, err := state.GatewayServices(ws, args.Match.Entries[0].Name, &entMeta)
			if err != nil {
				return err
			}
			if idx > maxIdx {
				maxIdx = idx
			}

			// Loop over the gateway <-> serviceName mappings and fetch all intentions for each
			seen := make(map[string]bool)
			result := make(structs.Intentions, 0)

			for _, gs := range gatewayServices {
				entry := structs.IntentionMatchEntry{
					Namespace: gs.Service.NamespaceOrDefault(),
					Partition: gs.Service.PartitionOrDefault(),
					Name:      gs.Service.Name,
				}
				idx, intentions, err := state.IntentionMatchOne(ws, entry, structs.IntentionMatchDestination)
				if err != nil {
					return err
				}
				if idx > maxIdx {
					maxIdx = idx
				}

				// Deduplicate wildcard intentions
				for _, ixn := range intentions {
					if !seen[ixn.ID] {
						result = append(result, ixn)
						seen[ixn.ID] = true
					}
				}
			}

			reply.Index, reply.Intentions = maxIdx, result
			if reply.Intentions == nil {
				reply.Intentions = make(structs.Intentions, 0)
			}

			if err := m.srv.filterACL(args.Token, reply); err != nil {
				return err
			}
			return nil
		},
	)
}

// EventFire is a bit of an odd endpoint, but it allows for a cross-DC RPC
// call to fire an event. The primary use case is to enable user events being
// triggered in a remote DC.
func (m *Internal) EventFire(args *structs.EventFireRequest,
	reply *structs.EventFireResponse) error {
	if done, err := m.srv.ForwardRPC("Internal.EventFire", args, reply); done {
		return err
	}

	// Check ACLs
	authz, err := m.srv.ResolveToken(args.Token)
	if err != nil {
		return err
	}

	if authz.EventWrite(args.Name, nil) != acl.Allow {
		accessorID := m.aclAccessorID(args.Token)
		m.logger.Warn("user event blocked by ACLs", "event", args.Name, "accessorID", accessorID)
		return acl.ErrPermissionDenied
	}

	// Set the query meta data
	m.srv.setQueryMeta(&reply.QueryMeta, args.Token)

	// Add the consul prefix to the event name
	eventName := userEventName(args.Name)

	// Fire the event on all LAN segments
	return m.srv.LANSendUserEvent(eventName, args.Payload, false)
}

// KeyringOperation will query the WAN and LAN gossip keyrings of all nodes.
func (m *Internal) KeyringOperation(
	args *structs.KeyringRequest,
	reply *structs.KeyringResponses) error {

	// Error aggressively to be clear about LocalOnly behavior
	if args.LocalOnly && args.Operation != structs.KeyringList {
		return fmt.Errorf("argument error: LocalOnly can only be used for List operations")
	}

	// Check ACLs
	identity, authz, err := m.srv.ACLResolver.ResolveTokenToIdentityAndAuthorizer(args.Token)
	if err != nil {
		return err
	}
	if err := m.srv.validateEnterpriseToken(identity); err != nil {
		return err
	}
	switch args.Operation {
	case structs.KeyringList:
		if authz.KeyringRead(nil) != acl.Allow {
			return fmt.Errorf("Reading keyring denied by ACLs")
		}
	case structs.KeyringInstall:
		fallthrough
	case structs.KeyringUse:
		fallthrough
	case structs.KeyringRemove:
		if authz.KeyringWrite(nil) != acl.Allow {
			return fmt.Errorf("Modifying keyring denied due to ACLs")
		}
	default:
		panic("Invalid keyring operation")
	}

	if args.LocalOnly || args.Forwarded || m.srv.serfWAN == nil {
		// Handle operations that are localOnly, already forwarded or
		// there is no serfWAN. If any of this is the case this
		// operation shouldn't go out to other dcs or WAN pool.
		reply.Responses = append(reply.Responses, m.executeKeyringOpLAN(args)...)
	} else {
		// Handle not already forwarded, non-local operations.

		// Marking this as forwarded because this is what we are about
		// to do. Prevents the same message from being fowarded by
		// other servers.
		args.Forwarded = true
		reply.Responses = append(reply.Responses, m.executeKeyringOpWAN(args))
		reply.Responses = append(reply.Responses, m.executeKeyringOpLAN(args)...)

		dcs := m.srv.router.GetRemoteDatacenters(m.srv.config.Datacenter)
		responses, err := m.srv.keyringRPCs("Internal.KeyringOperation", args, dcs)
		if err != nil {
			return err
		}
		reply.Add(responses)
	}
	return nil
}

func (m *Internal) executeKeyringOpLAN(args *structs.KeyringRequest) []*structs.KeyringResponse {
	responses := []*structs.KeyringResponse{}
	_ = m.srv.DoWithLANSerfs(func(poolName, poolKind string, pool *serf.Serf) error {
		mgr := pool.KeyManager()
		serfResp, err := m.executeKeyringOpMgr(mgr, args)
		resp := translateKeyResponseToKeyringResponse(serfResp, m.srv.config.Datacenter, err)
		if poolKind == PoolKindSegment {
			resp.Segment = poolName
		} else {
			resp.Partition = poolName
		}
		responses = append(responses, &resp)
		return nil
	}, nil)
	return responses
}

func (m *Internal) executeKeyringOpWAN(args *structs.KeyringRequest) *structs.KeyringResponse {
	mgr := m.srv.KeyManagerWAN()
	serfResp, err := m.executeKeyringOpMgr(mgr, args)
	resp := translateKeyResponseToKeyringResponse(serfResp, m.srv.config.Datacenter, err)
	resp.WAN = true
	return &resp
}

func translateKeyResponseToKeyringResponse(keyresponse *serf.KeyResponse, datacenter string, err error) structs.KeyringResponse {
	resp := structs.KeyringResponse{
		Datacenter:  datacenter,
		Messages:    keyresponse.Messages,
		Keys:        keyresponse.Keys,
		PrimaryKeys: keyresponse.PrimaryKeys,
		NumNodes:    keyresponse.NumNodes,
	}
	if err != nil {
		resp.Error = err.Error()
	}
	return resp
}

// executeKeyringOpMgr executes the appropriate keyring-related function based on
// the type of keyring operation in the request. It takes the KeyManager as an
// argument, so it can handle any operation for either LAN or WAN pools.
func (m *Internal) executeKeyringOpMgr(
	mgr *serf.KeyManager,
	args *structs.KeyringRequest,
) (*serf.KeyResponse, error) {
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

	return serfResp, err
}

// aclAccessorID is used to convert an ACLToken's secretID to its accessorID for non-
// critical purposes, such as logging. Therefore we interpret all errors as empty-string
// so we can safely log it without handling non-critical errors at the usage site.
func (m *Internal) aclAccessorID(secretID string) string {
	_, ident, err := m.srv.ResolveIdentityFromToken(secretID)
	if acl.IsErrNotFound(err) {
		return ""
	}
	if err != nil {
		m.logger.Debug("non-critical error resolving acl token accessor for logging", "error", err)
		return ""
	}
	if ident == nil {
		return ""
	}
	return ident.ID()
}
