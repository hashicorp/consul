// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"bytes"
	"testing"
	"text/template"

	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_http_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"github.com/hashicorp/go-hclog"
	testinf "github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/xds/configfetcher"
)

type customListenerJSONOptions struct {
	Name       string
	TLSContext string
}

const customListenerJSONTpl = `{
	"@type": "type.googleapis.com/envoy.config.listener.v3.Listener",
	"name": "{{ .Name }}",
	"address": {
		"socketAddress": {
			"address": "11.11.11.11",
			"portValue": 11111
		}
	},
	"filterChains": [
		{
			{{ if .TLSContext -}}
			"transport_socket": {
				"name": "tls",
				"typed_config": {
					"@type": "type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.UpstreamTlsContext",
					{{ .TLSContext }}
				}
			},
			{{- end }}
			"filters": [
				{
					"name": "envoy.filters.network.tcp_proxy",
					"typedConfig": {
						"@type": "type.googleapis.com/envoy.extensions.filters.network.tcp_proxy.v3.TcpProxy",
							"cluster": "random-cluster",
							"statPrefix": "foo-stats"
						}
				}
			]
		}
	]
}`

type customHTTPListenerJSONOptions struct {
	Name                      string
	HTTPConnectionManagerName string
}

const customHTTPListenerJSONTpl = `{
	"@type": "type.googleapis.com/envoy.config.listener.v3.Listener",
	"name": "{{ .Name }}",
	"address": {
		"socketAddress": {
			"address": "11.11.11.11",
			"portValue": 11111
		}
	},
	"filterChains": [
		{
			"filters": [
				{
					"name": "{{ .HTTPConnectionManagerName }}",
					"typedConfig": {
						"@type": "type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager",
						"http_filters": [
							{
								"name": "envoy.filters.http.router"
							}
						],
						"route_config": {
							"name": "public_listener",
							"virtual_hosts": [
								{
									"domains": [
										"*"
									],
									"name": "public_listener",
									"routes": [
										{
											"match": {
												"prefix": "/"
											},
											"route": {
												"cluster": "random-cluster"
											}
										}
									]
								}
							]
						}
					}
				}
			]
		}
	]
}`

var (
	customListenerJSONTemplate     = template.Must(template.New("").Parse(customListenerJSONTpl))
	customHTTPListenerJSONTemplate = template.Must(template.New("").Parse(customHTTPListenerJSONTpl))
)

func customListenerJSON(t testinf.T, opts customListenerJSONOptions) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, customListenerJSONTemplate.Execute(&buf, opts))
	return buf.String()
}

func customHTTPListenerJSON(t testinf.T, opts customHTTPListenerJSONOptions) string {
	t.Helper()
	if opts.HTTPConnectionManagerName == "" {
		opts.HTTPConnectionManagerName = httpConnectionManagerNewName
	}
	var buf bytes.Buffer
	require.NoError(t, customHTTPListenerJSONTemplate.Execute(&buf, opts))
	return buf.String()
}

func customTraceJSON(t testinf.T) string {
	t.Helper()
	return `
	{
        "@type" : "type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager.Tracing",
        "provider" : {
          "name" : "envoy.tracers.zipkin",
          "typed_config" : {
            "@type" : "type.googleapis.com/envoy.config.trace.v3.ZipkinConfig",
            "collector_cluster" : "otelcolector",
            "collector_endpoint" : "/api/v2/spans",
            "collector_endpoint_version" : "HTTP_JSON",
            "shared_span_context" : false
          }
        },
        "custom_tags" : [
          {
            "tag" : "custom_header",
            "request_header" : {
              "name" : "x-custom-traceid",
              "default_value" : ""
            }
          },
          {
            "tag" : "alloc_id",
            "environment" : {
              "name" : "NOMAD_ALLOC_ID"
            }
          }
        ]
      }
	`
}

type configFetcherFunc func() string

var _ configfetcher.ConfigFetcher = (configFetcherFunc)(nil)

func (f configFetcherFunc) AdvertiseAddrLAN() string {
	return f()
}

