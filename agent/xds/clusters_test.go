// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"bytes"
	"path/filepath"
	"testing"
	"text/template"
	"time"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_tls_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	"github.com/hashicorp/go-hclog"
	testinf "github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
)

type mockCfgFetcher struct {
	addressLan string
}

func (s *mockCfgFetcher) AdvertiseAddrLAN() string {
	return s.addressLan
}

func uint32ptr(i uint32) *uint32 {
	return &i
}

func durationPtr(d time.Duration) *time.Duration {
	return &d
}

type customClusterJSONOptions struct {
	Name       string
	TLSContext string
}

var customAppClusterJSONTpl = `{
	"@type": "type.googleapis.com/envoy.config.cluster.v3.Cluster",
	{{ if .TLSContext -}}
	"transport_socket": {
		"name": "tls",
		"typed_config": {
			"@type": "type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.UpstreamTlsContext",
			{{ .TLSContext }}
		}
	},
	{{- end }}
	"name": "{{ .Name }}",
	"connectTimeout": "15s",
	"loadAssignment": {
		"clusterName": "{{ .Name }}",
		"endpoints": [
			{
				"lbEndpoints": [
					{
						"endpoint": {
							"address": {
								"socketAddress": {
									"address": "127.0.0.1",
									"portValue": 8080
								}
							}
						}
					}
				]
			}
		]
	}
}`

var customAppClusterJSONTemplate = template.Must(template.New("").Parse(customAppClusterJSONTpl))

func customAppClusterJSON(t testinf.T, opts customClusterJSONOptions) string {
	t.Helper()
	var buf bytes.Buffer
	err := customAppClusterJSONTemplate.Execute(&buf, opts)
	require.NoError(t, err)
	return buf.String()
}

var customClusterJSONTpl = `{
	"@type": "type.googleapis.com/envoy.config.cluster.v3.Cluster",
	"name": "{{ .Name }}",
	"connectTimeout": "15s",
	"loadAssignment": {
		"clusterName": "{{ .Name }}",
		"endpoints": [
			{
				"lbEndpoints": [
					{
						"endpoint": {
							"address": {
								"socketAddress": {
									"address": "1.2.3.4",
									"portValue": 8443
								}
							}
						}
					}
				]
			}
		]
	}
}`

var customClusterJSONTemplate = template.Must(template.New("").Parse(customClusterJSONTpl))

func customClusterJSON(t testinf.T, opts customClusterJSONOptions) string {
	t.Helper()
	var buf bytes.Buffer
	err := customClusterJSONTemplate.Execute(&buf, opts)
	require.NoError(t, err)
	return buf.String()
}

