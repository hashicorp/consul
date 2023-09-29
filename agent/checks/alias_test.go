// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package checks

import (
	"context"
	"fmt"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/mock"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/types"
	//"github.com/stretchr/testify/require"
)

// Test that we do a backoff on error.
func TestCheckAlias_remoteErrBackoff(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	notify := newMockAliasNotify()
	chkID := structs.NewCheckID(types.CheckID("foo"), nil)
	rpc := &mockRPC{}
	chk := &CheckAlias{
		Node:      "remote",
		ServiceID: structs.ServiceID{ID: "web"},
		CheckID:   chkID,
		Notify:    notify,
		RPC:       rpc,
	}

	rpc.AddReply("Health.NodeChecks", fmt.Errorf("failure"))

	chk.Start()
	defer chk.Stop()

	time.Sleep(100 * time.Millisecond)
	if got, want := atomic.LoadUint32(&rpc.Calls), uint32(6); got > want {
		t.Fatalf("got %d updates want at most %d", got, want)
	}

	retry.Run(t, func(r *retry.R) {
		if got, want := notify.State(chkID), api.HealthCritical; got != want {
			r.Fatalf("got state %q want %q", got, want)
		}
	})
}

// No remote health checks should result in passing on the check.
func TestCheckAlias_remoteNoChecks(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	notify := newMockAliasNotify()
	chkID := structs.NewCheckID(types.CheckID("foo"), nil)
	rpc := &mockRPC{}
	chk := &CheckAlias{
		Node:      "remote",
		ServiceID: structs.ServiceID{ID: "web"},
		CheckID:   chkID,
		Notify:    notify,
		RPC:       rpc,
	}

	rpc.AddReply("Health.NodeChecks", structs.IndexedHealthChecks{})

	chk.Start()
	defer chk.Stop()
	retry.Run(t, func(r *retry.R) {
		if got, want := notify.State(chkID), api.HealthPassing; got != want {
			r.Fatalf("got state %q want %q", got, want)
		}
	})
}

// If the node is critical then the check is critical
func TestCheckAlias_remoteNodeFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	notify := newMockAliasNotify()
	chkID := structs.NewCheckID(types.CheckID("foo"), nil)
	rpc := &mockRPC{}
	chk := &CheckAlias{
		Node:      "remote",
		ServiceID: structs.ServiceID{ID: "web"},
		CheckID:   chkID,
		Notify:    notify,
		RPC:       rpc,
	}

	rpc.AddReply("Health.NodeChecks", structs.IndexedHealthChecks{
		HealthChecks: []*structs.HealthCheck{
			// Should ignore non-matching node
			{
				Node:      "A",
				ServiceID: "web",
				Status:    api.HealthCritical,
			},

			// Node failure
			{
				Node:      "remote",
				ServiceID: "",
				Status:    api.HealthCritical,
			},

			// Match
			{
				Node:      "remote",
				ServiceID: "web",
				Status:    api.HealthPassing,
			},
		},
	})

	chk.Start()
	defer chk.Stop()
	retry.Run(t, func(r *retry.R) {
		if got, want := notify.State(chkID), api.HealthCritical; got != want {
			r.Fatalf("got state %q want %q", got, want)
		}
	})
}

// Only passing should result in passing
func TestCheckAlias_remotePassing(t *testing.T) {
	t.Parallel()

	notify := newMockAliasNotify()
	chkID := structs.NewCheckID("foo", nil)
	rpc := &mockRPC{}
	chk := &CheckAlias{
		Node:      "remote",
		ServiceID: structs.ServiceID{ID: "web"},
		CheckID:   chkID,
		Notify:    notify,
		RPC:       rpc,
	}

	rpc.AddReply("Health.NodeChecks", structs.IndexedHealthChecks{
		HealthChecks: []*structs.HealthCheck{
			// Should ignore non-matching node
			{
				Node:      "A",
				ServiceID: "web",
				Status:    api.HealthCritical,
			},

			// Should ignore non-matching service
			{
				Node:      "remote",
				ServiceID: "db",
				Status:    api.HealthCritical,
			},

			// Match
			{
				Node:      "remote",
				ServiceID: "web",
				Status:    api.HealthPassing,
			},
		},
	})

	chk.Start()
	defer chk.Stop()
	retry.Run(t, func(r *retry.R) {
		if got, want := notify.State(chkID), api.HealthPassing; got != want {
			r.Fatalf("got state %q want %q", got, want)
		}
	})
}

