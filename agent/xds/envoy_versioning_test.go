package xds

import (
	"testing"

	envoycore "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	envoytype "github.com/envoyproxy/go-control-plane/envoy/type"
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
