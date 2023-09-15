package gateways

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/controller"
	"github.com/hashicorp/consul/agent/consul/fsm"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
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
					Status: structs.Status{
						Conditions: []structs.Condition{
							gatewayAccepted(),
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
				Status: structs.Status{
					Conditions: []structs.Condition{
						routeAccepted(),
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
					Status: structs.Status{
						Conditions: []structs.Condition{
							gatewayAccepted(),
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
				Status: structs.Status{
					Conditions: []structs.Condition{
						routeAccepted(),
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
						Kind:        "Foo",
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
			expectedErr:     fmt.Errorf("failed to bind route Route to gateway Gateway with listener 'Other Listener'"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ref := tc.route.GetParents()[0]

			actualDidBind, _, actualErrors := (&tc.gateway).initialize().updateRouteBinding(tc.route)

			require.Equal(t, tc.expectedDidBind, actualDidBind)
			require.Equal(t, tc.expectedErr, actualErrors[ref])
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
				Gateway: &structs.APIGatewayConfigEntry{},
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
			routeRef := structs.ResourceReference{
				Kind:           tc.route.GetKind(),
				Name:           tc.route.GetName(),
				EnterpriseMeta: *tc.route.GetEnterpriseMeta(),
			}
			actualDidUnbind := (&tc.gateway).initialize().unbindRoute(routeRef)

			require.Equal(t, tc.expectedDidUnbind, actualDidUnbind)
			require.Equal(t, tc.expectedGateway.Listeners, tc.gateway.BoundGateway.Listeners)
		})
	}
}

func TestBindRoutesToGateways(t *testing.T) {
	t.Parallel()

	type testCase struct {
		gateways                 []*gatewayMeta
		routes                   []structs.BoundRoute
		expectedBoundAPIGateways []*structs.BoundAPIGatewayConfigEntry
		expectedReferenceErrors  map[structs.ResourceReference]error
	}

	cases := map[string]testCase{
		"TCP Route binds to gateway": {
			gateways: []*gatewayMeta{
				{
					BoundGateway: &structs.BoundAPIGatewayConfigEntry{
						Name: "Gateway",
						Listeners: []structs.BoundAPIGatewayListener{
							{
								Name: "Listener",
							},
						},
					},
					Gateway: &structs.APIGatewayConfigEntry{
						Name: "Gateway",
						Listeners: []structs.APIGatewayListener{
							{
								Name:     "Listener",
								Protocol: structs.ListenerProtocolTCP,
							},
						},
						Status: structs.Status{
							Conditions: []structs.Condition{
								gatewayAccepted(),
							},
						},
					},
				},
			},
			routes: []structs.BoundRoute{
				&structs.TCPRouteConfigEntry{
					Kind: structs.TCPRoute,
					Name: "TCP Route",
					Parents: []structs.ResourceReference{
						{
							Name:        "Gateway",
							Kind:        structs.APIGateway,
							SectionName: "Listener",
						},
					},
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
						},
					},
				},
			},
			expectedBoundAPIGateways: []*structs.BoundAPIGatewayConfigEntry{
				{
					Name: "Gateway",
					Listeners: []structs.BoundAPIGatewayListener{
						{
							Name: "Listener",
							Routes: []structs.ResourceReference{
								{
									Name:        "TCP Route",
									Kind:        structs.TCPRoute,
									SectionName: "",
								},
							},
						},
					},
				},
			},
			expectedReferenceErrors: map[structs.ResourceReference]error{},
		},
		"TCP Route unbinds from gateway": {
			gateways: []*gatewayMeta{
				{
					BoundGateway: &structs.BoundAPIGatewayConfigEntry{
						Name: "Gateway",
						Listeners: []structs.BoundAPIGatewayListener{
							{
								Name: "Listener",
								Routes: []structs.ResourceReference{
									{
										Name:        "TCP Route",
										Kind:        structs.TCPRoute,
										SectionName: "",
									},
								},
							},
						},
					},
					Gateway: &structs.APIGatewayConfigEntry{
						Name: "Gateway",
						Listeners: []structs.APIGatewayListener{
							{
								Name:     "Listener",
								Protocol: structs.ListenerProtocolTCP,
							},
						},
					},
				},
			},
			routes: []structs.BoundRoute{
				&structs.TCPRouteConfigEntry{
					Kind:    structs.TCPRoute,
					Name:    "TCP Route",
					Parents: []structs.ResourceReference{},
				},
			},
			expectedBoundAPIGateways: []*structs.BoundAPIGatewayConfigEntry{
				{
					Name: "Gateway",
					Listeners: []structs.BoundAPIGatewayListener{
						{
							Name:   "Listener",
							Routes: []structs.ResourceReference{},
						},
					},
				},
			},
			expectedReferenceErrors: map[structs.ResourceReference]error{},
		},
		"TCP Route binds to multiple gateways": {
			gateways: []*gatewayMeta{
				{
					BoundGateway: &structs.BoundAPIGatewayConfigEntry{
						Name: "Gateway 1",
						Listeners: []structs.BoundAPIGatewayListener{
							{
								Name:   "Listener",
								Routes: []structs.ResourceReference{},
							},
						},
					},
					Gateway: &structs.APIGatewayConfigEntry{
						Name: "Gateway 1",
						Listeners: []structs.APIGatewayListener{
							{
								Name:     "Listener",
								Protocol: structs.ListenerProtocolTCP,
							},
						},
						Status: structs.Status{
							Conditions: []structs.Condition{
								gatewayAccepted(),
							},
						},
					},
				},
				{
					BoundGateway: &structs.BoundAPIGatewayConfigEntry{
						Name: "Gateway 2",
						Listeners: []structs.BoundAPIGatewayListener{
							{
								Name:   "Listener",
								Routes: []structs.ResourceReference{},
							},
						},
					},
					Gateway: &structs.APIGatewayConfigEntry{
						Name: "Gateway 2",
						Listeners: []structs.APIGatewayListener{
							{
								Name:     "Listener",
								Protocol: structs.ListenerProtocolTCP,
							},
						},
						Status: structs.Status{
							Conditions: []structs.Condition{
								gatewayAccepted(),
							},
						},
					},
				},
			},
			routes: []structs.BoundRoute{
				&structs.TCPRouteConfigEntry{
					Kind: structs.TCPRoute,
					Name: "TCP Route",
					Parents: []structs.ResourceReference{
						{
							Name:        "Gateway 1",
							Kind:        structs.APIGateway,
							SectionName: "Listener",
						},
						{
							Name:        "Gateway 2",
							Kind:        structs.APIGateway,
							SectionName: "Listener",
						},
					},
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
						},
					},
				},
			},
			expectedBoundAPIGateways: []*structs.BoundAPIGatewayConfigEntry{
				{
					Name: "Gateway 1",
					Listeners: []structs.BoundAPIGatewayListener{
						{
							Name: "Listener",
							Routes: []structs.ResourceReference{
								{
									Name:        "TCP Route",
									Kind:        structs.TCPRoute,
									SectionName: "",
								},
							},
						},
					},
				},
				{
					Name: "Gateway 2",
					Listeners: []structs.BoundAPIGatewayListener{
						{
							Name: "Listener",
							Routes: []structs.ResourceReference{
								{
									Name:        "TCP Route",
									Kind:        structs.TCPRoute,
									SectionName: "",
								},
							},
						},
					},
				},
			},
			expectedReferenceErrors: map[structs.ResourceReference]error{},
		},
		"TCP Route binds to a single listener on a gateway with multiple listeners": {
			gateways: []*gatewayMeta{
				{
					BoundGateway: &structs.BoundAPIGatewayConfigEntry{
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
						},
					},
					Gateway: &structs.APIGatewayConfigEntry{
						Name: "Gateway",
						Listeners: []structs.APIGatewayListener{
							{
								Name:     "Listener 1",
								Protocol: structs.ListenerProtocolHTTP,
							},
							{
								Name:     "Listener 2",
								Protocol: structs.ListenerProtocolTCP,
							},
						},
						Status: structs.Status{
							Conditions: []structs.Condition{
								gatewayAccepted(),
							},
						},
					},
				},
			},
			routes: []structs.BoundRoute{
				&structs.TCPRouteConfigEntry{
					Name: "TCP Route",
					Kind: structs.TCPRoute,
					Parents: []structs.ResourceReference{
						{
							Name:        "Gateway",
							Kind:        structs.APIGateway,
							SectionName: "Listener 2",
						},
					},
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
						},
					},
				},
			},
			expectedBoundAPIGateways: []*structs.BoundAPIGatewayConfigEntry{
				{
					Name: "Gateway",
					Listeners: []structs.BoundAPIGatewayListener{
						{
							Name:   "Listener 1",
							Routes: []structs.ResourceReference{},
						},
						{
							Name: "Listener 2",
							Routes: []structs.ResourceReference{
								{
									Name:        "TCP Route",
									Kind:        structs.TCPRoute,
									SectionName: "",
								},
							},
						},
					},
				},
			},
			expectedReferenceErrors: map[structs.ResourceReference]error{},
		},
		"TCP Route binds to all listeners on a gateway": {
			gateways: []*gatewayMeta{
				{
					BoundGateway: &structs.BoundAPIGatewayConfigEntry{
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
						},
					},
					Gateway: &structs.APIGatewayConfigEntry{
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
						},
						Status: structs.Status{
							Conditions: []structs.Condition{
								gatewayAccepted(),
							},
						},
					},
				},
			},
			routes: []structs.BoundRoute{
				&structs.TCPRouteConfigEntry{
					Name: "TCP Route",
					Kind: structs.TCPRoute,
					Parents: []structs.ResourceReference{
						{
							Name:        "Gateway",
							Kind:        structs.APIGateway,
							SectionName: "",
						},
					},
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
						},
					},
				},
			},
			expectedBoundAPIGateways: []*structs.BoundAPIGatewayConfigEntry{
				{
					Name: "Gateway",
					Listeners: []structs.BoundAPIGatewayListener{
						{
							Name: "Listener 1",
							Routes: []structs.ResourceReference{
								{
									Name:        "TCP Route",
									Kind:        structs.TCPRoute,
									SectionName: "",
								},
							},
						},
						{
							Name: "Listener 2",
							Routes: []structs.ResourceReference{
								{
									Name:        "TCP Route",
									Kind:        structs.TCPRoute,
									SectionName: "",
								},
							},
						},
					},
				},
			},
			expectedReferenceErrors: map[structs.ResourceReference]error{},
		},
		"TCP Route binds to gateway with multiple listeners, one of which is already bound": {
			gateways: []*gatewayMeta{
				{
					BoundGateway: &structs.BoundAPIGatewayConfigEntry{
						Name: "Gateway",
						Listeners: []structs.BoundAPIGatewayListener{
							{
								Name: "Listener 1",
								Routes: []structs.ResourceReference{
									{
										Name:        "TCP Route",
										Kind:        structs.TCPRoute,
										SectionName: "",
									},
								},
							},
							{
								Name:   "Listener 2",
								Routes: []structs.ResourceReference{},
							},
						},
					},
					Gateway: &structs.APIGatewayConfigEntry{
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
						},
						Status: structs.Status{
							Conditions: []structs.Condition{
								gatewayAccepted(),
							},
						},
					},
				},
			},
			routes: []structs.BoundRoute{
				&structs.TCPRouteConfigEntry{
					Name: "TCP Route",
					Kind: structs.TCPRoute,
					Parents: []structs.ResourceReference{
						{
							Name:        "Gateway",
							Kind:        structs.APIGateway,
							SectionName: "",
						},
					},
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
						},
					},
				},
			},
			expectedBoundAPIGateways: []*structs.BoundAPIGatewayConfigEntry{
				{
					Name: "Gateway",
					Listeners: []structs.BoundAPIGatewayListener{
						{
							Name: "Listener 1",
							Routes: []structs.ResourceReference{
								{
									Name:        "TCP Route",
									Kind:        structs.TCPRoute,
									SectionName: "",
								},
							},
						},
						{
							Name: "Listener 2",
							Routes: []structs.ResourceReference{
								{
									Name:        "TCP Route",
									Kind:        structs.TCPRoute,
									SectionName: "",
								},
							},
						},
					},
				},
			},
			expectedReferenceErrors: map[structs.ResourceReference]error{},
		},
		"TCP Route binds to a listener on multiple gateways": {
			gateways: []*gatewayMeta{
				{
					BoundGateway: &structs.BoundAPIGatewayConfigEntry{
						Name: "Gateway 1",
						Listeners: []structs.BoundAPIGatewayListener{
							{
								Name:   "Listener 1",
								Routes: []structs.ResourceReference{},
							},
							{
								Name:   "Listener 2",
								Routes: []structs.ResourceReference{},
							},
						},
					},
					Gateway: &structs.APIGatewayConfigEntry{
						Name: "Gateway 1",
						Listeners: []structs.APIGatewayListener{
							{
								Name:     "Listener 1",
								Protocol: structs.ListenerProtocolTCP,
							},
							{
								Name:     "Listener 2",
								Protocol: structs.ListenerProtocolTCP,
							},
						},
						Status: structs.Status{
							Conditions: []structs.Condition{
								gatewayAccepted(),
							},
						},
					},
				},
				{
					BoundGateway: &structs.BoundAPIGatewayConfigEntry{
						Name: "Gateway 2",
						Listeners: []structs.BoundAPIGatewayListener{
							{
								Name:   "Listener 1",
								Routes: []structs.ResourceReference{},
							},
							{
								Name:   "Listener 2",
								Routes: []structs.ResourceReference{},
							},
						},
					},
					Gateway: &structs.APIGatewayConfigEntry{
						Name: "Gateway 2",
						Listeners: []structs.APIGatewayListener{
							{
								Name:     "Listener 1",
								Protocol: structs.ListenerProtocolTCP,
							},
							{
								Name:     "Listener 2",
								Protocol: structs.ListenerProtocolTCP,
							},
						},
						Status: structs.Status{
							Conditions: []structs.Condition{
								gatewayAccepted(),
							},
						},
					},
				},
			},
			routes: []structs.BoundRoute{
				&structs.TCPRouteConfigEntry{
					Name: "TCP Route",
					Kind: structs.TCPRoute,
					Parents: []structs.ResourceReference{
						{
							Name:        "Gateway 1",
							Kind:        structs.APIGateway,
							SectionName: "Listener 2",
						},
						{
							Name:        "Gateway 2",
							Kind:        structs.APIGateway,
							SectionName: "Listener 2",
						},
					},
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
						},
					},
				},
			},
			expectedBoundAPIGateways: []*structs.BoundAPIGatewayConfigEntry{
				{
					Name: "Gateway 1",
					Listeners: []structs.BoundAPIGatewayListener{
						{
							Name:   "Listener 1",
							Routes: []structs.ResourceReference{},
						},
						{
							Name: "Listener 2",
							Routes: []structs.ResourceReference{
								{
									Name:        "TCP Route",
									Kind:        structs.TCPRoute,
									SectionName: "",
								},
							},
						},
					},
				},
				{
					Name: "Gateway 2",
					Listeners: []structs.BoundAPIGatewayListener{
						{
							Name:   "Listener 1",
							Routes: []structs.ResourceReference{},
						},
						{
							Name: "Listener 2",
							Routes: []structs.ResourceReference{
								{
									Name:        "TCP Route",
									Kind:        structs.TCPRoute,
									SectionName: "",
								},
							},
						},
					},
				},
			},
			expectedReferenceErrors: map[structs.ResourceReference]error{},
		},
		"TCP Route swaps from one listener to another on a gateway": {
			gateways: []*gatewayMeta{
				{
					BoundGateway: &structs.BoundAPIGatewayConfigEntry{
						Name: "Gateway",
						Listeners: []structs.BoundAPIGatewayListener{
							{
								Name: "Listener 1",
								Routes: []structs.ResourceReference{
									{
										Name:        "TCP Route",
										Kind:        structs.TCPRoute,
										SectionName: "",
									},
								},
							},
							{
								Name:   "Listener 2",
								Routes: []structs.ResourceReference{},
							},
						},
					},
					Gateway: &structs.APIGatewayConfigEntry{
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
						},
						Status: structs.Status{
							Conditions: []structs.Condition{
								gatewayAccepted(),
							},
						},
					},
				},
			},
			routes: []structs.BoundRoute{
				&structs.TCPRouteConfigEntry{
					Name: "TCP Route",
					Kind: structs.TCPRoute,
					Parents: []structs.ResourceReference{
						{
							Name:        "Gateway",
							Kind:        structs.APIGateway,
							SectionName: "Listener 2",
						},
					},
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
						},
					},
				},
			},
			expectedBoundAPIGateways: []*structs.BoundAPIGatewayConfigEntry{
				{
					Name: "Gateway",
					Listeners: []structs.BoundAPIGatewayListener{
						{
							Name:   "Listener 1",
							Routes: []structs.ResourceReference{},
						},
						{
							Name: "Listener 2",
							Routes: []structs.ResourceReference{
								{
									Name:        "TCP Route",
									Kind:        structs.TCPRoute,
									SectionName: "",
								},
							},
						},
					},
				},
			},
			expectedReferenceErrors: map[structs.ResourceReference]error{},
		},
		"Multiple TCP Routes bind to different gateways": {
			gateways: []*gatewayMeta{
				{
					BoundGateway: &structs.BoundAPIGatewayConfigEntry{
						Name: "Gateway 1",
						Listeners: []structs.BoundAPIGatewayListener{
							{
								Name:   "Listener 1",
								Routes: []structs.ResourceReference{},
							},
						},
					},
					Gateway: &structs.APIGatewayConfigEntry{
						Name: "Gateway 1",
						Listeners: []structs.APIGatewayListener{
							{
								Name:     "Listener 1",
								Protocol: structs.ListenerProtocolTCP,
							},
						},
						Status: structs.Status{
							Conditions: []structs.Condition{
								gatewayAccepted(),
							},
						},
					},
				},
				{
					BoundGateway: &structs.BoundAPIGatewayConfigEntry{
						Name: "Gateway 2",
						Listeners: []structs.BoundAPIGatewayListener{
							{
								Name:   "Listener 2",
								Routes: []structs.ResourceReference{},
							},
						},
					},
					Gateway: &structs.APIGatewayConfigEntry{
						Name: "Gateway 2",
						Listeners: []structs.APIGatewayListener{
							{
								Name:     "Listener 2",
								Protocol: structs.ListenerProtocolTCP,
							},
						},
						Status: structs.Status{
							Conditions: []structs.Condition{
								gatewayAccepted(),
							},
						},
					},
				},
			},
			routes: []structs.BoundRoute{
				&structs.TCPRouteConfigEntry{
					Name: "TCP Route 1",
					Kind: structs.TCPRoute,
					Parents: []structs.ResourceReference{
						{
							Name:        "Gateway 1",
							Kind:        structs.APIGateway,
							SectionName: "Listener 1",
						},
					},
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
						},
					},
				},
				&structs.TCPRouteConfigEntry{
					Name: "TCP Route 2",
					Kind: structs.TCPRoute,
					Parents: []structs.ResourceReference{
						{
							Name:        "Gateway 2",
							Kind:        structs.APIGateway,
							SectionName: "Listener 2",
						},
					},
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
						},
					},
				},
			},
			expectedBoundAPIGateways: []*structs.BoundAPIGatewayConfigEntry{
				{
					Name: "Gateway 1",
					Listeners: []structs.BoundAPIGatewayListener{
						{
							Name: "Listener 1",
							Routes: []structs.ResourceReference{
								{
									Name:        "TCP Route 1",
									Kind:        structs.TCPRoute,
									SectionName: "",
								},
							},
						},
					},
				},
				{
					Name: "Gateway 2",
					Listeners: []structs.BoundAPIGatewayListener{
						{
							Name: "Listener 2",
							Routes: []structs.ResourceReference{
								{
									Name:        "TCP Route 2",
									Kind:        structs.TCPRoute,
									SectionName: "",
								},
							},
						},
					},
				},
			},
			expectedReferenceErrors: map[structs.ResourceReference]error{},
		},
		"TCP Route cannot be bound to a listener with an HTTP protocol": {
			gateways: []*gatewayMeta{
				{
					BoundGateway: &structs.BoundAPIGatewayConfigEntry{
						Name: "Gateway",
						Listeners: []structs.BoundAPIGatewayListener{
							{
								Name:   "Listener",
								Routes: []structs.ResourceReference{},
							},
						},
					},
					Gateway: &structs.APIGatewayConfigEntry{
						Name: "Gateway",
						Listeners: []structs.APIGatewayListener{
							{
								Name:     "Listener",
								Protocol: structs.ListenerProtocolHTTP,
							},
						},
						Status: structs.Status{
							Conditions: []structs.Condition{
								gatewayAccepted(),
							},
						},
					},
				},
			},
			routes: []structs.BoundRoute{
				&structs.TCPRouteConfigEntry{
					Name: "TCP Route",
					Kind: structs.TCPRoute,
					Parents: []structs.ResourceReference{
						{
							Name:        "Gateway",
							Kind:        structs.APIGateway,
							SectionName: "Listener",
						},
					},
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
						},
					},
				},
			},
			expectedBoundAPIGateways: []*structs.BoundAPIGatewayConfigEntry{},
			expectedReferenceErrors: map[structs.ResourceReference]error{
				{
					Name:        "Gateway",
					Kind:        structs.APIGateway,
					SectionName: "Listener",
				}: fmt.Errorf("failed to bind route TCP Route to gateway Gateway: listener Listener is not a tcp listener"),
			},
		},
		"If a route/listener protocol mismatch occurs with the wildcard, but a bind to another listener was possible, no error is returned": {
			gateways: []*gatewayMeta{
				{
					BoundGateway: &structs.BoundAPIGatewayConfigEntry{
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
						},
					},
					Gateway: &structs.APIGatewayConfigEntry{
						Name: "Gateway",
						Listeners: []structs.APIGatewayListener{
							{
								Name:     "Listener 1",
								Protocol: structs.ListenerProtocolHTTP,
							},
							{
								Name:     "Listener 2",
								Protocol: structs.ListenerProtocolTCP,
							},
						},
						Status: structs.Status{
							Conditions: []structs.Condition{
								gatewayAccepted(),
							},
						},
					},
				},
			},
			routes: []structs.BoundRoute{
				&structs.TCPRouteConfigEntry{
					Name: "TCP Route",
					Kind: structs.TCPRoute,
					Parents: []structs.ResourceReference{
						{
							Name:        "Gateway",
							Kind:        structs.APIGateway,
							SectionName: "",
						},
					},
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
						},
					},
				},
			},
			expectedBoundAPIGateways: []*structs.BoundAPIGatewayConfigEntry{
				{
					Name: "Gateway",
					Listeners: []structs.BoundAPIGatewayListener{
						{
							Name:   "Listener 1",
							Routes: []structs.ResourceReference{},
						},
						{
							Name: "Listener 2",
							Routes: []structs.ResourceReference{
								{
									Name:        "TCP Route",
									Kind:        structs.TCPRoute,
									SectionName: "",
								},
							},
						},
					},
				},
			},
			expectedReferenceErrors: map[structs.ResourceReference]error{},
		},
		"TCP Route references a listener that does not exist": {
			gateways: []*gatewayMeta{
				{
					BoundGateway: &structs.BoundAPIGatewayConfigEntry{
						Name: "Gateway",
						Listeners: []structs.BoundAPIGatewayListener{
							{
								Name:   "Listener",
								Routes: []structs.ResourceReference{},
							},
						},
					},
					Gateway: &structs.APIGatewayConfigEntry{
						Name: "Gateway",
						Listeners: []structs.APIGatewayListener{
							{
								Name:     "Listener",
								Protocol: structs.ListenerProtocolTCP,
							},
						},
					},
				},
			},
			routes: []structs.BoundRoute{
				&structs.TCPRouteConfigEntry{
					Name: "TCP Route",
					Kind: structs.TCPRoute,
					Parents: []structs.ResourceReference{
						{
							Name:        "Gateway",
							Kind:        structs.APIGateway,
							SectionName: "Non-existent Listener",
						},
					},
				},
			},
			expectedBoundAPIGateways: []*structs.BoundAPIGatewayConfigEntry{},
			expectedReferenceErrors: map[structs.ResourceReference]error{
				{
					Name:        "Gateway",
					Kind:        structs.APIGateway,
					SectionName: "Non-existent Listener",
				}: fmt.Errorf("failed to bind route TCP Route to gateway Gateway with listener 'Non-existent Listener'"),
			},
		},
		"Already bound TCP Route": {
			gateways: []*gatewayMeta{
				{
					BoundGateway: &structs.BoundAPIGatewayConfigEntry{
						Name: "Gateway",
						Listeners: []structs.BoundAPIGatewayListener{
							{
								Name: "Listener",
								Routes: []structs.ResourceReference{{
									Kind: structs.TCPRoute,
									Name: "TCP Route",
								}},
							},
						},
					},
					Gateway: &structs.APIGatewayConfigEntry{
						Name: "Gateway",
						Listeners: []structs.APIGatewayListener{
							{
								Name:     "Listener",
								Protocol: structs.ListenerProtocolTCP,
							},
						},
						Status: structs.Status{
							Conditions: []structs.Condition{
								gatewayAccepted(),
							},
						},
					},
				},
			},
			routes: []structs.BoundRoute{
				&structs.TCPRouteConfigEntry{
					Name: "TCP Route",
					Kind: structs.TCPRoute,
					Parents: []structs.ResourceReference{
						{
							Name: "Gateway",
							Kind: structs.APIGateway,
						},
					},
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
						},
					},
				},
			},
			expectedBoundAPIGateways: []*structs.BoundAPIGatewayConfigEntry{},
			expectedReferenceErrors:  map[structs.ResourceReference]error{},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			for i := range tc.gateways {
				tc.gateways[i].initialize()
			}

			actualBoundAPIGatewaysMap := make(map[string]*structs.BoundAPIGatewayConfigEntry)
			referenceErrors := make(map[structs.ResourceReference]error)
			for _, route := range tc.routes {
				bound, _, errs := bindRoutesToGateways(route, tc.gateways...)
				for ref, err := range errs {
					referenceErrors[ref] = err
				}
				for _, g := range bound {
					actualBoundAPIGatewaysMap[g.Name] = g
				}
			}

			actualBoundAPIGateways := []*structs.BoundAPIGatewayConfigEntry{}
			for _, g := range actualBoundAPIGatewaysMap {
				actualBoundAPIGateways = append(actualBoundAPIGateways, g)
			}

			require.ElementsMatch(t, tc.expectedBoundAPIGateways, actualBoundAPIGateways)
			require.Equal(t, tc.expectedReferenceErrors, referenceErrors)
		})
	}
}

