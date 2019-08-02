package discoverychain

import (
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/mitchellh/hashstructure"
	"github.com/mitchellh/mapstructure"
)

type CompileRequest struct {
	ServiceName       string
	CurrentNamespace  string
	CurrentDatacenter string

	// OverrideMeshGateway allows for the setting to be overridden for any
	// resolver in the compiled chain.
	OverrideMeshGateway structs.MeshGatewayConfig

	// OverrideProtocol allows for the final protocol for the chain to be
	// altered.
	//
	// - If the chain ordinarily would be TCP and an L7 protocol is passed here
	// the chain will not include Routers or Splitters.
	//
	// - If the chain ordinarily would be L7 and TCP is passed here the chain
	// will not include Routers or Splitters.
	OverrideProtocol string

	// OverrideConnectTimeout allows for the ConnectTimeout setting to be
	// overridden for any resolver in the compiled chain.
	OverrideConnectTimeout time.Duration

	Entries *structs.DiscoveryChainConfigEntries
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
		serviceName:            serviceName,
		currentNamespace:       currentNamespace,
		currentDatacenter:      currentDatacenter,
		overrideMeshGateway:    req.OverrideMeshGateway,
		overrideProtocol:       req.OverrideProtocol,
		overrideConnectTimeout: req.OverrideConnectTimeout,
		entries:                entries,

		resolvers:     make(map[string]*structs.ServiceResolverConfigEntry),
		splitterNodes: make(map[string]*structs.DiscoveryGraphNode),
		resolveNodes:  make(map[structs.DiscoveryTarget]*structs.DiscoveryGraphNode),

		nodes: make(map[string]*structs.DiscoveryGraphNode),

		targets: make(map[structs.DiscoveryTarget]structs.DiscoveryTargetConfig),
	}

	if req.OverrideProtocol != "" {
		c.disableAdvancedRoutingFeatures = !enableAdvancedRoutingForProtocol(req.OverrideProtocol)
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
	serviceName            string
	currentNamespace       string
	currentDatacenter      string
	overrideMeshGateway    structs.MeshGatewayConfig
	overrideProtocol       string
	overrideConnectTimeout time.Duration

	// config entries that are being compiled (will be mutated during compilation)
	//
	// This is an INPUT field.
	entries *structs.DiscoveryChainConfigEntries

	// resolvers is initially seeded by copying the provided entries.Resolvers
	// map and default resolvers are added as they are needed.
	resolvers map[string]*structs.ServiceResolverConfigEntry

	// cached nodes
	splitterNodes map[string]*structs.DiscoveryGraphNode
	resolveNodes  map[structs.DiscoveryTarget]*structs.DiscoveryGraphNode

	// usesAdvancedRoutingFeatures is set to true if config entries for routing
	// or splitting appear in the compiled chain
	usesAdvancedRoutingFeatures bool

	// disableAdvancedRoutingFeatures is set to true if overrideProtocol is set to tcp
	disableAdvancedRoutingFeatures bool

	// customizedBy indicates which override values customized how the
	// compilation behaved.
	//
	// This is an OUTPUT field.
	customizedBy customizationMarkers

	// protocol is the common protocol used for all referenced services. These
	// cannot be mixed.
	//
	// This is an OUTPUT field.
	protocol string

	// startNode is computed inside of assembleChain()
	//
	// This is an OUTPUT field.
	startNode string

	// nodes is computed inside of compile()
	//
	// This is an OUTPUT field.
	nodes map[string]*structs.DiscoveryGraphNode

	// This is an OUTPUT field.
	targets map[structs.DiscoveryTarget]structs.DiscoveryTargetConfig
}

type customizationMarkers struct {
	MeshGateway    bool
	Protocol       bool
	ConnectTimeout bool
}

func (m *customizationMarkers) IsZero() bool {
	return !m.MeshGateway && !m.Protocol && !m.ConnectTimeout
}