// Remote service has no healtchecks, but service exists on remote host
func TestCheckAlias_remotePassingWithoutChecksButWithService(t *testing.T) {
	t.Parallel()

	notify := newMockAliasNotify()
	chkID := structs.NewCheckID("foo", nil)
	rpc := &mockRPC{}
	chk := &CheckAlias{
		Node:      "remote",
		ServiceID: structs.ServiceID{ID: "web"},
		CheckID:   chkID,
		Notify:    notify,
		RPC:       rpc,
	}

	rpc.AddReply("Health.NodeChecks", structs.IndexedHealthChecks{
		HealthChecks: []*structs.HealthCheck{
			// Should ignore non-matching node
			{
				Node:      "A",
				ServiceID: "web",
				Status:    api.HealthCritical,
			},

			// Should ignore non-matching service
			{
				Node:      "remote",
				ServiceID: "db",
				Status:    api.HealthCritical,
			},
		},
	})

	injected := structs.IndexedNodeServices{
		NodeServices: &structs.NodeServices{
			Node: &structs.Node{
				Node: "remote",
			},
			Services: make(map[string]*structs.NodeService),
		},
		QueryMeta: structs.QueryMeta{},
	}
	injected.NodeServices.Services["web"] = &structs.NodeService{
		Service: "web",
		ID:      "web",
	}
	rpc.AddReply("Catalog.NodeServices", injected)

	chk.Start()
	defer chk.Stop()
	retry.Run(t, func(r *retry.R) {
		if got, want := notify.State(chkID), api.HealthPassing; got != want {
			r.Fatalf("got state %q want %q", got, want)
		}
	})
}

// Remote service has no healtchecks, service does not exists on remote host
func TestCheckAlias_remotePassingWithoutChecksAndWithoutService(t *testing.T) {
	t.Parallel()

	notify := newMockAliasNotify()
	chkID := structs.NewCheckID("foo", nil)
	rpc := &mockRPC{}
	chk := &CheckAlias{
		Node:      "remote",
		ServiceID: structs.ServiceID{ID: "web"},
		CheckID:   chkID,
		Notify:    notify,
		RPC:       rpc,
	}

	rpc.AddReply("Health.NodeChecks", structs.IndexedHealthChecks{
		HealthChecks: []*structs.HealthCheck{
			// Should ignore non-matching node
			{
				Node:      "A",
				ServiceID: "web",
				Status:    api.HealthCritical,
			},

			// Should ignore non-matching service
			{
				Node:      "remote",
				ServiceID: "db",
				Status:    api.HealthCritical,
			},
		},
	})

	injected := structs.IndexedNodeServices{
		NodeServices: &structs.NodeServices{
			Node: &structs.Node{
				Node: "remote",
			},
			Services: make(map[string]*structs.NodeService),
		},
		QueryMeta: structs.QueryMeta{},
	}
	rpc.AddReply("Catalog.NodeServices", injected)

	chk.Start()
	defer chk.Stop()
	retry.Run(t, func(r *retry.R) {
		if got, want := notify.State(chkID), api.HealthCritical; got != want {
			r.Fatalf("got state %q want %q", got, want)
		}
	})
}

// If any checks are critical, it should be critical
func TestCheckAlias_remoteCritical(t *testing.T) {
	t.Parallel()

	notify := newMockAliasNotify()
	chkID := structs.NewCheckID("foo", nil)
	rpc := &mockRPC{}
	chk := &CheckAlias{
		Node:      "remote",
		ServiceID: structs.ServiceID{ID: "web"},
		CheckID:   chkID,
		Notify:    notify,
		RPC:       rpc,
	}

	rpc.AddReply("Health.NodeChecks", structs.IndexedHealthChecks{
		HealthChecks: []*structs.HealthCheck{
			// Should ignore non-matching node
			{
				Node:      "A",
				ServiceID: "web",
				Status:    api.HealthCritical,
			},

			// Should ignore non-matching service
			{
				Node:      "remote",
				ServiceID: "db",
				Status:    api.HealthCritical,
			},

			// Match
			{
				Node:      "remote",
				ServiceID: "web",
				Status:    api.HealthPassing,
			},

			{
				Node:      "remote",
				ServiceID: "web",
				Status:    api.HealthCritical,
			},
		},
	})

	chk.Start()
	defer chk.Stop()
	retry.Run(t, func(r *retry.R) {
		if got, want := notify.State(chkID), api.HealthCritical; got != want {
			r.Fatalf("got state %q want %q", got, want)
		}
	})
}

