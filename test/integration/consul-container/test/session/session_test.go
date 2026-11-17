// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package session

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

func createSession(t *testing.T, c *api.Client, entry *api.SessionEntry) string {
	t.Helper()
	id, _, err := c.Session().Create(entry, nil)
	require.NoError(t, err)
	require.NotEmpty(t, id)
	return id
}

func sessionInfo(t retry.TestingTB, c *api.Client, id string) *api.SessionEntry {
	entry, _, err := c.Session().Info(id, nil)
	require.NoError(t, err)
	return entry
}

func keyState(t retry.TestingTB, c *api.Client, key string) *api.KVPair {
	pair, _, err := c.KV().Get(key, nil)
	require.NoError(t, err)
	return pair
}

func acquire(t *testing.T, c *api.Client, key, session string) bool {
	t.Helper()
	ok, _, err := c.KV().Acquire(&api.KVPair{Key: key, Value: []byte("held-by-" + session[:8]), Session: session}, nil)
	require.NoError(t, err)
	return ok
}

// TestSession_Lifecycle covers creating, inspecting, renewing and destroying
// sessions, and the validation of their settings.
func TestSession_Lifecycle(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "", "")
	c := e.client
	nodeName, err := c.Agent().NodeName()
	require.NoError(t, err)

	t.Run("a session records its settings and is bound to the agent's node", func(t *testing.T) {
		id := createSession(t, c, &api.SessionEntry{Name: "web-leader", TTL: "30s", Behavior: api.SessionBehaviorDelete, LockDelay: 5 * time.Second})

		got := sessionInfo(t, e.server, id)
		require.NotNil(t, got)
		require.Equal(t, id, got.ID)
		require.Equal(t, "web-leader", got.Name)
		require.Equal(t, nodeName, got.Node)
		require.Equal(t, "30s", got.TTL)
		require.Equal(t, api.SessionBehaviorDelete, got.Behavior)
		require.Equal(t, 5*time.Second, got.LockDelay)
		require.Equal(t, []string{"serfHealth"}, got.NodeChecks, "by default a session depends on its node being alive")
	})

	t.Run("defaults", func(t *testing.T) {
		id := createSession(t, c, &api.SessionEntry{})

		got := sessionInfo(t, e.server, id)
		require.Equal(t, api.SessionBehaviorRelease, got.Behavior)
		require.Equal(t, 15*time.Second, got.LockDelay)
		require.Empty(t, got.TTL, "without a TTL the session lives until it is destroyed or its node fails")
	})

	t.Run("sessions are listed cluster-wide and per node", func(t *testing.T) {
		id := createSession(t, c, &api.SessionEntry{Name: "listed"})

		all, _, err := e.server.Session().List(nil)
		require.NoError(t, err)
		var ids []string
		for _, s := range all {
			ids = append(ids, s.ID)
		}
		require.Contains(t, ids, id)

		onNode, _, err := e.server.Session().Node(nodeName, nil)
		require.NoError(t, err)
		ids = nil
		for _, s := range onNode {
			ids = append(ids, s.ID)
		}
		require.Contains(t, ids, id)

		elsewhere, _, err := e.server.Session().Node("no-such-node", nil)
		require.NoError(t, err)
		require.Empty(t, elsewhere)
	})

	t.Run("renewing returns the session, and an unknown session cannot be renewed", func(t *testing.T) {
		id := createSession(t, c, &api.SessionEntry{Name: "renewable", TTL: "30s"})

		renewed, _, err := c.Session().Renew(id, nil)
		require.NoError(t, err)
		require.NotNil(t, renewed)
		require.Equal(t, id, renewed.ID)

		missing, _, err := c.Session().Renew("00000000-0000-0000-0000-000000000000", nil)
		require.NoError(t, err)
		require.Nil(t, missing, "an unknown or expired session cannot be renewed")
	})

	t.Run("destroying removes the session and is idempotent", func(t *testing.T) {
		id := createSession(t, c, &api.SessionEntry{Name: "short-lived"})

		_, err := c.Session().Destroy(id, nil)
		require.NoError(t, err)
		require.Nil(t, sessionInfo(t, e.server, id))

		_, err = c.Session().Destroy(id, nil)
		require.NoError(t, err)
	})

	t.Run("invalid settings are rejected", func(t *testing.T) {
		for name, tc := range map[string]struct {
			entry   *api.SessionEntry
			wantErr string
		}{
			"a TTL below the minimum":   {&api.SessionEntry{TTL: "1s"}, "Invalid Session TTL"},
			"a TTL above the maximum":   {&api.SessionEntry{TTL: "25h"}, "Invalid Session TTL"},
			"an unknown behavior":       {&api.SessionEntry{Behavior: "explode"}, "Invalid Behavior"},
			"a check that is not there": {&api.SessionEntry{NodeChecks: []string{"no-such-check"}}, "Missing check"},
		} {
			_, _, err := c.Session().Create(tc.entry, nil)
			require.Error(t, err, name)
			require.Contains(t, err.Error(), tc.wantErr, name)
		}
	})
}

