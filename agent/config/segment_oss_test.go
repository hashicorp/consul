// +build !ent

package config

import (
	"os"
	"testing"

	"github.com/hashicorp/consul/testutil"
)

func TestSegments(t *testing.T) {
	dataDir := testutil.TempDir(t, "consul")
	defer os.RemoveAll(dataDir)

	tests := []configTest{
		{
			desc: "segment name not in OSS",
			args: []string{
				`-data-dir=` + dataDir,
			},
			json: []string{`{ "server": true, "segment": "a" }`},
			hcl:  []string{` server = true segment = "a" `},
			err:  `Network segments are not supported in this version of Consul`,
		},
		{
			desc: "segment port must be set",
			args: []string{
				`-data-dir=` + dataDir,
			},
			json: []string{`{ "segments":[{ "name":"x" }] }`},
			hcl:  []string{`segments = [{ name = "x" }]`},
			err:  `Port for segment "x" cannot be <= 0`,
		},
		{
			desc: "segments not in OSS",
			args: []string{
				`-data-dir=` + dataDir,
			},
			json: []string{`{ "segments":[{ "name":"x", "port": 123 }] }`},
			hcl:  []string{`segments = [{ name = "x" port = 123 }]`},
			err:  `Network segments are not supported in this version of Consul`,
		},
	}

	testConfig(t, tests, dataDir)
}
