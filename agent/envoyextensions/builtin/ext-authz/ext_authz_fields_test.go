// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package extauthz

import (
	"testing"
	"time"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_http_ext_authz_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_authz/v3"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/envoyextensions/extensioncommon"
)

// renderExtAuthz builds the extension from arguments (with the given Required
// flag) and returns the decoded Envoy ext_authz HTTP filter config.
func renderExtAuthz(t *testing.T, required bool, args map[string]any) *envoy_http_ext_authz_v3.ExtAuthz {
	t.Helper()
	ext, err := newExtAuthz(api.EnvoyExtension{
		Name:      api.BuiltinExtAuthzExtension,
		Required:  required,
		Arguments: args,
	})
	require.NoError(t, err)

	filter, err := ext.Config.toEnvoyHttpFilter(&extensioncommon.RuntimeConfig{})
	require.NoError(t, err)

	var got envoy_http_ext_authz_v3.ExtAuthz
	require.NoError(t, filter.GetTypedConfig().UnmarshalTo(&got))
	return &got
}

// grpcURIArgs builds connect-proxy ext-authz arguments with a localhost gRPC
// URI target, merging any extra top-level Config keys.
func grpcURIArgs(extra map[string]any) map[string]any {
	cfg := map[string]any{
		"GrpcService": map[string]any{
			"Target": map[string]any{"URI": "localhost:9191"},
		},
	}
	for k, v := range extra {
		cfg[k] = v
	}
	return map[string]any{"ProxyType": "connect-proxy", "Config": cfg}
}

// TestExtAuthz_FailureModeAllow verifies that the explicit Config.FailureModeAllow
// overrides the default derived from the extension's Required flag.
func TestExtAuthz_FailureModeAllow(t *testing.T) {
	t.Parallel()

	t.Run("default: required=true -> fail closed (false)", func(t *testing.T) {
		t.Parallel()
		got := renderExtAuthz(t, true, grpcURIArgs(nil))
		require.False(t, got.GetFailureModeAllow())
	})

	t.Run("default: required=false -> fail open (true)", func(t *testing.T) {
		t.Parallel()
		got := renderExtAuthz(t, false, grpcURIArgs(nil))
		require.True(t, got.GetFailureModeAllow())
	})

	t.Run("override true wins over required=true", func(t *testing.T) {
		t.Parallel()
		got := renderExtAuthz(t, true, grpcURIArgs(map[string]any{"FailureModeAllow": true}))
		require.True(t, got.GetFailureModeAllow(), "explicit Config.FailureModeAllow=true must override Required-derived default")
	})

	t.Run("override false wins over required=false", func(t *testing.T) {
		t.Parallel()
		got := renderExtAuthz(t, false, grpcURIArgs(map[string]any{"FailureModeAllow": false}))
		require.False(t, got.GetFailureModeAllow(), "explicit Config.FailureModeAllow=false must override Required-derived default")
	})
}

