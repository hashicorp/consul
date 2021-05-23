// +build !consulent

package config

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
