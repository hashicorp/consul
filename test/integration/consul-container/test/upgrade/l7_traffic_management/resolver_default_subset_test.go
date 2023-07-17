package upgrade

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	libutils "github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	upgrade "github.com/hashicorp/consul/test/integration/consul-container/test/upgrade"
	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/require"
)

// TestTrafficManagement_ServiceResolver tests that upgraded cluster inherits and interpret
// the resolver config entry correctly.
//
// The basic topology is a cluster with one static-client and one static-server. Addtional
// services and resolver can be added to the create func() for each test cases.
func TestTrafficManagement_ServiceResolver(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name string
		// create creates addtional resources in the cluster depending on cases, e.g., static-client,
		// static server, and config-entries. It returns the proxy services of the client, an assertation
		// function to be called to verify the resources, and a restartFn to be called after upgrade.
		create func(*libcluster.Cluster, libservice.Service) (libservice.Service, func(), func(), error)
		// extraAssertion adds additional assertion function to the common resources across cases.
		// common resources includes static-client in dialing cluster, and static-server in accepting cluster.
		//
		// extraAssertion needs to be run before and after upgrade
		extraAssertion func(libservice.Service)
	}
	tcs := []testcase{
		{
			// Test resolver directs traffic to default subset
			// - Create 2 additional static-server instances: one in V1 subset and the other in V2 subset
			// - resolver directs traffic to the default subset, which is V2.
			name: "resolver default subset",
			create: func(cluster *libcluster.Cluster, clientConnectProxy libservice.Service) (libservice.Service, func(), func(), error) {
				node := cluster.Agents[0]
				client := node.GetClient()

				// Create static-server-v1 and static-server-v2
				serviceOptsV1 := &libservice.ServiceOpts{
					Name:     libservice.StaticServerServiceName,
					ID:       "static-server-v1",
					Meta:     map[string]string{"version": "v1"},
					HTTPPort: 8081,
					GRPCPort: 8078,
				}
				_, serverConnectProxyV1, err := libservice.CreateAndRegisterStaticServerAndSidecar(node, serviceOptsV1)
				require.NoError(t, err)

				serviceOptsV2 := &libservice.ServiceOpts{
					Name:     libservice.StaticServerServiceName,
					ID:       "static-server-v2",
					Meta:     map[string]string{"version": "v2"},
					HTTPPort: 8082,
					GRPCPort: 8077,
				}
				_, serverConnectProxyV2, err := libservice.CreateAndRegisterStaticServerAndSidecar(node, serviceOptsV2)
				require.NoError(t, err)
				libassert.CatalogServiceExists(t, client, "static-server", nil)

				// TODO: verify the number of instance of static-server is 3
				libassert.AssertServiceHasHealthyInstances(t, node, libservice.StaticServerServiceName, true, 3)

				// Register service resolver
				serviceResolver := &api.ServiceResolverConfigEntry{
					Kind:          api.ServiceResolver,
					Name:          libservice.StaticServerServiceName,
					DefaultSubset: "v2",
					Subsets: map[string]api.ServiceResolverSubset{
						"v1": {
							Filter: "Service.Meta.version == v1",
						},
						"v2": {
							Filter: "Service.Meta.version == v2",
						},
					},
				}
				err = cluster.ConfigEntryWrite(serviceResolver)
				if err != nil {
					return nil, nil, nil, fmt.Errorf("error writing config entry %s", err)
				}

				_, serverAdminPortV1 := serverConnectProxyV1.GetAdminAddr()
				_, serverAdminPortV2 := serverConnectProxyV2.GetAdminAddr()

				restartFn := func() {
					require.NoError(t, serverConnectProxyV1.Restart())
					require.NoError(t, serverConnectProxyV2.Restart())
				}

				_, adminPort := clientConnectProxy.GetAdminAddr()
				assertionFn := func() {
					libassert.AssertEnvoyRunning(t, serverAdminPortV1)
					libassert.AssertEnvoyRunning(t, serverAdminPortV2)

					libassert.AssertEnvoyPresentsCertURI(t, serverAdminPortV1, "static-server")
					libassert.AssertEnvoyPresentsCertURI(t, serverAdminPortV2, "static-server")

					libassert.AssertUpstreamEndpointStatus(t, adminPort, "v2.static-server.default", "HEALTHY", 1)

					// assert static-server proxies should be healthy
					libassert.AssertServiceHasHealthyInstances(t, node, libservice.StaticServerServiceName, true, 3)
				}
				return nil, assertionFn, restartFn, nil
			},
			extraAssertion: func(clientConnectProxy libservice.Service) {
				_, port := clientConnectProxy.GetAddr()
				_, adminPort := clientConnectProxy.GetAdminAddr()

				libassert.AssertUpstreamEndpointStatus(t, adminPort, "v2.static-server.default", "HEALTHY", 1)

				// static-client upstream should connect to static-server-v2 because the default subset value is to v2 set in the service resolver
				libassert.AssertFortioName(t, fmt.Sprintf("http://localhost:%d", port), "static-server-v2", "")
			},
		},
		{
			// Test resolver resolves service instance based on their check status
			// - Create one addtional static-server with checks and V1 subset
			// - resolver directs traffic to "test" service
			name: "resolver default onlypassing",
			create: func(cluster *libcluster.Cluster, clientConnectProxy libservice.Service) (libservice.Service, func(), func(), error) {
				node := cluster.Agents[0]

				serviceOptsV1 := &libservice.ServiceOpts{
					Name:     libservice.StaticServerServiceName,
					ID:       "static-server-v1",
					Meta:     map[string]string{"version": "v1"},
					HTTPPort: 8081,
					GRPCPort: 8078,
					Checks: libservice.Checks{
						Name: "main",
						TTL:  "30m",
					},
					Connect: libservice.SidecarService{
						Port: 21011,
					},
				}
				_, serverConnectProxyV1, err := libservice.CreateAndRegisterStaticServerAndSidecarWithChecks(node, serviceOptsV1)
				require.NoError(t, err)

				// Register service resolver
				serviceResolver := &api.ServiceResolverConfigEntry{
					Kind:          api.ServiceResolver,
					Name:          libservice.StaticServerServiceName,
					DefaultSubset: "test",
					Subsets: map[string]api.ServiceResolverSubset{
						"test": {
							OnlyPassing: true,
						},
					},
					ConnectTimeout: 120 * time.Second,
				}
				_, serverAdminPortV1 := serverConnectProxyV1.GetAdminAddr()

				restartFn := func() {
					require.NoError(t, serverConnectProxyV1.Restart())
				}

				_, port := clientConnectProxy.GetAddr()
				_, adminPort := clientConnectProxy.GetAdminAddr()
				assertionFn := func() {
					// force static-server-v1 into a warning state
					err = node.GetClient().Agent().UpdateTTL("service:static-server-v1", "", "warn")
					require.NoError(t, err)

					// ###########################
					// ## with onlypassing=true
					// assert only one static-server proxy is healthy
					err = cluster.ConfigEntryWrite(serviceResolver)
					require.NoError(t, err)
					libassert.AssertServiceHasHealthyInstances(t, node, libservice.StaticServerServiceName, true, 1)

					libassert.AssertEnvoyRunning(t, serverAdminPortV1)
					libassert.AssertEnvoyPresentsCertURI(t, serverAdminPortV1, "static-server")

					// assert static-server proxies should be healthy
					libassert.AssertServiceHasHealthyInstances(t, node, libservice.StaticServerServiceName, true, 1)

					// static-client upstream should have 1 healthy endpoint for test.static-server
					libassert.AssertUpstreamEndpointStatus(t, adminPort, "test.static-server.default", "HEALTHY", 1)

					// static-client upstream should have 1 unhealthy endpoint for test.static-server
					libassert.AssertUpstreamEndpointStatus(t, adminPort, "test.static-server.default", "UNHEALTHY", 1)

					// static-client upstream should connect to static-server since it is passing
					libassert.AssertFortioName(t, fmt.Sprintf("http://localhost:%d", port), libservice.StaticServerServiceName, "")

					// ###########################
					// ## with onlypassing=false
					// revert to OnlyPassing=false by deleting the config
					err = cluster.ConfigEntryDelete(serviceResolver)
					require.NoError(t, err)

					// Consul health check assert only one static-server proxy is healthy when onlyPassing is false
					libassert.AssertServiceHasHealthyInstances(t, node, libservice.StaticServerServiceName, false, 2)

					// Although the service status is in warning state, when onlypassing is set to false Envoy
					// health check returns all service instances with "warning" or "passing" state as Healthy enpoints
					libassert.AssertUpstreamEndpointStatus(t, adminPort, "static-server.default", "HEALTHY", 2)

					// static-client upstream should have 0 unhealthy endpoint for static-server
					libassert.AssertUpstreamEndpointStatus(t, adminPort, "static-server.default", "UNHEALTHY", 0)

				}
				return nil, assertionFn, restartFn, nil
			},
			extraAssertion: func(clientConnectProxy libservice.Service) {
			},
		},
		{
			// Test resolver directs traffic to default subset
			// - Create 3 static-server-2 server instances: one in V1, one in V2, one without any version
			// - service2Resolver directs traffic to static-server-2-v2
			name: "resolver subset redirect",
			create: func(cluster *libcluster.Cluster, clientConnectProxy libservice.Service) (libservice.Service, func(), func(), error) {
				node := cluster.Agents[0]
				client := node.GetClient()

				serviceOpts2 := &libservice.ServiceOpts{
					Name:     libservice.StaticServer2ServiceName,
					ID:       "static-server-2",
					HTTPPort: 8081,
					GRPCPort: 8078,
				}
				_, server2ConnectProxy, err := libservice.CreateAndRegisterStaticServerAndSidecar(node, serviceOpts2)
				require.NoError(t, err)
				libassert.CatalogServiceExists(t, client, libservice.StaticServer2ServiceName, nil)

				serviceOptsV1 := &libservice.ServiceOpts{
					Name:     libservice.StaticServer2ServiceName,
					ID:       "static-server-2-v1",
					Meta:     map[string]string{"version": "v1"},
					HTTPPort: 8082,
					GRPCPort: 8077,
				}
				_, server2ConnectProxyV1, err := libservice.CreateAndRegisterStaticServerAndSidecar(node, serviceOptsV1)
				require.NoError(t, err)

				serviceOptsV2 := &libservice.ServiceOpts{
					Name:     libservice.StaticServer2ServiceName,
					ID:       "static-server-2-v2",
					Meta:     map[string]string{"version": "v2"},
					HTTPPort: 8083,
					GRPCPort: 8076,
				}
				_, server2ConnectProxyV2, err := libservice.CreateAndRegisterStaticServerAndSidecar(node, serviceOptsV2)
				require.NoError(t, err)
				libassert.CatalogServiceExists(t, client, libservice.StaticServer2ServiceName, nil)

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
				err = cluster.ConfigEntryWrite(serviceResolver)
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

				_, server2AdminPort := server2ConnectProxy.GetAdminAddr()
				_, server2AdminPortV1 := server2ConnectProxyV1.GetAdminAddr()
				_, server2AdminPortV2 := server2ConnectProxyV2.GetAdminAddr()

				restartFn := func() {
					require.NoErrorf(t, server2ConnectProxy.Restart(), "%s", server2ConnectProxy.GetName())
					require.NoErrorf(t, server2ConnectProxyV1.Restart(), "%s", server2ConnectProxyV1.GetName())
					require.NoErrorf(t, server2ConnectProxyV2.Restart(), "%s", server2ConnectProxyV2.GetName())
				}

				assertionFn := func() {
					// assert 3 static-server-2 instances are healthy
					libassert.AssertServiceHasHealthyInstances(t, node, libservice.StaticServer2ServiceName, false, 3)

					libassert.AssertEnvoyRunning(t, server2AdminPort)
					libassert.AssertEnvoyRunning(t, server2AdminPortV1)
					libassert.AssertEnvoyRunning(t, server2AdminPortV2)

					libassert.AssertEnvoyPresentsCertURI(t, server2AdminPort, libservice.StaticServer2ServiceName)
					libassert.AssertEnvoyPresentsCertURI(t, server2AdminPortV1, libservice.StaticServer2ServiceName)
					libassert.AssertEnvoyPresentsCertURI(t, server2AdminPortV2, libservice.StaticServer2ServiceName)

					// assert static-server proxies should be healthy
					libassert.AssertServiceHasHealthyInstances(t, node, libservice.StaticServer2ServiceName, true, 3)
				}
				return nil, assertionFn, restartFn, nil
			},
			extraAssertion: func(clientConnectProxy libservice.Service) {
				_, appPort := clientConnectProxy.GetAddr()
				_, adminPort := clientConnectProxy.GetAdminAddr()

				libassert.AssertFortioName(t, fmt.Sprintf("http://localhost:%d", appPort), "static-server-2-v2", "")
				libassert.AssertUpstreamEndpointStatus(t, adminPort, "v2.static-server-2.default", "HEALTHY", 1)
			},
		},
	}

	run := func(t *testing.T, tc testcase, oldVersion, targetVersion string) {
		buildOpts := &libcluster.BuildOptions{
			ConsulVersion:        oldVersion,
			Datacenter:           "dc1",
			InjectAutoEncryption: true,
		}
		// If version < 1.14 disable AutoEncryption
		oldVersionTmp, _ := version.NewVersion(oldVersion)
		if oldVersionTmp.LessThan(libutils.Version_1_14) {
			buildOpts.InjectAutoEncryption = false
		}
		cluster, _, _ := topology.NewCluster(t, &topology.ClusterConfig{
			NumServers:                1,
			NumClients:                1,
			BuildOpts:                 buildOpts,
			ApplyDefaultProxySettings: true,
		})
		node := cluster.Agents[0]
		client := node.GetClient()

		staticClientProxy, staticServerProxy, err := createStaticClientAndServer(cluster)
		require.NoError(t, err)
		libassert.CatalogServiceExists(t, client, libservice.StaticServerServiceName, nil)
		libassert.CatalogServiceExists(t, client, fmt.Sprintf("%s-sidecar-proxy", libservice.StaticClientServiceName), nil)

		err = cluster.ConfigEntryWrite(&api.ProxyConfigEntry{
			Kind: api.ProxyDefaults,
			Name: "global",
			Config: map[string]interface{}{
				"protocol": "http",
			},
		})
		require.NoError(t, err)

		_, port := staticClientProxy.GetAddr()
		_, adminPort := staticClientProxy.GetAdminAddr()
		_, serverAdminPort := staticServerProxy.GetAdminAddr()
		libassert.HTTPServiceEchoes(t, "localhost", port, "")
		libassert.AssertEnvoyPresentsCertURI(t, adminPort, libservice.StaticClientServiceName)
		libassert.AssertEnvoyPresentsCertURI(t, serverAdminPort, libservice.StaticServerServiceName)

		_, assertionAdditionalResources, restartFn, err := tc.create(cluster, staticClientProxy)
		require.NoError(t, err)
		// validate client and proxy is up and running
		libassert.AssertContainerState(t, staticClientProxy, "running")
		assertionAdditionalResources()
		tc.extraAssertion(staticClientProxy)

		// Upgrade cluster, restart sidecars then begin service traffic validation
		require.NoError(t, cluster.StandardUpgrade(t, context.Background(), utils.GetTargetImageName(), targetVersion))
		require.NoError(t, staticClientProxy.Restart())
		require.NoError(t, staticServerProxy.Restart())
		restartFn()

		// POST upgrade validation; repeat client & proxy validation
		libassert.HTTPServiceEchoes(t, "localhost", port, "")
		libassert.AssertEnvoyRunning(t, adminPort)
		libassert.AssertEnvoyRunning(t, serverAdminPort)

		// certs are valid
		libassert.AssertEnvoyPresentsCertURI(t, adminPort, libservice.StaticClientServiceName)
		libassert.AssertEnvoyPresentsCertURI(t, serverAdminPort, libservice.StaticServerServiceName)

		assertionAdditionalResources()
		tc.extraAssertion(staticClientProxy)
	}

	targetVersion := libutils.TargetVersion
	for _, oldVersion := range upgrade.UpgradeFromVersions {
		for _, tc := range tcs {
			t.Run(fmt.Sprintf("%s upgrade from %s to %s", tc.name, oldVersion, targetVersion),
				func(t *testing.T) {
					run(t, tc, oldVersion, targetVersion)
				})
		}
	}
}

// createStaticClientAndServer creates a static-client and a static-server in the cluster
func createStaticClientAndServer(cluster *libcluster.Cluster) (libservice.Service, libservice.Service, error) {
	node := cluster.Agents[0]
	serviceOpts := &libservice.ServiceOpts{
		Name:     libservice.StaticServerServiceName,
		ID:       "static-server",
		HTTPPort: 8080,
		GRPCPort: 8079,
	}
	_, serverConnectProxy, err := libservice.CreateAndRegisterStaticServerAndSidecar(node, serviceOpts)
	if err != nil {
		return nil, nil, err
	}

	// Create a client proxy instance with the server as an upstream
	clientConnectProxy, err := libservice.CreateAndRegisterStaticClientSidecar(node, "", false)
	if err != nil {
		return nil, nil, err
	}

	return clientConnectProxy, serverConnectProxy, nil
}
