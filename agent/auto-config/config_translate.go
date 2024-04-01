// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package autoconf

import (
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/private/pbautoconf"
	"github.com/hashicorp/consul/proto/private/pbconfig"
	"github.com/hashicorp/consul/proto/private/pbconnect"
	"github.com/hashicorp/consul/types"
)

// translateAgentConfig is meant to take in a proto/pbconfig.Config type
// and craft the corresponding agent/config.Config type. The need for this function
// should eventually be removed with the protobuf and normal version converging.
// In the meantime, its not desirable to have the flatter Config struct in protobufs
// as in the long term we want a configuration with more nested groupings.
//
// Why is this function not in the proto/pbconfig package? The answer, that
// package cannot import the agent/config package without running into import cycles.
func translateConfig(c *pbconfig.Config) config.Config {
	result := config.Config{
		Datacenter:        stringPtrOrNil(c.Datacenter),
		PrimaryDatacenter: stringPtrOrNil(c.PrimaryDatacenter),
		NodeName:          stringPtrOrNil(c.NodeName),
		// only output the SegmentName in the configuration if it's non-empty
		// this will avoid a warning later when parsing the persisted configuration
		SegmentName: stringPtrOrNil(c.SegmentName),
		// only output the Partition in the configuration if it's non-empty
		// this will avoid a warning later when parsing the persisted configuration
		Partition: stringPtrOrNil(c.Partition),
	}

	if a := c.AutoEncrypt; a != nil {
		result.AutoEncrypt = config.AutoEncrypt{
			TLS:      &a.TLS,
			DNSSAN:   a.DNSSAN,
			IPSAN:    a.IPSAN,
			AllowTLS: &a.AllowTLS,
		}
	}

	if a := c.ACL; a != nil {
		result.ACL = config.ACL{
			Enabled:                &a.Enabled,
			PolicyTTL:              stringPtrOrNil(a.PolicyTTL),
			RoleTTL:                stringPtrOrNil(a.RoleTTL),
			TokenTTL:               stringPtrOrNil(a.TokenTTL),
			DownPolicy:             stringPtrOrNil(a.DownPolicy),
			DefaultPolicy:          stringPtrOrNil(a.DefaultPolicy),
			EnableKeyListPolicy:    &a.EnableKeyListPolicy,
			EnableTokenPersistence: &a.EnableTokenPersistence,
		}

		if t := c.ACL.Tokens; t != nil {
			tokens := make([]config.ServiceProviderToken, 0, len(t.ManagedServiceProvider))
			for _, mspToken := range t.ManagedServiceProvider {
				tokens = append(tokens, config.ServiceProviderToken{
					AccessorID: &mspToken.AccessorID,
					SecretID:   &mspToken.SecretID,
				})
			}

			result.ACL.Tokens = config.Tokens{
				InitialManagement:      stringPtrOrNil(t.InitialManagement),
				AgentRecovery:          stringPtrOrNil(t.AgentRecovery),
				Replication:            stringPtrOrNil(t.Replication),
				Default:                stringPtrOrNil(t.Default),
				Agent:                  stringPtrOrNil(t.Agent),
				ManagedServiceProvider: tokens,
			}
		}
	}

	if g := c.Gossip; g != nil {
		result.RetryJoinLAN = g.RetryJoinLAN

		if e := c.Gossip.Encryption; e != nil {
			result.EncryptKey = stringPtrOrNil(e.Key)
			result.EncryptVerifyIncoming = &e.VerifyIncoming
			result.EncryptVerifyOutgoing = &e.VerifyOutgoing
		}
	}

	if t := c.TLS; t != nil {
		result.TLS.Defaults = config.TLSProtocolConfig{
			VerifyOutgoing:  &t.VerifyOutgoing,
			TLSCipherSuites: stringPtrOrNil(t.CipherSuites),
		}

		// NOTE: This inner check for deprecated values should eventually be
		// removed, and possibly replaced with a versioning scheme for autoconfig
		// or a proper integration with the deprecated config handling in
		// agent/config/deprecated.go
		if v, ok := types.DeprecatedConsulAgentTLSVersions[t.MinVersion]; ok {
			result.TLS.Defaults.TLSMinVersion = stringPtrOrNil(v.String())
		} else {
			result.TLS.Defaults.TLSMinVersion = stringPtrOrNil(t.MinVersion)
		}

		result.TLS.InternalRPC.VerifyServerHostname = &t.VerifyServerHostname
	}

	return result
}

func stringPtrOrNil(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func extractSignedResponse(resp *pbautoconf.AutoConfigResponse) (*structs.SignedResponse, error) {
	roots, err := pbconnect.CARootsToStructs(resp.CARoots)
	if err != nil {
		return nil, err
	}

	cert, err := pbconnect.IssuedCertToStructs(resp.Certificate)
	if err != nil {
		return nil, err
	}

	out := &structs.SignedResponse{
		IssuedCert:     *cert,
		ConnectCARoots: *roots,
		ManualCARoots:  resp.ExtraCACertificates,
	}

	if resp.Config != nil && resp.Config.TLS != nil {
		out.VerifyServerHostname = resp.Config.TLS.VerifyServerHostname
	}

	return out, err
}
