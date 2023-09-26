// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package builder

import (
	"fmt"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/envoyextensions/xdscommon"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/mesh/internal/types/intermediate"
	"github.com/hashicorp/consul/internal/protoutil"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbmesh/v2beta1/pbproxystate"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// BuildDestinations creates listeners, routers, clusters, and endpointRefs for all destinations
// and adds them to the proxyState.
func (b *Builder) BuildDestinations(destinations []*intermediate.Destination) *Builder {
	var lb *ListenerBuilder
	if b.proxyCfg.IsTransparentProxy() {
		lb = b.addTransparentProxyOutboundListener(b.proxyCfg.DynamicConfig.TransparentProxy.OutboundListenerPort)
	}

	for _, destination := range destinations {
		b.buildDestination(lb, destination)
	}

	if b.proxyCfg.IsTransparentProxy() {
		lb.buildListener()
	}

	return b
}

func (b *Builder) buildDestination(
	tproxyOutboundListenerBuilder *ListenerBuilder,
	destination *intermediate.Destination,
) *Builder {
	var (
		effectiveProtocol = destination.ComputedPortRoutes.Protocol
		targets           = destination.ComputedPortRoutes.Targets
	)

	cpr := destination.ComputedPortRoutes

	var lb *ListenerBuilder
	if destination.Explicit != nil {
		lb = b.addExplicitOutboundListener(destination.Explicit)
	} else {
		lb = tproxyOutboundListenerBuilder
	}

	// router matches based on destination ports should only occur on
	// implicit destinations for explicit
	var virtualPortNumber uint32
	if destination.Explicit == nil {
		for _, port := range destination.Service.Data.Ports {
			if port.TargetPort == cpr.ParentRef.Port {
				virtualPortNumber = port.VirtualPort
			}
		}
	}

	defaultDC := func(dc string) string {
		if destination.Explicit != nil {
			dc = orDefault(dc, destination.Explicit.Datacenter)
		}
		dc = orDefault(dc, b.localDatacenter)
		if dc != b.localDatacenter {
			panic("cross datacenter service discovery clusters are not supported in v2")
		}
		return dc
	}

	statPrefix := DestinationStatPrefix(
		cpr.ParentRef.Ref,
		cpr.ParentRef.Port,
		defaultDC(""),
	)

	var (
		useRDS                bool
		needsNullRouteCluster bool
	)
	switch config := cpr.Config.(type) {
	case *pbmesh.ComputedPortRoutes_Http:
		// NOTE: this could be HTTP/HTTP2/GRPC
		useRDS = true

		route := config.Http

		listenerName := lb.listener.Name

		// this corresponds to roughly "makeUpstreamRouteForDiscoveryChain"

		var proxyRouteRules []*pbproxystate.RouteRule
		for _, routeRule := range route.Rules {
			for _, backendRef := range routeRule.BackendRefs {
				if backendRef.BackendTarget == types.NullRouteBackend {
					needsNullRouteCluster = true
				}
			}
			destConfig := b.makeDestinationConfiguration(routeRule.Timeouts, routeRule.Retries)
			headerMutations := applyRouteFilters(destConfig, routeRule.Filters)
			applyLoadBalancerPolicy(destConfig, cpr, routeRule.BackendRefs)

			dest := b.makeHTTPRouteDestination(
				routeRule.BackendRefs,
				destConfig,
				targets,
				defaultDC,
			)

			// Explode out by matches
			for _, match := range routeRule.Matches {
				routeMatch := makeHTTPRouteMatch(match)

				proxyRouteRules = append(proxyRouteRules, &pbproxystate.RouteRule{
					Match:           routeMatch,
					Destination:     protoutil.Clone(dest),
					HeaderMutations: protoutil.CloneSlice(headerMutations),
				})
			}
		}

		b.addRoute(listenerName, &pbproxystate.Route{
			VirtualHosts: []*pbproxystate.VirtualHost{{
				Name:       listenerName,
				RouteRules: proxyRouteRules,
			}},
		})

	case *pbmesh.ComputedPortRoutes_Grpc:
		useRDS = true
		route := config.Grpc

		listenerName := lb.listener.Name

		var proxyRouteRules []*pbproxystate.RouteRule
		for _, routeRule := range route.Rules {
			for _, backendRef := range routeRule.BackendRefs {
				if backendRef.BackendTarget == types.NullRouteBackend {
					needsNullRouteCluster = true
				}
			}
			destConfig := b.makeDestinationConfiguration(routeRule.Timeouts, routeRule.Retries)
			headerMutations := applyRouteFilters(destConfig, routeRule.Filters)
			applyLoadBalancerPolicy(destConfig, cpr, routeRule.BackendRefs)

			// nolint:staticcheck
			dest := b.makeGRPCRouteDestination(
				routeRule.BackendRefs,
				destConfig,
				targets,
				defaultDC,
			)

			// Explode out by matches
			for _, match := range routeRule.Matches {
				routeMatch := makeGRPCRouteMatch(match)

				proxyRouteRules = append(proxyRouteRules, &pbproxystate.RouteRule{
					Match:           routeMatch,
					Destination:     protoutil.Clone(dest),
					HeaderMutations: protoutil.CloneSlice(headerMutations),
				})
			}
		}

		b.addRoute(listenerName, &pbproxystate.Route{
			VirtualHosts: []*pbproxystate.VirtualHost{{
				Name:       listenerName,
				RouteRules: proxyRouteRules,
			}},
		})

	case *pbmesh.ComputedPortRoutes_Tcp:
		route := config.Tcp
		useRDS = false

		if len(route.Rules) != 1 {
			panic("not possible due to validation and computation")
		}

		// When not using RDS we must generate a cluster name to attach to
		// the filter chain. With RDS, cluster names get attached to the
		// dynamic routes instead.

		routeRule := route.Rules[0]

		for _, backendRef := range routeRule.BackendRefs {
			if backendRef.BackendTarget == types.NullRouteBackend {
				needsNullRouteCluster = true
			}
		}

		switch len(routeRule.BackendRefs) {
		case 0:
			panic("not possible to have a tcp route rule with no backend refs")
		case 1:
			tcpBackendRef := routeRule.BackendRefs[0]

			clusterName := b.backendTargetToClusterName(tcpBackendRef.BackendTarget, targets, defaultDC)

			rb := lb.addL4RouterForDirect(clusterName, statPrefix)
			if destination.Explicit == nil {
				rb.addIPAndPortMatch(destination.VirtualIPs, virtualPortNumber)
			}
			rb.buildRouter()
		default:
			clusters := make([]*pbproxystate.L4WeightedDestinationCluster, 0, len(routeRule.BackendRefs))
			for _, tcpBackendRef := range routeRule.BackendRefs {
				clusterName := b.backendTargetToClusterName(tcpBackendRef.BackendTarget, targets, defaultDC)

				clusters = append(clusters, &pbproxystate.L4WeightedDestinationCluster{
					Name:   clusterName,
					Weight: wrapperspb.UInt32(tcpBackendRef.Weight),
				})
			}

			rb := lb.addL4RouterForSplit(clusters, statPrefix)
			if destination.Explicit == nil {
				rb.addIPAndPortMatch(destination.VirtualIPs, virtualPortNumber)
			}
			rb.buildRouter()
		}
	}

	if useRDS {
		if !isProtocolHTTPLike(effectiveProtocol) {
			panic(fmt.Sprintf("it should not be possible to have a tcp protocol here: %v", effectiveProtocol))
		}

		rb := lb.addL7Router("", effectiveProtocol)
		if destination.Explicit == nil {
			rb.addIPMatch(destination.VirtualIPs)
		}
		rb.buildRouter()
	} else {
		if isProtocolHTTPLike(effectiveProtocol) {
			panic(fmt.Sprintf("it should not be possible to have an http-like protocol here: %v", effectiveProtocol))
		}
	}

	// Build outbound listener if the destination is explicit.
	if destination.Explicit != nil {
		lb.buildListener()
	}

	if needsNullRouteCluster {
		b.addNullRouteCluster()
	}

	for _, details := range targets {
		// NOTE: we only emit clusters for DIRECT targets here. The others will
		// be folded into one or more aggregate clusters somehow.
		if details.Type != pbmesh.BackendTargetDetailsType_BACKEND_TARGET_DETAILS_TYPE_DIRECT {
			continue
		}

		connectTimeout := details.DestinationConfig.ConnectTimeout
		loadBalancer := details.DestinationConfig.LoadBalancer

		// NOTE: we collect both DIRECT and INDIRECT target information here.
		dc := defaultDC(details.BackendRef.Datacenter)
		portName := details.BackendRef.Port

		sni := DestinationSNI(
			details.BackendRef.Ref,
			dc,
			b.trustDomain,
		)
		clusterName := fmt.Sprintf("%s.%s", portName, sni)

		egBase := b.newClusterEndpointGroup("", sni, portName, details.IdentityRefs, connectTimeout, loadBalancer)

		var endpointGroups []*pbproxystate.EndpointGroup

		// Original target is the first (or only) target.
		endpointGroups = append(endpointGroups, egBase)
		b.addEndpointsRef(clusterName, details.ServiceEndpointsId, details.MeshPort)

		if details.FailoverConfig != nil {
			failover := details.FailoverConfig
			// TODO(v2): handle other forms of failover (regions/locality/etc)

			for i, dest := range failover.Destinations {
				if dest.BackendTarget == types.NullRouteBackend {
					continue // not possible
				}
				destDetails, ok := targets[dest.BackendTarget]
				if !ok {
					continue // not possible
				}

				destConnectTimeout := destDetails.DestinationConfig.ConnectTimeout
				destLoadBalancer := destDetails.DestinationConfig.LoadBalancer

				destDC := defaultDC(destDetails.BackendRef.Datacenter)
				destPortName := destDetails.BackendRef.Port

				destSNI := DestinationSNI(
					destDetails.BackendRef.Ref,
					destDC,
					b.trustDomain,
				)
				destClusterName := fmt.Sprintf("%s%d~%s", xdscommon.FailoverClusterNamePrefix, i, clusterName)

				egDest := b.newClusterEndpointGroup(destClusterName, destSNI, destPortName, destDetails.IdentityRefs, destConnectTimeout, destLoadBalancer)

				endpointGroups = append(endpointGroups, egDest)
				b.addEndpointsRef(destClusterName, destDetails.ServiceEndpointsId, destDetails.MeshPort)
			}
		}

		b.addCluster(clusterName, endpointGroups, connectTimeout)
	}

	return b
}

const NullRouteClusterName = "null_route_cluster"

func (b *Builder) addNullRouteCluster() *Builder {
	cluster := &pbproxystate.Cluster{
		Name: NullRouteClusterName,
		Group: &pbproxystate.Cluster_EndpointGroup{
			EndpointGroup: &pbproxystate.EndpointGroup{
				Group: &pbproxystate.EndpointGroup_Static{
					Static: &pbproxystate.StaticEndpointGroup{
						Config: &pbproxystate.StaticEndpointGroupConfig{
							ConnectTimeout: durationpb.New(10 * time.Second),
						},
					},
				},
			},
		},
	}

	b.proxyStateTemplate.ProxyState.Clusters[cluster.Name] = cluster
	return b
}

func (b *ListenerBuilder) addL4RouterForDirect(clusterName, statPrefix string) *RouterBuilder {
	// For explicit destinations, we have no filter chain match, and filters
	// are based on port protocol.
	router := &pbproxystate.Router{}

	if statPrefix == "" {
		statPrefix = "upstream."
	}

	router.Destination = &pbproxystate.Router_L4{
		L4: &pbproxystate.L4Destination{
			Destination: &pbproxystate.L4Destination_Cluster{
				Cluster: &pbproxystate.DestinationCluster{
					Name: clusterName,
				},
			},
			StatPrefix: statPrefix,
		},
	}

	return b.NewRouterBuilder(router)
}

func (b *ListenerBuilder) addL4RouterForSplit(
	clusters []*pbproxystate.L4WeightedDestinationCluster,
	statPrefix string,
) *RouterBuilder {
	// For explicit destinations, we have no filter chain match, and filters
	// are based on port protocol.
	router := &pbproxystate.Router{}

	if statPrefix == "" {
		statPrefix = "upstream."
	}

	router.Destination = &pbproxystate.Router_L4{
		L4: &pbproxystate.L4Destination{
			Destination: &pbproxystate.L4Destination_WeightedClusters{
				WeightedClusters: &pbproxystate.L4WeightedClusterGroup{
					Clusters: clusters,
				},
			},
			StatPrefix: statPrefix,
			// TODO(rb/v2): can we use RDS for TCPRoute split?
		},
	}

	return b.NewRouterBuilder(router)
}

func (b *ListenerBuilder) addL7Router(statPrefix string, protocol pbcatalog.Protocol) *RouterBuilder {
	// For explicit destinations, we have no filter chain match, and filters
	// are based on port protocol.
	router := &pbproxystate.Router{}

	listenerName := b.listener.Name
	if listenerName == "" {
		panic("listenerName is required")
	}

	if statPrefix == "" {
		statPrefix = "upstream."
	}

	if !isProtocolHTTPLike(protocol) {
		panic(fmt.Sprintf("unexpected protocol: %v", protocol))
	}

	router.Destination = &pbproxystate.Router_L7{
		L7: &pbproxystate.L7Destination{
			Name:        listenerName,
			StatPrefix:  statPrefix,
			StaticRoute: false,
		},
	}

	return b.NewRouterBuilder(router)
}

// addExplicitOutboundListener creates an outbound listener for an explicit destination.
func (b *Builder) addExplicitOutboundListener(explicit *pbmesh.Destination) *ListenerBuilder {
	listener := makeExplicitListener(explicit, pbproxystate.Direction_DIRECTION_OUTBOUND)

	return b.NewListenerBuilder(listener)
}

func makeExplicitListener(explicit *pbmesh.Destination, direction pbproxystate.Direction) *pbproxystate.Listener {
	if explicit == nil {
		panic("explicit upstream required")
	}

	listener := &pbproxystate.Listener{
		Direction: direction,
	}

	// TODO(v2): access logs, connection balancing

	// Create outbound listener address.
	switch explicit.ListenAddr.(type) {
	case *pbmesh.Destination_IpPort:
		destinationAddr := explicit.ListenAddr.(*pbmesh.Destination_IpPort)
		listener.BindAddress = &pbproxystate.Listener_HostPort{
			HostPort: &pbproxystate.HostPortAddress{
				Host: destinationAddr.IpPort.Ip,
				Port: destinationAddr.IpPort.Port,
			},
		}
		listener.Name = DestinationListenerName(explicit.DestinationRef.Name, explicit.DestinationPort, destinationAddr.IpPort.Ip, destinationAddr.IpPort.Port)
	case *pbmesh.Destination_Unix:
		destinationAddr := explicit.ListenAddr.(*pbmesh.Destination_Unix)
		listener.BindAddress = &pbproxystate.Listener_UnixSocket{
			UnixSocket: &pbproxystate.UnixSocketAddress{
				Path: destinationAddr.Unix.Path,
				Mode: destinationAddr.Unix.Mode,
			},
		}
		listener.Name = DestinationListenerName(explicit.DestinationRef.Name, explicit.DestinationPort, destinationAddr.Unix.Path, 0)
	}

	return listener
}

// addTransparentProxyOutboundListener creates an outbound listener for transparent proxy mode.
func (b *Builder) addTransparentProxyOutboundListener(port uint32) *ListenerBuilder {
	listener := &pbproxystate.Listener{
		Name:      xdscommon.OutboundListenerName,
		Direction: pbproxystate.Direction_DIRECTION_OUTBOUND,
		BindAddress: &pbproxystate.Listener_HostPort{
			HostPort: &pbproxystate.HostPortAddress{
				Host: "127.0.0.1",
				Port: port,
			},
		},
		Capabilities: []pbproxystate.Capability{pbproxystate.Capability_CAPABILITY_TRANSPARENT},
	}

	return b.NewListenerBuilder(listener)
}

func isProtocolHTTPLike(protocol pbcatalog.Protocol) bool {
	// enumcover:pbcatalog.Protocol
	switch protocol {
	case pbcatalog.Protocol_PROTOCOL_TCP:
		return false
	case pbcatalog.Protocol_PROTOCOL_HTTP2,
		pbcatalog.Protocol_PROTOCOL_HTTP,
		pbcatalog.Protocol_PROTOCOL_GRPC:
		return true
	case pbcatalog.Protocol_PROTOCOL_MESH:
		fallthrough // to default
	case pbcatalog.Protocol_PROTOCOL_UNSPECIFIED:
		fallthrough // to default
	default:
		return false
	}
}

func (b *RouterBuilder) addIPMatch(vips []string) *RouterBuilder {
	return b.addIPAndPortMatch(vips, 0)
}

func (b *RouterBuilder) addIPAndPortMatch(vips []string, virtualPort uint32) *RouterBuilder {
	b.router.Match = makeRouterMatchForIPAndPort(vips, virtualPort)
	return b
}

func makeRouterMatchForIPAndPort(vips []string, virtualPort uint32) *pbproxystate.Match {
	match := &pbproxystate.Match{}
	for _, vip := range vips {
		match.PrefixRanges = append(match.PrefixRanges, &pbproxystate.CidrRange{
			AddressPrefix: vip,
			PrefixLen:     &wrapperspb.UInt32Value{Value: 32},
		})

		if virtualPort > 0 {
			match.DestinationPort = &wrapperspb.UInt32Value{Value: virtualPort}
		}
	}
	return match
}

// addCluster creates and adds a cluster to the proxyState based on the destination.
func (b *Builder) addCluster(
	clusterName string,
	endpointGroups []*pbproxystate.EndpointGroup,
	connectTimeout *durationpb.Duration,
) {
	cluster := &pbproxystate.Cluster{
		Name:        clusterName,
		AltStatName: clusterName,
	}
	switch len(endpointGroups) {
	case 0:
		panic("no endpoint groups provided")
	case 1:
		cluster.Group = &pbproxystate.Cluster_EndpointGroup{
			EndpointGroup: endpointGroups[0],
		}
	default:
		cluster.Group = &pbproxystate.Cluster_FailoverGroup{
			FailoverGroup: &pbproxystate.FailoverGroup{
				EndpointGroups: endpointGroups,
				Config: &pbproxystate.FailoverGroupConfig{
					UseAltStatName: true,
					ConnectTimeout: connectTimeout,
				},
			},
		}
	}

	b.proxyStateTemplate.ProxyState.Clusters[cluster.Name] = cluster
}

func (b *Builder) newClusterEndpointGroup(
	clusterName string,
	sni string,
	portName string,
	destinationIdentities []*pbresource.Reference,
	connectTimeout *durationpb.Duration,
	loadBalancer *pbmesh.LoadBalancer,
) *pbproxystate.EndpointGroup {
	var spiffeIDs []string
	for _, identity := range destinationIdentities {
		spiffeIDs = append(spiffeIDs, connect.SpiffeIDFromIdentityRef(b.trustDomain, identity))
	}

	// TODO(v2): DestinationPolicy: circuit breakers, outlier detection

	// TODO(v2): if http2/grpc then set http2protocol options

	degConfig := &pbproxystate.DynamicEndpointGroupConfig{
		DisablePanicThreshold: true,
		ConnectTimeout:        connectTimeout,
	}

	if loadBalancer != nil {
		// enumcover:pbmesh.LoadBalancerPolicy
		switch loadBalancer.Policy {
		case pbmesh.LoadBalancerPolicy_LOAD_BALANCER_POLICY_RANDOM:
			degConfig.LbPolicy = &pbproxystate.DynamicEndpointGroupConfig_Random{}

		case pbmesh.LoadBalancerPolicy_LOAD_BALANCER_POLICY_ROUND_ROBIN:
			degConfig.LbPolicy = &pbproxystate.DynamicEndpointGroupConfig_RoundRobin{}

		case pbmesh.LoadBalancerPolicy_LOAD_BALANCER_POLICY_LEAST_REQUEST:
			var choiceCount uint32
			cfg, ok := loadBalancer.Config.(*pbmesh.LoadBalancer_LeastRequestConfig)
			if ok {
				choiceCount = cfg.LeastRequestConfig.GetChoiceCount()
			}
			degConfig.LbPolicy = &pbproxystate.DynamicEndpointGroupConfig_LeastRequest{
				LeastRequest: &pbproxystate.LBPolicyLeastRequest{
					ChoiceCount: wrapperspb.UInt32(choiceCount),
				},
			}

		case pbmesh.LoadBalancerPolicy_LOAD_BALANCER_POLICY_MAGLEV:
			degConfig.LbPolicy = &pbproxystate.DynamicEndpointGroupConfig_Maglev{}

		case pbmesh.LoadBalancerPolicy_LOAD_BALANCER_POLICY_RING_HASH:
			policy := &pbproxystate.DynamicEndpointGroupConfig_RingHash{}

			cfg, ok := loadBalancer.Config.(*pbmesh.LoadBalancer_RingHashConfig)
			if ok {
				policy.RingHash = &pbproxystate.LBPolicyRingHash{
					MinimumRingSize: wrapperspb.UInt64(cfg.RingHashConfig.MinimumRingSize),
					MaximumRingSize: wrapperspb.UInt64(cfg.RingHashConfig.MaximumRingSize),
				}
			}

			degConfig.LbPolicy = policy

		case pbmesh.LoadBalancerPolicy_LOAD_BALANCER_POLICY_UNSPECIFIED:
			// fallthrough to default
		default:
			// do nothing
		}
	}

	return &pbproxystate.EndpointGroup{
		Name: clusterName,
		Group: &pbproxystate.EndpointGroup_Dynamic{
			Dynamic: &pbproxystate.DynamicEndpointGroup{
				Config: degConfig,
				OutboundTls: &pbproxystate.TransportSocket{
					ConnectionTls: &pbproxystate.TransportSocket_OutboundMesh{
						OutboundMesh: &pbproxystate.OutboundMeshMTLS{
							IdentityKey: b.proxyStateTemplate.ProxyState.Identity.Name,
							ValidationContext: &pbproxystate.MeshOutboundValidationContext{
								SpiffeIds:              spiffeIDs,
								TrustBundlePeerNameKey: b.id.Tenancy.PeerName,
							},
							Sni: sni,
						},
					},
					AlpnProtocols: []string{getAlpnProtocolFromPortName(portName)},
				},
			},
		},
	}
}

func (b *Builder) addRoute(listenerName string, route *pbproxystate.Route) {
	b.proxyStateTemplate.ProxyState.Routes[listenerName] = route
}

// addEndpointsRef creates and add an endpointRef for each serviceEndpoint for a destination and
// adds it to the proxyStateTemplate so it will be processed later during reconciliation by
// the XDS controller.
func (b *Builder) addEndpointsRef(clusterName string, serviceEndpointsID *pbresource.ID, destinationPort string) {
	b.proxyStateTemplate.RequiredEndpoints[clusterName] = &pbproxystate.EndpointRef{
		Id:   serviceEndpointsID,
		Port: destinationPort,
	}
}

func orDefault(v, def string) string {
	if v != "" {
		return v
	}
	return def
}