// TestSession_Locks verifies key locking with sessions: exclusive acquisition,
// release, and what happens to a lock when its session ends.
func TestSession_Locks(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "", "")
	c := e.client
	kv := c.KV()

	holder := createSession(t, c, &api.SessionEntry{Name: "holder", TTL: "60s", LockDelay: time.Second})
	rival := createSession(t, c, &api.SessionEntry{Name: "rival", TTL: "60s", LockDelay: time.Second})

	t.Run("only one session can hold a lock", func(t *testing.T) {
		require.True(t, acquire(t, c, "locks/leader", holder))

		pair := keyState(t, c, "locks/leader")
		require.Equal(t, holder, pair.Session)
		require.EqualValues(t, 1, pair.LockIndex)

		require.False(t, acquire(t, c, "locks/leader", rival), "the lock is taken")
		require.Equal(t, holder, keyState(t, c, "locks/leader").Session)
	})

	t.Run("acquiring a lock you already hold is allowed and does not count again", func(t *testing.T) {
		require.True(t, acquire(t, c, "locks/leader", holder))
		require.EqualValues(t, 1, keyState(t, c, "locks/leader").LockIndex)
	})

	t.Run("only the holder can release", func(t *testing.T) {
		ok, _, err := kv.Release(&api.KVPair{Key: "locks/leader", Session: rival}, nil)
		require.NoError(t, err)
		require.False(t, ok)
		require.Equal(t, holder, keyState(t, c, "locks/leader").Session)

		ok, _, err = kv.Release(&api.KVPair{Key: "locks/leader", Session: holder}, nil)
		require.NoError(t, err)
		require.True(t, ok)
		require.Empty(t, keyState(t, c, "locks/leader").Session, "the key survives its release")
	})

	t.Run("each acquisition increments the lock index", func(t *testing.T) {
		require.True(t, acquire(t, c, "locks/leader", rival))
		require.EqualValues(t, 2, keyState(t, c, "locks/leader").LockIndex)
	})

	t.Run("acquiring with an unknown session fails", func(t *testing.T) {
		_, _, err := kv.Acquire(&api.KVPair{Key: "locks/ghost", Value: []byte("x"), Session: "00000000-0000-0000-0000-000000000000"}, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid session")
	})
}

// TestSession_Behaviors verifies what happens to held keys when a session is
// destroyed: release keeps the data and frees the lock (after the lock delay),
// delete removes the key.
func TestSession_Behaviors(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "", "")
	c := e.client

	t.Run("release keeps the key, frees the lock and delays the next holder", func(t *testing.T) {
		first := createSession(t, c, &api.SessionEntry{Name: "first", Behavior: api.SessionBehaviorRelease, LockDelay: 3 * time.Second})
		second := createSession(t, c, &api.SessionEntry{Name: "second", Behavior: api.SessionBehaviorRelease})
		require.True(t, acquire(t, c, "behavior/release", first))

		_, err := c.Session().Destroy(first, nil)
		require.NoError(t, err)

		pair := keyState(t, c, "behavior/release")
		require.NotNil(t, pair, "release behavior keeps the key")
		require.Empty(t, pair.Session)
		require.Contains(t, string(pair.Value), "held-by-")

		require.False(t, acquire(t, c, "behavior/release", second), "the lock delay protects against a stale holder that has not noticed yet")

		eventually(t, func(r *retry.R) {
			ok, _, err := c.KV().Acquire(&api.KVPair{Key: "behavior/release", Value: []byte("second"), Session: second}, nil)
			require.NoError(r, err)
			require.True(r, ok, "the lock is available once the delay has passed")
		})
	})

	t.Run("delete removes the key", func(t *testing.T) {
		id := createSession(t, c, &api.SessionEntry{Name: "deleter", Behavior: api.SessionBehaviorDelete, LockDelay: 0})
		require.True(t, acquire(t, c, "behavior/delete", id))
		require.NotNil(t, keyState(t, c, "behavior/delete"))

		_, err := c.Session().Destroy(id, nil)
		require.NoError(t, err)

		require.Nil(t, keyState(t, c, "behavior/delete"))
	})

	t.Run("a session holds many keys and gives them all up together", func(t *testing.T) {
		id := createSession(t, c, &api.SessionEntry{Name: "many", Behavior: api.SessionBehaviorDelete, LockDelay: 0})
		for _, key := range []string{"behavior/many/a", "behavior/many/b", "behavior/many/c"} {
			require.True(t, acquire(t, c, key, id), key)
		}

		_, err := c.Session().Destroy(id, nil)
		require.NoError(t, err)

		list, _, err := c.KV().List("behavior/many/", nil)
		require.NoError(t, err)
		require.Empty(t, list)
	})
}

