// Copyright IBM Corp. 2026
// SPDX-License-Identifier: BUSL-1.1

package pbconfigentry

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

// TestTerminatingGatewayProtoRoundTrip ensures every terminating-gateway field,
// including credential injection, survives the proto wire format.
func TestTerminatingGatewayProtoRoundTrip(t *testing.T) {
	src := &structs.TerminatingGatewayConfigEntry{
		Kind: structs.TerminatingGateway,
		Name: "camp-egress",
		CredentialInjection: &structs.GatewayCredentialInjection{
			UDSPath:        "/run/camp-auth/processor.sock",
			MessageTimeout: "250ms",
		},
		Services: []structs.LinkedService{
			{
				Name:           "openai-a",
				CAFile:         "/etc/ssl/certs/ca-certificates.crt",
				SNI:            "api.openai.com",
				EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
				Credential: &structs.GatewayServiceCredential{
					Mode:      "inject",
					BindingID: "openai-a",
				},
			},
			{
				Name:                   "legacy",
				CAFile:                 "ca.pem",
				CertFile:               "cert.pem",
				KeyFile:                "key.pem",
				SNI:                    "legacy.example.com",
				DisableAutoHostRewrite: true,
				EnterpriseMeta:         *acl.DefaultEnterpriseMeta(),
				Credential:             &structs.GatewayServiceCredential{Mode: "none"},
			},
		},
		Meta:           map[string]string{"team": "ai"},
		Hash:           42,
		EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
		RaftIndex:      structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
	}

	pb := ConfigEntryFromStructs(src)
	require.Equal(t, Kind_KindTerminatingGateway, pb.Kind)

	encoded, err := proto.Marshal(pb)
	require.NoError(t, err)

	var decoded ConfigEntry
	require.NoError(t, proto.Unmarshal(encoded, &decoded))

	got, ok := ConfigEntryToStructs(&decoded).(*structs.TerminatingGatewayConfigEntry)
	require.True(t, ok)

	// Like the other config entry kinds, Kind is carried by the proto Kind enum
	// rather than copied onto the decoded struct.
	want := *src
	want.Kind = ""
	require.Equal(t, &want, got)
}
