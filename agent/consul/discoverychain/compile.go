package discoverychain

import (
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/mitchellh/mapstructure"
)

type CompileRequest struct {
	ServiceName       string
	CurrentNamespace  string
	CurrentDatacenter string
	InferDefaults     bool // TODO(rb): remove this?
	Entries           *structs.DiscoveryChainConfigEntries
}

// Compile assembles a discovery chain in the form of a graph of nodes using
// raw config entries and local context.
//
// "Node" referenced in this file refers to a node in a graph and not to the
// Consul construct called a "Node".
//
// Omitting router and splitter entries for services not using an L7 protocol
// (like HTTP) happens during initial fetching, but for sanity purposes a quick
// reinforcement of that happens here, too.
//
// May return a *structs.ConfigEntryGraphError, but that is only expected when
// being used to validate modifications to the config entry graph. It should
// not be expected when compiling existing entries at runtime that are already
// valid.
func Compile(req CompileRequest) (*structs.CompiledDiscoveryChain, error) {
	var (
		serviceName       = req.ServiceName
		currentNamespace  = req.CurrentNamespace
		currentDatacenter = req.CurrentDatacenter
		inferDefaults     = req.InferDefaults
		entries           = req.Entries
	)
	if serviceName == "" {
		return nil, fmt.Errorf("serviceName is required")
	}
	if currentNamespace == "" {
		return nil, fmt.Errorf("currentNamespace is required")
	}
	if currentDatacenter == "" {
		return nil, fmt.Errorf("currentDatacenter is required")
	}
	if entries == nil {
		return nil, fmt.Errorf("entries is required")
	}

	c := &compiler{
		serviceName:       serviceName,
		currentNamespace:  currentNamespace,
		currentDatacenter: currentDatacenter,
		inferDefaults:     inferDefaults,
		entries:           entries,

		splitterNodes:      make(map[string]*structs.DiscoveryGraphNode),
		groupResolverNodes: make(map[structs.DiscoveryTarget]*structs.DiscoveryGraphNode),

		resolvers:       make(map[string]*structs.ServiceResolverConfigEntry),
		retainResolvers: make(map[string]struct{}),
		targets:         make(map[structs.DiscoveryTarget]struct{}),
	}

	// Clone this resolver map to avoid mutating the input map during compilation.
	if len(entries.Resolvers) > 0 {
		for k, v := range entries.Resolvers {
			c.resolvers[k] = v
		}
	}

	return c.compile()
}

// compiler is a single-use struct for handling intermediate state necessary
// for assembling a discovery chain from raw config entries.
type compiler struct {
	serviceName       string
	currentNamespace  string
	currentDatacenter string
	inferDefaults     bool

	// config entries that are being compiled (will be mutated during compilation)
	//
	// This is an INPUT field.
	entries *structs.DiscoveryChainConfigEntries

	// cached nodes
	splitterNodes      map[string]*structs.DiscoveryGraphNode
	groupResolverNodes map[structs.DiscoveryTarget]*structs.DiscoveryGraphNode // this is also an OUTPUT field

	// usesAdvancedRoutingFeatures is set to true if config entries for routing
	// or splitting appear in the compiled chain
	usesAdvancedRoutingFeatures bool

	// topNode is computed inside of assembleChain()
	//
	// This is an OUTPUT field.
	topNode *structs.DiscoveryGraphNode

	// protocol is the common protocol used for all referenced services. These
	// cannot be mixed.
	//
	// This is an OUTPUT field.
	protocol string

	// resolvers is initially seeded by copying the provided entries.Resolvers
	// map and default resolvers are added as they are needed.
	//
	// If redirects cause a resolver to not be needed it will be omitted from
	// this map.
	//
	// This is an OUTPUT field.
	resolvers map[string]*structs.ServiceResolverConfigEntry
	// retainResolvers flags the elements of the resolvers map that should be
	// retained in the final results.
	retainResolvers map[string]struct{}

	// This is an OUTPUT field.
	targets map[structs.DiscoveryTarget]struct{}
}

