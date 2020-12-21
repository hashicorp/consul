// +build !consulent

package config

import (
	"testing"

	"github.com/hashicorp/consul/sdk/testutil"
)

func TestSegments(t *testing.T) {
	dataDir := testutil.TempDir(t, "consul")

	tests := []testCase{
		{
			desc: "segment name not in OSS",
			args: []string{
				`-data-dir=` + dataDir,
			},
			json: []string{`{ "server": true, "segment": "a" }`},
			hcl:  []string{` server = true segment = "a" `},
			err:  `Network segments are not supported in this version of Consul`,
			warns: []string{
				enterpriseConfigKeyError{key: "segment"}.Error(),
			},
		},
		{
			desc: "segment port must be set",
			args: []string{
				`-data-dir=` + dataDir,
			},
			json: []string{`{ "segments":[{ "name":"x" }] }`},
			hcl:  []string{`segments = [{ name = "x" }]`},
			err:  `Port for segment "x" cannot be <= 0`,
			warns: []string{
				enterpriseConfigKeyError{key: "segments"}.Error(),
			},
		},
		{
			desc: "segments not in OSS",
			args: []string{
				`-data-dir=` + dataDir,
			},
			json: []string{`{ "segments":[{ "name":"x", "port": 123 }] }`},
			hcl:  []string{`segments = [{ name = "x" port = 123 }]`},
			err:  `Network segments are not supported in this version of Consul`,
			warns: []string{
				enterpriseConfigKeyError{key: "segments"}.Error(),
			},
		},
	}

	for _, tt := range tests {
		for _, format := range []string{"json", "hcl"} {
			testConfig(t, tt, format, dataDir)
		}
	}
}