func TestResolveListenerSDSConfig(t *testing.T) {
	type testCase struct {
		name    string
		gwSDS   *structs.GatewayTLSSDSConfig
		lisSDS  *structs.GatewayTLSSDSConfig
		want    *structs.GatewayTLSSDSConfig
		wantErr string
	}

	run := func(tc testCase) {
		// fake a snapshot with just the data we care about
		snap := proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp", "default", nil, func(entry *structs.IngressGatewayConfigEntry) {
			entry.TLS = structs.GatewayTLSConfig{
				SDS: &structs.GatewayTLSSDSConfig{
					ClusterName:  "sds-cluster",
					CertResource: "cert-resource",
				},
			}
		}, nil)
		// Override TLS configs
		snap.IngressGateway.TLSConfig.SDS = tc.gwSDS
		var listenerCfg structs.IngressListener
		for k, lisCfg := range snap.IngressGateway.Listeners {
			if tc.lisSDS == nil {
				lisCfg.TLS = nil
			} else {
				lisCfg.TLS = &structs.GatewayTLSConfig{
					SDS: tc.lisSDS,
				}
			}
			// Override listener cfg in map
			snap.IngressGateway.Listeners[k] = lisCfg
			// Save the last cfg doesn't matter which as we set same for all.
			listenerCfg = lisCfg
		}

		got, err := resolveListenerSDSConfig(snap.IngressGateway.TLSConfig.SDS, listenerCfg.TLS, listenerCfg.Port)
		if tc.wantErr != "" {
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantErr)
		} else {
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		}
	}

	cases := []testCase{
		{
			name:   "no SDS config",
			gwSDS:  nil,
			lisSDS: nil,
			want:   nil,
		},
		{
			name: "all cluster-level SDS config",
			gwSDS: &structs.GatewayTLSSDSConfig{
				ClusterName:  "cluster",
				CertResource: "cert",
			},
			lisSDS: nil,
			want: &structs.GatewayTLSSDSConfig{
				ClusterName:  "cluster",
				CertResource: "cert",
			},
		},
		{
			name:  "all listener-level SDS config",
			gwSDS: nil,
			lisSDS: &structs.GatewayTLSSDSConfig{
				ClusterName:  "cluster",
				CertResource: "cert",
			},
			want: &structs.GatewayTLSSDSConfig{
				ClusterName:  "cluster",
				CertResource: "cert",
			},
		},
		{
			name: "mixed level SDS config",
			gwSDS: &structs.GatewayTLSSDSConfig{
				ClusterName: "cluster",
			},
			lisSDS: &structs.GatewayTLSSDSConfig{
				CertResource: "cert",
			},
			want: &structs.GatewayTLSSDSConfig{
				ClusterName:  "cluster",
				CertResource: "cert",
			},
		},
		{
			name: "override cert",
			gwSDS: &structs.GatewayTLSSDSConfig{
				ClusterName:  "cluster",
				CertResource: "gw-cert",
			},
			lisSDS: &structs.GatewayTLSSDSConfig{
				CertResource: "lis-cert",
			},
			want: &structs.GatewayTLSSDSConfig{
				ClusterName:  "cluster",
				CertResource: "lis-cert",
			},
		},
		{
			name: "override both",
			gwSDS: &structs.GatewayTLSSDSConfig{
				ClusterName:  "gw-cluster",
				CertResource: "gw-cert",
			},
			lisSDS: &structs.GatewayTLSSDSConfig{
				ClusterName:  "lis-cluster",
				CertResource: "lis-cert",
			},
			want: &structs.GatewayTLSSDSConfig{
				ClusterName:  "lis-cluster",
				CertResource: "lis-cert",
			},
		},
		{
			name:  "missing cluster listener",
			gwSDS: nil,
			lisSDS: &structs.GatewayTLSSDSConfig{
				CertResource: "lis-cert",
			},
			wantErr: "missing SDS cluster name",
		},
		{
			name:  "missing cert listener",
			gwSDS: nil,
			lisSDS: &structs.GatewayTLSSDSConfig{
				ClusterName: "cluster",
			},
			wantErr: "missing SDS cert resource",
		},
		{
			name: "missing cluster gw",
			gwSDS: &structs.GatewayTLSSDSConfig{
				CertResource: "lis-cert",
			},
			lisSDS:  nil,
			wantErr: "missing SDS cluster name",
		},
		{
			name: "missing cert gw",
			gwSDS: &structs.GatewayTLSSDSConfig{
				ClusterName: "cluster",
			},
			lisSDS:  nil,
			wantErr: "missing SDS cert resource",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			run(tc)
		})
	}

}

