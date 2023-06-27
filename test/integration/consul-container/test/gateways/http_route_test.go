// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package gateways

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	libtopology "github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
)

// randomName generates a random name of n length with the provided
// prefix. If prefix is omitted, the then entire name is random char.
func randomName(prefix string, n int) string {
	if n == 0 {
		n = 32
	}
	if len(prefix) >= n {
		return prefix
	}
	p := make([]byte, n)
	rand.Read(p)
	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(p))[:n]
}

func TestHTTPRouteFlattening(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	// infrastructure set up
	listenerPort := 6004
	serviceOneHTTPPort := 6005
	serviceOneGRPCPort := 6006
	serviceTwoHTTPPort := 6007
	serviceTwoGRPCPort := 6008

	serviceOneName := randomName("service", 16)
	serviceTwoName := randomName("service", 16)
	serviceOneResponseCode := 200
	serviceTwoResponseCode := 418
	gatewayName := randomName("gw", 16)
	routeOneName := randomName("route", 16)
	routeTwoName := randomName("route", 16)
	fooHostName := "test.foo"
	exampleHostName := "test.example"
	path1 := "/"
	path2 := "/v2"

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
			listenerPort,
			serviceOneHTTPPort,
			serviceOneGRPCPort,
			serviceTwoHTTPPort,
			serviceTwoGRPCPort,
		},
		ApplyDefaultProxySettings: true,
	}

	cluster, _, _ := libtopology.NewCluster(t, clusterConfig)
	client := cluster.Agents[0].GetClient()

	gwNamespace := getOrCreateNamespace(t, client)
	serviceOneNamespace := getOrCreateNamespace(t, client)
	serviceTwoNamespace := getOrCreateNamespace(t, client)

	_, _, err := libservice.CreateAndRegisterStaticServerAndSidecar(cluster.Agents[0], &libservice.ServiceOpts{
		Name:      serviceOneName,
		ID:        serviceOneName,
		HTTPPort:  serviceOneHTTPPort,
		GRPCPort:  serviceOneGRPCPort,
		Namespace: serviceOneNamespace,
	},
		// customizes response code so we can distinguish between which service is responding
		"-echo-server-default-params", fmt.Sprintf("status=%d", serviceOneResponseCode),
	)

	require.NoError(t, err)

	_, _, err = libservice.CreateAndRegisterStaticServerAndSidecar(cluster.Agents[0], &libservice.ServiceOpts{
		Name:      serviceTwoName,
		ID:        serviceTwoName,
		HTTPPort:  serviceTwoHTTPPort,
		GRPCPort:  serviceTwoGRPCPort,
		Namespace: serviceTwoNamespace,
	},
		"-echo-server-default-params", fmt.Sprintf("status=%d", serviceTwoResponseCode),
	)

	require.NoError(t, err)

	// write config entries
	proxyDefaults := &api.ProxyConfigEntry{
		Kind:      api.ProxyDefaults,
		Name:      api.ProxyConfigGlobal,
		Namespace: "", // proxy-defaults can only be set in the default namespace
		Config: map[string]interface{}{
			"protocol": "http",
		},
	}

	require.NoError(t, cluster.ConfigEntryWrite(proxyDefaults))

	apiGateway := &api.APIGatewayConfigEntry{
		Kind: "api-gateway",
		Name: gatewayName,
		Listeners: []api.APIGatewayListener{
			{
				Name:     "listener",
				Port:     listenerPort,
				Protocol: "http",
			},
		},
		Namespace: gwNamespace,
	}

	routeOne := &api.HTTPRouteConfigEntry{
		Kind: api.HTTPRoute,
		Name: routeOneName,
		Parents: []api.ResourceReference{
			{
				Kind:      api.APIGateway,
				Name:      gatewayName,
				Namespace: gwNamespace,
			},
		},
		Hostnames: []string{
			fooHostName,
			exampleHostName,
		},
		Namespace: gwNamespace,
		Rules: []api.HTTPRouteRule{
			{
				Services: []api.HTTPService{
					{
						Name:      serviceOneName,
						Namespace: serviceOneNamespace,
					},
				},
				Matches: []api.HTTPMatch{
					{
						Path: api.HTTPPathMatch{
							Match: api.HTTPPathMatchPrefix,
							Value: path1,
						},
					},
				},
			},
		},
	}

	routeTwo := &api.HTTPRouteConfigEntry{
		Kind: api.HTTPRoute,
		Name: routeTwoName,
		Parents: []api.ResourceReference{
			{
				Kind:      api.APIGateway,
				Name:      gatewayName,
				Namespace: gwNamespace,
			},
		},
		Hostnames: []string{
			fooHostName,
		},
		Namespace: gwNamespace,
		Rules: []api.HTTPRouteRule{
			{
				Services: []api.HTTPService{
					{
						Name:      serviceTwoName,
						Namespace: serviceTwoNamespace,
					},
				},
				Matches: []api.HTTPMatch{
					{
						Path: api.HTTPPathMatch{
							Match: api.HTTPPathMatchPrefix,
							Value: path2,
						},
					},
					{
						Headers: []api.HTTPHeaderMatch{{
							Match: api.HTTPHeaderMatchExact,
							Name:  "x-v2",
							Value: "v2",
						}},
					},
				},
			},
		},
	}

	require.NoError(t, cluster.ConfigEntryWrite(apiGateway))
	require.NoError(t, cluster.ConfigEntryWrite(routeOne))
	require.NoError(t, cluster.ConfigEntryWrite(routeTwo))

	// create gateway service
	gwCfg := libservice.GatewayConfig{
		Name:      gatewayName,
		Kind:      "api",
		Namespace: gwNamespace,
	}
	gatewayService, err := libservice.NewGatewayService(context.Background(), gwCfg, cluster.Agents[0], listenerPort)
	require.NoError(t, err)
	libassert.CatalogServiceExists(t, client, gatewayName, &api.QueryOptions{Namespace: gwNamespace})

	// make sure config entries have been properly created
	checkGatewayConfigEntry(t, client, gatewayName, &api.QueryOptions{Namespace: gwNamespace})
	checkHTTPRouteConfigEntry(t, client, routeOneName, &api.QueryOptions{Namespace: gwNamespace})
	checkHTTPRouteConfigEntry(t, client, routeTwoName, &api.QueryOptions{Namespace: gwNamespace})

	// gateway resolves routes
	gatewayPort, err := gatewayService.GetPort(listenerPort)
	require.NoError(t, err)

	// Same v2 path with and without header
	checkRoute(t, gatewayPort, "/v2", map[string]string{
		"Host": fooHostName,
		"x-v2": "v2",
	}, checkOptions{statusCode: serviceTwoResponseCode, testName: "service2 header and path"})

	checkRoute(t, gatewayPort, "/v2", map[string]string{
		"Host": fooHostName,
	}, checkOptions{statusCode: serviceTwoResponseCode, testName: "service2 just path match"})

	// //v1 path with the header
	checkRoute(t, gatewayPort, "/check", map[string]string{
		"Host": fooHostName,
		"x-v2": "v2",
	}, checkOptions{statusCode: serviceTwoResponseCode, testName: "service2 just header match"})

	checkRoute(t, gatewayPort, "/v2/path/value", map[string]string{
		"Host": fooHostName,
		"x-v2": "v2",
	}, checkOptions{statusCode: serviceTwoResponseCode, testName: "service2 v2 with path"})

	// hit service 1 by hitting root path
	checkRoute(t, gatewayPort, "", map[string]string{
		"Host": fooHostName,
	}, checkOptions{debug: false, statusCode: serviceOneResponseCode, testName: "service1 root prefix"})

	// hit service 1 by hitting v2 path with v1 hostname
	checkRoute(t, gatewayPort, "/v2", map[string]string{
		"Host": exampleHostName,
	}, checkOptions{debug: false, statusCode: serviceOneResponseCode, testName: "service1, v2 path with v2 hostname"})
}

