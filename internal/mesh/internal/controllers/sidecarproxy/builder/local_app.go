// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package builder

import (
	"fmt"

	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/envoyextensions/xdscommon"
	"github.com/hashicorp/consul/internal/resource"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbmesh/v2beta1/pbproxystate"
)

func (b *Builder) BuildLocalApp(workload *pbcatalog.Workload, ctp *pbauth.ComputedTrafficPermissions) *Builder {
	// Add the public listener.
	lb := b.addInboundListener(xdscommon.PublicListenerName, workload)
	lb.buildListener()

	trafficPermissions := buildTrafficPermissions(b.defaultAllow, b.trustDomain, workload, ctp)

	// Go through workload ports and add the routers, clusters, endpoints, and TLS.
	// Note that the order of ports is non-deterministic here but the xds generation
	// code should make sure to send it in the same order to Envoy to avoid unnecessary
	// updates.
	foundInboundNonMeshPorts := false
	for portName, port := range workload.Ports {
		clusterName := fmt.Sprintf("%s:%s", xdscommon.LocalAppClusterName, portName)
		routeName := fmt.Sprintf("%s:%s", lb.listener.Name, portName)

		if port.Protocol != pbcatalog.Protocol_PROTOCOL_MESH {
			foundInboundNonMeshPorts = true
			lb.addInboundRouter(clusterName, routeName, port, portName, trafficPermissions[portName], b.proxyCfg.GetDynamicConfig().GetInboundConnections()).
				addInboundTLS()

			if isL7(port.Protocol) {
				b.addLocalAppRoute(routeName, clusterName, portName)
			}
			b.addLocalAppCluster(clusterName, &portName, pbproxystate.Protocol(port.Protocol)).
				addLocalAppStaticEndpoints(clusterName, port.GetPort())
		}
	}

	b.buildExposePaths(workload)

	// If there are no inbound ports other than the mesh port, we black-hole all inbound traffic.
	if !foundInboundNonMeshPorts {
		lb.addBlackHoleRouter()
		b.addBlackHoleCluster()
	}

	return b
}

func buildTrafficPermissions(globalDefaultAllow bool, trustDomain string, workload *pbcatalog.Workload, computed *pbauth.ComputedTrafficPermissions) map[string]*pbproxystate.TrafficPermissions {
	portsWithProtocol := workload.GetPortsByProtocol()
	var defaultAllow bool
	// If the computed traffic permissions don't exist yet, use default deny just to be safe.
	// When it exists, use default deny unless no traffic permissions exist and default allow
	// is configured globally.
	if computed != nil && computed.IsDefault && globalDefaultAllow {
		defaultAllow = true
	}

	out := make(map[string]*pbproxystate.TrafficPermissions)
	portToProtocol := make(map[string]pbcatalog.Protocol)
	var allPorts []string
	for protocol, ports := range portsWithProtocol {
		if protocol == pbcatalog.Protocol_PROTOCOL_MESH {
			continue
		}

		for _, p := range ports {
			allPorts = append(allPorts, p)
			portToProtocol[p] = protocol
			out[p] = &pbproxystate.TrafficPermissions{
				DefaultAllow: defaultAllow,
			}
		}
	}

	if computed == nil {
		return out
	}

	for _, p := range computed.DenyPermissions {
		drsByPort := destinationRulesByPort(allPorts, p.DestinationRules)
		principals := makePrincipals(trustDomain, p)
		for port, rules := range drsByPort {
			if len(rules) > 0 {
				out[port].DenyPermissions = append(out[port].DenyPermissions, &pbproxystate.Permission{
					Principals:       principals,
					DestinationRules: rules,
				})
			} else {
				out[port].DenyPermissions = append(out[port].DenyPermissions, &pbproxystate.Permission{
					Principals: principals,
				})
			}
		}
	}

	for _, p := range computed.AllowPermissions {
		drsByPort := destinationRulesByPort(allPorts, p.DestinationRules)
		principals := makePrincipals(trustDomain, p)
		for port, rules := range drsByPort {
			if _, ok := out[port]; !ok {
				continue
			}
			if len(rules) > 0 {
				out[port].AllowPermissions = append(out[port].AllowPermissions, &pbproxystate.Permission{
					Principals:       principals,
					DestinationRules: rules,
				})
			} else {
				out[port].AllowPermissions = append(out[port].AllowPermissions, &pbproxystate.Permission{
					Principals: principals,
				})
			}
		}
	}

	return out
}

