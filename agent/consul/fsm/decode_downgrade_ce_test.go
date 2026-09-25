// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package fsm

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
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
