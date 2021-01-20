package xds

import (
	"testing"

	envoycore "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	envoytype "github.com/envoyproxy/go-control-plane/envoy/type"

	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/sdk/testutil"
)

func TestDetermineEnvoyVersionFromNode(t *testing.T) {
	cases := map[string]struct {
		node   *envoycore.Node
		expect *version.Version
	}{
		"empty": {
			node:   &envoycore.Node{},
			expect: nil,
		},
		"user agent build version but no user agent": {
			node: &envoycore.Node{
				UserAgentName: "",
				UserAgentVersionType: &envoycore.Node_UserAgentBuildVersion{
					UserAgentBuildVersion: &envoycore.BuildVersion{
						Version: &envoytype.SemanticVersion{
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
			node: &envoycore.Node{
				UserAgentName: "envoy",
				UserAgentVersionType: &envoycore.Node_UserAgentBuildVersion{
					UserAgentBuildVersion: &envoycore.BuildVersion{
						Version: &envoytype.SemanticVersion{
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
	}

	// Insert a bunch of valid versions.
	for _, v := range []string{
		"1.14.1", "1.14.2", "1.14.3", "1.14.4", "1.14.5", "1.14.6",
		"1.15.0", "1.15.1", "1.15.2", "1.15.3",
		"1.16.0", "1.16.1", "1.16.2",
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
