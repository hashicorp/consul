package consul

import (
	"fmt"
	"sort"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/ipaddr"
	"github.com/hashicorp/consul/types"
	bexpr "github.com/hashicorp/go-bexpr"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-uuid"
)

// Catalog endpoint is used to manipulate the service catalog
type Catalog struct {
	srv *Server
}

// nodePreApply does the verification of a node before it is applied to Raft.
func nodePreApply(nodeName, nodeID string) error {
	if nodeName == "" {
		return fmt.Errorf("Must provide node")
	}
	if nodeID != "" {
		if _, err := uuid.ParseUUID(nodeID); err != nil {
			return fmt.Errorf("Bad node ID: %v", err)
		}
	}

	return nil
}

func servicePreApply(service *structs.NodeService, rule acl.Authorizer) error {
	// Validate the service. This is in addition to the below since
	// the above just hasn't been moved over yet. We should move it over
	// in time.
	if err := service.Validate(); err != nil {
		return err
	}

	// If no service id, but service name, use default
	if service.ID == "" && service.Service != "" {
		service.ID = service.Service
	}

	// Verify ServiceName provided if ID.
	if service.ID != "" && service.Service == "" {
		return fmt.Errorf("Must provide service name with ID")
	}

	// Check the service address here and in the agent endpoint
	// since service registration isn't synchronous.
	if ipaddr.IsAny(service.Address) {
		return fmt.Errorf("Invalid service address")
	}

	// Apply the ACL policy if any. The 'consul' service is excluded
	// since it is managed automatically internally (that behavior
	// is going away after version 0.8). We check this same policy
	// later if version 0.8 is enabled, so we can eventually just
	// delete this and do all the ACL checks down there.
	if service.Service != structs.ConsulServiceName {
		if rule != nil && !rule.ServiceWrite(service.Service, nil) {
			return acl.ErrPermissionDenied
		}
	}

	// Proxies must have write permission on their destination
	if service.Kind == structs.ServiceKindConnectProxy {
		if rule != nil && !rule.ServiceWrite(service.Proxy.DestinationServiceName, nil) {
			return acl.ErrPermissionDenied
		}
	}

	return nil
}

// checkPreApply does the verification of a check before it is applied to Raft.
func checkPreApply(check *structs.HealthCheck) {
	if check.CheckID == "" && check.Name != "" {
		check.CheckID = types.CheckID(check.Name)
	}
}

// Register is used register that a node is providing a given service.
func (c *Catalog) Register(args *structs.RegisterRequest, reply *struct{}) error {
	if done, err := c.srv.forward("Catalog.Register", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"catalog", "register"}, time.Now())

	// Fetch the ACL token, if any.
	rule, err := c.srv.ResolveToken(args.Token)
	if err != nil {
		return err
	}

	// Verify the args.
	if err := nodePreApply(args.Node, string(args.ID)); err != nil {
		return err
	}
	if args.Address == "" && !args.SkipNodeUpdate {
		return fmt.Errorf("Must provide address if SkipNodeUpdate is not set")
	}

	// Handle a service registration.
	if args.Service != nil {
		if err := servicePreApply(args.Service, rule); err != nil {
			return err
		}
	}

	// Move the old format single check into the slice, and fixup IDs.
	if args.Check != nil {
		args.Checks = append(args.Checks, args.Check)
		args.Check = nil
	}
	for _, check := range args.Checks {
		if check.Node == "" {
			check.Node = args.Node
		}
		checkPreApply(check)
	}

	// Check the complete register request against the given ACL policy.
	if rule != nil && c.srv.config.ACLEnforceVersion8 {
		state := c.srv.fsm.State()
		_, ns, err := state.NodeServices(nil, args.Node)
		if err != nil {
			return fmt.Errorf("Node lookup failed: %v", err)
		}
		if err := vetRegisterWithACL(rule, args, ns); err != nil {
			return err
		}
	}

	resp, err := c.srv.raftApply(structs.RegisterRequestType, args)
	if err != nil {
		return err
	}
	if respErr, ok := resp.(error); ok {
		return respErr
	}
	return nil
}