func TestAPIGatewayController(t *testing.T) {
	defaultMeta := acl.DefaultEnterpriseMeta()
	for name, tc := range map[string]struct {
		requests       []controller.Request
		initialEntries []structs.ConfigEntry
		finalEntries   []structs.ConfigEntry
		enqueued       []controller.Request
	}{
		"gateway-no-routes": {
			requests: []controller.Request{{
				Kind: structs.APIGateway,
				Name: "gateway",
				Meta: acl.DefaultEnterpriseMeta(),
			}},
			initialEntries: []structs.ConfigEntry{
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
				},
			},
			finalEntries: []structs.ConfigEntry{
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{
							gatewayAccepted(),
						},
					},
				},
				&structs.BoundAPIGatewayConfigEntry{
					Kind:           structs.BoundAPIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
				},
			},
		},
		"tcp-route-no-gateways-no-targets": {
			requests: []controller.Request{{
				Kind: structs.TCPRoute,
				Name: "tcp-route",
				Meta: acl.DefaultEnterpriseMeta(),
			}},
			initialEntries: []structs.ConfigEntry{
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route",
					EnterpriseMeta: *defaultMeta,
				},
			},
			finalEntries: []structs.ConfigEntry{
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeNoUpstreams(),
						},
					},
				},
			},
		},
		"http-route-no-gateways-no-targets": {
			requests: []controller.Request{{
				Kind: structs.HTTPRoute,
				Name: "http-route",
				Meta: acl.DefaultEnterpriseMeta(),
			}},
			initialEntries: []structs.ConfigEntry{
				&structs.HTTPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "http-route",
					EnterpriseMeta: *defaultMeta,
				},
			},
			finalEntries: []structs.ConfigEntry{
				&structs.HTTPRouteConfigEntry{
					Kind:           structs.HTTPRoute,
					Name:           "http-route",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeNoUpstreams(),
						},
					},
				},
			},
		},
		"tcp-route-not-accepted-bind": {
			requests: []controller.Request{{
				Kind: structs.TCPRoute,
				Name: "tcp-route",
				Meta: acl.DefaultEnterpriseMeta(),
			}},
			initialEntries: []structs.ConfigEntry{
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route",
					EnterpriseMeta: *defaultMeta,
					Services: []structs.TCPService{{
						Name: "tcp-upstream",
					}},
					Parents: []structs.ResourceReference{{
						Name:           "api-gateway",
						EnterpriseMeta: *defaultMeta,
					}},
				},
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "api-gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.APIGatewayListener{{
						Name: "listener",
						Port: 80,
					}},
				},
				&structs.BoundAPIGatewayConfigEntry{
					Kind:           structs.BoundAPIGateway,
					Name:           "api-gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.BoundAPIGatewayListener{{
						Name: "listener",
					}},
				},
			},
			finalEntries: []structs.ConfigEntry{
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
							routeUnbound(structs.ResourceReference{
								Name:           "api-gateway",
								EnterpriseMeta: *defaultMeta,
							}, errors.New("failed to bind route to gateway api-gateway: gateway has not been accepted")),
						},
					},
				},
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "api-gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.APIGatewayListener{{
						Name: "listener",
						Port: 80,
					}},
					Status: structs.Status{
						Conditions: []structs.Condition{
							gatewayListenerNoConflicts(structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "api-gateway",
								SectionName:    "listener",
								EnterpriseMeta: *defaultMeta,
							}),
						},
					},
				},
				&structs.BoundAPIGatewayConfigEntry{
					Kind:           structs.BoundAPIGateway,
					Name:           "api-gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.BoundAPIGatewayListener{{
						Name: "listener",
					}},
				},
			},
		},
		"tcp-route-no-gateways-invalid-targets-bad-protocol": {
			requests: []controller.Request{{
				Kind: structs.TCPRoute,
				Name: "tcp-route",
				Meta: acl.DefaultEnterpriseMeta(),
			}},
			initialEntries: []structs.ConfigEntry{
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route",
					EnterpriseMeta: *defaultMeta,
					Services: []structs.TCPService{{
						Name: "tcp-upstream",
					}},
				},
				&structs.ServiceConfigEntry{
					Kind:           structs.ServiceDefaults,
					Name:           "tcp-upstream",
					EnterpriseMeta: *defaultMeta,
					Protocol:       "http",
				},
			},
			finalEntries: []structs.ConfigEntry{
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeInvalidDiscoveryChain(errInvalidProtocol),
						},
					},
				},
			},
		},
		"tcp-route-no-gateways-invalid-parent": {
			requests: []controller.Request{{
				Kind: structs.TCPRoute,
				Name: "tcp-route",
				Meta: acl.DefaultEnterpriseMeta(),
			}},
			initialEntries: []structs.ConfigEntry{
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route",
					EnterpriseMeta: *defaultMeta,
					Services: []structs.TCPService{{
						Name: "tcp-upstream",
					}},
					Parents: []structs.ResourceReference{{
						Kind:           structs.APIGateway,
						Name:           "gateway",
						EnterpriseMeta: *defaultMeta,
					}},
				},
				&structs.ServiceConfigEntry{
					Kind:           structs.ServiceDefaults,
					Name:           "tcp-upstream",
					EnterpriseMeta: *defaultMeta,
				},
			},
			finalEntries: []structs.ConfigEntry{
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
							gatewayNotFound(structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
							}),
						},
					},
				},
			},
		},
		"tcp-route-parent-unreconciled": {
			requests: []controller.Request{{
				Kind: structs.TCPRoute,
				Name: "tcp-route",
				Meta: acl.DefaultEnterpriseMeta(),
			}},
			initialEntries: []structs.ConfigEntry{
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route",
					EnterpriseMeta: *defaultMeta,
					Services: []structs.TCPService{{
						Name: "tcp-upstream",
					}},
					Parents: []structs.ResourceReference{{
						Kind:           structs.APIGateway,
						Name:           "gateway",
						EnterpriseMeta: *defaultMeta,
					}},
				},
				&structs.ServiceConfigEntry{
					Kind:           structs.ServiceDefaults,
					Name:           "tcp-upstream",
					EnterpriseMeta: *defaultMeta,
				},
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
				},
			},
			finalEntries: []structs.ConfigEntry{
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
				},
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
							gatewayNotFound(structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
							}),
						},
					},
				},
			},
		},
		"tcp-route-parent-no-listeners": {
			requests: []controller.Request{{
				Kind: structs.TCPRoute,
				Name: "tcp-route",
				Meta: acl.DefaultEnterpriseMeta(),
			}},
			initialEntries: []structs.ConfigEntry{
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route",
					EnterpriseMeta: *defaultMeta,
					Services: []structs.TCPService{{
						Name: "tcp-upstream",
					}},
					Parents: []structs.ResourceReference{{
						Kind:           structs.APIGateway,
						Name:           "gateway",
						EnterpriseMeta: *defaultMeta,
					}},
				},
				&structs.ServiceConfigEntry{
					Kind:           structs.ServiceDefaults,
					Name:           "tcp-upstream",
					EnterpriseMeta: *defaultMeta,
				},
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
				},
				&structs.BoundAPIGatewayConfigEntry{
					Kind:           structs.BoundAPIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
				},
			},
			finalEntries: []structs.ConfigEntry{
				&structs.BoundAPIGatewayConfigEntry{
					Kind:           structs.BoundAPIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
				},
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
				},
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
							routeUnbound(structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
							}, errors.New("route cannot bind because gateway has no listeners")),
						},
					},
				},
			},
		},
		"tcp-route-parent-out-of-date-state": {
			requests: []controller.Request{{
				Kind: structs.TCPRoute,
				Name: "tcp-route",
				Meta: acl.DefaultEnterpriseMeta(),
			}},
			initialEntries: []structs.ConfigEntry{
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route",
					EnterpriseMeta: *defaultMeta,
					Services: []structs.TCPService{{
						Name: "tcp-upstream",
					}},
					Parents: []structs.ResourceReference{{
						Kind:           structs.APIGateway,
						Name:           "gateway",
						EnterpriseMeta: *defaultMeta,
					}},
				},
				&structs.ServiceConfigEntry{
					Kind:           structs.ServiceDefaults,
					Name:           "tcp-upstream",
					EnterpriseMeta: *defaultMeta,
				},
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.APIGatewayListener{{
						Name:     "listener",
						Protocol: structs.ListenerProtocolTCP,
						Port:     22,
					}},
				},
				&structs.BoundAPIGatewayConfigEntry{
					Kind:           structs.BoundAPIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
				},
			},
			finalEntries: []structs.ConfigEntry{
				&structs.BoundAPIGatewayConfigEntry{
					Kind:           structs.BoundAPIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
				},
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
				},
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
							gatewayNotFound(structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
							}),
						},
					},
				},
			},
		},
		"tcp-route-parent-out-of-date-state-reconcile": {
			requests: []controller.Request{{
				Kind: structs.TCPRoute,
				Name: "tcp-route",
				Meta: acl.DefaultEnterpriseMeta(),
			}, {
				Kind: structs.APIGateway,
				Name: "gateway",
				Meta: acl.DefaultEnterpriseMeta(),
			}},
			initialEntries: []structs.ConfigEntry{
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route",
					EnterpriseMeta: *defaultMeta,
					Services: []structs.TCPService{{
						Name: "tcp-upstream",
					}},
					Parents: []structs.ResourceReference{{
						Kind:           structs.APIGateway,
						Name:           "gateway",
						EnterpriseMeta: *defaultMeta,
					}},
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
						},
					},
				},
				&structs.ServiceConfigEntry{
					Kind:           structs.ServiceDefaults,
					Name:           "tcp-upstream",
					EnterpriseMeta: *defaultMeta,
				},
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.APIGatewayListener{{
						Name:     "listener",
						Protocol: structs.ListenerProtocolTCP,
						Port:     22,
					}},
					Status: structs.Status{
						Conditions: []structs.Condition{
							gatewayAccepted(),
						},
					},
				},
				&structs.BoundAPIGatewayConfigEntry{
					Kind:           structs.BoundAPIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
				},
			},
			finalEntries: []structs.ConfigEntry{
				&structs.BoundAPIGatewayConfigEntry{
					Kind:           structs.BoundAPIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.BoundAPIGatewayListener{{
						Name: "listener",
						Routes: []structs.ResourceReference{{
							Kind:           structs.TCPRoute,
							Name:           "tcp-route",
							EnterpriseMeta: *defaultMeta,
						}},
					}},
				},
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{
							gatewayAccepted(),
							gatewayListenerNoConflicts(structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
								SectionName:    "listener",
							}),
							validCertificate(structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
								SectionName:    "listener",
							}),
						},
					},
				},
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
							routeBound(structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
							}),
						},
					},
				},
			},
		},
		"tcp-route-parent-protocol-mismatch": {
			requests: []controller.Request{{
				Kind: structs.APIGateway,
				Name: "gateway",
				Meta: acl.DefaultEnterpriseMeta(),
			}, {
				Kind: structs.TCPRoute,
				Name: "tcp-route",
				Meta: acl.DefaultEnterpriseMeta(),
			}},
			initialEntries: []structs.ConfigEntry{
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route",
					EnterpriseMeta: *defaultMeta,
					Services: []structs.TCPService{{
						Name: "tcp-upstream",
					}},
					Parents: []structs.ResourceReference{{
						Kind:           structs.APIGateway,
						Name:           "gateway",
						EnterpriseMeta: *defaultMeta,
					}},
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
						},
					},
				},
				&structs.ServiceConfigEntry{
					Kind:           structs.ServiceDefaults,
					Name:           "tcp-upstream",
					EnterpriseMeta: *defaultMeta,
				},
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.APIGatewayListener{{
						Name:     "listener",
						Protocol: structs.ListenerProtocolHTTP,
						Port:     80,
					}},
				},
				&structs.BoundAPIGatewayConfigEntry{
					Kind:           structs.BoundAPIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
				},
			},
			finalEntries: []structs.ConfigEntry{
				&structs.BoundAPIGatewayConfigEntry{
					Kind:           structs.BoundAPIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.BoundAPIGatewayListener{{
						Name: "listener",
					}},
				},
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{
							gatewayAccepted(),
							gatewayListenerNoConflicts(structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
								SectionName:    "listener",
							}),
							validCertificate(structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
								SectionName:    "listener",
							}),
						},
					},
				},
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
							routeUnbound(structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
							}, errors.New("failed to bind route tcp-route to gateway gateway with listener ''")),
						},
					},
				},
			},
		},
		"tcp-conflicted-routes": {
			requests: []controller.Request{{
				Kind: structs.APIGateway,
				Name: "gateway",
				Meta: acl.DefaultEnterpriseMeta(),
			}, {
				Kind: structs.TCPRoute,
				Name: "tcp-route-one",
				Meta: acl.DefaultEnterpriseMeta(),
			}, {
				Kind: structs.TCPRoute,
				Name: "tcp-route-two",
				Meta: acl.DefaultEnterpriseMeta(),
			}},
			initialEntries: []structs.ConfigEntry{
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route-one",
					EnterpriseMeta: *defaultMeta,
					Services: []structs.TCPService{{
						Name: "tcp-upstream",
					}},
					Parents: []structs.ResourceReference{{
						Kind:           structs.APIGateway,
						Name:           "gateway",
						EnterpriseMeta: *defaultMeta,
					}},
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
						},
					},
				},
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route-two",
					EnterpriseMeta: *defaultMeta,
					Services: []structs.TCPService{{
						Name: "tcp-upstream",
					}},
					Parents: []structs.ResourceReference{{
						Kind:           structs.APIGateway,
						Name:           "gateway",
						EnterpriseMeta: *defaultMeta,
					}},
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
						},
					},
				},
				&structs.ServiceConfigEntry{
					Kind:           structs.ServiceDefaults,
					Name:           "tcp-upstream",
					EnterpriseMeta: *defaultMeta,
				},
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.APIGatewayListener{{
						Name:     "listener",
						Protocol: structs.ListenerProtocolTCP,
						Port:     22,
					}},
				},
				&structs.BoundAPIGatewayConfigEntry{
					Kind:           structs.BoundAPIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
				},
			},
			finalEntries: []structs.ConfigEntry{
				&structs.BoundAPIGatewayConfigEntry{
					Kind:           structs.BoundAPIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.BoundAPIGatewayListener{{
						Name: "listener",
						Routes: []structs.ResourceReference{{
							Kind:           structs.TCPRoute,
							Name:           "tcp-route-one",
							EnterpriseMeta: *defaultMeta,
						}, {
							Kind:           structs.TCPRoute,
							Name:           "tcp-route-two",
							EnterpriseMeta: *defaultMeta,
						}},
					}},
				},
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{
							gatewayAccepted(),
							gatewayListenerConflicts(structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
								SectionName:    "listener",
							}),
							validCertificate(structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
								SectionName:    "listener",
							}),
						},
					},
				},
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route-one",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
							routeBound(structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
							}),
						},
					},
				},
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route-two",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
							routeBound(structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
							}),
						},
					},
				},
			},
		},
		"http-route-multiple-routes": {
			requests: []controller.Request{{
				Kind: structs.APIGateway,
				Name: "gateway",
				Meta: acl.DefaultEnterpriseMeta(),
			}, {
				Kind: structs.HTTPRoute,
				Name: "http-route-one",
				Meta: acl.DefaultEnterpriseMeta(),
			}, {
				Kind: structs.HTTPRoute,
				Name: "http-route-two",
				Meta: acl.DefaultEnterpriseMeta(),
			}},
			initialEntries: []structs.ConfigEntry{
				&structs.HTTPRouteConfigEntry{
					Kind:           structs.HTTPRoute,
					Name:           "http-route-one",
					EnterpriseMeta: *defaultMeta,
					Rules: []structs.HTTPRouteRule{{
						Services: []structs.HTTPService{{
							Name: "http-upstream",
						}},
					}},
					Parents: []structs.ResourceReference{{
						Kind:           structs.APIGateway,
						Name:           "gateway",
						EnterpriseMeta: *defaultMeta,
					}},
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
						},
					},
				},
				&structs.HTTPRouteConfigEntry{
					Kind:           structs.HTTPRoute,
					Name:           "http-route-two",
					EnterpriseMeta: *defaultMeta,
					Rules: []structs.HTTPRouteRule{{
						Services: []structs.HTTPService{{
							Name: "http-upstream",
						}},
					}},
					Parents: []structs.ResourceReference{{
						Kind:           structs.APIGateway,
						Name:           "gateway",
						EnterpriseMeta: *defaultMeta,
					}},
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
						},
					},
				},
				&structs.ServiceConfigEntry{
					Kind:           structs.ServiceDefaults,
					Name:           "http-upstream",
					EnterpriseMeta: *defaultMeta,
					Protocol:       "http",
				},
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.APIGatewayListener{{
						Name:     "listener",
						Protocol: structs.ListenerProtocolHTTP,
						Port:     80,
					}},
				},
				&structs.BoundAPIGatewayConfigEntry{
					Kind:           structs.BoundAPIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
				},
			},
			finalEntries: []structs.ConfigEntry{
				&structs.BoundAPIGatewayConfigEntry{
					Kind:           structs.BoundAPIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.BoundAPIGatewayListener{{
						Name: "listener",
						Routes: []structs.ResourceReference{{
							Kind:           structs.HTTPRoute,
							Name:           "http-route-one",
							EnterpriseMeta: *defaultMeta,
						}, {
							Kind:           structs.HTTPRoute,
							Name:           "http-route-two",
							EnterpriseMeta: *defaultMeta,
						}},
					}},
				},
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{
							gatewayAccepted(),
							gatewayListenerNoConflicts(structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
								SectionName:    "listener",
							}),
							validCertificate(structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
								SectionName:    "listener",
							}),
						},
					},
				},
				&structs.HTTPRouteConfigEntry{
					Kind:           structs.HTTPRoute,
					Name:           "http-route-one",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
							routeBound(structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
							}),
						},
					},
				},
				&structs.HTTPRouteConfigEntry{
					Kind:           structs.HTTPRoute,
					Name:           "http-route-two",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
							routeBound(structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
							}),
						},
					},
				},
			},
		},
		"mixed-routes-mismatch": {
			requests: []controller.Request{{
				Kind: structs.APIGateway,
				Name: "gateway",
				Meta: acl.DefaultEnterpriseMeta(),
			}, {
				Kind: structs.TCPRoute,
				Name: "tcp-route",
				Meta: acl.DefaultEnterpriseMeta(),
			}, {
				Kind: structs.TCPRoute,
				Name: "tcp-route",
				Meta: acl.DefaultEnterpriseMeta(),
			}, {
				Kind: structs.HTTPRoute,
				Name: "http-route",
				Meta: acl.DefaultEnterpriseMeta(),
			}, {
				Kind: structs.HTTPRoute,
				Name: "http-route",
				Meta: acl.DefaultEnterpriseMeta(),
			}},
			initialEntries: []structs.ConfigEntry{
				&structs.HTTPRouteConfigEntry{
					Kind:           structs.HTTPRoute,
					Name:           "http-route",
					EnterpriseMeta: *defaultMeta,
					Rules: []structs.HTTPRouteRule{{
						Services: []structs.HTTPService{{
							Name: "http-upstream",
						}},
					}},
					Parents: []structs.ResourceReference{{
						Kind:           structs.APIGateway,
						Name:           "gateway",
						EnterpriseMeta: *defaultMeta,
					}},
				},
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route",
					EnterpriseMeta: *defaultMeta,
					Services: []structs.TCPService{{
						Name: "tcp-upstream",
					}},
					Parents: []structs.ResourceReference{{
						Kind:           structs.APIGateway,
						Name:           "gateway",
						EnterpriseMeta: *defaultMeta,
					}},
				},
				&structs.ServiceConfigEntry{
					Kind:           structs.ServiceDefaults,
					Name:           "http-upstream",
					EnterpriseMeta: *defaultMeta,
					Protocol:       "http",
				},
				&structs.ServiceConfigEntry{
					Kind:           structs.ServiceDefaults,
					Name:           "tcp-upstream",
					EnterpriseMeta: *defaultMeta,
					Protocol:       "tcp",
				},
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.APIGatewayListener{{
						Name:     "listener",
						Protocol: structs.ListenerProtocolHTTP,
						Port:     80,
					}},
				},
				&structs.BoundAPIGatewayConfigEntry{
					Kind:           structs.BoundAPIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
				},
			},
			finalEntries: []structs.ConfigEntry{
				&structs.BoundAPIGatewayConfigEntry{
					Kind:           structs.BoundAPIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.BoundAPIGatewayListener{{
						Name: "listener",
						Routes: []structs.ResourceReference{{
							Kind:           structs.HTTPRoute,
							Name:           "http-route",
							EnterpriseMeta: *defaultMeta,
						}},
					}},
				},
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{
							gatewayAccepted(),
							gatewayListenerNoConflicts(structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
								SectionName:    "listener",
							}),
							validCertificate(structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
								SectionName:    "listener",
							}),
						},
					},
				},
				&structs.HTTPRouteConfigEntry{
					Kind:           structs.HTTPRoute,
					Name:           "http-route",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
							routeBound(structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
							}),
						},
					},
				},
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
							routeUnbound(structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
							}, errors.New("failed to bind route tcp-route to gateway gateway with listener ''")),
						},
					},
				},
			},
		},
		"mixed-routes-multi-listener": {
			requests: []controller.Request{{
				Kind: structs.APIGateway,
				Name: "gateway",
				Meta: acl.DefaultEnterpriseMeta(),
			}, {
				Kind: structs.TCPRoute,
				Name: "tcp-route",
				Meta: acl.DefaultEnterpriseMeta(),
			}, {
				Kind: structs.HTTPRoute,
				Name: "http-route",
				Meta: acl.DefaultEnterpriseMeta(),
			}},
			initialEntries: []structs.ConfigEntry{
				&structs.HTTPRouteConfigEntry{
					Kind:           structs.HTTPRoute,
					Name:           "http-route",
					EnterpriseMeta: *defaultMeta,
					Rules: []structs.HTTPRouteRule{{
						Services: []structs.HTTPService{{
							Name: "http-upstream",
						}},
					}},
					Parents: []structs.ResourceReference{{
						Kind:           structs.APIGateway,
						Name:           "gateway",
						EnterpriseMeta: *defaultMeta,
					}},
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
						},
					},
				},
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route",
					EnterpriseMeta: *defaultMeta,
					Services: []structs.TCPService{{
						Name: "tcp-upstream",
					}},
					Parents: []structs.ResourceReference{{
						Kind:           structs.APIGateway,
						Name:           "gateway",
						EnterpriseMeta: *defaultMeta,
					}},
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
						},
					},
				},
				&structs.ServiceConfigEntry{
					Kind:           structs.ServiceDefaults,
					Name:           "http-upstream",
					EnterpriseMeta: *defaultMeta,
					Protocol:       "http",
				},
				&structs.ServiceConfigEntry{
					Kind:           structs.ServiceDefaults,
					Name:           "tcp-upstream",
					EnterpriseMeta: *defaultMeta,
					Protocol:       "tcp",
				},
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.APIGatewayListener{{
						Name:     "http-listener",
						Protocol: structs.ListenerProtocolHTTP,
						Port:     80,
					}, {
						Name:     "tcp-listener",
						Protocol: structs.ListenerProtocolTCP,
						Port:     22,
					}},
				},
				&structs.BoundAPIGatewayConfigEntry{
					Kind:           structs.BoundAPIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
				},
			},
			finalEntries: []structs.ConfigEntry{
				&structs.BoundAPIGatewayConfigEntry{
					Kind:           structs.BoundAPIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.BoundAPIGatewayListener{{
						Name: "http-listener",
						Routes: []structs.ResourceReference{{
							Kind:           structs.HTTPRoute,
							Name:           "http-route",
							EnterpriseMeta: *defaultMeta,
						}},
					}, {
						Name: "tcp-listener",
						Routes: []structs.ResourceReference{{
							Kind:           structs.TCPRoute,
							Name:           "tcp-route",
							EnterpriseMeta: *defaultMeta,
						}},
					}},
				},
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{
							gatewayAccepted(),
							gatewayListenerNoConflicts(structs.ResourceReference{
								Kind:        structs.APIGateway,
								Name:        "gateway",
								SectionName: "http-listener",
							}),
							gatewayListenerNoConflicts(structs.ResourceReference{
								Kind:        structs.APIGateway,
								Name:        "gateway",
								SectionName: "tcp-listener",
							}),
							validCertificate(structs.ResourceReference{
								Kind:        structs.APIGateway,
								Name:        "gateway",
								SectionName: "http-listener",
							}),
							validCertificate(structs.ResourceReference{
								Kind:        structs.APIGateway,
								Name:        "gateway",
								SectionName: "tcp-listener",
							}),
						},
					},
				},
				&structs.HTTPRouteConfigEntry{
					Kind:           structs.HTTPRoute,
					Name:           "http-route",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
							routeBound(structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
							}),
						},
					},
				},
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
							routeBound(structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
							}),
						},
					},
				},
			},
		},
		"basic-route-removal": {
			requests: []controller.Request{{
				Kind: structs.TCPRoute,
				Name: "tcp-route",
				Meta: acl.DefaultEnterpriseMeta(),
			}},
			initialEntries: []structs.ConfigEntry{
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route",
					EnterpriseMeta: *defaultMeta,
					Services: []structs.TCPService{{
						Name: "tcp-upstream",
					}},
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
							routeBound(structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
							}),
						},
					},
				},
				&structs.ServiceConfigEntry{
					Kind:           structs.ServiceDefaults,
					Name:           "tcp-upstream",
					EnterpriseMeta: *defaultMeta,
					Protocol:       "tcp",
				},
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.APIGatewayListener{{
						Name:     "tcp-listener",
						Protocol: structs.ListenerProtocolTCP,
						Port:     22,
					}},
				},
				&structs.BoundAPIGatewayConfigEntry{
					Kind:           structs.BoundAPIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.BoundAPIGatewayListener{{
						Name: "tcp-listener",
						Routes: []structs.ResourceReference{{
							Kind:           structs.TCPRoute,
							Name:           "tcp-route",
							EnterpriseMeta: *defaultMeta,
						}},
					}},
				},
			},
			finalEntries: []structs.ConfigEntry{
				&structs.BoundAPIGatewayConfigEntry{
					Kind:           structs.BoundAPIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.BoundAPIGatewayListener{{
						Name:   "tcp-listener",
						Routes: []structs.ResourceReference{},
					}},
				},
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{
							gatewayListenerNoConflicts(structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								SectionName:    "tcp-listener",
								EnterpriseMeta: *defaultMeta,
							}),
						},
					},
				},
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
						},
					},
				},
			},
		},
		"invalidated-route": {
			requests: []controller.Request{{
				Kind: structs.TCPRoute,
				Name: "tcp-route",
				Meta: acl.DefaultEnterpriseMeta(),
			}},
			initialEntries: []structs.ConfigEntry{
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
							routeBound(structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
							}),
						},
					},
				},
				&structs.ServiceConfigEntry{
					Kind:           structs.ServiceDefaults,
					Name:           "tcp-upstream",
					EnterpriseMeta: *defaultMeta,
					Protocol:       "tcp",
				},
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.APIGatewayListener{{
						Name:     "tcp-listener",
						Protocol: structs.ListenerProtocolTCP,
						Port:     22,
					}},
				},
				&structs.BoundAPIGatewayConfigEntry{
					Kind:           structs.BoundAPIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.BoundAPIGatewayListener{{
						Name: "tcp-listener",
						Routes: []structs.ResourceReference{{
							Kind:           structs.TCPRoute,
							Name:           "tcp-route",
							EnterpriseMeta: *defaultMeta,
						}},
					}},
				},
			},
			finalEntries: []structs.ConfigEntry{
				&structs.BoundAPIGatewayConfigEntry{
					Kind:           structs.BoundAPIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.BoundAPIGatewayListener{{
						Name:   "tcp-listener",
						Routes: []structs.ResourceReference{},
					}},
				},
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{
							gatewayListenerNoConflicts(structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								SectionName:    "tcp-listener",
								EnterpriseMeta: *defaultMeta,
							}),
						},
					},
				},
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeNoUpstreams(),
						},
					},
				},
			},
		},
		"swap-protocols": {
			requests: []controller.Request{{
				Kind: structs.APIGateway,
				Name: "gateway",
				Meta: acl.DefaultEnterpriseMeta(),
			}},
			initialEntries: []structs.ConfigEntry{
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route",
					EnterpriseMeta: *defaultMeta,
					Parents: []structs.ResourceReference{{
						Kind:           structs.APIGateway,
						Name:           "gateway",
						EnterpriseMeta: *defaultMeta,
					}},
					Services: []structs.TCPService{{
						Name:           "tcp-upstream",
						EnterpriseMeta: *defaultMeta,
					}},
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
							routeBound(structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
							}),
						},
					},
				},
				&structs.HTTPRouteConfigEntry{
					Kind:           structs.HTTPRoute,
					Name:           "http-route",
					EnterpriseMeta: *defaultMeta,
					Rules: []structs.HTTPRouteRule{{
						Services: []structs.HTTPService{{
							Name:           "http-upstream",
							EnterpriseMeta: *defaultMeta,
						}},
					}},
					Parents: []structs.ResourceReference{{
						Kind:           structs.APIGateway,
						Name:           "gateway",
						EnterpriseMeta: *defaultMeta,
					}},
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
							routeUnbound(structs.ResourceReference{
								Kind: structs.APIGateway,
								Name: "gateway",
							}, errors.New("foo")),
						},
					},
				},
				&structs.ServiceConfigEntry{
					Kind:           structs.ServiceDefaults,
					Name:           "tcp-upstream",
					EnterpriseMeta: *defaultMeta,
					Protocol:       "tcp",
				},
				&structs.ServiceConfigEntry{
					Kind:           structs.ServiceDefaults,
					Name:           "http-upstream",
					EnterpriseMeta: *defaultMeta,
					Protocol:       "http",
				},
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.APIGatewayListener{{
						Name:     "http-listener",
						Protocol: structs.ListenerProtocolHTTP,
						Port:     80,
					}},
					Status: structs.Status{
						Conditions: []structs.Condition{
							gatewayAccepted(),
							gatewayListenerNoConflicts(structs.ResourceReference{
								Kind:        structs.APIGateway,
								Name:        "gateway",
								SectionName: "tcp-listener",
							}),
						},
					},
				},
				&structs.BoundAPIGatewayConfigEntry{
					Kind:           structs.BoundAPIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.BoundAPIGatewayListener{{
						Name: "tcp-listener",
						Routes: []structs.ResourceReference{{
							Kind:           structs.TCPRoute,
							Name:           "tcp-route",
							EnterpriseMeta: *defaultMeta,
						}},
					}},
				},
			},
			finalEntries: []structs.ConfigEntry{
				&structs.BoundAPIGatewayConfigEntry{
					Kind:           structs.BoundAPIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.BoundAPIGatewayListener{{
						Name: "http-listener",
						Routes: []structs.ResourceReference{{
							Kind:           structs.HTTPRoute,
							Name:           "http-route",
							EnterpriseMeta: *defaultMeta,
						}},
					}},
				},
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{
							gatewayAccepted(),
							gatewayListenerNoConflicts(structs.ResourceReference{
								Kind:        structs.APIGateway,
								Name:        "gateway",
								SectionName: "http-listener",
							}),
							validCertificate(structs.ResourceReference{
								Kind:        structs.APIGateway,
								Name:        "gateway",
								SectionName: "http-listener",
							}),
						},
					},
				},
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
							routeUnbound(structs.ResourceReference{
								Kind: structs.APIGateway,
								Name: "gateway",
							}, errors.New("failed to bind route tcp-route to gateway gateway with listener ''")),
						},
					},
				},
				&structs.HTTPRouteConfigEntry{
					Kind:           structs.HTTPRoute,
					Name:           "http-route",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
							routeBound(structs.ResourceReference{
								Kind: structs.APIGateway,
								Name: "gateway",
							}),
						},
					},
				},
			},
		},
		"delete-gateway": {
			requests: []controller.Request{{
				Kind: structs.APIGateway,
				Name: "gateway",
				Meta: acl.DefaultEnterpriseMeta(),
			}, {
				Kind: structs.BoundAPIGateway,
				Name: "gateway",
				Meta: acl.DefaultEnterpriseMeta(),
			}},
			initialEntries: []structs.ConfigEntry{
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route",
					EnterpriseMeta: *defaultMeta,
					Parents: []structs.ResourceReference{{
						Kind:           structs.APIGateway,
						Name:           "gateway",
						EnterpriseMeta: *defaultMeta,
					}},
					Services: []structs.TCPService{{
						Name:           "tcp-upstream",
						EnterpriseMeta: *defaultMeta,
					}},
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
							routeBound(structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
							}),
						},
					},
				},
				&structs.ServiceConfigEntry{
					Kind:           structs.ServiceDefaults,
					Name:           "tcp-upstream",
					EnterpriseMeta: *defaultMeta,
					Protocol:       "tcp",
				},
				&structs.BoundAPIGatewayConfigEntry{
					Kind:           structs.BoundAPIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.BoundAPIGatewayListener{{
						Name: "tcp-listener",
						Routes: []structs.ResourceReference{{
							Kind:           structs.TCPRoute,
							Name:           "tcp-route",
							EnterpriseMeta: *defaultMeta,
						}},
					}},
				},
			},
			finalEntries: []structs.ConfigEntry{
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
							gatewayNotFound(structs.ResourceReference{
								Kind: structs.APIGateway,
								Name: "gateway",
							}),
						},
					},
				},
			},
		},
		"delete-route": {
			requests: []controller.Request{{
				Kind: structs.TCPRoute,
				Name: "tcp-route",
				Meta: acl.DefaultEnterpriseMeta(),
			}},
			initialEntries: []structs.ConfigEntry{
				&structs.BoundAPIGatewayConfigEntry{
					Kind:           structs.BoundAPIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.BoundAPIGatewayListener{{
						Name: "tcp-listener",
						Routes: []structs.ResourceReference{{
							Kind:           structs.TCPRoute,
							Name:           "tcp-route",
							EnterpriseMeta: *defaultMeta,
						}},
					}},
				},
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.APIGatewayListener{{
						Name:     "tcp-listener",
						Port:     22,
						Protocol: structs.ListenerProtocolTCP,
					}},
					Status: structs.Status{
						Conditions: []structs.Condition{
							gatewayAccepted(),
							gatewayListenerNoConflicts(structs.ResourceReference{
								Kind:        structs.APIGateway,
								Name:        "gateway",
								SectionName: "tcp-listener",
							}),
						},
					},
				},
			},
			finalEntries: []structs.ConfigEntry{
				&structs.BoundAPIGatewayConfigEntry{
					Kind:           structs.BoundAPIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.BoundAPIGatewayListener{{
						Name:   "tcp-listener",
						Routes: []structs.ResourceReference{},
					}},
				},
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{
							gatewayAccepted(),
							gatewayListenerNoConflicts(structs.ResourceReference{
								Kind:        structs.APIGateway,
								Name:        "gateway",
								SectionName: "tcp-listener",
							}),
						},
					},
				},
			},
		},
		"orphaned-bound-gateway": {
			requests: []controller.Request{{
				Kind: structs.BoundAPIGateway,
				Name: "gateway",
				Meta: acl.DefaultEnterpriseMeta(),
			}, {
				Kind: structs.BoundAPIGateway,
				Name: "gateway",
				Meta: acl.DefaultEnterpriseMeta(),
			}},
			initialEntries: []structs.ConfigEntry{
				&structs.BoundAPIGatewayConfigEntry{
					Kind:           structs.BoundAPIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.BoundAPIGatewayListener{{
						Name: "tcp-listener",
						Routes: []structs.ResourceReference{{
							Kind:           structs.TCPRoute,
							Name:           "tcp-route",
							EnterpriseMeta: *defaultMeta,
						}},
					}},
				},
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route",
					EnterpriseMeta: *defaultMeta,
					Parents: []structs.ResourceReference{{
						Kind: structs.APIGateway,
						Name: "gateway",
					}},
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
							routeBound(structs.ResourceReference{
								Kind: structs.APIGateway,
								Name: "gateway",
							}),
						},
					},
				},
			},
			finalEntries: []structs.ConfigEntry{
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{
							routeAccepted(),
							gatewayNotFound(structs.ResourceReference{
								Kind: structs.APIGateway,
								Name: "gateway",
							}),
						},
					},
				},
			},
		},
		"invalid-gateway-certificates": {
			requests: []controller.Request{{
				Kind: structs.APIGateway,
				Name: "gateway",
				Meta: acl.DefaultEnterpriseMeta(),
			}},
			initialEntries: []structs.ConfigEntry{
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.APIGatewayListener{{
						Name: "http-listener",
						Port: 80,
						TLS: structs.APIGatewayTLSConfiguration{
							Certificates: []structs.ResourceReference{{
								Kind: structs.InlineCertificate,
								Name: "certificate",
							}},
						},
					}},
				},
			},
			finalEntries: []structs.ConfigEntry{
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.APIGatewayListener{{
						Name: "http-listener",
						Port: 80,
						TLS: structs.APIGatewayTLSConfiguration{
							Certificates: []structs.ResourceReference{{
								Kind: structs.InlineCertificate,
								Name: "certificate",
							}},
						},
					}},
					Status: structs.Status{
						Conditions: []structs.Condition{
							invalidCertificate(structs.ResourceReference{
								Kind:        structs.APIGateway,
								Name:        "gateway",
								SectionName: "http-listener",
							}, errors.New("certificate \"certificate\" not found")),
							invalidCertificates(),
							gatewayListenerNoConflicts(structs.ResourceReference{
								Kind:        structs.APIGateway,
								Name:        "gateway",
								SectionName: "http-listener",
							}),
						},
					},
				},
				&structs.BoundAPIGatewayConfigEntry{
					Kind:           structs.BoundAPIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.BoundAPIGatewayListener{{
						Name: "http-listener",
					}},
				},
			},
		},
		"valid-gateway-certificates": {
			requests: []controller.Request{{
				Kind: structs.APIGateway,
				Name: "gateway",
				Meta: acl.DefaultEnterpriseMeta(),
			}},
			initialEntries: []structs.ConfigEntry{
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.APIGatewayListener{{
						Name: "http-listener",
						Port: 80,
						TLS: structs.APIGatewayTLSConfiguration{
							Certificates: []structs.ResourceReference{{
								Kind: structs.InlineCertificate,
								Name: "certificate",
							}},
						},
					}},
				},
				&structs.InlineCertificateConfigEntry{
					Kind:           structs.InlineCertificate,
					Name:           "certificate",
					EnterpriseMeta: *defaultMeta,
				},
			},
			finalEntries: []structs.ConfigEntry{
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.APIGatewayListener{{
						Name: "http-listener",
						Port: 80,
						TLS: structs.APIGatewayTLSConfiguration{
							Certificates: []structs.ResourceReference{{
								Kind: structs.InlineCertificate,
								Name: "certificate",
							}},
						},
					}},
					Status: structs.Status{
						Conditions: []structs.Condition{
							gatewayAccepted(),
							gatewayListenerNoConflicts(structs.ResourceReference{
								Kind:        structs.APIGateway,
								Name:        "gateway",
								SectionName: "http-listener",
							}),
							validCertificate(structs.ResourceReference{
								Kind:        structs.APIGateway,
								Name:        "gateway",
								SectionName: "http-listener",
							}),
						},
					},
				},
				&structs.BoundAPIGatewayConfigEntry{
					Kind:           structs.BoundAPIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.BoundAPIGatewayListener{{
						Name: "http-listener",
						Certificates: []structs.ResourceReference{{
							Kind:           structs.InlineCertificate,
							Name:           "certificate",
							EnterpriseMeta: *defaultMeta,
						}},
					}},
				},
			},
		},
		"all-listeners-valid-certificate-refs": {
			requests: []controller.Request{{
				Kind: structs.APIGateway,
				Name: "gateway",
				Meta: acl.DefaultEnterpriseMeta(),
			}},
			initialEntries: []structs.ConfigEntry{
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.APIGatewayListener{
						{
							Name: "listener-1",
							Port: 80,
							TLS: structs.APIGatewayTLSConfiguration{
								Certificates: []structs.ResourceReference{{
									Kind: structs.InlineCertificate,
									Name: "cert-1",
								}},
							},
						},
						{
							Name: "listener-2",
							Port: 80,
							TLS: structs.APIGatewayTLSConfiguration{
								Certificates: []structs.ResourceReference{{
									Kind: structs.InlineCertificate,
									Name: "cert-2",
								}},
							},
						},
					},
				},
				&structs.InlineCertificateConfigEntry{
					Kind:           structs.InlineCertificate,
					Name:           "cert-1",
					EnterpriseMeta: *defaultMeta,
				},
				&structs.InlineCertificateConfigEntry{
					Kind:           structs.InlineCertificate,
					Name:           "cert-2",
					EnterpriseMeta: *defaultMeta,
				},
			},
			finalEntries: []structs.ConfigEntry{
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.APIGatewayListener{
						{
							Name: "listener-1",
							Port: 80,
							TLS: structs.APIGatewayTLSConfiguration{
								Certificates: []structs.ResourceReference{{
									Kind: structs.InlineCertificate,
									Name: "cert-1",
								}},
							},
						},
						{
							Name: "listener-2",
							Port: 80,
							TLS: structs.APIGatewayTLSConfiguration{
								Certificates: []structs.ResourceReference{{
									Kind: structs.InlineCertificate,
									Name: "cert-2",
								}},
							},
						},
					},
					Status: structs.Status{
						Conditions: []structs.Condition{
							validCertificate(structs.ResourceReference{
								Kind:        structs.APIGateway,
								Name:        "gateway",
								SectionName: "listener-1",
							}),
							validCertificate(structs.ResourceReference{
								Kind:        structs.APIGateway,
								Name:        "gateway",
								SectionName: "listener-2",
							}),
							gatewayListenerNoConflicts(structs.ResourceReference{
								Kind:        structs.APIGateway,
								Name:        "gateway",
								SectionName: "listener-1",
							}),
							gatewayListenerNoConflicts(structs.ResourceReference{
								Kind:        structs.APIGateway,
								Name:        "gateway",
								SectionName: "listener-2",
							}),
							gatewayAccepted(),
						},
					},
				},
				&structs.BoundAPIGatewayConfigEntry{
					Kind:           structs.BoundAPIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.BoundAPIGatewayListener{
						{
							Name: "listener-1",
							Certificates: []structs.ResourceReference{
								{
									Kind: structs.InlineCertificate,
									Name: "cert-1",
								},
							},
						},
						{
							Name: "listener-2",
							Certificates: []structs.ResourceReference{
								{
									Kind: structs.InlineCertificate,
									Name: "cert-2",
								},
							},
						},
					},
				},
			},
		},
		"all-listeners-invalid-certificates": {
			requests: []controller.Request{{
				Kind: structs.APIGateway,
				Name: "gateway",
				Meta: acl.DefaultEnterpriseMeta(),
			}},
			initialEntries: []structs.ConfigEntry{
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.APIGatewayListener{
						{
							Name: "listener-1",
							Port: 80,
							TLS: structs.APIGatewayTLSConfiguration{
								Certificates: []structs.ResourceReference{{
									Kind: structs.InlineCertificate,
									Name: "missing certificate",
								}},
							},
						},
						{
							Name: "listener-2",
							Port: 80,
							TLS: structs.APIGatewayTLSConfiguration{
								Certificates: []structs.ResourceReference{{
									Kind: structs.InlineCertificate,
									Name: "another missing certificate",
								}},
							},
						},
					},
				},
			},
			finalEntries: []structs.ConfigEntry{
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.APIGatewayListener{
						{
							Name: "listener-1",
							Port: 80,
							TLS: structs.APIGatewayTLSConfiguration{
								Certificates: []structs.ResourceReference{{
									Kind: structs.InlineCertificate,
									Name: "missing certificate",
								}},
							},
						},
						{
							Name: "listener-2",
							Port: 80,
							TLS: structs.APIGatewayTLSConfiguration{
								Certificates: []structs.ResourceReference{{
									Kind: structs.InlineCertificate,
									Name: "another missing certificate",
								}},
							},
						},
					},
					Status: structs.Status{
						Conditions: []structs.Condition{
							invalidCertificate(structs.ResourceReference{
								Kind:        structs.APIGateway,
								Name:        "gateway",
								SectionName: "listener-1",
							}, errors.New("certificate \"missing certificate\" not found")),
							invalidCertificate(structs.ResourceReference{
								Kind:        structs.APIGateway,
								Name:        "gateway",
								SectionName: "listener-2",
							}, errors.New("certificate \"another missing certificate\" not found")),
							invalidCertificates(),
							gatewayListenerNoConflicts(structs.ResourceReference{
								Kind:        structs.APIGateway,
								Name:        "gateway",
								SectionName: "listener-1",
							}),
							gatewayListenerNoConflicts(structs.ResourceReference{
								Kind:        structs.APIGateway,
								Name:        "gateway",
								SectionName: "listener-2",
							}),
						},
					},
				},
				&structs.BoundAPIGatewayConfigEntry{
					Kind:           structs.BoundAPIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.BoundAPIGatewayListener{
						{Name: "listener-1"},
						{Name: "listener-2"},
					},
				},
			},
		},
		"mixed-valid-and-invalid-certificate-refs-for-listeners": {
			requests: []controller.Request{{
				Kind: structs.APIGateway,
				Name: "gateway",
				Meta: acl.DefaultEnterpriseMeta(),
			}},
			initialEntries: []structs.ConfigEntry{
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.APIGatewayListener{
						{
							Name: "invalid-listener",
							Port: 80,
							TLS: structs.APIGatewayTLSConfiguration{
								Certificates: []structs.ResourceReference{{
									Kind: structs.InlineCertificate,
									Name: "missing certificate",
								}},
							},
						},
						{
							Name: "valid-listener",
							Port: 80,
							TLS: structs.APIGatewayTLSConfiguration{
								Certificates: []structs.ResourceReference{{
									Kind: structs.InlineCertificate,
									Name: "certificate",
								}},
							},
						},
					},
				},
				&structs.InlineCertificateConfigEntry{
					Kind:           structs.InlineCertificate,
					Name:           "certificate",
					EnterpriseMeta: *defaultMeta,
				},
			},
			finalEntries: []structs.ConfigEntry{
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.APIGatewayListener{
						{
							Name: "invalid-listener",
							Port: 80,
							TLS: structs.APIGatewayTLSConfiguration{
								Certificates: []structs.ResourceReference{{
									Kind: structs.InlineCertificate,
									Name: "missing certificate",
								}},
							},
						},
						{
							Name: "valid-listener",
							Port: 80,
							TLS: structs.APIGatewayTLSConfiguration{
								Certificates: []structs.ResourceReference{{
									Kind: structs.InlineCertificate,
									Name: "certificate",
								}},
							},
						},
					},
					Status: structs.Status{
						Conditions: []structs.Condition{
							invalidCertificate(structs.ResourceReference{
								Kind:        structs.APIGateway,
								Name:        "gateway",
								SectionName: "invalid-listener",
							}, errors.New("certificate \"missing certificate\" not found")),
							validCertificate(structs.ResourceReference{
								Kind:        structs.APIGateway,
								Name:        "gateway",
								SectionName: "valid-listener",
							}),

							invalidCertificates(),
							gatewayListenerNoConflicts(structs.ResourceReference{
								Kind:        structs.APIGateway,
								Name:        "gateway",
								SectionName: "valid-listener",
							}),
							gatewayListenerNoConflicts(structs.ResourceReference{
								Kind:        structs.APIGateway,
								Name:        "gateway",
								SectionName: "invalid-listener",
							}),
						},
					},
				},
				&structs.BoundAPIGatewayConfigEntry{
					Kind:           structs.BoundAPIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.BoundAPIGatewayListener{
						{
							Name: "valid-listener",
							Certificates: []structs.ResourceReference{
								{
									Kind: structs.InlineCertificate,
									Name: "certificate",
								},
							},
						},
						{
							Name: "invalid-listener",
						},
					},
				},
			},
		},
		"updated-gateway-certificates": {
			requests: []controller.Request{{
				Kind: structs.APIGateway,
				Name: "gateway",
				Meta: acl.DefaultEnterpriseMeta(),
			}},
			initialEntries: []structs.ConfigEntry{
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.APIGatewayListener{{
						Name: "http-listener",
						Port: 80,
						TLS: structs.APIGatewayTLSConfiguration{
							Certificates: []structs.ResourceReference{{
								Kind: structs.InlineCertificate,
								Name: "certificate",
							}},
						},
					}},
					Status: structs.Status{
						Conditions: []structs.Condition{
							invalidCertificate(structs.ResourceReference{
								Kind: structs.InlineCertificate,
								Name: "certificate",
							}, errors.New("certificate not found")),
							invalidCertificates(),
							gatewayListenerNoConflicts(structs.ResourceReference{
								Kind:        structs.APIGateway,
								Name:        "gateway",
								SectionName: "http-listener",
							}),
						},
					},
				},
				&structs.InlineCertificateConfigEntry{
					Kind:           structs.InlineCertificate,
					Name:           "certificate",
					EnterpriseMeta: *defaultMeta,
				},
			},
			finalEntries: []structs.ConfigEntry{
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.APIGatewayListener{{
						Name: "http-listener",
						Port: 80,
						TLS: structs.APIGatewayTLSConfiguration{
							Certificates: []structs.ResourceReference{{
								Kind: structs.InlineCertificate,
								Name: "certificate",
							}},
						},
					}},
					Status: structs.Status{
						Conditions: []structs.Condition{
							gatewayAccepted(),
							gatewayListenerNoConflicts(structs.ResourceReference{
								Kind:        structs.APIGateway,
								Name:        "gateway",
								SectionName: "http-listener",
							}),
							validCertificate(structs.ResourceReference{
								Kind:        structs.APIGateway,
								Name:        "gateway",
								SectionName: "http-listener",
							}),
						},
					},
				},
				&structs.BoundAPIGatewayConfigEntry{
					Kind:           structs.BoundAPIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.BoundAPIGatewayListener{{
						Name: "http-listener",
						Certificates: []structs.ResourceReference{{
							Kind:           structs.InlineCertificate,
							Name:           "certificate",
							EnterpriseMeta: *defaultMeta,
						}},
					}},
				},
			},
		},
		"trigger-gateway-certificates": {
			requests: []controller.Request{{
				Kind: structs.InlineCertificate,
				Name: "certificate",
				Meta: defaultMeta,
			}},
			initialEntries: []structs.ConfigEntry{
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.APIGatewayListener{{
						Name: "http-listener",
						Port: 80,
						TLS: structs.APIGatewayTLSConfiguration{
							Certificates: []structs.ResourceReference{{
								Kind: structs.InlineCertificate,
								Name: "certificate",
							}},
						},
					}},
				},
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "gateway-two",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.APIGatewayListener{{
						Name: "http-listener",
						Port: 80,
						TLS: structs.APIGatewayTLSConfiguration{
							Certificates: []structs.ResourceReference{{
								Kind: structs.InlineCertificate,
								Name: "certificate",
							}},
						},
					}},
				},
			},
			finalEntries: []structs.ConfigEntry{
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "gateway",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.APIGatewayListener{{
						Name: "http-listener",
						Port: 80,
						TLS: structs.APIGatewayTLSConfiguration{
							Certificates: []structs.ResourceReference{{
								Kind: structs.InlineCertificate,
								Name: "certificate",
							}},
						},
					}},
				},
				&structs.APIGatewayConfigEntry{
					Kind:           structs.APIGateway,
					Name:           "gateway-two",
					EnterpriseMeta: *defaultMeta,
					Listeners: []structs.APIGatewayListener{{
						Name: "http-listener",
						Port: 80,
						TLS: structs.APIGatewayTLSConfiguration{
							Certificates: []structs.ResourceReference{{
								Kind: structs.InlineCertificate,
								Name: "certificate",
							}},
						},
					}},
				},
			},
			enqueued: []controller.Request{{
				Kind: structs.APIGateway,
				Name: "gateway",
				Meta: defaultMeta,
			}, {
				Kind: structs.APIGateway,
				Name: "gateway-two",
				Meta: defaultMeta,
			}},
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			publisher := stream.NewEventPublisher(1 * time.Millisecond)
			go publisher.Run(ctx)

			fsm := fsm.NewFromDeps(fsm.Deps{
				Logger: hclog.New(nil),
				NewStateStore: func() *state.Store {
					return state.NewStateStoreWithEventPublisher(nil, publisher)
				},
				Publisher: publisher,
			})

			var index uint64
			updater := &Updater{
				UpdateWithStatus: func(entry structs.ControlledConfigEntry) error {
					index++
					store := fsm.State()
					_, err := store.EnsureConfigEntryWithStatusCAS(index, entry.GetRaftIndex().ModifyIndex, entry)
					return err
				},
				Update: func(entry structs.ConfigEntry) error {
					index++
					store := fsm.State()
					_, err := store.EnsureConfigEntryCAS(index, entry.GetRaftIndex().ModifyIndex, entry)
					return err
				},
				Delete: func(entry structs.ConfigEntry) error {
					index++
					store := fsm.State()
					_, err := store.DeleteConfigEntryCAS(index, entry.GetRaftIndex().ModifyIndex, entry)
					return err
				},
			}

			for _, entry := range tc.initialEntries {
				if controlled, ok := entry.(structs.ControlledConfigEntry); ok {
					require.NoError(t, updater.UpdateWithStatus(controlled))
					continue
				}
				err := updater.Update(entry)
				require.NoError(t, err)
			}

			controller := &noopController{
				triggers: make(map[controller.Request]struct{}),
			}
			reconciler := apiGatewayReconciler{
				fsm:        fsm,
				logger:     hclog.Default(),
				updater:    updater,
				controller: controller,
			}

			for _, req := range tc.requests {
				require.NoError(t, reconciler.Reconcile(ctx, req))
			}

			_, entries, err := fsm.State().ConfigEntries(nil, acl.WildcardEnterpriseMeta())
			require.NoError(t, err)
			for _, entry := range entries {
				controlled, ok := entry.(structs.ControlledConfigEntry)
				if !ok {
					continue
				}

				found := false
				for _, expected := range tc.finalEntries {
					if controlled.GetKind() == expected.GetKind() &&
						controlled.GetName() == expected.GetName() &&
						controlled.GetEnterpriseMeta().IsSame(expected.GetEnterpriseMeta()) {
						expectedStatus := expected.(structs.ControlledConfigEntry).GetStatus()
						acualStatus := controlled.GetStatus()
						statusEqual := acualStatus.SameConditions(expectedStatus)
						ppActual, err := json.MarshalIndent(acualStatus, "", "  ")
						require.NoError(t, err)
						ppExpected, err := json.MarshalIndent(expectedStatus, "", "  ")
						require.NoError(t, err)
						require.True(t, statusEqual, fmt.Sprintf("statuses are unequal (actual != expected): %+v != %+v", string(ppActual), string(ppExpected)))
						if bound, ok := controlled.(*structs.BoundAPIGatewayConfigEntry); ok {
							ppActual, err := json.MarshalIndent(bound, "", "  ")
							require.NoError(t, err)
							ppExpected, err := json.MarshalIndent(expected, "", "  ")
							require.NoError(t, err)
							require.True(t, bound.IsSame(expected.(*structs.BoundAPIGatewayConfigEntry)), fmt.Sprintf("api bound states do not match: %+v != %+v", string(ppActual), string(ppExpected)))
						}
						found = true
						break
					}
				}
				require.True(t, found, fmt.Sprintf("unexpected entry found: %+v", entry))
			}
			for _, expected := range tc.finalEntries {
				found := false
				for _, entry := range entries {
					if entry.GetKind() == expected.GetKind() &&
						entry.GetName() == expected.GetName() &&
						entry.GetEnterpriseMeta().IsSame(expected.GetEnterpriseMeta()) {
						found = true
						break
					}
				}
				require.True(t, found, fmt.Sprintf("expected entry not found: %+v", expected))
			}
			for _, queued := range tc.enqueued {
				found := false
				for _, entry := range controller.enqueued {
					if entry.Kind == queued.Kind &&
						entry.Name == queued.Name &&
						entry.Meta.IsSame(queued.Meta) {
						found = true
						break
					}
				}
				require.True(t, found, fmt.Sprintf("expected queued entry not found: %+v", queued))
			}
		})
	}
}

