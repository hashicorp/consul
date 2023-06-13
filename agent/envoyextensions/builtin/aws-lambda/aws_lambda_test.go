// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package awslambda

import (
	"fmt"
	"testing"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	envoy_lambda_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/aws_lambda/v3"
	envoy_http_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	envoy_tls_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	pstruct "google.golang.org/protobuf/types/known/structpb"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/envoyextensions/extensioncommon"
	"github.com/hashicorp/consul/proto/private/prototest"
)

func TestConstructor(t *testing.T) {
	kind := api.ServiceKindTerminatingGateway
	cases := map[string]struct {
		extensionName      string
		arn                string
		payloadPassthrough bool
		region             string
		expected           awsLambda
		ok                 bool
	}{
		"no arguments": {
			ok: false,
		},
		"a bad name": {
			arn:           "arn",
			region:        "blah",
			extensionName: "bad",
			ok:            false,
		},
		"missing arn": {
			region: "blah",
			ok:     false,
		},
		"including payload passthrough": {
			arn:                "arn",
			region:             "blah",
			payloadPassthrough: true,
			expected: awsLambda{
				ARN:                "arn",
				PayloadPassthrough: true,
			},
			ok: true,
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			extensionName := api.BuiltinAWSLambdaExtension
			if tc.extensionName != "" {
				extensionName = tc.extensionName
			}
			svc := api.CompoundServiceName{Name: "svc"}
			ext := extensioncommon.RuntimeConfig{
				ServiceName: svc,
				Upstreams: map[api.CompoundServiceName]*extensioncommon.UpstreamData{
					svc: {OutgoingProxyKind: kind},
				},
				EnvoyExtension: api.EnvoyExtension{
					Name: extensionName,
					Arguments: map[string]interface{}{
						"ARN":                tc.arn,
						"PayloadPassthrough": tc.payloadPassthrough,
					},
				},
			}

			e, err := Constructor(ext.EnvoyExtension)

			if tc.ok {
				require.NoError(t, err)
				require.Equal(t, &extensioncommon.UpstreamEnvoyExtender{Extension: &tc.expected}, e)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestCanApply(t *testing.T) {
	a := awsLambda{}
	require.False(t, a.CanApply(&extensioncommon.RuntimeConfig{
		Kind:        api.ServiceKindConnectProxy,
		ServiceName: api.CompoundServiceName{Name: "s1"},
		Upstreams: map[api.CompoundServiceName]*extensioncommon.UpstreamData{
			{Name: "s1"}: {
				OutgoingProxyKind: api.ServiceKindTerminatingGateway,
			},
		},
	}))
	require.True(t, a.CanApply(&extensioncommon.RuntimeConfig{
		Kind:        api.ServiceKindConnectProxy,
		ServiceName: api.CompoundServiceName{Name: "s1"},
		Upstreams: map[api.CompoundServiceName]*extensioncommon.UpstreamData{
			{Name: "s1"}: {
				OutgoingProxyKind: api.ServiceKindConnectProxy,
			},
		},
	}))
}

func TestPatchCluster(t *testing.T) {
	cases := []struct {
		name           string
		lambda         awsLambda
		input          *envoy_cluster_v3.Cluster
		expectedRegion string
		isErrExpected  bool
	}{
		{
			name: "nominal",
			input: &envoy_cluster_v3.Cluster{
				Name: "test-cluster",
			},
			lambda: awsLambda{
				ARN:                "arn:aws:lambda:us-east-1:111111111111:function:lambda-1234",
				PayloadPassthrough: true,
				InvocationMode:     "Asynchronous",
			},
			expectedRegion: "us-east-1",
		},
		{
			name: "error invalid arn",
			input: &envoy_cluster_v3.Cluster{
				Name: "test-cluster",
			},
			lambda: awsLambda{
				ARN:                "?!@%^SA",
				PayloadPassthrough: true,
				InvocationMode:     "Asynchronous",
			},
			isErrExpected: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			transportSocket, err := extensioncommon.MakeUpstreamTLSTransportSocket(&envoy_tls_v3.UpstreamTlsContext{
				Sni: "*.amazonaws.com",
			})
			require.NoError(t, err)

			expectedCluster := &envoy_cluster_v3.Cluster{
				Name:                 tc.input.Name,
				ConnectTimeout:       tc.input.ConnectTimeout,
				ClusterDiscoveryType: &envoy_cluster_v3.Cluster_Type{Type: envoy_cluster_v3.Cluster_LOGICAL_DNS},
				DnsLookupFamily:      envoy_cluster_v3.Cluster_V4_ONLY,
				LbPolicy:             envoy_cluster_v3.Cluster_ROUND_ROBIN,
				Metadata: &envoy_core_v3.Metadata{
					FilterMetadata: map[string]*pstruct.Struct{
						"com.amazonaws.lambda": {
							Fields: map[string]*pstruct.Value{
								"egress_gateway": {Kind: &pstruct.Value_BoolValue{BoolValue: true}},
							},
						},
					},
				},
				LoadAssignment: &envoy_endpoint_v3.ClusterLoadAssignment{
					ClusterName: tc.input.Name,
					Endpoints: []*envoy_endpoint_v3.LocalityLbEndpoints{
						{
							LbEndpoints: []*envoy_endpoint_v3.LbEndpoint{
								{
									HostIdentifier: &envoy_endpoint_v3.LbEndpoint_Endpoint{
										Endpoint: &envoy_endpoint_v3.Endpoint{
											Address: &envoy_core_v3.Address{
												Address: &envoy_core_v3.Address_SocketAddress{
													SocketAddress: &envoy_core_v3.SocketAddress{
														Address: fmt.Sprintf("lambda.%s.amazonaws.com", tc.expectedRegion),
														PortSpecifier: &envoy_core_v3.SocketAddress_PortValue{
															PortValue: 443,
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				TransportSocket: transportSocket,
			}

			// Test patching the cluster
			rc := extensioncommon.RuntimeConfig{}
			patchedCluster, patchSuccess, err := tc.lambda.PatchCluster(extensioncommon.ClusterPayload{
				RuntimeConfig: &rc,
				Message:       tc.input,
			})
			if tc.isErrExpected {
				assert.Error(t, err)
				assert.False(t, patchSuccess)
			} else {
				assert.NoError(t, err)
				assert.True(t, patchSuccess)
				assert.Equal(t, expectedCluster, patchedCluster)
			}
		})
	}
}

func TestPatchRoute(t *testing.T) {
	tests := map[string]struct {
		conf        *extensioncommon.RuntimeConfig
		route       *envoy_route_v3.RouteConfiguration
		expectRoute *envoy_route_v3.RouteConfiguration
		expectBool  bool
	}{
		"non terminating gateway unmodified": {
			conf: &extensioncommon.RuntimeConfig{
				Kind: api.ServiceKindConnectProxy,
			},
			route: &envoy_route_v3.RouteConfiguration{
				VirtualHosts: []*envoy_route_v3.VirtualHost{
					{
						Routes: []*envoy_route_v3.Route{
							{
								Action: &envoy_route_v3.Route_Route{
									Route: &envoy_route_v3.RouteAction{
										HostRewriteSpecifier: &envoy_route_v3.RouteAction_HostRewriteLiteral{},
									},
								},
							},
						},
					},
				},
			},
			expectRoute: &envoy_route_v3.RouteConfiguration{
				VirtualHosts: []*envoy_route_v3.VirtualHost{
					{
						Routes: []*envoy_route_v3.Route{
							{
								Action: &envoy_route_v3.Route_Route{
									Route: &envoy_route_v3.RouteAction{
										HostRewriteSpecifier: &envoy_route_v3.RouteAction_HostRewriteLiteral{},
									},
								},
							},
						},
					},
				},
			},
			expectBool: false,
		},
		"terminating gateway modified": {
			conf: &extensioncommon.RuntimeConfig{
				Kind: api.ServiceKindTerminatingGateway,
			},
			route: &envoy_route_v3.RouteConfiguration{
				VirtualHosts: []*envoy_route_v3.VirtualHost{
					{
						Routes: []*envoy_route_v3.Route{
							// This should be modified.
							{
								Action: &envoy_route_v3.Route_Route{
									Route: &envoy_route_v3.RouteAction{
										HostRewriteSpecifier: &envoy_route_v3.RouteAction_HostRewriteLiteral{},
									},
								},
							},
							// This should be not be modified.
							{
								Action: &envoy_route_v3.Route_DirectResponse{},
							},
						},
					},
				},
			},
			expectRoute: &envoy_route_v3.RouteConfiguration{
				VirtualHosts: []*envoy_route_v3.VirtualHost{
					{
						Routes: []*envoy_route_v3.Route{
							{
								Action: &envoy_route_v3.Route_Route{
									Route: &envoy_route_v3.RouteAction{
										HostRewriteSpecifier: nil,
									},
								},
							},
							{
								Action: &envoy_route_v3.Route_DirectResponse{},
							},
						},
					},
				},
			},
			expectBool: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			l := awsLambda{}
			r, ok, err := l.PatchRoute(extensioncommon.RoutePayload{
				RuntimeConfig: tc.conf,
				Message:       tc.route,
			})
			require.NoError(t, err)
			require.Equal(t, tc.expectRoute, r)
			require.Equal(t, tc.expectBool, ok)
		})
	}
}

func TestPatchFilter(t *testing.T) {

	makeAny := func(m proto.Message) *anypb.Any {
		v, err := anypb.New(m)
		require.NoError(t, err)
		return v
	}
	tests := map[string]struct {
		filter          *envoy_listener_v3.Filter
		isInboundFilter bool
		expectFilter    *envoy_listener_v3.Filter
		expectBool      bool
		expectErr       string
	}{
		"invalid filter name is ignored": {
			filter:       &envoy_listener_v3.Filter{Name: "something"},
			expectFilter: &envoy_listener_v3.Filter{Name: "something"},
			expectBool:   false,
		},
		"error getting typed config": {
			filter:       &envoy_listener_v3.Filter{Name: "envoy.filters.network.http_connection_manager"},
			expectFilter: &envoy_listener_v3.Filter{Name: "envoy.filters.network.http_connection_manager"},
			expectBool:   false,
			expectErr:    "error getting typed config for http filter",
		},
		"error getting http connection manager": {
			filter: &envoy_listener_v3.Filter{
				Name: "envoy.filters.network.http_connection_manager",
				ConfigType: &envoy_listener_v3.Filter_TypedConfig{
					TypedConfig: &anypb.Any{},
				},
			},
			expectFilter: &envoy_listener_v3.Filter{
				Name: "envoy.filters.network.http_connection_manager",
				ConfigType: &envoy_listener_v3.Filter_TypedConfig{
					TypedConfig: &anypb.Any{},
				},
			},
			expectBool: false,
			expectErr:  "error unmarshalling filter",
		},
		"StripAnyHostPort is set": {
			filter: &envoy_listener_v3.Filter{
				Name: "envoy.filters.network.http_connection_manager",
				ConfigType: &envoy_listener_v3.Filter_TypedConfig{
					TypedConfig: makeAny(&envoy_http_v3.HttpConnectionManager{}),
				},
			},
			expectFilter: &envoy_listener_v3.Filter{
				Name: "envoy.filters.network.http_connection_manager",
				ConfigType: &envoy_listener_v3.Filter_TypedConfig{
					TypedConfig: makeAny(&envoy_http_v3.HttpConnectionManager{
						StripPortMode: &envoy_http_v3.HttpConnectionManager_StripAnyHostPort{
							StripAnyHostPort: true,
						},
					}),
				},
			},
			expectBool: true,
		},
		"lambda filter injected correctly": {
			filter: &envoy_listener_v3.Filter{
				Name: "envoy.filters.network.http_connection_manager",
				ConfigType: &envoy_listener_v3.Filter_TypedConfig{
					TypedConfig: makeAny(&envoy_http_v3.HttpConnectionManager{
						HttpFilters: []*envoy_http_v3.HttpFilter{
							{Name: "one"},
							{Name: "two"},
							{Name: "envoy.filters.http.router"},
							{Name: "three"},
						},
					}),
				},
			},
			expectFilter: &envoy_listener_v3.Filter{
				Name: "envoy.filters.network.http_connection_manager",
				ConfigType: &envoy_listener_v3.Filter_TypedConfig{
					TypedConfig: makeAny(&envoy_http_v3.HttpConnectionManager{
						StripPortMode: &envoy_http_v3.HttpConnectionManager_StripAnyHostPort{
							StripAnyHostPort: true,
						},
						HttpFilters: []*envoy_http_v3.HttpFilter{
							{Name: "one"},
							{Name: "two"},
							{
								Name: "envoy.filters.http.aws_lambda",
								ConfigType: &envoy_http_v3.HttpFilter_TypedConfig{TypedConfig: makeAny(
									&envoy_lambda_v3.Config{
										Arn:                "some-arn",
										PayloadPassthrough: true,
										InvocationMode:     envoy_lambda_v3.Config_ASYNCHRONOUS,
									},
								)},
							},
							{Name: "envoy.filters.http.router"},
							{Name: "three"},
						},
					}),
				},
			},
			expectBool: true,
		},
		"inbound filter ignored": {
			filter: &envoy_listener_v3.Filter{
				Name: "envoy.filters.network.http_connection_manager",
				ConfigType: &envoy_listener_v3.Filter_TypedConfig{
					TypedConfig: makeAny(&envoy_http_v3.HttpConnectionManager{
						HttpFilters: []*envoy_http_v3.HttpFilter{
							{Name: "one"},
							{Name: "two"},
							{Name: "envoy.filters.http.router"},
							{Name: "three"},
						},
					}),
				},
			},
			expectFilter: &envoy_listener_v3.Filter{
				Name: "envoy.filters.network.http_connection_manager",
				ConfigType: &envoy_listener_v3.Filter_TypedConfig{
					TypedConfig: makeAny(&envoy_http_v3.HttpConnectionManager{
						HttpFilters: []*envoy_http_v3.HttpFilter{
							{Name: "one"},
							{Name: "two"},
							{Name: "envoy.filters.http.router"},
							{Name: "three"},
						},
					}),
				},
			},
			isInboundFilter: true,
			expectBool:      false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			l := awsLambda{
				ARN:                "some-arn",
				PayloadPassthrough: true,
				InvocationMode:     "asynchronous",
			}
			d := extensioncommon.TrafficDirectionOutbound
			if tc.isInboundFilter {
				d = extensioncommon.TrafficDirectionInbound
			}
			f, ok, err := l.PatchFilter(extensioncommon.FilterPayload{
				Message:          tc.filter,
				TrafficDirection: d,
			})
			require.Equal(t, tc.expectBool, ok)
			if tc.expectErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.expectErr)
			}
			prototest.AssertDeepEqual(t, tc.expectFilter, f)
		})
	}
}