func TestGetAlpnProtocols(t *testing.T) {
	tests := map[string]struct {
		protocol string
		want     []string
	}{
		"http": {
			protocol: "http",
			want:     []string{"http/1.1"},
		},
		"http2": {
			protocol: "http2",
			want:     []string{"h2", "http/1.1"},
		},
		"grpc": {
			protocol: "grpc",
			want:     []string{"h2", "http/1.1"},
		},
		"tcp": {
			protocol: "",
			want:     nil,
		},
		"empty": {
			protocol: "",
			want:     nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := getAlpnProtocols(tc.protocol)
			assert.Equal(t, tc.want, got)
		})
	}
}

func Test_setNormalizationOptions(t *testing.T) {
	tests := map[string]struct {
		rn   *structs.RequestNormalizationMeshConfig
		opts *listenerFilterOpts
		want *listenerFilterOpts
	}{
		"nil entry": {
			rn:   nil,
			opts: &listenerFilterOpts{},
			want: &listenerFilterOpts{
				normalizePath: true,
			},
		},
		"empty entry": {
			rn:   &structs.RequestNormalizationMeshConfig{},
			opts: &listenerFilterOpts{},
			want: &listenerFilterOpts{
				normalizePath: true,
			},
		},
		"empty is equivalent to defaults": {
			rn:   &structs.RequestNormalizationMeshConfig{},
			opts: &listenerFilterOpts{},
			want: &listenerFilterOpts{
				normalizePath:                true,
				mergeSlashes:                 false,
				pathWithEscapedSlashesAction: envoy_http_v3.HttpConnectionManager_IMPLEMENTATION_SPECIFIC_DEFAULT,
				headersWithUnderscoresAction: envoy_core_v3.HttpProtocolOptions_ALLOW,
			},
		},
		"some options": {
			rn: &structs.RequestNormalizationMeshConfig{
				InsecureDisablePathNormalization: false,
				MergeSlashes:                     true,
				PathWithEscapedSlashesAction:     "",
				HeadersWithUnderscoresAction:     "DROP_HEADER",
			},
			opts: &listenerFilterOpts{},
			want: &listenerFilterOpts{
				normalizePath:                true,
				mergeSlashes:                 true,
				headersWithUnderscoresAction: envoy_core_v3.HttpProtocolOptions_DROP_HEADER,
			},
		},
		"all options": {
			rn: &structs.RequestNormalizationMeshConfig{
				InsecureDisablePathNormalization: true, // note: this is the opposite of the recommended default
				MergeSlashes:                     true,
				PathWithEscapedSlashesAction:     "REJECT_REQUEST",
				HeadersWithUnderscoresAction:     "DROP_HEADER",
			},
			opts: &listenerFilterOpts{},
			want: &listenerFilterOpts{
				normalizePath:                false,
				mergeSlashes:                 true,
				pathWithEscapedSlashesAction: envoy_http_v3.HttpConnectionManager_REJECT_REQUEST,
				headersWithUnderscoresAction: envoy_core_v3.HttpProtocolOptions_DROP_HEADER,
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			setNormalizationOptions(tc.rn, tc.opts)
			assert.Equal(t, tc.want, tc.opts)
		})
	}
}

func Test_makeHTTPFilter_maxRequestHeadersKb(t *testing.T) {
	tests := map[string]struct {
		maxRequestHeadersKb *uint32
		wantPresent         bool
		wantValue           uint32
	}{
		"nil - not set": {
			maxRequestHeadersKb: nil,
			wantPresent:         false,
		},
		"96KB": {
			maxRequestHeadersKb: uintPointer(96),
			wantPresent:         true,
			wantValue:           96,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			opts := listenerFilterOpts{
				protocol:            "http",
				filterName:          "test",
				routeName:           "test",
				cluster:             "test",
				maxRequestHeadersKb: tc.maxRequestHeadersKb,
			}

			filter, err := makeHTTPFilter(opts)
			require.NoError(t, err)
			require.NotNil(t, filter)

			// Decode the typed config to check MaxRequestHeadersKb
			var httpConnMgr envoy_http_v3.HttpConnectionManager
			err = filter.GetTypedConfig().UnmarshalTo(&httpConnMgr)
			require.NoError(t, err)

			if tc.wantPresent {
				require.NotNil(t, httpConnMgr.MaxRequestHeadersKb)
				require.Equal(t, tc.wantValue, httpConnMgr.MaxRequestHeadersKb.GetValue())
			} else {
				require.Nil(t, httpConnMgr.MaxRequestHeadersKb)
			}
		})
	}
}

