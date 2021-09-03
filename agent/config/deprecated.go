package config

import "fmt"

type DeprecatedConfig struct {
	// DEPRECATED (ACL-Legacy-Compat) - moved into the "acl.tokens" stanza
	ACLAgentMasterToken *string `mapstructure:"acl_agent_master_token"`
	// DEPRECATED (ACL-Legacy-Compat) - moved into the "acl.tokens" stanza
	ACLAgentToken *string `mapstructure:"acl_agent_token"`
	// DEPRECATED (ACL-Legacy-Compat) - moved into the "acl.tokens" stanza
	ACLToken *string `mapstructure:"acl_token"`

	// DEPRECATED (ACL-Legacy-Compat) - moved into the "acl" stanza
	ACLMasterToken *string `mapstructure:"acl_master_token"`
	// DEPRECATED (ACL-Legacy-Compat) - moved into the "acl.tokens" stanza
	ACLReplicationToken *string `mapstructure:"acl_replication_token"`
	// DEPRECATED (ACL-Legacy-Compat) - moved to "acl.enable_token_replication"
	EnableACLReplication *bool `mapstructure:"enable_acl_replication"`

	// DEPRECATED (ACL-Legacy-Compat) - moved to "primary_datacenter"
	ACLDatacenter *string `mapstructure:"acl_datacenter"`

	// DEPRECATED (ACL-Legacy-Compat) - moved to "acl.default_policy"
	ACLDefaultPolicy *string `mapstructure:"acl_default_policy"`
	// DEPRECATED (ACL-Legacy-Compat) - moved to "acl.down_policy"
	ACLDownPolicy *string `mapstructure:"acl_down_policy"`
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

	if dep.ACLAgentToken != nil {
		if d.Config.ACL.Tokens.Agent == nil {
			d.Config.ACL.Tokens.Agent = dep.ACLAgentToken
		}
		warns = append(warns, deprecationWarning("acl_agent_token", "acl.tokens.agent"))
	}

	if dep.ACLToken != nil {
		if d.Config.ACL.Tokens.Default == nil {
			d.Config.ACL.Tokens.Default = dep.ACLToken
		}
		warns = append(warns, deprecationWarning("acl_token", "acl.tokens.default"))
	}

	if dep.ACLMasterToken != nil {
		if d.Config.ACL.Tokens.Master == nil {
			d.Config.ACL.Tokens.Master = dep.ACLMasterToken
		}
		warns = append(warns, deprecationWarning("acl_master_token", "acl.tokens.master"))
	}

	if dep.ACLReplicationToken != nil {
		if d.Config.ACL.Tokens.Replication == nil {
			d.Config.ACL.Tokens.Replication = dep.ACLReplicationToken
		}
		d.Config.ACL.TokenReplication = pBool(true)
		warns = append(warns, deprecationWarning("acl_replication_token", "acl.tokens.replication"))
	}

	if dep.EnableACLReplication != nil {
		if d.Config.ACL.TokenReplication == nil {
			d.Config.ACL.TokenReplication = dep.EnableACLReplication
		}
		warns = append(warns, deprecationWarning("enable_acl_replication", "acl.enable_token_replication"))
	}

	if dep.ACLDatacenter != nil {
		if d.Config.PrimaryDatacenter == nil {
			d.Config.PrimaryDatacenter = dep.ACLDatacenter
		}

		// when the acl_datacenter config is used it implicitly enables acls
		d.Config.ACL.Enabled = pBool(true)
		warns = append(warns, deprecationWarning("acl_datacenter", "primary_datacenter"))
	}

	if dep.ACLDefaultPolicy != nil {
		if d.Config.ACL.DefaultPolicy == nil {
			d.Config.ACL.DefaultPolicy = dep.ACLDefaultPolicy
		}
		warns = append(warns, deprecationWarning("acl_default_policy", "acl.default_policy"))
	}

	if dep.ACLDownPolicy != nil {
		if d.Config.ACL.DownPolicy == nil {
			d.Config.ACL.DownPolicy = dep.ACLDownPolicy
		}
		warns = append(warns, deprecationWarning("acl_down_policy", "acl.down_policy"))
	}

	return d.Config, warns
}

func deprecationWarning(old, new string) string {
	return fmt.Sprintf("The '%v' field is deprecated. Use the '%v' field instead.", old, new)
}

func pBool(v bool) *bool {
	return &v
}
