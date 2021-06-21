// +build !consulent

package config

var entMetaJSON = `{}`

var entAuthMethodFieldsJSON = `{}`

var entRuntimeConfigSanitize = `{}`

var entFullDNSJSONConfig = ``

var entFullDNSHCLConfig = ``

var entFullRuntimeConfig = EnterpriseRuntimeConfig{}

var enterpriseNonVotingServerWarnings []string = []string{enterpriseConfigKeyError{key: "non_voting_server"}.Error()}

var enterpriseConfigKeyWarnings []string

func init() {
	for k, _ := range enterpriseConfigMap {
		enterpriseConfigKeyWarnings = append(enterpriseConfigKeyWarnings, enterpriseConfigKeyError{key: k}.Error())
	}
}
