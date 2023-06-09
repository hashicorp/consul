// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package xdscommon

import (
	"testing"

	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_type_v3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"

	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/sdk/testutil"
)

func TestDetermineEnvoyVersionFromNode(t *testing.T) {
	cases := map[string]struct {
		node   *envoy_core_v3.Node
		expect *version.Version
	}{
		"empty": {
			node:   &envoy_core_v3.Node{},
			expect: nil,
		},
		"user agent build version but no user agent": {
			node: &envoy_core_v3.Node{
				UserAgentName: "",
				UserAgentVersionType: &envoy_core_v3.Node_UserAgentBuildVersion{
					UserAgentBuildVersion: &envoy_core_v3.BuildVersion{
						Version: &envoy_type_v3.SemanticVersion{
							MajorNumber: 1,
							MinorNumber: 14,
							Patch:       4,
						},
					},
				},
			},
			expect: nil,
		},
		"user agent build version with user agent": {
			node: &envoy_core_v3.Node{
				UserAgentName: "envoy",
				UserAgentVersionType: &envoy_core_v3.Node_UserAgentBuildVersion{
					UserAgentBuildVersion: &envoy_core_v3.BuildVersion{
						Version: &envoy_type_v3.SemanticVersion{
							MajorNumber: 1,
							MinorNumber: 14,
							Patch:       4,
						},
					},
				},
			},
			expect: version.Must(version.NewVersion("1.14.4")),
		},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			got := DetermineEnvoyVersionFromNode(tc.node)
			if tc.expect != nil {
				require.Equal(t, tc.expect, got)
			} else {
				require.Nil(t, got)
			}
		})
	}
}

