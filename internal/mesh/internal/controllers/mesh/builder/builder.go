package builder

import (
	"fmt"

	"github.com/hashicorp/consul/envoyextensions/xdscommon"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type Builder struct {
	id                 *pbresource.ID
	proxyStateTemplate *pbmesh.ProxyStateTemplate

	lastBuiltListener lastListenerData
}

type lastListenerData struct {
	index int
}

func New(id *pbresource.ID, identity *pbresource.Reference) *Builder {
	return &Builder{
		id: id,
		proxyStateTemplate: &pbmesh.ProxyStateTemplate{
			ProxyState: &pbmesh.ProxyState{
				Identity:  identity,
				Clusters:  make(map[string]*pbmesh.Cluster),
				Endpoints: make(map[string]*pbmesh.Endpoints),
			},
			RequiredEndpoints:        make(map[string]*pbmesh.EndpointRef),
			RequiredLeafCertificates: make(map[string]*pbmesh.LeafCertificateRef),
			RequiredTrustBundles:     make(map[string]*pbmesh.TrustBundleRef),
		},
	}
}

func (b *Builder) Build() *pbmesh.ProxyStateTemplate {
	b.lastBuiltListener = lastListenerData{}
	return b.proxyStateTemplate
}

func (b *Builder) AddInboundListener(name string, workload *pbcatalog.Workload) *Builder {
	listener := &pbmesh.Listener{
		Name:      name,
		Direction: pbmesh.Direction_DIRECTION_INBOUND,
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

	listener.BindAddress = &pbmesh.Listener_IpPort{
		IpPort: &pbmesh.IPPortAddress{
			Ip:   meshAddress,
			Port: workload.Ports[meshPort].Port,
		},
	}

	// Track the last added listener.
	b.lastBuiltListener.index = len(b.proxyStateTemplate.ProxyState.Listeners)
	// Add listener to proxy state template
	b.proxyStateTemplate.ProxyState.Listeners = append(b.proxyStateTemplate.ProxyState.Listeners, listener)

	return b
}

func (b *Builder) AddInboundRouters(workload *pbcatalog.Workload) *Builder {
	listener := b.proxyStateTemplate.ProxyState.Listeners[b.lastBuiltListener.index]

	// Go through workload ports and add the first non-mesh port we see.
	// todo (ishustava): Note we will need to support multiple ports in the future.
	// todo (ishustava): make sure we always iterate through ports in the same order so we don't need to send more updates to envoy.
	for portName, port := range workload.Ports {
		clusterName := fmt.Sprintf("%s:%s", xdscommon.LocalAppClusterName, portName)
		if port.Protocol == pbcatalog.Protocol_PROTOCOL_TCP {
			r := &pbmesh.Router{
				Destination: &pbmesh.Router_L4{
					L4: &pbmesh.L4Destination{
						Name:       clusterName,
						StatPrefix: listener.Name,
					},
				},
			}
			listener.Routers = append(listener.Routers, r)

			// Make cluster for this router destination.
			b.proxyStateTemplate.ProxyState.Clusters[clusterName] = &pbmesh.Cluster{
				Group: &pbmesh.Cluster_EndpointGroup{
					EndpointGroup: &pbmesh.EndpointGroup{
						Group: &pbmesh.EndpointGroup_Static{
							Static: &pbmesh.StaticEndpointGroup{
								Name: clusterName,
							},
						},
					},
				},
			}

			// Finally, add static endpoints. We're adding it statically as opposed to creating an endpoint ref
			// because this endpoint is less likely to change as we're not tracking the health.
			endpoint := &pbmesh.Endpoint{
				Address: &pbmesh.Endpoint_HostPort{
					HostPort: &pbmesh.HostPortAddress{
						Host: "127.0.0.1",
						Port: port.Port,
					},
				},
			}
			b.proxyStateTemplate.ProxyState.Endpoints[clusterName] = &pbmesh.Endpoints{
				Name:      clusterName,
				Endpoints: []*pbmesh.Endpoint{endpoint},
			}
			break
		}
	}
	return b
}

func (b *Builder) AddInboundTLS() *Builder {
	listener := b.proxyStateTemplate.ProxyState.Listeners[b.lastBuiltListener.index]
	// For inbound TLS, we want to use this proxy's identity.
	workloadIdentity := b.proxyStateTemplate.ProxyState.Identity.Name

	inboundTLS := &pbmesh.TransportSocket{
		ConnectionTls: &pbmesh.TransportSocket_InboundMesh{
			InboundMesh: &pbmesh.InboundMeshMTLS{
				IdentityKey:       workloadIdentity,
				ValidationContext: &pbmesh.MeshInboundValidationContext{TrustBundlePeerNameKeys: []string{b.id.Tenancy.PeerName}},
			},
		},
	}
	b.proxyStateTemplate.RequiredLeafCertificates[workloadIdentity] = &pbmesh.LeafCertificateRef{
		Name:      workloadIdentity,
		Namespace: b.id.Tenancy.Namespace,
		Partition: b.id.Tenancy.Partition,
	}

	b.proxyStateTemplate.RequiredTrustBundles[b.id.Tenancy.PeerName] = &pbmesh.TrustBundleRef{
		Peer: b.id.Tenancy.PeerName,
	}

	for i := range listener.Routers {
		listener.Routers[i].InboundTls = inboundTLS
	}
	return b
}
