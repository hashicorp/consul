package discoverychain

import (
	"fmt"
	"strings"
	"time"

	"github.com/mitchellh/hashstructure"
	"github.com/mitchellh/mapstructure"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
)

type CompileRequest struct {
	ServiceName           string
	EvaluateInNamespace   string
	EvaluateInPartition   string
	EvaluateInDatacenter  string
	EvaluateInTrustDomain string

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

	Entries *configentry.DiscoveryChainSet
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
		serviceName           = req.ServiceName
		evaluateInNamespace   = req.EvaluateInNamespace
		evaluateInPartition   = req.EvaluateInPartition
		evaluateInDatacenter  = req.EvaluateInDatacenter
		evaluateInTrustDomain = req.EvaluateInTrustDomain
		entries               = req.Entries
	)
	if serviceName == "" {
		return nil, fmt.Errorf("serviceName is required")
	}
	if evaluateInNamespace == "" {
		return nil, fmt.Errorf("evaluateInNamespace is required")
	}
	if evaluateInPartition == "" {
		return nil, fmt.Errorf("evaluateInPartition is required")
	}
	if evaluateInDatacenter == "" {
		return nil, fmt.Errorf("evaluateInDatacenter is required")
	}
	if evaluateInTrustDomain == "" {
		return nil, fmt.Errorf("evaluateInTrustDomain is required")
	}
	if entries == nil {
		return nil, fmt.Errorf("entries is required")
	}

	c := &compiler{
		serviceName:            serviceName,
		evaluateInPartition:    evaluateInPartition,
		evaluateInNamespace:    evaluateInNamespace,
		evaluateInDatacenter:   evaluateInDatacenter,
		evaluateInTrustDomain:  evaluateInTrustDomain,
		overrideMeshGateway:    req.OverrideMeshGateway,
		overrideProtocol:       req.OverrideProtocol,
		overrideConnectTimeout: req.OverrideConnectTimeout,
		entries:                entries,

		resolvers:     make(map[structs.ServiceID]*structs.ServiceResolverConfigEntry),
		splitterNodes: make(map[string]*structs.DiscoveryGraphNode),
		resolveNodes:  make(map[string]*structs.DiscoveryGraphNode),

		nodes:           make(map[string]*structs.DiscoveryGraphNode),
		loadedTargets:   make(map[string]*structs.DiscoveryTarget),
		retainedTargets: make(map[string]struct{}),
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
	evaluateInNamespace    string
	evaluateInPartition    string
	evaluateInDatacenter   string
	evaluateInTrustDomain  string
	overrideMeshGateway    structs.MeshGatewayConfig
	overrideProtocol       string
	overrideConnectTimeout time.Duration

	// config entries that are being compiled (will be mutated during compilation)
	//
	// This is an INPUT field.
	entries *configentry.DiscoveryChainSet

	// resolvers is initially seeded by copying the provided entries.Resolvers
	// map and default resolvers are added as they are needed.
	resolvers map[structs.ServiceID]*structs.ServiceResolverConfigEntry

	// cached nodes
	splitterNodes map[string]*structs.DiscoveryGraphNode
	resolveNodes  map[string]*structs.DiscoveryGraphNode

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

	// serviceMeta is the Meta field from the service-defaults entry that
	// shares a name with this discovery chain.
	//
	// This is an OUTPUT field.
	serviceMeta map[string]string

	// envoyExtensions contains the Envoy Extensions configured through service defaults or proxy defaults config
	// entries for this discovery chain.
	//
	// This is an OUTPUT field.
	envoyExtensions []structs.EnvoyExtension

	// startNode is computed inside of assembleChain()
	//
	// This is an OUTPUT field.
	startNode string

	// nodes is computed inside of compile()
	//
	// This is an OUTPUT field.
	nodes map[string]*structs.DiscoveryGraphNode

	// This is an OUTPUT field.
	loadedTargets   map[string]*structs.DiscoveryTarget
	retainedTargets map[string]struct{}
}

type customizationMarkers struct {
	MeshGateway    bool
	Protocol       bool
	ConnectTimeout bool
}

