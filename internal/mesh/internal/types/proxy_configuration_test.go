// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"math"
	"testing"

	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"

	catalogtesthelpers "github.com/hashicorp/consul/internal/catalog/catalogtest/helpers"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/iptables"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestProxyConfigurationACLs(t *testing.T) {
	catalogtesthelpers.RunWorkloadSelectingTypeACLsTests[*pbmesh.ProxyConfiguration](t, pbmesh.ProxyConfigurationType,
		func(selector *pbcatalog.WorkloadSelector) *pbmesh.ProxyConfiguration {
			return &pbmesh.ProxyConfiguration{
				Workloads:     selector,
				DynamicConfig: &pbmesh.DynamicConfig{},
			}
		},
		RegisterProxyConfiguration,
	)
}

func TestMutateProxyConfiguration(t *testing.T) {
	cases := map[string]struct {
		data    *pbmesh.ProxyConfiguration
		expData *pbmesh.ProxyConfiguration
	}{
		"tproxy disabled": {
			data:    &pbmesh.ProxyConfiguration{},
			expData: &pbmesh.ProxyConfiguration{},
		},
		"tproxy disabled explicitly": {
			data: &pbmesh.ProxyConfiguration{
				DynamicConfig: &pbmesh.DynamicConfig{
					Mode: pbmesh.ProxyMode_PROXY_MODE_DIRECT,
				},
			},
			expData: &pbmesh.ProxyConfiguration{
				DynamicConfig: &pbmesh.DynamicConfig{
					Mode: pbmesh.ProxyMode_PROXY_MODE_DIRECT,
				},
			},
		},
		"tproxy enabled and tproxy config is nil": {
			data: &pbmesh.ProxyConfiguration{
				DynamicConfig: &pbmesh.DynamicConfig{
					Mode: pbmesh.ProxyMode_PROXY_MODE_TRANSPARENT,
				},
			},
			expData: &pbmesh.ProxyConfiguration{
				DynamicConfig: &pbmesh.DynamicConfig{
					Mode: pbmesh.ProxyMode_PROXY_MODE_TRANSPARENT,
					TransparentProxy: &pbmesh.TransparentProxy{
						OutboundListenerPort: iptables.DefaultTProxyOutboundPort,
					},
				},
			},
		},
		"tproxy enabled and tproxy config is empty": {
			data: &pbmesh.ProxyConfiguration{
				DynamicConfig: &pbmesh.DynamicConfig{
					Mode:             pbmesh.ProxyMode_PROXY_MODE_TRANSPARENT,
					TransparentProxy: &pbmesh.TransparentProxy{},
				},
			},
			expData: &pbmesh.ProxyConfiguration{
				DynamicConfig: &pbmesh.DynamicConfig{
					Mode: pbmesh.ProxyMode_PROXY_MODE_TRANSPARENT,
					TransparentProxy: &pbmesh.TransparentProxy{
						OutboundListenerPort: iptables.DefaultTProxyOutboundPort,
					},
				},
			},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			res := resourcetest.Resource(pbmesh.ProxyConfigurationType, "test").
				WithData(t, c.data).
				Build()

			err := MutateProxyConfiguration(res)
			require.NoError(t, err)

			got := resourcetest.MustDecode[*pbmesh.ProxyConfiguration](t, res)
			prototest.AssertDeepEqual(t, c.expData, got.GetData())
		})
	}
}

func TestValidateProxyConfiguration_MissingBothDynamicAndBootstrapConfig(t *testing.T) {
	proxyCfg := &pbmesh.ProxyConfiguration{
		Workloads: &pbcatalog.WorkloadSelector{Names: []string{"foo"}},
	}

	res := resourcetest.Resource(pbmesh.ProxyConfigurationType, "test").
		WithData(t, proxyCfg).
		Build()

	err := ValidateProxyConfiguration(res)

	var expError error
	expError = multierror.Append(expError,
		resource.ErrInvalidFields{
			Names:   []string{"dynamic_config", "bootstrap_config"},
			Wrapped: errMissingProxyConfigData,
		},
	)
	require.Equal(t, err, expError)
}

