// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"testing"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
)

// TestStateStore_GatewayServices_APIGateway verifies that writing a
// bound-api-gateway config entry materializes gateway<->service mappings in the
// gateway-services table (mirroring ingress gateways), which powers DNS
// auto-registration of services exposed via an API gateway.
func TestStateStore_GatewayServices_APIGateway(t *testing.T) {
	s := testStateStore(t)
	ws := memdb.NewWatchSet()

	// Register a node and an api-gateway service instance plus backend services.
	testRegisterNode(t, s, 0, "node1")
	testRegisterAPIService(t, s, 1, "node1", "api-gw")
	testRegisterConnectService(t, s, 2, "node1", "web")
	testRegisterConnectService(t, s, 3, "node1", "admin")

	// Default protocol to http so http-routes are valid.
	proxyDefaults := &structs.ProxyConfigEntry{
		Name: structs.ProxyConfigGlobal,
		Kind: structs.ProxyDefaults,
		Config: map[string]interface{}{
			"protocol": "http",
		},
	}
	require.NoError(t, s.EnsureConfigEntry(4, proxyDefaults))

	// The api-gateway config entry on its own does not produce any mappings
	// because services are only known once routes bind.
	apigw := &structs.APIGatewayConfigEntry{
		Kind: structs.APIGateway,
		Name: "api-gw",
		Listeners: []structs.APIGatewayListener{
			{
				Name:     "http-listener",
				Port:     8443,
				Protocol: structs.ListenerProtocolHTTP,
			},
		},
	}
	require.NoError(t, s.EnsureConfigEntry(5, apigw))

	_, res, err := s.GatewayServices(ws, "api-gw", nil)
	require.NoError(t, err)
	require.Empty(t, res, "no mappings expected before any route binds")

	// Register an http-route targeting "web" with a custom hostname.
	route := &structs.HTTPRouteConfigEntry{
		Kind:      structs.HTTPRoute,
		Name:      "web-route",
		Parents:   []structs.ResourceReference{{Kind: structs.APIGateway, Name: "api-gw"}},
		Hostnames: []string{"web.example.com"},
		Rules: []structs.HTTPRouteRule{
			{Services: []structs.HTTPService{{Name: "web"}}},
		},
	}
	require.NoError(t, s.EnsureConfigEntry(6, route))

	// The bound-api-gateway is what the controller materializes and is what
	// drives the gateway<->service mapping.
	bound := &structs.BoundAPIGatewayConfigEntry{
		Kind: structs.BoundAPIGateway,
		Name: "api-gw",
		Listeners: []structs.BoundAPIGatewayListener{
			{
				Name:     "http-listener",
				Port:     8443,
				Protocol: structs.ListenerProtocolHTTP,
				Routes:   []structs.ResourceReference{{Kind: structs.HTTPRoute, Name: "web-route"}},
			},
		},
	}
	require.NoError(t, s.EnsureConfigEntry(7, bound))

	// Now the mapping should exist with the listener port and route hostname.
	_, res, err = s.GatewayServices(ws, "api-gw", nil)
	require.NoError(t, err)
	require.Len(t, res, 1)
	require.Equal(t, "web", res[0].Service.Name)
	require.Equal(t, structs.ServiceKindAPIGateway, res[0].GatewayKind)
	require.Equal(t, 8443, res[0].Port)
	require.Equal(t, []string{"web.example.com"}, res[0].Hosts)

	// DNS resolution: looking up the backend service should return the gateway node.
	_, nodes, err := s.CheckAPIGatewayServiceNodes(memdb.NewWatchSet(), "web", nil)
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.Equal(t, "api-gw", nodes[0].Service.Service)

	// A service not behind the gateway resolves to nothing.
	_, nodes, err = s.CheckAPIGatewayServiceNodes(memdb.NewWatchSet(), "admin", nil)
	require.NoError(t, err)
	require.Empty(t, nodes)

	// Deleting the bound-api-gateway clears the mappings.
	require.NoError(t, s.DeleteConfigEntry(8, structs.BoundAPIGateway, "api-gw", nil))
	_, res, err = s.GatewayServices(ws, "api-gw", nil)
	require.NoError(t, err)
	require.Empty(t, res, "mappings should be cleared after bound gateway deletion")

	_, nodes, err = s.CheckAPIGatewayServiceNodes(memdb.NewWatchSet(), "web", nil)
	require.NoError(t, err)
	require.Empty(t, nodes)
}

