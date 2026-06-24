// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
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
