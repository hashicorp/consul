package agent

import (
	"context"
	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/checks"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testrpc"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestAgent_ServiceHTTPChecksNotification(t *testing.T) {
	t.Parallel()

	a := NewTestAgent(t, t.Name(), "")
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

	// Adding first TTL type should lead to a timeout, since only HTTP-based checks are watched
	if err := a.AddService(&service, chkTypes[2:], false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("failed to add service: %v", err)
	}

	var val cache.UpdateEvent
	select {
	case val = <-ch:
		t.Fatal("unexpected cache update, wanted HTTP checks, got TTL")
	case <-time.After(100 * time.Millisecond):
	}

	// Adding service with HTTP check should lead notification for check
	if err := a.AddService(&service, chkTypes[0:1], false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("failed to add service: %v", err)
	}

	select {
	case val = <-ch:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("didn't get cache update event")
	}

	got, ok := val.Result.(*[]structs.CheckType)
	if !ok {
		t.Fatalf("notified of result of wrong type, got %T, want []structs.CheckType", got)
	}
	want := chkTypes[0:1]
	for i, c := range *got {
		require.Equal(t, c, *want[i])
	}

	// Adding GRPC check should lead to a notification from the cache with both checks
	hc := structs.HealthCheck{
		CheckID:   chkTypes[1].CheckID,
		ServiceID: service.ID,
	}
	if err := a.AddCheck(&hc, chkTypes[1], false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("failed to add service: %v", err)
	}

	select {
	case val = <-ch:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("didn't get cache update event")
	}

	got, ok = val.Result.(*[]structs.CheckType)
	if !ok {
		t.Fatalf("notified of result of wrong type, got %T, want []structs.CheckType", got)
	}
	want = chkTypes[0:2]
	for i, c := range *got {
		require.Equal(t, c, *want[i])
	}

	// Removing the GRPC check should leave only the HTTP check
	if err := a.RemoveCheck(chkTypes[1].CheckID, false); err != nil {
		t.Fatalf("failed to remove check: %v", err)
	}

	select {
	case val = <-ch:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("didn't get cache update event")
	}

	got, ok = val.Result.(*[]structs.CheckType)
	if !ok {
		t.Fatalf("notified of result of wrong type, got %T, want []structs.CheckType", got)
	}
	want = chkTypes[0:1]
	for i, c := range *got {
		require.Equal(t, c, *want[i])
	}

	// Removing the HTTP check should leave an empty list
	if err := a.RemoveCheck(chkTypes[0].CheckID, false); err != nil {
		t.Fatalf("failed to remove check: %v", err)
	}

	select {
	case val = <-ch:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("didn't get cache update event")
	}

	got, ok = val.Result.(*[]structs.CheckType)
	if !ok {
		t.Fatalf("notified of result of wrong type, got %T, want []structs.CheckType", got)
	}
	if len(*got) != 0 {
		t.Fatalf("expected empty result, got: %+v", got)
	}
}
