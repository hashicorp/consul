package gateways

import (
	"fmt"
	"testing"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/require"
)

func TestGetGatewayMeta(t *testing.T) {
	name := "Gateway"

	bound := &structs.BoundAPIGatewayConfigEntry{
		Kind: structs.BoundAPIGateway,
		Name: name,
		Listeners: []structs.BoundAPIGatewayListener{
			{
				Name:   "Listener",
				Routes: []structs.ResourceReference{},
			},
		},
	}
	gateway := &structs.APIGatewayConfigEntry{
		Kind: structs.APIGateway,
		Name: name,
		Listeners: []structs.APIGatewayListener{
			{
				Name:     "Listener",
				Protocol: structs.ListenerProtocolHTTP,
			},
		},
	}

	store := state.NewStateStore(nil)
	err := store.EnsureConfigEntry(0, bound)
	require.NoError(t, err)
	err = store.EnsureConfigEntry(1, gateway)
	require.NoError(t, err)

	gatewayMeta, err := GetGatewayMeta(store, name, nil)

	require.NoError(t, err)
	require.Equal(t, bound, gatewayMeta.Bound)
	require.Equal(t, gateway, gatewayMeta.Gateway)
}

func TestBoundAPIGatewayBindRoute(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		gateway              GatewayMeta
		route                structs.BoundRoute
		expectedBoundGateway structs.BoundAPIGatewayConfigEntry
		expectedDidBind      bool
		expectedErr          error
	}{
		"Bind TCP Route to Gateway": {
			gateway: GatewayMeta{
				Bound: &structs.BoundAPIGatewayConfigEntry{
					Kind: structs.BoundAPIGateway,
					Name: "Gateway",
					Listeners: []structs.BoundAPIGatewayListener{
						{
							Name:   "Listener",
							Routes: []structs.ResourceReference{},
						},
					},
				},
				Gateway: &structs.APIGatewayConfigEntry{
					Kind: structs.APIGateway,
					Name: "Gateway",
					Listeners: []structs.APIGatewayListener{
						{
							Name:     "Listener",
							Protocol: structs.ListenerProtocolTCP,
						},
					},
				},
			},
			route: &structs.TCPRouteConfigEntry{
				Kind: structs.TCPRoute,
				Name: "Route",
				Parents: []structs.ResourceReference{
					{
						Kind:        structs.APIGateway,
						Name:        "Gateway",
						SectionName: "Listener",
					},
				},
			},
			expectedBoundGateway: structs.BoundAPIGatewayConfigEntry{
				Kind: structs.BoundAPIGateway,
				Name: "Gateway",
				Listeners: []structs.BoundAPIGatewayListener{
					{
						Name: "Listener",
						Routes: []structs.ResourceReference{
							{
								Kind: structs.TCPRoute,
								Name: "Route",
							},
						},
					},
				},
			},
			expectedDidBind: true,
		},
		"Bind TCP Route with wildcard section name to all listeners on Gateway": {
			gateway: GatewayMeta{
				Bound: &structs.BoundAPIGatewayConfigEntry{
					Kind: structs.BoundAPIGateway,
					Name: "Gateway",
					Listeners: []structs.BoundAPIGatewayListener{
						{
							Name:   "Listener 1",
							Routes: []structs.ResourceReference{},
						},
						{
							Name:   "Listener 2",
							Routes: []structs.ResourceReference{},
						},
						{
							Name:   "Listener 3",
							Routes: []structs.ResourceReference{},
						},
					},
				},
				Gateway: &structs.APIGatewayConfigEntry{
					Kind: structs.APIGateway,
					Name: "Gateway",
					Listeners: []structs.APIGatewayListener{
						{
							Name:     "Listener 1",
							Protocol: structs.ListenerProtocolTCP,
						},
						{
							Name:     "Listener 2",
							Protocol: structs.ListenerProtocolTCP,
						},
						{
							Name:     "Listener 3",
							Protocol: structs.ListenerProtocolTCP,
						},
					},
				},
			},
			route: &structs.TCPRouteConfigEntry{
				Kind: structs.TCPRoute,
				Name: "Route",
				Parents: []structs.ResourceReference{
					{
						Kind: structs.APIGateway,
						Name: "Gateway",
					},
				},
			},
			expectedBoundGateway: structs.BoundAPIGatewayConfigEntry{
				Kind: structs.BoundAPIGateway,
				Name: "Gateway",
				Listeners: []structs.BoundAPIGatewayListener{
					{
						Name: "Listener 1",
						Routes: []structs.ResourceReference{
							{
								Kind: structs.TCPRoute,
								Name: "Route",
							},
						},
					},
					{
						Name: "Listener 2",
						Routes: []structs.ResourceReference{
							{
								Kind: structs.TCPRoute,
								Name: "Route",
							},
						},
					},
					{
						Name: "Listener 3",
						Routes: []structs.ResourceReference{
							{
								Kind: structs.TCPRoute,
								Name: "Route",
							},
						},
					},
				},
			},
			expectedDidBind: true,
		},
		"TCP Route cannot bind to Gateway because the parent reference kind is not APIGateway": {
			gateway: GatewayMeta{
				Bound: &structs.BoundAPIGatewayConfigEntry{
					Kind:      structs.BoundAPIGateway,
					Name:      "Gateway",
					Listeners: []structs.BoundAPIGatewayListener{},
				},
				Gateway: &structs.APIGatewayConfigEntry{
					Kind:      structs.APIGateway,
					Name:      "Gateway",
					Listeners: []structs.APIGatewayListener{},
				},
			},
			route: &structs.TCPRouteConfigEntry{
				Kind: structs.TCPRoute,
				Name: "Route",
				Parents: []structs.ResourceReference{
					{
						Name:        "Gateway",
						SectionName: "Listener",
					},
				},
			},
			expectedBoundGateway: structs.BoundAPIGatewayConfigEntry{
				Kind:      structs.TerminatingGateway,
				Name:      "Gateway",
				Listeners: []structs.BoundAPIGatewayListener{},
			},
			expectedDidBind: false,
			expectedErr:     nil,
		},
		"TCP Route cannot bind to Gateway because the parent reference name does not match": {
			gateway: GatewayMeta{
				Bound: &structs.BoundAPIGatewayConfigEntry{
					Kind:      structs.BoundAPIGateway,
					Name:      "Gateway",
					Listeners: []structs.BoundAPIGatewayListener{},
				},
				Gateway: &structs.APIGatewayConfigEntry{
					Kind:      structs.APIGateway,
					Name:      "Gateway",
					Listeners: []structs.APIGatewayListener{},
				},
			},
			route: &structs.TCPRouteConfigEntry{
				Kind: structs.TCPRoute,
				Name: "Route",
				Parents: []structs.ResourceReference{
					{
						Kind:        structs.APIGateway,
						Name:        "Other Gateway",
						SectionName: "Listener",
					},
				},
			},
			expectedBoundGateway: structs.BoundAPIGatewayConfigEntry{
				Kind:      structs.BoundAPIGateway,
				Name:      "Gateway",
				Listeners: []structs.BoundAPIGatewayListener{},
			},
			expectedDidBind: false,
			expectedErr:     nil,
		},
		"TCP Route cannot bind to Gateway because it lacks listeners": {
			gateway: GatewayMeta{
				Bound: &structs.BoundAPIGatewayConfigEntry{
					Kind:      structs.BoundAPIGateway,
					Name:      "Gateway",
					Listeners: []structs.BoundAPIGatewayListener{},
				},
				Gateway: &structs.APIGatewayConfigEntry{
					Kind:      structs.APIGateway,
					Name:      "Gateway",
					Listeners: []structs.APIGatewayListener{},
				},
			},
			route: &structs.TCPRouteConfigEntry{
				Kind: structs.TCPRoute,
				Name: "Route",
				Parents: []structs.ResourceReference{
					{
						Kind:        structs.APIGateway,
						Name:        "Gateway",
						SectionName: "Listener",
					},
				},
			},
			expectedBoundGateway: structs.BoundAPIGatewayConfigEntry{
				Kind:      structs.BoundAPIGateway,
				Name:      "Gateway",
				Listeners: []structs.BoundAPIGatewayListener{},
			},
			expectedDidBind: false,
			expectedErr:     fmt.Errorf("route cannot bind because gateway has no listeners"),
		},
		"TCP Route cannot bind to Gateway because it has an invalid section name": {
			gateway: GatewayMeta{
				Bound: &structs.BoundAPIGatewayConfigEntry{
					Kind: structs.BoundAPIGateway,
					Name: "Gateway",
					Listeners: []structs.BoundAPIGatewayListener{
						{
							Name:   "Listener",
							Routes: []structs.ResourceReference{},
						},
					},
				},
				Gateway: &structs.APIGatewayConfigEntry{
					Kind: structs.APIGateway,
					Name: "Gateway",
					Listeners: []structs.APIGatewayListener{
						{
							Name:     "Listener",
							Protocol: structs.ListenerProtocolTCP,
						},
					},
				},
			},
			route: &structs.TCPRouteConfigEntry{
				Kind: structs.TCPRoute,
				Name: "Route",
				Parents: []structs.ResourceReference{
					{
						Kind:        structs.APIGateway,
						Name:        "Gateway",
						SectionName: "Other Listener",
					},
				},
			},
			expectedBoundGateway: structs.BoundAPIGatewayConfigEntry{
				Kind: structs.BoundAPIGateway,
				Name: "Gateway",
				Listeners: []structs.BoundAPIGatewayListener{
					{
						Name:   "Listener",
						Routes: []structs.ResourceReference{},
					},
				},
			},
			expectedDidBind: false,
			expectedErr:     fmt.Errorf("failed to bind route Route to gateway Gateway: no valid listener has name 'Other Listener' and uses tcp protocol"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ref := tc.route.GetParents()[0]

			actualDidBind, actualErr := tc.gateway.BindRoute(ref, tc.route)

			require.Equal(t, tc.expectedDidBind, actualDidBind)
			require.Equal(t, tc.expectedErr, actualErr)
			require.Equal(t, tc.expectedBoundGateway.Listeners, tc.gateway.Bound.Listeners)
		})
	}
}

