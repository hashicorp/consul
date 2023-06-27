// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package extauthz

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/envoyextensions/extensioncommon"
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
			errMsg:  `expected extension name "builtin/ext-authz"`,
		},
		"invalid proxy type": {
			args:   map[string]any{"ProxyType": "invalid"},
			errMsg: `unsupported ProxyType`,
		},
		"no service type": {
			args:   map[string]any{"ProxyType": "connect-proxy"},
			errMsg: `exactly one of GrpcService or HttpService must be set`,
		},
		"both service types": {
			args: map[string]any{
				"ProxyType": "connect-proxy",
				"Config": map[string]any{
					"GrpcService": map[string]any{
						"Target": map[string]any{
							"URI": "localhost:9191",
						},
					},
					"HttpService": map[string]any{
						"Target": map[string]any{
							"URI": "localhost:9191",
						},
					},
				},
			},
			errMsg: `exactly one of GrpcService or HttpService must be set`,
		},
		"non-loopback address hostname": {
			args: map[string]any{
				"ProxyType": "connect-proxy",
				"Config": map[string]any{
					"GrpcService": map[string]any{
						"Target": map[string]any{
							"URI": "foo.bar.com:9191",
						},
					},
				},
			},
			errMsg: `invalid host for Target.URI "foo.bar.com:9191": expected "localhost", "127.0.0.1", or "::1"`,
		},
		"non-loopback address": {
			args: map[string]any{
				"ProxyType": "connect-proxy",
				"Config": map[string]any{
					"GrpcService": map[string]any{
						"Target": map[string]any{
							"URI": "10.0.0.1:9191",
						},
					},
				},
			},
			errMsg: `invalid host for Target.URI "10.0.0.1:9191": expected "localhost", "127.0.0.1", or "::1"`,
		},
		"invalid target port": {
			args: map[string]any{
				"ProxyType": "connect-proxy",
				"Config": map[string]any{
					"GrpcService": map[string]any{
						"Target": map[string]any{
							"URI": "localhost:zero",
						},
					},
				},
			},
			errMsg: `invalid format for Target.URI "localhost:zero": expected host:port`,
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
			errMsg: `failed to parse Target.Timeout "one" as a duration`,
		},
		"no uri or service target": {
			args: map[string]any{
				"ProxyType": "connect-proxy",
				"Config": map[string]any{
					"HttpService": map[string]any{
						"Target": map[string]any{
							"Timeout": "1s",
						},
					},
				},
			},
			errMsg: `exactly one of Target.Service or Target.URI must be set`,
		},
		"uri and service target": {
			args: map[string]any{
				"ProxyType": "connect-proxy",
				"Config": map[string]any{
					"GrpcService": map[string]any{
						"Target": map[string]any{
							"URI": "10.0.0.1:9191",
							"Service": map[string]any{
								"Name": "test-service",
							},
						},
					},
				},
			},
			errMsg: `exactly one of Target.Service or Target.URI must be set`,
		},
		"invalid status on error": {
			args: map[string]any{
				"ProxyType": "connect-proxy",
				"Config": map[string]any{
					"StatusOnError": 1,
					"GrpcService": map[string]any{
						"Target": map[string]any{
							"URI": "10.0.0.1:9191",
							"Service": map[string]any{
								"Name": "test-service",
							},
						},
					},
				},
			},
			errMsg: `failed to validate Config.StatusOnError`,
		},
		"valid grpc service": {
			args: map[string]any{
				"ProxyType": "connect-proxy",
				"Config": map[string]any{
					"GrpcService": map[string]any{
						"Target": map[string]any{
							"URI": "localhost:9191",
						},
					},
				},
			},
		},
		"valid http service": {
			args: map[string]any{
				"ProxyType": "connect-proxy",
				"Config": map[string]any{
					"HttpService": map[string]any{
						"Target": map[string]any{
							"URI": "127.0.0.1:9191",
						},
					},
				},
			},
		},
	}
	for name, c := range cases {
		c := c
		t.Run(name, func(t *testing.T) {
			extName := api.BuiltinExtAuthzExtension
			if c.extName != "" {
				extName = c.extName
			}
			ext, err := newExtAuthz(api.EnvoyExtension{Name: extName, Arguments: c.args})
			if c.errMsg == "" {
				require.NoError(t, err)

				httpFilter, err := ext.Config.toEnvoyHttpFilter(&extensioncommon.RuntimeConfig{})
				require.NoError(t, err)
				require.NotNil(t, httpFilter)

				if ext.Config.isGRPC() {
					netFilter, err := ext.Config.toEnvoyNetworkFilter(&extensioncommon.RuntimeConfig{})
					require.NoError(t, err)
					require.NotNil(t, netFilter)
				}
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), c.errMsg)
			}
		})
	}
}
