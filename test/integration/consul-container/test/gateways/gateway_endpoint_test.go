package gateways_test

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"github.com/hashicorp/consul/api"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestAPIGatewayCreate(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	cluster := createCluster(t)

	client := cluster.APIClient(0)

	//setup
	apiGateway := &api.APIGatewayConfigEntry{
		Kind: "api-gateway",
		Name: "api-gateway",
		Listeners: []api.APIGatewayListener{
			{
				Port:     8443,
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
	clientConnectProxy, gatewayService := createServices(t, cluster)
	fmt.Println(clientConnectProxy)

	//how to exec into the consul CLI
	//agentUrl, err := cluster.Agents[0].GetPod().PortEndpoint(context.Background(), "8500", "http")
	//cmdStr := "consul connect envoy -gateway api -register -service api-gateway -proxy-id api-gateway -http-addr " + agentUrl
	//
	//c := strings.Split(cmdStr, " ")
	//t.Log("------------\n\n\n")
	//cmd := exec.Command(c[0], c[1:]...)
	//out := bytes.NewBufferString("")
	//stdErr := bytes.NewBufferString("")
	//cmd.Stdout = out
	//cmd.Stderr = stdErr
	//err = cmd.Run()
	//t.Log(out)
	//t.Log(stdErr)
	//t.Log("------------\n\n\n")
	//assert.NoError(t, err)

	//TODO this can and should be broken up more effectively, this is just proof of concept
	//check statuses
	gatewayReady := false
	routeReady := false

	for !gatewayReady || !routeReady {
		//check status
		entry, _, err := client.ConfigEntries().Get("api-gateway", "api-gateway", nil)
		assert.NoError(t, err)
		t.Log(entry)
		apiEntry := entry.(*api.APIGatewayConfigEntry)
		t.Log(apiEntry.Status)
		gatewayReady = isAccepted(apiEntry.Status.Conditions)

		e, _, err := client.ConfigEntries().Get("tcp-route", "api-gateway-route", nil)
		assert.NoError(t, err)
		t.Log(entry)
		routeEntry := e.(*api.TCPRouteConfigEntry)
		t.Log(routeEntry.Status)
		routeReady = isBound(routeEntry.Status.Conditions)
		//this doesn't seem to actually do anything
		time.Sleep(10 * time.Second)
	}

	agentServices, err := client.Agent().Services()
	assert.NoError(t, err)
	for _, s := range agentServices {
		t.Log(s)
	}

	services, _, err := client.Catalog().Services(nil)
	assert.NoError(t, err)
	for key, s := range services {
		t.Log(key, s)
	}

	gateways, _, err := client.Catalog().GatewayServices("api-gateway", nil)
	assert.NoError(t, err)
	for _, g := range gateways {
		t.Log(g)
	}

	t.Log(gatewayService.GetAddr())
	assert.NoError(t, err)
	fmt.Println(gatewayService)

	ip, port := gatewayService.GetAddr()
	t.Log("ip:", ip)
	stdOut := bufio.NewWriter(os.Stdout)
	stdOut.Write([]byte(ip + "\n"))
	stdOut.Write([]byte(strconv.Itoa(port)))
	stdOut.Flush()
	resp, err := http.Get(fmt.Sprintf("http://%s:%d", ip, port))
	t.Log(resp, err)
	assert.NoError(t, err)

	buf := bytes.NewBufferString("abcdefg")
	resp, err = http.Post(fmt.Sprintf("http://%s:%d", ip, port), "text/plain", buf)
	t.Log(resp, err)

	for {
	}
	//t.Log(gatewayService.Restart())
	//t.Log(gatewayService.GetStatus())
	//t.Log(gatewayService.GetLogs())

	t.Fail()

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
func createCluster(t *testing.T) *libcluster.Cluster {
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

	cluster, err := libcluster.New(t, configs)
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

func createServices(t *testing.T, cluster *libcluster.Cluster) (libservice.Service, libservice.Service) {
	node := cluster.Agents[0]
	client := node.GetClient()
	// Create a service and proxy instance
	serviceOpts := &libservice.ServiceOpts{
		Name:     libservice.StaticServerServiceName,
		ID:       "static-server",
		HTTPPort: 8080,
		GRPCPort: 8079,
	}

	// Create a service and proxy instance
	_, _, err := libservice.CreateAndRegisterStaticServerAndSidecar(node, serviceOpts)
	require.NoError(t, err)

	libassert.CatalogServiceExists(t, client, "static-server-sidecar-proxy")
	libassert.CatalogServiceExists(t, client, libservice.StaticServerServiceName)

	// Create a client proxy instance with the server as an upstream
	clientConnectProxy, err := libservice.CreateAndRegisterStaticClientSidecar(node, "", false)
	require.NoError(t, err)

	libassert.CatalogServiceExists(t, client, "static-client-sidecar-proxy")

	gatewayService, err := libservice.NewGatewayService(context.Background(), "api-gateway", "api", cluster.Agents[0])
	libassert.CatalogServiceExists(t, client, "api-gateway")
	require.NoError(t, err)

	return clientConnectProxy, gatewayService
}
