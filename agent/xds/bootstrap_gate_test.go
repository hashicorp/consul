// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
)

// fakeSnapshot stands in for *proxycfg.ConfigSnapshot so these tests can drive
// the gate through states that are awkward to assemble from a real snapshot.
type fakeSnapshot struct{ missing []string }

func (f fakeSnapshot) APIGatewayDiscoveryChainsMissingEndpoints() []string { return f.missing }

var (
	incompleteSnapshot = fakeSnapshot{missing: []string{"web/web.default.default.dc1"}}
	completeSnapshot   = fakeSnapshot{}
)

func TestBootstrapGate(t *testing.T) {
	logger := testutil.Logger(t)

	t.Run("complete snapshot is pushed immediately", func(t *testing.T) {
		g := newBootstrapGate(0)
		require.True(t, g.allow(logger, completeSnapshot))
		require.Nil(t, g.expiryCh(), "a gate that never held should not arm its timer")
	})

	t.Run("incomplete snapshot is held until endpoints arrive", func(t *testing.T) {
		g := newBootstrapGate(time.Minute)

		require.False(t, g.allow(logger, incompleteSnapshot))
		require.NotNil(t, g.expiryCh(), "holding a push must arm the deadline")

		require.True(t, g.allow(logger, completeSnapshot))
		require.Nil(t, g.expiryCh(), "the deadline must be released once the gate opens")
	})

	t.Run("gate stays open once a push is allowed", func(t *testing.T) {
		g := newBootstrapGate(time.Minute)
		require.True(t, g.allow(logger, completeSnapshot))

		// Steady-state churn can legitimately produce a transiently incomplete
		// snapshot. That must never re-close the gate and withhold updates from
		// an Envoy that is already serving traffic.
		require.True(t, g.allow(logger, incompleteSnapshot))
	})

	t.Run("resumed stream is never held", func(t *testing.T) {
		g := newBootstrapGate(time.Minute)
		g.markResumedStream()

		// Envoy already finished initialization, so there is nothing to protect
		// and delaying its config would only slow reconvergence.
		require.True(t, g.allow(logger, incompleteSnapshot))
	})

	t.Run("resumed stream detected while already holding", func(t *testing.T) {
		g := newBootstrapGate(time.Minute)
		require.False(t, g.allow(logger, incompleteSnapshot))

		g.markResumedStream()
		require.True(t, g.allow(logger, incompleteSnapshot))
		require.Nil(t, g.expiryCh())
	})

	t.Run("expiry releases the push rather than wedging the stream", func(t *testing.T) {
		g := newBootstrapGate(time.Millisecond)

		require.False(t, g.allow(logger, incompleteSnapshot))

		select {
		case <-g.expiryCh():
		case <-time.After(time.Second):
			t.Fatal("gate deadline never fired")
		}
		g.markExpired()

		require.True(t, g.allow(logger, incompleteSnapshot),
			"an expired gate must push what it has; Envoy is configured to wait forever otherwise")
	})

	t.Run("negative timeout disables the gate", func(t *testing.T) {
		g := newBootstrapGate(-1)
		require.True(t, g.allow(logger, incompleteSnapshot))
		require.Nil(t, g.expiryCh())
	})

	t.Run("stop is safe on an unused gate", func(t *testing.T) {
		newBootstrapGate(time.Minute).stop()
	})
}

// TestBootstrapGate_NonAPIGatewayKinds asserts the gate is inert for every proxy
// kind other than api-gateway, so sidecars and mesh gateways keep their existing
// cold-start behaviour.
func TestBootstrapGate_NonAPIGatewayKinds(t *testing.T) {
	logger := testutil.Logger(t)

	for _, kind := range []structs.ServiceKind{
		structs.ServiceKindConnectProxy,
		structs.ServiceKindMeshGateway,
		structs.ServiceKindTerminatingGateway,
		structs.ServiceKindIngressGateway,
	} {
		t.Run(string(kind), func(t *testing.T) {
			snap := &proxycfg.ConfigSnapshot{Kind: kind}
			require.True(t, newBootstrapGate(time.Minute).allow(logger, snap))
		})
	}
}
