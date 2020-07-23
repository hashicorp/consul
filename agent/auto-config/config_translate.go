package autoconf

import (
	"github.com/hashicorp/consul/proto/pbconfig"
)

// translateAgentConfig is meant to take in a proto/pbconfig.Config type
// and craft the corresponding agent/config.Config type. The need for this function
// should eventually be removed with the protobuf and normal version converging.
// In the meantime, its not desirable to have the flatter Config struct in protobufs
// as in the long term we want a configuration with more nested groupings.
//
// Why is this function not in the proto/pbconfig package? The answer, that
// package cannot import the agent/config package without running into import cycles.
//
// If this function is meant to output an agent/config.Config then why does it output
// a map[string]interface{}? The answer is that our config and command line option
// parsing is messed up and it would require major changes to fix (we probably should
// do them but not for the auto-config feature). To understand this we need to work
// backwards. What we want to be able to do is persist the config settings from an
// auto-config response configuration to disk. We then want that configuration
// to be able to be parsed with the normal configuration parser/builder. It sort of was
// working with returning a filled out agent/config.Config but the problem was that
// the struct has a lot of non-pointer struct members. Thus, JSON serializtion caused
// these to always be emitted even if they contained no non-empty fields. The
// configuration would then seem to parse okay, but in OSS we would get warnings for
// setting a bunch of enterprise fields like "audit" at the top level. In an attempt
// to quiet those warnings, I had converted all the existing non-pointer struct fields
// to pointers. Then there were issues with the builder code expecting concrete values.
// I could add nil checks **EVERYWHERE** in builder.go or take a different approach.
// I then made a function utilizing github.com/mitchellh/reflectwalk to un-nil all the
// struct pointers after parsing to prevent any nil pointer dereferences. At first
// glance this seemed like it was going to work but then I saw that nearly all of the
// tests in runtime_test.go were failing. The first issues was that we were not merging
// pointers to struct fields properly. It was simply taking the new pointer if non-nil
// and defaulting to the original. So I updated that code, to properly merge pointers
// to structs. That fixed a bunch of tests but then there was another issue with
// the runtime tests where it was emitting warnings for using consul enterprise only
// configuration. After spending some time tracking this down it turns out that it
// was coming from our CLI option parsing. Our CLI option parsing works by filling
// in a agent/config.Config struct. Along the way when converting to pointers to
// structs I had to add a call to that function to un-nil various pointers to prevent
// the CLI from segfaulting. However this un-nil operation was causing the various
// enterprise only keys to be materialized. Thus we were back to where we were before
// the conversion to pointers to structs and mostly stuck.
//
// Therefore, this function will create a map[string]interface{} that should be
// compatible with the agent/config.Config struct but where we can more tightly
// control which fields are output. Its not a nice solution. It has a non-trivial
// maintenance burden. In the long run we should unify the protobuf Config and
// the normal agent/config.Config so that we can just serialize the protobuf version
// without any translation. For now, this hack is necessary :(
func translateConfig(c *pbconfig.Config) map[string]interface{} {
	out := map[string]interface{}{
		"datacenter":         c.Datacenter,
		"primary_datacenter": c.PrimaryDatacenter,
		"node_name":          c.NodeName,
	}

	// only output the SegmentName in the configuration if its non-empty
	// this will avoid a warning later when parsing the persisted configuration
	if c.SegmentName != "" {
		out["segment"] = c.SegmentName
	}

	// Translate Auto Encrypt settings
	if a := c.AutoEncrypt; a != nil {
		autoEncryptConfig := map[string]interface{}{
			"tls":       a.TLS,
			"allow_tls": a.AllowTLS,
		}

		if len(a.DNSSAN) > 0 {
			autoEncryptConfig["dns_san"] = a.DNSSAN
		}
		if len(a.IPSAN) > 0 {
			autoEncryptConfig["ip_san"] = a.IPSAN
		}

		out["auto_encrypt"] = autoEncryptConfig
	}

	// Translate all the ACL settings
	if a := c.ACL; a != nil {
		aclConfig := map[string]interface{}{
			"enabled":                  a.Enabled,
			"policy_ttl":               a.PolicyTTL,
			"role_ttl":                 a.RoleTTL,
			"token_ttl":                a.TokenTTL,
			"down_policy":              a.DownPolicy,
			"default_policy":           a.DefaultPolicy,
			"enable_key_list_policy":   a.EnableKeyListPolicy,
			"disabled_ttl":             a.DisabledTTL,
			"enable_token_persistence": a.EnableTokenPersistence,
		}

		if t := c.ACL.Tokens; t != nil {
			var mspTokens []map[string]string

			// create the slice of msp tokens if any
			for _, mspToken := range t.ManagedServiceProvider {
				mspTokens = append(mspTokens, map[string]string{
					"accessor_id": mspToken.AccessorID,
					"secret_id":   mspToken.SecretID,
				})
			}

			tokenConfig := make(map[string]interface{})

			if t.Master != "" {
				tokenConfig["master"] = t.Master
			}
			if t.Replication != "" {
				tokenConfig["replication"] = t.Replication
			}
			if t.AgentMaster != "" {
				tokenConfig["agent_master"] = t.AgentMaster
			}
			if t.Default != "" {
				tokenConfig["default"] = t.Default
			}
			if t.Agent != "" {
				tokenConfig["agent"] = t.Agent
			}
			if len(mspTokens) > 0 {
				tokenConfig["managed_service_provider"] = mspTokens
			}

			aclConfig["tokens"] = tokenConfig
		}
		out["acl"] = aclConfig
	}

	// Translate the Gossip settings
	if g := c.Gossip; g != nil {
		out["retry_join"] = g.RetryJoinLAN

		// Translate the Gossip Encryption settings
		if e := c.Gossip.Encryption; e != nil {
			out["encrypt"] = e.Key
			out["encrypt_verify_incoming"] = e.VerifyIncoming
			out["encrypt_verify_outgoing"] = e.VerifyOutgoing
		}
	}

	// Translate the Generic TLS settings
	if t := c.TLS; t != nil {
		out["verify_outgoing"] = t.VerifyOutgoing
		out["verify_server_hostname"] = t.VerifyServerHostname
		if t.MinVersion != "" {
			out["tls_min_version"] = t.MinVersion
		}
		if t.CipherSuites != "" {
			out["tls_cipher_suites"] = t.CipherSuites
		}
		out["tls_prefer_server_cipher_suites"] = t.PreferServerCipherSuites
	}

	return out
}