func (c *compiler) recordServiceProtocol(serviceName string) error {
	if serviceDefault := c.entries.GetService(serviceName); serviceDefault != nil {
		return c.recordProtocol(serviceName, serviceDefault.Protocol)
	}
	if c.entries.GlobalProxy != nil {
		var cfg proxyConfig
		// Ignore errors and fallback on defaults if it does happen.
		_ = mapstructure.WeakDecode(c.entries.GlobalProxy.Config, &cfg)
		if cfg.Protocol != "" {
			return c.recordProtocol(serviceName, cfg.Protocol)
		}
	}
	return c.recordProtocol(serviceName, "")
}

// proxyConfig is a snippet from agent/xds/config.go:ProxyConfig
type proxyConfig struct {
	Protocol string `mapstructure:"protocol"`
}

func (c *compiler) recordProtocol(fromService, protocol string) error {
	if protocol == "" {
		protocol = "tcp"
	} else {
		protocol = strings.ToLower(protocol)
	}

	if c.protocol == "" {
		c.protocol = protocol
	} else if c.protocol != protocol {
		return &structs.ConfigEntryGraphError{
			Message: fmt.Sprintf(
				"discovery chain %q uses inconsistent protocols; service %q has %q which is not %q",
				c.serviceName, fromService, protocol, c.protocol,
			),
		}
	}

	return nil
}

func (c *compiler) compile() (*structs.CompiledDiscoveryChain, error) {
	if err := c.assembleChain(); err != nil {
		return nil, err
	}

	if c.topNode == nil {
		if c.inferDefaults {
			panic("impossible to return no results with infer defaults set to true")
		}
		return nil, nil
	}

	if err := c.detectCircularSplits(); err != nil {
		return nil, err
	}
	if err := c.detectCircularResolves(); err != nil {
		return nil, err
	}

	c.flattenAdjacentSplitterNodes()

	// Remove any unused resolvers.
	for name, _ := range c.resolvers {
		if _, ok := c.retainResolvers[name]; !ok {
			delete(c.resolvers, name)
		}
	}

	if !enableAdvancedRoutingForProtocol(c.protocol) && c.usesAdvancedRoutingFeatures {
		return nil, &structs.ConfigEntryGraphError{
			Message: fmt.Sprintf(
				"discovery chain %q uses a protocol %q that does not permit advanced routing or splitting behavior",
				c.serviceName, c.protocol,
			),
		}
	}

	targets := make([]structs.DiscoveryTarget, 0, len(c.targets))
	for target, _ := range c.targets {
		targets = append(targets, target)
	}
	structs.DiscoveryTargets(targets).Sort()

	return &structs.CompiledDiscoveryChain{
		ServiceName:        c.serviceName,
		Namespace:          c.currentNamespace,
		Datacenter:         c.currentDatacenter,
		Protocol:           c.protocol,
		Node:               c.topNode,
		Resolvers:          c.resolvers,
		Targets:            targets,
		GroupResolverNodes: c.groupResolverNodes, // TODO(rb): prune unused
	}, nil
}

func (c *compiler) detectCircularSplits() error {
	// TODO(rb): detect when a tree of splitters backtracks
	return nil
}

func (c *compiler) detectCircularResolves() error {
	// TODO(rb): detect when a series of redirects and failovers cause a circular reference
	return nil
}

func (c *compiler) flattenAdjacentSplitterNodes() {
	for {
		anyChanged := false
		for _, splitterNode := range c.splitterNodes {
			fixedSplits := make([]*structs.DiscoverySplit, 0, len(splitterNode.Splits))
			changed := false
			for _, split := range splitterNode.Splits {
				if split.Node.Type != structs.DiscoveryGraphNodeTypeSplitter {
					fixedSplits = append(fixedSplits, split)
					continue
				}

				changed = true

				for _, innerSplit := range split.Node.Splits {
					effectiveWeight := split.Weight * innerSplit.Weight / 100

					newDiscoverySplit := &structs.DiscoverySplit{
						Weight: structs.NormalizeServiceSplitWeight(effectiveWeight),
						Node:   innerSplit.Node,
					}

					fixedSplits = append(fixedSplits, newDiscoverySplit)
				}
			}

			if changed {
				splitterNode.Splits = fixedSplits
				anyChanged = true
			}
		}

		if !anyChanged {
			return
		}
	}
}