// serviceIDString deviates from the standard formatting you would get with
// the String() method on the type itself. It is this way to be more
// consistent with other string ids within the discovery chain.
func serviceIDString(sid structs.ServiceID) string {
	return fmt.Sprintf("%s.%s.%s", sid.ID, sid.NamespaceOrDefault(), sid.PartitionOrDefault())
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
		c.splitterNodes[node.Name] = node
	case structs.DiscoveryGraphNodeTypeResolver:
		c.resolveNodes[node.Resolver.Target] = node
	default:
		panic("unknown node type '" + node.Type + "'")
	}

	c.nodes[node.MapKey()] = node
}

func (c *compiler) recordServiceProtocol(sid structs.ServiceID) error {
	if serviceDefault := c.entries.GetService(sid); serviceDefault != nil {
		return c.recordProtocol(sid, serviceDefault.Protocol)
	}
	if proxyDefault := c.entries.GetProxyDefaults(sid.PartitionOrDefault()); proxyDefault != nil {
		var cfg proxyConfig
		// Ignore errors and fallback on defaults if it does happen.
		_ = mapstructure.WeakDecode(proxyDefault.Config, &cfg)
		if cfg.Protocol != "" {
			return c.recordProtocol(sid, cfg.Protocol)
		}
	}
	return c.recordProtocol(sid, "")
}

// proxyConfig is a snippet from agent/xds/config.go:ProxyConfig
type proxyConfig struct {
	Protocol string `mapstructure:"protocol"`
}

