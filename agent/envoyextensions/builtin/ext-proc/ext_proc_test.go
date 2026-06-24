// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package ext_proc

import (
	"testing"
	"time"

	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	envoy_http_ext_proc_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	envoy_http_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/api"
	ext_cmn "github.com/hashicorp/consul/envoyextensions/extensioncommon"
)

func TestConstructor(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		extName string
		args    map[string]any
		errMsg  string
	}{
		"invalid name": {
			extName: "invalid",
			errMsg:  `expected extension name "builtin/ext-proc"`,
		},
		"invalid proxy type": {
			args:   map[string]any{"ProxyType": "invalid"},
			errMsg: `unsupported ProxyType`,
		},
		"invalid listener type": {
			args: map[string]any{
				"ProxyType":    "connect-proxy",
				"ListenerType": "sideways",
				"Config": map[string]any{
					"GrpcService": map[string]any{
						"Target": map[string]any{"URI": "localhost:9191"},
					},
				},
			},
			errMsg: `unexpected ListenerType "sideways"`,
		},
		"no service type": {
			args:   map[string]any{"ProxyType": "connect-proxy"},
			errMsg: `exactly one of Config.GrpcService or Config.HttpService must be set`,
		},
		"both service types": {
			args: map[string]any{
				"ProxyType": "connect-proxy",
				"Config": map[string]any{
					"GrpcService": map[string]any{
						"Target": map[string]any{"URI": "localhost:9191"},
					},
					"HttpService": map[string]any{
						"Target": map[string]any{"URI": "localhost:9191"},
					},
				},
			},
			errMsg: `exactly one of Config.GrpcService or Config.HttpService must be set`,
		},
		"grpc missing target": {
			args: map[string]any{
				"ProxyType": "connect-proxy",
				"Config": map[string]any{
					"GrpcService": map[string]any{},
				},
			},
			errMsg: `GrpcService.Target must be set`,
		},
		"http missing target": {
			args: map[string]any{
				"ProxyType": "connect-proxy",
				"Config": map[string]any{
					"HttpService": map[string]any{},
				},
			},
			errMsg: `HttpService.Target must be set`,
		},
		"http invalid path": {
			args: map[string]any{
				"ProxyType": "connect-proxy",
				"Config": map[string]any{
					"HttpService": map[string]any{
						"Target": map[string]any{"URI": "localhost:9191"},
						"Path":   "decide",
					},
				},
			},
			errMsg: `HttpService.Path must start with "/"`,
		},
		"no uri or service target": {
			args: map[string]any{
				"ProxyType": "connect-proxy",
				"Config": map[string]any{
					"HttpService": map[string]any{
						"Target": map[string]any{"Timeout": "1s"},
					},
				},
			},
			errMsg: `exactly one of Target.Service, Target.URI, or Target.Path must be set`,
		},
		"uri and service target": {
			args: map[string]any{
				"ProxyType": "connect-proxy",
				"Config": map[string]any{
					"GrpcService": map[string]any{
						"Target": map[string]any{
							"URI":     "127.0.0.1:9191",
							"Service": map[string]any{"Name": "test-service"},
						},
					},
				},
			},
			errMsg: `exactly one of Target.Service, Target.URI, or Target.Path must be set`,
		},
		"invalid target uri": {
			args: map[string]any{
				"ProxyType": "connect-proxy",
				"Config": map[string]any{
					"GrpcService": map[string]any{
						"Target": map[string]any{"URI": "localhost:zero"},
					},
				},
			},
			errMsg: `invalid Target.URI "localhost:zero"`,
		},
		"invalid target timeout": {
			args: map[string]any{
				"ProxyType": "connect-proxy",
				"Config": map[string]any{
					"GrpcService": map[string]any{
						"Target": map[string]any{
							"URI":     "localhost:9191",
							"Timeout": "one",
						},
					},
				},
			},
			errMsg: `invalid Target.Timeout "one"`,
		},
		"invalid route cache action": {
			args: map[string]any{
				"ProxyType": "connect-proxy",
				"Config": map[string]any{
					"RouteCacheAction": "bogus",
					"GrpcService": map[string]any{
						"Target": map[string]any{"URI": "localhost:9191"},
					},
				},
			},
			errMsg: `invalid Config.RouteCacheAction "bogus"`,
		},
		"valid grpc service uri": {
			args: map[string]any{
				"ProxyType": "connect-proxy",
				"Config": map[string]any{
					"GrpcService": map[string]any{
						"Target": map[string]any{"URI": "localhost:9191"},
					},
				},
			},
		},
		"valid http service uri": {
			args: map[string]any{
				"ProxyType": "connect-proxy",
				"Config": map[string]any{
					"HttpService": map[string]any{
						"Target": map[string]any{"URI": "127.0.0.1:9191"},
						"Path":   "/decide",
					},
				},
			},
		},
		"valid grpc service target": {
			args: map[string]any{
				"ProxyType": "api-gateway",
				"Config": map[string]any{
					"GrpcService": map[string]any{
						"Target": map[string]any{
							"Service": map[string]any{"Name": "processor"},
						},
					},
				},
			},
		},
		"valid grpc uds target on inference-gateway": {
			args: map[string]any{
				"ProxyType": "inference-gateway",
				"Config": map[string]any{
					"GrpcService": map[string]any{
						"Target": map[string]any{"Path": "/run/consul/ext_proc.sock"},
					},
				},
			},
		},
		"uds and uri target": {
			args: map[string]any{
				"ProxyType": "inference-gateway",
				"Config": map[string]any{
					"GrpcService": map[string]any{
						"Target": map[string]any{
							"Path": "/run/consul/ext_proc.sock",
							"URI":  "127.0.0.1:9191",
						},
					},
				},
			},
			errMsg: `exactly one of Target.Service, Target.URI, or Target.Path must be set`,
		},
		"relative uds path": {
			args: map[string]any{
				"ProxyType": "inference-gateway",
				"Config": map[string]any{
					"GrpcService": map[string]any{
						"Target": map[string]any{"Path": "run/ext_proc.sock"},
					},
				},
			},
			errMsg: `must be an absolute Unix socket path`,
		},
		"unsupported proxy type": {
			args: map[string]any{
				"ProxyType": "mesh-gateway",
				"Config": map[string]any{
					"GrpcService": map[string]any{
						"Target": map[string]any{"URI": "localhost:9191"},
					},
				},
			},
			errMsg: `unsupported ProxyType "mesh-gateway"`,
		},
		"valid camelCase args": {
			args: map[string]any{
				"proxyType": "connect-proxy",
				"config": map[string]any{
					"grpcService": map[string]any{
						"target": map[string]any{"uri": "localhost:9191"},
					},
				},
			},
		},
	}
	for name, c := range cases {
		c := c
		t.Run(name, func(t *testing.T) {
			extName := api.BuiltinExtProcExtension
			if c.extName != "" {
				extName = c.extName
			}
			ext, err := newExtProc(api.EnvoyExtension{Name: extName, Arguments: c.args})
			if c.errMsg == "" {
				require.NoError(t, err)
				require.NotNil(t, ext)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), c.errMsg)
			}
		})
	}
}

