// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package proxycfg

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
)

// panickingHandler stands in for a kind handler that panics while processing an
// update. state.run recovers those panics, which unwinds unsafeRun and closes
// doneCh - but leaves the state's context, and therefore all of its data source
// watches, alive.
type panickingHandler struct {
	inner kindHandler
}

func (h panickingHandler) initialize(ctx context.Context) (ConfigSnapshot, error) {
	return h.inner.initialize(ctx)
}

func (h panickingHandler) handleUpdate(context.Context, UpdateEvent, *ConfigSnapshot) error {
	panic("boom")
}

// TestStateClose_ReclaimsWatchesAfterPanic asserts that a state whose run loop
// died can still have its watches reclaimed.
//
// state.run recovers panics so that one bad update does not take the agent
// down, and local.Sync recreates terminated states on its next resync. But
// Close returns early once the run loop has stopped and Manager.register
// replaces a stopped state without closing it at all, so the dead state's
// context is never cancelled. Every watch it registered stays live for the
// lifetime of the process, and the replacement state registers a fresh set.
//
// See https://github.com/hashicorp/consul/issues/23931.
func TestStateClose_ReclaimsWatchesAfterPanic(t *testing.T) {
	ns := structs.TestNodeServiceProxy(t)
	proxyID := ProxyID{ServiceID: ns.CompoundServiceID()}

	sc := stateConfig{
		logger: testutil.Logger(t),
		source: &structs.QuerySource{Datacenter: "dc1"},
		dnsConfig: DNSConfig{
			Domain:    "consul.",
			AltDomain: "alt.consul.",
		},
	}
	rec := countingWatches(&sc)

	state, err := newState(proxyID, ns, testSource, aclToken, sc, rate.NewLimiter(rate.Inf, 0))
	require.NoError(t, err)
	state.handler = panickingHandler{inner: state.handler}

	_, err = state.Watch()
	require.NoError(t, err)

	calls := rec.callsFor(rootsWatchID)
	require.Len(t, calls, 1, "expected the roots watch to have been registered")
	watchCtx := calls[0].ctx

	// Kill the run loop the way a bad update does.
	state.ch <- UpdateEvent{CorrelationID: rootsWatchID}
	select {
	case <-state.doneCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the run loop to exit")
	}

	// The watches are still live at this point, which is expected: nothing has
	// asked for them to stop yet.
	require.NoError(t, watchCtx.Err(), "watch context was cancelled before Close")

	// Closing the state must reclaim them, whether or not the run loop is still
	// going.
	require.NoError(t, state.Close(false))

	select {
	case <-watchCtx.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not cancel the watches of a state whose run loop had already exited")
	}
}
