package gateways

import (
	"context"
	"fmt"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	"github.com/hashicorp/go-cleanhttp"
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
	require.NoError(t, err)

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
	require.NoError(t, err)

	// Create a client proxy instance with the server as an upstream
	_, gatewayService := createServices(t, cluster, listenerPortOne)

	//make sure the gateway/route come online
	//make sure config entries have been properly created
	checkGatewayConfigEntry(t, client, "api-gateway", "")
	checkTCPRouteConfigEntry(t, client, "api-gateway-route", "")

	port, err := gatewayService.GetPort(listenerPortOne)
	require.NoError(t, err)
	libassert.HTTPServiceEchoes(t, "localhost", port, "")
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

func createGateway(gatewayName string, protocol string, listenerPort int) *api.APIGatewayConfigEntry {
	return &api.APIGatewayConfigEntry{
		Kind: api.APIGateway,
		Name: gatewayName,
		Listeners: []api.APIGatewayListener{
			{
				Name:     "listener",
				Port:     listenerPort,
				Protocol: protocol,
			},
		},
	}
}

func checkGatewayConfigEntry(t *testing.T, client *api.Client, gatewayName string, namespace string) {
	require.Eventually(t, func() bool {
		entry, _, err := client.ConfigEntries().Get(api.APIGateway, gatewayName, &api.QueryOptions{Namespace: namespace})
		require.NoError(t, err)
		if entry == nil {
			return false
		}
		apiEntry := entry.(*api.APIGatewayConfigEntry)
		return isAccepted(apiEntry.Status.Conditions)
	}, time.Second*10, time.Second*1)
}

func checkHTTPRouteConfigEntry(t *testing.T, client *api.Client, routeName string, namespace string) {
	require.Eventually(t, func() bool {
		entry, _, err := client.ConfigEntries().Get(api.HTTPRoute, routeName, &api.QueryOptions{Namespace: namespace})
		require.NoError(t, err)
		if entry == nil {
			return false
		}

		apiEntry := entry.(*api.HTTPRouteConfigEntry)
		return isBound(apiEntry.Status.Conditions)
	}, time.Second*10, time.Second*1)
}

func checkTCPRouteConfigEntry(t *testing.T, client *api.Client, routeName string, namespace string) {
	require.Eventually(t, func() bool {
		entry, _, err := client.ConfigEntries().Get(api.TCPRoute, routeName, &api.QueryOptions{Namespace: namespace})
		require.NoError(t, err)
		if entry == nil {
			return false
		}

		apiEntry := entry.(*api.TCPRouteConfigEntry)
		return isBound(apiEntry.Status.Conditions)
	}, time.Second*10, time.Second*1)
}

func createService(t *testing.T, cluster *libcluster.Cluster, serviceOpts *libservice.ServiceOpts, containerArgs []string) libservice.Service {
	node := cluster.Agents[0]
	client := node.GetClient()
	// Create a service and proxy instance
	service, _, err := libservice.CreateAndRegisterStaticServerAndSidecar(node, serviceOpts, containerArgs...)
	require.NoError(t, err)

	libassert.CatalogServiceExists(t, client, serviceOpts.Name+"-sidecar-proxy")
	libassert.CatalogServiceExists(t, client, serviceOpts.Name)

	return service

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

	clientConnectProxy := createService(t, cluster, serviceOpts, nil)

	gatewayService, err := libservice.NewGatewayService(context.Background(), "api-gateway", "api", cluster.Agents[0], ports...)
	require.NoError(t, err)
	libassert.CatalogServiceExists(t, client, "api-gateway")

	return clientConnectProxy, gatewayService
}

type checkOptions struct {
	debug      bool
	statusCode int
	testName   string
}

// checkRoute, customized version of libassert.RouteEchos to allow for headers/distinguishing between the server instances
func checkRoute(t *testing.T, port int, path string, headers map[string]string, expected checkOptions) {
	ip := "localhost"
	if expected.testName != "" {
		t.Log("running " + expected.testName)
	}
	const phrase = "hello"

	failer := func() *retry.Timer {
		return &retry.Timer{Timeout: time.Second * 60, Wait: time.Second * 60}
	}

	client := cleanhttp.DefaultClient()

	path = strings.TrimPrefix(path, "/")
	url := fmt.Sprintf("http://%s:%d/%s", ip, port, path)

	retry.RunWith(failer(), t, func(r *retry.R) {
		t.Logf("making call to %s", url)
		reader := strings.NewReader(phrase)
		req, err := http.NewRequest("POST", url, reader)
		require.NoError(t, err)
		headers["content-type"] = "text/plain"

		for k, v := range headers {
			req.Header.Set(k, v)

			if k == "Host" {
				req.Host = v
			}
		}
		res, err := client.Do(req)
		if err != nil {
			t.Log(err)
			r.Fatal("could not make call to service ", url)
		}
		defer res.Body.Close()

		body, err := io.ReadAll(res.Body)
		if err != nil {
			r.Fatal("could not read response body ", url)
		}

		assert.Equal(t, expected.statusCode, res.StatusCode)
		if expected.statusCode != res.StatusCode {
			r.Fatal("unexpected response code returned")
		}

		//if debug is expected, debug should be in the response body
		assert.Equal(t, expected.debug, strings.Contains(string(body), "debug"))
		if expected.statusCode != res.StatusCode {
			r.Fatal("unexpected response body returned")
		}

		if !strings.Contains(string(body), phrase) {
			r.Fatal("received an incorrect response ", string(body))
		}

	})
}

func checkRouteError(t *testing.T, ip string, port int, path string, headers map[string]string, expected string) {
	failer := func() *retry.Timer {
		return &retry.Timer{Timeout: time.Second * 60, Wait: time.Second * 60}
	}

	client := cleanhttp.DefaultClient()
	url := fmt.Sprintf("http://%s:%d", ip, port)

	if path != "" {
		url += "/" + path
	}

	retry.RunWith(failer(), t, func(r *retry.R) {
		t.Logf("making call to %s", url)
		req, err := http.NewRequest("GET", url, nil)
		assert.NoError(t, err)

		for k, v := range headers {
			req.Header.Set(k, v)

			if k == "Host" {
				req.Host = v
			}
		}
		_, err = client.Do(req)
		assert.Error(t, err)

		if expected != "" {
			assert.ErrorContains(t, err, expected)
		}
	})
}
