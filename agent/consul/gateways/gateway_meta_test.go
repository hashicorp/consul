package gateways

import (
	"fmt"
	"testing"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/require"
)

func TestBoundAPIGatewayBindRoute(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		gateway              gatewayMeta
		route                structs.BoundRoute
		expectedBoundGateway structs.BoundAPIGatewayConfigEntry
		expectedDidBind      bool
		expectedErr          error
	}{
		"Bind TCP Route to Gateway": {
			gateway: gatewayMeta{
				BoundGateway: &structs.BoundAPIGatewayConfigEntry{
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
			gateway: gatewayMeta{
				BoundGateway: &structs.BoundAPIGatewayConfigEntry{
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
			gateway: gatewayMeta{
				BoundGateway: &structs.BoundAPIGatewayConfigEntry{
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
			gateway: gatewayMeta{
				BoundGateway: &structs.BoundAPIGatewayConfigEntry{
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
			gateway: gatewayMeta{
				BoundGateway: &structs.BoundAPIGatewayConfigEntry{
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
			gateway: gatewayMeta{
				BoundGateway: &structs.BoundAPIGatewayConfigEntry{
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

			actualDidBind, actualErr := tc.gateway.bindRoute(ref, tc.route)

			require.Equal(t, tc.expectedDidBind, actualDidBind)
			require.Equal(t, tc.expectedErr, actualErr)
			require.Equal(t, tc.expectedBoundGateway.Listeners, tc.gateway.BoundGateway.Listeners)
		})
	}
}

func TestBoundAPIGatewayUnbindRoute(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		gateway           gatewayMeta
		route             structs.BoundRoute
		expectedGateway   structs.BoundAPIGatewayConfigEntry
		expectedDidUnbind bool
	}{
		"TCP Route unbinds from Gateway": {
			gateway: gatewayMeta{
				BoundGateway: &structs.BoundAPIGatewayConfigEntry{
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
			actualDidUnbind := tc.gateway.unbindRoute(tc.route)

			require.Equal(t, tc.expectedDidUnbind, actualDidUnbind)
			require.Equal(t, tc.expectedGateway.Listeners, tc.gateway.BoundGateway.Listeners)
		})
	}
}
