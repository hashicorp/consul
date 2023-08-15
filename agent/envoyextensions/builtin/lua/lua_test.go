// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package lua

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/envoyextensions/extensioncommon"
)

func TestConstructor(t *testing.T) {
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
		"default proxy type": {
			arguments: makeArguments(map[string]interface{}{"ProxyType": ""}),
			expected: lua{
				ProxyType: "connect-proxy",
				Listener:  "inbound",
				Script:    "lua-script",
			},
			ok: true,
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
			ext := extensioncommon.RuntimeConfig{
				ServiceName: svc,
				EnvoyExtension: api.EnvoyExtension{
					Name:      extensionName,
					Arguments: tc.arguments,
				},
			}

			e, err := Constructor(ext.EnvoyExtension)

			if tc.ok {
				require.NoError(t, err)
				require.Equal(t, &extensioncommon.BasicEnvoyExtender{Extension: &tc.expected}, e)
			} else {
				require.Error(t, err)
			}
		})
	}
}
