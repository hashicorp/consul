package gateways

import (
	"testing"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/require"
)

func TestBindGateways(t *testing.T) {
	type testCase struct {
		gateways                 []*structs.BoundAPIGatewayConfigEntry
		route                    structs.BoundRoute
		expectedBoundAPIGateways []*structs.BoundAPIGatewayConfigEntry
		expectedReferenceErrors  map[structs.ResourceReference]error
		expectedError            error
	}

	cases := map[string]testCase{
		"TCP Route binds to gateway": {
			gateways: []*structs.BoundAPIGatewayConfigEntry{
				makeGateway("Test Bound API Gateway", []structs.BoundAPIGatewayListener{
					makeListener("Test Listener", []structs.ResourceReference{}),
				}),
			},
			route: makeRoute(structs.TCPRoute, "Test TCP Route", []structs.ResourceReference{
				makeRef(structs.APIGateway, "Test Bound API Gateway", "Test Listener"),
			}),
			expectedBoundAPIGateways: []*structs.BoundAPIGatewayConfigEntry{
				makeGateway("Test Bound API Gateway", []structs.BoundAPIGatewayListener{
					makeListener("Test Listener", []structs.ResourceReference{
						makeRef(structs.TCPRoute, "Test TCP Route", ""),
					}),
				}),
			},
			expectedReferenceErrors: map[structs.ResourceReference]error{},
			expectedError:           nil,
		},
		"TCP Route unbinds from gateway": {
			gateways: []*structs.BoundAPIGatewayConfigEntry{
				makeGateway("Test Bound API Gateway", []structs.BoundAPIGatewayListener{
					makeListener("Test Listener", []structs.ResourceReference{
						makeRef(structs.TCPRoute, "Test TCP Route", ""),
					}),
				}),
				makeGateway("Other Test Bound API Gateway", []structs.BoundAPIGatewayListener{
					makeListener("Test Listener", []structs.ResourceReference{
						makeRef(structs.TCPRoute, "Test TCP Route", ""),
					}),
				}),
			},
			route: makeRoute(structs.TCPRoute, "Test TCP Route", []structs.ResourceReference{
				makeRef(structs.APIGateway, "Test Bound API Gateway", "Test Listener"),
			}),
			expectedBoundAPIGateways: []*structs.BoundAPIGatewayConfigEntry{
				makeGateway("Test Bound API Gateway", []structs.BoundAPIGatewayListener{
					makeListener("Test Listener", []structs.ResourceReference{
						makeRef(structs.TCPRoute, "Test TCP Route", ""),
					}),
				}),
				makeGateway("Other Test Bound API Gateway", []structs.BoundAPIGatewayListener{
					makeListener("Test Listener", []structs.ResourceReference{}),
				}),
			},
			expectedReferenceErrors: map[structs.ResourceReference]error{},
			expectedError:           nil,
		},
		"TCP Route binds to multiple gateways": {
			gateways: []*structs.BoundAPIGatewayConfigEntry{
				makeGateway("Test Bound API Gateway", []structs.BoundAPIGatewayListener{
					makeListener("Test Listener", []structs.ResourceReference{}),
				}),
				makeGateway("Other Test Bound API Gateway", []structs.BoundAPIGatewayListener{
					makeListener("Test Listener", []structs.ResourceReference{}),
				}),
			},
			route: makeRoute(structs.TCPRoute, "Test TCP Route", []structs.ResourceReference{
				makeRef(structs.APIGateway, "Test Bound API Gateway", "Test Listener"),
				makeRef(structs.APIGateway, "Other Test Bound API Gateway", "Test Listener"),
			}),
			expectedBoundAPIGateways: []*structs.BoundAPIGatewayConfigEntry{
				makeGateway("Test Bound API Gateway", []structs.BoundAPIGatewayListener{
					makeListener("Test Listener", []structs.ResourceReference{
						makeRef(structs.TCPRoute, "Test TCP Route", ""),
					}),
				}),
				makeGateway("Other Test Bound API Gateway", []structs.BoundAPIGatewayListener{
					makeListener("Test Listener", []structs.ResourceReference{
						makeRef(structs.TCPRoute, "Test TCP Route", ""),
					}),
				}),
			},
			expectedReferenceErrors: map[structs.ResourceReference]error{},
			expectedError:           nil,
		},
		"TCP Route binds to gateway with multiple listeners": {
			gateways: []*structs.BoundAPIGatewayConfigEntry{
				makeGateway("Test Bound API Gateway", []structs.BoundAPIGatewayListener{
					makeListener("Test Listener", []structs.ResourceReference{}),
					makeListener("Other Test Listener", []structs.ResourceReference{}),
				}),
			},
			route: makeRoute(structs.TCPRoute, "Test TCP Route", []structs.ResourceReference{
				makeRef(structs.APIGateway, "Test Bound API Gateway", "Test Listener"),
			}),
			expectedBoundAPIGateways: []*structs.BoundAPIGatewayConfigEntry{
				makeGateway("Test Bound API Gateway", []structs.BoundAPIGatewayListener{
					makeListener("Test Listener", []structs.ResourceReference{
						makeRef(structs.TCPRoute, "Test TCP Route", ""),
					}),
					makeListener("Other Test Listener", []structs.ResourceReference{}),
				}),
			},
			expectedReferenceErrors: map[structs.ResourceReference]error{},
			expectedError:           nil,
		},
		"TCP Route binds to all listeners on a gateway": {
			gateways: []*structs.BoundAPIGatewayConfigEntry{
				makeGateway("Test Bound API Gateway", []structs.BoundAPIGatewayListener{
					makeListener("Test Listener", []structs.ResourceReference{}),
					makeListener("Other Test Listener", []structs.ResourceReference{}),
				}),
			},
			route: makeRoute(structs.TCPRoute, "Test TCP Route", []structs.ResourceReference{
				makeRef(structs.APIGateway, "Test Bound API Gateway", ""),
			}),
			expectedBoundAPIGateways: []*structs.BoundAPIGatewayConfigEntry{
				makeGateway("Test Bound API Gateway", []structs.BoundAPIGatewayListener{
					makeListener("Test Listener", []structs.ResourceReference{
						makeRef(structs.TCPRoute, "Test TCP Route", ""),
					}),
					makeListener("Other Test Listener", []structs.ResourceReference{
						makeRef(structs.TCPRoute, "Test TCP Route", ""),
					}),
				}),
			},
			expectedReferenceErrors: map[structs.ResourceReference]error{},
			expectedError:           nil,
		},
		"TCP Route binds to gateway with multiple listeners, one of which is already bound": {
			gateways: []*structs.BoundAPIGatewayConfigEntry{
				makeGateway("Test Bound API Gateway", []structs.BoundAPIGatewayListener{
					makeListener("Test Listener", []structs.ResourceReference{}),
					makeListener("Other Test Listener", []structs.ResourceReference{
						makeRef(structs.TCPRoute, "Test TCP Route", ""),
					}),
				}),
			},
			route: makeRoute(structs.TCPRoute, "Test TCP Route", []structs.ResourceReference{
				makeRef(structs.APIGateway, "Test Bound API Gateway", "Test Listener"),
			}),
			expectedBoundAPIGateways: []*structs.BoundAPIGatewayConfigEntry{
				makeGateway("Test Bound API Gateway", []structs.BoundAPIGatewayListener{
					makeListener("Test Listener", []structs.ResourceReference{
						makeRef(structs.TCPRoute, "Test TCP Route", ""),
					}),
					makeListener("Other Test Listener", []structs.ResourceReference{
						makeRef(structs.TCPRoute, "Test TCP Route", ""),
					}),
				}),
			},
			expectedReferenceErrors: map[structs.ResourceReference]error{},
			expectedError:           nil,
		},
		"TCP Route binds to a listener on multiple gateways": {
			gateways: []*structs.BoundAPIGatewayConfigEntry{
				makeGateway("Test Bound API Gateway", []structs.BoundAPIGatewayListener{
					makeListener("Test Listener", []structs.ResourceReference{}),
					makeListener("Other Test Listener", []structs.ResourceReference{}),
				}),
				makeGateway("Other Test Bound API Gateway", []structs.BoundAPIGatewayListener{
					makeListener("Test Listener", []structs.ResourceReference{}),
					makeListener("Other Test Listener", []structs.ResourceReference{}),
				}),
			},
			route: makeRoute(structs.TCPRoute, "Test TCP Route", []structs.ResourceReference{
				makeRef(structs.APIGateway, "Test Bound API Gateway", "Test Listener"),
				makeRef(structs.APIGateway, "Other Test Bound API Gateway", "Test Listener"),
			}),
			expectedBoundAPIGateways: []*structs.BoundAPIGatewayConfigEntry{
				makeGateway("Test Bound API Gateway", []structs.BoundAPIGatewayListener{
					makeListener("Test Listener", []structs.ResourceReference{
						makeRef(structs.TCPRoute, "Test TCP Route", ""),
					}),
					makeListener("Other Test Listener", []structs.ResourceReference{}),
				}),
				makeGateway("Other Test Bound API Gateway", []structs.BoundAPIGatewayListener{
					makeListener("Test Listener", []structs.ResourceReference{
						makeRef(structs.TCPRoute, "Test TCP Route", ""),
					}),
					makeListener("Other Test Listener", []structs.ResourceReference{}),
				}),
			},
			expectedReferenceErrors: map[structs.ResourceReference]error{},
			expectedError:           nil,
		},
		"TCP Route swaps from one listener to another on a gateway": {
			gateways: []*structs.BoundAPIGatewayConfigEntry{
				makeGateway("Test Bound API Gateway", []structs.BoundAPIGatewayListener{
					makeListener("Test Listener", []structs.ResourceReference{
						makeRef(structs.TCPRoute, "Test TCP Route", ""),
					}),
					makeListener("Other Test Listener", []structs.ResourceReference{}),
				}),
			},
			route: makeRoute(structs.TCPRoute, "Test TCP Route", []structs.ResourceReference{
				makeRef(structs.APIGateway, "Test Bound API Gateway", "Other Test Listener"),
			}),
			expectedBoundAPIGateways: []*structs.BoundAPIGatewayConfigEntry{
				makeGateway("Test Bound API Gateway", []structs.BoundAPIGatewayListener{
					makeListener("Test Listener", []structs.ResourceReference{}),
					makeListener("Other Test Listener", []structs.ResourceReference{
						makeRef(structs.TCPRoute, "Test TCP Route", ""),
					}),
				}),
			},
			expectedReferenceErrors: map[structs.ResourceReference]error{},
			expectedError:           nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			actualBoundAPIGateways, actualReferenceErrors, actualError := BindRouteToGateways(tc.gateways, tc.route)

			require.Equal(t, tc.expectedBoundAPIGateways, actualBoundAPIGateways)
			require.Equal(t, tc.expectedReferenceErrors, actualReferenceErrors, "ReferenceErrors should match")
			require.Equal(t, tc.expectedError, actualError, "Error should match")
		})
	}
}

func makeRef(kind, name, sectionName string) structs.ResourceReference {
	return structs.ResourceReference{
		Kind:        kind,
		Name:        name,
		SectionName: sectionName,
	}
}

func makeRoute(kind, name string, parents []structs.ResourceReference) structs.BoundRoute {
	switch kind {
	case structs.TCPRoute:
		return &structs.TCPRouteConfigEntry{
			Kind:    structs.TCPRoute,
			Name:    name,
			Parents: parents,
		}
	default:
		panic("unknown route kind")
	}
}

func makeListener(name string, routes []structs.ResourceReference) structs.BoundAPIGatewayListener {
	return structs.BoundAPIGatewayListener{
		Name:   name,
		Routes: routes,
	}
}

func makeGateway(name string, listeners []structs.BoundAPIGatewayListener) *structs.BoundAPIGatewayConfigEntry {
	return &structs.BoundAPIGatewayConfigEntry{
		Kind:      structs.BoundAPIGateway,
		Name:      name,
		Listeners: listeners,
	}
}
