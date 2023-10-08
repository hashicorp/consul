package consul

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"
	"github.com/hashicorp/go-bexpr"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-uuid"
	hashstructure_v2 "github.com/mitchellh/hashstructure/v2"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/acl/resolver"
	"github.com/hashicorp/consul/agent/configentry"
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

func hasPeerNameInRequest(req *structs.RegisterRequest) bool {
	if req == nil {
		return false
	}
	// nodes, services, checks
	if req.PeerName != structs.DefaultPeerKeyword {
		return true
	}
	if req.Service != nil && req.Service.PeerName != structs.DefaultPeerKeyword {
		return true
	}
	if req.Check != nil && req.Check.PeerName != structs.DefaultPeerKeyword {
		return true
	}
	for _, check := range req.Checks {
		if check.PeerName != structs.DefaultPeerKeyword {
			return true
		}
	}

	return false
}

// Register a service and/or check(s) in a node, creating the node if it doesn't exist.
// It is valid to pass no service or checks to simply create the node itself.
func (c *Catalog) Register(args *structs.RegisterRequest, reply *struct{}) error {
	if !c.srv.config.PeeringTestAllowPeerRegistrations && hasPeerNameInRequest(args) {
		return fmt.Errorf("cannot register requests with PeerName in them")
	}

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
		if err := servicePreApply(args.Service, authz, args.Service.FillAuthzContext); err != nil {
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
	_, ns, err := state.NodeServices(nil, args.Node, entMeta, args.PeerName)
	if err != nil {
		return fmt.Errorf("Node lookup failed: %v", err)
	}
	if err := vetRegisterWithACL(authz, args, ns); err != nil {
		return err
	}

	_, err = c.srv.raftApply(structs.RegisterRequestType, args)
	return err
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

func servicePreApply(service *structs.NodeService, authz resolver.Result, authzCtxFill func(*acl.AuthorizerContext)) error {
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
		return fmt.Errorf("Must provide service name (Service.Service) when service ID is provided")
	}

	// Check the service address here and in the agent endpoint
	// since service registration isn't synchronous.
	if ipaddr.IsAny(service.Address) {
		return fmt.Errorf("Invalid service address")
	}

	var authzContext acl.AuthorizerContext
	authzCtxFill(&authzContext)

	// Apply the ACL policy if any. The 'consul' service is excluded
	// since it is managed automatically internally (that behavior
	// is going away after version 0.8). We check this same policy
	// later if version 0.8 is enabled, so we can eventually just
	// delete this and do all the ACL checks down there.
	if service.Service != structs.ConsulServiceName {
		if err := authz.ToAllowAuthorizer().ServiceWriteAllowed(service.Service, &authzContext); err != nil {
			return err
		}
	}

	// Proxies must have write permission on their destination
	if service.Kind == structs.ServiceKindConnectProxy {
		if err := authz.ToAllowAuthorizer().ServiceWriteAllowed(service.Proxy.DestinationServiceName, &authzContext); err != nil {
			return err
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

// vetRegisterWithACL applies the given ACL's policy to the catalog update and
// determines if it is allowed. Since the catalog register request is so
// dynamic, this is a pretty complex algorithm and was worth breaking out of the
// endpoint. The NodeServices record for the node must be supplied, and can be
// nil.
//
// This is a bit racy because we have to check the state store outside of a
// transaction. It's the best we can do because we don't want to flow ACL
// checking down there. The node information doesn't change in practice, so this
// will be fine. If we expose ways to change node addresses in a later version,
// then we should split the catalog API at the node and service level so we can
// address this race better (even then it would be super rare, and would at
// worst let a service update revert a recent node update, so it doesn't open up
// too much abuse).
func vetRegisterWithACL(
	authz resolver.Result,
	subj *structs.RegisterRequest,
	ns *structs.NodeServices,
) error {
	var authzContext acl.AuthorizerContext
	subj.FillAuthzContext(&authzContext)

	// Vet the node info. This allows service updates to re-post the required
	// node info for each request without having to have node "write"
	// privileges.
	needsNode := ns == nil || subj.ChangesNode(ns.Node)

	if needsNode {
		if err := authz.ToAllowAuthorizer().NodeWriteAllowed(subj.Node, &authzContext); err != nil {
			return err
		}
	}

	// Vet the service change. This includes making sure they can register
	// the given service, and that we can write to any existing service that
	// is being modified by id (if any).
	if subj.Service != nil {
		if err := authz.ToAllowAuthorizer().ServiceWriteAllowed(subj.Service.Service, &authzContext); err != nil {
			return err
		}

		if ns != nil {
			other, ok := ns.Services[subj.Service.ID]

			if ok {
				// This is effectively a delete, so we DO NOT apply the
				// sentinel scope to the service we are overwriting, just
				// the regular ACL policy.
				var secondaryCtx acl.AuthorizerContext
				other.FillAuthzContext(&secondaryCtx)

				if err := authz.ToAllowAuthorizer().ServiceWriteAllowed(other.Service, &secondaryCtx); err != nil {
					return acl.ErrPermissionDenied
				}
			}
		}
	}

	// Make sure that the member was flattened before we got there. This
	// keeps us from having to verify this check as well.
	if subj.Check != nil {
		return fmt.Errorf("check member must be nil")
	}

	// Vet the checks. Node-level checks require node write, and
	// service-level checks require service write.
	for _, check := range subj.Checks {
		// Make sure that the node matches - we don't allow you to mix
		// checks from other nodes because we'd have to pull a bunch
		// more state store data to check this. If ACLs are enabled then
		// we simply require them to match in a given request. There's a
		// note in state_store.go to ban this down there in Consul 0.8,
		// but it's good to leave this here because it's required for
		// correctness wrt. ACLs.
		if !strings.EqualFold(check.Node, subj.Node) {
			return fmt.Errorf("Node '%s' for check '%s' doesn't match register request node '%s'",
				check.Node, check.CheckID, subj.Node)
		}

		// Node-level check.
		if check.ServiceID == "" {
			if err := authz.ToAllowAuthorizer().NodeWriteAllowed(subj.Node, &authzContext); err != nil {
				return err
			}
			continue
		}

		// Service-level check, check the common case where it
		// matches the service part of this request, which has
		// already been vetted above, and might be being registered
		// along with its checks.
		if subj.Service != nil && subj.Service.ID == check.ServiceID {
			continue
		}

		// Service-level check for some other service. Make sure they've
		// got write permissions for that service.
		if ns == nil {
			return fmt.Errorf("Unknown service ID '%s' for check ID '%s'", check.ServiceID, check.CheckID)
		}

		other, ok := ns.Services[check.ServiceID]
		if !ok {
			return fmt.Errorf("Unknown service ID '%s' for check ID '%s'", check.ServiceID, check.CheckID)
		}

		// We are only adding a check here, so we don't add the scope,
		// since the sentinel policy doesn't apply to adding checks at
		// this time.
		var secondaryCtx acl.AuthorizerContext
		other.FillAuthzContext(&secondaryCtx)

		if err := authz.ToAllowAuthorizer().ServiceWriteAllowed(other.Service, &secondaryCtx); err != nil {
			return err
		}
	}

	return nil
}

// Deregister a service or check in a node, or the entire node itself.
//
// If a ServiceID is provided in the request, any associated Checks
// with that service are also deregistered.
//
// If a ServiceID or CheckID is not provided in the request, the entire
// node is deregistered.
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
	state := c.srv.fsm.State()

	var ns *structs.NodeService
	if args.ServiceID != "" {
		_, ns, err = state.NodeService(nil, args.Node, args.ServiceID, &args.EnterpriseMeta, args.PeerName)
		if err != nil {
			return fmt.Errorf("Service lookup failed: %v", err)
		}
	}

	var nc *structs.HealthCheck
	if args.CheckID != "" {
		_, nc, err = state.NodeCheck(args.Node, args.CheckID, &args.EnterpriseMeta, args.PeerName)
		if err != nil {
			return fmt.Errorf("Check lookup failed: %v", err)
		}
	}

	if err := vetDeregisterWithACL(authz, args, ns, nc); err != nil {
		return err
	}

	_, err = c.srv.raftApply(structs.DeregisterRequestType, args)
	return err
}

// vetDeregisterWithACL applies the given ACL's policy to the catalog update and
// determines if it is allowed. Since the catalog deregister request is so
// dynamic, this is a pretty complex algorithm and was worth breaking out of the
// endpoint. The NodeService for the referenced service must be supplied, and can
// be nil; similar for the HealthCheck for the referenced health check.
func vetDeregisterWithACL(
	authz resolver.Result,
	subj *structs.DeregisterRequest,
	ns *structs.NodeService,
	nc *structs.HealthCheck,
) error {
	// We don't apply sentinel in this path, since at this time sentinel
	// only applies to create and update operations.

	var authzContext acl.AuthorizerContext
	// fill with the defaults for use with the NodeWrite check
	subj.FillAuthzContext(&authzContext)

	// Allow service deregistration if the token has write permission for the node.
	// This accounts for cases where the agent no longer has a token with write permission
	// on the service to deregister it.
	nodeWriteErr := authz.ToAllowAuthorizer().NodeWriteAllowed(subj.Node, &authzContext)
	if nodeWriteErr == nil {
		return nil
	}

	// This order must match the code in applyDeregister() in
	// fsm/commands_ce.go since it also evaluates things in this order,
	// and will ignore fields based on this precedence. This lets us also
	// ignore them from an ACL perspective.
	if subj.ServiceID != "" {
		if ns == nil {
			return fmt.Errorf("Unknown service ID '%s'", subj.ServiceID)
		}

		ns.FillAuthzContext(&authzContext)

		if err := authz.ToAllowAuthorizer().ServiceWriteAllowed(ns.Service, &authzContext); err != nil {
			return err
		}
	} else if subj.CheckID != "" {
		if nc == nil {
			return fmt.Errorf("Unknown check ID '%s'", subj.CheckID)
		}

		nc.FillAuthzContext(&authzContext)

		if nc.ServiceID != "" {
			if err := authz.ToAllowAuthorizer().ServiceWriteAllowed(nc.ServiceName, &authzContext); err != nil {
				return err
			}
		} else {
			if err := authz.ToAllowAuthorizer().NodeWriteAllowed(subj.Node, &authzContext); err != nil {
				return err
			}
		}
	} else {
		// Since NodeWrite is not given - otherwise the earlier check
		// would've returned already - we can deny here.
		return nodeWriteErr
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

// ListNodes is used to query the nodes in a DC.
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
				reply.Index, reply.Nodes, err = state.NodesByMeta(ws, args.NodeMetaFilters, &args.EnterpriseMeta, args.PeerName)
			} else {
				reply.Index, reply.Nodes, err = state.Nodes(ws, &args.EnterpriseMeta, args.PeerName)
			}
			if err != nil {
				return err
			}
			if isUnmodified(args.QueryOptions, reply.Index) {
				reply.QueryMeta.NotModified = true
				reply.Nodes = nil
				return nil
			}

			raw, err := filter.Execute(reply.Nodes)
			if err != nil {
				return err
			}
			reply.Nodes = raw.(structs.Nodes)

			// Note: we filter the results with ACLs *after* applying the user-supplied
			// bexpr filter, to ensure QueryMeta.ResultsFilteredByACLs does not include
			// results that would be filtered out even if the user did have permission.
			if err := c.srv.filterACL(args.Token, reply); err != nil {
				return err
			}

			return c.srv.sortNodesByDistanceFrom(args.Source, reply.Nodes)
		})
}

func isUnmodified(opts structs.QueryOptions, index uint64) bool {
	return opts.AllowNotModifiedResponse && opts.MinQueryIndex > 0 && opts.MinQueryIndex == index
}

// ListServices is used to query the services in a DC.
// Returns services as a map of service names to available tags.
func (c *Catalog) ListServices(args *structs.DCSpecificRequest, reply *structs.IndexedServices) error {
	if done, err := c.srv.ForwardRPC("Catalog.ListServices", args, reply); done {
		return err
	}

	fmt.Println("in list services")

	// Supporting querying by PeerName in this API would require modifying the return type or the ACL
	// filtering logic so that it can be made aware that the data queried is coming from a peer.
	// Currently the ACL filter will receive plain name strings with no awareness of the peer name,
	// which means that authz will be done as if these were local service names.
	if args.PeerName != structs.DefaultPeerKeyword {
		return errors.New("listing service names imported from a peer is not supported")
	}

	authz, err := c.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, nil)
	if err != nil {
		return err
	}

	if err := c.srv.validateEnterpriseRequest(&args.EnterpriseMeta, false); err != nil {
		return err
	}

	filter, err := bexpr.CreateFilter(args.Filter, nil, []*structs.ServiceNode{})
	if err != nil {
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
			var serviceNodes structs.ServiceNodes
			if len(args.NodeMetaFilters) > 0 {
				fmt.Println("in node meta filters > 0")
				reply.Index, serviceNodes, err = state.ServicesByNodeMeta(ws, args.NodeMetaFilters, &args.EnterpriseMeta, args.PeerName)
			} else {
				fmt.Println("in node meta filters == 0")
				reply.Index, serviceNodes, err = state.Services(ws, &args.EnterpriseMeta, args.PeerName)
			}
			if err != nil {
				return err
			}
			if isUnmodified(args.QueryOptions, reply.Index) {
				reply.Services = nil
				reply.QueryMeta.NotModified = true
				return nil
			}

			raw, err := filter.Execute(serviceNodes)
			if err != nil {
				return err
			}

			reply.Services = servicesTagsByName(raw.(structs.ServiceNodes))

			c.srv.filterACLWithAuthorizer(authz, reply)

			return nil
		})
}