func (c *compiler) recordProtocol(fromService structs.ServiceID, protocol string) error {
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
				c.serviceName, fromService.String(), protocol, c.protocol,
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

	if err := c.detectCircularReferences(); err != nil {
		return nil, err
	}

	if err := c.flattenAdjacentSplitterNodes(); err != nil {
		return nil, err
	}

	if err := c.removeUnusedNodes(); err != nil {
		return nil, err
	}

	for targetID := range c.loadedTargets {
		if _, ok := c.retainedTargets[targetID]; !ok {
			delete(c.loadedTargets, targetID)
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
		Namespace:         c.evaluateInNamespace,
		Partition:         c.evaluateInPartition,
		Datacenter:        c.evaluateInDatacenter,
		Default:           c.determineIfDefaultChain(),
		CustomizationHash: customizationHash,
		Protocol:          c.protocol,
		ServiceMeta:       c.serviceMeta,
		EnvoyExtensions:   c.envoyExtensions,
		StartNode:         c.startNode,
		Nodes:             c.nodes,
		Targets:           c.loadedTargets,
	}, nil
}

//	determineIfDefaultChain returns true if the compiled chain represents no
//	routing, no splitting, and only the default resolution.  We have to be
//	careful here to avoid returning "yep this is default" when the only
//	resolver action being applied is redirection to another resolver that is
//	default, so we double check the resolver matches the requested resolver.
//
// NOTE: "default chain" mostly means that this is compatible with how things
// worked (roughly) in consul 1.5 pre-discovery chain, not that there are zero
// config entries in play (like service-defaults).
func (c *compiler) determineIfDefaultChain() bool {
	if c.startNode == "" || len(c.nodes) == 0 {
		return true
	}

	node := c.nodes[c.startNode]
	if node == nil {
		panic("not possible: missing node named '" + c.startNode + "' in chain '" + c.serviceName + "'")
	}

	if node.Type != structs.DiscoveryGraphNodeTypeResolver {
		return false
	}
	if !node.Resolver.Default {
		return false
	}

	target := c.loadedTargets[node.Resolver.Target]

	return target.Service == c.serviceName && target.Namespace == c.evaluateInNamespace && target.Partition == c.evaluateInPartition
}

func (c *compiler) detectCircularReferences() error {
	var (
		todo       stringStack
		visited    = make(map[string]struct{})
		visitChain stringStack
	)

	todo.Push(c.startNode)
	for {
		current, ok := todo.Pop()
		if !ok {
			break
		}
		if current == "_popvisit" {
			if v, ok := visitChain.Pop(); ok {
				delete(visited, v)
			}
			continue
		}
		visitChain.Push(current)

		if _, ok := visited[current]; ok {
			return &structs.ConfigEntryGraphError{
				Message: fmt.Sprintf(
					"detected circular reference: [%s]",
					strings.Join(visitChain.Items(), " -> "),
				),
			}
		}
		visited[current] = struct{}{}

		todo.Push("_popvisit")

		node := c.nodes[current]

		switch node.Type {
		case structs.DiscoveryGraphNodeTypeRouter:
			for _, route := range node.Routes {
				todo.Push(route.NextNode)
			}
		case structs.DiscoveryGraphNodeTypeSplitter:
			for _, split := range node.Splits {
				todo.Push(split.NextNode)
			}
		case structs.DiscoveryGraphNodeTypeResolver:
			// Circular redirects are detected elsewhere and failover isn't
			// recursive so there's nothing more to do here.
		default:
			return fmt.Errorf("unexpected graph node type: %s", node.Type)
		}
	}

	return nil
}

func (c *compiler) flattenAdjacentSplitterNodes() error {
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

					// Copy the definition from the inner node but merge in the parent
					// to preserve any config it needs to pass through.
					newDef, err := innerSplit.Definition.MergeParent(split.Definition)
					if err != nil {
						return err
					}
					newDiscoverySplit := &structs.DiscoverySplit{
						Definition: newDef,
						Weight:     structs.NormalizeServiceSplitWeight(effectiveWeight),
						NextNode:   innerSplit.NextNode,
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
			return nil
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
		for k := range todo {
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

	for name := range c.nodes {
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

	sid := structs.NewServiceID(c.serviceName, c.GetEnterpriseMeta())

	// Extract extensions from proxy defaults.
	proxyDefaults := c.entries.GetProxyDefaults(c.GetEnterpriseMeta().PartitionOrDefault())
	if proxyDefaults != nil {
		c.envoyExtensions = proxyDefaults.EnvoyExtensions
	}

	// Extract the service meta for the service named by this discovery chain and add extensions from the service
	// defaults.
	if serviceDefault := c.entries.GetService(sid); serviceDefault != nil {
		c.serviceMeta = serviceDefault.GetMeta()
		c.envoyExtensions = append(c.envoyExtensions, serviceDefault.EnvoyExtensions...)
	}

	// Check for short circuit path.
	if len(c.resolvers) == 0 && c.entries.IsChainEmpty() {
		// Materialize defaults and cache.
		c.resolvers[sid] = newDefaultServiceResolver(sid)
	}

	// The only router we consult is the one for the service name at the top of
	// the chain.
	router := c.entries.GetRouter(sid)
	if router != nil && c.disableAdvancedRoutingFeatures {
		router = nil
		c.customizedBy.Protocol = true
	}

	if router == nil {
		// If no router is configured, move on down the line to the next hop of
		// the chain.
		node, err := c.getSplitterOrResolverNode(c.newTarget(structs.DiscoveryTargetOpts{
			Service: c.serviceName,
		}))

		if err != nil {
			return err
		}

		c.startNode = node.MapKey()
		return nil
	}

	routerID := structs.NewServiceID(router.Name, router.GetEnterpriseMeta())

	routeNode := &structs.DiscoveryGraphNode{
		Type:   structs.DiscoveryGraphNodeTypeRouter,
		Name:   serviceIDString(routerID),
		Routes: make([]*structs.DiscoveryRoute, 0, len(router.Routes)+1),
	}
	c.usesAdvancedRoutingFeatures = true
	if err := c.recordServiceProtocol(routerID); err != nil {
		return err
	}

	for i := range router.Routes {
		// We don't use range variables here because we'll take the address of
		// this route and store that in a DiscoveryGraphNode and the range
		// variables share memory addresses between iterations which is exactly
		// wrong for us here.
		route := router.Routes[i]

		compiledRoute := &structs.DiscoveryRoute{Definition: &route}
		routeNode.Routes = append(routeNode.Routes, compiledRoute)

		dest := route.Destination
		if dest == nil {
			dest = &structs.ServiceRouteDestination{
				Service:   c.serviceName,
				Namespace: router.NamespaceOrDefault(),
				Partition: router.PartitionOrDefault(),
			}
		}
		svc := defaultIfEmpty(dest.Service, c.serviceName)
		destNamespace := defaultIfEmpty(dest.Namespace, router.NamespaceOrDefault())
		destPartition := defaultIfEmpty(dest.Partition, router.PartitionOrDefault())

		// Check to see if the destination is eligible for splitting.
		var (
			node *structs.DiscoveryGraphNode
			err  error
		)
		if dest.ServiceSubset == "" {
			node, err = c.getSplitterOrResolverNode(
				c.newTarget(structs.DiscoveryTargetOpts{
					Service:   svc,
					Namespace: destNamespace,
					Partition: destPartition,
				},
				))
		} else {
			node, err = c.getResolverNode(
				c.newTarget(structs.DiscoveryTargetOpts{
					Service:       svc,
					ServiceSubset: dest.ServiceSubset,
					Namespace:     destNamespace,
					Partition:     destPartition,
				}),
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
	opts := structs.DiscoveryTargetOpts{
		Service:   router.Name,
		Namespace: router.NamespaceOrDefault(),
		Partition: router.PartitionOrDefault(),
	}
	defaultDestinationNode, err := c.getSplitterOrResolverNode(c.newTarget(opts))
	if err != nil {
		return err
	}

	defaultRoute := &structs.DiscoveryRoute{
		Definition: newDefaultServiceRoute(router.Name, router.NamespaceOrDefault(), router.PartitionOrDefault()),
		NextNode:   defaultDestinationNode.MapKey(),
	}
	routeNode.Routes = append(routeNode.Routes, defaultRoute)

	c.startNode = routeNode.MapKey()
	c.recordNode(routeNode)

	return nil
}

func newDefaultServiceRoute(serviceName, namespace, partition string) *structs.ServiceRoute {
	return &structs.ServiceRoute{
		Match: &structs.ServiceRouteMatch{
			HTTP: &structs.ServiceRouteHTTPMatch{
				PathPrefix: "/",
			},
		},
		Destination: &structs.ServiceRouteDestination{
			Service:   serviceName,
			Namespace: namespace,
			Partition: partition,
		},
	}
}

func (c *compiler) newTarget(opts structs.DiscoveryTargetOpts) *structs.DiscoveryTarget {
	if opts.Service == "" {
		panic("newTarget called with empty service which makes no sense")
	}

	if opts.Peer == "" {
		opts.Datacenter = defaultIfEmpty(opts.Datacenter, c.evaluateInDatacenter)
		opts.Namespace = defaultIfEmpty(opts.Namespace, c.evaluateInNamespace)
		opts.Partition = defaultIfEmpty(opts.Partition, c.evaluateInPartition)
	} else {
		// Don't allow Peer and Datacenter.
		opts.Datacenter = ""
		// Since discovery targets (for peering) are ONLY used to query the catalog, and
		// not to generate the SNI it is more correct to switch this to the calling-side
		// of the peering's partition as that matches where the replicated data is stored
		// in the catalog. This is done to simplify the usage of peer targets in both
		// the xds and proxycfg packages.
		//
		// The peer info data attached to service instances will have the embedded opaque
		// SNI/SAN information generated by the remote side and that will have the
		// OTHER partition properly specified.
		opts.Partition = acl.PartitionOrDefault(c.evaluateInPartition)
		// Default to "default" rather than c.evaluateInNamespace.
		// Note that the namespace is not swapped out, because it should
		// always match the value in the remote cluster (and shouldn't
		// have been changed anywhere).
		opts.Namespace = acl.NamespaceOrDefault(opts.Namespace)
	}

	t := structs.NewDiscoveryTarget(opts)

	// We don't have the peer's trust domain yet so we can't construct the SNI.
	if opts.Peer == "" {
		// Set default connect SNI. This will be overridden later if the service
		// has an explicit SNI value configured in service-defaults.
		t.SNI = connect.TargetSNI(t, c.evaluateInTrustDomain)

		// Use the same representation for the name. This will NOT be overridden
		// later.
		t.Name = t.SNI
	}

	prev, ok := c.loadedTargets[t.ID]
	if ok {
		return prev
	}
	c.loadedTargets[t.ID] = t
	return t
}

func (c *compiler) rewriteTarget(t *structs.DiscoveryTarget, opts structs.DiscoveryTargetOpts) *structs.DiscoveryTarget {
	mergedOpts := t.ToDiscoveryTargetOpts()

	if opts.Service != "" && opts.Service != mergedOpts.Service {
		mergedOpts.Service = opts.Service
		// Reset the chosen subset if we reference a service other than our own.
		mergedOpts.ServiceSubset = ""
	}
	if opts.ServiceSubset != "" {
		mergedOpts.ServiceSubset = opts.ServiceSubset
	}
	if opts.Partition != "" {
		mergedOpts.Partition = opts.Partition
	}
	// Only use explicit Namespace with Peer
	if opts.Namespace != "" || opts.Peer != "" {
		mergedOpts.Namespace = opts.Namespace
	}
	if opts.Datacenter != "" {
		mergedOpts.Datacenter = opts.Datacenter
	}
	mergedOpts.Peer = opts.Peer

	return c.newTarget(mergedOpts)
}

func (c *compiler) getSplitterOrResolverNode(target *structs.DiscoveryTarget) (*structs.DiscoveryGraphNode, error) {
	nextNode, err := c.getSplitterNode(target.ServiceID())
	if err != nil {
		return nil, err
	} else if nextNode != nil {
		return nextNode, nil
	}
	return c.getResolverNode(target, false)
}

func (c *compiler) getSplitterNode(sid structs.ServiceID) (*structs.DiscoveryGraphNode, error) {
	name := serviceIDString(sid)
	// Do we already have the node?
	if prev, ok := c.splitterNodes[name]; ok {
		return prev, nil
	}

	// Fetch the config entry.
	splitter := c.entries.GetSplitter(sid)
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
	// reasonably if there is some sort of graph loop below.
	c.recordNode(splitNode)

	var hasLB bool
	for i := range splitter.Splits {
		// We don't use range variables here because we'll take the address of
		// this split and store that in a DiscoveryGraphNode and the range
		// variables share memory addresses between iterations which is exactly
		// wrong for us here.
		split := splitter.Splits[i]

		compiledSplit := &structs.DiscoverySplit{
			Definition: &split,
			Weight:     split.Weight,
		}
		splitNode.Splits = append(splitNode.Splits, compiledSplit)

		svc := defaultIfEmpty(split.Service, sid.ID)
		splitID := structs.ServiceID{
			ID:             svc,
			EnterpriseMeta: *split.GetEnterpriseMeta(&sid.EnterpriseMeta),
		}

		// Check to see if the split is eligible for additional splitting.
		if !splitID.Matches(sid) && split.ServiceSubset == "" {
			nextNode, err := c.getSplitterNode(splitID)
			if err != nil {
				return nil, err
			} else if nextNode != nil {
				compiledSplit.NextNode = nextNode.MapKey()
				continue
			}
			// fall through to group-resolver
		}

		opts := structs.DiscoveryTargetOpts{
			Service:       splitID.ID,
			ServiceSubset: split.ServiceSubset,
			Namespace:     splitID.NamespaceOrDefault(),
			Partition:     splitID.PartitionOrDefault(),
		}
		node, err := c.getResolverNode(c.newTarget(opts), false)
		if err != nil {
			return nil, err
		}
		compiledSplit.NextNode = node.MapKey()

		// There exists the possibility that a splitter may split between two distinct service names
		// with distinct hash-based load balancer configs specified in their service resolvers.
		// We cannot apply multiple hash policies to a splitter node's route action.
		// Therefore, we attach the first hash-based load balancer config we encounter.
		if !hasLB {
			if lb := node.LoadBalancer; lb != nil && lb.IsHashBased() {
				splitNode.LoadBalancer = node.LoadBalancer
				hasLB = true
			}
		}
	}

	c.usesAdvancedRoutingFeatures = true
	return splitNode, nil
}

// getResolverNode handles most of the code to handle redirection/rewriting
// capabilities from a resolver config entry. It recurses into itself to
// _generate_ targets used for failover out of convenience.
func (c *compiler) getResolverNode(target *structs.DiscoveryTarget, recursedForFailover bool) (*structs.DiscoveryGraphNode, error) {
	var (
		// State to help detect redirect cycles and print helpful error
		// messages.
		redirectHistory = make(map[string]struct{})
		redirectOrder   []string
	)

RESOLVE_AGAIN:
	// Do we already have the node?
	if prev, ok := c.resolveNodes[target.ID]; ok {
		return prev, nil
	}

	targetID := target.ServiceID()

	// Only validate protocol if it is not a peered service.
	// TODO: Add in remote peer protocol validation when building a chain.
	// This likely would require querying the imported services, which
	// shouldn't belong in the discovery chain compilation here.
	if target.Peer == "" {
		if err := c.recordServiceProtocol(targetID); err != nil {
			return nil, err
		}
	}

	// Fetch the config entry.
	resolver, ok := c.resolvers[targetID]
	if !ok {
		// Materialize defaults and cache.
		resolver = newDefaultServiceResolver(targetID)
		c.resolvers[targetID] = resolver
	}

	if _, ok := redirectHistory[target.ID]; ok {
		redirectOrder = append(redirectOrder, target.ID)

		return nil, &structs.ConfigEntryGraphError{
			Message: fmt.Sprintf(
				"detected circular resolver redirect: [%s]",
				strings.Join(redirectOrder, " -> "),
			),
		}
	}
	redirectHistory[target.ID] = struct{}{}
	redirectOrder = append(redirectOrder, target.ID)

	// Handle redirects right up front.
	//
	// TODO(rb): What about a redirected subset reference? (web/v2, but web redirects to alt/"")

	if resolver.Redirect != nil {
		redirect := resolver.Redirect

		redirectedTarget := c.rewriteTarget(
			target,
			redirect.ToDiscoveryTargetOpts(),
		)
		if redirectedTarget.ID != target.ID {
			target = redirectedTarget
			goto RESOLVE_AGAIN
		}
	}

	// Handle default subset.
	if target.ServiceSubset == "" && resolver.DefaultSubset != "" {
		target = c.rewriteTarget(target, structs.DiscoveryTargetOpts{
			ServiceSubset: resolver.DefaultSubset,
		})
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

	// Expose a copy of this on the targets for ease of access.
	target.ConnectTimeout = connectTimeout

	// Build node.
	node := &structs.DiscoveryGraphNode{
		Type: structs.DiscoveryGraphNodeTypeResolver,
		Name: target.ID,
		Resolver: &structs.DiscoveryResolver{
			Default:        resolver.IsDefault(),
			Target:         target.ID,
			ConnectTimeout: connectTimeout,
			RequestTimeout: resolver.RequestTimeout,
		},
		LoadBalancer: resolver.LoadBalancer,
	}

	target.Subset = resolver.Subsets[target.ServiceSubset]

	if serviceDefault := c.entries.GetService(targetID); serviceDefault != nil && serviceDefault.ExternalSNI != "" {
		// Override the default SNI value.
		target.SNI = serviceDefault.ExternalSNI
		target.External = true
	}

	// If using external SNI the service is fundamentally external.
	if target.External {
		if resolver.Redirect != nil {
			return nil, &structs.ConfigEntryGraphError{
				Message: fmt.Sprintf(
					"service %q has an external SNI set; cannot define redirects for external services",
					target.Service,
				),
			}
		}
		if len(resolver.Subsets) > 0 {
			return nil, &structs.ConfigEntryGraphError{
				Message: fmt.Sprintf(
					"service %q has an external SNI set; cannot define subsets for external services",
					target.Service,
				),
			}
		}
		if len(resolver.Failover) > 0 {
			return nil, &structs.ConfigEntryGraphError{
				Message: fmt.Sprintf(
					"service %q has an external SNI set; cannot define failover for external services",
					target.Service,
				),
			}
		}
	}

	// TODO(partitions): Document this change in behavior. Discovery chain targets will now return a mesh gateway
	// 					 mode as long as they are not external. Regardless of the datacenter/partition where
	// 					 the chain will be used.
	if target.External {
		// Bypass mesh gateways if it is an external service.
		target.MeshGateway.Mode = structs.MeshGatewayModeDefault
	} else {
		// Default mesh gateway settings
		if serviceDefault := c.entries.GetService(targetID); serviceDefault != nil {
			target.MeshGateway = serviceDefault.MeshGateway
			target.TransparentProxy.DialedDirectly = serviceDefault.TransparentProxy.DialedDirectly
		}
		proxyDefault := c.entries.GetProxyDefaults(targetID.PartitionOrDefault())
		if proxyDefault != nil {
			if target.MeshGateway.Mode == structs.MeshGatewayModeDefault {
				target.MeshGateway.Mode = proxyDefault.MeshGateway.Mode
			}
			if !target.TransparentProxy.DialedDirectly {
				target.TransparentProxy.DialedDirectly = proxyDefault.TransparentProxy.DialedDirectly
			}
		}

		if c.overrideMeshGateway.Mode != structs.MeshGatewayModeDefault {
			if target.MeshGateway.Mode != c.overrideMeshGateway.Mode {
				target.MeshGateway.Mode = c.overrideMeshGateway.Mode
				c.customizedBy.MeshGateway = true
			}
		}
	}

	// Retain this target in the final results.
	c.retainedTargets[target.ID] = struct{}{}

	if recursedForFailover {
		// If we recursed here from ourselves in a failover context, just emit
		// this node without caching it or even processing failover again.
		// This is a little weird but it keeps the redirect/default-subset
		// logic in one place.
		return node, nil
	}

	// If we record this exists before recursing down it will short-circuit
	// reasonably if there is some sort of graph loop below.
	c.recordNode(node)

	if len(resolver.Failover) > 0 {
		f := resolver.Failover

		// Determine which failover section applies.
		failover, ok := f[target.ServiceSubset]
		if !ok {
			failover, ok = f["*"]
		}

		if !ok {
			return node, nil
		}

		// Determine which failover definitions apply.
		var failoverTargets []*structs.DiscoveryTarget
		if len(failover.Datacenters) > 0 {
			opts := failover.ToDiscoveryTargetOpts()
			for _, dc := range failover.Datacenters {
				// Rewrite the target as per the failover policy.
				opts.Datacenter = dc
				failoverTarget := c.rewriteTarget(target, opts)
				if failoverTarget.ID != target.ID { // don't failover to yourself
					failoverTargets = append(failoverTargets, failoverTarget)
				}
			}
		} else if len(failover.Targets) > 0 {
			for _, t := range failover.Targets {
				// Rewrite the target as per the failover policy.
				failoverTarget := c.rewriteTarget(target, t.ToDiscoveryTargetOpts())
				if failoverTarget.ID != target.ID { // don't failover to yourself
					failoverTargets = append(failoverTargets, failoverTarget)
				}
			}
		} else {
			// Rewrite the target as per the failover policy.
			failoverTarget := c.rewriteTarget(target, failover.ToDiscoveryTargetOpts())
			if failoverTarget.ID != target.ID { // don't failover to yourself
				failoverTargets = append(failoverTargets, failoverTarget)
			}
		}

		// If we filtered everything out then no point in having a failover.
		if len(failoverTargets) > 0 {
			df := &structs.DiscoveryFailover{}
			node.Resolver.Failover = df

			// Take care of doing any redirects or configuration loading
			// related to targets by cheating a bit and recursing into
			// ourselves.
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

	return node, nil
}

func newDefaultServiceResolver(sid structs.ServiceID) *structs.ServiceResolverConfigEntry {
	return &structs.ServiceResolverConfigEntry{
		Kind:           structs.ServiceResolver,
		Name:           sid.ID,
		EnterpriseMeta: sid.EnterpriseMeta,
	}
}

func defaultIfEmpty(val, defaultVal string) string {
	if val != "" {
		return val
	}
	return defaultVal
}

func enableAdvancedRoutingForProtocol(protocol string) bool {
	return structs.IsProtocolHTTPLike(protocol)
}