// assembleChain will do the initial assembly of a chain of DiscoveryGraphNode
// entries from the provided config entries.  No default resolvers are injected
// here so it is expected that if there are no discovery chain config entries
// set up for a given service that it will produce no topNode from this.
func (c *compiler) assembleChain() error {
	if c.topNode != nil {
		return fmt.Errorf("assembleChain should only be called once")
	}

	// Check for short circuit path.
	if len(c.resolvers) == 0 && c.entries.IsChainEmpty() {
		if !c.inferDefaults {
			return nil // nothing explicitly configured
		}

		// Materialize defaults and cache.
		c.resolvers[c.serviceName] = newDefaultServiceResolver(c.serviceName)
	}

	// The only router we consult is the one for the service name at the top of
	// the chain.
	router := c.entries.GetRouter(c.serviceName)
	if router == nil {
		// If no router is configured, move on down the line to the next hop of
		// the chain.
		node, err := c.getSplitterOrGroupResolverNode(c.newTarget(c.serviceName, "", "", ""))
		if err != nil {
			return err
		}

		c.topNode = node
		return nil
	}

	routeNode := &structs.DiscoveryGraphNode{
		Type:   structs.DiscoveryGraphNodeTypeRouter,
		Name:   router.Name,
		Routes: make([]*structs.DiscoveryRoute, 0, len(router.Routes)+1),
	}
	c.usesAdvancedRoutingFeatures = true
	if err := c.recordServiceProtocol(router.Name); err != nil {
		return err
	}

	for i, _ := range router.Routes {
		// We don't use range variables here because we'll take the address of
		// this route and store that in a DiscoveryGraphNode and the range
		// variables share memory addresses between iterations which is exactly
		// wrong for us here.
		route := router.Routes[i]

		compiledRoute := &structs.DiscoveryRoute{Definition: &route}
		routeNode.Routes = append(routeNode.Routes, compiledRoute)

		dest := route.Destination

		svc := defaultIfEmpty(dest.Service, c.serviceName)

		// Check to see if the destination is eligible for splitting.
		var (
			node *structs.DiscoveryGraphNode
			err  error
		)
		if dest.ServiceSubset == "" && dest.Namespace == "" {
			node, err = c.getSplitterOrGroupResolverNode(
				c.newTarget(svc, dest.ServiceSubset, dest.Namespace, ""),
			)
		} else {
			node, err = c.getGroupResolverNode(
				c.newTarget(svc, dest.ServiceSubset, dest.Namespace, ""),
				false,
			)
		}
		if err != nil {
			return err
		}
		compiledRoute.DestinationNode = node
	}

	// If we have a router, we'll add a catch-all route at the end to send
	// unmatched traffic to the next hop in the chain.
	defaultDestinationNode, err := c.getSplitterOrGroupResolverNode(c.newTarget(c.serviceName, "", "", ""))
	if err != nil {
		return err
	}

	defaultRoute := &structs.DiscoveryRoute{
		Definition:      newDefaultServiceRoute(c.serviceName),
		DestinationNode: defaultDestinationNode,
	}
	routeNode.Routes = append(routeNode.Routes, defaultRoute)

	c.topNode = routeNode
	return nil
}

func newDefaultServiceRoute(serviceName string) *structs.ServiceRoute {
	return &structs.ServiceRoute{
		Match: &structs.ServiceRouteMatch{
			HTTP: &structs.ServiceRouteHTTPMatch{
				PathPrefix: "/",
			},
		},
		Destination: &structs.ServiceRouteDestination{
			Service: serviceName,
		},
	}
}

