package consul

import (
	"fmt"
	"sort"
	"time"

	"github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"
	bexpr "github.com/hashicorp/go-bexpr"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-uuid"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/ipaddr"
	"github.com/hashicorp/consul/types"
)

var CatalogCounters = []prometheus.CounterDefinition{
	{
		Name: []string{"catalog", "service", "query"},
		Help: "Increments for each catalog query for the given service.",
	},
	{
		Name: []string{"catalog", "connect", "query"},
		Help: "Increments for each connect-based catalog query for the given service.",
	},
	{
		Name: []string{"catalog", "service", "query-tag"},
		Help: "Increments for each catalog query for the given service with the given tag.",
	},
	{
		Name: []string{"catalog", "connect", "query-tag"},
		Help: "Increments for each connect-based catalog query for the given service with the given tag.",
	},
	{
		Name: []string{"catalog", "service", "query-tags"},
		Help: "Increments for each catalog query for the given service with the given tags.",
	},
	{
		Name: []string{"catalog", "connect", "query-tags"},
		Help: "Increments for each connect-based catalog query for the given service with the given tags.",
	},
	{
		Name: []string{"catalog", "service", "not-found"},
		Help: "Increments for each catalog query where the given service could not be found.",
	},
	{
		Name: []string{"catalog", "connect", "not-found"},
		Help: "Increments for each connect-based catalog query where the given service could not be found.",
	},
}

var CatalogSummaries = []prometheus.SummaryDefinition{
	{
		Name: []string{"catalog", "deregister"},
		Help: "Measures the time it takes to complete a catalog deregister operation.",
	},
	{
		Name: []string{"catalog", "register"},
		Help: "Measures the time it takes to complete a catalog register operation.",
	},
}