func servicesTagsByName(services []*structs.ServiceNode) structs.Services {
	unique := make(map[string]map[string]struct{})
	for _, svc := range services {
		tags, ok := unique[svc.ServiceName]
		if !ok {
			unique[svc.ServiceName] = make(map[string]struct{})
			tags = unique[svc.ServiceName]
		}
		for _, tag := range svc.ServiceTags {
			tags[tag] = struct{}{}
		}
	}

	// Generate the output structure.
	var results = make(structs.Services)
	for service, tags := range unique {
		results[service] = make([]string, 0, len(tags))
		for tag := range tags {
			results[service] = append(results[service], tag)
		}
	}
	return results
}

// ServiceList is used to query the services in a DC.
// Returns services as a list of ServiceNames.
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
			index, services, err := state.ServiceList(ws, &args.EnterpriseMeta, args.PeerName)
			if err != nil {
				return err
			}

			reply.Index, reply.Services = index, services
			c.srv.filterACLWithAuthorizer(authz, reply)
			return nil
		})
}

// ServiceNodes returns all the nodes registered as part of a service.
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
			return s.ConnectServiceNodes(ws, args.ServiceName, &args.EnterpriseMeta, args.PeerName)
		}

	default:
		f = func(ws memdb.WatchSet, s *state.Store) (uint64, structs.ServiceNodes, error) {
			if args.ServiceAddress != "" {
				return s.ServiceAddressNodes(ws, args.ServiceAddress, &args.EnterpriseMeta, args.PeerName)
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

				return s.ServiceTagNodes(ws, args.ServiceName, tags, &args.EnterpriseMeta, args.PeerName)
			}

			return s.ServiceNodes(ws, args.ServiceName, &args.EnterpriseMeta, args.PeerName)
		}
	}

	authzContext := acl.AuthorizerContext{
		Peer: args.PeerName,
	}
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
		// TODO(acl-error-enhancements) can this be improved? What happens if we returned an error here?
		// Is this similar to filters where we might want to return a hint?
		if authz.ServiceRead(args.ServiceName, &authzContext) != acl.Allow {
			// Just return nil, which will return an empty response (tested)
			return nil
		}
	}

	filter, err := bexpr.CreateFilter(args.Filter, nil, reply.ServiceNodes)
	if err != nil {
		return err
	}

	var (
		priorMergeHash uint64
		ranMergeOnce   bool
	)

	err = c.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, services, err := f(ws, state)
			if err != nil {
				return err
			}

			mergedServices := services

			if args.MergeCentralConfig {
				var mergedServiceNodes structs.ServiceNodes
				for _, sn := range services {
					mergedsn := sn
					ns := sn.ToNodeService()
					if ns.IsSidecarProxy() || ns.IsGateway() {
						cfgIndex, mergedns, err := configentry.MergeNodeServiceWithCentralConfig(ws, state, args, ns, c.logger)
						if err != nil {
							return err
						}
						if cfgIndex > index {
							index = cfgIndex
						}
						mergedsn = mergedns.ToServiceNode(sn.Node)
					}
					mergedServiceNodes = append(mergedServiceNodes, mergedsn)
				}
				if len(mergedServiceNodes) > 0 {
					mergedServices = mergedServiceNodes
				}

				// Generate a hash of the mergedServices driving this response.
				// Use it to determine if the response is identical to a prior wakeup.
				newMergeHash, err := hashstructure_v2.Hash(mergedServices, hashstructure_v2.FormatV2, nil)
				if err != nil {
					return fmt.Errorf("error hashing reply for spurious wakeup suppression: %w", err)
				}
				if ranMergeOnce && priorMergeHash == newMergeHash {
					// the below assignment is not required as the if condition already validates equality,
					// but makes it more clear that prior value is being reset to the new hash on each run.
					priorMergeHash = newMergeHash
					reply.Index = index
					// NOTE: the prior response is still alive inside of *reply, which is desirable
					return errNotChanged
				} else {
					priorMergeHash = newMergeHash
					ranMergeOnce = true
				}

			}

			reply.Index, reply.ServiceNodes = index, mergedServices
			if len(args.NodeMetaFilters) > 0 {
				var filtered structs.ServiceNodes
				for _, service := range mergedServices {
					if structs.SatisfiesMetaFilters(service.NodeMeta, args.NodeMetaFilters) {
						filtered = append(filtered, service)
					}
				}
				reply.ServiceNodes = filtered
			}

			// This is safe to do even when the filter is nil - its just a no-op then
			raw, err := filter.Execute(reply.ServiceNodes)
			if err != nil {
				return err
			}
			reply.ServiceNodes = raw.(structs.ServiceNodes)

			// Note: we filter the results with ACLs *after* applying the user-supplied
			// bexpr filter, to ensure QueryMeta.ResultsFilteredByACLs does not include
			// results that would be filtered out even if the user did have permission.
			if err := c.srv.filterACL(args.Token, reply); err != nil {
				return err
			}

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

