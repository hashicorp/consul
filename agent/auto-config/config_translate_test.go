// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package autoconf

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/structs"
	pbconfig "github.com/hashicorp/consul/proto/private/pbconfig"
	"github.com/hashicorp/consul/proto/private/pbconnect"
)

func stringPointer(s string) *string {
	return &s
}

func boolPointer(b bool) *bool {
	return &b
}

func mustTranslateCARootToProtobuf(t *testing.T, in *structs.CARoot) *pbconnect.CARoot {
	out, err := pbconnect.NewCARootFromStructs(in)
	require.NoError(t, err)
	return out
}

func mustTranslateCARootsToStructs(t *testing.T, in *pbconnect.CARoots) *structs.IndexedCARoots {
	out, err := pbconnect.CARootsToStructs(in)
	require.NoError(t, err)
	return out
}

func mustTranslateCARootsToProtobuf(t *testing.T, in *structs.IndexedCARoots) *pbconnect.CARoots {
	out, err := pbconnect.NewCARootsFromStructs(in)
	require.NoError(t, err)
	return out
}

func mustTranslateIssuedCertToProtobuf(t *testing.T, in *structs.IssuedCert) *pbconnect.IssuedCert {
	var out, err = pbconnect.NewIssuedCertFromStructs(in)
	require.NoError(t, err)
	return out
}

func TestTranslateConfig(t *testing.T) {
	original := pbconfig.Config{
		Datacenter:        "abc",
		PrimaryDatacenter: "def",
		NodeName:          "ghi",
		SegmentName:       "jkl",
		ACL: &pbconfig.ACL{
			Enabled:                true,
			PolicyTTL:              "1s",
			RoleTTL:                "2s",
			TokenTTL:               "3s",
			DownPolicy:             "deny",
			DefaultPolicy:          "deny",
			EnableKeyListPolicy:    true,
			EnableTokenPersistence: true,
			MSPDisableBootstrap:    false,
			Tokens: &pbconfig.ACLTokens{
				InitialManagement: "99e7e490-6baf-43fc-9010-78b6aa9a6813",
				Replication:       "51308d40-465c-4ac6-a636-7c0747edec89",
				AgentRecovery:     "e012e1ea-78a2-41cc-bc8b-231a44196f39",
				Default:           "8781a3f5-de46-4b45-83e1-c92f4cfd0332",
				Agent:             "ddb8f1b0-8a99-4032-b601-87926bce244e",
				ManagedServiceProvider: []*pbconfig.ACLServiceProviderToken{
					{
						AccessorID: "23f37987-7b9e-4e5b-acae-dbc9bc137bae",
						SecretID:   "e28b820a-438e-4e2b-ad24-fe59e6a4914f",
					},
				},
			},
		},
		AutoEncrypt: &pbconfig.AutoEncrypt{
			TLS:      true,
			DNSSAN:   []string{"dns"},
			IPSAN:    []string{"198.18.0.1"},
			AllowTLS: false,
		},
		Gossip: &pbconfig.Gossip{
			RetryJoinLAN: []string{"10.0.0.1"},
			Encryption: &pbconfig.GossipEncryption{
				Key:            "blarg",
				VerifyOutgoing: true,
				VerifyIncoming: true,
			},
		},
		TLS: &pbconfig.TLS{
			VerifyOutgoing:       true,
			VerifyServerHostname: true,
			CipherSuites:         "stuff",
			MinVersion:           "tls13",
		},
	}

	expected := config.Config{
		Datacenter:            stringPointer("abc"),
		PrimaryDatacenter:     stringPointer("def"),
		NodeName:              stringPointer("ghi"),
		SegmentName:           stringPointer("jkl"),
		RetryJoinLAN:          []string{"10.0.0.1"},
		EncryptKey:            stringPointer("blarg"),
		EncryptVerifyIncoming: boolPointer(true),
		EncryptVerifyOutgoing: boolPointer(true),
		TLS: config.TLS{
			Defaults: config.TLSProtocolConfig{
				VerifyOutgoing:  boolPointer(true),
				TLSCipherSuites: stringPointer("stuff"),
				TLSMinVersion:   stringPointer("TLSv1_3"),
			},
			InternalRPC: config.TLSProtocolConfig{
				VerifyServerHostname: boolPointer(true),
			},
		},
		ACL: config.ACL{
			Enabled:                boolPointer(true),
			PolicyTTL:              stringPointer("1s"),
			RoleTTL:                stringPointer("2s"),
			TokenTTL:               stringPointer("3s"),
			DownPolicy:             stringPointer("deny"),
			DefaultPolicy:          stringPointer("deny"),
			EnableKeyListPolicy:    boolPointer(true),
			EnableTokenPersistence: boolPointer(true),
			Tokens: config.Tokens{
				InitialManagement: stringPointer("99e7e490-6baf-43fc-9010-78b6aa9a6813"),
				AgentRecovery:     stringPointer("e012e1ea-78a2-41cc-bc8b-231a44196f39"),
				Replication:       stringPointer("51308d40-465c-4ac6-a636-7c0747edec89"),
				Default:           stringPointer("8781a3f5-de46-4b45-83e1-c92f4cfd0332"),
				Agent:             stringPointer("ddb8f1b0-8a99-4032-b601-87926bce244e"),
				ManagedServiceProvider: []config.ServiceProviderToken{
					{
						AccessorID: stringPointer("23f37987-7b9e-4e5b-acae-dbc9bc137bae"),
						SecretID:   stringPointer("e28b820a-438e-4e2b-ad24-fe59e6a4914f"),
					},
				},
			},
		},
		AutoEncrypt: config.AutoEncrypt{
			TLS:      boolPointer(true),
			DNSSAN:   []string{"dns"},
			IPSAN:    []string{"198.18.0.1"},
			AllowTLS: boolPointer(false),
		},
	}

	translated := translateConfig(&original)
	require.Equal(t, expected, translated)
}

func TestCArootsTranslation(t *testing.T) {
	_, indexedRoots, _ := testCerts(t, "autoconf", "dc1")
	protoRoots := mustTranslateCARootsToProtobuf(t, indexedRoots)
	require.Equal(t, indexedRoots, mustTranslateCARootsToStructs(t, protoRoots))
}

func caRootRoundtrip(t *testing.T, s *structs.CARoot) *structs.CARoot {
	pbRoot, err := pbconnect.NewCARootFromStructs(s)
	require.NoError(t, err)
	root, err := pbconnect.CARootToStructs(pbRoot)
	require.NoError(t, err)
	return root
}
func caRootsRoundtrip(t *testing.T, s *structs.IndexedCARoots) *structs.IndexedCARoots {
	pbRoot, err := pbconnect.NewCARootsFromStructs(s)
	require.NoError(t, err)
	root, err := pbconnect.CARootsToStructs(pbRoot)
	require.NoError(t, err)
	return root
}

func issuedCertRoundtrip(t *testing.T, s *structs.IssuedCert) *structs.IssuedCert {
	pbCert, err := pbconnect.NewIssuedCertFromStructs(s)
	require.NoError(t, err)
	cert, err := pbconnect.IssuedCertToStructs(pbCert)
	require.NoError(t, err)
	return cert
}