func TestEnvoyLBConfig_InjectToCluster(t *testing.T) {
	var tests = []struct {
		name     string
		lb       *structs.LoadBalancer
		expected *envoy_cluster_v3.Cluster
	}{
		{
			name: "skip empty",
			lb: &structs.LoadBalancer{
				Policy: "",
			},
			expected: &envoy_cluster_v3.Cluster{},
		},
		{
			name: "round robin",
			lb: &structs.LoadBalancer{
				Policy: structs.LBPolicyRoundRobin,
			},
			expected: &envoy_cluster_v3.Cluster{LbPolicy: envoy_cluster_v3.Cluster_ROUND_ROBIN},
		},
		{
			name: "random",
			lb: &structs.LoadBalancer{
				Policy: structs.LBPolicyRandom,
			},
			expected: &envoy_cluster_v3.Cluster{LbPolicy: envoy_cluster_v3.Cluster_RANDOM},
		},
		{
			name: "maglev",
			lb: &structs.LoadBalancer{
				Policy: structs.LBPolicyMaglev,
			},
			expected: &envoy_cluster_v3.Cluster{LbPolicy: envoy_cluster_v3.Cluster_MAGLEV},
		},
		{
			name: "ring_hash",
			lb: &structs.LoadBalancer{
				Policy: structs.LBPolicyRingHash,
				RingHashConfig: &structs.RingHashConfig{
					MinimumRingSize: 3,
					MaximumRingSize: 7,
				},
			},
			expected: &envoy_cluster_v3.Cluster{
				LbPolicy: envoy_cluster_v3.Cluster_RING_HASH,
				LbConfig: &envoy_cluster_v3.Cluster_RingHashLbConfig_{
					RingHashLbConfig: &envoy_cluster_v3.Cluster_RingHashLbConfig{
						MinimumRingSize: &wrapperspb.UInt64Value{Value: 3},
						MaximumRingSize: &wrapperspb.UInt64Value{Value: 7},
					},
				},
			},
		},
		{
			name: "least_request",
			lb: &structs.LoadBalancer{
				Policy: "least_request",
				LeastRequestConfig: &structs.LeastRequestConfig{
					ChoiceCount: 3,
				},
			},
			expected: &envoy_cluster_v3.Cluster{
				LbPolicy: envoy_cluster_v3.Cluster_LEAST_REQUEST,
				LbConfig: &envoy_cluster_v3.Cluster_LeastRequestLbConfig_{
					LeastRequestLbConfig: &envoy_cluster_v3.Cluster_LeastRequestLbConfig{
						ChoiceCount: &wrapperspb.UInt32Value{Value: 3},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var c envoy_cluster_v3.Cluster
			err := injectLBToCluster(tc.lb, &c)
			require.NoError(t, err)

			require.Equal(t, tc.expected, &c)
		})
	}
}

func TestMakeJWTProviderCluster(t *testing.T) {
	// All tests here depend on golden files located under: agent/xds/testdata/jwt_authn_cluster/*
	tests := map[string]struct {
		provider      *structs.JWTProviderConfigEntry
		expectedError string
	}{
		"remote-jwks-not-configured": {
			provider: &structs.JWTProviderConfigEntry{
				Kind:          "jwt-provider",
				Name:          "okta",
				JSONWebKeySet: &structs.JSONWebKeySet{},
			},
			expectedError: "cannot create JWKS cluster for non remote JWKS. Provider Name: okta",
		},
		"local-jwks-configured": {
			provider: &structs.JWTProviderConfigEntry{
				Kind: "jwt-provider",
				Name: "okta",
				JSONWebKeySet: &structs.JSONWebKeySet{
					Local: &structs.LocalJWKS{
						Filename: "filename",
					},
				},
			},
			expectedError: "cannot create JWKS cluster for non remote JWKS. Provider Name: okta",
		},
		"https-provider-with-hostname-no-port-with-sni": {
			provider: makeTestProviderWithJWKS("https://example-okta.com/.well-known/jwks.json", true),
		},
		"https-provider-with-hostname-no-port": {
			provider: makeTestProviderWithJWKS("https://example-okta.com/.well-known/jwks.json", false),
		},
		"http-provider-with-hostname-no-port": {
			provider: makeTestProviderWithJWKS("http://example-okta.com/.well-known/jwks.json", true),
		},
		"http-provider-with-hostname-no-port-with-sni": {
			provider: makeTestProviderWithJWKS("http://example-okta.com/.well-known/jwks.json", true),
		},
		"https-provider-with-hostname-and-port": {
			provider: makeTestProviderWithJWKS("https://example-okta.com:90/.well-known/jwks.json", false),
		},
		"http-provider-with-hostname-and-port": {
			provider: makeTestProviderWithJWKS("http://example-okta.com:90/.well-known/jwks.json", false),
		},
		"http-provider-with-hostname-and-port-with-sni": {
			provider: makeTestProviderWithJWKS("http://example-okta.com:90/.well-known/jwks.json", true),
		},
		"https-provider-with-ip-no-port-with-sni": {
			provider: makeTestProviderWithJWKS("https://127.0.0.1", true),
		},
		"https-provider-with-ip-no-port": {
			provider: makeTestProviderWithJWKS("https://127.0.0.1", false),
		},
		"http-provider-with-ip-no-port": {
			provider: makeTestProviderWithJWKS("http://127.0.0.1", false),
		},
		"https-provider-with-ip-and-port-with-sni": {
			provider: makeTestProviderWithJWKS("https://127.0.0.1:9091", true),
		},
		"https-provider-with-ip-and-port": {
			provider: makeTestProviderWithJWKS("https://127.0.0.1:9091", false),
		},
		"http-provider-with-ip-and-port": {
			provider: makeTestProviderWithJWKS("http://127.0.0.1:9091", true),
		},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			cluster, err := makeJWTProviderCluster(tt.provider)
			if tt.expectedError != "" {
				require.Error(t, err, tt.expectedError)
			} else {
				require.NoError(t, err)
				gotJSON := protoToJSON(t, cluster)
				require.JSONEq(t, goldenSimple(t, filepath.Join("jwt_authn_clusters", name), gotJSON), gotJSON)
			}

		})
	}
}

func makeTestProviderWithJWKS(uri string, useSNI bool) *structs.JWTProviderConfigEntry {
	return &structs.JWTProviderConfigEntry{
		Kind:   "jwt-provider",
		Name:   "okta",
		Issuer: "test-issuer",
		JSONWebKeySet: &structs.JSONWebKeySet{
			Remote: &structs.RemoteJWKS{
				RequestTimeoutMs:    1000,
				FetchAsynchronously: true,
				URI:                 uri,
				UseSNI:              useSNI,
				JWKSCluster: &structs.JWKSCluster{
					DiscoveryType:  structs.DiscoveryTypeStatic,
					ConnectTimeout: time.Duration(5) * time.Second,
					TLSCertificates: &structs.JWKSTLSCertificate{
						TrustedCA: &structs.JWKSTLSCertTrustedCA{
							Filename: "mycert.crt",
						},
					},
				},
			},
		},
	}
}