func (c *compiler) newTarget(service, serviceSubset, namespace, datacenter string) structs.DiscoveryTarget {
	if service == "" {
		panic("newTarget called with empty service which makes no sense")
	}
	return structs.DiscoveryTarget{
		Service:       service,
		ServiceSubset: serviceSubset,
		Namespace:     defaultIfEmpty(namespace, c.currentNamespace),
		Datacenter:    defaultIfEmpty(datacenter, c.currentDatacenter),
	}
}

func (c *compiler) getSplitterOrGroupResolverNode(target structs.DiscoveryTarget) (*structs.DiscoveryGraphNode, error) {
	nextNode, err := c.getSplitterNode(target.Service)
	if err != nil {
		return nil, err
	} else if nextNode != nil {
		return nextNode, nil
	}
	return c.getGroupResolverNode(target, false)
}

func (c *compiler) getSplitterNode(name string) (*structs.DiscoveryGraphNode, error) {
	// Do we already have the node?
	if prev, ok := c.splitterNodes[name]; ok {
		return prev, nil
	}

	// Fetch the config entry.
	splitter := c.entries.GetSplitter(name)
	if splitter == nil {
		return nil, nil
	}

	// Build node.
	splitNode := &structs.DiscoveryGraphNode{
		Type:   structs.DiscoveryGraphNodeTypeSplitter,
		Name:   name,
		Splits: make([]*structs.DiscoverySplit, 0, len(splitter.Splits)),
	}

	// If we record this exists before recursing down it will short-circuit
	// sanely if there is some sort of graph loop below.
	c.splitterNodes[name] = splitNode

	for _, split := range splitter.Splits {
		compiledSplit := &structs.DiscoverySplit{
			Weight: split.Weight,
		}
		splitNode.Splits = append(splitNode.Splits, compiledSplit)

		svc := defaultIfEmpty(split.Service, name)
		// Check to see if the split is eligible for additional splitting.
		if svc != name && split.ServiceSubset == "" && split.Namespace == "" {
			nextNode, err := c.getSplitterNode(svc)
			if err != nil {
				return nil, err
			} else if nextNode != nil {
				compiledSplit.Node = nextNode
				continue
			}
			// fall through to group-resolver
		}

		node, err := c.getGroupResolverNode(
			c.newTarget(svc, split.ServiceSubset, split.Namespace, ""),
			false,
		)
		if err != nil {
			return nil, err
		}
		compiledSplit.Node = node
	}

	c.usesAdvancedRoutingFeatures = true
	return splitNode, nil
}

