// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package builder

import (
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/envoyextensions/xdscommon"
	"github.com/hashicorp/consul/internal/mesh/internal/types/intermediate"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1/pbproxystate"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func (b *Builder) BuildDestinations(destinations []*intermediate.Destination) *Builder {
	if b.proxyCfg.GetDynamicConfig() != nil &&
		b.proxyCfg.DynamicConfig.Mode == pbmesh.ProxyMode_PROXY_MODE_TRANSPARENT {

		b.addOutboundListener(b.proxyCfg.DynamicConfig.TransparentProxy.OutboundListenerPort)
	}

	for _, destination := range destinations {
		if destination.Explicit != nil {
			b.buildExplicitDestination(destination)
		} else {
			b.buildImplicitDestination(destination)
		}
	}

	return b
}

func (b *Builder) buildExplicitDestination(destination *intermediate.Destination) {
	clusterName := DestinationClusterName(destination.Explicit.DestinationRef, destination.Explicit.Datacenter, b.trustDomain)
	statPrefix := DestinationStatPrefix(destination.Explicit.DestinationRef, destination.Explicit.Datacenter)

	// All endpoints should have the same protocol as the endpoints controller ensures that is the case,
	// so it's sufficient to read just the first endpoint.
	if len(destination.ServiceEndpoints.Endpoints.Endpoints) > 0 {
		// Get destination port so that we can configure this destination correctly based on its protocol.
		destPort := destination.ServiceEndpoints.Endpoints.Endpoints[0].Ports[destination.Explicit.DestinationPort]

		// Find the destination proxy's port.
		// Endpoints refs will need to route to mesh port instead of the destination port as that
		// is the port of the destination's proxy.
		meshPortName := findMeshPort(destination.ServiceEndpoints.Endpoints.Endpoints[0].Ports)

		if destPort != nil {
			b.addOutboundDestinationListener(destination.Explicit).
				addRouter(clusterName, statPrefix, destPort.Protocol).
				buildListener().
				addCluster(clusterName, destination.Identities).
				addEndpointsRef(clusterName, destination.ServiceEndpoints.Resource.Id, meshPortName)
		}
	}
}

func (b *Builder) buildImplicitDestination(destination *intermediate.Destination) {
	serviceRef := resource.Reference(destination.ServiceEndpoints.Resource.Owner, "")
	clusterName := DestinationClusterName(serviceRef, b.localDatacenter, b.trustDomain)
	statPrefix := DestinationStatPrefix(serviceRef, b.localDatacenter)

	// We assume that all endpoints have the same port protocol and name, and so it's sufficient
	// to check ports just from the first endpoint.
	if len(destination.ServiceEndpoints.Endpoints.Endpoints) > 0 {
		// Find the destination proxy's port.
		// Endpoints refs will need to route to mesh port instead of the destination port as that
		// is the port of the destination's proxy.
		meshPortName := findMeshPort(destination.ServiceEndpoints.Endpoints.Endpoints[0].Ports)

		for _, port := range destination.ServiceEndpoints.Endpoints.Endpoints[0].Ports {
			b.outboundListenerBuilder.
				addRouterWithIPMatch(clusterName, statPrefix, port.Protocol, destination.VirtualIPs).
				buildListener().
				addCluster(clusterName, destination.Identities).
				addEndpointsRef(clusterName, destination.ServiceEndpoints.Resource.Id, meshPortName)
		}
	}
}

func (b *Builder) addOutboundDestinationListener(explicit *pbmesh.Upstream) *ListenerBuilder {
	listener := &pbproxystate.Listener{
		Direction: pbproxystate.Direction_DIRECTION_OUTBOUND,
	}

	// Create outbound listener address.
	switch explicit.ListenAddr.(type) {
	case *pbmesh.Upstream_IpPort:
		destinationAddr := explicit.ListenAddr.(*pbmesh.Upstream_IpPort)
		listener.BindAddress = &pbproxystate.Listener_HostPort{
			HostPort: &pbproxystate.HostPortAddress{
				Host: destinationAddr.IpPort.Ip,
				Port: destinationAddr.IpPort.Port,
			},
		}
		listener.Name = DestinationListenerName(explicit.DestinationRef.Name, explicit.DestinationPort, destinationAddr.IpPort.Ip, destinationAddr.IpPort.Port)
	case *pbmesh.Upstream_Unix:
		destinationAddr := explicit.ListenAddr.(*pbmesh.Upstream_Unix)
		listener.BindAddress = &pbproxystate.Listener_UnixSocket{
			UnixSocket: &pbproxystate.UnixSocketAddress{
				Path: destinationAddr.Unix.Path,
				Mode: destinationAddr.Unix.Mode,
			},
		}
		listener.Name = DestinationListenerName(explicit.DestinationRef.Name, explicit.DestinationPort, destinationAddr.Unix.Path, 0)
	}

	return b.NewListenerBuilder(listener)
}