func TestMakeJWKSDiscoveryClusterType(t *testing.T) {
	tests := map[string]struct {
		remoteJWKS          *structs.RemoteJWKS
		expectedClusterType *envoy_cluster_v3.Cluster_Type
	}{
		"nil remote jwks": {
			remoteJWKS:          nil,
			expectedClusterType: &envoy_cluster_v3.Cluster_Type{},
		},
		"nil jwks cluster": {
			remoteJWKS:          &structs.RemoteJWKS{},
			expectedClusterType: &envoy_cluster_v3.Cluster_Type{},
		},
		"jwks cluster defaults to Strict DNS": {
			remoteJWKS: &structs.RemoteJWKS{
				JWKSCluster: &structs.JWKSCluster{},
			},
			expectedClusterType: &envoy_cluster_v3.Cluster_Type{
				Type: envoy_cluster_v3.Cluster_STRICT_DNS,
			},
		},
		"jwks with cluster EDS": {
			remoteJWKS: &structs.RemoteJWKS{
				JWKSCluster: &structs.JWKSCluster{
					DiscoveryType: "EDS",
				},
			},
			expectedClusterType: &envoy_cluster_v3.Cluster_Type{
				Type: envoy_cluster_v3.Cluster_EDS,
			},
		},
		"jwks with static dns": {
			remoteJWKS: &structs.RemoteJWKS{
				JWKSCluster: &structs.JWKSCluster{
					DiscoveryType: "STATIC",
				},
			},
			expectedClusterType: &envoy_cluster_v3.Cluster_Type{
				Type: envoy_cluster_v3.Cluster_STATIC,
			},
		},

		"jwks with original dst": {
			remoteJWKS: &structs.RemoteJWKS{
				JWKSCluster: &structs.JWKSCluster{
					DiscoveryType: "ORIGINAL_DST",
				},
			},
			expectedClusterType: &envoy_cluster_v3.Cluster_Type{
				Type: envoy_cluster_v3.Cluster_ORIGINAL_DST,
			},
		},
		"jwks with strict dns": {
			remoteJWKS: &structs.RemoteJWKS{
				JWKSCluster: &structs.JWKSCluster{
					DiscoveryType: "STRICT_DNS",
				},
			},
			expectedClusterType: &envoy_cluster_v3.Cluster_Type{
				Type: envoy_cluster_v3.Cluster_STRICT_DNS,
			},
		},
		"jwks with logical dns": {
			remoteJWKS: &structs.RemoteJWKS{
				JWKSCluster: &structs.JWKSCluster{
					DiscoveryType: "LOGICAL_DNS",
				},
			},
			expectedClusterType: &envoy_cluster_v3.Cluster_Type{
				Type: envoy_cluster_v3.Cluster_LOGICAL_DNS,
			},
		},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			clusterType := makeJWKSDiscoveryClusterType(tt.remoteJWKS)

			require.Equal(t, tt.expectedClusterType, clusterType)
		})
	}
}

func TestMakeJWKSClusterDNSLookupFamilyType(t *testing.T) {
	tests := map[string]struct {
		clusterType             *envoy_cluster_v3.Cluster_Type
		expectedDNSLookupFamily envoy_cluster_v3.Cluster_DnsLookupFamily
	}{
		// strict dns and logical dns are the only ones that are different
		"jwks with strict dns": {
			clusterType: &envoy_cluster_v3.Cluster_Type{
				Type: envoy_cluster_v3.Cluster_STRICT_DNS,
			},
			expectedDNSLookupFamily: envoy_cluster_v3.Cluster_V4_PREFERRED,
		},
		"jwks with logical dns": {
			clusterType: &envoy_cluster_v3.Cluster_Type{
				Type: envoy_cluster_v3.Cluster_LOGICAL_DNS,
			},
			expectedDNSLookupFamily: envoy_cluster_v3.Cluster_ALL,
		},
		// all should be auto from here down
		"jwks with cluster EDS": {
			clusterType: &envoy_cluster_v3.Cluster_Type{
				Type: envoy_cluster_v3.Cluster_EDS,
			},
			expectedDNSLookupFamily: envoy_cluster_v3.Cluster_AUTO,
		},
		"jwks with static dns": {
			clusterType: &envoy_cluster_v3.Cluster_Type{
				Type: envoy_cluster_v3.Cluster_STATIC,
			},
			expectedDNSLookupFamily: envoy_cluster_v3.Cluster_AUTO,
		},

		"jwks with original dst": {
			clusterType: &envoy_cluster_v3.Cluster_Type{
				Type: envoy_cluster_v3.Cluster_ORIGINAL_DST,
			},
			expectedDNSLookupFamily: envoy_cluster_v3.Cluster_AUTO,
		},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			actualDNSLookupFamily := makeJWKSClusterDNSLookupFamilyType(tt.clusterType)

			require.Equal(t, tt.expectedDNSLookupFamily, actualDNSLookupFamily)
		})
	}
}

