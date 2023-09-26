// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package builder

import (
	"fmt"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/envoyextensions/xdscommon"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbmesh/v2beta1/pbproxystate"
)

func (b *Builder) BuildLocalApp(workload *pbcatalog.Workload, ctp *pbauth.ComputedTrafficPermissions) *Builder {
	// Add the public listener.
	lb := b.addInboundListener(xdscommon.PublicListenerName, workload)
	lb.buildListener()

	trafficPermissions := buildTrafficPermissions(b.trustDomain, workload, ctp)

	// Go through workload ports and add the routers, clusters, endpoints, and TLS.
	// Note that the order of ports is non-deterministic here but the xds generation
	// code should make sure to send it in the same order to Envoy to avoid unnecessary
	// updates.
	foundInboundNonMeshPorts := false
	for portName, port := range workload.Ports {
		clusterName := fmt.Sprintf("%s:%s", xdscommon.LocalAppClusterName, portName)

		if port.Protocol != pbcatalog.Protocol_PROTOCOL_MESH {
			foundInboundNonMeshPorts = true
			lb.addInboundRouter(clusterName, port, portName, trafficPermissions[portName]).
				addInboundTLS()

			b.addLocalAppCluster(clusterName).
				addLocalAppStaticEndpoints(clusterName, port)
		}
	}

	// If there are no inbound ports other than the mesh port, we black-hole all inbound traffic.
	if !foundInboundNonMeshPorts {
		lb.addBlackHoleRouter()
		b.addBlackHoleCluster()
	}

	return b
}

func buildTrafficPermissions(trustDomain string, workload *pbcatalog.Workload, computed *pbauth.ComputedTrafficPermissions) map[string]*pbproxystate.TrafficPermissions {
	portsWithProtocol := workload.GetPortsByProtocol()

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
			out[p] = &pbproxystate.TrafficPermissions{}
		}
	}

	if computed == nil {
		return out
	}

	for _, p := range computed.DenyPermissions {
		drsByPort := destinationRulesByPort(allPorts, p.DestinationRules)
		principals := makePrincipals(trustDomain, p)
		for port := range drsByPort {
			out[port].DenyPermissions = append(out[port].DenyPermissions, &pbproxystate.Permission{
				Principals: principals,
			})
		}
	}

	for _, p := range computed.AllowPermissions {
		drsByPort := destinationRulesByPort(allPorts, p.DestinationRules)
		principals := makePrincipals(trustDomain, p)
		for port := range drsByPort {
			out[port].AllowPermissions = append(out[port].AllowPermissions, &pbproxystate.Permission{
				Principals: principals,
			})
		}
	}

	return out
}

// TODO this is a placeholder until we add them to the IR.
type DestinationRule struct{}

func destinationRulesByPort(allPorts []string, destinationRules []*pbauth.DestinationRule) map[string][]DestinationRule {
	out := make(map[string][]DestinationRule)

	if len(destinationRules) == 0 {
		for _, p := range allPorts {
			out[p] = nil
		}

		return out
	}

	for _, destinationRule := range destinationRules {
		ports, dr := convertDestinationRule(allPorts, destinationRule)
		for _, p := range ports {
			out[p] = append(out[p], dr)
		}
	}

	return out
}

func convertDestinationRule(allPorts []string, dr *pbauth.DestinationRule) ([]string, DestinationRule) {
	ports := make(map[string]struct{})
	if len(dr.PortNames) > 0 {
		for _, p := range dr.PortNames {
			ports[p] = struct{}{}
		}
	} else {
		for _, p := range allPorts {
			ports[p] = struct{}{}
		}
	}

	for _, exclude := range dr.Exclude {
		for _, p := range exclude.PortNames {
			delete(ports, p)
		}
	}

	var out []string
	for p := range ports {
		out = append(out, p)
	}

	return out, DestinationRule{}
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

	return b.NewListenerBuilder(listener)
}

func (l *ListenerBuilder) addInboundRouter(clusterName string, port *pbcatalog.WorkloadPort, portName string, tp *pbproxystate.TrafficPermissions) *ListenerBuilder {
	if l.listener == nil {
		return l
	}

	if port.Protocol == pbcatalog.Protocol_PROTOCOL_TCP || port.Protocol == pbcatalog.Protocol_PROTOCOL_UNSPECIFIED {
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

func (b *Builder) addLocalAppCluster(clusterName string) *Builder {
	// Make cluster for this router destination.
	b.proxyStateTemplate.ProxyState.Clusters[clusterName] = &pbproxystate.Cluster{
		Group: &pbproxystate.Cluster_EndpointGroup{
			EndpointGroup: &pbproxystate.EndpointGroup{
				Group: &pbproxystate.EndpointGroup_Static{
					Static: &pbproxystate.StaticEndpointGroup{},
				},
			},
		},
	}
	return b
}

func (b *Builder) addBlackHoleCluster() *Builder {
	b.proxyStateTemplate.ProxyState.Clusters[xdscommon.BlackHoleClusterName] = &pbproxystate.Cluster{
		Group: &pbproxystate.Cluster_EndpointGroup{
			EndpointGroup: &pbproxystate.EndpointGroup{
				Group: &pbproxystate.EndpointGroup_Static{
					Static: &pbproxystate.StaticEndpointGroup{},
				},
			},
		},
	}
	return b
}

func (b *Builder) addLocalAppStaticEndpoints(clusterName string, port *pbcatalog.WorkloadPort) *Builder {
	// We're adding endpoints statically as opposed to creating an endpoint ref
	// because this endpoint is less likely to change as we're not tracking the health.
	endpoint := &pbproxystate.Endpoint{
		Address: &pbproxystate.Endpoint_HostPort{
			HostPort: &pbproxystate.HostPortAddress{
				Host: "127.0.0.1",
				Port: port.Port,
			},
		},
	}
	b.proxyStateTemplate.ProxyState.Endpoints[clusterName] = &pbproxystate.Endpoints{
		Endpoints: []*pbproxystate.Endpoint{endpoint},
	}

	return b
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
					TrustBundlePeerNameKeys: []string{l.builder.id.Tenancy.PeerName},
				},
			},
		},
	}

	for i := range l.listener.Routers {
		l.listener.Routers[i].InboundTls = inboundTLS
	}
	return l
}
