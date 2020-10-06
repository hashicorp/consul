package xds

import (
	"testing"

	envoycore "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	envoytype "github.com/envoyproxy/go-control-plane/envoy/type"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/require"
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
		"only build version": {
			node: &envoycore.Node{
				BuildVersion: "1580db37e9a97c37e410bad0e1507ae1a0fd9e77/1.9.0/Clean/RELEASE/BoringSSL",
			},
			expect: version.Must(version.NewVersion("1.9.0")),
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
		err1_12   = "is too old of a point release and is not supported by Consul because it does not support RBAC rules using url_path. Please upgrade to version 1.12.3+."
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
		"1.12.0": {expectErr: "Envoy 1.12.0 " + err1_12},
		"1.12.1": {expectErr: "Envoy 1.12.1 " + err1_12},
		"1.12.2": {expectErr: "Envoy 1.12.2 " + err1_12},
		"1.13.0": {expectErr: "Envoy 1.13.0 " + err1_13},
	}

	// Insert a bunch of valid versions.
	for _, v := range []string{
		"1.12.3", "1.12.4", "1.12.5", "1.12.6", "1.12.7",
		"1.13.1", "1.13.2", "1.13.3", "1.13.4", "1.13.6", "1.14.1",
		"1.14.2", "1.14.3", "1.14.4", "1.14.5",
		"1.15.0", "1.15.1", "1.15.2",
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