func TestConstructor_ReturnsExtender(t *testing.T) {
	t.Parallel()
	ext, err := Constructor(api.EnvoyExtension{
		Name: api.BuiltinExtProcExtension,
		Arguments: map[string]any{
			"ProxyType": "connect-proxy",
			"Config": map[string]any{
				"GrpcService": map[string]any{
					"Target": map[string]any{"URI": "localhost:9191"},
				},
			},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, ext)

	_, err = Constructor(api.EnvoyExtension{Name: "wrong"})
	require.Error(t, err)
}

func TestNormalizeDefaults(t *testing.T) {
	t.Parallel()
	ext, err := newExtProc(api.EnvoyExtension{
		Name: api.BuiltinExtProcExtension,
		Arguments: map[string]any{
			"Config": map[string]any{
				"GrpcService": map[string]any{
					"Target": map[string]any{
						"Service": map[string]any{"Name": "processor"},
					},
				},
			},
		},
	})
	require.NoError(t, err)
	// ProxyType and ListenerType default when unset.
	require.Equal(t, api.ServiceKindAPIGateway, ext.ProxyType)
	require.Equal(t, "inbound", ext.ListenerType)
	// Service namespace/partition normalized to defaults.
	target := ext.Config.target()
	require.NotNil(t, target)
	require.Equal(t, "default", target.Service.Namespace)
	require.Equal(t, "default", target.Service.Partition)
}

func TestConfig_FailureModeAllow(t *testing.T) {
	t.Parallel()
	newCfg := func(args map[string]any) *extProc {
		ext, err := newExtProc(api.EnvoyExtension{Name: api.BuiltinExtProcExtension, Arguments: args})
		require.NoError(t, err)
		return ext
	}
	base := func(extra map[string]any) map[string]any {
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

	require.False(t, newCfg(base(nil)).Config.failureModeAllow, "defaults to false")
	require.True(t, newCfg(base(map[string]any{"FailureModeAllow": true})).Config.failureModeAllow)
	require.False(t, newCfg(base(map[string]any{"FailureModeAllow": false})).Config.failureModeAllow)
}

func TestConfig_RouteCacheAction(t *testing.T) {
	t.Parallel()
	cases := map[string]envoy_http_ext_proc_v3.ExternalProcessor_RouteCacheAction{
		"":        envoy_http_ext_proc_v3.ExternalProcessor_DEFAULT,
		"DEFAULT": envoy_http_ext_proc_v3.ExternalProcessor_DEFAULT,
		"default": envoy_http_ext_proc_v3.ExternalProcessor_DEFAULT,
		"CLEAR":   envoy_http_ext_proc_v3.ExternalProcessor_CLEAR,
		"clear":   envoy_http_ext_proc_v3.ExternalProcessor_CLEAR,
		"RETAIN":  envoy_http_ext_proc_v3.ExternalProcessor_RETAIN,
	}
	for in, want := range cases {
		in, want := in, want
		t.Run(in, func(t *testing.T) {
			ext, err := newExtProc(api.EnvoyExtension{
				Name: api.BuiltinExtProcExtension,
				Arguments: map[string]any{
					"ProxyType": "connect-proxy",
					"Config": map[string]any{
						"RouteCacheAction": in,
						"GrpcService": map[string]any{
							"Target": map[string]any{"URI": "localhost:9191"},
						},
					},
				},
			})
			require.NoError(t, err)
			require.Equal(t, want, ext.Config.routeCacheAction)
		})
	}
}

func TestTarget_clusterName(t *testing.T) {
	t.Parallel()
	svc := api.CompoundServiceName{Name: "processor", Namespace: "default", Partition: "default"}

	t.Run("not a service target", func(t *testing.T) {
		target := Target{URI: "localhost:9191"}
		_, err := target.clusterName(&ext_cmn.RuntimeConfig{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "target is not configured with an upstream service")
	})

	t.Run("no upstream definition", func(t *testing.T) {
		target := Target{Service: svc}
		_, err := target.clusterName(&ext_cmn.RuntimeConfig{Upstreams: map[api.CompoundServiceName]*ext_cmn.UpstreamData{}})
		require.Error(t, err)
		require.Contains(t, err.Error(), "no upstream definition found")
	})

	t.Run("empty primary SNI", func(t *testing.T) {
		target := Target{Service: svc}
		_, err := target.clusterName(&ext_cmn.RuntimeConfig{
			Upstreams: map[api.CompoundServiceName]*ext_cmn.UpstreamData{
				svc: {PrimarySNI: ""},
			},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "no upstream SNI found")
	})

	t.Run("success", func(t *testing.T) {
		target := Target{Service: svc}
		name, err := target.clusterName(&ext_cmn.RuntimeConfig{
			Upstreams: map[api.CompoundServiceName]*ext_cmn.UpstreamData{
				svc: {PrimarySNI: "processor.default.dc1.internal"},
			},
		})
		require.NoError(t, err)
		require.Equal(t, "processor.default.dc1.internal", name)
	})
}

func TestTarget_timeoutDurationPB(t *testing.T) {
	t.Parallel()
	// nil target / nil timeout default to 5s.
	require.Equal(t, 5*time.Second, (*Target)(nil).timeoutDurationPB().AsDuration())
	require.Equal(t, 5*time.Second, (&Target{}).timeoutDurationPB().AsDuration())

	d := 2 * time.Second
	require.Equal(t, d, (&Target{timeout: &d}).timeoutDurationPB().AsDuration())
}

func TestConfig_getClusterName(t *testing.T) {
	t.Parallel()
	t.Run("uri target returns local cluster", func(t *testing.T) {
		ext, err := newExtProc(api.EnvoyExtension{
			Name: api.BuiltinExtProcExtension,
			Arguments: map[string]any{
				"ProxyType": "connect-proxy",
				"Config": map[string]any{
					"GrpcService": map[string]any{
						"Target": map[string]any{"URI": "localhost:9191"},
					},
				},
			},
		})
		require.NoError(t, err)
		name, err := ext.Config.getClusterName(&ext_cmn.RuntimeConfig{})
		require.NoError(t, err)
		require.Equal(t, LocalExtProcClusterName, name)
	})

	t.Run("service target returns primary SNI", func(t *testing.T) {
		svc := api.CompoundServiceName{Name: "processor", Namespace: "default", Partition: "default"}
		ext, err := newExtProc(api.EnvoyExtension{
			Name: api.BuiltinExtProcExtension,
			Arguments: map[string]any{
				"ProxyType": "api-gateway",
				"Config": map[string]any{
					"GrpcService": map[string]any{
						"Target": map[string]any{"Service": map[string]any{"Name": "processor"}},
					},
				},
			},
		})
		require.NoError(t, err)
		name, err := ext.Config.getClusterName(&ext_cmn.RuntimeConfig{
			Upstreams: map[api.CompoundServiceName]*ext_cmn.UpstreamData{
				svc: {PrimarySNI: "processor.sni"},
			},
		})
		require.NoError(t, err)
		require.Equal(t, "processor.sni", name)
	})
}

func TestConfig_envoyGrpcService(t *testing.T) {
	t.Parallel()
	ext, err := newExtProc(api.EnvoyExtension{
		Name: api.BuiltinExtProcExtension,
		Arguments: map[string]any{
			"ProxyType": "connect-proxy",
			"Config": map[string]any{
				"GrpcService": map[string]any{
					"Authority": "my-authority",
					"Target":    map[string]any{"URI": "localhost:9191", "Timeout": "3s"},
				},
			},
		},
	})
	require.NoError(t, err)
	grpcSvc, err := ext.Config.envoyGrpcService(&ext_cmn.RuntimeConfig{})
	require.NoError(t, err)
	require.NotNil(t, grpcSvc)
	envoyGrpc := grpcSvc.GetEnvoyGrpc()
	require.NotNil(t, envoyGrpc)
	require.Equal(t, LocalExtProcClusterName, envoyGrpc.ClusterName)
	require.Equal(t, "my-authority", envoyGrpc.Authority)
	require.Equal(t, 3*time.Second, grpcSvc.Timeout.AsDuration())
}

func TestConfig_envoyHttpService(t *testing.T) {
	t.Parallel()
	t.Run("uri target builds uri with host:port", func(t *testing.T) {
		ext, err := newExtProc(api.EnvoyExtension{
			Name: api.BuiltinExtProcExtension,
			Arguments: map[string]any{
				"ProxyType": "connect-proxy",
				"Config": map[string]any{
					"HttpService": map[string]any{
						"Target": map[string]any{"URI": "127.0.0.1:9191"},
						"Path":   "/decide",
					},
				},
			},
		})
		require.NoError(t, err)
		httpSvc, err := ext.Config.envoyHttpService(&ext_cmn.RuntimeConfig{})
		require.NoError(t, err)
		require.Equal(t, "http://127.0.0.1:9191/decide", httpSvc.HttpService.HttpUri.Uri)
		require.Equal(t, LocalExtProcClusterName, httpSvc.HttpService.HttpUri.GetCluster())
	})

	t.Run("uri target defaults path to slash", func(t *testing.T) {
		ext, err := newExtProc(api.EnvoyExtension{
			Name: api.BuiltinExtProcExtension,
			Arguments: map[string]any{
				"ProxyType": "connect-proxy",
				"Config": map[string]any{
					"HttpService": map[string]any{
						"Target": map[string]any{"URI": "127.0.0.1:9191"},
					},
				},
			},
		})
		require.NoError(t, err)
		httpSvc, err := ext.Config.envoyHttpService(&ext_cmn.RuntimeConfig{})
		require.NoError(t, err)
		require.Equal(t, "http://127.0.0.1:9191/", httpSvc.HttpService.HttpUri.Uri)
	})

	t.Run("service target builds uri with cluster name", func(t *testing.T) {
		svc := api.CompoundServiceName{Name: "processor", Namespace: "default", Partition: "default"}
		ext, err := newExtProc(api.EnvoyExtension{
			Name: api.BuiltinExtProcExtension,
			Arguments: map[string]any{
				"ProxyType": "api-gateway",
				"Config": map[string]any{
					"HttpService": map[string]any{
						"Target": map[string]any{"Service": map[string]any{"Name": "processor"}},
						"Path":   "/decide",
					},
				},
			},
		})
		require.NoError(t, err)
		httpSvc, err := ext.Config.envoyHttpService(&ext_cmn.RuntimeConfig{
			Upstreams: map[api.CompoundServiceName]*ext_cmn.UpstreamData{
				svc: {PrimarySNI: "processor.sni"},
			},
		})
		require.NoError(t, err)
		require.Equal(t, "http://processor.sni/decide", httpSvc.HttpService.HttpUri.Uri)
		require.Equal(t, "processor.sni", httpSvc.HttpService.HttpUri.GetCluster())
	})
}

func TestConfig_toEnvoyCluster(t *testing.T) {
	t.Parallel()
	t.Run("service target returns nil", func(t *testing.T) {
		ext, err := newExtProc(api.EnvoyExtension{
			Name: api.BuiltinExtProcExtension,
			Arguments: map[string]any{
				"ProxyType": "api-gateway",
				"Config": map[string]any{
					"GrpcService": map[string]any{
						"Target": map[string]any{"Service": map[string]any{"Name": "processor"}},
					},
				},
			},
		})
		require.NoError(t, err)
		cluster, err := ext.Config.toEnvoyCluster(&ext_cmn.RuntimeConfig{})
		require.NoError(t, err)
		require.Nil(t, cluster)
	})

	t.Run("ip uri builds static cluster", func(t *testing.T) {
		ext, err := newExtProc(api.EnvoyExtension{
			Name: api.BuiltinExtProcExtension,
			Arguments: map[string]any{
				"ProxyType": "connect-proxy",
				"Config": map[string]any{
					"GrpcService": map[string]any{
						"Target": map[string]any{"URI": "127.0.0.1:9191"},
					},
				},
			},
		})
		require.NoError(t, err)
		cluster, err := ext.Config.toEnvoyCluster(&ext_cmn.RuntimeConfig{})
		require.NoError(t, err)
		require.NotNil(t, cluster)
		require.Equal(t, LocalExtProcClusterName, cluster.Name)
		require.Contains(t, cluster.TypedExtensionProtocolOptions, "envoy.extensions.upstreams.http.v3.HttpProtocolOptions")
	})

	t.Run("dns uri builds strict dns cluster", func(t *testing.T) {
		ext, err := newExtProc(api.EnvoyExtension{
			Name: api.BuiltinExtProcExtension,
			Arguments: map[string]any{
				"ProxyType": "connect-proxy",
				"Config": map[string]any{
					"HttpService": map[string]any{
						"Target": map[string]any{"URI": "localhost:9191"},
					},
				},
			},
		})
		require.NoError(t, err)
		cluster, err := ext.Config.toEnvoyCluster(&ext_cmn.RuntimeConfig{})
		require.NoError(t, err)
		require.NotNil(t, cluster)
		require.NotNil(t, cluster.DnsLookupFamily)
	})

	t.Run("uds path builds static pipe cluster", func(t *testing.T) {
		const sock = "/run/consul/ext_proc.sock"
		ext, err := newExtProc(api.EnvoyExtension{
			Name: api.BuiltinExtProcExtension,
			Arguments: map[string]any{
				"ProxyType": "inference-gateway",
				"Config": map[string]any{
					"GrpcService": map[string]any{
						"Target": map[string]any{"Path": sock},
					},
				},
			},
		})
		require.NoError(t, err)
		cluster, err := ext.Config.toEnvoyCluster(&ext_cmn.RuntimeConfig{})
		require.NoError(t, err)
		require.NotNil(t, cluster)
		require.Equal(t, LocalExtProcClusterName, cluster.Name)
		// STATIC discovery: DNS lookup family stays the default for a pipe address.
		require.Zero(t, cluster.DnsLookupFamily)
		ep := cluster.LoadAssignment.Endpoints[0].LbEndpoints[0].GetEndpoint()
		require.Equal(t, sock, ep.GetAddress().GetPipe().GetPath())
	})
}

func TestCanApply(t *testing.T) {
	t.Parallel()
	ext, err := newExtProc(api.EnvoyExtension{
		Name: api.BuiltinExtProcExtension,
		Arguments: map[string]any{
			"ProxyType": "connect-proxy",
			"Config": map[string]any{
				"GrpcService": map[string]any{
					"Target": map[string]any{"URI": "localhost:9191"},
				},
			},
		},
	})
	require.NoError(t, err)
	require.True(t, ext.CanApply(&ext_cmn.RuntimeConfig{Kind: api.ServiceKindConnectProxy}))
	require.False(t, ext.CanApply(&ext_cmn.RuntimeConfig{Kind: api.ServiceKindAPIGateway}))
}

func TestMatchesListenerDirection(t *testing.T) {
	t.Parallel()
	newExt := func(proxyType, listenerType string) *extProc {
		ext, err := newExtProc(api.EnvoyExtension{
			Name: api.BuiltinExtProcExtension,
			Arguments: map[string]any{
				"ProxyType":    proxyType,
				"ListenerType": listenerType,
				"Config": map[string]any{
					"GrpcService": map[string]any{
						"Target": map[string]any{"URI": "localhost:9191"},
					},
				},
			},
		})
		require.NoError(t, err)
		return ext
	}

	// API gateway always matches.
	apigw := newExt("api-gateway", "inbound")
	require.True(t, apigw.matchesListenerDirection(true))
	require.True(t, apigw.matchesListenerDirection(false))

	inbound := newExt("connect-proxy", "inbound")
	require.True(t, inbound.matchesListenerDirection(true))
	require.False(t, inbound.matchesListenerDirection(false))

	outbound := newExt("connect-proxy", "outbound")
	require.False(t, outbound.matchesListenerDirection(true))
	require.True(t, outbound.matchesListenerDirection(false))
}

func TestConfigureInsertOptions(t *testing.T) {
	t.Parallel()
	t.Run("defaults when unset", func(t *testing.T) {
		p := &extProc{}
		p.configureInsertOptions()
		require.Equal(t, ext_cmn.InsertBeforeFirstMatch, p.InsertOptions.Location)
		require.Equal(t, "envoy.filters.http.router", p.InsertOptions.FilterName)
	})
	t.Run("respects explicit location", func(t *testing.T) {
		p := &extProc{}
		p.InsertOptions.Location = ext_cmn.InsertFirst
		p.configureInsertOptions()
		require.Equal(t, ext_cmn.InsertFirst, p.InsertOptions.Location)
		require.Empty(t, p.InsertOptions.FilterName)
	})
}

func TestPatchClusters(t *testing.T) {
	t.Parallel()
	t.Run("service target leaves clusters untouched", func(t *testing.T) {
		svc := api.CompoundServiceName{Name: "processor", Namespace: "default", Partition: "default"}
		ext, err := newExtProc(api.EnvoyExtension{
			Name: api.BuiltinExtProcExtension,
			Arguments: map[string]any{
				"ProxyType": "api-gateway",
				"Config": map[string]any{
					"GrpcService": map[string]any{
						"Target": map[string]any{"Service": map[string]any{"Name": "processor"}},
					},
				},
			},
		})
		require.NoError(t, err)
		cfg := &ext_cmn.RuntimeConfig{
			Upstreams: map[api.CompoundServiceName]*ext_cmn.UpstreamData{
				svc: {PrimarySNI: "processor.sni"},
			},
		}
		out, err := ext.PatchClusters(cfg, ext_cmn.ClusterMap{})
		require.NoError(t, err)
		require.Empty(t, out)
	})

	t.Run("uri target adds local cluster", func(t *testing.T) {
		ext, err := newExtProc(api.EnvoyExtension{
			Name: api.BuiltinExtProcExtension,
			Arguments: map[string]any{
				"ProxyType": "connect-proxy",
				"Config": map[string]any{
					"GrpcService": map[string]any{
						"Target": map[string]any{"URI": "127.0.0.1:9191"},
					},
				},
			},
		})
		require.NoError(t, err)
		out, err := ext.PatchClusters(&ext_cmn.RuntimeConfig{}, ext_cmn.ClusterMap{})
		require.NoError(t, err)
		require.Contains(t, out, LocalExtProcClusterName)
	})

	t.Run("existing cluster is not overwritten", func(t *testing.T) {
		ext, err := newExtProc(api.EnvoyExtension{
			Name: api.BuiltinExtProcExtension,
			Arguments: map[string]any{
				"ProxyType": "connect-proxy",
				"Config": map[string]any{
					"GrpcService": map[string]any{
						"Target": map[string]any{"URI": "127.0.0.1:9191"},
					},
				},
			},
		})
		require.NoError(t, err)
		in := ext_cmn.ClusterMap{LocalExtProcClusterName: nil}
		out, err := ext.PatchClusters(&ext_cmn.RuntimeConfig{}, in)
		require.NoError(t, err)
		require.Len(t, out, 1)
	})
}

func TestPatchFilters(t *testing.T) {
	t.Parallel()
	newExt := func(t *testing.T, args map[string]any) *extProc {
		ext, err := newExtProc(api.EnvoyExtension{Name: api.BuiltinExtProcExtension, Arguments: args})
		require.NoError(t, err)
		return ext
	}

	grpcArgs := map[string]any{
		"ProxyType": "connect-proxy",
		"Config": map[string]any{
			"GrpcService": map[string]any{
				"Target": map[string]any{"URI": "localhost:9191"},
			},
		},
	}

	// hcmFilters builds a listener filter chain containing an HTTP connection
	// manager with a single router HTTP filter, which is what ext_proc inserts
	// itself before.
	hcmFilters := func(t *testing.T) []*envoy_listener_v3.Filter {
		hcm := &envoy_http_v3.HttpConnectionManager{
			HttpFilters: []*envoy_http_v3.HttpFilter{{Name: "envoy.filters.http.router"}},
		}
		any, err := anypb.New(hcm)
		require.NoError(t, err)
		return []*envoy_listener_v3.Filter{{
			Name:       "envoy.filters.network.http_connection_manager",
			ConfigType: &envoy_listener_v3.Filter_TypedConfig{TypedConfig: any},
		}}
	}

	// httpFilterNames extracts the HTTP filter names from the HCM in a patched
	// listener filter chain.
	httpFilterNames := func(t *testing.T, filters []*envoy_listener_v3.Filter) []string {
		require.Len(t, filters, 1)
		var hcm envoy_http_v3.HttpConnectionManager
		require.NoError(t, filters[0].GetTypedConfig().UnmarshalTo(&hcm))
		names := make([]string, 0, len(hcm.HttpFilters))
		for _, f := range hcm.HttpFilters {
			names = append(names, f.Name)
		}
		return names
	}

	t.Run("skips unsupported protocol", func(t *testing.T) {
		ext := newExt(t, grpcArgs)
		filters := hcmFilters(t)
		out, err := ext.PatchFilters(&ext_cmn.RuntimeConfig{Protocol: "tcp"}, filters, true)
		require.NoError(t, err)
		require.Equal(t, filters, out)
	})

	t.Run("skips non-matching listener direction", func(t *testing.T) {
		ext := newExt(t, map[string]any{
			"ProxyType":    "connect-proxy",
			"ListenerType": "outbound",
			"Config": map[string]any{
				"GrpcService": map[string]any{
					"Target": map[string]any{"URI": "localhost:9191"},
				},
			},
		})
		filters := hcmFilters(t)
		// inbound listener does not match outbound config
		out, err := ext.PatchFilters(&ext_cmn.RuntimeConfig{Protocol: "http"}, filters, true)
		require.NoError(t, err)
		require.Equal(t, filters, out)
	})

	t.Run("adds grpc ext_proc filter", func(t *testing.T) {
		ext := newExt(t, grpcArgs)
		out, err := ext.PatchFilters(&ext_cmn.RuntimeConfig{Protocol: "http"}, hcmFilters(t), true)
		require.NoError(t, err)
		require.Equal(t, []string{"envoy.filters.http.ext_proc", "envoy.filters.http.router"}, httpFilterNames(t, out))
	})

	t.Run("adds http ext_proc filter", func(t *testing.T) {
		ext := newExt(t, map[string]any{
			"ProxyType": "connect-proxy",
			"Config": map[string]any{
				"HttpService": map[string]any{
					"Target": map[string]any{"URI": "localhost:9191"},
					"Path":   "/decide",
				},
			},
		})
		out, err := ext.PatchFilters(&ext_cmn.RuntimeConfig{Protocol: "http"}, hcmFilters(t), true)
		require.NoError(t, err)
		require.Equal(t, []string{"envoy.filters.http.ext_proc", "envoy.filters.http.router"}, httpFilterNames(t, out))
	})
}

func TestPatchRoutes(t *testing.T) {
	t.Parallel()
	t.Run("grpc mode leaves routes untouched", func(t *testing.T) {
		ext, err := newExtProc(api.EnvoyExtension{
			Name: api.BuiltinExtProcExtension,
			Arguments: map[string]any{
				"ProxyType": "connect-proxy",
				"Config": map[string]any{
					"GrpcService": map[string]any{
						"Target": map[string]any{"URI": "localhost:9191"},
					},
				},
			},
		})
		require.NoError(t, err)
		rc := &envoy_route_v3.RouteConfiguration{
			VirtualHosts: []*envoy_route_v3.VirtualHost{{
				Routes: []*envoy_route_v3.Route{{
					Match: &envoy_route_v3.RouteMatch{PathSpecifier: &envoy_route_v3.RouteMatch_Path{Path: "/decide"}},
				}},
			}},
		}
		routes := ext_cmn.RouteMap{"r": rc}
		out, err := ext.PatchRoutes(&ext_cmn.RuntimeConfig{}, routes)
		require.NoError(t, err)
		require.Nil(t, rc.VirtualHosts[0].Routes[0].TypedPerFilterConfig)
		require.Equal(t, routes, out)
	})

	t.Run("http mode disables ext_proc on matching path", func(t *testing.T) {
		ext, err := newExtProc(api.EnvoyExtension{
			Name: api.BuiltinExtProcExtension,
			Arguments: map[string]any{
				"ProxyType": "connect-proxy",
				"Config": map[string]any{
					"HttpService": map[string]any{
						"Target": map[string]any{"URI": "localhost:9191"},
						"Path":   "/decide",
					},
				},
			},
		})
		require.NoError(t, err)
		matching := &envoy_route_v3.Route{
			Match: &envoy_route_v3.RouteMatch{PathSpecifier: &envoy_route_v3.RouteMatch_Path{Path: "/decide"}},
		}
		other := &envoy_route_v3.Route{
			Match: &envoy_route_v3.RouteMatch{PathSpecifier: &envoy_route_v3.RouteMatch_Path{Path: "/other"}},
		}
		rc := &envoy_route_v3.RouteConfiguration{
			VirtualHosts: []*envoy_route_v3.VirtualHost{{Routes: []*envoy_route_v3.Route{matching, other}}},
		}
		_, err = ext.PatchRoutes(&ext_cmn.RuntimeConfig{}, ext_cmn.RouteMap{"r": rc})
		require.NoError(t, err)
		require.Contains(t, matching.TypedPerFilterConfig, "envoy.filters.http.ext_proc")
		require.Nil(t, other.TypedPerFilterConfig)
	})

	t.Run("http mode with empty path leaves routes untouched", func(t *testing.T) {
		ext, err := newExtProc(api.EnvoyExtension{
			Name: api.BuiltinExtProcExtension,
			Arguments: map[string]any{
				"ProxyType": "connect-proxy",
				"Config": map[string]any{
					"HttpService": map[string]any{
						"Target": map[string]any{"URI": "localhost:9191"},
					},
				},
			},
		})
		require.NoError(t, err)
		r := &envoy_route_v3.Route{
			Match: &envoy_route_v3.RouteMatch{PathSpecifier: &envoy_route_v3.RouteMatch_Path{Path: "/decide"}},
		}
		rc := &envoy_route_v3.RouteConfiguration{
			VirtualHosts: []*envoy_route_v3.VirtualHost{{Routes: []*envoy_route_v3.Route{r}}},
		}
		_, err = ext.PatchRoutes(&ext_cmn.RuntimeConfig{}, ext_cmn.RouteMap{"r": rc})
		require.NoError(t, err)
		require.Nil(t, r.TypedPerFilterConfig)
	})

	t.Run("EnableRoutes disables non-matching routes", func(t *testing.T) {
		ext, err := newExtProc(api.EnvoyExtension{
			Name: api.BuiltinExtProcExtension,
			Arguments: map[string]any{
				"ProxyType": "connect-proxy",
				"Config": map[string]any{
					"GrpcService": map[string]any{
						"Target": map[string]any{"URI": "localhost:9191"},
					},
					"EnableRoutes": []any{"/checkout"},
				},
			},
		})
		require.NoError(t, err)
		enabled := &envoy_route_v3.Route{
			Match: &envoy_route_v3.RouteMatch{PathSpecifier: &envoy_route_v3.RouteMatch_Path{Path: "/checkout"}},
		}
		disabled := &envoy_route_v3.Route{
			Match: &envoy_route_v3.RouteMatch{PathSpecifier: &envoy_route_v3.RouteMatch_Path{Path: "/other"}},
		}
		rc := &envoy_route_v3.RouteConfiguration{
			VirtualHosts: []*envoy_route_v3.VirtualHost{{Routes: []*envoy_route_v3.Route{enabled, disabled}}},
		}
		_, err = ext.PatchRoutes(&ext_cmn.RuntimeConfig{}, ext_cmn.RouteMap{"r": rc})
		require.NoError(t, err)
		require.Nil(t, enabled.TypedPerFilterConfig)
		require.Contains(t, disabled.TypedPerFilterConfig, "envoy.filters.http.ext_proc")
	})

	t.Run("DisableRoutes disables matching routes", func(t *testing.T) {
		ext, err := newExtProc(api.EnvoyExtension{
			Name: api.BuiltinExtProcExtension,
			Arguments: map[string]any{
				"ProxyType": "connect-proxy",
				"Config": map[string]any{
					"GrpcService": map[string]any{
						"Target": map[string]any{"URI": "localhost:9191"},
					},
					"DisableRoutes": []any{"/admin"},
				},
			},
		})
		require.NoError(t, err)
		disabled := &envoy_route_v3.Route{
			Match: &envoy_route_v3.RouteMatch{PathSpecifier: &envoy_route_v3.RouteMatch_Path{Path: "/admin"}},
		}
		enabled := &envoy_route_v3.Route{
			Match: &envoy_route_v3.RouteMatch{PathSpecifier: &envoy_route_v3.RouteMatch_Path{Path: "/other"}},
		}
		rc := &envoy_route_v3.RouteConfiguration{
			VirtualHosts: []*envoy_route_v3.VirtualHost{{Routes: []*envoy_route_v3.Route{disabled, enabled}}},
		}
		_, err = ext.PatchRoutes(&ext_cmn.RuntimeConfig{}, ext_cmn.RouteMap{"r": rc})
		require.NoError(t, err)
		require.Contains(t, disabled.TypedPerFilterConfig, "envoy.filters.http.ext_proc")
		require.Nil(t, enabled.TypedPerFilterConfig)
	})

	t.Run("EnableRoutes and DisableRoutes are mutually exclusive", func(t *testing.T) {
		_, err := newExtProc(api.EnvoyExtension{
			Name: api.BuiltinExtProcExtension,
			Arguments: map[string]any{
				"ProxyType": "connect-proxy",
				"Config": map[string]any{
					"GrpcService": map[string]any{
						"Target": map[string]any{"URI": "localhost:9191"},
					},
					"EnableRoutes":  []any{"/checkout"},
					"DisableRoutes": []any{"/admin"},
				},
			},
		})
		require.ErrorContains(t, err, "only one of Config.EnableRoutes or Config.DisableRoutes may be set")
	})
}

func TestRouteMatchTargetsBypassPath(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		match *envoy_route_v3.RouteMatch
		path  string
		want  bool
	}{
		"nil match":       {nil, "/decide", false},
		"empty path":      {&envoy_route_v3.RouteMatch{PathSpecifier: &envoy_route_v3.RouteMatch_Path{Path: "/decide"}}, "", false},
		"path match":      {&envoy_route_v3.RouteMatch{PathSpecifier: &envoy_route_v3.RouteMatch_Path{Path: "/decide"}}, "/decide", true},
		"path no match":   {&envoy_route_v3.RouteMatch{PathSpecifier: &envoy_route_v3.RouteMatch_Path{Path: "/x"}}, "/decide", false},
		"prefix match":    {&envoy_route_v3.RouteMatch{PathSpecifier: &envoy_route_v3.RouteMatch_Prefix{Prefix: "/decide"}}, "/decide", true},
		"separated match": {&envoy_route_v3.RouteMatch{PathSpecifier: &envoy_route_v3.RouteMatch_PathSeparatedPrefix{PathSeparatedPrefix: "/decide"}}, "/decide", true},
		"regex no match":  {&envoy_route_v3.RouteMatch{}, "/decide", false},
	}
	for name, c := range cases {
		c := c
		t.Run(name, func(t *testing.T) {
			require.Equal(t, c.want, routeMatchTargetsBypassPath(c.match, c.path))
		})
	}
}

func TestParseAddr(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		in       string
		wantHost string
		wantPort int
		wantErr  bool
	}{
		"host:port":         {"localhost:9191", "localhost", 9191, false},
		"ip:port":           {"127.0.0.1:9191", "127.0.0.1", 9191, false},
		"with scheme":       {"http://localhost:9191", "localhost", 9191, false},
		"missing port":      {"localhost:", "", 0, true},
		"no colon":          {"localhost", "", 0, true},
		"missing host":      {":9191", "", 0, true},
		"non-numeric port":  {"localhost:zero", "", 0, true},
		"port out of range": {"localhost:70000", "", 0, true},
		"zero port":         {"localhost:0", "", 0, true},
	}
	for name, c := range cases {
		c := c
		t.Run(name, func(t *testing.T) {
			host, port, err := parseAddr(c.in)
			if c.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, c.wantHost, host)
			require.Equal(t, c.wantPort, port)
		})
	}
}