func TestValidateProxyConfiguration_AllFieldsInvalid(t *testing.T) {
	proxyCfg := &pbmesh.ProxyConfiguration{
		// Omit workload selector.

		DynamicConfig: &pbmesh.DynamicConfig{
			// Set unsupported fields.
			MutualTlsMode:           pbmesh.MutualTLSMode_MUTUAL_TLS_MODE_PERMISSIVE,
			AccessLogs:              &pbmesh.AccessLogsConfig{},
			PublicListenerJson:      "listener-json",
			ListenerTracingJson:     "tracing-json",
			LocalClusterJson:        "cluster-json",
			LocalWorkloadAddress:    "1.1.1.1",
			LocalWorkloadPort:       1234,
			LocalWorkloadSocketPath: "/foo/bar",

			TransparentProxy: &pbmesh.TransparentProxy{
				DialedDirectly:       true,               // unsupported
				OutboundListenerPort: math.MaxUint16 + 1, // invalid
			},

			// Create invalid expose paths config.
			ExposeConfig: &pbmesh.ExposeConfig{
				ExposePaths: []*pbmesh.ExposePath{
					{
						ListenerPort:  0,
						LocalPathPort: math.MaxUint16 + 1,
					},
				},
			},
		},

		OpaqueConfig: &structpb.Struct{},
	}

	res := resourcetest.Resource(pbmesh.ProxyConfigurationType, "test").
		WithData(t, proxyCfg).
		Build()

	err := ValidateProxyConfiguration(res)

	var dynamicCfgErr error
	unsupportedFields := []string{
		"mutual_tls_mode",
		"access_logs",
		"public_listener_json",
		"listener_tracing_json",
		"local_cluster_json",
		"local_workload_address",
		"local_workload_port",
		"local_workload_socket_path",
	}
	for _, f := range unsupportedFields {
		dynamicCfgErr = multierror.Append(dynamicCfgErr,
			resource.ErrInvalidField{
				Name:    f,
				Wrapped: resource.ErrUnsupported,
			},
		)
	}
	dynamicCfgErr = multierror.Append(dynamicCfgErr,
		resource.ErrInvalidField{
			Name: "transparent_proxy",
			Wrapped: resource.ErrInvalidField{
				Name:    "dialed_directly",
				Wrapped: resource.ErrUnsupported,
			},
		},
		resource.ErrInvalidField{
			Name: "transparent_proxy",
			Wrapped: resource.ErrInvalidField{
				Name:    "outbound_listener_port",
				Wrapped: errInvalidPort,
			},
		},
		resource.ErrInvalidField{
			Name: "expose_config",
			Wrapped: resource.ErrInvalidListElement{
				Name: "expose_paths",
				Wrapped: resource.ErrInvalidField{
					Name:    "listener_port",
					Wrapped: errInvalidPort,
				},
			},
		},
		resource.ErrInvalidField{
			Name: "expose_config",
			Wrapped: resource.ErrInvalidListElement{
				Name: "expose_paths",
				Wrapped: resource.ErrInvalidField{
					Name:    "local_path_port",
					Wrapped: errInvalidPort,
				},
			},
		},
	)

	var expError error
	expError = multierror.Append(expError,
		resource.ErrInvalidField{
			Name:    "workloads",
			Wrapped: resource.ErrEmpty,
		},
		resource.ErrInvalidField{
			Name:    "opaque_config",
			Wrapped: resource.ErrUnsupported,
		},
		resource.ErrInvalidField{
			Name:    "dynamic_config",
			Wrapped: dynamicCfgErr,
		},
	)

	require.Equal(t, err, expError)
}

