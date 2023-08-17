// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package config

import (
	"fmt"
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
			json:        []string{`{ "server": true, "segment": "a" }`},
			hcl:         []string{` server = true segment = "a" `},
			expectedErr: `Network segments are not supported in this version of Consul`,
			expectedWarnings: []string{
				enterpriseConfigKeyError{key: "segment"}.Error(),
			},
		},
		{
			desc: "segment port must be set",
			args: []string{
				`-data-dir=` + dataDir,
			},
			json:        []string{`{ "segments":[{ "name":"x" }] }`},
			hcl:         []string{`segments = [{ name = "x" }]`},
			expectedErr: `Port for segment "x" cannot be <= 0`,
			expectedWarnings: []string{
				enterpriseConfigKeyError{key: "segments"}.Error(),
			},
		},
		{
			desc: "segments not in OSS",
			args: []string{
				`-data-dir=` + dataDir,
			},
			json:        []string{`{ "segments":[{ "name":"x", "port": 123 }] }`},
			hcl:         []string{`segments = [{ name = "x" port = 123 }]`},
			expectedErr: `Network segments are not supported in this version of Consul`,
			expectedWarnings: []string{
				enterpriseConfigKeyError{key: "segments"}.Error(),
			},
		},
	}

	for _, tc := range tests {
		for _, format := range []string{"json", "hcl"} {
			name := fmt.Sprintf("%v_%v", tc.desc, format)
			t.Run(name, tc.run(format, dataDir))
		}
	}
}