func destinationRulesByPort(allPorts []string, destinationRules []*pbauth.DestinationRule) map[string][]*pbproxystate.DestinationRule {
	out := make(map[string][]*pbproxystate.DestinationRule)

	if len(destinationRules) == 0 {
		for _, p := range allPorts {
			out[p] = nil
		}
		return out
	}

	for _, destinationRule := range destinationRules {
		portRules := convertDestinationRule(allPorts, destinationRule)
		for p, pr := range portRules {
			if pr.rule == nil {
				out[p] = nil
				continue
			}
			out[p] = append(out[p], pr.rule)
		}
	}

	return out
}

type PortRule struct {
	rule *pbproxystate.DestinationRule
}

func convertDestinationRule(allPorts []string, dr *pbauth.DestinationRule) map[string]*PortRule {
	portRules := make(map[string]*PortRule)
	targetPorts := allPorts
	if len(dr.PortNames) > 0 {
		targetPorts = dr.PortNames
	}
	for _, p := range targetPorts {
		if dr.PortsOnly() {
			portRules[p] = &PortRule{}
			for _, exclude := range dr.Exclude {
				for _, ep := range exclude.PortNames {
					delete(portRules, ep)
				}
			}
		} else {
			portRules[p] = makePortRule(dr, p)
		}
	}
	return portRules
}

func makePortRule(dr *pbauth.DestinationRule, p string) *PortRule {
	psdr := &pbproxystate.DestinationRule{
		PathExact:  dr.PathExact,
		PathPrefix: dr.PathPrefix,
		PathRegex:  dr.PathRegex,
		Methods:    dr.Methods,
	}
	psdr.DestinationRuleHeader = destinationRuleHeaders(dr.Headers)

	var excls []*pbproxystate.ExcludePermissionRule
	for _, ex := range dr.Exclude {
		if len(ex.PortNames) == 0 || listContains(ex.PortNames, p) {
			excls = append(excls, &pbproxystate.ExcludePermissionRule{
				PathExact:  ex.PathExact,
				PathPrefix: ex.PathPrefix,
				PathRegex:  ex.PathRegex,
				Methods:    ex.Methods,
				Headers:    destinationRuleHeaders(ex.Headers),
			})
		}
	}
	psdr.Exclude = excls
	return &PortRule{psdr}
}

func listContains(list []string, str string) bool {
	for _, item := range list {
		if item == str {
			return true
		}
	}
	return false
}

func destinationRuleHeaders(headers []*pbauth.DestinationRuleHeader) []*pbproxystate.DestinationRuleHeader {
	hrs := make([]*pbproxystate.DestinationRuleHeader, len(headers))
	for i, hr := range headers {
		hrs[i] = &pbproxystate.DestinationRuleHeader{
			Name:    hr.Name,
			Present: hr.Present,
			Exact:   hr.Exact,
			Prefix:  hr.Prefix,
			Suffix:  hr.Suffix,
			Regex:   hr.Regex,
			Invert:  hr.Invert,
		}
	}
	return hrs
}

func makePrincipals(trustDomain string, perm *pbauth.Permission) []*pbproxystate.Principal {
	var principals []*pbproxystate.Principal
	for _, s := range perm.Sources {
		principals = append(principals, makePrincipal(trustDomain, s))
	}

	return principals
}

func makePrincipal(trustDomain string, s *pbauth.Source) *pbproxystate.Principal {
	excludes := make([]*pbproxystate.Spiffe, 0, len(s.Exclude))
	for _, es := range s.Exclude {
		excludes = append(excludes, sourceToSpiffe(trustDomain, es))
	}

	return &pbproxystate.Principal{
		Spiffe:         sourceToSpiffe(trustDomain, s),
		ExcludeSpiffes: excludes,
	}
}

