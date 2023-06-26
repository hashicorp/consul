// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package wasm

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_http_wasm_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/wasm/v3"
	envoy_http_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	envoy_network_wasm_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/wasm/v3"
	envoy_wasm_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/wasm/v3"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/envoyextensions/extensioncommon"
	"github.com/hashicorp/consul/proto/private/prototest"
)

func TestHttpWasmExtension(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		extName         string
		canApply        bool
		cfg             func(string, bool) *testWasmConfig
		rtCfg           func(string, bool) *extensioncommon.RuntimeConfig
		isInboundFilter bool
		errStr          string
		debug           bool
	}{
		"remote file": {
			extName:         api.BuiltinWasmExtension,
			canApply:        true,
			cfg:             func(proto string, ent bool) *testWasmConfig { return makeTestWasmConfig(proto, ent) },
			rtCfg:           func(proto string, ent bool) *extensioncommon.RuntimeConfig { return makeTestRuntimeConfig(proto, ent) },
			isInboundFilter: true,
		},
		"local file": {
			extName:  api.BuiltinWasmExtension,
			canApply: true,
			cfg: func(proto string, ent bool) *testWasmConfig {
				cfg := newTestWasmConfig(proto, ent)
				cfg.ListenerType = "inbound"
				cfg.PluginConfig.VmConfig.Code.Local.Filename = "plugin.wasm"
				return cfg
			},
			rtCfg:           func(proto string, ent bool) *extensioncommon.RuntimeConfig { return makeTestRuntimeConfig(proto, ent) },
			isInboundFilter: true,
		},
		"inbound filters ignored": {
			extName:         api.BuiltinWasmExtension,
			canApply:        true,
			cfg:             func(proto string, ent bool) *testWasmConfig { return makeTestWasmConfig(proto, ent) },
			rtCfg:           func(proto string, ent bool) *extensioncommon.RuntimeConfig { return makeTestRuntimeConfig(proto, ent) },
			isInboundFilter: false,
		},
		"no cluster for remote file": {
			extName:  api.BuiltinWasmExtension,
			canApply: true,
			cfg:      func(proto string, ent bool) *testWasmConfig { return makeTestWasmConfig(proto, ent) },
			rtCfg: func(proto string, ent bool) *extensioncommon.RuntimeConfig {
				rt := makeTestRuntimeConfig(proto, ent)
				rt.Upstreams = nil
				return rt
			},
			isInboundFilter: true,
			errStr:          "no upstream found for remote service",
		},
		"protocol mismatch": {
			extName:  api.BuiltinWasmExtension,
			canApply: false,
			cfg:      func(proto string, ent bool) *testWasmConfig { return makeTestWasmConfig(proto, ent) },
			rtCfg: func(proto string, ent bool) *extensioncommon.RuntimeConfig {
				rt := makeTestRuntimeConfig(proto, ent)
				switch proto {
				case "http":
					rt.Protocol = "tcp"
				case "tcp":
					rt.Protocol = "http"
				}
				return rt
			},
			isInboundFilter: true,
		},
	}

	for _, enterprise := range []bool{false, true} {
		for _, protocol := range []string{"tcp", "http"} {
			for name, c := range cases {
				c := c
				t.Run(fmt.Sprintf("%s_%s_ent_%t", name, protocol, enterprise), func(t *testing.T) {
					cfg := c.cfg(protocol, enterprise)
					rtCfg := c.rtCfg(protocol, enterprise)
					rtCfg.EnvoyExtension = api.EnvoyExtension{
						Name:      c.extName,
						Arguments: cfg.toMap(t),
					}

					w, err := construct(rtCfg.EnvoyExtension)
					require.NoError(t, err)
					require.Equal(t, c.canApply, w.CanApply(rtCfg))
					if !c.canApply {
						return
					}

					var inputFilters []*envoy_listener_v3.Filter
					if protocol == "http" {
						inputFilters = append(inputFilters, makeHttpConMgr(t, makeTestHttpFilters()))
					} else {
						inputFilters = makeTestFilters()
					}

					obsFilters, err := w.PatchFilters(rtCfg, inputFilters, c.isInboundFilter)
					if c.errStr == "" {
						require.NoError(t, err)

						expFilters := cfg.expFilters(t)
						if !cfg.matchesDirection(c.isInboundFilter) {
							// If the listener type does not match the filter direction then the
							// filter should not be applied and the output should match the input.
							expFilters = inputFilters
						}

						if c.debug {
							t.Logf("cfg =\n%s\n\n", cfg.toJSON(t))
							require.Equal(t, len(expFilters), len(obsFilters))
							for idx, expFilter := range expFilters {
								t.Logf("expFilterJSON[%d] =\n%s\n\n", idx, protoToJSON(t, expFilter))
								t.Logf("obsfilterJSON[%d] =\n%s\n\n", idx, protoToJSON(t, obsFilters[idx]))
							}
						}

						prototest.AssertDeepEqual(t, expFilters, obsFilters)
					} else {
						require.Error(t, err)
						require.Contains(t, err.Error(), c.errStr)
					}
				})
			}
		}
	}
}

