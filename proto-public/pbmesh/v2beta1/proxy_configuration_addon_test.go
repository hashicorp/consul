// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package meshv2beta1

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsTransparentProxy(t *testing.T) {
	cases := map[string]struct {
		dynamicConfig *DynamicConfig
		exp           bool
	}{
		"nil dynamic config": {
			dynamicConfig: nil,
			exp:           false,
		},
		"default mode": {
			dynamicConfig: &DynamicConfig{
				Mode: ProxyMode_PROXY_MODE_DEFAULT,
			},
			exp: false,
		},
		"direct mode": {
			dynamicConfig: &DynamicConfig{
				Mode: ProxyMode_PROXY_MODE_DEFAULT,
			},
			exp: false,
		},
		"transparent mode": {
			dynamicConfig: &DynamicConfig{
				Mode: ProxyMode_PROXY_MODE_TRANSPARENT,
			},
			exp: true,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			proxyCfg := &ProxyConfiguration{
				DynamicConfig: c.dynamicConfig,
			}
			compProxyCfg := &ComputedProxyConfiguration{
				DynamicConfig: c.dynamicConfig,
			}
			require.Equal(t, c.exp, proxyCfg.IsTransparentProxy())
			require.Equal(t, c.exp, compProxyCfg.IsTransparentProxy())
		})
	}
}
