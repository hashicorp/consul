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
			flags: []string{
				`-data-dir=` + dataDir,
			},
			json: []string{`{ "server": true, "segment": "a" }`},
			hcl:  []string{` server = true segment = "a" `},
			err:  `Network segments are not supported in this version of Consul`,
		},
		{
			desc: "segments not in OSS",
			flags: []string{
				`-data-dir=` + dataDir,
			},
			json: []string{`{ "segments":[{ "name":"x", "advertise": "unix:///foo" }] }`},
			hcl:  []string{`segments = [{ name = "x" advertise = "unix:///foo" }]`},
			err:  `Network segments are not supported in this version of Consul`,
		},
	}

	testConfig(t, tests, dataDir)
}