// NodeServices returns all the services registered as part of a node.
// Returns NodeServices as a map of service IDs to services.
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
			index, services, err := state.NodeServices(ws, args.Node, &args.EnterpriseMeta, args.PeerName)
			if err != nil {
				return err
			}
			reply.Index, reply.NodeServices = index, services

			if reply.NodeServices != nil {
				raw, err := filter.Execute(reply.NodeServices.Services)
				if err != nil {
					return err
				}
				reply.NodeServices.Services = raw.(map[string]*structs.NodeService)
			}

			// Note: we filter the results with ACLs *after* applying the user-supplied
			// bexpr filter, to ensure QueryMeta.ResultsFilteredByACLs does not include
			// results that would be filtered out even if the user did have permission.
			if err := c.srv.filterACL(args.Token, reply); err != nil {
				return err
			}

			return nil
		})
}

// NodeServiceList returns all the services registered as part of a node.
// Returns NodeServices as a list of services.
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

	var (
		priorMergeHash uint64
		ranMergeOnce   bool
	)

	return c.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, services, err := state.NodeServiceList(ws, args.Node, &args.EnterpriseMeta, args.PeerName)
			if err != nil {
				return err
			}

			mergedServices := services
			var cfgIndex uint64
			if services != nil && args.MergeCentralConfig {
				var mergedNodeServices []*structs.NodeService
				for _, ns := range services.Services {
					mergedns := ns
					if ns.IsSidecarProxy() || ns.IsGateway() {
						serviceSpecificReq := structs.ServiceSpecificRequest{
							Datacenter:   args.Datacenter,
							QueryOptions: args.QueryOptions,
						}
						cfgIndex, mergedns, err = configentry.MergeNodeServiceWithCentralConfig(ws, state, &serviceSpecificReq, ns, c.logger)
						if err != nil {
							return err
						}
						if cfgIndex > index {
							index = cfgIndex
						}
					}
					mergedNodeServices = append(mergedNodeServices, mergedns)
				}
				if len(mergedNodeServices) > 0 {
					mergedServices.Services = mergedNodeServices
				}

				// Generate a hash of the mergedServices driving this response.
				// Use it to determine if the response is identical to a prior wakeup.
				newMergeHash, err := hashstructure_v2.Hash(mergedServices, hashstructure_v2.FormatV2, nil)
				if err != nil {
					return fmt.Errorf("error hashing reply for spurious wakeup suppression: %w", err)
				}
				if ranMergeOnce && priorMergeHash == newMergeHash {
					// the below assignment is not required as the if condition already validates equality,
					// but makes it more clear that prior value is being reset to the new hash on each run.
					priorMergeHash = newMergeHash
					reply.Index = index
					// NOTE: the prior response is still alive inside of *reply, which is desirable
					return errNotChanged
				} else {
					priorMergeHash = newMergeHash
					ranMergeOnce = true
				}

			}

			reply.Index = index

			if mergedServices != nil {
				reply.NodeServices = *mergedServices

				raw, err := filter.Execute(reply.NodeServices.Services)
				if err != nil {
					return err
				}
				reply.NodeServices.Services = raw.([]*structs.NodeService)
			}

			// Note: we filter the results with ACLs *after* applying the user-supplied
			// bexpr filter, to ensure QueryMeta.ResultsFilteredByACLs does not include
			// results that would be filtered out even if the user did have permission.
			if err := c.srv.filterACL(args.Token, reply); err != nil {
				return err
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

	if err := authz.ToAllowAuthorizer().ServiceReadAllowed(args.ServiceName, &authzContext); err != nil {
		return err
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
			reply.Index, reply.Services = index, services

			if err := c.srv.filterACL(args.Token, reply); err != nil {
				return err
			}
			return nil
		})
}

func (c *Catalog) VirtualIPForService(args *structs.ServiceSpecificRequest, reply *string) error {
	if done, err := c.srv.ForwardRPC("Catalog.VirtualIPForService", args, reply); done {
		return err
	}

	authzContext := acl.AuthorizerContext{
		Peer: args.PeerName,
	}
	authz, err := c.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, &authzContext)
	if err != nil {
		return err
	}

	if err := c.srv.validateEnterpriseRequest(&args.EnterpriseMeta, false); err != nil {
		return err
	}

	if err := authz.ToAllowAuthorizer().ServiceReadAllowed(args.ServiceName, &authzContext); err != nil {
		return err
	}

	state := c.srv.fsm.State()
	psn := structs.PeeredServiceName{Peer: args.PeerName, ServiceName: structs.NewServiceName(args.ServiceName, &args.EnterpriseMeta)}
	*reply, err = state.VirtualIPForService(psn)
	return err
}