// TestStateStore_GatewayServices_APIGateway_MultiRoute verifies that both
// http-route and tcp-route bound services are mapped, each picking up the
// correct listener port and protocol.
func TestStateStore_GatewayServices_APIGateway_MultiRoute(t *testing.T) {
	s := testStateStore(t)
	ws := memdb.NewWatchSet()

	testRegisterNode(t, s, 0, "node1")
	testRegisterAPIService(t, s, 1, "node1", "api-gw")
	testRegisterConnectService(t, s, 2, "node1", "web")
	testRegisterService(t, s, 3, "node1", "tcp-backend")

	proxyDefaults := &structs.ProxyConfigEntry{
		Name: structs.ProxyConfigGlobal,
		Kind: structs.ProxyDefaults,
		Config: map[string]interface{}{
			"protocol": "http",
		},
	}
	require.NoError(t, s.EnsureConfigEntry(4, proxyDefaults))

	apigw := &structs.APIGatewayConfigEntry{
		Kind: structs.APIGateway,
		Name: "api-gw",
		Listeners: []structs.APIGatewayListener{
			{Name: "http-listener", Port: 8443, Protocol: structs.ListenerProtocolHTTP},
			{Name: "tcp-listener", Port: 9000, Protocol: structs.ListenerProtocolTCP},
		},
	}
	require.NoError(t, s.EnsureConfigEntry(5, apigw))

	httpRoute := &structs.HTTPRouteConfigEntry{
		Kind:    structs.HTTPRoute,
		Name:    "web-route",
		Parents: []structs.ResourceReference{{Kind: structs.APIGateway, Name: "api-gw"}},
		Rules: []structs.HTTPRouteRule{
			{Services: []structs.HTTPService{{Name: "web"}}},
		},
	}
	require.NoError(t, s.EnsureConfigEntry(6, httpRoute))

	tcpRoute := &structs.TCPRouteConfigEntry{
		Kind:     structs.TCPRoute,
		Name:     "tcp-route",
		Parents:  []structs.ResourceReference{{Kind: structs.APIGateway, Name: "api-gw"}},
		Services: []structs.TCPService{{Name: "tcp-backend"}},
	}
	require.NoError(t, s.EnsureConfigEntry(7, tcpRoute))

	bound := &structs.BoundAPIGatewayConfigEntry{
		Kind: structs.BoundAPIGateway,
		Name: "api-gw",
		Listeners: []structs.BoundAPIGatewayListener{
			{
				Name:     "http-listener",
				Port:     8443,
				Protocol: structs.ListenerProtocolHTTP,
				Routes:   []structs.ResourceReference{{Kind: structs.HTTPRoute, Name: "web-route"}},
			},
			{
				Name:     "tcp-listener",
				Port:     9000,
				Protocol: structs.ListenerProtocolTCP,
				Routes:   []structs.ResourceReference{{Kind: structs.TCPRoute, Name: "tcp-route"}},
			},
		},
	}
	require.NoError(t, s.EnsureConfigEntry(8, bound))

	_, res, err := s.GatewayServices(ws, "api-gw", nil)
	require.NoError(t, err)
	require.Len(t, res, 2)

	byService := make(map[string]*structs.GatewayService)
	for _, gs := range res {
		byService[gs.Service.Name] = gs
	}
	require.Contains(t, byService, "web")
	require.Contains(t, byService, "tcp-backend")
	require.Equal(t, 8443, byService["web"].Port)
	require.Equal(t, "http", byService["web"].Protocol)
	require.Equal(t, 9000, byService["tcp-backend"].Port)
	require.Equal(t, "tcp", byService["tcp-backend"].Protocol)
}

// registerAPIGatewayWithPort registers an api-gateway service instance with an
// explicit catalog port. Port 0 reproduces what consul-k8s writes: its
// registration builds an api.AgentService with no Port field at all, whereas a
// hand-registered gateway carries a real one.
func registerAPIGatewayWithPort(t *testing.T, s *Store, idx uint64, node, name string, port int) {
	t.Helper()
	require.NoError(t, s.EnsureService(idx, node, &structs.NodeService{
		ID:      name,
		Service: name,
		Kind:    structs.ServiceKindAPIGateway,
		Address: "1.1.1.1",
		Port:    port,
	}))
}

