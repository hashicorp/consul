package config

import (
	"fmt"

	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/types"
)

type DeprecatedConfig struct {
	// DEPRECATED (ACL-Legacy-Compat) - moved into the "acl.tokens" stanza
	ACLAgentMasterToken *string `mapstructure:"acl_agent_master_token"`
	// DEPRECATED (ACL-Legacy-Compat) - moved into the "acl.tokens" stanza
	ACLAgentToken *string `mapstructure:"acl_agent_token"`
	// DEPRECATED (ACL-Legacy-Compat) - moved into the "acl.tokens" stanza
	ACLToken *string `mapstructure:"acl_token"`
	// DEPRECATED (ACL-Legacy-Compat) - moved to "acl.enable_key_list_policy"
	ACLEnableKeyListPolicy *bool `mapstructure:"acl_enable_key_list_policy"`

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
	// DEPRECATED (ACL-Legacy-Compat) - moved to "acl.token_ttl"
	ACLTTL *string `mapstructure:"acl_ttl"`

	// DEPRECATED(TLS) - moved to "tls.defaults.ca_file"
	CAFile *string `mapstructure:"ca_file"`

	// DEPRECATED(TLS) - moved to "tls.defaults.ca_path"
	CAPath *string `mapstructure:"ca_path"`

	// DEPRECATED(TLS) - moved to "tls.defaults.cert_file"
	CertFile *string `mapstructure:"cert_file"`

	// DEPRECATED(TLS) - moved to "tls.defaults.key_file"
	KeyFile *string `mapstructure:"key_file"`

	// DEPRECATED(TLS) - moved to "tls.defaults.tls_cipher_suites"
	TLSCipherSuites *string `mapstructure:"tls_cipher_suites"`

	// DEPRECATED(TLS) - moved to "tls.defaults.tls_min_version"
	TLSMinVersion *string `mapstructure:"tls_min_version"`

	// DEPRECATED(TLS) - moved to "tls.defaults.verify_incoming"
	VerifyIncoming *bool `mapstructure:"verify_incoming"`

	// DEPRECATED(TLS) - moved to "tls.https.verify_incoming"
	VerifyIncomingHTTPS *bool `mapstructure:"verify_incoming_https"`

	// DEPRECATED(TLS) - moved to "tls.internal_rpc.verify_incoming"
	VerifyIncomingRPC *bool `mapstructure:"verify_incoming_rpc"`

	// DEPRECATED(TLS) - moved to "tls.defaults.verify_outgoing"
	VerifyOutgoing *bool `mapstructure:"verify_outgoing"`

	// DEPRECATED(TLS) - moved to "tls.internal_rpc.verify_server_hostname"
	VerifyServerHostname *bool `mapstructure:"verify_server_hostname"`

	// DEPRECATED(TLS) - this isn't honored by crypto/tls anymore.
	TLSPreferServerCipherSuites *bool `mapstructure:"tls_prefer_server_cipher_suites"`

	// DEPRECATED(JOIN) - replaced by retry_join
	StartJoinAddrsLAN []string `mapstructure:"start_join"`

	// DEPRECATED(JOIN) - replaced by retry_join_wan
	StartJoinAddrsWAN []string `mapstructure:"start_join_wan"`

	// DEPRECATED see RaftLogStore
	RaftBoltDBConfig *consul.RaftBoltDBConfig `mapstructure:"raft_boltdb" json:"-"`
}

