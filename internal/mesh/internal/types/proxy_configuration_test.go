// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/iptables"
	"github.com/hashicorp/consul/sdk/testutil"
)

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

func TestValidateProxyConfiguration(t *testing.T) {
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
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}
