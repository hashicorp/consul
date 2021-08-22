package autoconf

import (
	"fmt"

	"github.com/mitchellh/mapstructure"

	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto"
	"github.com/hashicorp/consul/proto/pbautoconf"
	"github.com/hashicorp/consul/proto/pbconfig"
	"github.com/hashicorp/consul/proto/pbconnect"
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
		// only output the SegmentName in the configuration if its non-empty
		// this will avoid a warning later when parsing the persisted configuration
		SegmentName: stringPtrOrNil(c.SegmentName),
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
				Master:                 stringPtrOrNil(t.Master),
				Replication:            stringPtrOrNil(t.Replication),
				AgentMaster:            stringPtrOrNil(t.AgentMaster),
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
		result.VerifyOutgoing = &t.VerifyOutgoing
		result.VerifyServerHostname = &t.VerifyServerHostname
		result.TLSMinVersion = stringPtrOrNil(t.MinVersion)
		result.TLSCipherSuites = stringPtrOrNil(t.CipherSuites)
		result.TLSPreferServerCipherSuites = &t.PreferServerCipherSuites
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
	roots, err := translateCARootsToStructs(resp.CARoots)
	if err != nil {
		return nil, err
	}

	cert, err := translateIssuedCertToStructs(resp.Certificate)
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

// translateCARootsToStructs will create a structs.IndexedCARoots object from the corresponding
// protobuf struct. Those structs are intended to be identical so the conversion just uses
// mapstructure to go from one to the other.
func translateCARootsToStructs(in *pbconnect.CARoots) (*structs.IndexedCARoots, error) {
	var out structs.IndexedCARoots
	if err := mapstructureTranslateToStructs(in, &out); err != nil {
		return nil, fmt.Errorf("Failed to re-encode CA Roots: %w", err)
	}

	return &out, nil
}

// translateIssuedCertToStructs will create a structs.IssuedCert object from the corresponding
// protobuf struct. Those structs are intended to be identical so the conversion just uses
// mapstructure to go from one to the other.
func translateIssuedCertToStructs(in *pbconnect.IssuedCert) (*structs.IssuedCert, error) {
	var out structs.IssuedCert
	if err := mapstructureTranslateToStructs(in, &out); err != nil {
		return nil, fmt.Errorf("Failed to re-encode CA Roots: %w", err)
	}

	return &out, nil
}

func mapstructureTranslateToStructs(in interface{}, out interface{}) error {
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook: proto.HookPBTimestampToTime,
		Result:     out,
	})

	if err != nil {
		return err
	}

	return decoder.Decode(in)
}

func translateCARootsToProtobuf(in *structs.IndexedCARoots) (*pbconnect.CARoots, error) {
	var out pbconnect.CARoots
	if err := mapstructureTranslateToProtobuf(in, &out); err != nil {
		return nil, fmt.Errorf("Failed to re-encode CA Roots: %w", err)
	}

	return &out, nil
}

func translateIssuedCertToProtobuf(in *structs.IssuedCert) (*pbconnect.IssuedCert, error) {
	var out pbconnect.IssuedCert
	if err := mapstructureTranslateToProtobuf(in, &out); err != nil {
		return nil, fmt.Errorf("Failed to re-encode CA Roots: %w", err)
	}

	return &out, nil
}

func mapstructureTranslateToProtobuf(in interface{}, out interface{}) error {
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook: proto.HookTimeToPBTimestamp,
		Result:     out,
	})

	if err != nil {
		return err
	}

	return decoder.Decode(in)
}
