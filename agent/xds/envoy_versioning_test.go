package xds

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
			got := determineEnvoyVersionFromNode(tc.node)
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
		err1_13   = "is too old of a point release and is not supported by Consul because it does not support RBAC rules using url_path. Please upgrade to version 1.13.1+."
		errTooOld = "is too old and is not supported by Consul"
	)

	type testcase struct {
		expect    supportedProxyFeatures
		expectErr string
	}

	// Just the bad versions
	cases := map[string]testcase{
		"1.9.0":  {expectErr: "Envoy 1.9.0 " + errTooOld},
		"1.10.0": {expectErr: "Envoy 1.10.0 " + errTooOld},
		"1.11.0": {expectErr: "Envoy 1.11.0 " + errTooOld},
		"1.12.0": {expectErr: "Envoy 1.12.0 " + errTooOld},
		"1.12.1": {expectErr: "Envoy 1.12.1 " + errTooOld},
		"1.12.2": {expectErr: "Envoy 1.12.2 " + errTooOld},
		"1.12.3": {expectErr: "Envoy 1.12.3 " + errTooOld},
		"1.12.4": {expectErr: "Envoy 1.12.4 " + errTooOld},
		"1.12.5": {expectErr: "Envoy 1.12.5 " + errTooOld},
		"1.12.6": {expectErr: "Envoy 1.12.6 " + errTooOld},
		"1.12.7": {expectErr: "Envoy 1.12.7 " + errTooOld},
		"1.13.0": {expectErr: "Envoy 1.13.0 " + errTooOld},
		"1.13.1": {expectErr: "Envoy 1.13.1 " + errTooOld},
		"1.13.2": {expectErr: "Envoy 1.13.2 " + errTooOld},
		"1.13.3": {expectErr: "Envoy 1.13.3 " + errTooOld},
		"1.13.4": {expectErr: "Envoy 1.13.4 " + errTooOld},
		"1.13.5": {expectErr: "Envoy 1.13.5 " + errTooOld},
		"1.13.6": {expectErr: "Envoy 1.13.6 " + errTooOld},
		"1.13.7": {expectErr: "Envoy 1.13.7 " + errTooOld},
		"1.14.0": {expectErr: "Envoy 1.14.0 " + errTooOld},
		"1.14.1": {expectErr: "Envoy 1.14.1 " + errTooOld},
		"1.14.2": {expectErr: "Envoy 1.14.2 " + errTooOld},
		"1.14.3": {expectErr: "Envoy 1.14.3 " + errTooOld},
		"1.14.4": {expectErr: "Envoy 1.14.4 " + errTooOld},
		"1.14.5": {expectErr: "Envoy 1.14.5 " + errTooOld},
		"1.14.6": {expectErr: "Envoy 1.14.6 " + errTooOld},
		"1.14.7": {expectErr: "Envoy 1.14.7 " + errTooOld},
	}

	// Insert a bunch of valid versions.
	for _, v := range []string{
		"1.15.0", "1.15.1", "1.15.2", "1.15.3", "1.15.4", "1.15.5",
	} {
		cases[v] = testcase{expect: supportedProxyFeatures{
			GatewaysNeedStubClusterWhenEmptyWithIncrementalXDS: true,
			IncrementalXDSUpdatesMustBeSerial:                  true,
		}}
	}
	for _, v := range []string{
		"1.16.0", "1.16.1", "1.16.2", "1.16.3", "1.16.4",
		"1.17.0", "1.17.1", "1.17.2", "1.17.3",
		"1.18.0", "1.18.1", "1.18.2", "1.18.3",
	} {
		cases[v] = testcase{expect: supportedProxyFeatures{}}
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			sf, err := determineSupportedProxyFeaturesFromString(name)
			if tc.expectErr == "" {
				require.Equal(t, tc.expect, sf)
			} else {
				testutil.RequireErrorContains(t, err, tc.expectErr)
			}
		})
	}
}
