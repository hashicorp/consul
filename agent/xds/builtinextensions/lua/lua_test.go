package lua

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/xds/xdscommon"
	"github.com/hashicorp/consul/api"
)

func TestMakeLuaPatcher(t *testing.T) {
	makeArguments := func(overrides map[string]interface{}) map[string]interface{} {
		m := map[string]interface{}{
			"ProxyType": "connect-proxy",
			"Listener":  "inbound",
			"Script":    "lua-script",
		}

		for k, v := range overrides {
			m[k] = v
		}

		return m
	}

	cases := map[string]struct {
		extensionName string
		arguments     map[string]interface{}
		expected      lua
		ok            bool
	}{
		"with no arguments": {
			arguments: nil,
			ok:        false,
		},
		"with an invalid name": {
			arguments:     makeArguments(map[string]interface{}{}),
			extensionName: "bad",
			ok:            false,
		},
		"empty script": {
			arguments: makeArguments(map[string]interface{}{"Script": ""}),
			ok:        false,
		},
		"invalid proxy type": {
			arguments: makeArguments(map[string]interface{}{"ProxyType": "terminating-gateway"}),
			ok:        false,
		},
		"invalid listener": {
			arguments: makeArguments(map[string]interface{}{"Listener": "invalid"}),
			ok:        false,
		},
		"valid everything": {
			arguments: makeArguments(map[string]interface{}{}),
			expected: lua{
				ProxyType: "connect-proxy",
				Listener:  "inbound",
				Script:    "lua-script",
			},
			ok: true,
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {

			extensionName := api.BuiltinLuaExtension
			if tc.extensionName != "" {
				extensionName = tc.extensionName
			}

			svc := api.CompoundServiceName{Name: "svc"}
			ext := xdscommon.ExtensionConfiguration{
				ServiceName: svc,
				EnvoyExtension: api.EnvoyExtension{
					Name:      extensionName,
					Arguments: tc.arguments,
				},
			}

			patcher, err := MakeLuaExtension(ext)

			if tc.ok {
				require.NoError(t, err)
				require.Equal(t, tc.expected, patcher)
			} else {
				require.Error(t, err)
			}
		})
	}
}