func TestWasmConstructor(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		name   string
		args   func(string, bool) map[string]any
		errStr string
	}{
		"with no arguments": {
			name:   api.BuiltinWasmExtension,
			args:   func(_ string, _ bool) map[string]any { return nil },
			errStr: "VmConfig.Code must provide exactly one of Local or Remote data source",
		},
		"invalid protocol": {
			name: api.BuiltinWasmExtension,
			args: func(proto string, ent bool) map[string]any {
				cfg := newTestWasmConfig(proto, ent)
				cfg.Protocol = "invalid"
				return cfg.toMap(t)
			},
			errStr: `unsupported Protocol "invalid", expected "tcp" or "http"`,
		},
		"invalid proxy type": {
			name: api.BuiltinWasmExtension,
			args: func(proto string, ent bool) map[string]any {
				cfg := newTestWasmConfig(proto, ent)
				cfg.ProxyType = "invalid"
				return cfg.toMap(t)
			},
			errStr: "unsupported ProxyType",
		},
		"invalid listener type": {
			name: api.BuiltinWasmExtension,
			args: func(proto string, ent bool) map[string]any {
				cfg := newTestWasmConfig(proto, ent)
				cfg.ListenerType = "invalid"
				return cfg.toMap(t)
			},
			errStr: `unsupported ListenerType "invalid", expected "inbound" or "outbound"`,
		},
		"invalid runtime": {
			name: api.BuiltinWasmExtension,
			args: func(proto string, ent bool) map[string]any {
				cfg := newTestWasmConfig(proto, ent)
				cfg.PluginConfig.VmConfig.Runtime = "invalid"
				return cfg.toMap(t)
			},
			errStr: "unsupported runtime",
		},
		"both local and remote files": {
			name: api.BuiltinWasmExtension,
			args: func(proto string, ent bool) map[string]any {
				cfg := newTestWasmConfig(proto, ent)
				cfg.PluginConfig.VmConfig.Code.Local.Filename = "plugin.wasm"
				cfg.PluginConfig.VmConfig.Code.Remote.HttpURI.Service.Name = "file-server"
				cfg.PluginConfig.VmConfig.Code.Remote.HttpURI.URI = "http://file-server/plugin.wasm"
				return cfg.toMap(t)
			},
			errStr: "VmConfig.Code must provide exactly one of Local or Remote data source",
		},
		"service and uri required for remote files": {
			name: api.BuiltinWasmExtension,
			args: func(proto string, ent bool) map[string]any {
				cfg := newTestWasmConfig(proto, ent)
				cfg.PluginConfig.VmConfig.Code.Remote.HttpURI.Service.Name = "file-server"
				return cfg.toMap(t)
			},
			errStr: "both Service and URI are required for Remote data sources",
		},
		"no sha for remote file": {
			name: api.BuiltinWasmExtension,
			args: func(proto string, ent bool) map[string]any {
				cfg := newTestWasmConfig(proto, ent)
				cfg.PluginConfig.VmConfig.Code.Remote.HttpURI.Service.Name = "file-server"
				cfg.PluginConfig.VmConfig.Code.Remote.HttpURI.URI = "http://file-server/plugin.wasm"
				return cfg.toMap(t)
			},
			errStr: "SHA256 checksum is required for Remote data sources",
		},
		"invalid url for remote file": {
			name: api.BuiltinWasmExtension,
			args: func(proto string, ent bool) map[string]any {
				cfg := newTestWasmConfig(proto, ent)
				cfg.PluginConfig.VmConfig.Code.Remote.HttpURI.Service.Name = "file-server"
				cfg.PluginConfig.VmConfig.Code.Remote.HttpURI.URI = "://bogus.url.com/error"
				return cfg.toMap(t)
			},
			errStr: `invalid HttpURI.URI: parse "://bogus.url.com/error": missing protocol scheme`,
		},
		"decoding error": {
			name: api.BuiltinWasmExtension,
			args: func(proto string, ent bool) map[string]any {
				a := makeTestWasmConfig(proto, ent).toMap(t)
				setField(a, "PluginConfig.VmConfig.Code.Remote.RetryPolicy.RetryBackOff.BaseInterval", 1000)
				return a
			},
			errStr: "got unconvertible type",
		},
		"invalid http timeout": {
			name: api.BuiltinWasmExtension,
			args: func(proto string, ent bool) map[string]any {
				cfg := newTestWasmConfig(proto, ent)
				cfg.PluginConfig.VmConfig.Code.Remote.HttpURI.Timeout = "invalid"
				return cfg.toMap(t)
			},
			errStr: `failed to parse HttpURI.Timeout "invalid" as a duration`,
		},
		"invalid num retries": {
			name: api.BuiltinWasmExtension,
			args: func(proto string, ent bool) map[string]any {
				cfg := newTestWasmConfig(proto, ent)
				cfg.PluginConfig.VmConfig.Code.Remote.RetryPolicy.NumRetries = -1
				return cfg.toMap(t)
			},
			errStr: "RetryPolicy.NumRetries must be greater than or equal to 0",
		},
		"invalid base interval": {
			name: api.BuiltinWasmExtension,
			args: func(proto string, ent bool) map[string]any {
				cfg := newTestWasmConfig(proto, ent)
				cfg.PluginConfig.VmConfig.Code.Remote.RetryPolicy.RetryBackOff.BaseInterval = "0s"
				return cfg.toMap(t)
			},
			errStr: `RetryBackOff.BaseInterval "0s" must be greater than zero and less than or equal to RetryBackOff.MaxInterval`,
		},
		"invalid max interval": {
			name: api.BuiltinWasmExtension,
			args: func(proto string, ent bool) map[string]any {
				cfg := newTestWasmConfig(proto, ent)
				cfg.PluginConfig.VmConfig.Code.Remote.RetryPolicy.RetryBackOff.BaseInterval = "10s"
				cfg.PluginConfig.VmConfig.Code.Remote.RetryPolicy.RetryBackOff.MaxInterval = "5s"
				return cfg.toMap(t)
			},
			errStr: `RetryBackOff.MaxInterval "5s" must be greater than or equal to RetryBackOff.BaseInterval "10s"`,
		},
		"invalid base interval duration": {
			name: api.BuiltinWasmExtension,
			args: func(proto string, ent bool) map[string]any {
				cfg := newTestWasmConfig(proto, ent)
				cfg.PluginConfig.VmConfig.Code.Remote.RetryPolicy.RetryBackOff.BaseInterval = "invalid"
				return cfg.toMap(t)
			},
			errStr: `failed to parse RetryBackOff.BaseInterval "invalid"`,
		},
		"invalid max interval duration": {
			name: api.BuiltinWasmExtension,
			args: func(proto string, ent bool) map[string]any {
				cfg := newTestWasmConfig(proto, ent)
				cfg.PluginConfig.VmConfig.Code.Remote.RetryPolicy.RetryBackOff.MaxInterval = "invalid"
				return cfg.toMap(t)
			},
			errStr: `failed to parse RetryBackOff.MaxInterval "invalid"`,
		},
		"invalid extension name": {
			name:   "invalid",
			args:   func(proto string, ent bool) map[string]any { return newTestWasmConfig(proto, ent).toMap(t) },
			errStr: `expected extension name "builtin/wasm" but got "invalid"`,
		},
		"valid configuration": {
			name: api.BuiltinWasmExtension,
			args: func(proto string, ent bool) map[string]any { return makeTestWasmConfig(proto, ent).toMap(t) },
		},
	}
	for _, enterprise := range []bool{false, true} {
		for _, protocol := range []string{"tcp", "http"} {
			for name, c := range cases {
				c := c
				t.Run(fmt.Sprintf("%s_%s_ent_%t", name, protocol, enterprise), func(t *testing.T) {
					svc := api.CompoundServiceName{Name: "svc"}
					ext := extensioncommon.RuntimeConfig{
						ServiceName: svc,
						EnvoyExtension: api.EnvoyExtension{
							Name:      c.name,
							Arguments: c.args(protocol, enterprise),
						},
					}

					e, err := Constructor(ext.EnvoyExtension)

					if c.errStr == "" {
						require.NoError(t, err)
						require.NotNil(t, e)
					} else {
						require.Error(t, err)
						require.Contains(t, err.Error(), c.errStr)
					}
				})
			}
		}
	}
}

