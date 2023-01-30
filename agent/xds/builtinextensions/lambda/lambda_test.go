package lambda

import (
	"fmt"
	"testing"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	envoy_tls_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pstruct "google.golang.org/protobuf/types/known/structpb"

	"github.com/hashicorp/consul/agent/xds/xdscommon"
	"github.com/hashicorp/consul/api"
)

func TestMakeLambdaExtension(t *testing.T) {
	kind := api.ServiceKindTerminatingGateway
	cases := map[string]struct {
		extensionName      string
		arn                string
		payloadPassthrough bool
		region             string
		expected           lambda
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
			expected: lambda{
				ARN:                "arn",
				PayloadPassthrough: true,
				Kind:               kind,
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
			ext := xdscommon.ExtensionConfiguration{
				ServiceName: svc,
				Upstreams: map[api.CompoundServiceName]xdscommon.UpstreamData{
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

			plugin, err := MakeLambdaExtension(ext)

			if tc.ok {
				require.NoError(t, err)
				require.Equal(t, tc.expected, plugin)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestPatchCluster(t *testing.T) {
	cases := []struct {
		name           string
		lambda         lambda
		input          *envoy_cluster_v3.Cluster
		expectedRegion string
		isErrExpected  bool
	}{
		{
			name: "nominal",
			input: &envoy_cluster_v3.Cluster{
				Name: "test-cluster",
			},
			lambda: lambda{
				ARN:                "arn:aws:lambda:us-east-1:111111111111:function:lambda-1234",
				PayloadPassthrough: true,
				Kind:               "some-name",
				InvocationMode:     "Asynchronous",
			},
			expectedRegion: "us-east-1",
		},
		{
			name: "error invalid arn",
			input: &envoy_cluster_v3.Cluster{
				Name: "test-cluster",
			},
			lambda: lambda{
				ARN:                "?!@%^SA",
				PayloadPassthrough: true,
				Kind:               "some-name",
				InvocationMode:     "Asynchronous",
			},
			isErrExpected: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			transportSocket, err := makeUpstreamTLSTransportSocket(&envoy_tls_v3.UpstreamTlsContext{
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
			patchedCluster, patchSuccess, err := tc.lambda.PatchCluster(tc.input)
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
