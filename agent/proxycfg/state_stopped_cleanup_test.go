// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package proxycfg

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
)

// stubKindHandler records the context its watches would be registered with
// and lets a test decide how each update is handled.
type stubKindHandler struct {
	mu       sync.Mutex
	watchCtx context.Context
	onUpdate func(UpdateEvent) error
}

func (h *stubKindHandler) initialize(ctx context.Context) (ConfigSnapshot, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.watchCtx = ctx
	return ConfigSnapshot{}, nil
}

func (h *stubKindHandler) handleUpdate(_ context.Context, u UpdateEvent, _ *ConfigSnapshot) error {
	return h.onUpdate(u)
}

func (h *stubKindHandler) ctx() context.Context {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.watchCtx
}

func newStoppedCleanupTestState(t *testing.T, handler kindHandler) *state {
	t.Helper()

	ns := structs.TestNodeServiceProxy(t)
	sc := stateConfig{
		logger: testutil.Logger(t),
		source: &structs.QuerySource{Datacenter: "dc1"},
	}
	recordWatches(&sc)

	s, err := newState(ProxyID{ServiceID: ns.CompoundServiceID()}, ns, testSource, aclToken, sc, rate.NewLimiter(rate.Inf, 0))
	require.NoError(t, err)
	if handler != nil {
		s.handler = handler
	}
	return s
}

// markStopped simulates a state whose run loop has exited without its
// watches being cancelled, and returns a context that is cancelled only by
// the state's cancel func.
func markStopped(s *state) context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	close(s.doneCh)
	return ctx
}

func requireStopped(t *testing.T, s *state) {
	t.Helper()
	select {
	case <-s.doneCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the state's run loop to stop")
	}
}

func requireCtxCancelled(t *testing.T, ctx context.Context, msg string) {
	t.Helper()
	select {
	case <-ctx.Done():
	case <-time.After(5 * time.Second):
		t.Fatal(msg)
	}
}

func TestState_RunLoopExitCancelsWatches(t *testing.T) {
	t.Run("recovered panic cancels every watch the state registered", func(t *testing.T) {
		handler := &stubKindHandler{onUpdate: func(UpdateEvent) error {
			panic("injected handler panic")
		}}
		s := newStoppedCleanupTestState(t, handler)

		_, err := s.Watch()
		require.NoError(t, err)
		s.ch <- UpdateEvent{CorrelationID: "boom", Result: struct{}{}}

		requireStopped(t, s)
		requireCtxCancelled(t, handler.ctx(), "watches must be cancelled when the run loop dies from a panic")
		require.False(t, s.failed(), "a recovered panic is not a terminal data source failure")
	})

	t.Run("terminal data source error cancels watches and marks the state failed", func(t *testing.T) {
		handler := &stubKindHandler{onUpdate: func(UpdateEvent) error { return nil }}
		s := newStoppedCleanupTestState(t, handler)

		_, err := s.Watch()
		require.NoError(t, err)
		s.ch <- UpdateEvent{CorrelationID: "terminal", Err: TerminalError(errors.New("acl not found"))}

		requireStopped(t, s)
		requireCtxCancelled(t, handler.ctx(), "watches must be cancelled after a terminal error")
		require.True(t, s.failed())
	})

	t.Run("close stops a running state", func(t *testing.T) {
		handler := &stubKindHandler{onUpdate: func(UpdateEvent) error { return nil }}
		s := newStoppedCleanupTestState(t, handler)

		_, err := s.Watch()
		require.NoError(t, err)
		require.NoError(t, s.Close(false))

		requireStopped(t, s)
		requireCtxCancelled(t, handler.ctx(), "close must cancel watches")
		require.False(t, s.failed())
	})
}

