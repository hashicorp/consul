package upgrade

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"
)

// TestTrafficManagement_ServiceResolverSubsetRedirect Summary
// This test starts up 4 servers and 1 client in the same datacenter.
//
// Steps:
//   - Create a single agent cluster.
//   - Create 2 static-servers, 2 subset servers and 1 client and sidecars for all services, then register them with Consul
//   - Validate traffic is successfully redirected from server 1 to sever2-v2 as defined in the service resolver
func TestTrafficManagement_ServiceResolverSubsetRedirect(t *testing.T) {
	t.Parallel()

	type testcase struct {
		oldversion    string
		targetVersion string
	}
	tcs := []testcase{
		{
			oldversion:    "1.13",
			targetVersion: utils.TargetVersion,
		},
		{
			oldversion:    "1.14",
			targetVersion: utils.TargetVersion,
		},
	}

	run := func(t *testing.T, tc testcase) {
		buildOpts := &libcluster.BuildOptions{
			ConsulVersion:        tc.oldversion,
			Datacenter:           "dc1",
			InjectAutoEncryption: true,
		}
		// If version < 1.14 disable AutoEncryption
		oldVersion, _ := version.NewVersion(tc.oldversion)
		if oldVersion.LessThan(utils.Version_1_14) {
			buildOpts.InjectAutoEncryption = false
		}
		cluster, _, _ := topology.NewPeeringCluster(t, 1, buildOpts)
		node := cluster.Agents[0]

		// Register static-server service resolver
		serviceResolver := &api.ServiceResolverConfigEntry{
			Kind: api.ServiceResolver,
			Name: libservice.StaticServer2ServiceName,
			Subsets: map[string]api.ServiceResolverSubset{
				"v1": {
					Filter: "Service.Meta.version == v1",
				},
				"v2": {
					Filter: "Service.Meta.version == v2",
				},
			},
		}
		err := cluster.ConfigEntryWrite(serviceResolver)
		require.NoError(t, err)

		// Register static-server-2 service resolver to redirect traffic
		// from static-server to static-server-2-v2
		service2Resolver := &api.ServiceResolverConfigEntry{
			Kind: api.ServiceResolver,
			Name: libservice.StaticServerServiceName,
			Redirect: &api.ServiceResolverRedirect{
				Service:       libservice.StaticServer2ServiceName,
				ServiceSubset: "v2",
			},
		}
		err = cluster.ConfigEntryWrite(service2Resolver)
		require.NoError(t, err)

		// register agent services
		agentServices := setupServiceAndSubsets(t, cluster)
		assertionFn, proxyRestartFn := agentServices.validateAgentServices(t)
		_, port := agentServices.client.GetAddr()
		_, adminPort := agentServices.client.GetAdminAddr()

		// validate static-client is up and running
		libassert.AssertContainerState(t, agentServices.client, "running")
		libassert.HTTPServiceEchoes(t, "localhost", port, "")
		libassert.AssertFortioName(t, fmt.Sprintf("http://localhost:%d", port), "static-server-2-v2")
		assertionFn()

		// Upgrade cluster, restart sidecars then begin service traffic validation
		require.NoError(t, cluster.StandardUpgrade(t, context.Background(), tc.targetVersion))
		proxyRestartFn()

		libassert.AssertContainerState(t, agentServices.client, "running")
		libassert.HTTPServiceEchoes(t, "localhost", port, "")
		libassert.AssertFortioName(t, fmt.Sprintf("http://localhost:%d", port), "static-server-2-v2")
		assertionFn()

		// assert 3 static-server instances are healthy
		libassert.AssertServiceHasHealthyInstances(t, node, libservice.StaticServer2ServiceName, false, 3)
		libassert.AssertUpstreamEndpointStatus(t, adminPort, "v2.static-server-2.default", "HEALTHY", 1)
	}

	for _, tc := range tcs {
		t.Run(fmt.Sprintf("upgrade from %s to %s", tc.oldversion, tc.targetVersion),
			func(t *testing.T) {
				run(t, tc)
			})
		// test sometimes fails with error: could not start or join all agents: could not add container index 0: port not found
		time.Sleep(1 * time.Second)
	}
}

