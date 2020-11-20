// +build !consulent

package config

var authMethodEntFields = `{}`

var entMetaJSON = `{}`

var entRuntimeConfigSanitize = `{}`

var entTokenConfigSanitize = `"EnterpriseConfig": {},`

func entFullRuntimeConfig(rt *RuntimeConfig) {}

var enterpriseReadReplicaWarnings = []string{enterpriseConfigKeyError{key: "read_replica"}.Error()}

var enterpriseConfigKeyWarnings []string

func init() {
	for k := range enterpriseConfigMap {
		if k == "non_voting_server" {
			// this is an alias for "read_replica" so we shouldn't see it in warnings
			continue
		}
		enterpriseConfigKeyWarnings = append(enterpriseConfigKeyWarnings, enterpriseConfigKeyError{key: k}.Error())
	}
}