func TestState_CloseAfterRunStopped(t *testing.T) {
	t.Run("close cancels watches of a stopped state", func(t *testing.T) {
		s := newStoppedCleanupTestState(t, nil)
		watchCtx := markStopped(s)

		require.NoError(t, s.Close(false))
		require.Error(t, watchCtx.Err(), "close must cancel watches even after the run loop stopped")
	})

	t.Run("close is idempotent", func(t *testing.T) {
		s := newStoppedCleanupTestState(t, nil)
		watchCtx := markStopped(s)

		require.NoError(t, s.Close(false))
		require.NoError(t, s.Close(false))
		require.NoError(t, s.Close(true))
		require.Error(t, watchCtx.Err())
	})

	t.Run("close on a state that never started is a no-op", func(t *testing.T) {
		s := newStoppedCleanupTestState(t, nil)
		require.NoError(t, s.Close(false))
		require.False(t, s.failed())
	})
}

const otherTestSource ProxySource = "other"

func newStoppedCleanupTestManager(t *testing.T) *Manager {
	t.Helper()
	m, err := NewManager(ManagerConfig{
		Source:      &structs.QuerySource{Node: "node1", Datacenter: "dc1"},
		Logger:      testutil.Logger(t),
		DataSources: NewTestDataSources().ToDataSources(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { m.Close() })
	return m
}

// injectStoppedState places a state that has stopped running, but still holds
// live watches, into the manager's registry for the proxy.
func injectStoppedState(t *testing.T, m *Manager, source ProxySource) (*structs.NodeService, ProxyID, context.Context) {
	t.Helper()

	ns := structs.TestNodeServiceProxy(t)
	id := ProxyID{ServiceID: ns.CompoundServiceID()}

	old := newStoppedCleanupTestState(t, nil)
	old.source = source
	watchCtx := markStopped(old)

	m.mu.Lock()
	m.proxies[id] = old
	m.mu.Unlock()
	return ns, id, watchCtx
}

func TestManager_StoppedStateCleanup(t *testing.T) {
	t.Run("re-registering replaces a stopped state and cancels its watches", func(t *testing.T) {
		m := newStoppedCleanupTestManager(t)
		ns, id, oldWatches := injectStoppedState(t, m, testSource)
		m.mu.Lock()
		old := m.proxies[id]
		m.mu.Unlock()

		require.NoError(t, m.Register(id, ns, testSource, aclToken, false))

		require.Error(t, oldWatches.Err(), "the replaced state's watches must be cancelled")
		m.mu.Lock()
		current := m.proxies[id]
		m.mu.Unlock()
		require.NotSame(t, old, current)
		require.False(t, current.stoppedRunning())
	})

	t.Run("a stopped state from another source is still replaced and cancelled", func(t *testing.T) {
		m := newStoppedCleanupTestManager(t)
		ns, id, oldWatches := injectStoppedState(t, m, otherTestSource)

		require.NoError(t, m.Register(id, ns, testSource, aclToken, false))

		require.Error(t, oldWatches.Err())
		m.mu.Lock()
		require.Equal(t, testSource, m.proxies[id].source)
		m.mu.Unlock()
	})

	t.Run("deregistering a stopped state cancels its watches", func(t *testing.T) {
		m := newStoppedCleanupTestManager(t)
		_, id, oldWatches := injectStoppedState(t, m, testSource)

		m.Deregister(id, testSource)

		require.Error(t, oldWatches.Err())
		m.mu.Lock()
		_, ok := m.proxies[id]
		m.mu.Unlock()
		require.False(t, ok)
	})

	t.Run("closing the manager cancels watches of stopped states", func(t *testing.T) {
		m := newStoppedCleanupTestManager(t)
		_, _, oldWatches := injectStoppedState(t, m, testSource)

		require.NoError(t, m.Close())
		require.Error(t, oldWatches.Err())
	})

	t.Run("re-registering an unchanged running state keeps it", func(t *testing.T) {
		m := newStoppedCleanupTestManager(t)
		ns := structs.TestNodeServiceProxy(t)
		id := ProxyID{ServiceID: ns.CompoundServiceID()}

		require.NoError(t, m.Register(id, ns, testSource, aclToken, false))
		m.mu.Lock()
		first := m.proxies[id]
		m.mu.Unlock()

		require.NoError(t, m.Register(id, ns, testSource, aclToken, false))
		m.mu.Lock()
		require.Same(t, first, m.proxies[id])
		m.mu.Unlock()
		require.False(t, first.stoppedRunning())
	})
}