func TestParseJWTRemoteURL(t *testing.T) {
	tests := map[string]struct {
		uri            string
		expectedHost   string
		expectedPort   int
		expectedScheme string
		expectError    bool
	}{
		"invalid-url": {
			uri:         ".com",
			expectError: true,
		},
		"https-hostname-no-port": {
			uri:            "https://test.test.com",
			expectedHost:   "test.test.com",
			expectedPort:   443,
			expectedScheme: "https",
		},
		"https-hostname-with-port": {
			uri:            "https://test.test.com:4545",
			expectedHost:   "test.test.com",
			expectedPort:   4545,
			expectedScheme: "https",
		},
		"https-hostname-with-port-and-path": {
			uri:            "https://test.test.com:4545/test",
			expectedHost:   "test.test.com",
			expectedPort:   4545,
			expectedScheme: "https",
		},
		"http-hostname-no-port": {
			uri:            "http://test.test.com",
			expectedHost:   "test.test.com",
			expectedPort:   80,
			expectedScheme: "http",
		},
		"http-hostname-with-port": {
			uri:            "http://test.test.com:4636",
			expectedHost:   "test.test.com",
			expectedPort:   4636,
			expectedScheme: "http",
		},
		"https-ip-no-port": {
			uri:            "https://127.0.0.1",
			expectedHost:   "127.0.0.1",
			expectedPort:   443,
			expectedScheme: "https",
		},
		"https-ip-with-port": {
			uri:            "https://127.0.0.1:3434",
			expectedHost:   "127.0.0.1",
			expectedPort:   3434,
			expectedScheme: "https",
		},
		"http-ip-no-port": {
			uri:            "http://127.0.0.1",
			expectedHost:   "127.0.0.1",
			expectedPort:   80,
			expectedScheme: "http",
		},
		"http-ip-with-port": {
			uri:            "http://127.0.0.1:9190",
			expectedHost:   "127.0.0.1",
			expectedPort:   9190,
			expectedScheme: "http",
		},
		"http-ip-with-port-and-path": {
			uri:            "http://127.0.0.1:9190/some/where",
			expectedHost:   "127.0.0.1",
			expectedPort:   9190,
			expectedScheme: "http",
		},
		"http-ip-no-port-with-path": {
			uri:            "http://127.0.0.1/test/path",
			expectedHost:   "127.0.0.1",
			expectedPort:   80,
			expectedScheme: "http",
		},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			host, scheme, port, err := parseJWTRemoteURL(tt.uri)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, host, tt.expectedHost)
				require.Equal(t, scheme, tt.expectedScheme)
				require.Equal(t, port, tt.expectedPort)
			}
		})
	}
}

// UID is just a convenience function to aid in writing tests less verbosely.
func UID(input string) proxycfg.UpstreamID {
	return proxycfg.UpstreamIDFromString(input)
}

func TestMakeJWTCertValidationContext(t *testing.T) {
	tests := map[string]struct {
		jwksCluster *structs.JWKSCluster
		expected    *envoy_tls_v3.CertificateValidationContext
	}{
		"when nil": {
			jwksCluster: nil,
			expected:    &envoy_tls_v3.CertificateValidationContext{},
		},
		"when trustedCA with filename": {
			jwksCluster: &structs.JWKSCluster{
				TLSCertificates: &structs.JWKSTLSCertificate{
					TrustedCA: &structs.JWKSTLSCertTrustedCA{
						Filename: "file.crt",
					},
				},
			},
			expected: &envoy_tls_v3.CertificateValidationContext{
				TrustedCa: &envoy_core_v3.DataSource{
					Specifier: &envoy_core_v3.DataSource_Filename{
						Filename: "file.crt",
					},
				},
			},
		},
		"when trustedCA with environment variable": {
			jwksCluster: &structs.JWKSCluster{
				TLSCertificates: &structs.JWKSTLSCertificate{
					TrustedCA: &structs.JWKSTLSCertTrustedCA{
						EnvironmentVariable: "MY_ENV",
					},
				},
			},
			expected: &envoy_tls_v3.CertificateValidationContext{
				TrustedCa: &envoy_core_v3.DataSource{
					Specifier: &envoy_core_v3.DataSource_EnvironmentVariable{
						EnvironmentVariable: "MY_ENV",
					},
				},
			},
		},
		"when trustedCA with inline string": {
			jwksCluster: &structs.JWKSCluster{
				TLSCertificates: &structs.JWKSTLSCertificate{
					TrustedCA: &structs.JWKSTLSCertTrustedCA{
						InlineString: "<my ca cert>",
					},
				},
			},
			expected: &envoy_tls_v3.CertificateValidationContext{
				TrustedCa: &envoy_core_v3.DataSource{
					Specifier: &envoy_core_v3.DataSource_InlineString{
						InlineString: "<my ca cert>",
					},
				},
			},
		},
		"when trustedCA with inline bytes": {
			jwksCluster: &structs.JWKSCluster{
				TLSCertificates: &structs.JWKSTLSCertificate{
					TrustedCA: &structs.JWKSTLSCertTrustedCA{
						InlineBytes: []byte{1, 2, 3},
					},
				},
			},
			expected: &envoy_tls_v3.CertificateValidationContext{
				TrustedCa: &envoy_core_v3.DataSource{
					Specifier: &envoy_core_v3.DataSource_InlineBytes{
						InlineBytes: []byte{1, 2, 3},
					},
				},
			},
		},
		"when caCertificateProviderInstance": {
			jwksCluster: &structs.JWKSCluster{
				TLSCertificates: &structs.JWKSTLSCertificate{
					CaCertificateProviderInstance: &structs.JWKSTLSCertProviderInstance{
						InstanceName:    "<my-instance-name>",
						CertificateName: "<my-cert>.crt",
					},
				},
			},
			expected: &envoy_tls_v3.CertificateValidationContext{
				CaCertificateProviderInstance: &envoy_tls_v3.CertificateProviderPluginInstance{
					InstanceName:    "<my-instance-name>",
					CertificateName: "<my-cert>.crt",
				},
			},
		},
	}
	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			vc := makeJWTCertValidationContext(tt.jwksCluster)

			require.Equal(t, tt.expected, vc)
		})
	}
}

