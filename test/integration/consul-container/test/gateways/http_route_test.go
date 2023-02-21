package gateways

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/hashicorp/consul/api"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"math/rand"
	"testing"
	"time"
)

// import (
//
//	"context"
//	"crypto/tls"
//	"encoding/json"
//	"fmt"
//	"io"
//	"net"
//	"net/http"
//	"os"
//	"strings"
//	"testing"
//	"time"
//
//	"github.com/hashicorp/consul/api"
//	"github.com/stretchr/testify/assert"
//	"github.com/stretchr/testify/require"
//	"golang.org/x/exp/slices"
//	apps "k8s.io/api/apps/v1"
//	core "k8s.io/api/core/v1"
//	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
//	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
//	"sigs.k8s.io/e2e-framework/pkg/env"
//	"sigs.k8s.io/e2e-framework/pkg/envconf"
//	"sigs.k8s.io/e2e-framework/pkg/features"
//	gwv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
//	gwv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
//
//	"github.com/hashicorp/consul-api-gateway/internal/k8s"
//	rstatus "github.com/hashicorp/consul-api-gateway/internal/k8s/reconciler/status"
//	"github.com/hashicorp/consul-api-gateway/internal/testing/e2e"
//	apigwv1alpha1 "github.com/hashicorp/consul-api-gateway/pkg/apis/v1alpha1"
//	"math/rand"
//
// )
// )
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
	rand.Seed(time.Now().UnixNano())
	p := make([]byte, n)
	rand.Read(p)
	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(p))[:n]
}

func TestHTTPRouteFlattening(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	//infrastructure set up
	listenerPort := 6000
	//create cluster
	cluster := createCluster(t, listenerPort)
	client := cluster.Agents[0].GetClient()
	serviceOne := createService(t, cluster, &libservice.ServiceOpts{
		Name:     "service1",
		ID:       "service1",
		HTTPPort: 8080,
		GRPCPort: 8079,
	})
	serviceTwo := createService(t, cluster, &libservice.ServiceOpts{
		Name:     "service2",
		ID:       "service2",
		HTTPPort: 8080,
		GRPCPort: 8079,
	})

	//TODO this should only matter in consul enterprise I believe?
	namespace := getNamespace()
	gatewayName := randomName("gw", 16)
	routeOneName := randomName("route", 16)
	routeTwoName := randomName("route", 16)
	path1 := "/"
	path2 := "/v2"

	//write config entries
	apiGateway := &api.APIGatewayConfigEntry{
		Kind: "api-gateway",
		Name: gatewayName,
		Listeners: []api.APIGatewayListener{
			{
				Port:     listenerPort,
				Protocol: "http",
				Hostname: "test.foo",
			},
		},
	}

	routeOne := &api.HTTPRouteConfigEntry{
		Kind: api.HTTPRoute,
		Name: routeOneName,
		Parents: []api.ResourceReference{
			{
				Kind: api.HTTPRoute,
				Name: gatewayName,
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
				Kind: api.HTTPRoute,
				Name: gatewayName,
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
				},
			},
		},
	}

	_, _, err := client.ConfigEntries().Set(apiGateway, nil)
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

	//gateway resolves
	//libassert.HTTPServiceEchoes(t, "localhost", gatewayService.GetPort(listenerPort), path1)
	//libassert.HTTPServiceEchoes(t, "localhost", gatewayService.GetPort(listenerPort), path2)
	fmt.Println(gatewayService)

	//
	//			checkRoute(t, checkPort, "/v2/test", httpResponse{
	//				StatusCode: http.StatusOK,
	//				Body:       serviceTwo.Name,
	//			}, map[string]string{
	//				"Host": "test.foo",
	//				"x-v2": "v2",
	//			}, "service two not routable in allotted time")
	//			checkRoute(t, checkPort, "/v2/test", httpResponse{
	//				StatusCode: http.StatusOK,
	//				Body:       serviceTwo.Name,
	//			}, map[string]string{
	//				"Host": "test.foo",
	//			}, "service two not routable in allotted time")
	//			checkRoute(t, checkPort, "/", httpResponse{
	//				StatusCode: http.StatusOK,
	//				Body:       serviceTwo.Name,
	//			}, map[string]string{
	//				"Host": "test.foo",
	//				"x-v2": "v2",
	//			}, "service two with headers is not routable in allotted time")
	//			checkRoute(t, checkPort, "/", httpResponse{
	//				StatusCode: http.StatusOK,
	//				Body:       serviceOne.Name,
	//			}, map[string]string{
	//				"Host": "test.foo",
	//			}, "service one not routable in allotted time")
	//			checkRoute(t, checkPort, "/v2/test", httpResponse{
	//				StatusCode: http.StatusOK,
	//				Body:       serviceOne.Name,
	//			}, map[string]string{
	//				"Host": "test.example",
	//			}, "service one not routable in allotted time")
	//
	//			err = resources.Delete(ctx, gw)
	//			require.NoError(t, err)
}