func Test_makeUpstreamFilterChain_maxRequestHeadersKb(t *testing.T) {
	tests := map[string]struct {
		maxRequestHeadersKb *uint32
		protocol            string
		wantPresent         bool
		wantValue           uint32
	}{
		"tcp protocol ignores header settings": {
			maxRequestHeadersKb: uintPointer(96),
			protocol:            "tcp",
			wantPresent:         false,
		},
		"http with nil header settings": {
			maxRequestHeadersKb: nil,
			protocol:            "http",
			wantPresent:         false,
		},
		"http with 96KB header limit": {
			maxRequestHeadersKb: uintPointer(96),
			protocol:            "http",
			wantPresent:         true,
			wantValue:           96,
		},
		"grpc with 128KB header limit": {
			maxRequestHeadersKb: uintPointer(128),
			protocol:            "grpc",
			wantPresent:         true,
			wantValue:           128,
		},
		"http2 with 256KB header limit": {
			maxRequestHeadersKb: uintPointer(256),
			protocol:            "http2",
			wantPresent:         true,
			wantValue:           256,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			s := &ResourceGenerator{
				Logger: hclog.NewNullLogger(),
			}

			opts := filterChainOpts{
				protocol:            tc.protocol,
				filterName:          "test",
				clusterName:         "test-cluster",
				maxRequestHeadersKb: tc.maxRequestHeadersKb,
			}

			filterChain, err := s.makeUpstreamFilterChain(opts)
			require.NoError(t, err)
			require.NotNil(t, filterChain)
			require.Len(t, filterChain.Filters, 1)

			filter := filterChain.Filters[0]

			if tc.protocol == "tcp" {
				// TCP proxy doesn't use HTTP connection manager
				return
			}

			// For HTTP-like protocols, check the HTTP connection manager
			var httpConnMgr envoy_http_v3.HttpConnectionManager
			err = filter.GetTypedConfig().UnmarshalTo(&httpConnMgr)
			require.NoError(t, err)

			if tc.wantPresent {
				require.NotNil(t, httpConnMgr.MaxRequestHeadersKb)
				require.Equal(t, tc.wantValue, httpConnMgr.MaxRequestHeadersKb.GetValue())
			} else {
				require.Nil(t, httpConnMgr.MaxRequestHeadersKb)
			}
		})
	}
}