func TestInjectGatewayServiceAddons_TerminatingGateway_NoCAFile(t *testing.T) {
	s := &ResourceGenerator{Logger: hclog.NewNullLogger()}
	svc := structs.NewServiceName("web", structs.DefaultEnterpriseMetaInDefaultPartition())
	snap := proxycfg.TestConfigSnapshotTerminatingGateway(t, true, nil, nil)
	snap.TerminatingGateway.GatewayServices = map[structs.ServiceName]structs.GatewayService{
		svc: {Service: svc},
	}

	c := &envoy_cluster_v3.Cluster{Name: "web"}
	err := s.injectGatewayServiceAddons(snap, c, svc, &structs.LoadBalancer{})

	require.NoError(t, err)
	require.Nil(t, c.TransportSocket)
}

func TestInjectGatewayServiceAddons_TerminatingGateway_WithCAFile(t *testing.T) {
	s := &ResourceGenerator{Logger: hclog.NewNullLogger()}
	svc := structs.NewServiceName("web", structs.DefaultEnterpriseMetaInDefaultPartition())
	snap := proxycfg.TestConfigSnapshotTerminatingGateway(t, true, nil, nil)
	snap.TerminatingGateway.GatewayServices = map[structs.ServiceName]structs.GatewayService{
		svc: {Service: svc, CAFile: "ca.pem"},
	}

	c := &envoy_cluster_v3.Cluster{Name: "web"}
	err := s.injectGatewayServiceAddons(snap, c, svc, &structs.LoadBalancer{})

	require.NoError(t, err)
	require.NotNil(t, c.TransportSocket)
	require.Equal(t, "tls", c.TransportSocket.Name)
}

func TestInjectGatewayServiceAddons_TerminatingGateway_WithCAFileAndSNI_UsesCombinedValidationContext(t *testing.T) {
	s := &ResourceGenerator{Logger: hclog.NewNullLogger()}
	svc := structs.NewServiceName("web", structs.DefaultEnterpriseMetaInDefaultPartition())
	snap := proxycfg.TestConfigSnapshotTerminatingGateway(t, true, nil, nil)
	snap.TerminatingGateway.GatewayServices = map[structs.ServiceName]structs.GatewayService{
		svc: {Service: svc, CAFile: "ca.pem", SNI: "web.example.com"},
	}

	c := &envoy_cluster_v3.Cluster{Name: "web"}
	err := s.injectGatewayServiceAddons(snap, c, svc, &structs.LoadBalancer{})

	require.NoError(t, err)
	require.NotNil(t, c.TransportSocket)

	upstreamTLS := &envoy_tls_v3.UpstreamTlsContext{}
	err = c.TransportSocket.GetTypedConfig().UnmarshalTo(upstreamTLS)
	require.NoError(t, err)
	require.Equal(t, "web.example.com", upstreamTLS.Sni)

	combined, ok := upstreamTLS.CommonTlsContext.ValidationContextType.(*envoy_tls_v3.CommonTlsContext_CombinedValidationContext)
	require.True(t, ok, "expected CombinedValidationContext, got %T", upstreamTLS.CommonTlsContext.ValidationContextType)
	require.Equal(t, "web-ca", combined.CombinedValidationContext.ValidationContextSdsSecretConfig.Name)
	require.NotEmpty(t, combined.CombinedValidationContext.DefaultValidationContext.MatchTypedSubjectAltNames)
}

