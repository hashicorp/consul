// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package ops

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

func listGates(t retry.TestingTB, n node) map[string]api.FeatureGate {
	gates, _, err := n.Client().Operator().FeatureGateList(nil)
	require.NoError(t, err)
	out := map[string]api.FeatureGate{}
	for _, g := range gates {
		out[g.Name] = g
	}
	return out
}

// TestFeatureGates verifies the cluster-wide feature gates: the registry of
// gates and its defaults, switching a gate on and off by operator decision
// through the API and the CLI, check-and-set protection, and that agents other
// than the servers see the decision.
func TestFeatureGates(t *testing.T) {
	e := newEnv(t, "", "")
	srv, agent := e.server, e.client

	gates := listGates(t, srv)
	require.NotEmpty(t, gates, "the registry lists the gates this version knows")

	var name string
	for n := range gates {
		name = n
		break
	}
	initial := gates[name]

	t.Run("gates start at their registry defaults", func(t *testing.T) {
		for n, g := range gates {
			require.Equal(t, n, g.Name)
			require.NotEmpty(t, g.Stage, n)
			require.NotEmpty(t, g.MinVersion, n)
			require.NotEmpty(t, g.Description, n)
			require.Equal(t, "registry-default", g.Source, n)
			require.Equal(t, g.DefaultEnabled, g.DesiredEnabled, n)
			require.Equal(t, g.DesiredEnabled, g.EffectiveEnabled, n)
			require.True(t, g.Eligible, n)
			require.NotZero(t, g.PolicyIndex, n)
		}

		got, _, err := srv.Client().Operator().FeatureGateGet(name, nil)
		require.NoError(t, err)
		require.Equal(t, initial, *got)
	})

	flipped := !initial.DefaultEnabled

	t.Run("an operator can switch a gate", func(t *testing.T) {
		res, err := srv.Client().Operator().FeatureGateSet(name, flipped, 0, nil)
		require.NoError(t, err)
		require.True(t, res.Applied)
		require.Equal(t, flipped, res.Feature.DesiredEnabled)
		require.Equal(t, flipped, res.Feature.EffectiveEnabled)
		require.Equal(t, "operator", res.Feature.Source)
		require.Greater(t, res.Feature.PolicyIndex, initial.PolicyIndex)

		got, _, err := srv.Client().Operator().FeatureGateGet(name, nil)
		require.NoError(t, err)
		require.Equal(t, res.Feature, *got)
		require.Equal(t, flipped, listGates(t, srv)[name].EffectiveEnabled)

		for other, g := range listGates(t, srv) {
			if other != name {
				require.Equal(t, gates[other].EffectiveEnabled, g.EffectiveEnabled, "other gates are untouched: %s", other)
			}
		}
	})

	t.Run("a stale check-and-set index does not change the gate", func(t *testing.T) {
		current := listGates(t, srv)[name]

		res, err := srv.Client().Operator().FeatureGateSet(name, initial.DefaultEnabled, current.PolicyIndex-1, nil)
		require.NoError(t, err)
		require.False(t, res.Applied)
		require.Equal(t, flipped, listGates(t, srv)[name].EffectiveEnabled)

		res, err = srv.Client().Operator().FeatureGateSet(name, initial.DefaultEnabled, current.PolicyIndex, nil)
		require.NoError(t, err)
		require.True(t, res.Applied)
		require.Equal(t, initial.DefaultEnabled, listGates(t, srv)[name].EffectiveEnabled)
	})

	t.Run("the CLI lists, reads and switches gates", func(t *testing.T) {
		out, code := srv.CLI(t, "operator", "feature", "list")
		require.Equal(t, 0, code, out)
		for n := range gates {
			require.Contains(t, out, n)
		}

		out, code = srv.CLI(t, "operator", "feature", "set", name, boolWord(flipped))
		require.Equal(t, 0, code, out)
		require.Equal(t, flipped, listGates(t, srv)[name].EffectiveEnabled)

		out, code = srv.CLI(t, "operator", "feature", "get", name)
		require.Equal(t, 0, code, out)
		require.Contains(t, out, name)
		require.Contains(t, out, "operator")

		out, code = srv.CLI(t, "operator", "feature", "set", name, boolWord(initial.DefaultEnabled))
		require.Equal(t, 0, code, out)
		require.Equal(t, initial.DefaultEnabled, listGates(t, srv)[name].EffectiveEnabled)
	})

	t.Run("a client agent sees changes made on the servers", func(t *testing.T) {
		_, err := srv.Client().Operator().FeatureGateSet(name, flipped, 0, nil)
		require.NoError(t, err)

		eventually(t, func(r *retry.R) {
			require.Equal(r, flipped, listGates(r, agent)[name].EffectiveEnabled)
		})
		got, _, err := agent.Client().Operator().FeatureGateGet(name, nil)
		require.NoError(t, err)
		require.Equal(t, flipped, got.EffectiveEnabled)
	})

	t.Run("an unknown gate is refused", func(t *testing.T) {
		_, _, err := srv.Client().Operator().FeatureGateGet("no-such-gate", nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "404")
		require.Contains(t, err.Error(), "unknown feature gate")

		_, err = srv.Client().Operator().FeatureGateSet("no-such-gate", true, 0, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown feature gate")

		out, code := srv.CLI(t, "operator", "feature", "set", "no-such-gate", "enabled")
		require.NotEqual(t, 0, code)
		require.Contains(t, out, "unknown feature gate")

		out, code = srv.CLI(t, "operator", "feature", "set", name, "maybe")
		require.NotEqual(t, 0, code)
		require.Contains(t, out, "Invalid enabled value")
	})
}

func boolWord(b bool) string {
	if b {
		return "enabled"
	}
	return "disabled"
}

// bootstrapGate is a gate that is off by default. The tests below name it in
// the agent configuration, which must only name gates the registry knows.
const bootstrapGate = "localized-dns"

// TestFeatureGates_Bootstrap verifies feature_gates.bootstrap: it sets the
// initial state of a gate when the datacenter first starts, and an operator
// decision made later takes over from it.
func TestFeatureGates_Bootstrap(t *testing.T) {
	e := newEnv(t, `feature_gates { bootstrap = { "`+bootstrapGate+`" = true } }`, "")
	srv := e.server

	gate := listGates(t, srv)[bootstrapGate]
	require.Equal(t, bootstrapGate, gate.Name)
	require.True(t, gate.EffectiveEnabled)
	require.Equal(t, "bootstrap", gate.Source)
	require.True(t, strings.HasPrefix(gate.Reason, "bootstrap"), gate.Reason)

	res, err := srv.Client().Operator().FeatureGateSet(bootstrapGate, false, 0, nil)
	require.NoError(t, err)
	require.True(t, res.Applied)
	require.False(t, res.Feature.EffectiveEnabled)
	require.Equal(t, "operator", res.Feature.Source)
}