func TestDetermineSupportedProxyFeaturesFromString(t *testing.T) {
	const (
		errTooOld = "is too old and is not supported by Consul"
	)

	type testcase struct {
		expect    SupportedProxyFeatures
		expectErr string
	}

	// Just the bad versions
	cases := map[string]testcase{
		"1.9.0":   {expectErr: "Envoy 1.9.0 " + errTooOld},
		"1.10.0":  {expectErr: "Envoy 1.10.0 " + errTooOld},
		"1.11.0":  {expectErr: "Envoy 1.11.0 " + errTooOld},
		"1.12.0":  {expectErr: "Envoy 1.12.0 " + errTooOld},
		"1.12.1":  {expectErr: "Envoy 1.12.1 " + errTooOld},
		"1.12.2":  {expectErr: "Envoy 1.12.2 " + errTooOld},
		"1.12.3":  {expectErr: "Envoy 1.12.3 " + errTooOld},
		"1.12.4":  {expectErr: "Envoy 1.12.4 " + errTooOld},
		"1.12.5":  {expectErr: "Envoy 1.12.5 " + errTooOld},
		"1.12.6":  {expectErr: "Envoy 1.12.6 " + errTooOld},
		"1.12.7":  {expectErr: "Envoy 1.12.7 " + errTooOld},
		"1.13.0":  {expectErr: "Envoy 1.13.0 " + errTooOld},
		"1.13.1":  {expectErr: "Envoy 1.13.1 " + errTooOld},
		"1.13.2":  {expectErr: "Envoy 1.13.2 " + errTooOld},
		"1.13.3":  {expectErr: "Envoy 1.13.3 " + errTooOld},
		"1.13.4":  {expectErr: "Envoy 1.13.4 " + errTooOld},
		"1.13.5":  {expectErr: "Envoy 1.13.5 " + errTooOld},
		"1.13.6":  {expectErr: "Envoy 1.13.6 " + errTooOld},
		"1.13.7":  {expectErr: "Envoy 1.13.7 " + errTooOld},
		"1.14.0":  {expectErr: "Envoy 1.14.0 " + errTooOld},
		"1.14.1":  {expectErr: "Envoy 1.14.1 " + errTooOld},
		"1.14.2":  {expectErr: "Envoy 1.14.2 " + errTooOld},
		"1.14.3":  {expectErr: "Envoy 1.14.3 " + errTooOld},
		"1.14.4":  {expectErr: "Envoy 1.14.4 " + errTooOld},
		"1.14.5":  {expectErr: "Envoy 1.14.5 " + errTooOld},
		"1.14.6":  {expectErr: "Envoy 1.14.6 " + errTooOld},
		"1.14.7":  {expectErr: "Envoy 1.14.7 " + errTooOld},
		"1.15.0":  {expectErr: "Envoy 1.15.0 " + errTooOld},
		"1.15.1":  {expectErr: "Envoy 1.15.1 " + errTooOld},
		"1.15.2":  {expectErr: "Envoy 1.15.2 " + errTooOld},
		"1.15.3":  {expectErr: "Envoy 1.15.3 " + errTooOld},
		"1.15.4":  {expectErr: "Envoy 1.15.4 " + errTooOld},
		"1.15.5":  {expectErr: "Envoy 1.15.5 " + errTooOld},
		"1.16.1":  {expectErr: "Envoy 1.16.1 " + errTooOld},
		"1.16.2":  {expectErr: "Envoy 1.16.2 " + errTooOld},
		"1.16.3":  {expectErr: "Envoy 1.16.3 " + errTooOld},
		"1.16.4":  {expectErr: "Envoy 1.16.4 " + errTooOld},
		"1.16.5":  {expectErr: "Envoy 1.16.5 " + errTooOld},
		"1.16.6":  {expectErr: "Envoy 1.16.6 " + errTooOld},
		"1.17.4":  {expectErr: "Envoy 1.17.4 " + errTooOld},
		"1.18.6":  {expectErr: "Envoy 1.18.6 " + errTooOld},
		"1.19.5":  {expectErr: "Envoy 1.19.5 " + errTooOld},
		"1.20.7":  {expectErr: "Envoy 1.20.7 " + errTooOld},
		"1.21.5":  {expectErr: "Envoy 1.21.5 " + errTooOld},
		"1.22.0":  {expectErr: "Envoy 1.22.0 " + errTooOld},
		"1.22.1":  {expectErr: "Envoy 1.22.1 " + errTooOld},
		"1.22.2":  {expectErr: "Envoy 1.22.2 " + errTooOld},
		"1.22.3":  {expectErr: "Envoy 1.22.3 " + errTooOld},
		"1.22.4":  {expectErr: "Envoy 1.22.4 " + errTooOld},
		"1.22.5":  {expectErr: "Envoy 1.22.5 " + errTooOld},
		"1.22.6":  {expectErr: "Envoy 1.22.6 " + errTooOld},
		"1.22.7":  {expectErr: "Envoy 1.22.7 " + errTooOld},
		"1.22.8":  {expectErr: "Envoy 1.22.8 " + errTooOld},
		"1.22.9":  {expectErr: "Envoy 1.22.9 " + errTooOld},
		"1.22.10": {expectErr: "Envoy 1.22.10 " + errTooOld},
		"1.22.11": {expectErr: "Envoy 1.22.11 " + errTooOld},
	}

	// Insert a bunch of valid versions.
	// Populate feature flags here when appropriate. See consul 1.10.x for reference.
	/* Example from 1.18
	for _, v := range []string{
		"1.18.0", "1.18.1", "1.18.2", "1.18.3", "1.18.4", "1.18.5", "1.18.6",
	} {
		cases[v] = testcase{expect: SupportedProxyFeatures{
			ForceLDSandCDSToAlwaysUseWildcardsOnReconnect: true,
		}}
	}
	*/
	for _, v := range []string{
		"1.23.0", "1.23.1", "1.23.2", "1.23.3", "1.23.4", "1.23.5", "1.23.6", "1.23.7", "1.23.8", "1.23.9", "1.23.10",
		"1.24.0", "1.24.1", "1.24.2", "1.24.3", "1.24.4", "1.24.5", "1.24.6", "1.24.7", "1.24.8",
		"1.25.0", "1.25.1", "1.25.2", "1.25.3", "1.25.4", "1.25.5", "1.25.6", "1.25.7",
		"1.26.0", "1.26.1", "1.26.2",
	} {
		cases[v] = testcase{expect: SupportedProxyFeatures{}}
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			sf, err := DetermineSupportedProxyFeaturesFromString(name)
			if tc.expectErr == "" {
				require.NoError(t, err)
				require.Equal(t, tc.expect, sf)
			} else {
				testutil.RequireErrorContains(t, err, tc.expectErr)
			}
		})
	}
}
