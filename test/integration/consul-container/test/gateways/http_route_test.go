package gateways

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"github.com/hashicorp/consul/api"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func getNamespace() string {
	return ""
}

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

	//infrastructure set up
	listenerPort := 6000
	//create cluster
	cluster := createCluster(t, listenerPort)
	client := cluster.Agents[0].GetClient()
	service1ResponseCode := 200
	service2ResponseCode := 418
	serviceOne := createService(t, cluster, &libservice.ServiceOpts{
		Name:     "service1",
		ID:       "service1",
		HTTPPort: 8080,
		GRPCPort: 8079,
	}, []string{
		//customizes response code so we can distinguish between which service is responding
		"-echo-server-default-params", fmt.Sprintf("status=%d", service1ResponseCode),
	})
	serviceTwo := createService(t, cluster, &libservice.ServiceOpts{
		Name:     "service2",
		ID:       "service2",
		HTTPPort: 8081,
		GRPCPort: 8082,
	}, []string{
		"-echo-server-default-params", fmt.Sprintf("status=%d", service2ResponseCode),
	},
	)

	//TODO this should only matter in consul enterprise I believe?
	namespace := getNamespace()
	gatewayName := randomName("gw", 16)
	routeOneName := randomName("route", 16)
	routeTwoName := randomName("route", 16)
	path1 := "/"
	path2 := "/v2"

	//write config entries
	proxyDefaults := &api.ProxyConfigEntry{
		Kind:      api.ProxyDefaults,
		Name:      api.ProxyConfigGlobal,
		Namespace: namespace,
		Config: map[string]interface{}{
			"protocol": "http",
		},
	}

	_, _, err := client.ConfigEntries().Set(proxyDefaults, nil)
	assert.NoError(t, err)

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
	}

	routeOne := &api.HTTPRouteConfigEntry{
		Kind: api.HTTPRoute,
		Name: routeOneName,
		Parents: []api.ResourceReference{
			{
				Kind:      api.APIGateway,
				Name:      gatewayName,
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
						Name:      serviceOne.GetServiceName(),
						Namespace: namespace,
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
				Namespace: namespace,
			},
		},
		Hostnames: []string{
			"test.foo",
		},
		Namespace: namespace,
		Rules: []api.HTTPRouteRule{
			{
				Services: []api.HTTPService{
					{
						Name:      serviceTwo.GetServiceName(),
						Namespace: namespace,
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

	_, _, err = client.ConfigEntries().Set(apiGateway, nil)
	assert.NoError(t, err)
	_, _, err = client.ConfigEntries().Set(routeOne, nil)
	assert.NoError(t, err)
	_, _, err = client.ConfigEntries().Set(routeTwo, nil)
	assert.NoError(t, err)

	//create gateway service
	gatewayService, err := libservice.NewGatewayService(context.Background(), gatewayName, "api", cluster.Agents[0], listenerPort)
	require.NoError(t, err)
	libassert.CatalogServiceExists(t, client, gatewayName)

	//make sure config entries have been properly created
	require.Eventually(t, func() bool {
		entry, _, err := client.ConfigEntries().Get(api.APIGateway, gatewayName, &api.QueryOptions{Namespace: namespace})
		assert.NoError(t, err)
		if entry == nil {
			return false
		}
		apiEntry := entry.(*api.APIGatewayConfigEntry)
		t.Log(entry)
		return isAccepted(apiEntry.Status.Conditions)
	}, time.Second*10, time.Second*1)

	require.Eventually(t, func() bool {
		entry, _, err := client.ConfigEntries().Get(api.HTTPRoute, routeOneName, &api.QueryOptions{Namespace: namespace})
		assert.NoError(t, err)
		if entry == nil {
			return false
		}

		apiEntry := entry.(*api.HTTPRouteConfigEntry)
		t.Log(entry)
		return isBound(apiEntry.Status.Conditions)
	}, time.Second*10, time.Second*1)

	require.Eventually(t, func() bool {
		entry, _, err := client.ConfigEntries().Get(api.HTTPRoute, routeTwoName, nil)
		assert.NoError(t, err)
		if entry == nil {
			return false
		}

		apiEntry := entry.(*api.HTTPRouteConfigEntry)
		return isBound(apiEntry.Status.Conditions)
	}, time.Second*10, time.Second*1)

	//gateway resolves routes
	ip := "localhost"
	gatewayPort, err := gatewayService.GetPort(listenerPort)
	assert.NoError(t, err)

	//Same v2 path with and without header
	checkRoute(t, ip, gatewayPort, "v2", map[string]string{
		"Host": "test.foo",
		"x-v2": "v2",
	}, checkOptions{statusCode: service2ResponseCode, testName: "service2 header and path"})
	checkRoute(t, ip, gatewayPort, "v2", map[string]string{
		"Host": "test.foo",
	}, checkOptions{statusCode: service2ResponseCode, testName: "service2 just path match"})

	////v1 path with the header
	checkRoute(t, ip, gatewayPort, "check", map[string]string{
		"Host": "test.foo",
		"x-v2": "v2",
	}, checkOptions{statusCode: service2ResponseCode, testName: "service2 just header match"})

	checkRoute(t, ip, gatewayPort, "v2/path/value", map[string]string{
		"Host": "test.foo",
		"x-v2": "v2",
	}, checkOptions{statusCode: service2ResponseCode, testName: "service2 v2 with path"})

	//hit service 1 by hitting root path
	checkRoute(t, ip, gatewayPort, "", map[string]string{
		"Host": "test.foo",
	}, checkOptions{debug: false, statusCode: service1ResponseCode, testName: "service1 root prefix"})

	//hit service 1 by hitting v2 path with v1 hostname
	checkRoute(t, ip, gatewayPort, "v2", map[string]string{
		"Host": "test.example",
	}, checkOptions{debug: false, statusCode: service1ResponseCode, testName: "service1, v2 path with v2 hostname"})

}

func TestHTTPRouteParentRefChange(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	// infrastructure set up
	address := "localhost"

	listenerOnePort := 6000
	listenerTwoPort := 6001

	// create cluster and service
	cluster := createCluster(t, listenerOnePort, listenerTwoPort)
	client := cluster.Agents[0].GetClient()
	service := createService(t, cluster, &libservice.ServiceOpts{
		Name:     "service",
		ID:       "service",
		HTTPPort: 8080,
		GRPCPort: 8079,
	}, []string{})

	// getNamespace() should always return an empty string in Consul OSS
	namespace := getNamespace()
	gatewayOneName := randomName("gw1", 16)
	gatewayTwoName := randomName("gw2", 16)
	routeName := randomName("route", 16)

	// write config entries
	proxyDefaults := &api.ProxyConfigEntry{
		Kind:      api.ProxyDefaults,
		Name:      api.ProxyConfigGlobal,
		Namespace: namespace,
		Config: map[string]interface{}{
			"protocol": "http",
		},
	}
	_, _, err := client.ConfigEntries().Set(proxyDefaults, nil)
	assert.NoError(t, err)

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
	}
	_, _, err = client.ConfigEntries().Set(gatewayOne, nil)
	assert.NoError(t, err)
	require.Eventually(t, func() bool {
		entry, _, err := client.ConfigEntries().Get(api.APIGateway, gatewayOneName, &api.QueryOptions{Namespace: namespace})
		assert.NoError(t, err)
		if entry == nil {
			return false
		}
		apiEntry := entry.(*api.APIGatewayConfigEntry)
		t.Log(entry)
		return isAccepted(apiEntry.Status.Conditions)
	}, time.Second*10, time.Second*1)

	// create gateway service
	gatewayOneService, err := libservice.NewGatewayService(context.Background(), gatewayOneName, "api", cluster.Agents[0], listenerOnePort)
	require.NoError(t, err)
	libassert.CatalogServiceExists(t, client, gatewayOneName)

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
	}
	_, _, err = client.ConfigEntries().Set(gatewayTwo, nil)
	assert.NoError(t, err)
	require.Eventually(t, func() bool {
		entry, _, err := client.ConfigEntries().Get(api.APIGateway, gatewayTwoName, &api.QueryOptions{Namespace: namespace})
		assert.NoError(t, err)
		if entry == nil {
			return false
		}
		apiEntry := entry.(*api.APIGatewayConfigEntry)
		t.Log(entry)
		return isAccepted(apiEntry.Status.Conditions)
	}, time.Second*10, time.Second*1)

	// create gateway service
	gatewayTwoService, err := libservice.NewGatewayService(context.Background(), gatewayTwoName, "api", cluster.Agents[0], listenerTwoPort)
	require.NoError(t, err)
	libassert.CatalogServiceExists(t, client, gatewayTwoName)

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
						Name:      service.GetServiceName(),
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
	_, _, err = client.ConfigEntries().Set(route, nil)
	assert.NoError(t, err)
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
	checkRoute(t, address, gatewayOnePort, "", map[string]string{
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
	_, _, err = client.ConfigEntries().Set(route, nil)
	assert.NoError(t, err)
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
	checkRoute(t, address, gatewayTwoPort, "", map[string]string{
		"Host": "test.example",
	}, checkOptions{debug: false, statusCode: 200})

	// check that first gateway has stopped resolving service
	checkRouteError(t, address, gatewayOnePort, "", map[string]string{
		"Host": "test.foo",
	}, "")
}