type testWasmConfig struct {
	Required     bool
	Protocol     string
	ProxyType    string
	ListenerType string
	PluginConfig struct {
		Name     string
		RootID   string
		VmConfig struct {
			VmID    string
			Runtime string
			Code    struct {
				Local struct {
					Filename string
				}
				Remote struct {
					HttpURI struct {
						Service api.CompoundServiceName
						URI     string
						Timeout string
					}
					SHA256      string
					RetryPolicy struct {
						RetryBackOff struct {
							BaseInterval string
							MaxInterval  string
						}
						NumRetries int
					}
				}
			}
			Configuration        string
			EnvironmentVariables struct {
				HostEnvKeys []string
				KeyValues   map[string]string
			}
		}
		Configuration                      string
		CapabilityRestrictionConfiguration struct {
			AllowedCapabilities map[string]any
		}
	}
}

func (c testWasmConfig) toMap(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal(c.toJSON(t), &m))
	return m
}

func (c testWasmConfig) toJSON(t *testing.T) []byte {
	t.Helper()
	b, err := json.MarshalIndent(c, "", "  ")
	require.NoError(t, err)
	return b
}

func (cfg testWasmConfig) toWasmPluginConfig(t *testing.T) *envoy_wasm_v3.PluginConfig {
	t.Helper()
	var code *envoy_core_v3.AsyncDataSource
	if cfg.PluginConfig.VmConfig.Code.Local.Filename != "" {
		code = &envoy_core_v3.AsyncDataSource{
			Specifier: &envoy_core_v3.AsyncDataSource_Local{
				Local: &envoy_core_v3.DataSource{
					Specifier: &envoy_core_v3.DataSource_Filename{
						Filename: cfg.PluginConfig.VmConfig.Code.Local.Filename,
					},
				},
			},
		}
	} else {
		cluster, err := url.Parse(cfg.PluginConfig.VmConfig.Code.Remote.HttpURI.URI)
		require.NoError(t, err)
		timeout, err := time.ParseDuration(cfg.PluginConfig.VmConfig.Code.Remote.HttpURI.Timeout)
		require.NoError(t, err)
		baseInterval, err := time.ParseDuration(cfg.PluginConfig.VmConfig.Code.Remote.RetryPolicy.RetryBackOff.BaseInterval)
		require.NoError(t, err)
		maxInterval, err := time.ParseDuration(cfg.PluginConfig.VmConfig.Code.Remote.RetryPolicy.RetryBackOff.MaxInterval)
		require.NoError(t, err)

		code = &envoy_core_v3.AsyncDataSource{
			Specifier: &envoy_core_v3.AsyncDataSource_Remote{
				Remote: &envoy_core_v3.RemoteDataSource{
					Sha256: cfg.PluginConfig.VmConfig.Code.Remote.SHA256,
					HttpUri: &envoy_core_v3.HttpUri{
						Uri:     cfg.PluginConfig.VmConfig.Code.Remote.HttpURI.URI,
						Timeout: &durationpb.Duration{Seconds: int64(timeout.Seconds())},
						HttpUpstreamType: &envoy_core_v3.HttpUri_Cluster{
							Cluster: cluster.Host,
						},
					},
					RetryPolicy: &envoy_core_v3.RetryPolicy{
						RetryBackOff: &envoy_core_v3.BackoffStrategy{
							BaseInterval: &durationpb.Duration{Seconds: int64(baseInterval.Seconds())},
							MaxInterval:  &durationpb.Duration{Seconds: int64(maxInterval.Seconds())},
						},
						NumRetries: wrapperspb.UInt32(uint32(cfg.PluginConfig.VmConfig.Code.Remote.RetryPolicy.NumRetries)),
					},
				},
			},
		}
	}

	var capConfig *envoy_wasm_v3.CapabilityRestrictionConfig
	if len(cfg.PluginConfig.CapabilityRestrictionConfiguration.AllowedCapabilities) > 0 {
		caps := make(map[string]*envoy_wasm_v3.SanitizationConfig)
		for cap := range cfg.PluginConfig.CapabilityRestrictionConfiguration.AllowedCapabilities {
			caps[cap] = &envoy_wasm_v3.SanitizationConfig{}
		}
		capConfig = &envoy_wasm_v3.CapabilityRestrictionConfig{AllowedCapabilities: caps}
	}

	var vmConfiguration *anypb.Any
	if cfg.PluginConfig.VmConfig.Configuration != "" {
		vmConfiguration = makeAny(t, wrapperspb.String(cfg.PluginConfig.VmConfig.Configuration))
	}

	var envVars *envoy_wasm_v3.EnvironmentVariables
	if len(cfg.PluginConfig.VmConfig.EnvironmentVariables.HostEnvKeys) > 0 ||
		len(cfg.PluginConfig.VmConfig.EnvironmentVariables.KeyValues) > 0 {
		envVars = &envoy_wasm_v3.EnvironmentVariables{
			HostEnvKeys: cfg.PluginConfig.VmConfig.EnvironmentVariables.HostEnvKeys,
			KeyValues:   cfg.PluginConfig.VmConfig.EnvironmentVariables.KeyValues,
		}
	}

	var pluginConfiguration *anypb.Any
	if cfg.PluginConfig.Configuration != "" {
		pluginConfiguration = makeAny(t, wrapperspb.String(cfg.PluginConfig.Configuration))
	}

	rt := cfg.PluginConfig.VmConfig.Runtime
	if rt == "" {
		rt = supportedRuntimes[0]
	}

	return &envoy_wasm_v3.PluginConfig{
		Name:   cfg.PluginConfig.Name,
		RootId: cfg.PluginConfig.RootID,
		Vm: &envoy_wasm_v3.PluginConfig_VmConfig{
			VmConfig: &envoy_wasm_v3.VmConfig{
				VmId:                 cfg.PluginConfig.VmConfig.VmID,
				Runtime:              fmt.Sprintf("envoy.wasm.runtime.%s", rt),
				Code:                 code,
				Configuration:        vmConfiguration,
				EnvironmentVariables: envVars,
			},
		},
		Configuration:               pluginConfiguration,
		FailOpen:                    !cfg.Required,
		CapabilityRestrictionConfig: capConfig,
	}
}