// recordNode stores the node internally in the compiled chain.
func (c *compiler) recordNode(node *structs.DiscoveryGraphNode) {
	// Some types have their own type-specific lookups, so record those, too.
	switch node.Type {
	case structs.DiscoveryGraphNodeTypeRouter:
		// no special storage
	case structs.DiscoveryGraphNodeTypeSplitter:
		c.splitterNodes[node.ServiceName()] = node
	case structs.DiscoveryGraphNodeTypeResolver:
		c.resolveNodes[node.Resolver.Target] = node
	default:
		panic("unknown node type '" + node.Type + "'")
	}

	c.nodes[node.MapKey()] = node
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

	// We don't need these intermediates anymore.
	c.splitterNodes = nil
	c.resolveNodes = nil

	if c.startNode == "" {
		panic("impossible to return no results")
	}

	if err := c.detectCircularSplits(); err != nil {
		return nil, err
	}
	if err := c.detectCircularResolves(); err != nil {
		return nil, err
	}

	c.flattenAdjacentSplitterNodes()

	if err := c.removeUnusedNodes(); err != nil {
		return nil, err
	}

	if !enableAdvancedRoutingForProtocol(c.protocol) && c.usesAdvancedRoutingFeatures {
		return nil, &structs.ConfigEntryGraphError{
			Message: fmt.Sprintf(
				"discovery chain %q uses a protocol %q that does not permit advanced routing or splitting behavior",
				c.serviceName, c.protocol,
			),
		}
	}

	if c.overrideProtocol != "" {
		if c.overrideProtocol != c.protocol {
			c.protocol = c.overrideProtocol
			c.customizedBy.Protocol = true
		}
	}

	var customizationHash string
	if !c.customizedBy.IsZero() {
		var customization struct {
			OverrideMeshGateway    structs.MeshGatewayConfig
			OverrideProtocol       string
			OverrideConnectTimeout time.Duration
		}

		if c.customizedBy.MeshGateway {
			customization.OverrideMeshGateway = c.overrideMeshGateway
		}
		if c.customizedBy.Protocol {
			customization.OverrideProtocol = c.overrideProtocol
		}
		if c.customizedBy.ConnectTimeout {
			customization.OverrideConnectTimeout = c.overrideConnectTimeout
		}
		v, err := hashstructure.Hash(customization, nil)
		if err != nil {
			return nil, fmt.Errorf("cannot create customization hash key: %v", err)
		}
		customizationHash = fmt.Sprintf("%x", v)[0:8]
	}

	return &structs.CompiledDiscoveryChain{
		ServiceName:       c.serviceName,
		Namespace:         c.currentNamespace,
		Datacenter:        c.currentDatacenter,
		CustomizationHash: customizationHash,
		Protocol:          c.protocol,
		StartNode:         c.startNode,
		Nodes:             c.nodes,
		Targets:           c.targets,
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
		for _, node := range c.nodes {
			if node.Type != structs.DiscoveryGraphNodeTypeSplitter {
				continue
			}

			fixedSplits := make([]*structs.DiscoverySplit, 0, len(node.Splits))
			changed := false
			for _, split := range node.Splits {
				nextNode := c.nodes[split.NextNode]
				if nextNode.Type != structs.DiscoveryGraphNodeTypeSplitter {
					fixedSplits = append(fixedSplits, split)
					continue
				}

				changed = true

				for _, innerSplit := range nextNode.Splits {
					effectiveWeight := split.Weight * innerSplit.Weight / 100

					newDiscoverySplit := &structs.DiscoverySplit{
						Weight:   structs.NormalizeServiceSplitWeight(effectiveWeight),
						NextNode: innerSplit.NextNode,
					}

					fixedSplits = append(fixedSplits, newDiscoverySplit)
				}
			}

			if changed {
				node.Splits = fixedSplits
				anyChanged = true
			}
		}

		if !anyChanged {
			return
		}
	}
}

