// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
)

func TestExtProcServiceName(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		args map[string]any
		want string
	}{
		"nil args": {
			args: nil,
			want: "",
		},
		"grpc service target (PascalCase)": {
			args: map[string]any{
				"Config": map[string]any{
					"GrpcService": map[string]any{
						"Target": map[string]any{
							"Service": map[string]any{"Name": "processor"},
						},
					},
				},
			},
			want: "processor",
		},
		"http service target (camelCase)": {
			args: map[string]any{
				"config": map[string]any{
					"httpService": map[string]any{
						"target": map[string]any{
							"service": map[string]any{"name": "decider"},
						},
					},
				},
			},
			want: "decider",
		},
		"grpc preferred over http": {
			args: map[string]any{
				"Config": map[string]any{
					"GrpcService": map[string]any{
						"Target": map[string]any{"Service": map[string]any{"Name": "grpc-svc"}},
					},
					"HttpService": map[string]any{
						"Target": map[string]any{"Service": map[string]any{"Name": "http-svc"}},
					},
				},
			},
			want: "grpc-svc",
		},
		"uri target yields empty name": {
			args: map[string]any{
				"Config": map[string]any{
					"GrpcService": map[string]any{
						"Target": map[string]any{"URI": "localhost:9191"},
					},
				},
			},
			want: "",
		},
		"no service configured": {
			args: map[string]any{"Config": map[string]any{}},
			want: "",
		},
	}
	for name, c := range cases {
		c := c
		t.Run(name, func(t *testing.T) {
			require.Equal(t, c.want, extProcServiceName(c.args))
		})
	}
}

func TestExtProcUpstreamIDs(t *testing.T) {
	t.Parallel()

	serviceExt := func(name string) structs.EnvoyExtension {
		return structs.EnvoyExtension{
			Name: api.BuiltinExtProcExtension,
			Arguments: map[string]any{
				"Config": map[string]any{
					"GrpcService": map[string]any{
						"Target": map[string]any{"Service": map[string]any{"Name": name}},
					},
				},
			},
		}
	}

	t.Run("no extensions", func(t *testing.T) {
		cfgSnap := &proxycfg.ConfigSnapshot{}
		require.Empty(t, extProcUpstreamIDs(cfgSnap))
	})

	t.Run("ignores non ext-proc extensions", func(t *testing.T) {
		cfgSnap := &proxycfg.ConfigSnapshot{
			Proxy: structs.ConnectProxyConfig{
				EnvoyExtensions: []structs.EnvoyExtension{
					{Name: api.BuiltinExtAuthzExtension, Arguments: map[string]any{}},
				},
			},
		}
		require.Empty(t, extProcUpstreamIDs(cfgSnap))
	})

	t.Run("skips uri targets", func(t *testing.T) {
		cfgSnap := &proxycfg.ConfigSnapshot{
			Proxy: structs.ConnectProxyConfig{
				EnvoyExtensions: []structs.EnvoyExtension{{
					Name: api.BuiltinExtProcExtension,
					Arguments: map[string]any{
						"Config": map[string]any{
							"GrpcService": map[string]any{
								"Target": map[string]any{"URI": "localhost:9191"},
							},
						},
					},
				}},
			},
		}
		require.Empty(t, extProcUpstreamIDs(cfgSnap))
	})

	t.Run("returns unique service upstream ids", func(t *testing.T) {
		cfgSnap := &proxycfg.ConfigSnapshot{
			Proxy: structs.ConnectProxyConfig{
				EnvoyExtensions: []structs.EnvoyExtension{
					serviceExt("processor"),
					serviceExt("processor"), // duplicate is deduped
					serviceExt("other"),
				},
			},
		}
		ids := extProcUpstreamIDs(cfgSnap)
		require.Len(t, ids, 2)

		expectedProcessor := proxycfg.NewUpstreamIDFromServiceName(
			structs.NewServiceName("processor", &cfgSnap.ProxyID.EnterpriseMeta))
		expectedOther := proxycfg.NewUpstreamIDFromServiceName(
			structs.NewServiceName("other", &cfgSnap.ProxyID.EnterpriseMeta))
		require.Contains(t, ids, expectedProcessor)
		require.Contains(t, ids, expectedOther)
	})
}