func applyDeprecatedConfig(d *decodeTarget) (Config, []string) {
	dep := d.DeprecatedConfig
	var warns []string

	// TODO(boxofrad): The DeprecatedConfig struct only holds fields that were once
	// on the top-level Config struct (not nested fields e.g. ACL.Tokens) maybe we
	// should rethink this a bit?
	if d.Config.ACL.Tokens.AgentMaster != nil {
		if d.Config.ACL.Tokens.AgentRecovery == nil {
			d.Config.ACL.Tokens.AgentRecovery = d.Config.ACL.Tokens.AgentMaster
		}
		warns = append(warns, deprecationWarning("acl.tokens.agent_master", "acl.tokens.agent_recovery"))
	}

	if dep.ACLAgentMasterToken != nil {
		if d.Config.ACL.Tokens.AgentRecovery == nil {
			d.Config.ACL.Tokens.AgentRecovery = dep.ACLAgentMasterToken
		}
		warns = append(warns, deprecationWarning("acl_agent_master_token", "acl.tokens.agent_recovery"))
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

	if d.Config.ACL.Tokens.Master != nil {
		if d.Config.ACL.Tokens.InitialManagement == nil {
			d.Config.ACL.Tokens.InitialManagement = d.Config.ACL.Tokens.Master
		}
		warns = append(warns, deprecationWarning("acl.tokens.master", "acl.tokens.initial_management"))
	}

	if dep.ACLMasterToken != nil {
		if d.Config.ACL.Tokens.InitialManagement == nil {
			d.Config.ACL.Tokens.InitialManagement = dep.ACLMasterToken
		}
		warns = append(warns, deprecationWarning("acl_master_token", "acl.tokens.initial_management"))
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

	if dep.ACLTTL != nil {
		if d.Config.ACL.TokenTTL == nil {
			d.Config.ACL.TokenTTL = dep.ACLTTL
		}
		warns = append(warns, deprecationWarning("acl_ttl", "acl.token_ttl"))
	}

	if dep.ACLEnableKeyListPolicy != nil {
		if d.Config.ACL.EnableKeyListPolicy == nil {
			d.Config.ACL.EnableKeyListPolicy = dep.ACLEnableKeyListPolicy
		}
		warns = append(warns, deprecationWarning("acl_enable_key_list_policy", "acl.enable_key_list_policy"))
	}

	if len(dep.StartJoinAddrsLAN) > 0 {
		d.Config.RetryJoinLAN = append(d.Config.RetryJoinLAN, dep.StartJoinAddrsLAN...)
		warns = append(warns, deprecationWarning("start_join", "retry_join"))
	}

	if len(dep.StartJoinAddrsWAN) > 0 {
		d.Config.RetryJoinWAN = append(d.Config.RetryJoinWAN, dep.StartJoinAddrsWAN...)
		warns = append(warns, deprecationWarning("start_join_wan", "retry_join_wan"))
	}

	if dep.RaftBoltDBConfig != nil {
		if d.Config.RaftLogStore.BoltDBConfig.NoFreelistSync == nil {
			d.Config.RaftLogStore.BoltDBConfig.NoFreelistSync = &dep.RaftBoltDBConfig.NoFreelistSync
		}
		warns = append(warns, deprecationWarning("raft_boltdb", "raft_logstore.boltdb"))
	}

	warns = append(warns, applyDeprecatedTLSConfig(dep, &d.Config)...)

	return d.Config, warns
}

func applyDeprecatedTLSConfig(dep DeprecatedConfig, cfg *Config) []string {
	var warns []string

	tls := &cfg.TLS
	defaults := &tls.Defaults
	internalRPC := &tls.InternalRPC
	https := &tls.HTTPS
	grpc := &tls.GRPC

	if v := dep.CAFile; v != nil {
		if defaults.CAFile == nil {
			defaults.CAFile = v
		}
		warns = append(warns, deprecationWarning("ca_file", "tls.defaults.ca_file"))
	}

	if v := dep.CAPath; v != nil {
		if defaults.CAPath == nil {
			defaults.CAPath = v
		}
		warns = append(warns, deprecationWarning("ca_path", "tls.defaults.ca_path"))
	}

	if v := dep.CertFile; v != nil {
		if defaults.CertFile == nil {
			defaults.CertFile = v
		}
		warns = append(warns, deprecationWarning("cert_file", "tls.defaults.cert_file"))
	}

	if v := dep.KeyFile; v != nil {
		if defaults.KeyFile == nil {
			defaults.KeyFile = v
		}
		warns = append(warns, deprecationWarning("key_file", "tls.defaults.key_file"))
	}

	if v := dep.TLSCipherSuites; v != nil {
		if defaults.TLSCipherSuites == nil {
			defaults.TLSCipherSuites = v
		}
		warns = append(warns, deprecationWarning("tls_cipher_suites", "tls.defaults.tls_cipher_suites"))
	}

	if v := dep.TLSMinVersion; v != nil {
		if defaults.TLSMinVersion == nil {
			// NOTE: This inner check for deprecated values should eventually be
			// removed
			if version, ok := types.DeprecatedConsulAgentTLSVersions[*v]; ok {
				// Log warning about deprecated config values
				warns = append(warns, fmt.Sprintf("'tls_min_version' value '%s' is deprecated, please specify '%s' instead", *v, version))
				versionString := version.String()
				defaults.TLSMinVersion = &versionString
			} else {
				defaults.TLSMinVersion = v
			}
		}
		warns = append(warns, deprecationWarning("tls_min_version", "tls.defaults.tls_min_version"))
	}

	if v := dep.VerifyIncoming; v != nil {
		if defaults.VerifyIncoming == nil {
			defaults.VerifyIncoming = v
		}

		// Prior to Consul 1.12 it was not possible to enable client certificate
		// verification on the gRPC port. We must override GRPC.VerifyIncoming to
		// prevent it from inheriting Defaults.VerifyIncoming when we've mapped the
		// deprecated top-level verify_incoming field.
		if grpc.VerifyIncoming == nil {
			grpc.VerifyIncoming = pBool(false)
			tls.GRPCModifiedByDeprecatedConfig = &struct{}{}
		}

		warns = append(warns, deprecationWarning("verify_incoming", "tls.defaults.verify_incoming"))
	}

	if v := dep.VerifyIncomingHTTPS; v != nil {
		if https.VerifyIncoming == nil {
			https.VerifyIncoming = v
		}
		warns = append(warns, deprecationWarning("verify_incoming_https", "tls.https.verify_incoming"))
	}

	if v := dep.VerifyIncomingRPC; v != nil {
		if internalRPC.VerifyIncoming == nil {
			internalRPC.VerifyIncoming = v
		}
		warns = append(warns, deprecationWarning("verify_incoming_rpc", "tls.internal_rpc.verify_incoming"))
	}

	if v := dep.VerifyOutgoing; v != nil {
		if defaults.VerifyOutgoing == nil {
			defaults.VerifyOutgoing = v
		}
		warns = append(warns, deprecationWarning("verify_outgoing", "tls.defaults.verify_outgoing"))
	}

	if v := dep.VerifyServerHostname; v != nil {
		if internalRPC.VerifyServerHostname == nil {
			internalRPC.VerifyServerHostname = v
		}
		warns = append(warns, deprecationWarning("verify_server_hostname", "tls.internal_rpc.verify_server_hostname"))
	}

	if dep.TLSPreferServerCipherSuites != nil {
		warns = append(warns, "The 'tls_prefer_server_cipher_suites' field is deprecated and will be ignored.")
	}

	return warns
}

func deprecationWarning(old, new string) string {
	return fmt.Sprintf("The '%v' field is deprecated. Use the '%v' field instead.", old, new)
}

func pBool(v bool) *bool {
	return &v
}