// Deregister is used to remove a service registration for a given node.
func (c *Catalog) Deregister(args *structs.DeregisterRequest, reply *struct{}) error {
	if done, err := c.srv.forward("Catalog.Deregister", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"catalog", "deregister"}, time.Now())

	// Verify the args
	if args.Node == "" {
		return fmt.Errorf("Must provide node")
	}

	// Fetch the ACL token, if any.
	rule, err := c.srv.ResolveToken(args.Token)
	if err != nil {
		return err
	}

	// Check the complete deregister request against the given ACL policy.
	if rule != nil && c.srv.config.ACLEnforceVersion8 {
		state := c.srv.fsm.State()

		var ns *structs.NodeService
		if args.ServiceID != "" {
			_, ns, err = state.NodeService(args.Node, args.ServiceID)
			if err != nil {
				return fmt.Errorf("Service lookup failed: %v", err)
			}
		}

		var nc *structs.HealthCheck
		if args.CheckID != "" {
			_, nc, err = state.NodeCheck(args.Node, args.CheckID)
			if err != nil {
				return fmt.Errorf("Check lookup failed: %v", err)
			}
		}

		if err := vetDeregisterWithACL(rule, args, ns, nc); err != nil {
			return err
		}

	}

	if _, err := c.srv.raftApply(structs.DeregisterRequestType, args); err != nil {
		return err
	}
	return nil
}

// ListDatacenters is used to query for the list of known datacenters
func (c *Catalog) ListDatacenters(args *structs.DatacentersRequest, reply *[]string) error {
	dcs, err := c.srv.router.GetDatacentersByDistance()
	if err != nil {
		return err
	}

	if len(dcs) == 0 { // no WAN federation, so return the local data center name
		dcs = []string{c.srv.config.Datacenter}
	}

	*reply = dcs
	return nil
}

// ListNodes is used to query the nodes in a DC
func (c *Catalog) ListNodes(args *structs.DCSpecificRequest, reply *structs.IndexedNodes) error {
	if done, err := c.srv.forward("Catalog.ListNodes", args, args, reply); done {
		return err
	}

	filter, err := bexpr.CreateFilter(args.Filter, nil, reply.Nodes)
	if err != nil {
		return err
	}

	return c.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			var index uint64
			var nodes structs.Nodes
			var err error
			if len(args.NodeMetaFilters) > 0 {
				index, nodes, err = state.NodesByMeta(ws, args.NodeMetaFilters)
			} else {
				index, nodes, err = state.Nodes(ws)
			}
			if err != nil {
				return err
			}

			reply.Index, reply.Nodes = index, nodes
			if err := c.srv.filterACL(args.Token, reply); err != nil {
				return err
			}

			raw, err := filter.Execute(reply.Nodes)
			if err != nil {
				return err
			}
			reply.Nodes = raw.(structs.Nodes)

			return c.srv.sortNodesByDistanceFrom(args.Source, reply.Nodes)
		})
}

// ListServices is used to query the services in a DC
func (c *Catalog) ListServices(args *structs.DCSpecificRequest, reply *structs.IndexedServices) error {
	if done, err := c.srv.forward("Catalog.ListServices", args, args, reply); done {
		return err
	}

	return c.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			var index uint64
			var services structs.Services
			var err error
			if len(args.NodeMetaFilters) > 0 {
				index, services, err = state.ServicesByNodeMeta(ws, args.NodeMetaFilters)
			} else {
				index, services, err = state.Services(ws)
			}
			if err != nil {
				return err
			}

			reply.Index, reply.Services = index, services
			return c.srv.filterACL(args.Token, reply)
		})
}

