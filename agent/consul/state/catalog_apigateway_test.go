// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"testing"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
)

// enableAPIGatewayDNS sets the cluster-wide feature flag that gates API gateway
// DNS auto-registration, so the state store materializes gateway-services rows.
func enableAPIGatewayDNS(t *testing.T, s *Store, idx uint64) {
	t.Helper()
	require.NoError(t, s.SystemMetadataSet(idx, &structs.SystemMetadataEntry{
		Key:   structs.SystemMetadataAPIGatewayDNSEnabled,
		Value: "true",
	}))
}

// TestStateStore_GatewayServices_APIGateway verifies that writing a
// bound-api-gateway config entry materializes gateway<->service mappings in the
// gateway-services table (mirroring ingress gateways), which powers DNS
// auto-registration of services exposed via an API gateway.
func TestStateStore_GatewayServices_APIGateway(t *testing.T) {
	s := testStateStore(t)
	ws := memdb.NewWatchSet()

	enableAPIGatewayDNS(t, s, 1)

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
				Name:   "http-listener",
				Routes: []structs.ResourceReference{{Kind: structs.HTTPRoute, Name: "web-route"}},
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

	enableAPIGatewayDNS(t, s, 1)

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
				Name:   "http-listener",
				Routes: []structs.ResourceReference{{Kind: structs.HTTPRoute, Name: "web-route"}},
			},
			{
				Name:   "tcp-listener",
				Routes: []structs.ResourceReference{{Kind: structs.TCPRoute, Name: "tcp-route"}},
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

// TestStateStore_GatewayServices_APIGateway_FeatureGated verifies that without
// the cluster-wide flag (e.g. mid rolling upgrade), no mappings are created, so
// mixed-version servers stay consistent.
func TestStateStore_GatewayServices_APIGateway_FeatureGated(t *testing.T) {
	s := testStateStore(t)
	ws := memdb.NewWatchSet()

	// NOTE: the feature flag is intentionally NOT set here.

	testRegisterNode(t, s, 0, "node1")
	testRegisterAPIService(t, s, 1, "node1", "api-gw")
	testRegisterConnectService(t, s, 2, "node1", "web")

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
		},
	}
	require.NoError(t, s.EnsureConfigEntry(5, apigw))

	route := &structs.HTTPRouteConfigEntry{
		Kind:    structs.HTTPRoute,
		Name:    "web-route",
		Parents: []structs.ResourceReference{{Kind: structs.APIGateway, Name: "api-gw"}},
		Rules: []structs.HTTPRouteRule{
			{Services: []structs.HTTPService{{Name: "web"}}},
		},
	}
	require.NoError(t, s.EnsureConfigEntry(6, route))

	bound := &structs.BoundAPIGatewayConfigEntry{
		Kind: structs.BoundAPIGateway,
		Name: "api-gw",
		Listeners: []structs.BoundAPIGatewayListener{
			{
				Name:   "http-listener",
				Routes: []structs.ResourceReference{{Kind: structs.HTTPRoute, Name: "web-route"}},
			},
		},
	}
	require.NoError(t, s.EnsureConfigEntry(7, bound))

	// With the flag off, no mappings should be created.
	_, res, err := s.GatewayServices(ws, "api-gw", nil)
	require.NoError(t, err)
	require.Empty(t, res, "no mappings expected while the feature flag is unset")

	// Enabling the flag and re-applying the bound entry (what the leader backfill
	// does) materializes the mappings.
	enableAPIGatewayDNS(t, s, 8)
	require.NoError(t, s.EnsureConfigEntry(9, bound))

	_, res, err = s.GatewayServices(ws, "api-gw", nil)
	require.NoError(t, err)
	require.Len(t, res, 1)
	require.Equal(t, "web", res[0].Service.Name)
	require.Equal(t, structs.ServiceKindAPIGateway, res[0].GatewayKind)
}
