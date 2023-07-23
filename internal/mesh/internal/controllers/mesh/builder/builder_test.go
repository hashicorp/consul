package builder

import (
	"testing"

	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/stretchr/testify/require"
)

func TestAddInboundListener(t *testing.T) {
	listenerName := "test-listener"

	cases := map[string]struct {
		workload    *pbcatalog.Workload
		expListener *pbmesh.Listener
	}{
		"single workload address without ports": {
			workload: &pbcatalog.Workload{
				Addresses: []*pbcatalog.WorkloadAddress{
					{
						Host: "10.0.0.1",
					},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"port1": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
					"port2": {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
			},
			expListener: &pbmesh.Listener{
				Name:      listenerName,
				Direction: pbmesh.Direction_DIRECTION_INBOUND,
				BindAddress: &pbmesh.Listener_IpPort{
					IpPort: &pbmesh.IPPortAddress{
						Ip:   "10.0.0.1",
						Port: 20000,
					},
				},
			},
		},
		"multiple workload addresses without ports: prefer first address": {
			workload: &pbcatalog.Workload{
				Addresses: []*pbcatalog.WorkloadAddress{
					{
						Host: "10.0.0.1",
					},
					{
						Host: "10.0.0.2",
					},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"port1": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
					"port2": {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
			},
			expListener: &pbmesh.Listener{
				Name:      listenerName,
				Direction: pbmesh.Direction_DIRECTION_INBOUND,
				BindAddress: &pbmesh.Listener_IpPort{
					IpPort: &pbmesh.IPPortAddress{
						Ip:   "10.0.0.1",
						Port: 20000,
					},
				},
			},
		},
		"multiple workload addresses with specific ports": {
			workload: &pbcatalog.Workload{
				Addresses: []*pbcatalog.WorkloadAddress{
					{
						Host:  "127.0.0.1",
						Ports: []string{"port1"},
					},
					{
						Host:  "10.0.0.2",
						Ports: []string{"port2"},
					},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"port1": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
					"port2": {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
			},
			expListener: &pbmesh.Listener{
				Name:      listenerName,
				Direction: pbmesh.Direction_DIRECTION_INBOUND,
				BindAddress: &pbmesh.Listener_IpPort{
					IpPort: &pbmesh.IPPortAddress{
						Ip:   "10.0.0.2",
						Port: 20000,
					},
				},
			},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			proxyStateTemplateID := testProxyStateTemplateID()

			proxyStateTemplate := New(proxyStateTemplateID, testIdentityRef()).AddInboundListener(listenerName, c.workload).Build()
			require.Len(t, proxyStateTemplate.ProxyState.Listeners, 1)
			prototest.AssertDeepEqual(t, c.expListener, proxyStateTemplate.ProxyState.Listeners[0])
		})
	}
}

func TestAddInboundRouters(t *testing.T) {
	workload := testWorkload()

	// Create new builder
	builder := New(testProxyStateTemplateID(), testIdentityRef()).
		AddInboundListener("test-listener", workload).
		AddInboundRouters(workload)

	clusterName := "local_app:port1"
	expRouters := []*pbmesh.Router{
		{
			Destination: &pbmesh.Router_L4{
				L4: &pbmesh.L4Destination{
					Name:       clusterName,
					StatPrefix: "test-listener",
				},
			},
		},
	}
	expCluster := &pbmesh.Cluster{
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

	expEndpoints := &pbmesh.Endpoints{
		Name: clusterName,
		Endpoints: []*pbmesh.Endpoint{
			{
				Address: &pbmesh.Endpoint_HostPort{
					HostPort: &pbmesh.HostPortAddress{
						Host: "127.0.0.1",
						Port: 8080,
					},
				},
			},
		},
	}

	proxyStateTemplate := builder.Build()

	// Check routers.
	require.Len(t, proxyStateTemplate.ProxyState.Listeners, 1)
	prototest.AssertDeepEqual(t, expRouters, proxyStateTemplate.ProxyState.Listeners[0].Routers)

	// Check that the cluster exists in the clusters map.
	prototest.AssertDeepEqual(t, expCluster, proxyStateTemplate.ProxyState.Clusters[clusterName])

	// Check that the endpoints exist in the endpoint map for this cluster name.
	prototest.AssertDeepEqual(t, expEndpoints, proxyStateTemplate.ProxyState.Endpoints[clusterName])
}

func TestAddInboundTLS(t *testing.T) {
	id := testProxyStateTemplateID()
	workload := testWorkload()

	proxyStateTemplate := New(id, testIdentityRef()).
		AddInboundListener("test-listener", workload).
		AddInboundRouters(workload).
		AddInboundTLS().
		Build()

	expTransportSocket := &pbmesh.TransportSocket{
		ConnectionTls: &pbmesh.TransportSocket_InboundMesh{
			InboundMesh: &pbmesh.InboundMeshMTLS{
				IdentityKey: workload.Identity,
				ValidationContext: &pbmesh.MeshInboundValidationContext{
					TrustBundlePeerNameKeys: []string{id.Tenancy.PeerName}},
			},
		},
	}
	expLeafCertRef := &pbmesh.LeafCertificateRef{
		Name:      workload.Identity,
		Namespace: id.Tenancy.Namespace,
		Partition: id.Tenancy.Partition,
	}

	require.Len(t, proxyStateTemplate.ProxyState.Listeners, 1)
	// Check that each router has the same TLS configuration.
	for _, router := range proxyStateTemplate.ProxyState.Listeners[0].Routers {
		prototest.AssertDeepEqual(t, expTransportSocket, router.InboundTls)
	}

	// Check that there's a leaf cert ref added to the map.
	prototest.AssertDeepEqual(t, expLeafCertRef, proxyStateTemplate.RequiredLeafCertificates[workload.Identity])

	// Check that there's trust bundle name added to the trust bundles names.
	_, ok := proxyStateTemplate.RequiredTrustBundles[id.Tenancy.PeerName]
	require.True(t, ok)
}

func testProxyStateTemplateID() *pbresource.ID {
	return resourcetest.Resource(types.ProxyStateTemplateType, "test").ID()
}

func testIdentityRef() *pbresource.Reference {
	return &pbresource.Reference{
		Name: "test-identity",
		Tenancy: &pbresource.Tenancy{
			Namespace: "default",
			Partition: "default",
			PeerName:  "local",
		},
	}
}

func testWorkload() *pbcatalog.Workload {
	return &pbcatalog.Workload{
		Identity: "test-identity",
		Addresses: []*pbcatalog.WorkloadAddress{
			{
				Host: "10.0.0.1",
			},
		},
		Ports: map[string]*pbcatalog.WorkloadPort{
			"port1": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
			"port2": {Port: 8081, Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			"port3": {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
		},
	}
}
