package checks

import (
	"fmt"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/mock"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testutil/retry"
	"github.com/hashicorp/consul/types"
	//"github.com/stretchr/testify/require"
)

// Test that we do a backoff on error.
func TestCheckAlias_remoteErrBackoff(t *testing.T) {
	t.Parallel()

	notify := newMockAliasNotify()
	chkID := types.CheckID("foo")
	rpc := &mockRPC{}
	chk := &CheckAlias{
		Node:      "remote",
		ServiceID: "web",
		CheckID:   chkID,
		Notify:    notify,
		RPC:       rpc,
	}

	rpc.Reply.Store(fmt.Errorf("failure"))

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
	t.Parallel()

	notify := newMockAliasNotify()
	chkID := types.CheckID("foo")
	rpc := &mockRPC{}
	chk := &CheckAlias{
		Node:      "remote",
		ServiceID: "web",
		CheckID:   chkID,
		Notify:    notify,
		RPC:       rpc,
	}

	rpc.Reply.Store(structs.IndexedHealthChecks{})

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
	t.Parallel()

	notify := newMockAliasNotify()
	chkID := types.CheckID("foo")
	rpc := &mockRPC{}
	chk := &CheckAlias{
		Node:      "remote",
		ServiceID: "web",
		CheckID:   chkID,
		Notify:    notify,
		RPC:       rpc,
	}

	rpc.Reply.Store(structs.IndexedHealthChecks{
		HealthChecks: []*structs.HealthCheck{
			// Should ignore non-matching node
			&structs.HealthCheck{
				Node:      "A",
				ServiceID: "web",
				Status:    api.HealthCritical,
			},

			// Node failure
			&structs.HealthCheck{
				Node:      "remote",
				ServiceID: "",
				Status:    api.HealthCritical,
			},

			// Match
			&structs.HealthCheck{
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
	chkID := types.CheckID("foo")
	rpc := &mockRPC{}
	chk := &CheckAlias{
		Node:      "remote",
		ServiceID: "web",
		CheckID:   chkID,
		Notify:    notify,
		RPC:       rpc,
	}

	rpc.Reply.Store(structs.IndexedHealthChecks{
		HealthChecks: []*structs.HealthCheck{
			// Should ignore non-matching node
			&structs.HealthCheck{
				Node:      "A",
				ServiceID: "web",
				Status:    api.HealthCritical,
			},

			// Should ignore non-matching service
			&structs.HealthCheck{
				Node:      "remote",
				ServiceID: "db",
				Status:    api.HealthCritical,
			},

			// Match
			&structs.HealthCheck{
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

// If any checks are critical, it should be critical
func TestCheckAlias_remoteCritical(t *testing.T) {
	t.Parallel()

	notify := newMockAliasNotify()
	chkID := types.CheckID("foo")
	rpc := &mockRPC{}
	chk := &CheckAlias{
		Node:      "remote",
		ServiceID: "web",
		CheckID:   chkID,
		Notify:    notify,
		RPC:       rpc,
	}

	rpc.Reply.Store(structs.IndexedHealthChecks{
		HealthChecks: []*structs.HealthCheck{
			// Should ignore non-matching node
			&structs.HealthCheck{
				Node:      "A",
				ServiceID: "web",
				Status:    api.HealthCritical,
			},

			// Should ignore non-matching service
			&structs.HealthCheck{
				Node:      "remote",
				ServiceID: "db",
				Status:    api.HealthCritical,
			},

			// Match
			&structs.HealthCheck{
				Node:      "remote",
				ServiceID: "web",
				Status:    api.HealthPassing,
			},

			&structs.HealthCheck{
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
	chkID := types.CheckID("foo")
	rpc := &mockRPC{}
	chk := &CheckAlias{
		Node:      "remote",
		ServiceID: "web",
		CheckID:   chkID,
		Notify:    notify,
		RPC:       rpc,
	}

	rpc.Reply.Store(structs.IndexedHealthChecks{
		HealthChecks: []*structs.HealthCheck{
			// Should ignore non-matching node
			&structs.HealthCheck{
				Node:      "A",
				ServiceID: "web",
				Status:    api.HealthCritical,
			},

			// Should ignore non-matching service
			&structs.HealthCheck{
				Node:      "remote",
				ServiceID: "db",
				Status:    api.HealthCritical,
			},

			// Match
			&structs.HealthCheck{
				Node:      "remote",
				ServiceID: "web",
				Status:    api.HealthPassing,
			},

			&structs.HealthCheck{
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
	chkID := types.CheckID("foo")
	rpc := &mockRPC{}
	chk := &CheckAlias{
		Node:    "remote",
		CheckID: chkID,
		Notify:  notify,
		RPC:     rpc,
	}

	rpc.Reply.Store(structs.IndexedHealthChecks{
		HealthChecks: []*structs.HealthCheck{
			// Should ignore non-matching node
			&structs.HealthCheck{
				Node:      "A",
				ServiceID: "web",
				Status:    api.HealthCritical,
			},

			// Should ignore any services
			&structs.HealthCheck{
				Node:      "remote",
				ServiceID: "db",
				Status:    api.HealthCritical,
			},

			// Match
			&structs.HealthCheck{
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

	notify := newMockAliasNotify()
	chkID := types.CheckID("foo")
	rpc := &mockRPC{}
	chk := &CheckAlias{
		Node:    "remote",
		CheckID: chkID,
		Notify:  notify,
		RPC:     rpc,
	}

	rpc.Reply.Store(structs.IndexedHealthChecks{
		HealthChecks: []*structs.HealthCheck{
			// Should ignore non-matching node
			&structs.HealthCheck{
				Node:      "A",
				ServiceID: "web",
				Status:    api.HealthCritical,
			},

			// Should ignore any services
			&structs.HealthCheck{
				Node:      "remote",
				ServiceID: "db",
				Status:    api.HealthCritical,
			},

			// Match
			&structs.HealthCheck{
				Node:   "remote",
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

type mockAliasNotify struct {
	*mock.Notify
}

func newMockAliasNotify() *mockAliasNotify {
	return &mockAliasNotify{
		Notify: mock.NewNotify(),
	}
}

func (m *mockAliasNotify) AddAliasCheck(chkID types.CheckID, serviceID string, ch chan<- struct{}) error {
	return nil
}

func (m *mockAliasNotify) RemoveAliasCheck(chkID types.CheckID, serviceID string) {
}

func (m *mockAliasNotify) Checks() map[types.CheckID]*structs.HealthCheck {
	return nil
}

// mockRPC is an implementation of RPC that can be used for tests. The
// atomic.Value fields can be set concurrently and will reflect on the next
// RPC call.
type mockRPC struct {
	Calls uint32       // Read-only, number of RPC calls
	Args  atomic.Value // Read-only, the last args sent

	// Write-only, the reply to send. If of type "error" then an error will
	// be returned from the RPC call.
	Reply atomic.Value
}

func (m *mockRPC) RPC(method string, args interface{}, reply interface{}) error {
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
	if v := m.Reply.Load(); v != nil {
		// Return an error if the reply is an error type
		if err, ok := v.(error); ok {
			return err
		}

		replyv.Set(reflect.ValueOf(v)) // Set to reply value if non-nil
	}

	return nil
}
