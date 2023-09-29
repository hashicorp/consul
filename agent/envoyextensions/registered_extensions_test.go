// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package envoyextensions

import (
	"fmt"
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/envoyextensions/extensioncommon"
	"github.com/stretchr/testify/require"
)

func TestValidateExtensions(t *testing.T) {
	tests := map[string]struct {
		input      []api.EnvoyExtension
		expectErrs []string
	}{
		"missing name": {
			input:      []api.EnvoyExtension{{}},
			expectErrs: []string{"Name is required"},
		},
		"bad name": {
			input: []api.EnvoyExtension{{
				Name: "bad",
			}},
			expectErrs: []string{"not a built-in extension"},
		},
		"multiple errors": {
			input: []api.EnvoyExtension{
				{},
				{
					Name: "bad",
				},
			},
			expectErrs: []string{
				"invalid EnvoyExtensions[0]: Name is required",
				"invalid EnvoyExtensions[1][bad]:",
			},
		},
		"invalid arguments to constructor": {
			input: []api.EnvoyExtension{{
				Name: "builtin/lua",
			}},
			expectErrs: []string{
				"invalid EnvoyExtensions[0][builtin/lua]",
				"missing Script value",
			},
		},
		"invalid consul version constraint": {
			input: []api.EnvoyExtension{{
				Name: "builtin/aws/lambda",
				Arguments: map[string]interface{}{
					"ARN": "arn:aws:lambda:us-east-1:111111111111:function:lambda-1234",
				},
				ConsulVersion: "bad",
			}},
			expectErrs: []string{
				"invalid EnvoyExtensions[0].ConsulVersion: Malformed constraint: bad",
			},
		},
		"invalid envoy version constraint": {
			input: []api.EnvoyExtension{{
				Name: "builtin/aws/lambda",
				Arguments: map[string]interface{}{
					"ARN": "arn:aws:lambda:us-east-1:111111111111:function:lambda-1234",
				},
				EnvoyVersion: "bad",
			}},
			expectErrs: []string{
				"invalid EnvoyExtensions[0].EnvoyVersion: Malformed constraint: bad",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := ValidateExtensions(tc.input)
			if len(tc.expectErrs) == 0 {
				require.NoError(t, err)
				return
			}
			for _, e := range tc.expectErrs {
				require.ErrorContains(t, err, e)
			}
		})
	}
}

// This test is included here so that we can test all registered extensions without creating a cyclic dependency between
// envoyextensions and extensioncommon.
func TestUpstreamExtenderLimitations(t *testing.T) {
	type testCase struct {
		config *extensioncommon.RuntimeConfig
		ok     bool
		errMsg string
	}
	unauthorizedExtensionCase := func(name string) testCase {
		return testCase{
			config: &extensioncommon.RuntimeConfig{
				Kind:                  api.ServiceKindConnectProxy,
				ServiceName:           api.CompoundServiceName{Name: "api"},
				Upstreams:             map[api.CompoundServiceName]*extensioncommon.UpstreamData{},
				IsSourcedFromUpstream: true,
				EnvoyExtension: api.EnvoyExtension{
					Name: name,
				},
			},
			ok:     false,
			errMsg: fmt.Sprintf("extension %q is not permitted to be applied via upstream service config", name),
		}
	}
	cases := map[string]testCase{
		// Make sure future extensions are theoretically covered, even if not registered in the same way.
		"unknown extension": unauthorizedExtensionCase("someotherextension"),
	}
	for name := range extensionConstructors {
		// AWS Lambda is the only extension permitted to modify downstream proxy resources.
		if name == api.BuiltinAWSLambdaExtension {
			continue
		}
		cases[name] = unauthorizedExtensionCase(name)
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			extender := extensioncommon.UpstreamEnvoyExtender{}
			_, err := extender.Extend(nil, tc.config)
			if tc.ok {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.ErrorContains(t, err, tc.errMsg)
			}
		})
	}
}