func (cfg testWasmConfig) toHttpFilter(t *testing.T) *envoy_http_v3.HttpFilter {
	return &envoy_http_v3.HttpFilter{
		Name: "envoy.filters.http.wasm",
		ConfigType: &envoy_http_v3.HttpFilter_TypedConfig{
			TypedConfig: makeAny(t,
				&envoy_http_wasm_v3.Wasm{
					Config: cfg.toWasmPluginConfig(t),
				}),
		},
	}
}

func (cfg testWasmConfig) toNetworkFilter(t *testing.T) *envoy_listener_v3.Filter {
	return &envoy_listener_v3.Filter{
		Name: "envoy.filters.network.wasm",
		ConfigType: &envoy_listener_v3.Filter_TypedConfig{
			TypedConfig: makeAny(t,
				&envoy_network_wasm_v3.Wasm{
					Config: cfg.toWasmPluginConfig(t),
				}),
		},
	}
}

func (cfg testWasmConfig) expFilters(t *testing.T) []*envoy_listener_v3.Filter {
	if cfg.Protocol == "http" {
		return []*envoy_listener_v3.Filter{makeHttpConMgr(t, cfg.expHttpFilters(t))}
	} else {
		return cfg.expNetworkFilters(t)
	}
}

func (cfg testWasmConfig) expHttpFilters(t *testing.T) []*envoy_http_v3.HttpFilter {
	return []*envoy_http_v3.HttpFilter{
		{Name: "one"},
		{Name: "two"},
		cfg.toHttpFilter(t),
		{Name: "envoy.filters.http.router"},
		{Name: "three"},
	}
}

