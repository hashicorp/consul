// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package otelaccesslogging

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/envoyextensions/extensioncommon"
)

func TestConstructor(t *testing.T) {
	makeArguments := func(overrides map[string]interface{}) map[string]interface{} {
		m := map[string]interface{}{
			"ProxyType":    "connect-proxy",
			"ListenerType": "inbound",
			"Config": AccessLog{
				LogName: "access.log",
				GrpcService: &GrpcService{
					Target: &Target{
						Service: api.CompoundServiceName{
							Name:      "otel-collector",
							Namespace: "default",
							Partition: "default",
						},
					},
				},
			},
		}

		for k, v := range overrides {
			m[k] = v
		}

		return m
	}

	cases := map[string]struct {
		extensionName string
		arguments     map[string]interface{}
		expected      otelAccessLogging
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
		"invalid proxy type": {
			arguments: makeArguments(map[string]interface{}{"ProxyType": "terminating-gateway"}),
			ok:        false,
		},
		"invalid listener": {
			arguments: makeArguments(map[string]interface{}{"ListenerType": "invalid"}),
			ok:        false,
		},
		"default proxy type": {
			arguments: makeArguments(map[string]interface{}{"ProxyType": ""}),
			expected: otelAccessLogging{
				ProxyType:    "connect-proxy",
				ListenerType: "inbound",
				Config: AccessLog{
					LogName: "access.log",
					GrpcService: &GrpcService{
						Target: &Target{
							Service: api.CompoundServiceName{
								Name:      "otel-collector",
								Namespace: "default",
								Partition: "default",
							},
						},
					},
				},
			},
			ok: true,
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {

			extensionName := api.BuiltinOTELAccessLoggingExtension
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