// removeUnusedNodes walks the chain from the start and prunes any nodes that
// are no longer referenced. This can happen as a result of operations like
// flattenAdjacentSplitterNodes().
func (c *compiler) removeUnusedNodes() error {
	var (
		visited = make(map[string]struct{})
		todo    = make(map[string]struct{})
	)

	todo[c.startNode] = struct{}{}

	getNext := func() string {
		if len(todo) == 0 {
			return ""
		}
		for k, _ := range todo {
			delete(todo, k)
			return k
		}
		return ""
	}

	for {
		next := getNext()
		if next == "" {
			break
		}
		if _, ok := visited[next]; ok {
			continue
		}
		visited[next] = struct{}{}

		node := c.nodes[next]
		if node == nil {
			return fmt.Errorf("compilation references non-retained node %q", next)
		}

		switch node.Type {
		case structs.DiscoveryGraphNodeTypeRouter:
			for _, route := range node.Routes {
				todo[route.NextNode] = struct{}{}
			}
		case structs.DiscoveryGraphNodeTypeSplitter:
			for _, split := range node.Splits {
				todo[split.NextNode] = struct{}{}
			}
		case structs.DiscoveryGraphNodeTypeResolver:
			// nothing special
		default:
			return fmt.Errorf("unknown node type %q", node.Type)
		}
	}

	if len(visited) == len(c.nodes) {
		return nil
	}

	for name, _ := range c.nodes {
		if _, ok := visited[name]; !ok {
			delete(c.nodes, name)
		}
	}

	return nil
}

// assembleChain will do the initial assembly of a chain of DiscoveryGraphNode
// entries from the provided config entries.
func (c *compiler) assembleChain() error {
	if c.startNode != "" || len(c.nodes) > 0 {
		return fmt.Errorf("assembleChain should only be called once")
	}

	// Check for short circuit path.
	if len(c.resolvers) == 0 && c.entries.IsChainEmpty() {
		// Materialize defaults and cache.
		c.resolvers[c.serviceName] = newDefaultServiceResolver(c.serviceName)
	}

	// The only router we consult is the one for the service name at the top of
	// the chain.
	router := c.entries.GetRouter(c.serviceName)
	if router != nil && c.disableAdvancedRoutingFeatures {
		router = nil
		c.customizedBy.Protocol = true
	}

	if router == nil {
		// If no router is configured, move on down the line to the next hop of
		// the chain.
		node, err := c.getSplitterOrResolverNode(c.newTarget(c.serviceName, "", "", ""))
		if err != nil {
			return err
		}

		c.startNode = node.MapKey()
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
			node, err = c.getSplitterOrResolverNode(
				c.newTarget(svc, dest.ServiceSubset, dest.Namespace, ""),
			)
		} else {
			node, err = c.getResolverNode(
				c.newTarget(svc, dest.ServiceSubset, dest.Namespace, ""),
				false,
			)
		}
		if err != nil {
			return err
		}
		compiledRoute.NextNode = node.MapKey()
	}

	// If we have a router, we'll add a catch-all route at the end to send
	// unmatched traffic to the next hop in the chain.
	defaultDestinationNode, err := c.getSplitterOrResolverNode(c.newTarget(c.serviceName, "", "", ""))
	if err != nil {
		return err
	}

	defaultRoute := &structs.DiscoveryRoute{
		Definition: newDefaultServiceRoute(c.serviceName),
		NextNode:   defaultDestinationNode.MapKey(),
	}
	routeNode.Routes = append(routeNode.Routes, defaultRoute)

	c.startNode = routeNode.MapKey()
	c.recordNode(routeNode)

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

func (c *compiler) getSplitterOrResolverNode(target structs.DiscoveryTarget) (*structs.DiscoveryGraphNode, error) {
	nextNode, err := c.getSplitterNode(target.Service)
	if err != nil {
		return nil, err
	} else if nextNode != nil {
		return nextNode, nil
	}
	return c.getResolverNode(target, false)
}

