// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package gateways

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-cleanhttp"

	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	libtopology "github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
)

// Creates a gateway service and tests to see if it is routable
func TestAPIGatewayCreate(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	gatewayName := randomName("gateway", 16)
	routeName := randomName("route", 16)
	serviceName := randomName("service", 16)
	listenerPortOne := 6000
	serviceHTTPPort := 6001
	serviceGRPCPort := 6002

	clusterConfig := &libtopology.ClusterConfig{
		NumServers: 1,
		NumClients: 1,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter:             "dc1",
			InjectAutoEncryption:   true,
			InjectGossipEncryption: true,
			AllowHTTPAnyway:        true,
		},
		Ports: []int{
			listenerPortOne,
			serviceHTTPPort,
			serviceGRPCPort,
		},
	}

	cluster, _, _ := libtopology.NewCluster(t, clusterConfig)
	client := cluster.APIClient(0)

	namespace := getOrCreateNamespace(t, client)

	// add api gateway config
	apiGateway := &api.APIGatewayConfigEntry{
		Kind:      api.APIGateway,
		Namespace: namespace,
		Name:      gatewayName,
		Listeners: []api.APIGatewayListener{
			{
				Name:     "listener",
				Port:     listenerPortOne,
				Protocol: "tcp",
			},
		},
	}

	require.NoError(t, cluster.ConfigEntryWrite(apiGateway))

	_, _, err := libservice.CreateAndRegisterStaticServerAndSidecar(cluster.Agents[0], &libservice.ServiceOpts{
		ID:        serviceName,
		Name:      serviceName,
		Namespace: namespace,
		HTTPPort:  serviceHTTPPort,
		GRPCPort:  serviceGRPCPort,
	})
	require.NoError(t, err)

	tcpRoute := &api.TCPRouteConfigEntry{
		Kind:      api.TCPRoute,
		Name:      routeName,
		Namespace: namespace,
		Parents: []api.ResourceReference{
			{
				Kind:      api.APIGateway,
				Namespace: namespace,
				Name:      gatewayName,
			},
		},
		Services: []api.TCPService{
			{
				Namespace: namespace,
				Name:      serviceName,
			},
		},
	}

	require.NoError(t, cluster.ConfigEntryWrite(tcpRoute))

	// Create a gateway
	gatewayService, err := libservice.NewGatewayService(context.Background(), libservice.GatewayConfig{
		Kind:      "api",
		Namespace: namespace,
		Name:      gatewayName,
	}, cluster.Agents[0], listenerPortOne)
	require.NoError(t, err)

	// make sure the gateway/route come online
	// make sure config entries have been properly created
	checkGatewayConfigEntry(t, client, gatewayName, &api.QueryOptions{Namespace: namespace})
	checkTCPRouteConfigEntry(t, client, routeName, namespace)

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
		if c.Type == typeName && string(c.Status) == statusValue {
			return true
		}
	}
	return false
}

func checkGatewayConfigEntry(t *testing.T, client *api.Client, gatewayName string, opts *api.QueryOptions) {
	t.Helper()

	require.Eventually(t, func() bool {
		entry, _, err := client.ConfigEntries().Get(api.APIGateway, gatewayName, opts)
		if err != nil {
			t.Log("error constructing request", err)
			return false
		}
		if entry == nil {
			t.Log("returned entry is nil")
			return false
		}

		apiEntry := entry.(*api.APIGatewayConfigEntry)
		return isAccepted(apiEntry.Status.Conditions)
	}, time.Second*10, time.Second*1)
}

func checkHTTPRouteConfigEntry(t *testing.T, client *api.Client, routeName string, opts *api.QueryOptions) {
	t.Helper()

	require.Eventually(t, func() bool {
		entry, _, err := client.ConfigEntries().Get(api.HTTPRoute, routeName, opts)
		if err != nil {
			t.Log("error constructing request", err)
			return false
		}
		if entry == nil {
			t.Log("returned entry is nil")
			return false
		}

		apiEntry := entry.(*api.HTTPRouteConfigEntry)
		return isBound(apiEntry.Status.Conditions)
	}, time.Second*10, time.Second*1)
}

