package config

import "fmt"

type DeprecatedConfig struct {
	// DEPRECATED (ACL-Legacy-Compat) - moved into the "acl.tokens" stanza
	ACLAgentMasterToken *string `mapstructure:"acl_agent_master_token"`
	// DEPRECATED (ACL-Legacy-Compat) - moved to "primary_datacenter"
	ACLDatacenter *string `mapstructure:"acl_datacenter"`
}

func applyDeprecatedConfig(d *decodeTarget) (Config, []string) {
	dep := d.DeprecatedConfig
	var warns []string

	if dep.ACLAgentMasterToken != nil {
		if d.Config.ACL.Tokens.AgentMaster == nil {
			d.Config.ACL.Tokens.AgentMaster = dep.ACLAgentMasterToken
		}
		warns = append(warns, deprecationWarning("acl_agent_master_token", "acl.tokens.agent_master"))
	}

	if dep.ACLDatacenter != nil {
		if d.Config.PrimaryDatacenter == nil {
			d.Config.PrimaryDatacenter = dep.ACLDatacenter
		}

		// when the acl_datacenter config is used it implicitly enables acls
		d.Config.ACL.Enabled = pBool(true)
		warns = append(warns, deprecationWarning("acl_datacenter", "primary_datacenter"))
	}

	return d.Config, warns
}

func deprecationWarning(old, new string) string {
	return fmt.Sprintf("The '%v' field is deprecated. Use the '%v' field instead.", old, new)
}

func pBool(v bool) *bool {
	return &v
}