// If no checks are critical and at least one is warning, then it should warn
func TestCheckAlias_remoteWarning(t *testing.T) {
	t.Parallel()

	notify := newMockAliasNotify()
	chkID := structs.NewCheckID("foo", nil)
	rpc := &mockRPC{}
	chk := &CheckAlias{
		Node:      "remote",
		ServiceID: structs.NewServiceID("web", nil),
		CheckID:   chkID,
		Notify:    notify,
		RPC:       rpc,
	}

	rpc.AddReply("Health.NodeChecks", structs.IndexedHealthChecks{
		HealthChecks: []*structs.HealthCheck{
			// Should ignore non-matching node
			{
				Node:      "A",
				ServiceID: "web",
				Status:    api.HealthCritical,
			},

			// Should ignore non-matching service
			{
				Node:      "remote",
				ServiceID: "db",
				Status:    api.HealthCritical,
			},

			// Match
			{
				Node:      "remote",
				ServiceID: "web",
				Status:    api.HealthPassing,
			},

			{
				Node:      "remote",
				ServiceID: "web",
				Status:    api.HealthWarning,
			},
		},
	})

	chk.Start()
	defer chk.Stop()
	retry.Run(t, func(r *retry.R) {
		if got, want := notify.State(chkID), api.HealthWarning; got != want {
			r.Fatalf("got state %q want %q", got, want)
		}
	})
}

// Only passing should result in passing for node-only checks
func TestCheckAlias_remoteNodeOnlyPassing(t *testing.T) {
	t.Parallel()

	notify := newMockAliasNotify()
	chkID := structs.NewCheckID(types.CheckID("foo"), nil)
	rpc := &mockRPC{}
	chk := &CheckAlias{
		Node:    "remote",
		CheckID: chkID,
		Notify:  notify,
		RPC:     rpc,
	}

	rpc.AddReply("Health.NodeChecks", structs.IndexedHealthChecks{
		HealthChecks: []*structs.HealthCheck{
			// Should ignore non-matching node
			{
				Node:      "A",
				ServiceID: "web",
				Status:    api.HealthCritical,
			},

			// Should ignore any services
			{
				Node:      "remote",
				ServiceID: "db",
				Status:    api.HealthCritical,
			},

			// Match
			{
				Node:   "remote",
				Status: api.HealthPassing,
			},
		},
	})

	chk.Start()
	defer chk.Stop()
	retry.Run(t, func(r *retry.R) {
		if got, want := notify.State(chkID), api.HealthPassing; got != want {
			r.Fatalf("got state %q want %q", got, want)
		}
	})
}

// Only critical should result in passing for node-only checks
func TestCheckAlias_remoteNodeOnlyCritical(t *testing.T) {
	t.Parallel()

	run := func(t *testing.T, responseNodeName string) {
		notify := newMockAliasNotify()
		chkID := structs.NewCheckID(types.CheckID("foo"), nil)
		rpc := &mockRPC{}
		chk := &CheckAlias{
			Node:    "remote",
			CheckID: chkID,
			Notify:  notify,
			RPC:     rpc,
		}

		rpc.AddReply("Health.NodeChecks", structs.IndexedHealthChecks{
			HealthChecks: []*structs.HealthCheck{
				// Should ignore non-matching node
				{
					Node:      "A",
					ServiceID: "web",
					Status:    api.HealthCritical,
				},

				// Should ignore any services
				{
					Node:      responseNodeName,
					ServiceID: "db",
					Status:    api.HealthCritical,
				},

				// Match
				{
					Node:   responseNodeName,
					Status: api.HealthCritical,
				},
			},
		})

		chk.Start()
		defer chk.Stop()
		retry.Run(t, func(r *retry.R) {
			if got, want := notify.State(chkID), api.HealthCritical; got != want {
				r.Fatalf("got state %q want %q", got, want)
			}
		})
	}

	t.Run("same case node name", func(t *testing.T) {
		run(t, "remote")
	})
	t.Run("lowercase node name", func(t *testing.T) {
		run(t, "ReMoTe")
	})
}

