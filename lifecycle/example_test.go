// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MPL-2.0

package lifecycle_test

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/hashicorp/consul/lifecycle"
)

// printObserver is a custom output sink plugged in without touching the module.
type printObserver struct {
	lifecycle.NoopObserver
	out chan string
}

func (p printObserver) OnEvent(ev lifecycle.ControlEvent) {
	if ev.Kind == lifecycle.EventTransition {
		p.out <- fmt.Sprintf("%s: %s -> %s (generation %d)", ev.Component(), ev.From.State, ev.To.State, ev.To.Generation)
	}
}

// Example shows how a team adds a component with its own states, a consumer
// with a custom requirement, and an output sink, using only the public API.
func Example() {
	out := make(chan string, 16)
	ex := lifecycle.NewExchange(lifecycle.WithObserver(printObserver{out: out}))
	defer ex.Close()

	// A component defines its own states and maps each to a shared phase.
	const (
		connecting lifecycle.StateName = "Connecting"
		streaming  lifecycle.StateName = "Streaming"
		resyncing  lifecycle.StateName = "Resyncing"
	)
	conn, err := ex.Register(lifecycle.ComponentSpec{
		Name:    "dataplane/server-conn",
		Initial: connecting,
		States: map[lifecycle.StateName]lifecycle.StateSpec{
			connecting: {Phase: lifecycle.PhaseStarting, Next: []lifecycle.StateName{streaming}},
			streaming:  {Phase: lifecycle.PhaseReady, Next: []lifecycle.StateName{resyncing}},
			resyncing:  {Phase: lifecycle.PhaseTransitioning, Next: []lifecycle.StateName{streaming}, MaxDuration: time.Minute},
		},
	})
	if err != nil {
		panic(err)
	}

	// A consumer declares what it needs where it does the work.
	gate := ex.RequireFunc("dataplane/server-conn", "streaming with at least 1 server", func(s lifecycle.Status) bool {
		n, _ := strconv.Atoi(s.Attributes["servers"])
		return s.Phase == lifecycle.PhaseReady && n >= 1
	})
	if _, err := gate.Acquire(context.Background(), lifecycle.FailFast); err != nil {
		fmt.Println("not yet:", lifecycle.IsNotReady(err))
	}

	_ = conn.Transition(streaming, lifecycle.WithAttributes(map[string]string{"servers": "3"}))
	lease, err := gate.Acquire(context.Background(), lifecycle.FailFast)
	fmt.Println("acquired:", err == nil)
	lease.Release()

	// A disruptive change waits for in-flight work, then finishes.
	done, _ := conn.BeginTransition(context.Background(), resyncing, lifecycle.WithReason("server rebalance"))
	_ = done(streaming)

	for i := 0; i < 3; i++ {
		fmt.Println(<-out)
	}
	// Output:
	// not yet: true
	// acquired: true
	// dataplane/server-conn: Connecting -> Streaming (generation 1)
	// dataplane/server-conn: Streaming -> Resyncing (generation 1)
	// dataplane/server-conn: Resyncing -> Streaming (generation 2)
}
