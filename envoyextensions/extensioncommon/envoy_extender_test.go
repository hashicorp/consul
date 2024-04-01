// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package extensioncommon

import (
	"fmt"
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/require"
)

func TestUpstreamConfigSourceLimitations(t *testing.T) {
	type testCase struct {
		extender EnvoyExtender
		config   *RuntimeConfig
		ok       bool
		errMsg   string
	}
	cases := map[string]testCase{
		"upstream extender non-upstream config": {
			extender: &UpstreamEnvoyExtender{},
			config: &RuntimeConfig{
				Kind:                  api.ServiceKindConnectProxy,
				ServiceName:           api.CompoundServiceName{Name: "api"},
				Upstreams:             map[api.CompoundServiceName]*UpstreamData{},
				IsSourcedFromUpstream: false,
				EnvoyExtension: api.EnvoyExtension{
					Name: api.BuiltinAWSLambdaExtension,
				},
			},
			ok:     false,
			errMsg: fmt.Sprintf("%q extension applied as upstream config but is not sourced from an upstream of the local service", api.BuiltinAWSLambdaExtension),
		},
		"basic extender upstream config": {
			extender: &BasicEnvoyExtender{},
			config: &RuntimeConfig{
				Kind:                  api.ServiceKindConnectProxy,
				ServiceName:           api.CompoundServiceName{Name: "api"},
				Upstreams:             map[api.CompoundServiceName]*UpstreamData{},
				IsSourcedFromUpstream: true,
				EnvoyExtension: api.EnvoyExtension{
					Name: api.BuiltinLuaExtension,
				},
			},
			ok:     false,
			errMsg: fmt.Sprintf("%q extension applied as local config but is sourced from an upstream of the local service", api.BuiltinLuaExtension),
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			_, err := tc.extender.Extend(nil, tc.config)
			if tc.ok {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.ErrorContains(t, err, tc.errMsg)
			}
		})
	}
}