func TestValidateProxyConfiguration_AllFieldsValid(t *testing.T) {
	proxyCfg := &pbmesh.ProxyConfiguration{
		Workloads: &pbcatalog.WorkloadSelector{Names: []string{"foo"}},

		DynamicConfig: &pbmesh.DynamicConfig{
			MutualTlsMode:   pbmesh.MutualTLSMode_MUTUAL_TLS_MODE_DEFAULT,
			MeshGatewayMode: pbmesh.MeshGatewayMode_MESH_GATEWAY_MODE_LOCAL,

			TransparentProxy: &pbmesh.TransparentProxy{
				DialedDirectly:       false,
				OutboundListenerPort: 15500,
			},

			ExposeConfig: &pbmesh.ExposeConfig{
				ExposePaths: []*pbmesh.ExposePath{
					{
						ListenerPort:  1234,
						LocalPathPort: 1235,
					},
				},
			},
		},

		BootstrapConfig: &pbmesh.BootstrapConfig{
			StatsdUrl:                       "stats-url",
			DogstatsdUrl:                    "dogstats-url",
			StatsTags:                       []string{"tags"},
			PrometheusBindAddr:              "prom-bind-addr",
			StatsBindAddr:                   "stats-bind-addr",
			ReadyBindAddr:                   "ready-bind-addr",
			OverrideJsonTpl:                 "override-json-tpl",
			StaticClustersJson:              "static-clusters-json",
			StaticListenersJson:             "static-listeners-json",
			StatsSinksJson:                  "stats-sinks-json",
			StatsConfigJson:                 "stats-config-json",
			StatsFlushInterval:              "stats-flush-interval",
			TracingConfigJson:               "tracing-config-json",
			TelemetryCollectorBindSocketDir: "telemetry-collector-bind-socket-dir",
		},
	}

	res := resourcetest.Resource(pbmesh.ProxyConfigurationType, "test").
		WithData(t, proxyCfg).
		Build()

	err := ValidateProxyConfiguration(res)
	require.NoError(t, err)
}

func TestValidateProxyConfiguration_WorkloadSelector(t *testing.T) {
	type testcase struct {
		data      *pbmesh.ProxyConfiguration
		expectErr string
	}

	run := func(t *testing.T, tc testcase) {
		res := resourcetest.Resource(pbmesh.ProxyConfigurationType, "api").
			WithTenancy(resource.DefaultNamespacedTenancy()).
			WithData(t, tc.data).
			Build()

		// Ensure things are properly mutated and updated in the inputs.
		err := MutateProxyConfiguration(res)
		require.NoError(t, err)
		{
			mutated := resourcetest.MustDecode[*pbmesh.ProxyConfiguration](t, res)
			tc.data = mutated.Data
		}

		err = ValidateProxyConfiguration(res)

		// Verify that validate didn't actually change the object.
		got := resourcetest.MustDecode[*pbmesh.ProxyConfiguration](t, res)
		prototest.AssertDeepEqual(t, tc.data, got.Data)

		if tc.expectErr == "" {
			require.NoError(t, err)
		} else {
			testutil.RequireErrorContains(t, err, tc.expectErr)
		}
	}

	cases := map[string]testcase{
		// emptiness
		"empty": {
			data:      &pbmesh.ProxyConfiguration{},
			expectErr: `invalid "workloads" field: cannot be empty`,
		},
		"empty selector": {
			data: &pbmesh.ProxyConfiguration{
				Workloads: &pbcatalog.WorkloadSelector{},
			},
			expectErr: `invalid "workloads" field: cannot be empty`,
		},
		"bad selector": {
			data: &pbmesh.ProxyConfiguration{
				Workloads: &pbcatalog.WorkloadSelector{
					Names:  []string{"blah"},
					Filter: "garbage.foo == bar",
				},
			},
			expectErr: `invalid "filter" field: filter "garbage.foo == bar" is invalid: Selector "garbage" is not valid`,
		},
		"good selector": {
			data: &pbmesh.ProxyConfiguration{
				Workloads: &pbcatalog.WorkloadSelector{
					Names:  []string{"blah"},
					Filter: "metadata.foo == bar",
				},
				DynamicConfig: &pbmesh.DynamicConfig{},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}