func Test_makeMeshGatewayPeerFilterChain_maxRequestHeadersKb(t *testing.T) {
	tests := map[string]struct {
		proxyConfig        map[string]interface{}
		serviceProxyConfig map[string]interface{}
		protocol           string
		wantPresent        bool
		wantValue          uint32
	}{
		"defaults to proxy config when no service override": {
			proxyConfig: map[string]interface{}{
				"max_request_headers_kb": int64(128),
			},
			serviceProxyConfig: map[string]interface{}{},
			protocol:           "http",
			wantPresent:        true,
			wantValue:          128,
		},
		"service override takes precedence over proxy config": {
			proxyConfig: map[string]interface{}{
				"max_request_headers_kb": int64(128),
			},
			serviceProxyConfig: map[string]interface{}{
				"max_request_headers_kb": int64(256),
			},
			protocol:    "http",
			wantPresent: true,
			wantValue:   256,
		},
		"no proxy config and no service override": {
			proxyConfig:        map[string]interface{}{},
			serviceProxyConfig: map[string]interface{}{},
			protocol:           "http",
			wantPresent:        false,
		},
		"service override with nil proxy config": {
			proxyConfig: nil,
			serviceProxyConfig: map[string]interface{}{
				"max_request_headers_kb": int64(64),
			},
			protocol:    "http",
			wantPresent: true,
			wantValue:   64,
		},
		"tcp protocol ignores header settings": {
			proxyConfig: map[string]interface{}{
				"max_request_headers_kb": int64(128),
			},
			serviceProxyConfig: map[string]interface{}{
				"max_request_headers_kb": int64(256),
			},
			protocol:    "tcp",
			wantPresent: false,
		},
		"grpc protocol uses service override": {
			proxyConfig: map[string]interface{}{
				"max_request_headers_kb": int64(128),
			},
			serviceProxyConfig: map[string]interface{}{
				"max_request_headers_kb": int64(512),
			},
			protocol:    "grpc",
			wantPresent: true,
			wantValue:   512,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Create test service name
			svc := structs.NewServiceName("test-service", structs.DefaultEnterpriseMetaInDefaultPartition())

			// Create mock config snapshot using the test helper
			snap := proxycfg.TestConfigSnapshotPeeredMeshGateway(t, "default-services-http", func(ns *structs.NodeService) {
				ns.Proxy.Config = tc.proxyConfig
			}, nil)

			// Add service group with proxy config if provided
			if tc.serviceProxyConfig != nil {
				snap.MeshGateway.ServiceGroups[svc] = structs.CheckServiceNodes{
					{
						Service: &structs.NodeService{
							Proxy: structs.ConnectProxyConfig{
								Config: tc.serviceProxyConfig,
							},
						},
					},
				}
			}

			// Create mock discovery chain with a first node
			chain := &structs.CompiledDiscoveryChain{
				ServiceName: svc.Name,
				Namespace:   svc.NamespaceOrDefault(),
				Partition:   svc.PartitionOrDefault(),
				Datacenter:  "dc1",
				Protocol:    tc.protocol,
				StartNode:   "test-node",
				Nodes: map[string]*structs.DiscoveryGraphNode{
					"test-node": {
						Type: structs.DiscoveryGraphNodeTypeResolver,
						Name: "test-service.default.default.dc1",
						Resolver: &structs.DiscoveryResolver{
							Target: "test-service.default.default.dc1",
						},
					},
				},
				Targets: map[string]*structs.DiscoveryTarget{
					"test-service.default.default.dc1": {
						ID:         "test-service.default.default.dc1",
						Service:    svc.Name,
						Namespace:  svc.NamespaceOrDefault(),
						Partition:  svc.PartitionOrDefault(),
						Datacenter: "dc1",
						SNI:        "test-service.default.default.dc1.internal.test-domain.consul",
						Name:       "test-service.default.default.dc1.internal.test-domain.consul",
					},
				},
			}

			// Create resource generator
			s := &ResourceGenerator{
				Logger: hclog.NewNullLogger(),
			}

			// Call the function under test
			filterChain, err := s.makeMeshGatewayPeerFilterChain(snap, svc, []string{"test-peer"}, chain)

			require.NoError(t, err)
			require.NotNil(t, filterChain)
			require.Len(t, filterChain.Filters, 1)

			filter := filterChain.Filters[0]

			if tc.protocol == "tcp" {
				// For TCP, verify we don't have HTTP connection manager
				require.Contains(t, filter.Name, "tcp_proxy")
				return
			}

			// For HTTP-like protocols, check the HTTP connection manager
			require.Contains(t, filter.Name, "http_connection_manager")
			var httpConnMgr envoy_http_v3.HttpConnectionManager
			err = filter.GetTypedConfig().UnmarshalTo(&httpConnMgr)
			require.NoError(t, err)

			if tc.wantPresent {
				require.NotNil(t, httpConnMgr.MaxRequestHeadersKb, "expected MaxRequestHeadersKb to be set but it was nil")
				require.Equal(t, tc.wantValue, httpConnMgr.MaxRequestHeadersKb.GetValue())
			} else {
				require.Nil(t, httpConnMgr.MaxRequestHeadersKb, "expected MaxRequestHeadersKb to be nil but it was set")
			}
		})
	}
}

// Helper function for uint32 pointers
func uintPointer(i uint32) *uint32 {
	return &i
}
