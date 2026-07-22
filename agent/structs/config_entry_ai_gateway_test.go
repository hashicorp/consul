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

// TestAIGatewayConfigEntry_RateLimitRoundTrip verifies the RateLimit + StateStore
// blocks decode from a written config entry and round-trip through JSON unchanged —
// the same verbatim-storage path Policy uses (no proto/mog codegen involved), which
// is how the co-located processor reads limits back over the HTTP config-entry API.
func TestAIGatewayConfigEntry_RateLimitRoundTrip(t *testing.T) {
	raw := map[string]interface{}{
		"Kind": AIGateway,
		"Name": "travel-inference-gateway",
		"StateStore": map[string]interface{}{
			"Service":       "valkey",
			"LocalBindPort": 6379,
		},
		"RateLimit": map[string]interface{}{
			"Enabled":                  true,
			"Enforcement":              "deny",
			"CountMode":                "total",
			"DegradeMode":              "fail_closed",
			"Dimensions":  []interface{}{"agent", "tier", "global", "model"},
			"Default": map[string]interface{}{
				"Requests": map[string]interface{}{"Count": 60},
				"Tokens":   map[string]interface{}{"Count": 10000},
			},
			"Global": map[string]interface{}{
				"Requests": map[string]interface{}{"Count": 20000},
				"Tokens":   map[string]interface{}{"Count": 1000000},
			},
			"TierLimits": []interface{}{
				map[string]interface{}{"Tier": "standard", "Requests": map[string]interface{}{"Count": 100}, "Tokens": map[string]interface{}{"Count": 20000, "Unit": "day"}, "MaxCompletionTokensCap": 4096},
			},
			"ModelLimits": []interface{}{
				map[string]interface{}{"Model": "gpt-4o", "Requests": map[string]interface{}{"Count": 30}, "Tokens": map[string]interface{}{"Count": 8000}},
			},
			"TierBindings": []interface{}{
				map[string]interface{}{"Tier": "standard", "Partition": "default"},
			},
		},
	}

	decoded, err := DecodeConfigEntry(raw)
	require.NoError(t, err, "unrecognized RateLimit/StateStore keys would fail decode")
	entry, ok := decoded.(*AIGatewayConfigEntry)
	require.True(t, ok)

	assertRL := func(t *testing.T, e *AIGatewayConfigEntry) {
		t.Helper()
		require.NotNil(t, e.StateStore)
		require.Equal(t, "valkey", e.StateStore.Service)
		require.Equal(t, 6379, e.StateStore.LocalBindPort)
		require.NotNil(t, e.RateLimit)
		require.True(t, e.RateLimit.Enabled)
		require.Equal(t, "deny", e.RateLimit.Enforcement)
		require.Equal(t, []string{"agent", "tier", "global", "model"}, e.RateLimit.Dimensions)
		require.NotNil(t, e.RateLimit.Default)
		require.Equal(t, &AIGatewayLimit{Count: 10000}, e.RateLimit.Default.Tokens)
		require.NotNil(t, e.RateLimit.Global)
		require.Equal(t, &AIGatewayLimit{Count: 1000000}, e.RateLimit.Global.Tokens)
		require.Equal(t, []AIGatewayTierLimit{{Tier: "standard", Requests: &AIGatewayLimit{Count: 100}, Tokens: &AIGatewayLimit{Count: 20000, Unit: "day"}, MaxCompletionTokensCap: 4096}}, e.RateLimit.TierLimits)
		require.Equal(t, []AIGatewayModelLimit{{Model: "gpt-4o", Requests: &AIGatewayLimit{Count: 30}, Tokens: &AIGatewayLimit{Count: 8000}}}, e.RateLimit.ModelLimits)
		require.Equal(t, []AIGatewayTierBinding{{Tier: "standard", Partition: "default"}}, e.RateLimit.TierBindings)
	}

	assertRL(t, entry)
	require.NoError(t, entry.Normalize())
	require.NoError(t, entry.Validate())

	encoded, err := json.Marshal(entry)
	require.NoError(t, err)
	var back AIGatewayConfigEntry
	require.NoError(t, json.Unmarshal(encoded, &back))
	assertRL(t, &back)
}

// TestAIGatewayConfigEntry_ValidateRateLimit exercises the write-time gate on the
// RateLimit policy and its backing StateStore.
func TestAIGatewayConfigEntry_ValidateRateLimit(t *testing.T) {
	base := func() *AIGatewayConfigEntry {
		return &AIGatewayConfigEntry{
			Name:       "gw",
			StateStore: &AIGatewayStateStore{Service: "valkey", LocalBindPort: 6379},
			RateLimit: &AIGatewayRateLimit{
				Enabled:      true,
				Enforcement:  "deny",
				Dimensions:   []string{"tier", "global"},
				TierLimits:   []AIGatewayTierLimit{{Tier: "standard", Requests: &AIGatewayLimit{Count: 100}}},
				TierBindings: []AIGatewayTierBinding{{Tier: "standard", Partition: "default"}},
			},
		}
	}
	cases := map[string]struct {
		mutate func(*AIGatewayConfigEntry)
		errMsg string
	}{
		"valid":            {mutate: func(e *AIGatewayConfigEntry) {}},
		"disabled skips":   {mutate: func(e *AIGatewayConfigEntry) { e.RateLimit.Enabled = false; e.StateStore = nil }},
		"bad enforcement":  {mutate: func(e *AIGatewayConfigEntry) { e.RateLimit.Enforcement = "throttle" }, errMsg: "RateLimit.Enforcement"},
		"strict rejected":  {mutate: func(e *AIGatewayConfigEntry) { e.RateLimit.Mode = "strict" }, errMsg: "not implemented"},
		"bad dimension":    {mutate: func(e *AIGatewayConfigEntry) { e.RateLimit.Dimensions = []string{"provider"} }, errMsg: "unsupported dimension"},
		"dup tier":         {mutate: func(e *AIGatewayConfigEntry) { e.RateLimit.TierLimits = append(e.RateLimit.TierLimits, AIGatewayTierLimit{Tier: "standard"}) }, errMsg: "declared more than once"},
		"binding no tier":  {mutate: func(e *AIGatewayConfigEntry) { e.RateLimit.TierBindings[0].Tier = "gold" }, errMsg: "references a tier with no TierLimit"},
		"bad unit":         {mutate: func(e *AIGatewayConfigEntry) { e.RateLimit.TierLimits[0].Tokens = &AIGatewayLimit{Count: 10, Unit: "week"} }, errMsg: "invalid Unit"},
		"missing store":    {mutate: func(e *AIGatewayConfigEntry) { e.StateStore = nil }, errMsg: "requires a StateStore"},
		"bad bind port":    {mutate: func(e *AIGatewayConfigEntry) { e.StateStore.LocalBindPort = 0 }, errMsg: "LocalBindPort"},
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
