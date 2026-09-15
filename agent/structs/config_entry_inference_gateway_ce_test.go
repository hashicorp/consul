// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package structs

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
)

const inferenceGatewayEnterpriseErr = "inference-gateway is a consul enterprise feature"

// TestInferenceGatewayConfigEntry_CE_Validate pins the CE gate: the entry type exists
// so API consumers compile against it, but a CE server refuses to store it.
func TestInferenceGatewayConfigEntry_CE_Validate(t *testing.T) {
	e := &InferenceGatewayConfigEntry{
		Name:      "gw",
		Processor: InferenceGatewayProcessor{UDSPath: "/run/consul/ext_proc.sock"},
	}
	require.NoError(t, e.Normalize())
	require.EqualError(t, e.Validate(), inferenceGatewayEnterpriseErr)
}

// TestInferenceGatewayConfigEntry_CE_Normalize covers the part of the entry CE still
// runs before rejecting it: MakeConfigEntry, the kind, the failure-mode default and
// the hash.
func TestInferenceGatewayConfigEntry_CE_Normalize(t *testing.T) {
	entry, err := MakeConfigEntry(InferenceGateway, "gw")
	require.NoError(t, err)
	e, ok := entry.(*InferenceGatewayConfigEntry)
	require.True(t, ok)

	e.Processor.FailureMode = "OPEN"
	require.NoError(t, e.Normalize())
	require.Equal(t, InferenceGateway, e.GetKind())
	require.Equal(t, InferenceGateway, e.Kind)
	require.Equal(t, InferenceGatewayFailureModeOpen, e.Processor.FailureMode)
	require.NotZero(t, e.GetHash())

	unset := &InferenceGatewayConfigEntry{Name: "gw"}
	require.NoError(t, unset.Normalize())
	require.Equal(t, InferenceGatewayFailureModeClosed, unset.Processor.FailureMode)
}

// TestInferenceGatewayConfigEntry_CE_DecodeAndJSONRoundTrip verifies every field class
// decodes and survives the JSON the HTTP API returns, so CE and ENT expose the same
// shape to API consumers.
func TestInferenceGatewayConfigEntry_CE_DecodeAndJSONRoundTrip(t *testing.T) {
	raw := map[string]interface{}{
		"Kind": InferenceGateway,
		"Name": "gw",
		"Meta": map[string]interface{}{"owner": "platform"},
		"Processor": map[string]interface{}{
			"UDSPath":          "/run/consul/ext_proc.sock",
			"FailureMode":      "open",
			"BodyModelRouting": true,
		},
		"Failover": map[string]interface{}{
			"RetryOn":       []interface{}{"401", "5xx"},
			"MaxTiers":      2,
			"PerTryTimeout": "30s",
		},
		"AuditLevel": "full",
		"PII": map[string]interface{}{
			"Scope":         "both",
			"DefaultAction": "mask",
			"Mask":          map[string]interface{}{"Char": "#", "KeepLast": 4},
			"Detectors":     []interface{}{map[string]interface{}{"Name": "ssn", "Action": "block"}},
		},
		"Observability": map[string]interface{}{
			"Metrics": map[string]interface{}{
				"Enabled":      true,
				"CustomLabels": []interface{}{"team"},
			},
			"Tracing": map[string]interface{}{
				"Enabled":     true,
				"SampleRatio": 0.05,
				"OTLP":        map[string]interface{}{"Endpoint": "collector:4317"},
			},
		},
	}

	decoded, err := DecodeConfigEntry(raw)
	require.NoError(t, err)
	e, ok := decoded.(*InferenceGatewayConfigEntry)
	require.True(t, ok)
	require.NoError(t, e.Normalize())

	require.Equal(t, "/run/consul/ext_proc.sock", e.Processor.UDSPath)
	require.True(t, e.Processor.BodyModelRouting)
	require.Equal(t, &InferenceGatewayFailover{RetryOn: []string{"401", "5xx"}, MaxTiers: 2, PerTryTimeout: "30s"}, e.Failover)
	require.Equal(t, "full", e.AuditLevelOrLegacy())
	require.Equal(t, &InferenceGatewayPIIMask{Char: "#", KeepLast: 4}, e.PII.Mask)
	require.Equal(t, []InferenceGatewayPIIDetector{{Name: "ssn", Action: "block"}}, e.PII.Detectors)
	require.NotNil(t, e.Observability.Metrics.Enabled)
	require.True(t, *e.Observability.Metrics.Enabled)
	require.Equal(t, "collector:4317", e.Observability.Tracing.OTLP.Endpoint)

	encoded, err := json.Marshal(e)
	require.NoError(t, err)
	var back InferenceGatewayConfigEntry
	require.NoError(t, json.Unmarshal(encoded, &back))
	require.Equal(t, e, &back)
}

// TestInferenceGatewayConfigEntry_CE_ACLs keeps the entry's ACL posture pinned in CE:
// reads need service:read on the entry name, writes need mesh:write, the same as
// api-gateway. The shared TestConfigEntries_ACLs table validates every entry before
// checking ACLs, which the CE gate rejects, so the same checks run here directly.
func TestInferenceGatewayConfigEntry_CE_ACLs(t *testing.T) {
	newAuthz := func(t *testing.T, src string) acl.Authorizer {
		policy, err := acl.NewPolicyFromSource(src, nil, nil)
		require.NoError(t, err)
		authorizer, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		require.NoError(t, err)
		return authorizer
	}

	entry := &InferenceGatewayConfigEntry{Name: "travel-inference-gateway"}
	require.NoError(t, entry.Normalize())

	cases := []struct {
		name     string
		policy   string
		canRead  bool
		canWrite bool
	}{
		{name: "no-authz", policy: ``},
		{name: "service read", policy: `service "travel-inference-gateway" { policy = "read" }`, canRead: true},
		{name: "read on a different service", policy: `service "other-gateway" { policy = "read" }`},
		{name: "service write is not mesh write", policy: `service "travel-inference-gateway" { policy = "write" }`, canRead: true},
		{name: "mesh write", policy: `mesh = "write"`, canWrite: true},
		{name: "service read and mesh write", policy: `service "travel-inference-gateway" { policy = "read" } mesh = "write"`, canRead: true, canWrite: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			authz := newAuthz(t, tc.policy)
			requireACL := func(t *testing.T, err error, allowed bool) {
				t.Helper()
				if allowed {
					require.NoError(t, err)
					return
				}
				require.Error(t, err)
				require.True(t, acl.IsErrPermissionDenied(err))
			}
			requireACL(t, entry.CanRead(authz), tc.canRead)
			requireACL(t, entry.CanWrite(authz), tc.canWrite)
		})
	}
}

// TestNodeService_CE_InferenceGatewayKind pins the registration-side gate: the kind is
// known to CE (so an agent config file does not degrade it to a typical service), but
// registering it is rejected.
func TestNodeService_CE_InferenceGatewayKind(t *testing.T) {
	ns := &NodeService{
		Kind:    ServiceKindInferenceGateway,
		Service: "inference-gateway",
		Address: "10.0.0.1",
		Port:    8443,
	}
	require.ErrorContains(t, ns.Validate(), inferenceGatewayEnterpriseErr)
	require.True(t, ns.Kind.IsProxy())
	require.True(t, ns.IsGateway())
}