// TestStateStore_CheckAPIGatewayServiceNodes_ListenerPort asserts that DNS
// lookups for a service fronted by an API gateway report the gateway's bound
// *listener* port rather than the port on the gateway's catalog registration.
//
// This is the root of the "SRV records answer with port 0" issue: the DNS SRV
// path resolves the port via findPort -> NodeService.Port, and
// CheckAPIGatewayServiceNodes used to return the gateway's raw catalog
// registration. consul-k8s registers gateways without a port, so SRV answered
// 0; only a hand-registered gateway (which sets one) appeared to work, and even
// then it reported the registration port rather than the listener port.
func TestStateStore_CheckAPIGatewayServiceNodes_ListenerPort(t *testing.T) {
	for name, tc := range map[string]struct {
		registeredPort int
	}{
		"gateway registered without a port (consul-k8s)": {registeredPort: 0},
		"gateway registered with a port (hand-rolled)":   {registeredPort: 1111},
	} {
		t.Run(name, func(t *testing.T) {
			s := testStateStore(t)

			testRegisterNode(t, s, 0, "node1")
			registerAPIGatewayWithPort(t, s, 1, "node1", "api-gw", tc.registeredPort)
			testRegisterConnectService(t, s, 2, "node1", "web")

			require.NoError(t, s.EnsureConfigEntry(3, &structs.ProxyConfigEntry{
				Name:   structs.ProxyConfigGlobal,
				Kind:   structs.ProxyDefaults,
				Config: map[string]interface{}{"protocol": "http"},
			}))
			require.NoError(t, s.EnsureConfigEntry(4, &structs.APIGatewayConfigEntry{
				Kind: structs.APIGateway,
				Name: "api-gw",
				Listeners: []structs.APIGatewayListener{
					{Name: "http-listener", Port: 8443, Protocol: structs.ListenerProtocolHTTP},
				},
			}))
			require.NoError(t, s.EnsureConfigEntry(5, &structs.HTTPRouteConfigEntry{
				Kind:    structs.HTTPRoute,
				Name:    "web-route",
				Parents: []structs.ResourceReference{{Kind: structs.APIGateway, Name: "api-gw"}},
				Rules:   []structs.HTTPRouteRule{{Services: []structs.HTTPService{{Name: "web"}}}},
			}))
			require.NoError(t, s.EnsureConfigEntry(6, &structs.BoundAPIGatewayConfigEntry{
				Kind: structs.BoundAPIGateway,
				Name: "api-gw",
				Listeners: []structs.BoundAPIGatewayListener{{
					Name:     "http-listener",
					Port:     8443,
					Protocol: structs.ListenerProtocolHTTP,
					Routes:   []structs.ResourceReference{{Kind: structs.HTTPRoute, Name: "web-route"}},
				}},
			}))

			_, nodes, err := s.CheckAPIGatewayServiceNodes(memdb.NewWatchSet(), "web", nil)
			require.NoError(t, err)
			require.Len(t, nodes, 1)
			require.Equal(t, "api-gw", nodes[0].Service.Service)
			require.Equal(t, 8443, nodes[0].Service.Port,
				"DNS must report the bound listener port, not the catalog registration port")

			// The catalog registration itself must be untouched: the nodes above
			// are shared memdb objects and rewriting them in place would corrupt
			// every other reader.
			_, direct, err := s.NodeServices(memdb.NewWatchSet(), "node1", nil, "")
			require.NoError(t, err)
			require.Equal(t, tc.registeredPort, direct.Services["api-gw"].Port,
				"the gateway's own registration must not be mutated")
		})
	}
}

// TestStateStore_CheckAPIGatewayServiceNodes_MultipleListenerPorts covers a
// gateway fronting the same service on two listeners: DNS should advertise both
// ports so a client can reach either listener.
func TestStateStore_CheckAPIGatewayServiceNodes_MultipleListenerPorts(t *testing.T) {
	s := testStateStore(t)

	testRegisterNode(t, s, 0, "node1")
	registerAPIGatewayWithPort(t, s, 1, "node1", "api-gw", 0)
	testRegisterConnectService(t, s, 2, "node1", "web")

	require.NoError(t, s.EnsureConfigEntry(3, &structs.ProxyConfigEntry{
		Name:   structs.ProxyConfigGlobal,
		Kind:   structs.ProxyDefaults,
		Config: map[string]interface{}{"protocol": "http"},
	}))
	require.NoError(t, s.EnsureConfigEntry(4, &structs.APIGatewayConfigEntry{
		Kind: structs.APIGateway,
		Name: "api-gw",
		Listeners: []structs.APIGatewayListener{
			{Name: "listener-a", Port: 8443, Protocol: structs.ListenerProtocolHTTP},
			{Name: "listener-b", Port: 9443, Protocol: structs.ListenerProtocolHTTP},
		},
	}))
	require.NoError(t, s.EnsureConfigEntry(5, &structs.HTTPRouteConfigEntry{
		Kind:    structs.HTTPRoute,
		Name:    "web-route",
		Parents: []structs.ResourceReference{{Kind: structs.APIGateway, Name: "api-gw"}},
		Rules:   []structs.HTTPRouteRule{{Services: []structs.HTTPService{{Name: "web"}}}},
	}))
	require.NoError(t, s.EnsureConfigEntry(6, &structs.BoundAPIGatewayConfigEntry{
		Kind: structs.BoundAPIGateway,
		Name: "api-gw",
		Listeners: []structs.BoundAPIGatewayListener{
			{
				Name:     "listener-a",
				Port:     8443,
				Protocol: structs.ListenerProtocolHTTP,
				Routes:   []structs.ResourceReference{{Kind: structs.HTTPRoute, Name: "web-route"}},
			},
			{
				Name:     "listener-b",
				Port:     9443,
				Protocol: structs.ListenerProtocolHTTP,
				Routes:   []structs.ResourceReference{{Kind: structs.HTTPRoute, Name: "web-route"}},
			},
		},
	}))

	_, nodes, err := s.CheckAPIGatewayServiceNodes(memdb.NewWatchSet(), "web", nil)
	require.NoError(t, err)
	require.Len(t, nodes, 2)

	ports := []int{nodes[0].Service.Port, nodes[1].Service.Port}
	require.ElementsMatch(t, []int{8443, 9443}, ports)
}
