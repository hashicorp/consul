package meshv2beta1

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsTransprentProxy(t *testing.T) {
	cases := map[string]struct {
		proxyCfg *ProxyConfiguration
		exp      bool
	}{
		"nil dynamic config": {
			proxyCfg: &ProxyConfiguration{},
			exp:      false,
		},
		"default mode": {
			proxyCfg: &ProxyConfiguration{
				DynamicConfig: &DynamicConfig{
					Mode: ProxyMode_PROXY_MODE_DEFAULT,
				},
			},
			exp: false,
		},
		"direct mode": {
			proxyCfg: &ProxyConfiguration{
				DynamicConfig: &DynamicConfig{
					Mode: ProxyMode_PROXY_MODE_DEFAULT,
				},
			},
			exp: false,
		},
		"transparent mode": {
			proxyCfg: &ProxyConfiguration{
				DynamicConfig: &DynamicConfig{
					Mode: ProxyMode_PROXY_MODE_TRANSPARENT,
				},
			},
			exp: true,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, c.exp, c.proxyCfg.IsTransparentProxy())
		})
	}
}
