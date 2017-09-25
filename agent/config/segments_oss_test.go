// +build !ent

package config

import (
	"net"
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
			privatev4: func() ([]*net.IPAddr, error) {
				return []*net.IPAddr{ipAddr("10.0.0.1")}, nil
			},
		},
		{
			desc: "segments not in OSS",
			flags: []string{
				`-data-dir=` + dataDir,
			},
			json: []string{`{ "segments":[{ "name":"x", "advertise": "unix:///foo" }] }`},
			hcl:  []string{`segments = [{ name = "x" advertise = "unix:///foo" }]`},
			err:  `Network segments are not supported in this version of Consul`,
			privatev4: func() ([]*net.IPAddr, error) {
				return []*net.IPAddr{ipAddr("10.0.0.1")}, nil
			},
		},
	}

	testConfig(t, tests, dataDir)
}
