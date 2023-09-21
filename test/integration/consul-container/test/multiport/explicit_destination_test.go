package multiport

import (
	"context"
	"fmt"
	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/mesh"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"

	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

var (
	requestRetryTimer = &retry.Timer{Timeout: 120 * time.Second, Wait: 500 * time.Millisecond}
)

// TestMultiportService_Explicit makes sure two services in the same datacenter have connectivity
// with transparent proxy enabled.
//
// Steps:
//   - Create a single server cluster.
//   - Create the example static-server and sidecar containers, then register them both with Consul
//   - Create an example static-client sidecar, then register both the service and sidecar with Consul
//   - Make sure a request from static-client to the virtual address (<svc>.virtual.consul) returns a
//     response from the upstream.
func TestMultiportService_Explicit(t *testing.T) {
	t.Parallel()

	cluster := createCluster(t) // 2 client agent pods
	followers, err := cluster.Followers()
	require.NoError(t, err)
	client := pbresource.NewResourceServiceClient(followers[0].GetGRPCConn())
	resourceClient := rtest.NewClient(client)

	serverService := createServerServicesAndWorkloads(t, resourceClient)
	createClientServicesAndWorkloads(t, resourceClient, serverService)

	clientDataplane := createServices(t, cluster)
	//_, adminPort := clientDataplane.GetAdminAddr()
	_, port := clientDataplane.GetAddr()

	createClientUpstreams(t, resourceClient, serverService, port)

	//libassert.AssertUpstreamEndpointStatus(t, adminPort, "static-server.default", "HEALTHY", 1)
	//libassert.GetEnvoyListenerTCPFilters(t, adminPort)

	assertDataplaneContainerState(t, clientDataplane, "running")
	libassert.HTTPServiceEchoes(t, "localhost", port, "")
	libassert.AssertFortioName(t, fmt.Sprintf("http://localhost:%d", port), "static-server-service", "")

}

// createServices creates the static-client and static-server services with
// transparent proxy enabled. It returns a Service for the static-client.
func createServices(t *testing.T, cluster *libcluster.Cluster) *libcluster.ConsulDataplaneContainer {
	{
		node := cluster.Agents[1]
		//client := node.GetClient()

		// Create a service and dataplane
		_, err := createServiceAndDataplane(t, node, "static-server-workload", "static-server", 8080, 8079)
		require.NoError(t, err)

		//libassert.CatalogServiceExists(t, client, "static-server-sidecar-proxy", nil)
		//libassert.CatalogServiceExists(t, client, libservice.StaticServerServiceName, nil)
	}

	{
		node := cluster.Agents[2]
		// Create a service and dataplane
		clientDataplane, err := createServiceAndDataplane(t, node, "static-client-workload", "static-client", 8080, 8079)
		require.NoError(t, err)

		//libassert.CatalogServiceExists(t, client, "static-client-sidecar-proxy", nil)
		return clientDataplane
	}
}

func createServiceAndDataplane(t *testing.T, node libcluster.Agent, proxyID, serviceName string, httpPort, grpcPort int) (*libcluster.ConsulDataplaneContainer, error) {
	// Do some trickery to ensure that partial completion is correctly torn
	// down, but successful execution is not.
	var deferClean utils.ResettableDefer
	defer deferClean.Execute()

	// Create a service and proxy instance
	svc, err := libservice.NewExampleService(context.Background(), serviceName, httpPort, grpcPort, node)
	if err != nil {
		return nil, err
	}
	deferClean.Add(func() {
		_ = svc.Terminate()
	})

	// Create Consul Dataplane
	dp, err := libcluster.NewConsulDataplane(context.Background(), proxyID, "0.0.0.0", 8502, node)
	require.NoError(t, err)
	deferClean.Add(func() {
		_ = dp.Terminate()
	})

	// disable cleanup functions now that we have an object with a Terminate() function
	deferClean.Reset()

	return dp, nil
}

func createServerServicesAndWorkloads(t *testing.T, resourceClient *rtest.Client) *pbresource.Resource {
	serverService := rtest.ResourceID(&pbresource.ID{
		Name:    "static-server-service",
		Type:    catalog.ServiceType,
		Tenancy: resource.DefaultNamespacedTenancy(),
	}).WithData(t, &pbcatalog.Service{
		Workloads: &pbcatalog.WorkloadSelector{Prefixes: []string{"static-server"}},
		Ports: []*pbcatalog.ServicePort{
			{TargetPort: "tcp", Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
			{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
		},
	}).Write(t, resourceClient)

	workloadPortMap := make(map[string]*pbcatalog.WorkloadPort, 2)
	workloadPortMap["tcp"] = &pbcatalog.WorkloadPort{
		Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP,
	}
	workloadPortMap["mesh"] = &pbcatalog.WorkloadPort{
		Port: 20001, Protocol: pbcatalog.Protocol_PROTOCOL_MESH,
	}

	rtest.ResourceID(&pbresource.ID{
		Name:    "static-server-workload",
		Type:    catalog.WorkloadType,
		Tenancy: resource.DefaultNamespacedTenancy(),
	}).
		WithData(t, &pbcatalog.Workload{
			Addresses: []*pbcatalog.WorkloadAddress{
				{Host: "127.0.0.1"},
			},
			Ports:    workloadPortMap,
			Identity: "static-server-identity",
		}).
		Write(t, resourceClient)
	return serverService
}

func createClientServicesAndWorkloads(t *testing.T, resourceClient *rtest.Client, staticServerRef *pbresource.Resource) {
	rtest.ResourceID(&pbresource.ID{
		Name:    "static-client-service",
		Type:    catalog.ServiceType,
		Tenancy: resource.DefaultNamespacedTenancy(),
	}).WithData(t, &pbcatalog.Service{
		Workloads: &pbcatalog.WorkloadSelector{Prefixes: []string{"static-client"}},
		Ports: []*pbcatalog.ServicePort{
			{TargetPort: "tcp", Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
			{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
		},
	}).Write(t, resourceClient)

	workloadPortMap := make(map[string]*pbcatalog.WorkloadPort, 2)
	workloadPortMap["tcp"] = &pbcatalog.WorkloadPort{
		Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP,
	}
	workloadPortMap["mesh"] = &pbcatalog.WorkloadPort{
		Port: 20001, Protocol: pbcatalog.Protocol_PROTOCOL_MESH,
	}

	rtest.ResourceID(&pbresource.ID{
		Name:    "static-client-workload",
		Type:    catalog.WorkloadType,
		Tenancy: resource.DefaultNamespacedTenancy(),
	}).
		WithData(t, &pbcatalog.Workload{
			Addresses: []*pbcatalog.WorkloadAddress{
				{Host: "127.0.0.1"},
			},
			Ports:    workloadPortMap,
			Identity: "static-client-identity",
		}).
		Write(t, resourceClient)

	rtest.ResourceID(&pbresource.ID{
		Name:    "static-client-upstreams",
		Type:    mesh.UpstreamsType,
		Tenancy: resource.DefaultNamespacedTenancy(),
	}).
		WithData(t, &pbmesh.Upstreams{
			Upstreams: []*pbmesh.Upstream{
				{
					DestinationRef:  resource.Reference(staticServerRef.GetId(), ""),
					DestinationPort: "tcp",
					ListenAddr: &pbmesh.Upstream_IpPort{
						IpPort: &pbmesh.IPPortAddress{
							Ip:   "127.0.0.1",
							Port: 1234,
						},
					},
				},
			},
			Workloads: &pbcatalog.WorkloadSelector{
				Prefixes: []string{"static-client"},
			},
		}).
		Write(t, resourceClient)
}

func createClientUpstreams(t *testing.T, resourceClient *rtest.Client, staticServerRef *pbresource.Resource, portNumber int) {
	rtest.ResourceID(&pbresource.ID{
		Name:    "static-client-upstreams",
		Type:    mesh.UpstreamsType,
		Tenancy: resource.DefaultNamespacedTenancy(),
	}).
		WithData(t, &pbmesh.Upstreams{
			Upstreams: []*pbmesh.Upstream{
				{
					DestinationRef:  resource.Reference(staticServerRef.GetId(), ""),
					DestinationPort: "tcp",
					ListenAddr: &pbmesh.Upstream_IpPort{
						IpPort: &pbmesh.IPPortAddress{
							Ip:   "127.0.0.1",
							Port: uint32(portNumber),
						},
					},
				},
			},
			Workloads: &pbcatalog.WorkloadSelector{
				Prefixes: []string{"static-client"},
			},
		}).
		Write(t, resourceClient)
}

func createCluster(t *testing.T) *libcluster.Cluster {
	cluster, _, _ := topology.NewCluster(t, &topology.ClusterConfig{
		NumServers: 3,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter:             "dc1",
			InjectAutoEncryption:   true,
			InjectGossipEncryption: true,
			AllowHTTPAnyway:        true,
		},
		Cmd: `-hcl=experiments=["resource-apis"] log_level="TRACE"`,
	})

	return cluster
}

// assertDataplaneContainerState validates service container status
func assertDataplaneContainerState(t *testing.T, dataplane *libcluster.ConsulDataplaneContainer, state string) {
	containerStatus, err := dataplane.GetStatus()
	require.NoError(t, err)
	require.Equal(t, containerStatus, state, fmt.Sprintf("Expected: %s. Got %s", state, containerStatus))
}

// assertHTTPRequestToServiceAddress checks the result of a request from the
// given `client` container to the given `server` container. If expSuccess is
// true, this checks for a successful request and otherwise it checks for the
// error we expect when traffic is rejected by mTLS.
//
// This assumes the destination service is running Fortio. It makes the request
// to `<serverIP>:8080/debug?env=dump` and checks for `FORTIO_NAME=<expServiceName>`
// in the response.
func assertHTTPRequestToServiceAddress(t *testing.T, client, server libcluster.Agent, expServiceName string, expSuccess bool) {
	upstreamURL := fmt.Sprintf("http://%s:8080/debug?env=dump", server.GetIP())
	retry.RunWith(requestRetryTimer, t, func(r *retry.R) {
		out, err := client.Exec(context.Background(), []string{"curl", "-s", upstreamURL})
		t.Logf("curl request to upstream service address: url=%s\nerr = %v\nout = %s", upstreamURL, err, out)

		if expSuccess {
			require.NoError(r, err)
			require.Contains(r, out, fmt.Sprintf("FORTIO_NAME=%s", expServiceName))
		} else {
			require.Error(r, err)
			require.Contains(r, err.Error(), "exit code 52")
		}
	})
}
