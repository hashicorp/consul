// +build !consulent

package config

var entMetaJSON = `{}`

var entRuntimeConfigSanitize = `{}`

var entTokenConfigSanitize = `"EnterpriseConfig": {},`

func entFullRuntimeConfig(rt *RuntimeConfig) {}

var enterpriseNonVotingServerWarnings []string = []string{enterpriseConfigKeyError{key: "non_voting_server"}.Error()}

var enterpriseConfigKeyWarnings []string

func init() {
	for k := range enterpriseConfigMap {
		enterpriseConfigKeyWarnings = append(enterpriseConfigKeyWarnings, enterpriseConfigKeyError{key: k}.Error())
	}
}
