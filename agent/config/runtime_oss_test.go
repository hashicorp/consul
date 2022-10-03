//go:build !consulent
// +build !consulent

package config

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/consul/sdk/testutil"
)

var testRuntimeConfigSanitizeExpectedFilename = "TestRuntimeConfig_Sanitize.golden"

func entFullRuntimeConfig(rt *RuntimeConfig) {}

var enterpriseReadReplicaWarnings = []string{enterpriseConfigKeyError{key: "read_replica (or the deprecated non_voting_server)"}.Error()}

var enterpriseConfigKeyWarnings = []string{
	enterpriseConfigKeyError{key: "license_path"}.Error(),
	enterpriseConfigKeyError{key: "read_replica (or the deprecated non_voting_server)"}.Error(),
	enterpriseConfigKeyError{key: "autopilot.redundancy_zone_tag"}.Error(),
	enterpriseConfigKeyError{key: "autopilot.upgrade_version_tag"}.Error(),
	enterpriseConfigKeyError{key: "autopilot.disable_upgrade_migration"}.Error(),
	enterpriseConfigKeyError{key: "dns_config.prefer_namespace"}.Error(),
	enterpriseConfigKeyError{key: "acl.msp_disable_bootstrap"}.Error(),
	enterpriseConfigKeyError{key: "acl.tokens.managed_service_provider"}.Error(),
	enterpriseConfigKeyError{key: "audit"}.Error(),
}

// OSS-only equivalent of TestConfigFlagsAndEdgecases
// used for flags validated in ent-only code
func TestLoad_IntegrationWithFlags_OSS(t *testing.T) {
	dataDir := testutil.TempDir(t, "consul")
	defer os.RemoveAll(dataDir)

	tests := []testCase{
		{
			desc: "partition config on a client",
			args: []string{
				`-data-dir=` + dataDir,
				`-server=false`,
			},
			json: []string{`{ "partition": "foo" }`},
			hcl:  []string{`partition = "foo"`},
			expectedWarnings: []string{
				`"partition" is a Consul Enterprise configuration and will have no effect`,
			},
			expected: func(rt *RuntimeConfig) {
				rt.DataDir = dataDir
				rt.ServerMode = false
			},
		},
		{
			desc: "partition config on a server",
			args: []string{
				`-data-dir=` + dataDir,
				`-server`,
			},
			json: []string{`{ "partition": "foo" }`},
			hcl:  []string{`partition = "foo"`},
			expectedWarnings: []string{
				`"partition" is a Consul Enterprise configuration and will have no effect`,
			},
			expected: func(rt *RuntimeConfig) {
				rt.DataDir = dataDir
				rt.ServerMode = true
				rt.TLS.ServerMode = true
				rt.LeaveOnTerm = false
				rt.SkipLeaveOnInt = true
				rt.RPCConfig.EnableStreaming = true
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