func (c *compiler) getSplitterNode(name string) (*structs.DiscoveryGraphNode, error) {
	// Do we already have the node?
	if prev, ok := c.splitterNodes[name]; ok {
		return prev, nil
	}

	// Fetch the config entry.
	splitter := c.entries.GetSplitter(name)
	if splitter != nil && c.disableAdvancedRoutingFeatures {
		splitter = nil
		c.customizedBy.Protocol = true
	}
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
	c.recordNode(splitNode)

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
				compiledSplit.NextNode = nextNode.MapKey()
				continue
			}
			// fall through to group-resolver
		}

		node, err := c.getResolverNode(
			c.newTarget(svc, split.ServiceSubset, split.Namespace, ""),
			false,
		)
		if err != nil {
			return nil, err
		}
		compiledSplit.NextNode = node.MapKey()
	}

	c.usesAdvancedRoutingFeatures = true
	return splitNode, nil
}

// getResolverNode handles most of the code to handle redirection/rewriting
// capabilities from a resolver config entry. It recurses into itself to
// _generate_ targets used for failover out of convenience.
func (c *compiler) getResolverNode(target structs.DiscoveryTarget, recursedForFailover bool) (*structs.DiscoveryGraphNode, error) {
RESOLVE_AGAIN:
	// Do we already have the node?
	if prev, ok := c.resolveNodes[target]; ok {
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

	connectTimeout := resolver.ConnectTimeout
	if connectTimeout < 1 {
		connectTimeout = 5 * time.Second
	}

	if c.overrideConnectTimeout > 0 {
		if connectTimeout != c.overrideConnectTimeout {
			connectTimeout = c.overrideConnectTimeout
			c.customizedBy.ConnectTimeout = true
		}
	}

	// Build node.
	node := &structs.DiscoveryGraphNode{
		Type: structs.DiscoveryGraphNodeTypeResolver,
		Name: target.Identifier(),
		Resolver: &structs.DiscoveryResolver{
			Definition:     resolver,
			Default:        resolver.IsDefault(),
			Target:         target,
			ConnectTimeout: connectTimeout,
		},
	}

	targetConfig := structs.DiscoveryTargetConfig{
		Subset: resolver.Subsets[target.ServiceSubset],
	}

	// Default mesh gateway settings
	if serviceDefault := c.entries.GetService(target.Service); serviceDefault != nil {
		targetConfig.MeshGateway = serviceDefault.MeshGateway
	}

	if c.entries.GlobalProxy != nil && targetConfig.MeshGateway.Mode == structs.MeshGatewayModeDefault {
		targetConfig.MeshGateway.Mode = c.entries.GlobalProxy.MeshGateway.Mode
	}

	if c.overrideMeshGateway.Mode != structs.MeshGatewayModeDefault {
		if targetConfig.MeshGateway.Mode != c.overrideMeshGateway.Mode {
			targetConfig.MeshGateway.Mode = c.overrideMeshGateway.Mode
			c.customizedBy.MeshGateway = true
		}
	}

	// Retain this target even if we may not retain the group resolver.
	c.targets[target] = targetConfig

	if recursedForFailover {
		// If we recursed here from ourselves in a failover context, just emit
		// this node without caching it or even processing failover again.
		// This is a little weird but it keeps the redirect/default-subset
		// logic in one place.
		return node, nil
	}

	// If we record this exists before recursing down it will short-circuit
	// sanely if there is some sort of graph loop below.
	c.recordNode(node)

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
				node.Resolver.Failover = df

				// Convert the targets into targets by cheating a bit and
				// recursing into ourselves.
				for _, target := range failoverTargets {
					failoverResolveNode, err := c.getResolverNode(target, true)
					if err != nil {
						return nil, err
					}
					failoverTarget := failoverResolveNode.Resolver.Target
					df.Targets = append(df.Targets, failoverTarget)
				}
			}
		}
	}

	return node, nil
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
