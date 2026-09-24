// Copyright IBM Corp. 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package structs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestTerminatingGatewayCredentialInjection_CE ensures a CE server never stores
// terminating-gateway credential injection configuration, while plain
// terminating gateways keep validating as before.
func TestTerminatingGatewayCredentialInjection_CE(t *testing.T) {
	cases := map[string]struct {
		entry   *TerminatingGatewayConfigEntry
		wantErr string
	}{
		"gateway credential injection rejected": {
			entry: &TerminatingGatewayConfigEntry{
				Kind: TerminatingGateway,
				Name: "camp-egress",
				CredentialInjection: &GatewayCredentialInjection{
					UDSPath:        "/run/camp-auth/processor.sock",
					MessageTimeout: "250ms",
				},
				Services: []LinkedService{{
					Name:       "openai-a",
					CAFile:     "/etc/ssl/certs/ca-certificates.crt",
					SNI:        "api.openai.com",
					Credential: &GatewayServiceCredential{Mode: "inject", BindingID: "openai-a"},
				}},
			},
			wantErr: "credential injection is a consul enterprise feature",
		},
		"gateway credential injection without services rejected": {
			entry: &TerminatingGatewayConfigEntry{
				Kind:                TerminatingGateway,
				Name:                "camp-egress",
				CredentialInjection: &GatewayCredentialInjection{UDSPath: "/run/camp-auth/processor.sock"},
			},
			wantErr: "credential injection is a consul enterprise feature",
		},
		"service credential rejected": {
			entry: &TerminatingGatewayConfigEntry{
				Kind: TerminatingGateway,
				Name: "camp-egress",
				Services: []LinkedService{{
					Name:       "openai-a",
					Credential: &GatewayServiceCredential{Mode: "none"},
				}},
			},
			wantErr: `service "openai-a": credential injection is a consul enterprise feature`,
		},
		"plain terminating gateway accepted": {
			entry: &TerminatingGatewayConfigEntry{
				Kind: TerminatingGateway,
				Name: "egress",
				Services: []LinkedService{{
					Name:   "openai-a",
					CAFile: "/etc/ssl/certs/ca-certificates.crt",
					SNI:    "api.openai.com",
				}},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, tc.entry.Normalize())
			err := tc.entry.Validate()
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, tc.wantErr)
		})
	}
}