func TestHTTPRoutePathRewrite(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	// infrastructure set up
	listenerPort := 6009
	fooHTTPPort := 6010
	fooGRPCPort := 6011
	barHTTPPort := 6012
	barGRPCPort := 6013

	fooName := randomName("foo", 16)
	barName := randomName("bar", 16)
	gatewayName := randomName("gw", 16)
	invalidRouteName := randomName("route", 16)
	validRouteName := randomName("route", 16)

	// create cluster
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
			listenerPort,
			fooHTTPPort,
			fooGRPCPort,
			barHTTPPort,
			barGRPCPort,
		},
		ApplyDefaultProxySettings: true,
	}

	cluster, _, _ := libtopology.NewCluster(t, clusterConfig)
	client := cluster.APIClient(0)

	fooStatusCode := 400
	barStatusCode := 201
	fooPath := "/v1/foo"
	barPath := "/v1/bar"

	namespace := getOrCreateNamespace(t, client)

	_, _, err := libservice.CreateAndRegisterStaticServerAndSidecar(cluster.Agents[0], &libservice.ServiceOpts{
		ID:        fooName,
		Name:      fooName,
		Namespace: namespace,
		HTTPPort:  fooHTTPPort,
		GRPCPort:  fooGRPCPort,
	},
		// customizes response code so we can distinguish between which service is responding
		"-echo-debug-path", fooPath,
		"-echo-server-default-params", fmt.Sprintf("status=%d", fooStatusCode),
	)
	require.NoError(t, err)

	_, _, err = libservice.CreateAndRegisterStaticServerAndSidecar(cluster.Agents[0], &libservice.ServiceOpts{
		ID:        barName,
		Name:      barName,
		Namespace: namespace,
		HTTPPort:  barHTTPPort,
		GRPCPort:  barGRPCPort,
	},
		// customizes response code so we can distinguish between which service is responding
		"-echo-debug-path", barPath,
		"-echo-server-default-params", fmt.Sprintf("status=%d", barStatusCode),
	)
	require.NoError(t, err)

	fooUnrewritten := "/foo"
	barUnrewritten := "/bar"

	// write config entries
	proxyDefaults := &api.ProxyConfigEntry{
		Kind:      api.ProxyDefaults,
		Name:      api.ProxyConfigGlobal,
		Namespace: "", // proxy-defaults can only be set in the default namespace
		Config: map[string]interface{}{
			"protocol": "http",
		},
	}

	require.NoError(t, cluster.ConfigEntryWrite(proxyDefaults))

	apiGateway := &api.APIGatewayConfigEntry{
		Kind: api.APIGateway,
		Name: gatewayName,
		Listeners: []api.APIGatewayListener{
			{
				Name:     "listener",
				Port:     listenerPort,
				Protocol: "http",
			},
		},
		Namespace: namespace,
	}

	fooRoute := &api.HTTPRouteConfigEntry{
		Kind: api.HTTPRoute,
		Name: invalidRouteName,
		Parents: []api.ResourceReference{
			{
				Kind:      api.APIGateway,
				Name:      gatewayName,
				Namespace: namespace,
			},
		},
		Hostnames: []string{
			"test.foo",
		},
		Namespace: namespace,
		Rules: []api.HTTPRouteRule{
			{
				Filters: api.HTTPFilters{
					URLRewrite: &api.URLRewrite{
						Path: fooPath,
					},
				},
				Services: []api.HTTPService{
					{
						Name:      fooName,
						Namespace: namespace,
					},
				},
				Matches: []api.HTTPMatch{
					{
						Path: api.HTTPPathMatch{
							Match: api.HTTPPathMatchPrefix,
							Value: fooUnrewritten,
						},
					},
				},
			},
		},
	}

	barRoute := &api.HTTPRouteConfigEntry{
		Kind: api.HTTPRoute,
		Name: validRouteName,
		Parents: []api.ResourceReference{
			{
				Kind:      api.APIGateway,
				Name:      gatewayName,
				Namespace: namespace,
			},
		},
		Hostnames: []string{
			"test.foo",
		},
		Namespace: namespace,
		Rules: []api.HTTPRouteRule{
			{
				Filters: api.HTTPFilters{
					URLRewrite: &api.URLRewrite{
						Path: barPath,
					},
				},
				Services: []api.HTTPService{
					{
						Name:      barName,
						Namespace: namespace,
					},
				},
				Matches: []api.HTTPMatch{
					{
						Path: api.HTTPPathMatch{
							Match: api.HTTPPathMatchPrefix,
							Value: barUnrewritten,
						},
					},
				},
			},
		},
	}

	require.NoError(t, cluster.ConfigEntryWrite(apiGateway))
	require.NoError(t, cluster.ConfigEntryWrite(fooRoute))
	require.NoError(t, cluster.ConfigEntryWrite(barRoute))

	// create gateway service
	gwCfg := libservice.GatewayConfig{
		Name:      gatewayName,
		Kind:      "api",
		Namespace: namespace,
	}
	gatewayService, err := libservice.NewGatewayService(context.Background(), gwCfg, cluster.Agents[0], listenerPort)
	require.NoError(t, err)
	libassert.CatalogServiceExists(t, client, gatewayName, &api.QueryOptions{Namespace: namespace})

	// make sure config entries have been properly created
	checkGatewayConfigEntry(t, client, gatewayName, &api.QueryOptions{Namespace: namespace})
	checkHTTPRouteConfigEntry(t, client, invalidRouteName, &api.QueryOptions{Namespace: namespace})
	checkHTTPRouteConfigEntry(t, client, validRouteName, &api.QueryOptions{Namespace: namespace})

	gatewayPort, err := gatewayService.GetPort(listenerPort)
	require.NoError(t, err)

	// TODO these were the assertions we had in the original test. potentially would want more test cases

	// NOTE: Hitting the debug path code overrides default expected value
	debugExpectedStatusCode := 200

	// hit foo, making sure path is being rewritten by hitting the debug page
	checkRoute(t, gatewayPort, fooUnrewritten, map[string]string{
		"Host": "test.foo",
	}, checkOptions{debug: true, statusCode: debugExpectedStatusCode, testName: "foo service"})
	// make sure foo is being sent to proper service
	checkRoute(t, gatewayPort, fooUnrewritten+"/foo", map[string]string{
		"Host": "test.foo",
	}, checkOptions{debug: false, statusCode: fooStatusCode, testName: "foo service 2"})

	// hit bar, making sure its been rewritten
	checkRoute(t, gatewayPort, barUnrewritten, map[string]string{
		"Host": "test.foo",
	}, checkOptions{debug: true, statusCode: debugExpectedStatusCode, testName: "bar service"})

	// hit bar, making sure its being sent to the proper service
	checkRoute(t, gatewayPort, barUnrewritten+"/bar", map[string]string{
		"Host": "test.foo",
	}, checkOptions{debug: false, statusCode: barStatusCode, testName: "bar service"})
}

