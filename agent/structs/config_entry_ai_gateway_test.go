// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAIGatewayConfigEntry_Normalize(t *testing.T) {
	e := &AIGatewayConfigEntry{Name: "gw"}
	require.NoError(t, e.Normalize())
	require.Equal(t, AIGateway, e.Kind)
	require.Equal(t, AIGatewayFailureModeClosed, e.Processor.FailureMode)
	require.Equal(t, AIGatewayConfigValidationWarn, e.Routing.ConfigValidation)
	require.NotZero(t, e.Hash)
}

func TestAIGatewayConfigEntry_Validate(t *testing.T) {
	base := func() *AIGatewayConfigEntry {
		return &AIGatewayConfigEntry{
			Name:      "gw",
			Processor: AIGatewayProcessor{UDSPath: "/run/consul/ext_proc.sock", FailureMode: AIGatewayFailureModeClosed},
			Routing: AIGatewayRouting{
				ConfigValidation: AIGatewayConfigValidationWarn,
				MatchRules: []AIGatewayMatchRule{
					{When: AIGatewayMatch{Path: "/v1/chat/completions", BodyHas: []string{"tools"}}, Candidates: []string{"openai"}},
				},
			},
		}
	}

	cases := map[string]struct {
		mutate func(*AIGatewayConfigEntry)
		errMsg string
	}{
		"valid":               {mutate: func(e *AIGatewayConfigEntry) {}},
		"missing name":        {mutate: func(e *AIGatewayConfigEntry) { e.Name = "" }, errMsg: "Name is required"},
		"relative uds":        {mutate: func(e *AIGatewayConfigEntry) { e.Processor.UDSPath = "run/x.sock" }, errMsg: "absolute Unix socket path"},
		"bad failure mode":    {mutate: func(e *AIGatewayConfigEntry) { e.Processor.FailureMode = "bogus" }, errMsg: "Processor.FailureMode"},
		"bad config valid":    {mutate: func(e *AIGatewayConfigEntry) { e.Routing.ConfigValidation = "loose" }, errMsg: "Routing.ConfigValidation"},
		"reserved budget set":  {mutate: func(e *AIGatewayConfigEntry) { e.Routing.Budget = map[string]interface{}{"x": 1} }, errMsg: "Routing.Budget is reserved"},
		"bad timeout":         {mutate: func(e *AIGatewayConfigEntry) { e.Routing.Timeout = &AIGatewayTimeout{Request: "soon"} }, errMsg: "not a valid duration"},
		"candidate-less rule": {mutate: func(e *AIGatewayConfigEntry) { e.Routing.MatchRules[0].Candidates = nil }, errMsg: "at least one Candidate"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			e := base()
			c.mutate(e)
			err := e.Validate()
			if c.errMsg == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, c.errMsg)
			}
		})
	}
}

// TestAIGatewayConfigEntry_PolicyRoundTrip verifies the Policy block decodes from
// a written config entry (as `consul config write` parses it) and round-trips
// through JSON unchanged — the path the co-located processor reads it back over
// (`GET /v1/config/ai-gateway/<name>`). Consul stores and returns it verbatim; it
// does not interpret the PII fields.
func TestAIGatewayConfigEntry_PolicyRoundTrip(t *testing.T) {
	raw := map[string]interface{}{
		"Kind": AIGateway,
		"Name": "travel-inference-gateway",
		"Policy": map[string]interface{}{
			"AuditLevel": "full",
			"PII": map[string]interface{}{
				"Scope":               "both",
				"DefaultAction":       "placeholder",
				"StreamHoldbackBytes": 128,
				"Mask":                map[string]interface{}{"Char": "*", "KeepLast": 4},
				"Detectors": []interface{}{
					map[string]interface{}{"Name": "ssn", "Action": "block"},
					map[string]interface{}{"Name": "credit_card", "Action": "mask"},
					map[string]interface{}{"Name": "badge", "Regex": "B-[0-9]+", "Action": "placeholder"},
				},
			},
		},
	}

	decoded, err := DecodeConfigEntry(raw)
	require.NoError(t, err, "unrecognized Policy keys would fail decode")
	entry, ok := decoded.(*AIGatewayConfigEntry)
	require.True(t, ok)

	assertPolicy := func(t *testing.T, e *AIGatewayConfigEntry) {
		t.Helper()
		require.NotNil(t, e.Policy)
		require.Equal(t, "full", e.Policy.AuditLevel)
		require.NotNil(t, e.Policy.PII)
		require.Equal(t, "both", e.Policy.PII.Scope)
		require.Equal(t, "placeholder", e.Policy.PII.DefaultAction)
		require.Equal(t, 128, e.Policy.PII.StreamHoldbackBytes)
		require.NotNil(t, e.Policy.PII.Mask)
		require.Equal(t, "*", e.Policy.PII.Mask.Char)
		require.Equal(t, 4, e.Policy.PII.Mask.KeepLast)
		require.Len(t, e.Policy.PII.Detectors, 3)
		require.Equal(t, AIGatewayPIIDetector{Name: "ssn", Action: "block"}, e.Policy.PII.Detectors[0])
		require.Equal(t, AIGatewayPIIDetector{Name: "credit_card", Action: "mask"}, e.Policy.PII.Detectors[1])
		require.Equal(t, AIGatewayPIIDetector{Name: "badge", Regex: "B-[0-9]+", Action: "placeholder"}, e.Policy.PII.Detectors[2])
	}

	// Decoded from the written entry.
	assertPolicy(t, entry)
	require.NoError(t, entry.Normalize())
	require.NoError(t, entry.Validate())

	// Round-trips through the JSON the HTTP API returns to the processor.
	encoded, err := json.Marshal(entry)
	require.NoError(t, err)
	var back AIGatewayConfigEntry
	require.NoError(t, json.Unmarshal(encoded, &back))
	assertPolicy(t, &back)
}

// TestAIGatewayConfigEntry_NoPolicyOmitted verifies an entry without a Policy
// block marshals it away (omitempty) so existing entries are byte-identical.
func TestAIGatewayConfigEntry_NoPolicyOmitted(t *testing.T) {
	e := &AIGatewayConfigEntry{Name: "gw"}
	require.NoError(t, e.Normalize())
	encoded, err := json.Marshal(e)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "Policy")
}

func TestAIGatewayConfigEntry_ShadowCheck(t *testing.T) {
	// A broad rule (no body constraint) placed before a more specific rule
	// (requires "tools") shadows it.
	e := &AIGatewayConfigEntry{
		Name: "gw",
		Routing: AIGatewayRouting{
			MatchRules: []AIGatewayMatchRule{
				{When: AIGatewayMatch{Path: "/v1/chat/completions"}, Candidates: []string{"a"}},
				{When: AIGatewayMatch{Path: "/v1/chat/completions", BodyHas: []string{"tools"}}, Candidates: []string{"b"}},
			},
		},
	}

	// warn mode: loads despite the shadow.
	e.Routing.ConfigValidation = AIGatewayConfigValidationWarn
	require.NoError(t, e.Validate())

	// strict mode: rejected.
	e.Routing.ConfigValidation = AIGatewayConfigValidationStrict
	require.ErrorContains(t, e.Validate(), "shadowed")

	// Reordering (specific first) removes the shadow even in strict mode.
	e.Routing.MatchRules[0], e.Routing.MatchRules[1] = e.Routing.MatchRules[1], e.Routing.MatchRules[0]
	require.NoError(t, e.Validate())
}