func checkHTTPRouteConfigEntryExists(t *testing.T, client *api.Client, routeName string, namespace string) {
	t.Helper()

	require.Eventually(t, func() bool {
		entry, _, err := client.ConfigEntries().Get(api.HTTPRoute, routeName, &api.QueryOptions{Namespace: namespace})
		if err != nil {
			t.Log("error constructing request", err)
			return false
		}
		if entry == nil {
			t.Log("returned entry is nil")
			return false
		}

		_ = entry.(*api.HTTPRouteConfigEntry)
		return true
	}, time.Second*10, time.Second*1)
}

func checkTCPRouteConfigEntry(t *testing.T, client *api.Client, routeName string, namespace string) {
	t.Helper()

	require.Eventually(t, func() bool {
		entry, _, err := client.ConfigEntries().Get(api.TCPRoute, routeName, &api.QueryOptions{Namespace: namespace})
		if err != nil {
			t.Log("error constructing request", err)
			return false
		}
		if entry == nil {
			t.Log("returned entry is nil")
			return false
		}

		apiEntry := entry.(*api.TCPRouteConfigEntry)
		return isBound(apiEntry.Status.Conditions)
	}, time.Second*10, time.Second*1)
}

type checkOptions struct {
	debug      bool
	statusCode int
	testName   string
}

// checkRoute, customized version of libassert.RouteEchos to allow for headers/distinguishing between the server instances
func checkRoute(t *testing.T, port int, path string, headers map[string]string, expected checkOptions) {
	t.Helper()

	if expected.testName != "" {
		t.Log("running " + expected.testName)
	}

	client := cleanhttp.DefaultClient()
	path = strings.TrimPrefix(path, "/")
	url := fmt.Sprintf("http://localhost:%d/%s", port, path)

	require.Eventually(t, func() bool {
		reader := strings.NewReader("hello")
		req, err := http.NewRequest("POST", url, reader)
		if err != nil {
			t.Log("error constructing request", err)
			return false
		}
		headers["content-type"] = "text/plain"

		for k, v := range headers {
			req.Header.Set(k, v)

			if k == "Host" {
				req.Host = v
			}
		}

		res, err := client.Do(req)
		if err != nil {
			t.Log("error sending request", err)
			return false
		}
		defer res.Body.Close()

		body, err := io.ReadAll(res.Body)
		if err != nil {
			t.Log("error reading response body", err)
			return false
		}

		if expected.statusCode != res.StatusCode {
			t.Logf("bad status code - expected: %d, actual: %d", expected.statusCode, res.StatusCode)
			return false
		}
		if expected.debug {
			if !strings.Contains(string(body), "debug") {
				t.Log("body does not contain 'debug'")
				return false
			}
		}
		if !strings.Contains(string(body), "hello") {
			t.Log("body does not contain 'hello'")
			return false
		}

		return true
	}, time.Second*30, time.Second*1)
}

func checkRouteError(t *testing.T, ip string, port int, path string, headers map[string]string, expected string) {
	t.Helper()

	client := cleanhttp.DefaultClient()
	url := fmt.Sprintf("http://%s:%d", ip, port)

	if path != "" {
		url += "/" + path
	}

	require.Eventually(t, func() bool {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			t.Log("error constructing request", err)
			return false
		}
		for k, v := range headers {
			req.Header.Set(k, v)

			if k == "Host" {
				req.Host = v
			}
		}
		_, err = client.Do(req)
		if err == nil {
			t.Log("client request should have errored, but didn't")
			return false
		}
		if expected != "" {
			if !strings.Contains(err.Error(), expected) {
				t.Logf("expected %q to contain %q", err.Error(), expected)
				return false
			}
		}
		return true
	}, time.Second*30, time.Second*1)
}