// getGroupResolverNode handles most of the code to handle
// redirection/rewriting capabilities from a resolver config entry. It recurses
// into itself to _generate_ targets used for failover out of convenience.
func (c *compiler) getGroupResolverNode(target structs.DiscoveryTarget, recursedForFailover bool) (*structs.DiscoveryGraphNode, error) {
RESOLVE_AGAIN:
	// Do we already have the node?
	if prev, ok := c.groupResolverNodes[target]; ok {
		return prev, nil
	}

	if err := c.recordServiceProtocol(target.Service); err != nil {
		return nil, err
	}

	// Fetch the config entry.
	resolver, ok := c.resolvers[target.Service]
	if !ok {
		// Materialize defaults and cache.
		resolver = newDefaultServiceResolver(target.Service)
		c.resolvers[target.Service] = resolver
	}

	// Handle redirects right up front.
	//
	// TODO(rb): What about a redirected subset reference? (web/v2, but web redirects to alt/"")
	if resolver.Redirect != nil {
		redirect := resolver.Redirect

		redirectedTarget := target.CopyAndModify(
			redirect.Service,
			redirect.ServiceSubset,
			redirect.Namespace,
			redirect.Datacenter,
		)
		if redirectedTarget != target {
			target = redirectedTarget
			goto RESOLVE_AGAIN
		}
	}

	// Handle default subset.
	if target.ServiceSubset == "" && resolver.DefaultSubset != "" {
		target.ServiceSubset = resolver.DefaultSubset
		goto RESOLVE_AGAIN
	}

	if target.ServiceSubset != "" && !resolver.SubsetExists(target.ServiceSubset) {
		return nil, &structs.ConfigEntryGraphError{
			Message: fmt.Sprintf(
				"service %q does not have a subset named %q",
				target.Service,
				target.ServiceSubset,
			),
		}
	}

	// Since we're actually building a node with it, we can keep it.
	c.retainResolvers[target.Service] = struct{}{}

	connectTimeout := resolver.ConnectTimeout
	if connectTimeout < 1 {
		connectTimeout = 5 * time.Second
	}

	// Build node.
	groupResolverNode := &structs.DiscoveryGraphNode{
		Type: structs.DiscoveryGraphNodeTypeGroupResolver,
		Name: resolver.Name,
		GroupResolver: &structs.DiscoveryGroupResolver{
			Definition:     resolver,
			Default:        resolver.IsDefault(),
			Target:         target,
			ConnectTimeout: connectTimeout,
		},
	}
	groupResolver := groupResolverNode.GroupResolver

	// Default mesh gateway settings
	if serviceDefault := c.entries.GetService(resolver.Name); serviceDefault != nil {
		groupResolver.MeshGateway = serviceDefault.MeshGateway
	}

	if c.entries.GlobalProxy != nil && groupResolver.MeshGateway.Mode == structs.MeshGatewayModeDefault {
		groupResolver.MeshGateway.Mode = c.entries.GlobalProxy.MeshGateway.Mode
	}

	// Retain this target even if we may not retain the group resolver.
	c.targets[target] = struct{}{}

	if recursedForFailover {
		// If we recursed here from ourselves in a failover context, just emit
		// this node without caching it or even processing failover again.
		// This is a little weird but it keeps the redirect/default-subset
		// logic in one place.
		return groupResolverNode, nil
	}

	// If we record this exists before recursing down it will short-circuit
	// sanely if there is some sort of graph loop below.
	c.groupResolverNodes[target] = groupResolverNode

	if len(resolver.Failover) > 0 {
		f := resolver.Failover

		// Determine which failover section applies.
		failover, ok := f[target.ServiceSubset]
		if !ok {
			failover, ok = f["*"]
		}

		if ok {
			// Determine which failover definitions apply.
			var failoverTargets []structs.DiscoveryTarget
			if len(failover.Datacenters) > 0 {
				for _, dc := range failover.Datacenters {
					// Rewrite the target as per the failover policy.
					failoverTarget := target.CopyAndModify(
						failover.Service,
						failover.ServiceSubset,
						failover.Namespace,
						dc,
					)
					if failoverTarget != target { // don't failover to yourself
						failoverTargets = append(failoverTargets, failoverTarget)
					}
				}
			} else {
				// Rewrite the target as per the failover policy.
				failoverTarget := target.CopyAndModify(
					failover.Service,
					failover.ServiceSubset,
					failover.Namespace,
					"",
				)
				if failoverTarget != target { // don't failover to yourself
					failoverTargets = append(failoverTargets, failoverTarget)
				}
			}

			// If we filtered everything out then no point in having a failover.
			if len(failoverTargets) > 0 {
				df := &structs.DiscoveryFailover{
					Definition: &failover,
				}
				groupResolver.Failover = df

				// Convert the targets into targets by cheating a bit and
				// recursing into ourselves.
				for _, target := range failoverTargets {
					failoverGroupResolverNode, err := c.getGroupResolverNode(target, true)
					if err != nil {
						return nil, err
					}
					failoverTarget := failoverGroupResolverNode.GroupResolver.Target
					df.Targets = append(df.Targets, failoverTarget)
				}
			}
		}
	}

	return groupResolverNode, nil
}

func newDefaultServiceResolver(serviceName string) *structs.ServiceResolverConfigEntry {
	return &structs.ServiceResolverConfigEntry{
		Kind: structs.ServiceResolver,
		Name: serviceName,
	}
}

func defaultIfEmpty(val, defaultVal string) string {
	if val != "" {
		return val
	}
	return defaultVal
}

func enableAdvancedRoutingForProtocol(protocol string) bool {
	switch protocol {
	case "http", "http2", "grpc":
		return true
	default:
		return false
	}
}