func (cfg testWasmConfig) expNetworkFilters(t *testing.T) []*envoy_listener_v3.Filter {
	return []*envoy_listener_v3.Filter{
		{Name: "one"},
		{Name: "two"},
		cfg.toNetworkFilter(t),
		{Name: "envoy.filters.network.tcp_proxy"},
		{Name: "three"},
	}
}

func (cfg testWasmConfig) matchesDirection(isInbound bool) bool {
	return (isInbound && cfg.ListenerType == "inbound") ||
		(!isInbound && cfg.ListenerType == "outbound")
}

func makeAny(t *testing.T, m proto.Message) *anypb.Any {
	t.Helper()
	v, err := anypb.New(m)
	require.NoError(t, err)
	return v
}

func makeHttpConMgr(t *testing.T, filters []*envoy_http_v3.HttpFilter) *envoy_listener_v3.Filter {
	t.Helper()
	return &envoy_listener_v3.Filter{
		Name: "envoy.filters.network.http_connection_manager",
		ConfigType: &envoy_listener_v3.Filter_TypedConfig{
			TypedConfig: makeAny(t, &envoy_http_v3.HttpConnectionManager{
				HttpFilters: filters,
			}),
		},
	}
}

func makeTestFilters() []*envoy_listener_v3.Filter {
	return []*envoy_listener_v3.Filter{
		{Name: "one"},
		{Name: "two"},
		{Name: "envoy.filters.network.tcp_proxy"},
		{Name: "three"},
	}
}