// TestSession_Expiry verifies that a session with a TTL ends when it is not
// renewed, and that renewals keep it alive. Consul allows a session to outlive
// its TTL by up to a factor of two, so these tests wait up to that long.
func TestSession_Expiry(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "", "")
	c := e.client

	waitGone := func(t *testing.T, id string) {
		t.Helper()
		eventually(t, func(r *retry.R) {
			require.Nil(r, sessionInfo(r, e.server, id), "the session should have expired")
		})
	}

	t.Run("an unrenewed session expires and releases its lock", func(t *testing.T) {
		t.Parallel()

		id := createSession(t, c, &api.SessionEntry{Name: "expires-release", TTL: "10s", Behavior: api.SessionBehaviorRelease, LockDelay: 0})
		require.True(t, acquire(t, c, "expiry/release", id))

		waitGone(t, id)
		pair := keyState(t, c, "expiry/release")
		require.NotNil(t, pair, "release behavior keeps the key")
		require.Empty(t, pair.Session)
	})

	t.Run("an unrenewed session expires and deletes its key", func(t *testing.T) {
		t.Parallel()

		id := createSession(t, c, &api.SessionEntry{Name: "expires-delete", TTL: "10s", Behavior: api.SessionBehaviorDelete, LockDelay: 0})
		require.True(t, acquire(t, c, "expiry/delete", id))

		waitGone(t, id)
		require.Nil(t, keyState(t, c, "expiry/delete"))
	})

	t.Run("renewing keeps a session alive beyond its TTL", func(t *testing.T) {
		t.Parallel()

		id := createSession(t, c, &api.SessionEntry{Name: "renewed", TTL: "10s", Behavior: api.SessionBehaviorDelete, LockDelay: 0})
		require.True(t, acquire(t, c, "expiry/renewed", id))

		// Longer than twice the TTL, so only the renewals can explain survival.
		deadline := time.Now().Add(25 * time.Second)
		for time.Now().Before(deadline) {
			_, _, err := c.Session().Renew(id, nil)
			require.NoError(t, err)
			time.Sleep(4 * time.Second)
		}
		require.NotNil(t, sessionInfo(t, e.server, id), "a renewed session must still exist")
		require.NotNil(t, keyState(t, c, "expiry/renewed"))

		waitGone(t, id)
		require.Nil(t, keyState(t, c, "expiry/renewed"), "once renewals stop it expires like any other")
	})
}

// TestSession_HealthChecks verifies that a session bound to health checks ends
// when one of them stops passing.
func TestSession_HealthChecks(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "", "")
	c := e.client

	t.Run("a node check", func(t *testing.T) {
		require.NoError(t, c.Agent().CheckRegister(&api.AgentCheckRegistration{
			ID: "session-node-check", Name: "session-node-check",
			AgentServiceCheck: api.AgentServiceCheck{TTL: "10m", Status: api.HealthPassing},
		}))

		id := createSession(t, c, &api.SessionEntry{
			Name: "node-check-bound", NodeChecks: []string{"session-node-check"}, Behavior: api.SessionBehaviorDelete, LockDelay: 0,
		})
		require.Equal(t, []string{"session-node-check"}, sessionInfo(t, e.server, id).NodeChecks)
		require.True(t, acquire(t, c, "health/node", id))

		require.NoError(t, c.Agent().UpdateTTL("session-node-check", "failing", api.HealthCritical))

		eventually(t, func(r *retry.R) {
			require.Nil(r, sessionInfo(r, e.server, id))
			require.Nil(r, keyState(r, e.server, "health/node"))
		})
	})

	t.Run("a service check", func(t *testing.T) {
		require.NoError(t, c.Agent().ServiceRegister(&api.AgentServiceRegistration{
			ID: "session-svc", Name: "session-svc", Port: 9000,
			Check: &api.AgentServiceCheck{TTL: "10m", Status: api.HealthPassing},
		}))

		id := createSession(t, c, &api.SessionEntry{
			Name:          "service-check-bound",
			NodeChecks:    []string{},
			ServiceChecks: []api.ServiceCheck{{ID: "service:session-svc"}},
			Behavior:      api.SessionBehaviorRelease,
			LockDelay:     0,
		})
		require.True(t, acquire(t, c, "health/service", id))

		require.NoError(t, c.Agent().UpdateTTL("service:session-svc", "failing", api.HealthCritical))

		eventually(t, func(r *retry.R) {
			require.Nil(r, sessionInfo(r, e.server, id))
			pair := keyState(r, e.server, "health/service")
			require.NotNil(r, pair, "release behavior keeps the key")
			require.Empty(r, pair.Session)
		})
	})

	t.Run("a session cannot be created on a check that is not passing", func(t *testing.T) {
		require.NoError(t, c.Agent().CheckRegister(&api.AgentCheckRegistration{
			ID: "already-critical", Name: "already-critical",
			AgentServiceCheck: api.AgentServiceCheck{TTL: "10m", Status: api.HealthCritical},
		}))

		_, _, err := c.Session().Create(&api.SessionEntry{NodeChecks: []string{"already-critical"}}, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "is in critical state")
	})
}