// Catalog endpoint is used to manipulate the service catalog
type Catalog struct {
	srv    *Server
	logger hclog.Logger
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

func servicePreApply(service *structs.NodeService, authz acl.Authorizer) error {
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

	var authzContext acl.AuthorizerContext
	service.FillAuthzContext(&authzContext)

	// Apply the ACL policy if any. The 'consul' service is excluded
	// since it is managed automatically internally (that behavior
	// is going away after version 0.8). We check this same policy
	// later if version 0.8 is enabled, so we can eventually just
	// delete this and do all the ACL checks down there.
	if service.Service != structs.ConsulServiceName {
		if authz != nil && authz.ServiceWrite(service.Service, &authzContext) != acl.Allow {
			return acl.ErrPermissionDenied
		}
	}

	// Proxies must have write permission on their destination
	if service.Kind == structs.ServiceKindConnectProxy {
		if authz != nil && authz.ServiceWrite(service.Proxy.DestinationServiceName, &authzContext) != acl.Allow {
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
	if done, err := c.srv.ForwardRPC("Catalog.Register", args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"catalog", "register"}, time.Now())

	// Fetch the ACL token, if any.
	authz, err := c.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, nil)
	if err != nil {
		return err
	}

	if err := c.srv.validateEnterpriseRequest(args.GetEnterpriseMeta(), true); err != nil {
		return err
	}

	// This needs to happen before the other preapply checks as it will fixup some of the
	// internal enterprise metas on the services and checks
	state := c.srv.fsm.State()
	entMeta, err := state.ValidateRegisterRequest(args)
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
		if err := servicePreApply(args.Service, authz); err != nil {
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

		// Populate check type for cases when a check is registered in the catalog directly
		// and not via anti-entropy
		if check.Type == "" {
			chkType := check.CheckType()
			check.Type = chkType.Type()
		}
	}

	// Check the complete register request against the given ACL policy.
	if authz != nil {
		state := c.srv.fsm.State()
		_, ns, err := state.NodeServices(nil, args.Node, entMeta)
		if err != nil {
			return fmt.Errorf("Node lookup failed: %v", err)
		}
		if err := vetRegisterWithACL(authz, args, ns); err != nil {
			return err
		}
	}

	_, err = c.srv.raftApply(structs.RegisterRequestType, args)
	return err
}

// Deregister is used to remove a service registration for a given node.
func (c *Catalog) Deregister(args *structs.DeregisterRequest, reply *struct{}) error {
	if done, err := c.srv.ForwardRPC("Catalog.Deregister", args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"catalog", "deregister"}, time.Now())

	// Verify the args
	if args.Node == "" {
		return fmt.Errorf("Must provide node")
	}

	// Fetch the ACL token, if any.
	authz, err := c.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, nil)
	if err != nil {
		return err
	}

	if err := c.srv.validateEnterpriseRequest(&args.EnterpriseMeta, true); err != nil {
		return err
	}

	// Check the complete deregister request against the given ACL policy.
	if authz != nil {
		state := c.srv.fsm.State()

		var ns *structs.NodeService
		if args.ServiceID != "" {
			_, ns, err = state.NodeService(args.Node, args.ServiceID, &args.EnterpriseMeta)
			if err != nil {
				return fmt.Errorf("Service lookup failed: %v", err)
			}
		}

		var nc *structs.HealthCheck
		if args.CheckID != "" {
			_, nc, err = state.NodeCheck(args.Node, args.CheckID, &args.EnterpriseMeta)
			if err != nil {
				return fmt.Errorf("Check lookup failed: %v", err)
			}
		}

		if err := vetDeregisterWithACL(authz, args, ns, nc); err != nil {
			return err
		}

	}

	_, err = c.srv.raftApply(structs.DeregisterRequestType, args)
	return err
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
	if done, err := c.srv.ForwardRPC("Catalog.ListNodes", args, reply); done {
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
			var err error
			if len(args.NodeMetaFilters) > 0 {
				reply.Index, reply.Nodes, err = state.NodesByMeta(ws, args.NodeMetaFilters)
			} else {
				reply.Index, reply.Nodes, err = state.Nodes(ws)
			}
			if err != nil {
				return err
			}
			if isUnmodified(args.QueryOptions, reply.Index) {
				reply.QueryMeta.NotModified = true
				reply.Nodes = nil
				return nil
			}

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

func isUnmodified(opts structs.QueryOptions, index uint64) bool {
	return opts.AllowNotModifiedResponse && opts.MinQueryIndex > 0 && opts.MinQueryIndex == index
}

// ListServices is used to query the services in a DC
func (c *Catalog) ListServices(args *structs.DCSpecificRequest, reply *structs.IndexedServices) error {
	if done, err := c.srv.ForwardRPC("Catalog.ListServices", args, reply); done {
		return err
	}

	authz, err := c.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, nil)
	if err != nil {
		return err
	}

	if err := c.srv.validateEnterpriseRequest(&args.EnterpriseMeta, false); err != nil {
		return err
	}

	// Set reply enterprise metadata after resolving and validating the token so
	// that we can properly infer metadata from the token.
	reply.EnterpriseMeta = args.EnterpriseMeta

	return c.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			var err error
			if len(args.NodeMetaFilters) > 0 {
				reply.Index, reply.Services, err = state.ServicesByNodeMeta(ws, args.NodeMetaFilters, &args.EnterpriseMeta)
			} else {
				reply.Index, reply.Services, err = state.Services(ws, &args.EnterpriseMeta)
			}
			if err != nil {
				return err
			}
			if isUnmodified(args.QueryOptions, reply.Index) {
				reply.Services = nil
				reply.QueryMeta.NotModified = true
				return nil
			}

			return c.srv.filterACLWithAuthorizer(authz, reply)
		})
}

func (c *Catalog) ServiceList(args *structs.DCSpecificRequest, reply *structs.IndexedServiceList) error {
	if done, err := c.srv.ForwardRPC("Catalog.ServiceList", args, reply); done {
		return err
	}

	authz, err := c.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, nil)
	if err != nil {
		return err
	}

	if err := c.srv.validateEnterpriseRequest(&args.EnterpriseMeta, false); err != nil {
		return err
	}

	return c.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, services, err := state.ServiceList(ws, nil, &args.EnterpriseMeta)
			if err != nil {
				return err
			}

			reply.Index, reply.Services = index, services
			return c.srv.filterACLWithAuthorizer(authz, reply)
		})
}