func makeTestHttpFilters() []*envoy_http_v3.HttpFilter {
	return []*envoy_http_v3.HttpFilter{
		{Name: "one"},
		{Name: "two"},
		{Name: "envoy.filters.http.router"},
		{Name: "three"},
	}
}

func makeTestRuntimeConfig(protocol string, enterprise bool) *extensioncommon.RuntimeConfig {
	var ns, ap string
	if enterprise {
		ns = "ns1"
		ap = "ap1"
	}
	return &extensioncommon.RuntimeConfig{
		Kind:        api.ServiceKindConnectProxy,
		ServiceName: api.CompoundServiceName{Name: "test-service"},
		Upstreams: map[api.CompoundServiceName]*extensioncommon.UpstreamData{
			{
				Name:      "test-file-server",
				Namespace: acl.NamespaceOrDefault(ns),
				Partition: acl.PartitionOrDefault(ap),
			}: {
				SNIs:    map[string]struct{}{"test-file-server": {}},
				EnvoyID: "test-file-server",
			},
		},
		Protocol: protocol,
	}
}

func makeTestWasmConfig(protocol string, enterprise bool) *testWasmConfig {
	cfg := newTestWasmConfig(protocol, enterprise)
	cfg.Required = false
	cfg.Protocol = protocol
	cfg.ProxyType = "connect-proxy"
	cfg.ListenerType = "inbound"
	cfg.PluginConfig.Name = "test-plugin-name"
	cfg.PluginConfig.RootID = "test-root-id"
	cfg.PluginConfig.VmConfig.VmID = "test-vm-id"
	cfg.PluginConfig.VmConfig.Runtime = "wasmtime"
	cfg.PluginConfig.VmConfig.Code.Remote.HttpURI.Service.Name = "test-file-server"
	cfg.PluginConfig.VmConfig.Code.Remote.HttpURI.URI = "https://test-file-server/plugin.wasm"
	cfg.PluginConfig.VmConfig.Code.Remote.HttpURI.Timeout = "5s"
	cfg.PluginConfig.VmConfig.Code.Remote.SHA256 = "d05d88b0ce8a8f1d5176481e0af3ae5c65ed82cbfb8c61506c5354b076078545"
	cfg.PluginConfig.VmConfig.Code.Remote.RetryPolicy.RetryBackOff.BaseInterval = "3s"
	cfg.PluginConfig.VmConfig.Code.Remote.RetryPolicy.RetryBackOff.MaxInterval = "15s"
	cfg.PluginConfig.VmConfig.Code.Remote.RetryPolicy.NumRetries = 3
	cfg.PluginConfig.VmConfig.Configuration = "test-vm-configuration"
	cfg.PluginConfig.VmConfig.EnvironmentVariables.HostEnvKeys = []string{"PATH"}
	cfg.PluginConfig.VmConfig.EnvironmentVariables.KeyValues = map[string]string{"TEST_VAR": "TEST_VAL"}
	cfg.PluginConfig.Configuration = "test-plugin-configuration"
	cfg.PluginConfig.CapabilityRestrictionConfiguration.AllowedCapabilities = map[string]any{"proxy_on_vm_start": true}
	return cfg
}

