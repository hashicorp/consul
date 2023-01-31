package gateways

import (
	"fmt"
	"testing"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/require"
)

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
				}: fmt.Errorf("failed to bind route TCP Route to gateway Gateway: no valid listener has name 'Non-existent Listener' and uses tcp protocol"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			actualBoundAPIGateways, referenceErrors := BindRoutesToGateways(tc.gateways, tc.routes...)

			require.Equal(t, tc.expectedBoundAPIGateways, actualBoundAPIGateways)
			require.Equal(t, tc.expectedReferenceErrors, referenceErrors)
		})
	}
}
