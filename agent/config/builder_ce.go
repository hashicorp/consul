// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package config

import (
	"fmt"
)

// validateEnterpriseConfig is a function to validate the enterprise specific
// configuration items after Parsing but before merging into the overall
// configuration. The original intent is to use it to ensure that we warn
// for enterprise configurations used in CE.
func validateEnterpriseConfigKeys(config *Config) []error {
	var result []error
	add := func(k string) {
		result = append(result, enterpriseConfigKeyError{key: k})
	}

	if config.ReadReplica != nil {
		add(`read_replica (or the deprecated non_voting_server)`)
	}
	if stringVal(config.SegmentName) != "" {
		add("segment")
	}
	if len(config.Segments) > 0 {
		add("segments")
	}
	if stringVal(config.Partition) != "" {
		add("partition")
	}
	if stringVal(config.Autopilot.RedundancyZoneTag) != "" {
		add("autopilot.redundancy_zone_tag")
	}
	if stringVal(config.Autopilot.UpgradeVersionTag) != "" {
		add("autopilot.upgrade_version_tag")
	}
	if config.Autopilot.DisableUpgradeMigration != nil {
		add("autopilot.disable_upgrade_migration")
	}
	if config.DNS.PreferNamespace != nil {
		add("dns_config.prefer_namespace")
		config.DNS.PreferNamespace = nil
	}
	if config.ACL.MSPDisableBootstrap != nil {
		add("acl.msp_disable_bootstrap")
		config.ACL.MSPDisableBootstrap = nil
	}
	if len(config.ACL.Tokens.ManagedServiceProvider) > 0 {
		add("acl.tokens.managed_service_provider")
		config.ACL.Tokens.ManagedServiceProvider = nil
	}
	if boolVal(config.Audit.Enabled) || len(config.Audit.Sinks) > 0 {
		add("audit")
	}
	if config.LicensePath != nil {
		add("license_path")
		config.LicensePath = nil
	}
	if config.Reporting.License.Enabled != nil {
		add("reporting.license.enabled")
		config.Reporting.License.Enabled = nil
	}

	return result
}

type enterpriseConfigKeyError struct {
	key string
}

func (e enterpriseConfigKeyError) Error() string {
	return fmt.Sprintf("%q is a Consul Enterprise configuration and will have no effect", e.key)
}

func (*builder) BuildEnterpriseRuntimeConfig(_ *RuntimeConfig, _ *Config) error {
	return nil
}

func (*builder) validateEnterpriseConfig(_ RuntimeConfig) error {
	return nil
}
