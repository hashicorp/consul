// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xdscommon

import (
	"fmt"
	"slices"
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
	const errTooOld = "is too old and is not supported by Consul"

	type testcase struct {
		name      string
		expect    SupportedProxyFeatures
		expectErr string
	}
	var cases []testcase

	// Bad versions.
	minMajorVersion := version.Must(version.NewVersion(getMinEnvoyVersion()))
	minMajorVersionMajorPart := minMajorVersion.Segments()[len(minMajorVersion.Segments())-2]
	for major := 9; major < minMajorVersionMajorPart; major++ {
		for minor := 0; minor < 10; minor++ {
			cases = append(cases, testcase{
				name:      version.Must(version.NewVersion(fmt.Sprintf("1.%d.%d", major, minor))).String(),
				expectErr: errTooOld,
			})
		}
	}

	// Good versions.
	// Sort ascending so test output is ordered like bad cases above.
	var supportedVersionsAscending []string
	supportedVersionsAscending = append(supportedVersionsAscending, EnvoyVersions...)
	slices.Reverse(supportedVersionsAscending)
	for _, v := range supportedVersionsAscending {
		envoyVersion := version.Must(version.NewVersion(v))
		// e.g. this is 27 in 1.27.4
		versionMajorPart := envoyVersion.Segments()[len(envoyVersion.Segments())-2]
		// e.g. this is 4 in 1.27.4
		versionMinorPart := envoyVersion.Segments()[len(envoyVersion.Segments())-1]

		// Create synthetic minor versions from .0 through the actual configured version.
		for minor := 0; minor <= versionMinorPart; minor++ {
			minorVersion := version.Must(version.NewVersion(fmt.Sprintf("1.%d.%d", versionMajorPart, minor)))
			cases = append(cases, testcase{
				name:   minorVersion.String(),
				expect: SupportedProxyFeatures{},
			})
		}
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			sf, err := DetermineSupportedProxyFeaturesFromString(tc.name)
			if tc.expectErr == "" {
				require.NoError(t, err)
				require.Equal(t, tc.expect, sf)
			} else {
				testutil.RequireErrorContains(t, err, tc.expectErr)
			}
		})
	}
}
