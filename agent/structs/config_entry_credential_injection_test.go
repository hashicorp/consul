// Copyright IBM Corp. 2026
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestGatewayServiceCredentialClone catches aliasing the credential policy in
// gateway associations copied across proxycfg snapshots.
func TestGatewayServiceCredentialClone(t *testing.T) {
	original := &GatewayService{
		CredentialInjection: &GatewayCredentialInjection{
			UDSPath:        "/run/camp-auth/processor.sock",
			MessageTimeout: "250ms",
		},
		Credential: &GatewayServiceCredential{
			Mode:      "inject",
			BindingID: "openai-a",
		},
	}

	clone := original.Clone()
	clone.CredentialInjection.UDSPath = "/run/camp-auth/next.sock"
	clone.Credential.BindingID = "openai-b"

	require.Equal(t, "/run/camp-auth/processor.sock", original.CredentialInjection.UDSPath)
	require.Equal(t, "openai-a", original.Credential.BindingID)
	require.False(t, original.IsSame(clone))
}

func TestDecodeTerminatingGatewayCredential(t *testing.T) {
	entry, err := DecodeConfigEntry(map[string]interface{}{
		"Kind": TerminatingGateway,
		"Name": "camp-egress",
		"CredentialInjection": map[string]interface{}{
			"UDSPath":        "/run/camp-auth/processor.sock",
			"MessageTimeout": "250ms",
		},
		"Services": []interface{}{map[string]interface{}{
			"Name":   "openai-a",
			"CAFile": "/etc/ssl/certs/ca-certificates.crt",
			"SNI":    "api.openai.com",
			"Credential": map[string]interface{}{
				"Mode":      "inject",
				"BindingID": "openai-a",
			},
		}},
	})
	require.NoError(t, err)

	got, ok := entry.(*TerminatingGatewayConfigEntry)
	require.True(t, ok)
	require.Equal(t, "250ms", got.CredentialInjection.MessageTimeout)
	require.Equal(t, "inject", got.Services[0].Credential.Mode)
	require.Equal(t, "openai-a", got.Services[0].Credential.BindingID)
}

func TestDecodeTerminatingGatewayCredential_SnakeCase(t *testing.T) {
	entry, err := DecodeConfigEntry(map[string]interface{}{
		"kind": TerminatingGateway,
		"name": "camp-egress",
		"credential_injection": map[string]interface{}{
			"uds_path":        "/run/camp-auth/processor.sock",
			"message_timeout": "500ms",
		},
		"services": []interface{}{map[string]interface{}{
			"name":    "openai-a",
			"ca_file": "/etc/ssl/certs/ca-certificates.crt",
			"sni":     "api.openai.com",
			"credential": map[string]interface{}{
				"mode":       "inject",
				"binding_id": "openai-a",
			},
		}},
	})
	require.NoError(t, err)

	got, ok := entry.(*TerminatingGatewayConfigEntry)
	require.True(t, ok)
	require.Equal(t, &GatewayCredentialInjection{
		UDSPath:        "/run/camp-auth/processor.sock",
		MessageTimeout: "500ms",
	}, got.CredentialInjection)
	require.Equal(t, &GatewayServiceCredential{Mode: "inject", BindingID: "openai-a"}, got.Services[0].Credential)
}