const (
	anyPath = `[^/]+`
)

func sourceToSpiffe(trustDomain string, s pbauth.SourceToSpiffe) *pbproxystate.Spiffe {
	var (
		name = s.GetIdentityName()
		ns   = s.GetNamespace()
		ap   = s.GetPartition()
	)

	if ns == "" && name != "" {
		panic(fmt.Sprintf("not possible to have a wildcarded namespace %q but an exact identity %q", ns, name))
	}

	if ap == "" {
		panic("not possible to have a wildcarded source partition")
	}

	if ns == "" {
		ns = anyPath
	}
	if name == "" {
		name = anyPath
	}

	spiffeURI := connect.SpiffeIDWorkloadIdentity{
		TrustDomain:      trustDomain,
		Partition:        ap,
		Namespace:        ns,
		WorkloadIdentity: name,
	}.URI()

	matcher := fmt.Sprintf(`^%s://%s%s$`, spiffeURI.Scheme, spiffeURI.Host, spiffeURI.Path)

	return &pbproxystate.Spiffe{
		Regex: matcher,
	}
}

func (b *Builder) addInboundListener(name string, workload *pbcatalog.Workload) *ListenerBuilder {
	listener := &pbproxystate.Listener{
		Name:      name,
		Direction: pbproxystate.Direction_DIRECTION_INBOUND,
	}

	// We will take listener bind port from the workload.
	// Find mesh port.
	meshPort, ok := workload.GetMeshPortName()
	if !ok {
		// At this point, we should only get workloads that have mesh ports.
		return &ListenerBuilder{
			builder: b,
		}
	}

	// Check if the workload has a specific address for the mesh port.
	meshAddresses := workload.GetNonExternalAddressesForPort(meshPort)

	// If there are no mesh addresses, return. This should be impossible.
	if len(meshAddresses) == 0 {
		return &ListenerBuilder{
			builder: b,
		}
	}

	// If there are more than one mesh address, use the first one in the list.
	var meshAddress string
	if len(meshAddresses) > 0 {
		meshAddress = meshAddresses[0].Host
	}

	listener.BindAddress = &pbproxystate.Listener_HostPort{
		HostPort: &pbproxystate.HostPortAddress{
			Host: meshAddress,
			Port: workload.Ports[meshPort].Port,
		},
	}

	// Add TLS inspection capability to be able to parse ALPN and/or SNI information from inbound connections.
	listener.Capabilities = append(listener.Capabilities, pbproxystate.Capability_CAPABILITY_L4_TLS_INSPECTION)

	if b.proxyCfg.GetDynamicConfig() != nil && b.proxyCfg.GetDynamicConfig().InboundConnections != nil {
		listener.BalanceConnections = pbproxystate.BalanceConnections(b.proxyCfg.DynamicConfig.InboundConnections.BalanceInboundConnections)
	}
	return b.NewListenerBuilder(listener)
}

