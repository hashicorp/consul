package autoconf

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/consul/agent/config"
	pbconfig "github.com/hashicorp/consul/proto/pbconfig"
	"github.com/stretchr/testify/require"
)

func stringPointer(s string) *string {
	return &s
}

func boolPointer(b bool) *bool {
	return &b
}

func TestConfig_translateConfig(t *testing.T) {
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
			DisabledTTL:            "4s",
			EnableTokenPersistence: true,
			MSPDisableBootstrap:    false,
			Tokens: &pbconfig.ACLTokens{
				Master:      "99e7e490-6baf-43fc-9010-78b6aa9a6813",
				Replication: "51308d40-465c-4ac6-a636-7c0747edec89",
				AgentMaster: "e012e1ea-78a2-41cc-bc8b-231a44196f39",
				Default:     "8781a3f5-de46-4b45-83e1-c92f4cfd0332",
				Agent:       "ddb8f1b0-8a99-4032-b601-87926bce244e",
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
			VerifyOutgoing:           true,
			VerifyServerHostname:     true,
			CipherSuites:             "stuff",
			MinVersion:               "tls13",
			PreferServerCipherSuites: true,
		},
	}

	expected := &config.Config{
		Datacenter:                  stringPointer("abc"),
		PrimaryDatacenter:           stringPointer("def"),
		NodeName:                    stringPointer("ghi"),
		SegmentName:                 stringPointer("jkl"),
		RetryJoinLAN:                []string{"10.0.0.1"},
		EncryptKey:                  stringPointer("blarg"),
		EncryptVerifyIncoming:       boolPointer(true),
		EncryptVerifyOutgoing:       boolPointer(true),
		VerifyOutgoing:              boolPointer(true),
		VerifyServerHostname:        boolPointer(true),
		TLSCipherSuites:             stringPointer("stuff"),
		TLSMinVersion:               stringPointer("tls13"),
		TLSPreferServerCipherSuites: boolPointer(true),
		ACL: config.ACL{
			Enabled:                boolPointer(true),
			PolicyTTL:              stringPointer("1s"),
			RoleTTL:                stringPointer("2s"),
			TokenTTL:               stringPointer("3s"),
			DownPolicy:             stringPointer("deny"),
			DefaultPolicy:          stringPointer("deny"),
			EnableKeyListPolicy:    boolPointer(true),
			DisabledTTL:            stringPointer("4s"),
			EnableTokenPersistence: boolPointer(true),
			Tokens: config.Tokens{
				Master:      stringPointer("99e7e490-6baf-43fc-9010-78b6aa9a6813"),
				Replication: stringPointer("51308d40-465c-4ac6-a636-7c0747edec89"),
				AgentMaster: stringPointer("e012e1ea-78a2-41cc-bc8b-231a44196f39"),
				Default:     stringPointer("8781a3f5-de46-4b45-83e1-c92f4cfd0332"),
				Agent:       stringPointer("ddb8f1b0-8a99-4032-b601-87926bce244e"),
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
	data, err := json.Marshal(translated)
	require.NoError(t, err)

	actual, _, err := config.Parse(string(data), "json")
	require.NoError(t, err)
	require.Equal(t, expected, &actual)
}