func (b *Builder) addOutboundListener(port uint32) *ListenerBuilder {
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

	lb := b.NewListenerBuilder(listener)

	// Save outbound listener builder so we can use it in the future.
	b.outboundListenerBuilder = lb

	return lb
}

func (l *ListenerBuilder) addRouter(clusterName, statPrefix string, protocol pbcatalog.Protocol) *ListenerBuilder {
	return l.addRouterWithIPMatch(clusterName, statPrefix, protocol, nil)
}

func (l *ListenerBuilder) addRouterWithIPMatch(clusterName, statPrefix string, protocol pbcatalog.Protocol, vips []string) *ListenerBuilder {
	// For explicit destinations, we have no filter chain match, and filters are based on port protocol.
	router := &pbproxystate.Router{}
	switch protocol {
	case pbcatalog.Protocol_PROTOCOL_TCP:
		router.Destination = &pbproxystate.Router_L4{
			L4: &pbproxystate.L4Destination{
				Name:       clusterName,
				StatPrefix: statPrefix,
			},
		}
	}

	if router.Destination != nil {
		for _, vip := range vips {
			if router.Match == nil {
				router.Match = &pbproxystate.Match{}
			}

			router.Match.PrefixRanges = append(router.Match.PrefixRanges, &pbproxystate.CidrRange{
				AddressPrefix: vip,
				PrefixLen:     &wrapperspb.UInt32Value{Value: 32},
			})
		}
		l.listener.Routers = append(l.listener.Routers, router)
	}
	return l
}

func (b *Builder) addCluster(clusterName string, destinationIdentities []*pbresource.Reference) *Builder {
	var spiffeIDs []string
	for _, identity := range destinationIdentities {
		spiffeIDs = append(spiffeIDs, connect.SpiffeIDFromIdentityRef(b.trustDomain, identity))
	}

	// Create destination cluster.
	cluster := &pbproxystate.Cluster{
		Group: &pbproxystate.Cluster_EndpointGroup{
			EndpointGroup: &pbproxystate.EndpointGroup{
				Group: &pbproxystate.EndpointGroup_Dynamic{
					Dynamic: &pbproxystate.DynamicEndpointGroup{
						Config: &pbproxystate.DynamicEndpointGroupConfig{
							DisablePanicThreshold: true,
						},
						OutboundTls: &pbproxystate.TransportSocket{
							ConnectionTls: &pbproxystate.TransportSocket_OutboundMesh{
								OutboundMesh: &pbproxystate.OutboundMeshMTLS{
									IdentityKey: b.proxyStateTemplate.ProxyState.Identity.Name,
									ValidationContext: &pbproxystate.MeshOutboundValidationContext{
										SpiffeIds: spiffeIDs,
									},
									Sni: clusterName,
								},
							},
						},
					},
				},
			},
		},
	}

	b.proxyStateTemplate.ProxyState.Clusters[clusterName] = cluster
	return b
}

func (b *Builder) addEndpointsRef(clusterName string, serviceEndpointsID *pbresource.ID, destinationPort string) {
	b.proxyStateTemplate.RequiredEndpoints[clusterName] = &pbproxystate.EndpointRef{
		Id:   serviceEndpointsID,
		Port: destinationPort,
	}
}

func findMeshPort(ports map[string]*pbcatalog.WorkloadPort) string {
	for name, port := range ports {
		if port.Protocol == pbcatalog.Protocol_PROTOCOL_MESH {
			return name
		}
	}
	return ""
}
