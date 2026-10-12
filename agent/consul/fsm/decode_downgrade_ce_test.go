// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package fsm

import (
	"bytes"
	"testing"

	"github.com/hashicorp/consul-net-rpc/go-msgpack/codec"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
)

// TestMakeShadowConfigEntry_AllKindsHandled is a regression guard for the downgrade
// replay path. decodeConfigEntryOperation resolves every config entry in an enterprise
// raft log through MakeShadowConfigEntry, and applyConfigEntryOperation panics on any
// error other than ErrDroppingTenantedReq. So a kind that reaches the factory's default
// arm turns a downgrade replay into a crash rather than a dropped entry.
//
// Every kind CE knows must therefore either decode into a shadow entry or be dropped
// explicitly. Adding a kind to AllConfigEntryKinds without touching the factory fails
// here instead of at a customer's downgrade.
func TestMakeShadowConfigEntry_AllKindsHandled(t *testing.T) {
	for _, kind := range structs.AllConfigEntryKinds {
		t.Run(kind, func(t *testing.T) {
			entry, err := MakeShadowConfigEntry(kind, "test")
			if err != nil {
				require.ErrorIs(t, err, ErrDroppingTenantedReq,
					"kind %q reaches the factory's default arm, which panics during downgrade replay; "+
						"add a shadow entry for it, or drop it with ErrDroppingTenantedReq", kind)
				require.Nil(t, entry)
				return
			}
			require.NotNil(t, entry)
		})
	}
}

// TestMakeShadowConfigEntry_InferenceGatewayIsDropped pins the enterprise-only kinds
// that CE deliberately drops on downgrade replay: CE carries the inference-gateway type
// for API consumers but never stores the entry, so there is nothing to decode into.
func TestMakeShadowConfigEntry_InferenceGatewayIsDropped(t *testing.T) {
	for _, kind := range []string{structs.InferenceGateway, structs.RateLimitIPConfig} {
		t.Run(kind, func(t *testing.T) {
			entry, err := MakeShadowConfigEntry(kind, "gw")
			require.ErrorIs(t, err, ErrDroppingTenantedReq)
			require.Nil(t, entry)
		})
	}
}

// TestFSM_ConfigEntry_StripsTerminatingGatewayCredentialInjection ensures a CE server
// replaying an enterprise raft log keeps the terminating gateway but never stores the
// enterprise-only credential injection configuration.
func TestFSM_ConfigEntry_StripsTerminatingGatewayCredentialInjection(t *testing.T) {
	structs.CEDowngrade = true
	t.Cleanup(func() { structs.CEDowngrade = false })

	fsm, err := New(nil, testutil.Logger(t))
	require.NoError(t, err)

	buf, err := structs.Encode(structs.ConfigEntryRequestType, testTerminatingGatewayCredentialRequest())
	require.NoError(t, err)
	resp := fsm.Apply(makeLog(buf))
	if err, ok := resp.(error); ok {
		t.Fatalf("apply: %v", err)
	}

	_, entry, err := fsm.state.ConfigEntry(nil, structs.TerminatingGateway, "camp-egress", nil)
	require.NoError(t, err)
	requireCredentialInjectionStripped(t, entry)
}

// TestRestoreConfigEntry_CEDowngradeStripsTerminatingGatewayCredentialInjection covers
// the same guarantee for an enterprise snapshot restored into a downgraded CE server.
func TestRestoreConfigEntry_CEDowngradeStripsTerminatingGatewayCredentialInjection(t *testing.T) {
	structs.CEDowngrade = true
	t.Cleanup(func() { structs.CEDowngrade = false })

	var buf bytes.Buffer
	require.NoError(t, codec.NewEncoder(&buf, structs.MsgpackHandle).Encode(testTerminatingGatewayCredentialRequest()))

	store := state.NewStateStore(nil)
	restore := store.Restore()
	require.NoError(t, restoreConfigEntry(nil, restore, codec.NewDecoder(&buf, structs.MsgpackHandle)))
	require.NoError(t, restore.Commit())

	_, entry, err := store.ConfigEntry(nil, structs.TerminatingGateway, "camp-egress", nil)
	require.NoError(t, err)
	requireCredentialInjectionStripped(t, entry)
}

func testTerminatingGatewayCredentialRequest() *structs.ConfigEntryRequest {
	return &structs.ConfigEntryRequest{
		Op: structs.ConfigEntryUpsert,
		Entry: &structs.TerminatingGatewayConfigEntry{
			Kind: structs.TerminatingGateway,
			Name: "camp-egress",
			CredentialInjection: &structs.GatewayCredentialInjection{
				UDSPath:        "/run/camp-auth/processor.sock",
				MessageTimeout: "250ms",
			},
			Services: []structs.LinkedService{
				{
					Name:       "openai-a",
					CAFile:     "/etc/ssl/certs/ca-certificates.crt",
					SNI:        "api.openai.com",
					Credential: &structs.GatewayServiceCredential{Mode: "inject", BindingID: "openai-a"},
				},
				{
					Name:   "plain",
					CAFile: "ca.pem",
				},
			},
		},
	}
}

func requireCredentialInjectionStripped(t *testing.T, entry structs.ConfigEntry) {
	t.Helper()
	got, ok := entry.(*structs.TerminatingGatewayConfigEntry)
	require.True(t, ok)
	require.Nil(t, got.CredentialInjection)
	require.Len(t, got.Services, 2)
	for _, svc := range got.Services {
		require.Nil(t, svc.Credential, "service %q", svc.Name)
	}
	require.Equal(t, "openai-a", got.Services[0].Name)
	require.Equal(t, "api.openai.com", got.Services[0].SNI)
	require.Equal(t, "plain", got.Services[1].Name)
}
