package gateways

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/hashicorp/consul/api"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

var (
	checkTimeout  = 1 * time.Minute
	checkInterval = 1 * time.Second
)

// Creates a gateway service and tests to see if it is routable
func TestAPIGatewayCreate(t *testing.T) {
	t.Skip()
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	listenerPortOne := 6000

	cluster := createCluster(t, listenerPortOne)

	client := cluster.APIClient(0)

	//setup
	apiGateway := &api.APIGatewayConfigEntry{
		Kind: "api-gateway",
		Name: "api-gateway",
		Listeners: []api.APIGatewayListener{
			{
				Port:     listenerPortOne,
				Protocol: "tcp",
			},
		},
	}
	_, _, err := client.ConfigEntries().Set(apiGateway, nil)
	assert.NoError(t, err)

	tcpRoute := &api.TCPRouteConfigEntry{
		Kind: "tcp-route",
		Name: "api-gateway-route",
		Parents: []api.ResourceReference{
			{
				Kind: "api-gateway",
				Name: "api-gateway",
			},
		},
		Services: []api.TCPService{
			{
				Name: libservice.StaticServerServiceName,
			},
		},
	}

	_, _, err = client.ConfigEntries().Set(tcpRoute, nil)
	assert.NoError(t, err)

	// Create a client proxy instance with the server as an upstream
	_, gatewayService := createServices(t, cluster, listenerPortOne)

	//TODO this can and should be broken up more effectively, this is just proof of concept
	//check statuses
	gatewayReady := false
	routeReady := false

	for !gatewayReady || !routeReady {
		//check status
		entry, _, err := client.ConfigEntries().Get("api-gateway", "api-gateway", nil)
		assert.NoError(t, err)
		apiEntry := entry.(*api.APIGatewayConfigEntry)
		gatewayReady = isAccepted(apiEntry.Status.Conditions)

		e, _, err := client.ConfigEntries().Get("tcp-route", "api-gateway-route", nil)
		assert.NoError(t, err)
		routeEntry := e.(*api.TCPRouteConfigEntry)
		routeReady = isBound(routeEntry.Status.Conditions)
	}

	libassert.HTTPServiceEchoes(t, "localhost", gatewayService.GetPort(listenerPortOne), "", nil)
}

func isAccepted(conditions []api.Condition) bool {
	return conditionStatusIsValue("Accepted", "True", conditions)
}

func isBound(conditions []api.Condition) bool {
	return conditionStatusIsValue("Bound", "True", conditions)
}

func conditionStatusIsValue(typeName string, statusValue string, conditions []api.Condition) bool {
	for _, c := range conditions {
		if c.Type == typeName && c.Status == statusValue {
			return true
		}
	}
	return false
}

// TODO this code is just copy pasted from elsewhere, it is likely we will need to modify it some
func createCluster(t *testing.T, ports ...int) *libcluster.Cluster {
	opts := libcluster.BuildOptions{
		InjectAutoEncryption:   true,
		InjectGossipEncryption: true,
		AllowHTTPAnyway:        true,
	}
	ctx := libcluster.NewBuildContext(t, opts)

	conf := libcluster.NewConfigBuilder(ctx).
		ToAgentConfig(t)
	t.Logf("Cluster config:\n%s", conf.JSON)

	configs := []libcluster.Config{*conf}

	cluster, err := libcluster.New(t, configs, ports...)
	require.NoError(t, err)

	node := cluster.Agents[0]
	client := node.GetClient()

	libcluster.WaitForLeader(t, cluster, client)
	libcluster.WaitForMembers(t, client, 1)

	// Default Proxy Settings
	ok, err := utils.ApplyDefaultProxySettings(client)
	require.NoError(t, err)
	require.True(t, ok)

	require.NoError(t, err)

	return cluster
}

func createService(t *testing.T, cluster *libcluster.Cluster, serviceOpts *libservice.ServiceOpts, ports ...int) libservice.Service {
	node := cluster.Agents[0]
	client := node.GetClient()
	// Create a service and proxy instance

	// Create a service and proxy instance
	_, _, err := libservice.CreateAndRegisterStaticServerAndSidecar(node, serviceOpts)
	require.NoError(t, err)

	libassert.CatalogServiceExists(t, client, serviceOpts.Name+"-sidecar-proxy")
	libassert.CatalogServiceExists(t, client, serviceOpts.Name)

	// Create a client proxy instance with the server as an upstream
	//TODO this is always going to be named static-client-sidecar-proxy and I don't know if that matters
	clientConnectProxy, err := libservice.CreateAndRegisterStaticClientSidecar(node, "", false)
	require.NoError(t, err)
	libassert.CatalogServiceExists(t, client, "static-client-sidecar-proxy")
	return clientConnectProxy

}
func createServices(t *testing.T, cluster *libcluster.Cluster, ports ...int) (libservice.Service, libservice.Service) {
	node := cluster.Agents[0]
	client := node.GetClient()
	// Create a service and proxy instance
	serviceOpts := &libservice.ServiceOpts{
		Name:     libservice.StaticServerServiceName,
		ID:       "static-server",
		HTTPPort: 8080,
		GRPCPort: 8079,
	}

	clientConnectProxy := createService(t, cluster, serviceOpts, ports...)

	gatewayService, err := libservice.NewGatewayService(context.Background(), "api-gateway", "api", cluster.Agents[0], ports...)
	require.NoError(t, err)
	libassert.CatalogServiceExists(t, client, "api-gateway")

	return clientConnectProxy, gatewayService
}

func checkRoute(t *testing.T, port int, path string, expectedStatusCode int, expectedBody string, headers map[string]string, message string) {
	t.Helper()

	require.Eventually(t, func() bool {
		client := &http.Client{Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}}
		req, err := http.NewRequest("GET", fmt.Sprintf("https://localhost:%d%s", port, path), nil)
		if err != nil {
			return false
		}

		for k, v := range headers {
			req.Header.Set(k, v)

			if k == "Host" {
				req.Host = v
			}
		}

		resp, err := client.Do(req)
		if err != nil {
			t.Log(err)
			return false
		}
		defer resp.Body.Close()

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Log(err)
			return false
		}
		t.Log(string(data))

		if resp.StatusCode != expectedStatusCode {
			t.Log("status code", resp.StatusCode)
			return false
		}

		return strings.HasPrefix(string(data), expectedBody)
	}, checkTimeout, checkInterval, message)
}
