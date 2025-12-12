// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

package extensioncommon

import (
	"fmt"
	"testing"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/api"
)

// TestBasicEnvoyExtender_CanApply tests the CanApply functionality
func TestBasicEnvoyExtender_CanApply(t *testing.T) {
	tests := []struct {
		name     string
		extender *BasicEnvoyExtender
		config   *RuntimeConfig
		expected bool
	}{
		{
			name: "API Gateway with matching proxy type",
			extender: &BasicEnvoyExtender{
				Extension: &mockExtension{
					proxyType: "api-gateway",
				},
			},
			config: &RuntimeConfig{
				Kind: api.ServiceKindAPIGateway,
			},
			expected: true,
		},
		{
			name: "API Gateway with non-matching proxy type",
			extender: &BasicEnvoyExtender{
				Extension: &mockExtension{
					proxyType: "connect-proxy",
				},
			},
			config: &RuntimeConfig{
				Kind: api.ServiceKindAPIGateway,
			},
			expected: false,
		},
		{
			name: "Connect Proxy with matching proxy type",
			extender: &BasicEnvoyExtender{
				Extension: &mockExtension{
					proxyType: "connect-proxy",
				},
			},
			config: &RuntimeConfig{
				Kind: api.ServiceKindConnectProxy,
			},
			expected: true,
		},
		{
			name: "Connect Proxy with non-matching proxy type",
			extender: &BasicEnvoyExtender{
				Extension: &mockExtension{
					proxyType: "api-gateway",
				},
			},
			config: &RuntimeConfig{
				Kind: api.ServiceKindConnectProxy,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := tt.extender.CanApply(tt.config)
			require.Equal(t, tt.expected, actual)
		})
	}
}

// TestBasicEnvoyExtender_PatchFilter tests the PatchFilter functionality
func TestBasicEnvoyExtender_PatchFilter(t *testing.T) {
	tests := []struct {
		name           string
		extender       *BasicEnvoyExtender
		payload        FilterPayload
		expectedConfig *envoy_listener_v3.Filter
		expectedError  string
	}{
		{
			name: "API Gateway successful filter patch",
			extender: &BasicEnvoyExtender{
				Extension: &mockExtension{
					proxyType: "api-gateway",
					patchFunc: func(payload FilterPayload) (*envoy_listener_v3.Filter, bool, error) {
						return &envoy_listener_v3.Filter{
							Name: "test-filter",
							ConfigType: &envoy_listener_v3.Filter_TypedConfig{
								TypedConfig: mustMarshalAny(&envoy_core_v3.DataSource{
									Specifier: &envoy_core_v3.DataSource_InlineString{
										InlineString: "test-data",
									},
								}),
							},
						}, true, nil
					},
				},
			},
			payload: FilterPayload{
				Message: &envoy_listener_v3.Filter{
					Name: "test-filter",
				},
				RuntimeConfig: &RuntimeConfig{
					Kind: api.ServiceKindAPIGateway,
				},
			},
			expectedConfig: &envoy_listener_v3.Filter{
				Name: "test-filter",
				ConfigType: &envoy_listener_v3.Filter_TypedConfig{
					TypedConfig: mustMarshalAny(&envoy_core_v3.DataSource{
						Specifier: &envoy_core_v3.DataSource_InlineString{
							InlineString: "test-data",
						},
					}),
				},
			},
		},
		{
			name: "Connect Proxy successful filter patch",
			extender: &BasicEnvoyExtender{
				Extension: &mockExtension{
					proxyType: "connect-proxy",
					patchFunc: func(payload FilterPayload) (*envoy_listener_v3.Filter, bool, error) {
						return &envoy_listener_v3.Filter{
							Name: "connect-proxy-filter",
							ConfigType: &envoy_listener_v3.Filter_TypedConfig{
								TypedConfig: mustMarshalAny(&envoy_core_v3.DataSource{
									Specifier: &envoy_core_v3.DataSource_InlineString{
										InlineString: "connect-proxy-data",
									},
								}),
							},
						}, true, nil
					},
				},
			},
			payload: FilterPayload{
				Message: &envoy_listener_v3.Filter{
					Name: "connect-proxy-filter",
				},
				RuntimeConfig: &RuntimeConfig{
					Kind: api.ServiceKindConnectProxy,
				},
			},
			expectedConfig: &envoy_listener_v3.Filter{
				Name: "connect-proxy-filter",
				ConfigType: &envoy_listener_v3.Filter_TypedConfig{
					TypedConfig: mustMarshalAny(&envoy_core_v3.DataSource{
						Specifier: &envoy_core_v3.DataSource_InlineString{
							InlineString: "connect-proxy-data",
						},
					}),
				},
			},
		},
		{
			name: "Connect Proxy filter patch with error",
			extender: &BasicEnvoyExtender{
				Extension: &mockExtension{
					proxyType: "connect-proxy",
					patchFunc: func(payload FilterPayload) (*envoy_listener_v3.Filter, bool, error) {
						return nil, false, fmt.Errorf("test error")
					},
				},
			},
			payload: FilterPayload{
				Message: &envoy_listener_v3.Filter{
					Name: "connect-proxy-filter",
				},
				RuntimeConfig: &RuntimeConfig{
					Kind: api.ServiceKindConnectProxy,
				},
			},
			expectedError: "test error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, _, err := tt.extender.Extension.PatchFilter(tt.payload)
			if tt.expectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedError)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expectedConfig, actual)
		})
	}
}

// mockExtension implements the Extension interface for testing
type mockExtension struct {
	proxyType string
	patchFunc func(FilterPayload) (*envoy_listener_v3.Filter, bool, error)
}

func (m *mockExtension) CanApply(config *RuntimeConfig) bool {
	return config.Kind == api.ServiceKind(m.proxyType)
}

func (m *mockExtension) Validate(config *RuntimeConfig) error {
	return nil
}

func (m *mockExtension) PatchFilter(payload FilterPayload) (*envoy_listener_v3.Filter, bool, error) {
	if m.patchFunc != nil {
		return m.patchFunc(payload)
	}
	return payload.Message, false, nil
}

func (m *mockExtension) PatchCluster(payload ClusterPayload) (*envoy_cluster_v3.Cluster, bool, error) {
	return payload.Message, false, nil
}

func (m *mockExtension) PatchClusterLoadAssignment(payload ClusterLoadAssignmentPayload) (*envoy_endpoint_v3.ClusterLoadAssignment, bool, error) {
	return payload.Message, false, nil
}

func (m *mockExtension) PatchClusters(config *RuntimeConfig, clusters ClusterMap) (ClusterMap, error) {
	return clusters, nil
}

func (m *mockExtension) PatchRoutes(config *RuntimeConfig, routes RouteMap) (RouteMap, error) {
	return routes, nil
}

func (m *mockExtension) PatchListeners(config *RuntimeConfig, listeners ListenerMap) (ListenerMap, error) {
	return listeners, nil
}

func (m *mockExtension) PatchFilters(config *RuntimeConfig, filters []*envoy_listener_v3.Filter, isInboundListener bool) ([]*envoy_listener_v3.Filter, error) {
	return filters, nil
}

func (m *mockExtension) PatchRoute(payload RoutePayload) (*envoy_route_v3.RouteConfiguration, bool, error) {
	return payload.Message, false, nil
}

func (m *mockExtension) PatchListener(payload ListenerPayload) (*envoy_listener_v3.Listener, bool, error) {
	return payload.Message, false, nil
}

func mustMarshalAny(pb proto.Message) *anypb.Any {
	a, err := anypb.New(pb)
	if err != nil {
		panic(err)
	}
	return a
}
