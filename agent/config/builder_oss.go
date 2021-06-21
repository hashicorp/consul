// +build !consulent

package config

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
)

var (
	enterpriseConfigMap map[string]func(*Config) = map[string]func(c *Config){
		"non_voting_server": func(c *Config) {
			// to maintain existing compatibility we don't nullify the value
		},
		"segment": func(c *Config) {
			// to maintain existing compatibility we don't nullify the value
		},
		"segments": func(c *Config) {
			// to maintain existing compatibility we don't nullify the value
		},
		"autopilot.redundancy_zone_tag": func(c *Config) {
			// to maintain existing compatibility we don't nullify the value
		},
		"autopilot.upgrade_version_tag": func(c *Config) {
			// to maintain existing compatibility we don't nullify the value
		},
		"autopilot.disable_upgrade_migration": func(c *Config) {
			// to maintain existing compatibility we don't nullify the value
		},
		"dns_config.prefer_namespace": func(c *Config) {
			c.DNS.PreferNamespace = nil
		},
		"acl.msp_disable_bootstrap": func(c *Config) {
			c.ACL.MSPDisableBootstrap = nil
		},
		"acl.tokens.managed_service_provider": func(c *Config) {
			c.ACL.Tokens.ManagedServiceProvider = nil
		},
		"audit": func(c *Config) {
			c.Audit = nil
		},
		"license_path": func(c *Config) {
			c.LicensePath = nil
		},
	}
)

type enterpriseConfigKeyError struct {
	key string
}

func (e enterpriseConfigKeyError) Error() string {
	return fmt.Sprintf("%q is a Consul Enterprise configuration and will have no effect", e.key)
}

func (_ *Builder) BuildEnterpriseRuntimeConfig(_ *Config) (EnterpriseRuntimeConfig, error) {
	return EnterpriseRuntimeConfig{}, nil
}

// validateEnterpriseConfig is a function to validate the enterprise specific
// configuration items after Parsing but before merging into the overall
// configuration. The original intent is to use it to ensure that we warn
// for enterprise configurations used in OSS.
func (b *Builder) validateEnterpriseConfigKeys(config *Config, keys []string) error {
	var err error

	for _, k := range keys {
		if unset, ok := enterpriseConfigMap[k]; ok {
			keyErr := enterpriseConfigKeyError{key: k}

			b.warn(keyErr.Error())
			err = multierror.Append(err, keyErr)
			unset(config)
		}
	}

	return err
}