func (l *ListenerBuilder) addInboundRouter(clusterName string, routeName string,
	port *pbcatalog.WorkloadPort, portName string, tp *pbproxystate.TrafficPermissions,
	ic *pbmesh.InboundConnectionsConfig) *ListenerBuilder {

	if l.listener == nil {
		return l
	}

	if port.Protocol == pbcatalog.Protocol_PROTOCOL_TCP {
		r := &pbproxystate.Router{
			Destination: &pbproxystate.Router_L4{
				L4: &pbproxystate.L4Destination{
					Destination: &pbproxystate.L4Destination_Cluster{
						Cluster: &pbproxystate.DestinationCluster{
							Name: clusterName,
						},
					},
					StatPrefix:         l.listener.Name,
					TrafficPermissions: tp,
				},
			},
			Match: &pbproxystate.Match{
				AlpnProtocols: []string{getAlpnProtocolFromPortName(portName)},
			},
		}

		if ic != nil {
			// MaxInboundConnections is uint32 that is used on:
			// - router destinations MaxInboundConnection (uint64).
			// - cluster circuit breakers UpstreamLimits.MaxConnections (uint32).
			// It is cast to a uint64 here similarly as it is to the proxystateconverter code.
			r.GetL4().MaxInboundConnections = uint64(ic.MaxInboundConnections)
		}

		l.listener.Routers = append(l.listener.Routers, r)
	} else if isL7(port.Protocol) {
		r := &pbproxystate.Router{
			Destination: &pbproxystate.Router_L7{
				L7: &pbproxystate.L7Destination{
					StatPrefix:         l.listener.Name,
					Protocol:           protocolMapCatalogToL7[port.Protocol],
					TrafficPermissions: tp,
					StaticRoute:        true,
					// Route name for l7 local app destinations differentiates between routes for each port.
					Route: &pbproxystate.L7DestinationRoute{
						Name: routeName,
					},
				},
			},
			Match: &pbproxystate.Match{
				AlpnProtocols: []string{getAlpnProtocolFromPortName(portName)},
			},
		}

		if ic != nil {
			// MaxInboundConnections is cast to a uint64 here similarly as it is to the
			// as the L4 case statement above and in proxystateconverter code.
			r.GetL7().MaxInboundConnections = uint64(ic.MaxInboundConnections)
		}

		l.listener.Routers = append(l.listener.Routers, r)
	}
	return l
}

func (l *ListenerBuilder) addBlackHoleRouter() *ListenerBuilder {
	if l.listener == nil {
		return l
	}

	r := &pbproxystate.Router{
		Destination: &pbproxystate.Router_L4{
			L4: &pbproxystate.L4Destination{
				Destination: &pbproxystate.L4Destination_Cluster{
					Cluster: &pbproxystate.DestinationCluster{
						Name: xdscommon.BlackHoleClusterName,
					},
				},
				StatPrefix: l.listener.Name,
			},
		},
	}
	l.listener.Routers = append(l.listener.Routers, r)

	return l
}

func getAlpnProtocolFromPortName(portName string) string {
	return fmt.Sprintf("consul~%s", portName)
}

func (b *Builder) addLocalAppRoute(routeName, clusterName, portName string) {
	proxyRouteRule := &pbproxystate.RouteRule{
		Match: &pbproxystate.RouteMatch{
			PathMatch: &pbproxystate.PathMatch{
				PathMatch: &pbproxystate.PathMatch_Prefix{
					Prefix: "/",
				},
			},
		},
		Destination: &pbproxystate.RouteDestination{
			Destination: &pbproxystate.RouteDestination_Cluster{
				Cluster: &pbproxystate.DestinationCluster{
					Name: clusterName,
				},
			},
		},
	}
	if b.proxyCfg.GetDynamicConfig() != nil && b.proxyCfg.GetDynamicConfig().LocalConnection != nil {
		lc, lcOK := b.proxyCfg.GetDynamicConfig().LocalConnection[portName]
		if lcOK {
			proxyRouteRule.Destination.DestinationConfiguration =
				&pbproxystate.DestinationConfiguration{
					TimeoutConfig: &pbproxystate.TimeoutConfig{
						Timeout: lc.RequestTimeout,
					},
				}
		}
	}

	// Each route name for the local app is listenerName:port since there is a route per port on the local app listener.
	b.addRoute(routeName, &pbproxystate.Route{
		VirtualHosts: []*pbproxystate.VirtualHost{{
			Name:       routeName,
			Domains:    []string{"*"},
			RouteRules: []*pbproxystate.RouteRule{proxyRouteRule},
		}},
	})
}

func isL7(protocol pbcatalog.Protocol) bool {
	if protocol == pbcatalog.Protocol_PROTOCOL_HTTP || protocol == pbcatalog.Protocol_PROTOCOL_HTTP2 || protocol == pbcatalog.Protocol_PROTOCOL_GRPC {
		return true
	}
	return false
}