func TestInjectGatewayServiceAddons_TerminatingGateway_SNIWithSDSContextUsesCombinedValidationContext(t *testing.T) {
	s := &ResourceGenerator{Logger: hclog.NewNullLogger()}
	svc := structs.NewServiceName("api", structs.DefaultEnterpriseMetaInDefaultPartition())
	snap := proxycfg.TestConfigSnapshotTerminatingGateway(t, true, nil, nil)
	snap.TerminatingGateway.GatewayServices = map[structs.ServiceName]structs.GatewayService{
		svc: {Service: svc, CAFile: "ca.pem", SNI: "api.example.com"},
	}

	c := &envoy_cluster_v3.Cluster{Name: "api"}
	err := s.injectGatewayServiceAddons(snap, c, svc, &structs.LoadBalancer{})

	require.NoError(t, err)
	require.NotNil(t, c.TransportSocket)

	upstreamTLS := &envoy_tls_v3.UpstreamTlsContext{}
	err = c.TransportSocket.GetTypedConfig().UnmarshalTo(upstreamTLS)
	require.NoError(t, err)
	require.Equal(t, "api.example.com", upstreamTLS.Sni)

	combined, ok := upstreamTLS.CommonTlsContext.ValidationContextType.(*envoy_tls_v3.CommonTlsContext_CombinedValidationContext)
	require.True(t, ok, "expected CombinedValidationContext, got %T", upstreamTLS.CommonTlsContext.ValidationContextType)
	require.Equal(t, "api-ca", combined.CombinedValidationContext.ValidationContextSdsSecretConfig.Name)
	require.NotEmpty(t, combined.CombinedValidationContext.DefaultValidationContext.MatchTypedSubjectAltNames)
}

func TestInjectGatewayServiceAddons_TerminatingGateway_NoSNINoSANMatchers(t *testing.T) {
	s := &ResourceGenerator{Logger: hclog.NewNullLogger()}
	svc := structs.NewServiceName("db", structs.DefaultEnterpriseMetaInDefaultPartition())
	snap := proxycfg.TestConfigSnapshotTerminatingGateway(t, true, nil, nil)
	snap.TerminatingGateway.GatewayServices = map[structs.ServiceName]structs.GatewayService{
		svc: {Service: svc, CAFile: "ca.pem"},
	}

	c := &envoy_cluster_v3.Cluster{Name: "db"}
	err := s.injectGatewayServiceAddons(snap, c, svc, &structs.LoadBalancer{})

	require.NoError(t, err)
	upstreamTLS := &envoy_tls_v3.UpstreamTlsContext{}
	err = c.TransportSocket.GetTypedConfig().UnmarshalTo(upstreamTLS)
	require.NoError(t, err)
	require.Equal(t, "", upstreamTLS.Sni)

	sdsCfg, ok := upstreamTLS.CommonTlsContext.ValidationContextType.(*envoy_tls_v3.CommonTlsContext_ValidationContextSdsSecretConfig)
	require.True(t, ok)
	require.Equal(t, "db-ca", sdsCfg.ValidationContextSdsSecretConfig.Name)
}

func TestInjectGatewayServiceAddons_TerminatingGateway_TLSContextUsesSDS(t *testing.T) {
	s := &ResourceGenerator{Logger: hclog.NewNullLogger()}
	svc := structs.NewServiceName("payments", structs.DefaultEnterpriseMetaInDefaultPartition())
	snap := proxycfg.TestConfigSnapshotTerminatingGateway(t, true, nil, nil)
	snap.TerminatingGateway.GatewayServices = map[structs.ServiceName]structs.GatewayService{
		svc: {Service: svc, CAFile: "ca.pem", CertFile: "cert.pem", KeyFile: "key.pem"},
	}

	c := &envoy_cluster_v3.Cluster{Name: "payments"}
	err := s.injectGatewayServiceAddons(snap, c, svc, &structs.LoadBalancer{})

	require.NoError(t, err)
	upstreamTLS := &envoy_tls_v3.UpstreamTlsContext{}
	err = c.TransportSocket.GetTypedConfig().UnmarshalTo(upstreamTLS)
	require.NoError(t, err)

	require.Len(t, upstreamTLS.CommonTlsContext.TlsCertificateSdsSecretConfigs, 1)
	certSDS := upstreamTLS.CommonTlsContext.TlsCertificateSdsSecretConfigs[0]
	require.Equal(t, "payments-cert", certSDS.Name)
	_, usesADS := certSDS.SdsConfig.ConfigSourceSpecifier.(*envoy_core_v3.ConfigSource_Ads)
	require.True(t, usesADS)
	require.Equal(t, envoy_core_v3.ApiVersion_V3, certSDS.SdsConfig.ResourceApiVersion)
}

