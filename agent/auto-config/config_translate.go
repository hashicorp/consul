package autoconf

import (
	pbconfig "github.com/hashicorp/consul/agent/agentpb/config"
	"github.com/hashicorp/consul/agent/config"
)

// translateAgentConfig is meant to take in a agent/agentpb/config.Config type
// and craft the corresponding agent/config.Config type. The need for this function
// should eventually be removed with the protobuf and normal version converging.
// In the meantime, its not desirable to have the flatter Config struct in protobufs
// as in the long term we want a configuration with more nested groupings.
//
// Why is this function not in the agent/agentpb/config package? The answer, that
// package cannot import the agent/config package without running into import cycles.
func translateConfig(c *pbconfig.Config) *config.Config {
	out := config.Config{
		Datacenter:        &c.Datacenter,
		PrimaryDatacenter: &c.PrimaryDatacenter,
		NodeName:          &c.NodeName,
		SegmentName:       &c.SegmentName,
	}

	// Translate Auto Encrypt settings
	if a := c.AutoEncrypt; a != nil {
		out.AutoEncrypt = config.AutoEncrypt{
			TLS:      &a.TLS,
			DNSSAN:   a.DNSSAN,
			IPSAN:    a.IPSAN,
			AllowTLS: &a.AllowTLS,
		}
	}

	// Translate all the ACL settings
	if a := c.ACL; a != nil {
		out.ACL = config.ACL{
			Enabled:                &a.Enabled,
			PolicyTTL:              &a.PolicyTTL,
			RoleTTL:                &a.RoleTTL,
			TokenTTL:               &a.TokenTTL,
			DownPolicy:             &a.DownPolicy,
			DefaultPolicy:          &a.DefaultPolicy,
			EnableKeyListPolicy:    &a.EnableKeyListPolicy,
			DisabledTTL:            &a.DisabledTTL,
			EnableTokenPersistence: &a.EnableTokenPersistence,
			MSPDisableBootstrap:    &a.MSPDisableBootstrap,
		}

		if t := c.ACL.Tokens; t != nil {
			var tokens []config.ServiceProviderToken

			// create the slice of msp tokens if any
			for _, mspToken := range t.ManagedServiceProvider {
				tokens = append(tokens, config.ServiceProviderToken{
					AccessorID: &mspToken.AccessorID,
					SecretID:   &mspToken.SecretID,
				})
			}

			out.ACL.Tokens = config.Tokens{
				Master:                 &t.Master,
				Replication:            &t.Replication,
				AgentMaster:            &t.AgentMaster,
				Default:                &t.Default,
				Agent:                  &t.Agent,
				ManagedServiceProvider: tokens,
			}
		}
	}

	// Translate the Gossip settings
	if g := c.Gossip; g != nil {
		out.RetryJoinLAN = g.RetryJoinLAN

		// Translate the Gossip Encryption settings
		if e := c.Gossip.Encryption; e != nil {
			out.EncryptKey = &e.Key
			out.EncryptVerifyIncoming = &e.VerifyIncoming
			out.EncryptVerifyOutgoing = &e.VerifyOutgoing
		}
	}

	// Translate the Generic TLS settings
	if t := c.TLS; t != nil {
		out.VerifyOutgoing = &t.VerifyOutgoing
		out.VerifyServerHostname = &t.VerifyServerHostname
		out.TLSMinVersion = &t.MinVersion
		out.TLSCipherSuites = &t.CipherSuites
		out.TLSPreferServerCipherSuites = &t.PreferServerCipherSuites
	}

	return &out
}
