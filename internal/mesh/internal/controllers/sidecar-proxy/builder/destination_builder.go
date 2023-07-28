package builder

import (
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/internal/mesh/internal/types/intermediate"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1/pbproxystate"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func (b *Builder) BuildDestinations(destinations []*intermediate.Destination) *Builder {
	for _, destination := range destinations {
		if destination.Explicit != nil {
			b.buildExplicitDestination(destination)
		}
	}

	return b
}

func (b *Builder) buildExplicitDestination(destination *intermediate.Destination) *Builder {
	clusterName := DestinationClusterName(destination.Explicit.DestinationRef, destination.Explicit.Datacenter, b.trustDomain)
	statPrefix := DestinationStatPrefix(destination.Explicit.DestinationRef, destination.Explicit.Datacenter)

	// We assume that all endpoints have the same port. Later, we will change service endpoints to
	// have the global ports map rather than per address.
	destPort := destination.ServiceEndpoints.Endpoints.Endpoints[0].Ports[destination.Explicit.DestinationPort]

	if destPort != nil {
		return b.addOutboundDestinationListener(destination.Explicit).
			addRouter(clusterName, statPrefix, destPort.Protocol).
			addCluster(clusterName, destination.Identities).
			addEndpointsRef(clusterName, destination.ServiceEndpoints.Resource.Id, destination.Explicit.DestinationPort)
	}

	return b
}

func (b *Builder) addOutboundDestinationListener(explicit *pbmesh.Upstream) *Builder {
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

	return b.addListener(listener)
}

// for explicit destinations, we have no filter chain match, and filters based on port protocol
func (b *Builder) addRouter(clusterName, statPrefix string, protocol pbcatalog.Protocol) *Builder {
	listener := b.getLastBuiltListener()

	switch protocol {
	case pbcatalog.Protocol_PROTOCOL_TCP:
		router := &pbproxystate.Router{
			Destination: &pbproxystate.Router_L4{
				L4: &pbproxystate.L4Destination{
					Name:       clusterName,
					StatPrefix: statPrefix,
				},
			},
		}
		listener.Routers = append(listener.Routers, router)
	}
	return b
}

func (b *Builder) addCluster(clusterName string, destinationIdentities []*pbresource.Reference) *Builder {
	var spiffeIDs []string
	for _, identity := range destinationIdentities {
		spiffeIDs = append(spiffeIDs, connect.SpiffeIDFromIdentityRef(b.trustDomain, identity))
	}

	// Create destination cluster
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

func (b *Builder) addEndpointsRef(clusterName string, serviceEndpointsID *pbresource.ID, destinationPort string) *Builder {
	// Finally, add endpoints references.
	b.proxyStateTemplate.RequiredEndpoints[clusterName] = &pbproxystate.EndpointRef{
		Id:   serviceEndpointsID,
		Port: destinationPort,
	}
	return b
}