func TestHTTPRouteParentRefChange(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	// infrastructure set up
	address := "localhost"

	listenerOnePort := 6014
	listenerTwoPort := 6015
	serviceHTTPPort := 6016
	serviceGRPCPort := 6017

	serviceName := randomName("service", 16)
	gatewayOneName := randomName("gw1", 16)
	gatewayTwoName := randomName("gw2", 16)
	routeName := randomName("route", 16)

	// create cluster
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
			listenerOnePort,
			listenerTwoPort,
			serviceHTTPPort,
			serviceGRPCPort,
		},
		ApplyDefaultProxySettings: true,
	}

	cluster, _, _ := libtopology.NewCluster(t, clusterConfig)
	client := cluster.APIClient(0)

	namespace := getOrCreateNamespace(t, client)

	_, _, err := libservice.CreateAndRegisterStaticServerAndSidecar(cluster.Agents[0], &libservice.ServiceOpts{
		ID:        serviceName,
		Name:      serviceName,
		Namespace: namespace,
		HTTPPort:  serviceHTTPPort,
		GRPCPort:  serviceGRPCPort,
	})
	require.NoError(t, err)

	// write config entries
	proxyDefaults := &api.ProxyConfigEntry{
		Kind: api.ProxyDefaults,
		Name: api.ProxyConfigGlobal,
		Config: map[string]interface{}{
			"protocol": "http",
		},
	}

	require.NoError(t, cluster.ConfigEntryWrite(proxyDefaults))

	// create gateway config entry
	gatewayOne := &api.APIGatewayConfigEntry{
		Kind: "api-gateway",
		Name: gatewayOneName,
		Listeners: []api.APIGatewayListener{
			{
				Name:     "listener",
				Port:     listenerOnePort,
				Protocol: "http",
				Hostname: "test.foo",
			},
		},
		Namespace: namespace,
	}
	require.NoError(t, cluster.ConfigEntryWrite(gatewayOne))
	checkGatewayConfigEntry(t, client, gatewayOneName, &api.QueryOptions{Namespace: namespace})

	// create gateway service
	gwOneCfg := libservice.GatewayConfig{
		Name:      gatewayOneName,
		Kind:      "api",
		Namespace: namespace,
	}
	gatewayOneService, err := libservice.NewGatewayService(context.Background(), gwOneCfg, cluster.Agents[0], listenerOnePort)
	require.NoError(t, err)
	libassert.CatalogServiceExists(t, client, gatewayOneName, &api.QueryOptions{Namespace: namespace})

	// create gateway config entry
	gatewayTwo := &api.APIGatewayConfigEntry{
		Kind: "api-gateway",
		Name: gatewayTwoName,
		Listeners: []api.APIGatewayListener{
			{
				Name:     "listener",
				Port:     listenerTwoPort,
				Protocol: "http",
				Hostname: "test.example",
			},
		},
		Namespace: namespace,
	}
	require.NoError(t, cluster.ConfigEntryWrite(gatewayTwo))
	checkGatewayConfigEntry(t, client, gatewayTwoName, &api.QueryOptions{Namespace: namespace})

	// create gateway service
	gwTwoCfg := libservice.GatewayConfig{
		Name:      gatewayTwoName,
		Kind:      "api",
		Namespace: namespace,
	}
	gatewayTwoService, err := libservice.NewGatewayService(context.Background(), gwTwoCfg, cluster.Agents[0], listenerTwoPort)
	require.NoError(t, err)
	libassert.CatalogServiceExists(t, client, gatewayTwoName, &api.QueryOptions{Namespace: namespace})

	// create route to service, targeting first gateway
	route := &api.HTTPRouteConfigEntry{
		Kind: api.HTTPRoute,
		Name: routeName,
		Parents: []api.ResourceReference{
			{
				Kind:      api.APIGateway,
				Name:      gatewayOneName,
				Namespace: namespace,
			},
		},
		Hostnames: []string{
			"test.foo",
			"test.example",
		},
		Namespace: namespace,
		Rules: []api.HTTPRouteRule{
			{
				Services: []api.HTTPService{
					{
						Name:      serviceName,
						Namespace: namespace,
					},
				},
				Matches: []api.HTTPMatch{
					{
						Path: api.HTTPPathMatch{
							Match: api.HTTPPathMatchPrefix,
							Value: "/",
						},
					},
				},
			},
		},
	}

	require.NoError(t, cluster.ConfigEntryWrite(route))

	require.Eventually(t, func() bool {
		entry, _, err := client.ConfigEntries().Get(api.HTTPRoute, routeName, &api.QueryOptions{Namespace: namespace})
		assert.NoError(t, err)
		if entry == nil {
			return false
		}

		apiEntry := entry.(*api.HTTPRouteConfigEntry)
		t.Log(entry)

		// check if bound only to correct gateway
		return len(apiEntry.Parents) == 1 &&
			apiEntry.Parents[0].Name == gatewayOneName &&
			isBound(apiEntry.Status.Conditions)
	}, time.Second*10, time.Second*1)

	// fetch gateway listener ports
	gatewayOnePort, err := gatewayOneService.GetPort(listenerOnePort)
	assert.NoError(t, err)
	gatewayTwoPort, err := gatewayTwoService.GetPort(listenerTwoPort)
	assert.NoError(t, err)

	// hit service by requesting root path
	// TODO: testName field in checkOptions struct looked to be unused, is it needed?
	checkRoute(t, gatewayOnePort, "", map[string]string{
		"Host": "test.foo",
	}, checkOptions{debug: false, statusCode: 200})

	// check that second gateway does not resolve service
	checkRouteError(t, address, gatewayTwoPort, "", map[string]string{
		"Host": "test.example",
	}, "")

	// swtich route target to second gateway
	route.Parents = []api.ResourceReference{
		{
			Kind:      api.APIGateway,
			Name:      gatewayTwoName,
			Namespace: namespace,
		},
	}

	require.NoError(t, cluster.ConfigEntryWrite(route))
	require.Eventually(t, func() bool {
		entry, _, err := client.ConfigEntries().Get(api.HTTPRoute, routeName, &api.QueryOptions{Namespace: namespace})
		assert.NoError(t, err)
		if entry == nil {
			return false
		}

		apiEntry := entry.(*api.HTTPRouteConfigEntry)
		t.Log(apiEntry)
		t.Log(fmt.Sprintf("%#v", apiEntry))

		// check if bound only to correct gateway
		return len(apiEntry.Parents) == 1 &&
			apiEntry.Parents[0].Name == gatewayTwoName &&
			isBound(apiEntry.Status.Conditions)
	}, time.Second*10, time.Second*1)

	// hit service by requesting root path on other gateway with different hostname
	checkRoute(t, gatewayTwoPort, "", map[string]string{
		"Host": "test.example",
	}, checkOptions{debug: false, statusCode: 200})

	// check that first gateway has stopped resolving service
	checkRouteError(t, address, gatewayOnePort, "", map[string]string{
		"Host": "test.foo",
	}, "")
}
