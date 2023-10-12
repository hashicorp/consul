// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/iptables"
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