// ServiceNodes returns all the nodes registered as part of a service
func (c *Catalog) ServiceNodes(args *structs.ServiceSpecificRequest, reply *structs.IndexedServiceNodes) error {
	if done, err := c.srv.ForwardRPC("Catalog.ServiceNodes", args, reply); done {
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
			return s.ConnectServiceNodes(ws, args.ServiceName, &args.EnterpriseMeta)
		}

	default:
		f = func(ws memdb.WatchSet, s *state.Store) (uint64, structs.ServiceNodes, error) {
			if args.ServiceAddress != "" {
				return s.ServiceAddressNodes(ws, args.ServiceAddress, &args.EnterpriseMeta)
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

				return s.ServiceTagNodes(ws, args.ServiceName, tags, &args.EnterpriseMeta)
			}

			return s.ServiceNodes(ws, args.ServiceName, &args.EnterpriseMeta)
		}
	}

	var authzContext acl.AuthorizerContext
	authz, err := c.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, &authzContext)
	if err != nil {
		return err
	}

	if err := c.srv.validateEnterpriseRequest(&args.EnterpriseMeta, false); err != nil {
		return err
	}

	// If we're doing a connect query, we need read access to the service
	// we're trying to find proxies for, so check that.
	if args.Connect {
		if authz != nil && authz.ServiceRead(args.ServiceName, &authzContext) != acl.Allow {
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
	if done, err := c.srv.ForwardRPC("Catalog.NodeServices", args, reply); done {
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

	_, err = c.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, nil)
	if err != nil {
		return err
	}

	if err := c.srv.validateEnterpriseRequest(&args.EnterpriseMeta, false); err != nil {
		return err
	}

	return c.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, services, err := state.NodeServices(ws, args.Node, &args.EnterpriseMeta)
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

func (c *Catalog) NodeServiceList(args *structs.NodeSpecificRequest, reply *structs.IndexedNodeServiceList) error {
	if done, err := c.srv.ForwardRPC("Catalog.NodeServiceList", args, reply); done {
		return err
	}

	// Verify the arguments
	if args.Node == "" {
		return fmt.Errorf("Must provide node")
	}

	var filterType []*structs.NodeService
	filter, err := bexpr.CreateFilter(args.Filter, nil, filterType)
	if err != nil {
		return err
	}

	_, err = c.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, nil)
	if err != nil {
		return err
	}

	if err := c.srv.validateEnterpriseRequest(&args.EnterpriseMeta, false); err != nil {
		return err
	}

	return c.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, services, err := state.NodeServiceList(ws, args.Node, &args.EnterpriseMeta)
			if err != nil {
				return err
			}

			if err := c.srv.filterACL(args.Token, &services); err != nil {
				return err
			}

			reply.Index = index
			if services != nil {
				reply.NodeServices = *services

				raw, err := filter.Execute(reply.NodeServices.Services)
				if err != nil {
					return err
				}
				reply.NodeServices.Services = raw.([]*structs.NodeService)
			}

			return nil
		})
}

func (c *Catalog) GatewayServices(args *structs.ServiceSpecificRequest, reply *structs.IndexedGatewayServices) error {
	if done, err := c.srv.ForwardRPC("Catalog.GatewayServices", args, reply); done {
		return err
	}

	var authzContext acl.AuthorizerContext
	authz, err := c.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, &authzContext)
	if err != nil {
		return err
	}

	if err := c.srv.validateEnterpriseRequest(&args.EnterpriseMeta, false); err != nil {
		return err
	}

	if authz != nil && authz.ServiceRead(args.ServiceName, &authzContext) != acl.Allow {
		return acl.ErrPermissionDenied
	}

	return c.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			var index uint64
			var services structs.GatewayServices

			supportedGateways := []string{structs.IngressGateway, structs.TerminatingGateway}
			var found bool
			for _, kind := range supportedGateways {
				// We only use this call to validate the RPC call, don't add the watch set
				_, entry, err := state.ConfigEntry(nil, kind, args.ServiceName, &args.EnterpriseMeta)
				if err != nil {
					return err
				}
				if entry != nil {
					found = true
					break
				}
			}

			// We log a warning here to indicate that there is a potential
			// misconfiguration. We explicitly do NOT return an error because this
			// can occur in the course of normal operation by deleting a
			// configuration entry or starting the proxy before registering the
			// config entry.
			if !found {
				c.logger.Warn("no terminating-gateway or ingress-gateway associated with this gateway",
					"gateway", args.ServiceName,
				)
			}

			index, services, err = state.GatewayServices(ws, args.ServiceName, &args.EnterpriseMeta)
			if err != nil {
				return err
			}

			if err := c.srv.filterACL(args.Token, &services); err != nil {
				return err
			}

			reply.Index, reply.Services = index, services
			return nil
		})
}