func TestInjectGatewayServiceAddons_TerminatingGateway_ServiceNotInMap(t *testing.T) {
	s := &ResourceGenerator{Logger: hclog.NewNullLogger()}
	svc := structs.NewServiceName("unknown", structs.DefaultEnterpriseMetaInDefaultPartition())
	snap := proxycfg.TestConfigSnapshotTerminatingGateway(t, true, nil, nil)
	snap.TerminatingGateway.GatewayServices = map[structs.ServiceName]structs.GatewayService{}

	c := &envoy_cluster_v3.Cluster{Name: "unknown"}
	err := s.injectGatewayServiceAddons(snap, c, svc, &structs.LoadBalancer{})

	require.NoError(t, err)
	require.Nil(t, c.TransportSocket)
}

func TestInjectGatewayServiceAddons_MeshGateway_DoesNotSetTransportSocket(t *testing.T) {
	s := &ResourceGenerator{Logger: hclog.NewNullLogger()}
	svc := structs.NewServiceName("web", structs.DefaultEnterpriseMetaInDefaultPartition())
	snap := proxycfg.TestConfigSnapshotTerminatingGateway(t, true, nil, nil)
	snap.Kind = structs.ServiceKindMeshGateway

	c := &envoy_cluster_v3.Cluster{Name: "web"}
	err := s.injectGatewayServiceAddons(snap, c, svc, &structs.LoadBalancer{})

	require.NoError(t, err)
	require.Nil(t, c.TransportSocket)
}

func TestInjectGatewayDestinationAddons_TerminatingGateway_NoCAFile(t *testing.T) {
	s := &ResourceGenerator{Logger: hclog.NewNullLogger()}
	svc := structs.NewServiceName("db", structs.DefaultEnterpriseMetaInDefaultPartition())
	snap := proxycfg.TestConfigSnapshotTerminatingGateway(t, true, nil, nil)
	snap.TerminatingGateway.DestinationServices = map[structs.ServiceName]structs.GatewayService{
		svc: {Service: svc},
	}

	c := &envoy_cluster_v3.Cluster{Name: "db"}
	err := s.injectGatewayDestinationAddons(snap, c, svc)

	require.NoError(t, err)
	require.Nil(t, c.TransportSocket)
}

func TestInjectGatewayDestinationAddons_TerminatingGateway_WithCAFile(t *testing.T) {
	s := &ResourceGenerator{Logger: hclog.NewNullLogger()}
	svc := structs.NewServiceName("db", structs.DefaultEnterpriseMetaInDefaultPartition())
	snap := proxycfg.TestConfigSnapshotTerminatingGateway(t, true, nil, nil)
	snap.TerminatingGateway.DestinationServices = map[structs.ServiceName]structs.GatewayService{
		svc: {Service: svc, CAFile: "ca.pem"},
	}

	c := &envoy_cluster_v3.Cluster{Name: "db"}
	err := s.injectGatewayDestinationAddons(snap, c, svc)

	require.NoError(t, err)
	require.NotNil(t, c.TransportSocket)
	require.Equal(t, "tls", c.TransportSocket.Name)
}

func TestInjectGatewayDestinationAddons_TerminatingGateway_WithCAFileAndSNI_UsesCombinedValidationContext(t *testing.T) {
	s := &ResourceGenerator{Logger: hclog.NewNullLogger()}
	svc := structs.NewServiceName("db", structs.DefaultEnterpriseMetaInDefaultPartition())
	snap := proxycfg.TestConfigSnapshotTerminatingGateway(t, true, nil, nil)
	snap.TerminatingGateway.DestinationServices = map[structs.ServiceName]structs.GatewayService{
		svc: {Service: svc, CAFile: "ca.pem", SNI: "db.example.com"},
	}

	c := &envoy_cluster_v3.Cluster{Name: "db"}
	err := s.injectGatewayDestinationAddons(snap, c, svc)

	require.NoError(t, err)
	require.NotNil(t, c.TransportSocket)

	upstreamTLS := &envoy_tls_v3.UpstreamTlsContext{}
	err = c.TransportSocket.GetTypedConfig().UnmarshalTo(upstreamTLS)
	require.NoError(t, err)
	require.Equal(t, "db.example.com", upstreamTLS.Sni)

	combined, ok := upstreamTLS.CommonTlsContext.ValidationContextType.(*envoy_tls_v3.CommonTlsContext_CombinedValidationContext)
	require.True(t, ok, "expected CombinedValidationContext, got %T", upstreamTLS.CommonTlsContext.ValidationContextType)
	require.Equal(t, "db-ca", combined.CombinedValidationContext.ValidationContextSdsSecretConfig.Name)
	require.NotEmpty(t, combined.CombinedValidationContext.DefaultValidationContext.MatchTypedSubjectAltNames)
}