func TestNewAPIGatewayController(t *testing.T) {
	t.Parallel()

	publisher := stream.NewEventPublisher(1 * time.Millisecond)
	fsm := fsm.NewFromDeps(fsm.Deps{
		Logger: hclog.New(nil),
		NewStateStore: func() *state.Store {
			return state.NewStateStoreWithEventPublisher(nil, publisher)
		},
		Publisher: publisher,
	})

	updater := &Updater{
		UpdateWithStatus: func(entry structs.ControlledConfigEntry) error { return nil },
		Update:           func(entry structs.ConfigEntry) error { return nil },
		Delete:           func(entry structs.ConfigEntry) error { return nil },
	}

	require.NotNil(t, NewAPIGatewayController(fsm, publisher, updater, hclog.Default()))
}

type noopController struct {
	triggers map[controller.Request]struct{}
	enqueued []controller.Request
}

func (n *noopController) Run(ctx context.Context) error { return nil }
func (n *noopController) Subscribe(request *stream.SubscribeRequest, transformers ...controller.Transformer) controller.Controller {
	return n
}
func (n *noopController) WithBackoff(base, max time.Duration) controller.Controller { return n }
func (n *noopController) WithLogger(logger hclog.Logger) controller.Controller      { return n }
func (n *noopController) WithWorkers(i int) controller.Controller                   { return n }
func (n *noopController) WithQueueFactory(fn func(ctx context.Context, baseBackoff time.Duration, maxBackoff time.Duration) controller.WorkQueue) controller.Controller {
	return n
}

func (n *noopController) AddTrigger(request controller.Request, trigger func(ctx context.Context) error) {
	n.triggers[request] = struct{}{}
}

func (n *noopController) RemoveTrigger(request controller.Request) {
	delete(n.triggers, request)
}

func (n *noopController) Enqueue(requests ...controller.Request) {
	n.enqueued = append(n.enqueued, requests...)
}
