// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package localratelimit

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
		}

		for k, v := range overrides {
			m[k] = v
		}

		return m
	}

	cases := map[string]struct {
		extensionName  string
		arguments      map[string]interface{}
		expected       ratelimit
		ok             bool
		expectedErrMsg string
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
		"MaxToken is missing": {
			arguments: makeArguments(map[string]interface{}{
				"ProxyType":     "connect-proxy",
				"FillInterval":  30,
				"TokensPerFill": 5,
			}),
			expectedErrMsg: "MaxTokens is missing",
			ok:             false,
		},
		"MaxTokens <= 0": {
			arguments: makeArguments(map[string]interface{}{
				"ProxyType":     "connect-proxy",
				"FillInterval":  30,
				"TokensPerFill": 5,
				"MaxTokens":     0,
			}),
			expectedErrMsg: "MaxTokens must be greater than 0",
			ok:             false,
		},
		"FillInterval is missing": {
			arguments: makeArguments(map[string]interface{}{
				"ProxyType":     "connect-proxy",
				"TokensPerFill": 5,
				"MaxTokens":     10,
			}),
			expectedErrMsg: "FillInterval(in second) is missing",
			ok:             false,
		},
		"FillInterval <= 0": {
			arguments: makeArguments(map[string]interface{}{
				"ProxyType":     "connect-proxy",
				"FillInterval":  0,
				"TokensPerFill": 5,
				"MaxTokens":     10,
			}),
			expectedErrMsg: "FillInterval(in second) must be greater than 0",
			ok:             false,
		},
		"TokensPerFill <= 0": {
			arguments: makeArguments(map[string]interface{}{
				"ProxyType":     "connect-proxy",
				"FillInterval":  30,
				"TokensPerFill": 0,
				"MaxTokens":     10,
			}),
			expectedErrMsg: "TokensPerFill must be greater than 0",
			ok:             false,
		},
		"FilterEnabled < 0": {
			arguments: makeArguments(map[string]interface{}{
				"ProxyType":     "connect-proxy",
				"FillInterval":  30,
				"TokensPerFill": 5,
				"MaxTokens":     10,
				"FilterEnabled": -1,
			}),
			expectedErrMsg: "cannot parse 'FilterEnabled', -1 overflows uint",
			ok:             false,
		},
		"FilterEnforced < 0": {
			arguments: makeArguments(map[string]interface{}{
				"ProxyType":      "connect-proxy",
				"FillInterval":   30,
				"TokensPerFill":  5,
				"MaxTokens":      10,
				"FilterEnforced": -1,
			}),
			expectedErrMsg: "cannot parse 'FilterEnforced', -1 overflows uint",
			ok:             false,
		},
		"invalid proxy type": {
			arguments: makeArguments(map[string]interface{}{
				"ProxyType":     "invalid",
				"FillInterval":  30,
				"MaxTokens":     20,
				"TokensPerFill": 5,
			}),
			expectedErrMsg: `unexpected ProxyType "invalid"`,
			ok:             false,
		},
		"default proxy type": {
			arguments: makeArguments(map[string]interface{}{
				"FillInterval":  30,
				"MaxTokens":     20,
				"TokensPerFill": 5,
			}),
			expected: ratelimit{
				ProxyType:     "connect-proxy",
				MaxTokens:     intPointer(20),
				FillInterval:  intPointer(30),
				TokensPerFill: intPointer(5),
			},
			ok: true,
		},
		"valid everything": {
			arguments: makeArguments(map[string]interface{}{
				"ProxyType":     "connect-proxy",
				"FillInterval":  30,
				"MaxTokens":     20,
				"TokensPerFill": 5,
			}),
			expected: ratelimit{
				ProxyType:     "connect-proxy",
				MaxTokens:     intPointer(20),
				FillInterval:  intPointer(30),
				TokensPerFill: intPointer(5),
			},
			ok: true,
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {

			extensionName := api.BuiltinLocalRatelimitExtension
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
				require.Contains(t, err.Error(), tc.expectedErrMsg)
			}
		})
	}
}

func intPointer(i int) *int {
	return &i
}