func newTestWasmConfig(protocol string, enterprise bool) *testWasmConfig {
	cfg := &testWasmConfig{Protocol: protocol}
	if enterprise {
		cfg.PluginConfig.VmConfig.Code.Remote.HttpURI.Service.Namespace = "ns1"
		cfg.PluginConfig.VmConfig.Code.Remote.HttpURI.Service.Partition = "ap1"
	}
	return cfg
}

func protoToJSON(t *testing.T, pb proto.Message) string {
	t.Helper()
	m := protojson.MarshalOptions{
		Indent: "  ",
	}
	gotJSON, err := m.Marshal(pb)
	require.NoError(t, err)
	return string(gotJSON)
}

func setField(m map[string]any, path string, value any) {
	upsertField(m, path, value, 0)
}

func upsertField(m map[string]any, path string, value any, index int) {
	keys := strings.Split(path, ".")
	key := keys[index]

	if val, ok := m[key]; ok {
		// update the value
		if index == len(keys)-1 {
			m[key] = value
		} else {
			upsertField(val.(map[string]any), path, value, index+1)
		}
	} else {
		// key does not exist so insert it
		if index == len(keys)-1 {
			m[key] = value
		} else {
			newMap := make(map[string]any)
			m[key] = newMap
			upsertField(newMap, path, value, index+1)
		}
	}
}