// TestExtAuthz_ServiceTimeout verifies the service-level Timeout field on both
// GrpcService and HttpService, including precedence over Target.Timeout.
func TestExtAuthz_ServiceTimeout(t *testing.T) {
	t.Parallel()

	t.Run("grpc service-level Timeout is used", func(t *testing.T) {
		t.Parallel()
		args := map[string]any{
			"ProxyType": "connect-proxy",
			"Config": map[string]any{
				"GrpcService": map[string]any{
					"Target":  map[string]any{"URI": "localhost:9191"},
					"Timeout": "5s",
				},
			},
		}
		got := renderExtAuthz(t, true, args)
		require.Equal(t, 5*time.Second, got.GetGrpcService().GetTimeout().AsDuration())
	})

	t.Run("grpc service-level Timeout takes precedence over Target.Timeout", func(t *testing.T) {
		t.Parallel()
		args := map[string]any{
			"ProxyType": "connect-proxy",
			"Config": map[string]any{
				"GrpcService": map[string]any{
					"Target":  map[string]any{"URI": "localhost:9191", "Timeout": "2s"},
					"Timeout": "5s",
				},
			},
		}
		got := renderExtAuthz(t, true, args)
		require.Equal(t, 5*time.Second, got.GetGrpcService().GetTimeout().AsDuration())
	})

	t.Run("grpc falls back to Target.Timeout when service Timeout unset", func(t *testing.T) {
		t.Parallel()
		args := map[string]any{
			"ProxyType": "connect-proxy",
			"Config": map[string]any{
				"GrpcService": map[string]any{
					"Target": map[string]any{"URI": "localhost:9191", "Timeout": "3s"},
				},
			},
		}
		got := renderExtAuthz(t, true, args)
		require.Equal(t, 3*time.Second, got.GetGrpcService().GetTimeout().AsDuration())
	})

	t.Run("http service-level Timeout is used on server_uri", func(t *testing.T) {
		t.Parallel()
		args := map[string]any{
			"ProxyType": "connect-proxy",
			"Config": map[string]any{
				"HttpService": map[string]any{
					"Target":  map[string]any{"URI": "localhost:4180"},
					"Timeout": "7s",
				},
			},
		}
		got := renderExtAuthz(t, true, args)
		require.Equal(t, 7*time.Second, got.GetHttpService().GetServerUri().GetTimeout().AsDuration())
	})

	t.Run("invalid grpc service Timeout is rejected", func(t *testing.T) {
		t.Parallel()
		_, err := newExtAuthz(api.EnvoyExtension{
			Name: api.BuiltinExtAuthzExtension,
			Arguments: map[string]any{
				"ProxyType": "connect-proxy",
				"Config": map[string]any{
					"GrpcService": map[string]any{
						"Target":  map[string]any{"URI": "localhost:9191"},
						"Timeout": "nope",
					},
				},
			},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to parse GrpcService.Timeout")
	})

	t.Run("invalid http service Timeout is rejected", func(t *testing.T) {
		t.Parallel()
		_, err := newExtAuthz(api.EnvoyExtension{
			Name: api.BuiltinExtAuthzExtension,
			Arguments: map[string]any{
				"ProxyType": "connect-proxy",
				"Config": map[string]any{
					"HttpService": map[string]any{
						"Target":  map[string]any{"URI": "localhost:4180"},
						"Timeout": "nope",
					},
				},
			},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to parse HttpService.Timeout")
	})
}

// TestExtAuthz_AllowedDisallowedHeaders verifies that the filter-level
// AllowedHeaders / DisallowedHeaders controls are rendered onto the Envoy
// ext_authz filter, and that they are omitted (nil) when unset.
func TestExtAuthz_AllowedDisallowedHeaders(t *testing.T) {
	t.Parallel()

	t.Run("unset leaves both nil", func(t *testing.T) {
		t.Parallel()
		got := renderExtAuthz(t, true, grpcURIArgs(nil))
		require.Nil(t, got.GetAllowedHeaders())
		require.Nil(t, got.GetDisallowedHeaders())
	})

	t.Run("allowed and disallowed headers are rendered", func(t *testing.T) {
		t.Parallel()
		got := renderExtAuthz(t, true, grpcURIArgs(map[string]any{
			"AllowedHeaders": []map[string]any{
				{"Exact": "x-allow-me"},
			},
			"DisallowedHeaders": []map[string]any{
				{"Prefix": "x-secret-"},
			},
		}))
		require.NotNil(t, got.GetAllowedHeaders())
		require.Len(t, got.GetAllowedHeaders().GetPatterns(), 1)
		require.Equal(t, "x-allow-me", got.GetAllowedHeaders().GetPatterns()[0].GetExact())

		require.NotNil(t, got.GetDisallowedHeaders())
		require.Len(t, got.GetDisallowedHeaders().GetPatterns(), 1)
		require.Equal(t, "x-secret-", got.GetDisallowedHeaders().GetPatterns()[0].GetPrefix())
	})

	t.Run("invalid AllowedHeaders matcher is rejected", func(t *testing.T) {
		t.Parallel()
		_, err := newExtAuthz(api.EnvoyExtension{
			Name:     api.BuiltinExtAuthzExtension,
			Required: true,
			Arguments: grpcURIArgs(map[string]any{
				// No match pattern set on the matcher -> validation error.
				"AllowedHeaders": []map[string]any{{}},
			}),
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to validate Config.AllowedHeaders")
	})

	t.Run("invalid DisallowedHeaders matcher is rejected", func(t *testing.T) {
		t.Parallel()
		_, err := newExtAuthz(api.EnvoyExtension{
			Name:     api.BuiltinExtAuthzExtension,
			Required: true,
			Arguments: grpcURIArgs(map[string]any{
				// More than one match pattern set -> validation error.
				"DisallowedHeaders": []map[string]any{{"Exact": "a", "Prefix": "b"}},
			}),
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to validate Config.DisallowedHeaders")
	})
}

// TestExtAuthz_ToEnvoyCluster verifies the local ext_authz cluster generated for
// a URI target: STRICT_DNS is used for DNS names (including "localhost") and
// STATIC for bare IP addresses, and the connect timeout reflects the effective
// service/target timeout.
func TestExtAuthz_ToEnvoyCluster(t *testing.T) {
	t.Parallel()

	newConfig := func(t *testing.T, uri, timeout string) *extAuthz {
		t.Helper()
		cfg := map[string]any{
			"GrpcService": map[string]any{
				"Target": map[string]any{"URI": uri},
			},
		}
		if timeout != "" {
			cfg["GrpcService"].(map[string]any)["Timeout"] = timeout
		}
		ext, err := newExtAuthz(api.EnvoyExtension{
			Name:      api.BuiltinExtAuthzExtension,
			Required:  true,
			Arguments: map[string]any{"ProxyType": "connect-proxy", "Config": cfg},
		})
		require.NoError(t, err)
		return ext
	}

	t.Run("localhost DNS name uses STRICT_DNS", func(t *testing.T) {
		t.Parallel()
		ext := newConfig(t, "localhost:9191", "")
		cluster, err := ext.Config.toEnvoyCluster(&extensioncommon.RuntimeConfig{})
		require.NoError(t, err)
		require.NotNil(t, cluster)
		require.Equal(t, LocalExtAuthzClusterName, cluster.GetName())
		require.Equal(t, envoy_cluster_v3.Cluster_STRICT_DNS, cluster.GetType())
	})

	t.Run("loopback IP uses STATIC", func(t *testing.T) {
		t.Parallel()
		ext := newConfig(t, "127.0.0.1:9191", "")
		cluster, err := ext.Config.toEnvoyCluster(&extensioncommon.RuntimeConfig{})
		require.NoError(t, err)
		require.NotNil(t, cluster)
		require.Equal(t, envoy_cluster_v3.Cluster_STATIC, cluster.GetType())
	})

	t.Run("connect timeout reflects service Timeout", func(t *testing.T) {
		t.Parallel()
		ext := newConfig(t, "localhost:9191", "8s")
		cluster, err := ext.Config.toEnvoyCluster(&extensioncommon.RuntimeConfig{})
		require.NoError(t, err)
		require.NotNil(t, cluster)
		require.Equal(t, 8*time.Second, cluster.GetConnectTimeout().AsDuration())
	})
}

// TestExtAuthz_ConnectProxyLocalhostEnforcement verifies that a connect-proxy
// sidecar rejects a non-loopback URI target host, and that the error points the
// user at the api-gateway proxy type for remote hosts.
func TestExtAuthz_ConnectProxyLocalhostEnforcement(t *testing.T) {
	t.Parallel()

	t.Run("remote host rejected for connect-proxy", func(t *testing.T) {
		t.Parallel()
		_, err := newExtAuthz(api.EnvoyExtension{
			Name:     api.BuiltinExtAuthzExtension,
			Required: true,
			Arguments: map[string]any{
				"ProxyType": "connect-proxy",
				"Config": map[string]any{
					"GrpcService": map[string]any{
						"Target": map[string]any{"URI": "oauth2-proxy:9191"},
					},
				},
			},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid host for Target.URI")
	})

	t.Run("loopback host accepted for connect-proxy", func(t *testing.T) {
		t.Parallel()
		for _, host := range []string{"localhost", "127.0.0.1"} {
			_, err := newExtAuthz(api.EnvoyExtension{
				Name:     api.BuiltinExtAuthzExtension,
				Required: true,
				Arguments: map[string]any{
					"ProxyType": "connect-proxy",
					"Config": map[string]any{
						"GrpcService": map[string]any{
							"Target": map[string]any{"URI": host + ":9191"},
						},
					},
				},
			})
			require.NoError(t, err, "host %q should be accepted", host)
		}
	})
}
