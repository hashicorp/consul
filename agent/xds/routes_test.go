// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"testing"
	"time"

	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/hashicorp/consul/agent/structs"
)

func TestEnvoyLBConfig_InjectToRouteAction(t *testing.T) {
	var tests = []struct {
		name     string
		lb       *structs.LoadBalancer
		expected *envoy_route_v3.RouteAction
	}{
		{
			name: "empty",
			lb: &structs.LoadBalancer{
				Policy: "",
			},
			// we only modify route actions for hash-based LB policies
			expected: &envoy_route_v3.RouteAction{},
		},
		{
			name: "least request",
			lb: &structs.LoadBalancer{
				Policy: structs.LBPolicyLeastRequest,
				LeastRequestConfig: &structs.LeastRequestConfig{
					ChoiceCount: 3,
				},
			},
			// we only modify route actions for hash-based LB policies
			expected: &envoy_route_v3.RouteAction{},
		},
		{
			name: "headers",
			lb: &structs.LoadBalancer{
				Policy: "ring_hash",
				RingHashConfig: &structs.RingHashConfig{
					MinimumRingSize: 3,
					MaximumRingSize: 7,
				},
				HashPolicies: []structs.HashPolicy{
					{
						Field:      structs.HashPolicyHeader,
						FieldValue: "x-route-key",
						Terminal:   true,
					},
				},
			},
			expected: &envoy_route_v3.RouteAction{
				HashPolicy: []*envoy_route_v3.RouteAction_HashPolicy{
					{
						PolicySpecifier: &envoy_route_v3.RouteAction_HashPolicy_Header_{
							Header: &envoy_route_v3.RouteAction_HashPolicy_Header{
								HeaderName: "x-route-key",
							},
						},
						Terminal: true,
					},
				},
			},
		},
		{
			name: "cookies",
			lb: &structs.LoadBalancer{
				Policy: structs.LBPolicyMaglev,
				HashPolicies: []structs.HashPolicy{
					{
						Field:      structs.HashPolicyCookie,
						FieldValue: "red-velvet",
						Terminal:   true,
					},
					{
						Field:      structs.HashPolicyCookie,
						FieldValue: "oatmeal",
					},
				},
			},
			expected: &envoy_route_v3.RouteAction{
				HashPolicy: []*envoy_route_v3.RouteAction_HashPolicy{
					{
						PolicySpecifier: &envoy_route_v3.RouteAction_HashPolicy_Cookie_{
							Cookie: &envoy_route_v3.RouteAction_HashPolicy_Cookie{
								Name: "red-velvet",
							},
						},
						Terminal: true,
					},
					{
						PolicySpecifier: &envoy_route_v3.RouteAction_HashPolicy_Cookie_{
							Cookie: &envoy_route_v3.RouteAction_HashPolicy_Cookie{
								Name: "oatmeal",
							},
						},
					},
				},
			},
		},
		{
			name: "non-zero session ttl gets zeroed out",
			lb: &structs.LoadBalancer{
				Policy: structs.LBPolicyMaglev,
				HashPolicies: []structs.HashPolicy{
					{
						Field:      structs.HashPolicyCookie,
						FieldValue: "oatmeal",
						CookieConfig: &structs.CookieConfig{
							TTL:     10 * time.Second,
							Session: true,
						},
					},
				},
			},
			expected: &envoy_route_v3.RouteAction{
				HashPolicy: []*envoy_route_v3.RouteAction_HashPolicy{
					{
						PolicySpecifier: &envoy_route_v3.RouteAction_HashPolicy_Cookie_{
							Cookie: &envoy_route_v3.RouteAction_HashPolicy_Cookie{
								Name: "oatmeal",
								Ttl:  durationpb.New(0 * time.Second),
							},
						},
					},
				},
			},
		},
		{
			name: "zero value ttl omitted if not session cookie",
			lb: &structs.LoadBalancer{
				Policy: structs.LBPolicyMaglev,
				HashPolicies: []structs.HashPolicy{
					{
						Field:      structs.HashPolicyCookie,
						FieldValue: "oatmeal",
						CookieConfig: &structs.CookieConfig{
							Path: "/oven",
						},
					},
				},
			},
			expected: &envoy_route_v3.RouteAction{
				HashPolicy: []*envoy_route_v3.RouteAction_HashPolicy{
					{
						PolicySpecifier: &envoy_route_v3.RouteAction_HashPolicy_Cookie_{
							Cookie: &envoy_route_v3.RouteAction_HashPolicy_Cookie{
								Name: "oatmeal",
								Path: "/oven",
								Ttl:  nil,
							},
						},
					},
				},
			},
		},
		{
			name: "source addr",
			lb: &structs.LoadBalancer{
				Policy: structs.LBPolicyMaglev,
				HashPolicies: []structs.HashPolicy{
					{
						SourceIP: true,
						Terminal: true,
					},
				},
			},
			expected: &envoy_route_v3.RouteAction{
				HashPolicy: []*envoy_route_v3.RouteAction_HashPolicy{
					{
						PolicySpecifier: &envoy_route_v3.RouteAction_HashPolicy_ConnectionProperties_{
							ConnectionProperties: &envoy_route_v3.RouteAction_HashPolicy_ConnectionProperties{
								SourceIp: true,
							},
						},
						Terminal: true,
					},
				},
			},
		},
		{
			name: "kitchen sink",
			lb: &structs.LoadBalancer{
				Policy: structs.LBPolicyMaglev,
				HashPolicies: []structs.HashPolicy{
					{
						SourceIP: true,
						Terminal: true,
					},
					{
						Field:      structs.HashPolicyCookie,
						FieldValue: "oatmeal",
						CookieConfig: &structs.CookieConfig{
							TTL:  10 * time.Second,
							Path: "/oven",
						},
					},
					{
						Field:      structs.HashPolicyCookie,
						FieldValue: "chocolate-chip",
						CookieConfig: &structs.CookieConfig{
							Session: true,
							Path:    "/oven",
						},
					},
					{
						Field:      structs.HashPolicyHeader,
						FieldValue: "special-header",
						Terminal:   true,
					},
					{
						Field:      structs.HashPolicyQueryParam,
						FieldValue: "my-pretty-param",
						Terminal:   true,
					},
				},
			},
			expected: &envoy_route_v3.RouteAction{
				HashPolicy: []*envoy_route_v3.RouteAction_HashPolicy{
					{
						PolicySpecifier: &envoy_route_v3.RouteAction_HashPolicy_ConnectionProperties_{
							ConnectionProperties: &envoy_route_v3.RouteAction_HashPolicy_ConnectionProperties{
								SourceIp: true,
							},
						},
						Terminal: true,
					},
					{
						PolicySpecifier: &envoy_route_v3.RouteAction_HashPolicy_Cookie_{
							Cookie: &envoy_route_v3.RouteAction_HashPolicy_Cookie{
								Name: "oatmeal",
								Ttl:  durationpb.New(10 * time.Second),
								Path: "/oven",
							},
						},
					},
					{
						PolicySpecifier: &envoy_route_v3.RouteAction_HashPolicy_Cookie_{
							Cookie: &envoy_route_v3.RouteAction_HashPolicy_Cookie{
								Name: "chocolate-chip",
								Ttl:  durationpb.New(0 * time.Second),
								Path: "/oven",
							},
						},
					},
					{
						PolicySpecifier: &envoy_route_v3.RouteAction_HashPolicy_Header_{
							Header: &envoy_route_v3.RouteAction_HashPolicy_Header{
								HeaderName: "special-header",
							},
						},
						Terminal: true,
					},
					{
						PolicySpecifier: &envoy_route_v3.RouteAction_HashPolicy_QueryParameter_{
							QueryParameter: &envoy_route_v3.RouteAction_HashPolicy_QueryParameter{
								Name: "my-pretty-param",
							},
						},
						Terminal: true,
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var ra envoy_route_v3.RouteAction
			err := injectLBToRouteAction(tc.lb, &ra)
			require.NoError(t, err)

			require.Equal(t, tc.expected, &ra)
		})
	}
}