func TestInjectGatewayDestinationAddons_TerminatingGateway_SNIWithSDSContextUsesCombinedValidationContext(t *testing.T) {
	s := &ResourceGenerator{Logger: hclog.NewNullLogger()}
	svc := structs.NewServiceName("cache", structs.DefaultEnterpriseMetaInDefaultPartition())
	snap := proxycfg.TestConfigSnapshotTerminatingGateway(t, true, nil, nil)
	snap.TerminatingGateway.DestinationServices = map[structs.ServiceName]structs.GatewayService{
		svc: {Service: svc, CAFile: "ca.pem", SNI: "cache.example.com"},
	}

	c := &envoy_cluster_v3.Cluster{Name: "cache"}
	err := s.injectGatewayDestinationAddons(snap, c, svc)

	require.NoError(t, err)
	require.NotNil(t, c.TransportSocket)

	upstreamTLS := &envoy_tls_v3.UpstreamTlsContext{}
	err = c.TransportSocket.GetTypedConfig().UnmarshalTo(upstreamTLS)
	require.NoError(t, err)
	require.Equal(t, "cache.example.com", upstreamTLS.Sni)

	combined, ok := upstreamTLS.CommonTlsContext.ValidationContextType.(*envoy_tls_v3.CommonTlsContext_CombinedValidationContext)
	require.True(t, ok, "expected CombinedValidationContext, got %T", upstreamTLS.CommonTlsContext.ValidationContextType)
	require.Equal(t, "cache-ca", combined.CombinedValidationContext.ValidationContextSdsSecretConfig.Name)
	require.NotEmpty(t, combined.CombinedValidationContext.DefaultValidationContext.MatchTypedSubjectAltNames)
}

func TestInjectGatewayDestinationAddons_TerminatingGateway_TLSContextUsesSDS(t *testing.T) {
	s := &ResourceGenerator{Logger: hclog.NewNullLogger()}
	svc := structs.NewServiceName("cache", structs.DefaultEnterpriseMetaInDefaultPartition())
	snap := proxycfg.TestConfigSnapshotTerminatingGateway(t, true, nil, nil)
	snap.TerminatingGateway.DestinationServices = map[structs.ServiceName]structs.GatewayService{
		svc: {Service: svc, CAFile: "ca.pem"},
	}

	c := &envoy_cluster_v3.Cluster{Name: "cache"}
	err := s.injectGatewayDestinationAddons(snap, c, svc)

	require.NoError(t, err)
	upstreamTLS := &envoy_tls_v3.UpstreamTlsContext{}
	err = c.TransportSocket.GetTypedConfig().UnmarshalTo(upstreamTLS)
	require.NoError(t, err)

	require.Len(t, upstreamTLS.CommonTlsContext.TlsCertificateSdsSecretConfigs, 1)
	certSDS := upstreamTLS.CommonTlsContext.TlsCertificateSdsSecretConfigs[0]
	require.Equal(t, "cache-cert", certSDS.Name)
	_, usesADS := certSDS.SdsConfig.ConfigSourceSpecifier.(*envoy_core_v3.ConfigSource_Ads)
	require.True(t, usesADS)
	require.Equal(t, envoy_core_v3.ApiVersion_V3, certSDS.SdsConfig.ResourceApiVersion)

	sdsCACfg, ok := upstreamTLS.CommonTlsContext.ValidationContextType.(*envoy_tls_v3.CommonTlsContext_ValidationContextSdsSecretConfig)
	require.True(t, ok)
	require.Equal(t, "cache-ca", sdsCACfg.ValidationContextSdsSecretConfig.Name)
	_, caUsesADS := sdsCACfg.ValidationContextSdsSecretConfig.SdsConfig.ConfigSourceSpecifier.(*envoy_core_v3.ConfigSource_Ads)
	require.True(t, caUsesADS)
}

func TestInjectGatewayDestinationAddons_TerminatingGateway_DestinationNotInMap(t *testing.T) {
	s := &ResourceGenerator{Logger: hclog.NewNullLogger()}
	svc := structs.NewServiceName("missing", structs.DefaultEnterpriseMetaInDefaultPartition())
	snap := proxycfg.TestConfigSnapshotTerminatingGateway(t, true, nil, nil)
	snap.TerminatingGateway.DestinationServices = map[structs.ServiceName]structs.GatewayService{}

	c := &envoy_cluster_v3.Cluster{Name: "missing"}
	err := s.injectGatewayDestinationAddons(snap, c, svc)

	require.NoError(t, err)
	require.Nil(t, c.TransportSocket)
}

func TestInjectGatewayDestinationAddons_NonTerminatingGatewayKindDoesNothing(t *testing.T) {
	s := &ResourceGenerator{Logger: hclog.NewNullLogger()}
	svc := structs.NewServiceName("web", structs.DefaultEnterpriseMetaInDefaultPartition())
	snap := proxycfg.TestConfigSnapshotTerminatingGateway(t, true, nil, nil)
	snap.Kind = structs.ServiceKindMeshGateway
	snap.TerminatingGateway.DestinationServices = map[structs.ServiceName]structs.GatewayService{
		svc: {Service: svc, CAFile: "ca.pem"},
	}

	c := &envoy_cluster_v3.Cluster{Name: "web"}
	err := s.injectGatewayDestinationAddons(snap, c, svc)

	require.NoError(t, err)
	require.Nil(t, c.TransportSocket)
}