type mockAliasNotify struct {
	*mock.Notify
}

func newMockAliasNotify() *mockAliasNotify {
	return &mockAliasNotify{
		Notify: mock.NewNotify(),
	}
}

func (m *mockAliasNotify) AddAliasCheck(chkID structs.CheckID, serviceID structs.ServiceID, ch chan<- struct{}) error {
	return nil
}

func (m *mockAliasNotify) RemoveAliasCheck(chkID structs.CheckID, serviceID structs.ServiceID) {
}

func (m *mockAliasNotify) Checks(*acl.EnterpriseMeta) map[structs.CheckID]*structs.HealthCheck {
	return nil
}

// mockRPC is an implementation of RPC that can be used for tests. The
// atomic.Value fields can be set concurrently and will reflect on the next
// RPC call.
type mockRPC struct {
	Calls uint32       // Read-only, number of RPC calls
	Args  atomic.Value // Read-only, the last args sent

	// Write-only, the replies to send, indexed per method. If of type "error" then an error will
	// be returned from the RPC call.
	Replies map[string]*atomic.Value
}

func (m *mockRPC) AddReply(method string, reply interface{}) {
	if m.Replies == nil {
		m.Replies = make(map[string]*atomic.Value)
	}
	val := &atomic.Value{}
	val.Store(reply)
	m.Replies[method] = val

}

func (m *mockRPC) RPC(ctx context.Context, method string, args interface{}, reply interface{}) error {
	atomic.AddUint32(&m.Calls, 1)
	m.Args.Store(args)

	// We don't adhere to blocking queries, so this helps prevent
	// too much CPU usage on the check loop.
	time.Sleep(10 * time.Millisecond)

	// This whole machinery below sets the value of the reply. This is
	// basically what net/rpc does internally, though much condensed.
	replyv := reflect.ValueOf(reply)
	if replyv.Kind() != reflect.Ptr {
		return fmt.Errorf("RPC reply must be pointer")
	}
	replyv = replyv.Elem()                  // Get pointer value
	replyv.Set(reflect.Zero(replyv.Type())) // Reset to zero value
	repl := m.Replies[method]
	if repl == nil {
		return fmt.Errorf("No Such Method: %s", method)
	}
	if v := m.Replies[method].Load(); v != nil {
		// Return an error if the reply is an error type
		if err, ok := v.(error); ok {
			return err
		}
		replyv.Set(reflect.ValueOf(v)) // Set to reply value if non-nil
	}

	return nil
}

// Test that local checks immediately reflect the subject states when added and
// don't require an update to the subject before being accurate.
func TestCheckAlias_localInitialStatus(t *testing.T) {
	t.Parallel()

	notify := newMockAliasNotify()
	// We fake a local service web to ensure check if passing works
	notify.Notify.AddServiceID(structs.ServiceID{ID: "web"})
	chkID := structs.NewCheckID(types.CheckID("foo"), nil)
	rpc := &mockRPC{}
	chk := &CheckAlias{
		ServiceID: structs.ServiceID{ID: "web"},
		CheckID:   chkID,
		Notify:    notify,
		RPC:       rpc,
	}

	chk.Start()
	defer chk.Stop()

	// Don't touch the aliased service or it's checks (there are none but this is
	// valid and should be consisded "passing").

	retry.Run(t, func(r *retry.R) {
		if got, want := notify.State(chkID), api.HealthPassing; got != want {
			r.Fatalf("got state %q want %q", got, want)
		}
	})
}

// Local check on non-existing service
func TestCheckAlias_localInitialStatusShouldFailBecauseNoService(t *testing.T) {
	t.Parallel()

	notify := newMockAliasNotify()
	chkID := structs.NewCheckID(types.CheckID("foo"), nil)
	rpc := &mockRPC{}
	chk := &CheckAlias{
		ServiceID: structs.ServiceID{ID: "web"},
		CheckID:   chkID,
		Notify:    notify,
		RPC:       rpc,
	}

	chk.Start()
	defer chk.Stop()

	retry.Run(t, func(r *retry.R) {
		if got, want := notify.State(chkID), api.HealthCritical; got != want {
			r.Fatalf("got state %q want %q", got, want)
		}
	})
}
