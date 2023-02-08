package gateways

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/controller"
	"github.com/hashicorp/consul/agent/consul/fsm"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
)

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
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "True",
							Reason:  "Accepted",
							Message: "gateway is valid",
						}},
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
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "False",
							Reason:  "NoUpstreamServicesTargeted",
							Message: "route must target at least one upstream service",
						}},
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
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "False",
							Reason:  "NoUpstreamServicesTargeted",
							Message: "route must target at least one upstream service",
						}},
					},
				},
			},
		},
		"tcp-route-no-gateways-invalid-targets": {
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
			},
			finalEntries: []structs.ConfigEntry{
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "False",
							Reason:  "InvalidDiscoveryChain",
							Message: "service does not exist",
						}},
					},
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
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "False",
							Reason:  "InvalidDiscoveryChain",
							Message: "route protocol does not match targeted service protocol",
						}},
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
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "True",
							Reason:  "Accepted",
							Message: "route is valid",
						}, {
							Type:   "Bound",
							Status: "False",
							Reason: "FailedToBind",
							Resource: &structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
							},
							Message: "invalid reference to missing parent",
						}},
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
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "True",
							Reason:  "Accepted",
							Message: "route is valid",
						}, {
							Type:   "Bound",
							Status: "False",
							Reason: "FailedToBind",
							Resource: &structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
							},
							Message: "invalid reference to missing parent",
						}},
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
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "True",
							Reason:  "Accepted",
							Message: "route is valid",
						}, {
							Type:   "Bound",
							Status: "False",
							Reason: "FailedToBind",
							Resource: &structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
							},
							Message: "route cannot bind because gateway has no listeners",
						}},
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
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "True",
							Reason:  "Accepted",
							Message: "route is valid",
						}, {
							Type:   "Bound",
							Status: "False",
							Reason: "FailedToBind",
							Resource: &structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
							},
							Message: "invalid reference to missing parent",
						}},
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
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "True",
							Reason:  "Accepted",
							Message: "gateway is valid",
						}, {
							Type:   "Conflicted",
							Status: "False",
							Reason: "NoConflict",
							Resource: &structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
								SectionName:    "listener",
							},
							Message: "listener has no route conflicts",
						}},
					},
				},
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "True",
							Reason:  "Accepted",
							Message: "route is valid",
						}, {
							Type:   "Bound",
							Status: "True",
							Reason: "Bound",
							Resource: &structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
							},
							Message: "successfully bound route",
						}},
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
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "True",
							Reason:  "Accepted",
							Message: "gateway is valid",
						}, {
							Type:   "Conflicted",
							Status: "False",
							Reason: "NoConflict",
							Resource: &structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
								SectionName:    "listener",
							},
							Message: "listener has no route conflicts",
						}},
					},
				},
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "True",
							Reason:  "Accepted",
							Message: "route is valid",
						}, {
							Type:   "Bound",
							Status: "False",
							Reason: "FailedToBind",
							Resource: &structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
							},
							Message: "failed to bind route tcp-route to gateway gateway: no valid listener has name '' and uses tcp protocol",
						}},
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
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "True",
							Reason:  "Accepted",
							Message: "gateway is valid",
						}, {
							Type:   "Conflicted",
							Status: "True",
							Reason: "RouteConflict",
							Resource: &structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
								SectionName:    "listener",
							},
							Message: "TCP-based listeners currently only support binding a single route",
						}},
					},
				},
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route-one",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "True",
							Reason:  "Accepted",
							Message: "route is valid",
						}, {
							Type:   "Bound",
							Status: "True",
							Reason: "Bound",
							Resource: &structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
							},
							Message: "successfully bound route",
						}},
					},
				},
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route-two",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "True",
							Reason:  "Accepted",
							Message: "route is valid",
						}, {
							Type:   "Bound",
							Status: "True",
							Reason: "Bound",
							Resource: &structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
							},
							Message: "successfully bound route",
						}},
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
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "True",
							Reason:  "Accepted",
							Message: "gateway is valid",
						}, {
							Type:   "Conflicted",
							Status: "False",
							Reason: "NoConflict",
							Resource: &structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
								SectionName:    "listener",
							},
							Message: "listener has no route conflicts",
						}},
					},
				},
				&structs.HTTPRouteConfigEntry{
					Kind:           structs.HTTPRoute,
					Name:           "http-route-one",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "True",
							Reason:  "Accepted",
							Message: "route is valid",
						}, {
							Type:   "Bound",
							Status: "True",
							Reason: "Bound",
							Resource: &structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
							},
							Message: "successfully bound route",
						}},
					},
				},
				&structs.HTTPRouteConfigEntry{
					Kind:           structs.HTTPRoute,
					Name:           "http-route-two",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "True",
							Reason:  "Accepted",
							Message: "route is valid",
						}, {
							Type:   "Bound",
							Status: "True",
							Reason: "Bound",
							Resource: &structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
							},
							Message: "successfully bound route",
						}},
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
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "True",
							Reason:  "Accepted",
							Message: "gateway is valid",
						}, {
							Type:   "Conflicted",
							Status: "False",
							Reason: "NoConflict",
							Resource: &structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
								SectionName:    "listener",
							},
							Message: "listener has no route conflicts",
						}},
					},
				},
				&structs.HTTPRouteConfigEntry{
					Kind:           structs.HTTPRoute,
					Name:           "http-route",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "True",
							Reason:  "Accepted",
							Message: "route is valid",
						}, {
							Type:   "Bound",
							Status: "True",
							Reason: "Bound",
							Resource: &structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
							},
							Message: "successfully bound route",
						}},
					},
				},
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "True",
							Reason:  "Accepted",
							Message: "route is valid",
						}, {
							Type:   "Bound",
							Status: "False",
							Reason: "FailedToBind",
							Resource: &structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
							},
							Message: "failed to bind route tcp-route to gateway gateway: no valid listener has name '' and uses tcp protocol",
						}},
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
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "True",
							Reason:  "Accepted",
							Message: "gateway is valid",
						}, {
							Type:   "Conflicted",
							Status: "False",
							Reason: "NoConflict",
							Resource: &structs.ResourceReference{
								Kind:        structs.APIGateway,
								Name:        "gateway",
								SectionName: "http-listener",
							},
							Message: "listener has no route conflicts",
						}, {
							Type:   "Conflicted",
							Status: "False",
							Reason: "NoConflict",
							Resource: &structs.ResourceReference{
								Kind:        structs.APIGateway,
								Name:        "gateway",
								SectionName: "tcp-listener",
							},
							Message: "listener has no route conflicts",
						}},
					},
				},
				&structs.HTTPRouteConfigEntry{
					Kind:           structs.HTTPRoute,
					Name:           "http-route",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "True",
							Reason:  "Accepted",
							Message: "route is valid",
						}, {
							Type:   "Bound",
							Status: "True",
							Reason: "Bound",
							Resource: &structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
							},
							Message: "successfully bound route",
						}},
					},
				},
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "True",
							Reason:  "Accepted",
							Message: "route is valid",
						}, {
							Type:   "Bound",
							Status: "True",
							Reason: "Bound",
							Resource: &structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
							},
							Message: "successfully bound route",
						}},
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
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "True",
							Reason:  "Accepted",
							Message: "route is valid",
						}, {
							Type:   "Bound",
							Status: "True",
							Reason: "Bound",
							Resource: &structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
							},
							Message: "successfully bound route",
						}},
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
				},
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "True",
							Reason:  "Accepted",
							Message: "route is valid",
						}},
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
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "True",
							Reason:  "Accepted",
							Message: "route is valid",
						}, {
							Type:   "Bound",
							Status: "True",
							Reason: "Bound",
							Resource: &structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
							},
							Message: "successfully bound route",
						}},
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
				},
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "False",
							Reason:  "NoUpstreamServicesTargeted",
							Message: "route must target at least one upstream service",
						}},
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
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "True",
							Reason:  "Accepted",
							Message: "route is valid",
						}, {
							Type:   "Bound",
							Status: "True",
							Reason: "Bound",
							Resource: &structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
							},
							Message: "successfully bound route",
						}},
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
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "True",
							Reason:  "Accepted",
							Message: "route is valid",
						}, {
							Type:   "Bound",
							Status: "False",
							Reason: "NotBound",
							Resource: &structs.ResourceReference{
								Kind: structs.APIGateway,
								Name: "gateway",
							},
						}},
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
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "True",
							Reason:  "Accepted",
							Message: "gateway is valid",
						}, {
							Type:   "Conflicted",
							Status: "False",
							Reason: "NoConflict",
							Resource: &structs.ResourceReference{
								Kind:        structs.APIGateway,
								Name:        "gateway",
								SectionName: "tcp-listener",
							},
							Message: "listener has no route conflicts",
						}},
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
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "True",
							Reason:  "Accepted",
							Message: "gateway is valid",
						}, {
							Type:   "Conflicted",
							Status: "False",
							Reason: "NoConflict",
							Resource: &structs.ResourceReference{
								Kind:        structs.APIGateway,
								Name:        "gateway",
								SectionName: "http-listener",
							},
							Message: "listener has no route conflicts",
						}},
					},
				},
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "True",
							Reason:  "Accepted",
							Message: "route is valid",
						}, {
							Type:    "Bound",
							Status:  "False",
							Reason:  "FailedToBind",
							Message: "failed to bind route tcp-route to gateway gateway: no valid listener has name '' and uses tcp protocol",
							Resource: &structs.ResourceReference{
								Kind: structs.APIGateway,
								Name: "gateway",
							},
						}},
					},
				},
				&structs.HTTPRouteConfigEntry{
					Kind:           structs.HTTPRoute,
					Name:           "http-route",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "True",
							Reason:  "Accepted",
							Message: "route is valid",
						}, {
							Type:    "Bound",
							Status:  "True",
							Reason:  "Bound",
							Message: "successfully bound route",
							Resource: &structs.ResourceReference{
								Kind: structs.APIGateway,
								Name: "gateway",
							},
						}},
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
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "True",
							Reason:  "Accepted",
							Message: "route is valid",
						}, {
							Type:   "Bound",
							Status: "True",
							Reason: "Bound",
							Resource: &structs.ResourceReference{
								Kind:           structs.APIGateway,
								Name:           "gateway",
								EnterpriseMeta: *defaultMeta,
							},
							Message: "successfully bound route",
						}},
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
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "True",
							Reason:  "Accepted",
							Message: "route is valid",
						}, {
							Type:    "Bound",
							Status:  "False",
							Reason:  "GatewayNotFound",
							Message: "gateway was not found",
							Resource: &structs.ResourceReference{
								Kind: structs.APIGateway,
								Name: "gateway",
							},
						}},
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
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "True",
							Reason:  "Accepted",
							Message: "gateway is valid",
						}, {
							Type:   "Conflicted",
							Status: "False",
							Reason: "NoConflict",
							Resource: &structs.ResourceReference{
								Kind:        structs.APIGateway,
								Name:        "gateway",
								SectionName: "tcp-listener",
							},
							Message: "listener has no route conflicts",
						}},
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
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "True",
							Reason:  "Accepted",
							Message: "gateway is valid",
						}, {
							Type:   "Conflicted",
							Status: "False",
							Reason: "NoConflict",
							Resource: &structs.ResourceReference{
								Kind:        structs.APIGateway,
								Name:        "gateway",
								SectionName: "tcp-listener",
							},
							Message: "listener has no route conflicts",
						}},
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
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "True",
							Reason:  "Accepted",
							Message: "route is valid",
						}, {
							Type:   "Bound",
							Status: "True",
							Reason: "Bound",
							Resource: &structs.ResourceReference{
								Kind: structs.APIGateway,
								Name: "gateway",
							},
						}},
					},
				},
			},
			finalEntries: []structs.ConfigEntry{
				&structs.TCPRouteConfigEntry{
					Kind:           structs.TCPRoute,
					Name:           "tcp-route",
					EnterpriseMeta: *defaultMeta,
					Status: structs.Status{
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "True",
							Reason:  "Accepted",
							Message: "route is valid",
						}, {
							Type:    "Bound",
							Status:  "False",
							Reason:  "GatewayNotFound",
							Message: "gateway was not found",
							Resource: &structs.ResourceReference{
								Kind: structs.APIGateway,
								Name: "gateway",
							},
						}},
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
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "False",
							Reason:  "InvalidCertificate",
							Message: "certificate not found",
							Resource: &structs.ResourceReference{
								Kind: structs.InlineCertificate,
								Name: "certificate",
							},
						}, {
							Type:    "Accepted",
							Status:  "False",
							Reason:  "InvalidCertificates",
							Message: "gateway references invalid certificates",
						}, {
							Type:    "Conflicted",
							Status:  "False",
							Reason:  "NoConflict",
							Message: "listener has no route conflicts",
							Resource: &structs.ResourceReference{
								Kind:        structs.APIGateway,
								Name:        "gateway",
								SectionName: "http-listener",
							},
						}},
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
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "True",
							Reason:  "Accepted",
							Message: "gateway is valid",
						}, {
							Type:    "Conflicted",
							Status:  "False",
							Reason:  "NoConflict",
							Message: "listener has no route conflicts",
							Resource: &structs.ResourceReference{
								Kind:        structs.APIGateway,
								Name:        "gateway",
								SectionName: "http-listener",
							},
						},
						}},
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
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "False",
							Reason:  "InvalidCertificate",
							Message: "certificate not found",
							Resource: &structs.ResourceReference{
								Kind: structs.InlineCertificate,
								Name: "certificate",
							},
						}, {
							Type:    "Accepted",
							Status:  "False",
							Reason:  "InvalidCertificates",
							Message: "gateway references invalid certificates",
						}, {
							Type:    "Conflicted",
							Status:  "False",
							Reason:  "NoConflict",
							Message: "listener has no route conflicts",
							Resource: &structs.ResourceReference{
								Kind:        structs.APIGateway,
								Name:        "gateway",
								SectionName: "http-listener",
							},
						}},
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
						Conditions: []structs.Condition{{
							Type:    "Accepted",
							Status:  "True",
							Reason:  "Accepted",
							Message: "gateway is valid",
						}, {
							Type:    "Conflicted",
							Status:  "False",
							Reason:  "NoConflict",
							Message: "listener has no route conflicts",
							Resource: &structs.ResourceReference{
								Kind:        structs.APIGateway,
								Name:        "gateway",
								SectionName: "http-listener",
							},
						},
						}},
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
						require.True(t, statusEqual, fmt.Sprintf("statuses are unequal: %+v != %+v", string(ppActual), string(ppExpected)))
						if bound, ok := controlled.(*structs.BoundAPIGatewayConfigEntry); ok {
							ppActual, err := json.MarshalIndent(bound, "", "  ")
							require.NoError(t, err)
							ppExpected, err := json.MarshalIndent(expected, "", "  ")
							require.NoError(t, err)
							require.False(t, stateIsDirty(bound, expected.(*structs.BoundAPIGatewayConfigEntry)), fmt.Sprintf("api bound states do not match: %+v != %+v", string(ppActual), string(ppExpected)))
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