func (s *registeredServices) validateAgentServices(t *testing.T) (func(), func()) {
	var (
		responseFormat = map[string]string{"format": "json"}
		proxyRestartFn func()
		assertionFn    func()
	)
	// validate services proxy admin is up
	assertionFn = func() {
		for serviceName, proxies := range s.services {
			for _, proxy := range proxies {
				_, adminPort := proxy.GetAdminAddr()
				_, statusCode, err := libassert.GetEnvoyOutput(adminPort, "stats", responseFormat)
				require.NoError(t, err)
				assert.Equal(t, http.StatusOK, statusCode, fmt.Sprintf("%s cannot be reached %v", serviceName, statusCode))

				// certs are valid
				libassert.AssertEnvoyPresentsCertURI(t, adminPort, serviceName)
			}
		}
	}

	for _, serviceConnectProxy := range s.services {
		for _, proxy := range serviceConnectProxy {
			proxyRestartFn = func() { require.NoErrorf(t, proxy.Restart(), "%s", proxy.GetName()) }
		}
	}
	return assertionFn, proxyRestartFn
}

type registeredServices struct {
	client   libservice.Service
	services map[string][]libservice.Service
}

// create 3 servers and 1 client
func setupServiceAndSubsets(t *testing.T, cluster *libcluster.Cluster) *registeredServices {
	node := cluster.Agents[0]
	client := node.GetClient()

	// create static-servers and subsets
	serviceOpts := &libservice.ServiceOpts{
		Name:     libservice.StaticServerServiceName,
		ID:       "static-server",
		HTTPPort: 8080,
		GRPCPort: 8079,
	}
	_, serverConnectProxy, err := libservice.CreateAndRegisterStaticServerAndSidecar(node, serviceOpts)
	require.NoError(t, err)
	libassert.CatalogServiceExists(t, client, libservice.StaticServerServiceName)

	serviceOpts2 := &libservice.ServiceOpts{
		Name:     libservice.StaticServer2ServiceName,
		ID:       "static-server-2",
		HTTPPort: 8081,
		GRPCPort: 8078,
	}
	_, server2ConnectProxy, err := libservice.CreateAndRegisterStaticServerAndSidecar(node, serviceOpts2)
	require.NoError(t, err)
	libassert.CatalogServiceExists(t, client, libservice.StaticServer2ServiceName)

	serviceOptsV1 := &libservice.ServiceOpts{
		Name:     libservice.StaticServer2ServiceName,
		ID:       "static-server-2-v1",
		Meta:     map[string]string{"version": "v1"},
		HTTPPort: 8082,
		GRPCPort: 8077,
	}
	_, server2ConnectProxyV1, err := libservice.CreateAndRegisterStaticServerAndSidecar(node, serviceOptsV1)
	require.NoError(t, err)
	libassert.CatalogServiceExists(t, client, libservice.StaticServer2ServiceName)

	serviceOptsV2 := &libservice.ServiceOpts{
		Name:     libservice.StaticServer2ServiceName,
		ID:       "static-server-2-v2",
		Meta:     map[string]string{"version": "v2"},
		HTTPPort: 8083,
		GRPCPort: 8076,
	}
	_, server2ConnectProxyV2, err := libservice.CreateAndRegisterStaticServerAndSidecar(node, serviceOptsV2)
	require.NoError(t, err)
	libassert.CatalogServiceExists(t, client, libservice.StaticServer2ServiceName)

	// Create a client proxy instance with the server as an upstream
	clientConnectProxy, err := libservice.CreateAndRegisterStaticClientSidecar(node, "", false)
	require.NoError(t, err)
	libassert.CatalogServiceExists(t, client, fmt.Sprintf("%s-sidecar-proxy", libservice.StaticClientServiceName))

	// return a map of all services created
	tmpServices := map[string][]libservice.Service{}
	tmpServices[libservice.StaticClientServiceName] = append(tmpServices[libservice.StaticClientServiceName], clientConnectProxy)
	tmpServices[libservice.StaticServerServiceName] = append(tmpServices[libservice.StaticServerServiceName], serverConnectProxy)
	tmpServices[libservice.StaticServer2ServiceName] = append(tmpServices[libservice.StaticServer2ServiceName], server2ConnectProxy)
	tmpServices[libservice.StaticServer2ServiceName] = append(tmpServices[libservice.StaticServer2ServiceName], server2ConnectProxyV1)
	tmpServices[libservice.StaticServer2ServiceName] = append(tmpServices[libservice.StaticServer2ServiceName], server2ConnectProxyV2)

	return &registeredServices{
		client:   clientConnectProxy,
		services: tmpServices,
	}
}
