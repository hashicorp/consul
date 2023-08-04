package builder

import (
	"fmt"

	"github.com/hashicorp/consul/envoyextensions/xdscommon"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1/pbproxystate"
)

func (b *Builder) BuildLocalApp(workload *pbcatalog.Workload) *Builder {
	return b.addInboundListener(xdscommon.PublicListenerName, workload).
		addInboundRouters(workload).
		addInboundTLS()
}

func (b *Builder) getLastBuiltListener() *pbproxystate.Listener {
	lastBuiltIndex := len(b.proxyStateTemplate.ProxyState.Listeners) - 1
	return b.proxyStateTemplate.ProxyState.Listeners[lastBuiltIndex]
}

func (b *Builder) addInboundListener(name string, workload *pbcatalog.Workload) *Builder {
	listener := &pbproxystate.Listener{
		Name:      name,
		Direction: pbproxystate.Direction_DIRECTION_INBOUND,
	}

	// We will take listener bind port from the workload for now.
	// Find mesh port.
	var meshPort string
	for portName, port := range workload.Ports {
		if port.Protocol == pbcatalog.Protocol_PROTOCOL_MESH {
			meshPort = portName
			break
		}
	}

	// Check if the workload has a specific address for the mesh port.
	var meshAddress string
	for _, address := range workload.Addresses {
		for _, port := range address.Ports {
			if port == meshPort {
				meshAddress = address.Host
			}
		}
	}
	// Otherwise, assume the first address in the addresses list.
	if meshAddress == "" {
		// It is safe to assume that there's at least one address because we validate it when creating the workload.
		meshAddress = workload.Addresses[0].Host
	}

	listener.BindAddress = &pbproxystate.Listener_HostPort{
		HostPort: &pbproxystate.HostPortAddress{
			Host: meshAddress,
			Port: workload.Ports[meshPort].Port,
		},
	}

	return b.addListener(listener)
}

func (b *Builder) addInboundRouters(workload *pbcatalog.Workload) *Builder {
	listener := b.getLastBuiltListener()

	// Go through workload ports and add the first non-mesh port we see.
	// Note that the order of ports is non-deterministic here but the xds generation
	// code should make sure to send it in the same order to Envoy to avoid unnecessary
	// updates.
	// todo (ishustava): Note we will need to support multiple ports in the future.
	for portName, port := range workload.Ports {
		clusterName := fmt.Sprintf("%s:%s", xdscommon.LocalAppClusterName, portName)
		if port.Protocol == pbcatalog.Protocol_PROTOCOL_TCP {
			r := &pbproxystate.Router{
				Destination: &pbproxystate.Router_L4{
					L4: &pbproxystate.L4Destination{
						Name:       clusterName,
						StatPrefix: listener.Name,
					},
				},
			}
			listener.Routers = append(listener.Routers, r)

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

			// Finally, add static endpoints. We're adding it statically as opposed to creating an endpoint ref
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
			break
		}
	}
	return b
}

func (b *Builder) addInboundTLS() *Builder {
	listener := b.getLastBuiltListener()
	// For inbound TLS, we want to use this proxy's identity.
	workloadIdentity := b.proxyStateTemplate.ProxyState.Identity.Name

	inboundTLS := &pbproxystate.TransportSocket{
		ConnectionTls: &pbproxystate.TransportSocket_InboundMesh{
			InboundMesh: &pbproxystate.InboundMeshMTLS{
				IdentityKey:       workloadIdentity,
				ValidationContext: &pbproxystate.MeshInboundValidationContext{TrustBundlePeerNameKeys: []string{b.id.Tenancy.PeerName}},
			},
		},
	}
	b.proxyStateTemplate.RequiredLeafCertificates[workloadIdentity] = &pbproxystate.LeafCertificateRef{
		Name:      workloadIdentity,
		Namespace: b.id.Tenancy.Namespace,
		Partition: b.id.Tenancy.Partition,
	}

	b.proxyStateTemplate.RequiredTrustBundles[b.id.Tenancy.PeerName] = &pbproxystate.TrustBundleRef{
		Peer: b.id.Tenancy.PeerName,
	}

	for i := range listener.Routers {
		listener.Routers[i].InboundTls = inboundTLS
	}
	return b
}
