// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/checks"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testrpc"
)

// Integration test for ServiceHTTPBasedChecks cache-type
// Placed in agent pkg rather than cache-types to avoid circular dependency when importing agent.TestAgent
func TestAgent_ServiceHTTPChecksNotification(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	service := structs.NodeService{
		ID:      "web",
		Service: "web",
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := make(chan cache.UpdateEvent)

	// Watch for service check updates
	err := a.cache.Notify(ctx, cachetype.ServiceHTTPChecksName, &cachetype.ServiceHTTPChecksRequest{
		ServiceID: service.ID,
		NodeName:  a.Config.NodeName,
	}, "service-checks:"+service.ID, ch)
	if err != nil {
		t.Fatalf("failed to set cache notification: %v", err)
	}

	chkTypes := []*structs.CheckType{
		{
			CheckID:       "http-check",
			HTTP:          "localhost:8080/health",
			Interval:      5 * time.Second,
			OutputMaxSize: checks.DefaultBufSize,
		},
		{
			CheckID:  "grpc-check",
			GRPC:     "localhost:9090/v1.Health",
			Interval: 5 * time.Second,
		},
		{
			CheckID: "ttl-check",
			TTL:     10 * time.Second,
		},
	}
	// Adding TTL type should lead to a timeout, since only HTTP-based checks are watched
	if err := a.addServiceFromSource(&service, chkTypes[2:], false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("failed to add service: %v", err)
	}

	var val cache.UpdateEvent
	select {
	case val = <-ch:
		t.Fatal("got cache update for TTL check, expected timeout")
	case <-time.After(100 * time.Millisecond):
	}

	// Adding service with HTTP checks should lead notification for them
	if err := a.addServiceFromSource(&service, chkTypes[0:2], false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("failed to add service: %v", err)
	}

	select {
	case val = <-ch:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("didn't get cache update event")
	}

	got, ok := val.Result.([]structs.CheckType)
	if !ok {
		t.Fatalf("notified of result of wrong type, got %T, want []structs.CheckType", got)
	}
	want := chkTypes[0:2]
	for i, c := range want {
		require.Equal(t, *c, got[i])
	}

	// Removing the GRPC check should leave only the HTTP check
	if err := a.RemoveCheck(structs.NewCheckID(chkTypes[1].CheckID, nil), false); err != nil {
		t.Fatalf("failed to remove check: %v", err)
	}

	select {
	case val = <-ch:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("didn't get cache update event")
	}

	got, ok = val.Result.([]structs.CheckType)
	if !ok {
		t.Fatalf("notified of result of wrong type, got %T, want []structs.CheckType", got)
	}
	want = chkTypes[0:1]
	for i, c := range want {
		require.Equal(t, *c, got[i])
	}
}