// ServiceNodes returns all the nodes registered as part of a service
func (c *Catalog) ServiceNodes(args *structs.ServiceSpecificRequest, reply *structs.IndexedServiceNodes) error {
	if done, err := c.srv.forward("Catalog.ServiceNodes", args, args, reply); done {
		return err
	}

	// Verify the arguments
	if args.ServiceName == "" && args.ServiceAddress == "" {
		return fmt.Errorf("Must provide service name")
	}

	// Determine the function we'll call
	var f func(memdb.WatchSet, *state.Store) (uint64, structs.ServiceNodes, error)
	switch {
	case args.Connect:
		f = func(ws memdb.WatchSet, s *state.Store) (uint64, structs.ServiceNodes, error) {
			return s.ConnectServiceNodes(ws, args.ServiceName)
		}

	default:
		f = func(ws memdb.WatchSet, s *state.Store) (uint64, structs.ServiceNodes, error) {
			if args.ServiceAddress != "" {
				return s.ServiceAddressNodes(ws, args.ServiceAddress)
			}

			if args.TagFilter {
				tags := args.ServiceTags
				// DEPRECATED (singular-service-tag) - remove this when backwards RPC compat
				// with 1.2.x is not required.
				// Agents < v1.3.0 populate the ServiceTag field. In this case,
				// use ServiceTag instead of the ServiceTags field.
				if args.ServiceTag != "" {
					tags = []string{args.ServiceTag}
				}

				return s.ServiceTagNodes(ws, args.ServiceName, tags)
			}

			return s.ServiceNodes(ws, args.ServiceName)
		}
	}

	// If we're doing a connect query, we need read access to the service
	// we're trying to find proxies for, so check that.
	if args.Connect {
		// Fetch the ACL token, if any.
		rule, err := c.srv.ResolveToken(args.Token)
		if err != nil {
			return err
		}

		if rule != nil && !rule.ServiceRead(args.ServiceName) {
			// Just return nil, which will return an empty response (tested)
			return nil
		}
	}

	filter, err := bexpr.CreateFilter(args.Filter, nil, reply.ServiceNodes)
	if err != nil {
		return err
	}

	err = c.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, services, err := f(ws, state)
			if err != nil {
				return err
			}

			reply.Index, reply.ServiceNodes = index, services
			if len(args.NodeMetaFilters) > 0 {
				var filtered structs.ServiceNodes
				for _, service := range services {
					if structs.SatisfiesMetaFilters(service.NodeMeta, args.NodeMetaFilters) {
						filtered = append(filtered, service)
					}
				}
				reply.ServiceNodes = filtered
			}

			if err := c.srv.filterACL(args.Token, reply); err != nil {
				return err
			}

			// This is safe to do even when the filter is nil - its just a no-op then
			raw, err := filter.Execute(reply.ServiceNodes)
			if err != nil {
				return err
			}

			reply.ServiceNodes = raw.(structs.ServiceNodes)

			return c.srv.sortNodesByDistanceFrom(args.Source, reply.ServiceNodes)
		})

	// Provide some metrics
	if err == nil {
		// For metrics, we separate Connect-based lookups from non-Connect
		key := "service"
		if args.Connect {
			key = "connect"
		}

		metrics.IncrCounterWithLabels([]string{"catalog", key, "query"}, 1,
			[]metrics.Label{{Name: "service", Value: args.ServiceName}})
		// DEPRECATED (singular-service-tag) - remove this when backwards RPC compat
		// with 1.2.x is not required.
		if args.ServiceTag != "" {
			metrics.IncrCounterWithLabels([]string{"catalog", key, "query-tag"}, 1,
				[]metrics.Label{{Name: "service", Value: args.ServiceName}, {Name: "tag", Value: args.ServiceTag}})
		}
		if len(args.ServiceTags) > 0 {
			// Sort tags so that the metric is the same even if the request
			// tags are in a different order
			sort.Strings(args.ServiceTags)

			// Build metric labels
			labels := []metrics.Label{{Name: "service", Value: args.ServiceName}}
			for _, tag := range args.ServiceTags {
				labels = append(labels, metrics.Label{Name: "tag", Value: tag})
			}
			metrics.IncrCounterWithLabels([]string{"catalog", key, "query-tags"}, 1, labels)
		}
		if len(reply.ServiceNodes) == 0 {
			metrics.IncrCounterWithLabels([]string{"catalog", key, "not-found"}, 1,
				[]metrics.Label{{Name: "service", Value: args.ServiceName}})
		}
	}

	return err
}

// NodeServices returns all the services registered as part of a node
func (c *Catalog) NodeServices(args *structs.NodeSpecificRequest, reply *structs.IndexedNodeServices) error {
	if done, err := c.srv.forward("Catalog.NodeServices", args, args, reply); done {
		return err
	}

	// Verify the arguments
	if args.Node == "" {
		return fmt.Errorf("Must provide node")
	}

	var filterType map[string]*structs.NodeService
	filter, err := bexpr.CreateFilter(args.Filter, nil, filterType)
	if err != nil {
		return err
	}

	return c.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, services, err := state.NodeServices(ws, args.Node)
			if err != nil {
				return err
			}

			reply.Index, reply.NodeServices = index, services
			if err := c.srv.filterACL(args.Token, reply); err != nil {
				return err
			}

			if reply.NodeServices != nil {
				raw, err := filter.Execute(reply.NodeServices.Services)
				if err != nil {
					return err
				}
				reply.NodeServices.Services = raw.(map[string]*structs.NodeService)
			}

			return nil
		})
}
