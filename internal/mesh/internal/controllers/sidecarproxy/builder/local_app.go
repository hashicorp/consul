// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package builder

import (
	"fmt"

	"github.com/hashicorp/consul/envoyextensions/xdscommon"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1/pbproxystate"
)

func (b *Builder) BuildLocalApp(workload *pbcatalog.Workload) *Builder {
	// Add the public listener.
	lb := b.addInboundListener(xdscommon.PublicListenerName, workload)
	lb.buildListener()

	// Go through workload ports and add the routers, clusters, endpoints, and TLS.
	// Note that the order of ports is non-deterministic here but the xds generation
	// code should make sure to send it in the same order to Envoy to avoid unnecessary
	// updates.
	for portName, port := range workload.Ports {
		clusterName := fmt.Sprintf("%s:%s", xdscommon.LocalAppClusterName, portName)

		if port.Protocol != pbcatalog.Protocol_PROTOCOL_MESH {
			lb.addInboundRouter(clusterName, port, portName).
				addInboundTLS()

			b.addLocalAppCluster(clusterName).
				addLocalAppStaticEndpoints(clusterName, port)
		}
	}

	return b
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

	return b.NewListenerBuilder(listener)
}

func (l *ListenerBuilder) addInboundRouter(clusterName string, port *pbcatalog.WorkloadPort, portName string) *ListenerBuilder {
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
					StatPrefix: l.listener.Name,
				},
			},
			Match: &pbproxystate.Match{
				AlpnProtocols: []string{getAlpnProtocolFromPortName(portName)},
			},
		}
		l.listener.Routers = append(l.listener.Routers, r)
	}
	l.listener.Capabilities = append(l.listener.Capabilities, pbproxystate.Capability_CAPABILITY_L4_TLS_INSPECTION)
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
					TrustBundlePeerNameKeys: []string{"local"},
				},
			},
		},
	}
	l.builder.proxyStateTemplate.RequiredLeafCertificates[workloadIdentity] = &pbproxystate.LeafCertificateRef{
		Name:      workloadIdentity,
		Namespace: l.builder.id.Tenancy.Namespace,
		Partition: l.builder.id.Tenancy.Partition,
	}

	l.builder.proxyStateTemplate.RequiredTrustBundles["local"] = &pbproxystate.TrustBundleRef{
		Peer: "local",
	}

	for i := range l.listener.Routers {
		l.listener.Routers[i].InboundTls = inboundTLS
	}
	return l
}