func TestBoundAPIGatewayUnbindRoute(t *testing.T) {
	cases := map[string]struct {
		gateway           GatewayMeta
		route             structs.BoundRoute
		expectedGateway   structs.BoundAPIGatewayConfigEntry
		expectedDidUnbind bool
	}{
		"TCP Route unbinds from Gateway": {
			gateway: GatewayMeta{
				Bound: &structs.BoundAPIGatewayConfigEntry{
					Kind: structs.BoundAPIGateway,
					Name: "Gateway",
					Listeners: []structs.BoundAPIGatewayListener{
						{
							Name: "Listener",
							Routes: []structs.ResourceReference{
								{
									Kind: structs.TCPRoute,
									Name: "Route",
								},
							},
						},
					},
				},
			},
			route: &structs.TCPRouteConfigEntry{
				Kind: structs.TCPRoute,
				Name: "Route",
				Parents: []structs.ResourceReference{
					{
						Kind:        structs.BoundAPIGateway,
						Name:        "Gateway",
						SectionName: "Listener",
					},
				},
			},
			expectedGateway: structs.BoundAPIGatewayConfigEntry{
				Kind: structs.BoundAPIGateway,
				Name: "Gateway",
				Listeners: []structs.BoundAPIGatewayListener{
					{
						Name:   "Listener",
						Routes: []structs.ResourceReference{},
					},
				},
			},
			expectedDidUnbind: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			actualDidUnbind := tc.gateway.UnbindRoute(tc.route)

			require.Equal(t, tc.expectedDidUnbind, actualDidUnbind)
			require.Equal(t, tc.expectedGateway.Listeners, tc.gateway.Bound.Listeners)
		})
	}
}