func (b *Builder) addLocalAppCluster(clusterName string, portName *string, protocol pbproxystate.Protocol) *Builder {
	// Make cluster for this router destination.
	cluster := &pbproxystate.Cluster{
		Group: &pbproxystate.Cluster_EndpointGroup{
			EndpointGroup: &pbproxystate.EndpointGroup{
				Group: &pbproxystate.EndpointGroup_Static{
					Static: &pbproxystate.StaticEndpointGroup{},
				},
			},
		},
		Protocol: protocol,
	}

	// configure inbound connections or connection timeout if either is defined
	if b.proxyCfg.GetDynamicConfig() != nil && portName != nil {
		lc, lcOK := b.proxyCfg.DynamicConfig.LocalConnection[*portName]

		if lcOK || b.proxyCfg.DynamicConfig.InboundConnections != nil {
			cluster.GetEndpointGroup().GetStatic().Config = &pbproxystate.StaticEndpointGroupConfig{}

			if lcOK {
				cluster.GetEndpointGroup().GetStatic().GetConfig().ConnectTimeout = lc.ConnectTimeout
			}

			if b.proxyCfg.DynamicConfig.InboundConnections != nil {
				cluster.GetEndpointGroup().GetStatic().GetConfig().CircuitBreakers = &pbproxystate.CircuitBreakers{
					UpstreamLimits: &pbproxystate.UpstreamLimits{
						MaxConnections: &wrapperspb.UInt32Value{Value: b.proxyCfg.DynamicConfig.InboundConnections.MaxInboundConnections},
					},
				}
			}
		}
	}

	b.proxyStateTemplate.ProxyState.Clusters[clusterName] = cluster
	return b
}

func (b *Builder) addBlackHoleCluster() *Builder {
	return b.addLocalAppCluster(xdscommon.BlackHoleClusterName, nil, pbproxystate.Protocol_PROTOCOL_TCP)
}

func (b *Builder) addLocalAppStaticEndpoints(clusterName string, port uint32) {
	// We're adding endpoints statically as opposed to creating an endpoint ref
	// because this endpoint is less likely to change as we're not tracking the health.
	endpoint := &pbproxystate.Endpoint{
		Address: &pbproxystate.Endpoint_HostPort{
			HostPort: &pbproxystate.HostPortAddress{
				Host: "127.0.0.1",
				Port: port,
			},
		},
	}
	b.proxyStateTemplate.ProxyState.Endpoints[clusterName] = &pbproxystate.Endpoints{
		Endpoints: []*pbproxystate.Endpoint{endpoint},
	}
}

func (l *ListenerBuilder) addInboundTLS() *ListenerBuilder {
	if l.listener == nil {
		return nil
	}

	// For inbound TLS, we want to use this proxy's identity.
	workloadIdentity := l.builder.proxyStateTemplate.ProxyState.Identity.Name

	inboundTLS := &pbproxystate.TransportSocket{
		ConnectionTls: &pbproxystate.TransportSocket_InboundMesh{
			InboundMesh: &pbproxystate.InboundMeshMTLS{
				IdentityKey: workloadIdentity,
				ValidationContext: &pbproxystate.MeshInboundValidationContext{
					TrustBundlePeerNameKeys: []string{resource.DefaultPeerName},
				},
			},
		},
	}

	for i := range l.listener.Routers {
		l.listener.Routers[i].InboundTls = inboundTLS
	}
	return l
}

var protocolMapCatalogToL7 = map[pbcatalog.Protocol]pbproxystate.L7Protocol{
	pbcatalog.Protocol_PROTOCOL_HTTP:  pbproxystate.L7Protocol_L7_PROTOCOL_HTTP,
	pbcatalog.Protocol_PROTOCOL_HTTP2: pbproxystate.L7Protocol_L7_PROTOCOL_HTTP2,
	pbcatalog.Protocol_PROTOCOL_GRPC:  pbproxystate.L7Protocol_L7_PROTOCOL_GRPC,
}
