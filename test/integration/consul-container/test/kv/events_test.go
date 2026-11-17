// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package kv

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

// eventPayloads lists the payloads of the events an agent has received under
// name, oldest first.
func eventPayloads(t retry.TestingTB, c *api.Client, name string) []string {
	events, _, err := c.Event().List(name, nil)
	require.NoError(t, err)

	var out []string
	for _, ev := range events {
		out = append(out, string(ev.Payload))
	}
	return out
}

// TestEvents_Fire verifies user events: delivery to every agent, payloads,
// filtering by name, node, service and tag, and the payload size limit.
func TestEvents_Fire(t *testing.T) {
	t.Parallel()

	e := newEnv(t, envOpts{})
	clientNode, err := e.client.Agent().NodeName()
	require.NoError(t, err)

	t.Run("an event is delivered to every agent with its payload", func(t *testing.T) {
		id, _, err := e.server.Event().Fire(&api.UserEvent{Name: "deploy", Payload: []byte("v1.0.0")}, nil)
		require.NoError(t, err)
		require.NotEmpty(t, id)

		eventually(t, func(r *retry.R) {
			for name, c := range map[string]*api.Client{"server": e.server, "client": e.client} {
				events, _, err := c.Event().List("deploy", nil)
				require.NoError(r, err, name)
				require.Len(r, events, 1, name)
				require.Equal(r, id, events[0].ID, name)
				require.Equal(r, "v1.0.0", string(events[0].Payload), name)
				require.Equal(r, 1, events[0].Version, name)
				require.NotZero(r, events[0].LTime, name)
			}
		})
	})

	t.Run("listing can be restricted to one event name", func(t *testing.T) {
		_, _, err := e.server.Event().Fire(&api.UserEvent{Name: "rollback", Payload: []byte("r1")}, nil)
		require.NoError(t, err)

		eventually(t, func(r *retry.R) {
			require.Equal(r, []string{"r1"}, eventPayloads(r, e.client, "rollback"))
			require.Equal(r, []string{"v1.0.0"}, eventPayloads(r, e.client, "deploy"))

			all, _, err := e.client.Event().List("", nil)
			require.NoError(r, err)
			require.Len(r, all, 2)
		})
	})

	t.Run("events are kept in the order they were fired", func(t *testing.T) {
		for _, p := range []string{"a", "b", "c"} {
			_, _, err := e.server.Event().Fire(&api.UserEvent{Name: "ordered", Payload: []byte(p)}, nil)
			require.NoError(t, err)
		}

		eventually(t, func(r *retry.R) {
			require.Equal(r, []string{"a", "b", "c"}, eventPayloads(r, e.client, "ordered"))
		})
	})

	t.Run("a node filter limits which agents receive the event", func(t *testing.T) {
		_, _, err := e.server.Event().Fire(&api.UserEvent{Name: "node-only", Payload: []byte("x"), NodeFilter: "^" + clientNode + "$"}, nil)
		require.NoError(t, err)

		eventually(t, func(r *retry.R) {
			require.Equal(r, []string{"x"}, eventPayloads(r, e.client, "node-only"))
		})
		// Delivery to the other agent would have happened by now.
		time.Sleep(3 * time.Second)
		require.Empty(t, eventPayloads(t, e.server, "node-only"), "the server did not match the node filter")
	})

	t.Run("service and tag filters match agents running the service", func(t *testing.T) {
		require.NoError(t, e.client.Agent().ServiceRegister(&api.AgentServiceRegistration{
			Name: "events-svc", Port: 9000, Tags: []string{"blue"},
		}))

		for _, ev := range []*api.UserEvent{
			{Name: "svc-match", Payload: []byte("1"), ServiceFilter: "events-svc"},
			{Name: "svc-tag-match", Payload: []byte("2"), ServiceFilter: "events-svc", TagFilter: "blue"},
			{Name: "svc-tag-miss", Payload: []byte("3"), ServiceFilter: "events-svc", TagFilter: "green"},
			{Name: "svc-miss", Payload: []byte("4"), ServiceFilter: "no-such-service"},
		} {
			_, _, err := e.server.Event().Fire(ev, nil)
			require.NoError(t, err, ev.Name)
		}

		eventually(t, func(r *retry.R) {
			require.Equal(r, []string{"1"}, eventPayloads(r, e.client, "svc-match"))
			require.Equal(r, []string{"2"}, eventPayloads(r, e.client, "svc-tag-match"))
		})
		time.Sleep(3 * time.Second)
		require.Empty(t, eventPayloads(t, e.client, "svc-tag-miss"), "no instance has the green tag")
		require.Empty(t, eventPayloads(t, e.client, "svc-miss"), "the service does not run here")
		require.Empty(t, eventPayloads(t, e.server, "svc-match"), "the server does not run the service")
	})

	t.Run("a blocking list wakes when a new event arrives", func(t *testing.T) {
		_, meta, err := e.client.Event().List("", nil)
		require.NoError(t, err)

		woke := make(chan []*api.UserEvent, 1)
		go func() {
			events, _, err := e.client.Event().List("", &api.QueryOptions{WaitIndex: meta.LastIndex, WaitTime: 60 * time.Second})
			if err == nil {
				woke <- events
			}
		}()
		time.Sleep(time.Second)
		_, _, err = e.server.Event().Fire(&api.UserEvent{Name: "wakeup", Payload: []byte("now")}, nil)
		require.NoError(t, err)

		select {
		case events := <-woke:
			var names []string
			for _, ev := range events {
				names = append(names, ev.Name)
			}
			require.Contains(t, names, "wakeup")
		case <-time.After(45 * time.Second):
			t.Fatal("the blocking event list did not return")
		}
	})

	t.Run("payloads are limited in size", func(t *testing.T) {
		_, _, err := e.server.Event().Fire(&api.UserEvent{Name: "too-big", Payload: []byte(strings.Repeat("x", 1000))}, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Event payload too large")
	})
}

// TestEvents_CLI covers `consul event` and `consul watch -type=event`.
func TestEvents_CLI(t *testing.T) {
	t.Parallel()

	e := newEnv(t, envOpts{})

	t.Run("consul event fires an event", func(t *testing.T) {
		res := e.shell(t, "consul event -name=cli-deploy v2.0.0")
		require.Equal(t, 0, res.ExitCode, res.Output)
		require.Contains(t, res.Output, "Event ID:")

		eventually(t, func(r *retry.R) {
			require.Equal(r, []string{"v2.0.0"}, eventPayloads(r, e.client, "cli-deploy"))
		})
	})

	t.Run("consul watch runs a handler for each event", func(t *testing.T) {
		script := `rm -f /tmp/event-watch.log
consul watch -type=event -name=watched 'cat >> /tmp/event-watch.log; echo >> /tmp/event-watch.log' &
WATCH=$!
sleep 4
consul event -name=watched first >/dev/null
sleep 3
consul event -name=watched second >/dev/null
sleep 4
kill $WATCH
cat /tmp/event-watch.log`
		res := e.shell(t, script)
		require.Equal(t, 0, res.ExitCode, res.Output)

		// The handler receives the list of events as JSON on stdin each time
		// the watch fires; collect every payload it was shown.
		seen := map[string]bool{}
		dec := json.NewDecoder(strings.NewReader(res.Output))
		for dec.More() {
			var events []api.UserEvent
			if err := dec.Decode(&events); err != nil {
				break
			}
			for _, ev := range events {
				seen[string(ev.Payload)] = true
			}
		}
		require.True(t, seen["first"], "the handler must see the first event: %s", res.Output)
		require.True(t, seen["second"], "the handler must see the second event: %s", res.Output)
	})
}

// TestEvents_ACLs verifies that firing and listing events are authorized by
// event name.
func TestEvents_ACLs(t *testing.T) {
	t.Parallel()

	e := newEnv(t, envOpts{acl: true})

	deployer := e.newToken(t, "event-deployer", `event_prefix "deploy" { policy = "write" }`)
	reader := e.newToken(t, "event-reader", `event_prefix "" { policy = "read" }`)
	denied := e.newToken(t, "event-denied", `event_prefix "" { policy = "deny" }`)

	t.Run("firing needs write access to the event name", func(t *testing.T) {
		_, _, err := deployer.Event().Fire(&api.UserEvent{Name: "deploy-app", Payload: []byte("ok")}, nil)
		require.NoError(t, err)

		_, _, err = deployer.Event().Fire(&api.UserEvent{Name: "shutdown", Payload: []byte("no")}, nil)
		requireDenied(t, err, "an event name outside the token's prefix")
		_, _, err = reader.Event().Fire(&api.UserEvent{Name: "deploy-app", Payload: []byte("no")}, nil)
		requireDenied(t, err, "a read-only token")
		_, _, err = denied.Event().Fire(&api.UserEvent{Name: "deploy-app", Payload: []byte("no")}, nil)
		requireDenied(t, err, "a denied token")
	})

	t.Run("listing only returns events the token may read", func(t *testing.T) {
		_, _, err := e.client.Event().Fire(&api.UserEvent{Name: "shutdown", Payload: []byte("admin")}, nil)
		require.NoError(t, err)

		eventually(t, func(r *retry.R) {
			require.Equal(r, []string{"ok"}, eventPayloads(r, deployer, "deploy-app"))
		})

		all, _, err := reader.Event().List("", nil)
		require.NoError(t, err)
		require.Len(t, all, 2, "a reader sees every event")

		none, _, err := denied.Event().List("", nil)
		require.NoError(t, err)
		require.Empty(t, none, "a denied token sees nothing")
	})
}
