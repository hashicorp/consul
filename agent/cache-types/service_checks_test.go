package cachetype_test

import (
	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/checks"
	"github.com/hashicorp/consul/agent/local"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/types"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestServiceHTTPChecks_Fetch(t *testing.T) {
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

	svcState := local.ServiceState{
		Service: &structs.NodeService{
			ID: "web",
		},
	}

	// Create mockAgent and cache type
	a := newMockAgent()
	a.LocalState().SetServiceState(&svcState)
	typ := cachetype.ServiceHTTPChecks{Agent: a}

	// Adding TTL check should not yield result from Fetch since TTL checks aren't tracked
	if err := a.AddCheck(*chkTypes[2]); err != nil {
		t.Fatalf("failed to add check: %v", err)
	}
	result, err := typ.Fetch(
		cache.FetchOptions{},
		&cachetype.ServiceHTTPChecksRequest{ServiceID: svcState.Service.ID, MaxQueryTime: 100 * time.Millisecond},
	)
	if err != nil {
		t.Fatalf("failed to fetch: %v", err)
	}
	got, ok := result.Value.(*[]structs.CheckType)
	if !ok {
		t.Fatalf("fetched value of wrong type, got %T, want *[]structs.CheckType", got)
	}
	require.Empty(t, *got)

	// Adding HTTP check should yield check in Fetch
	if err := a.AddCheck(*chkTypes[0]); err != nil {
		t.Fatalf("failed to add check: %v", err)
	}
	result, err = typ.Fetch(
		cache.FetchOptions{},
		&cachetype.ServiceHTTPChecksRequest{ServiceID: svcState.Service.ID},
	)
	if err != nil {
		t.Fatalf("failed to fetch: %v", err)
	}
	if result.Index != 1 {
		t.Fatalf("expected index of 1 after first cache hit, got %d", result.Index)
	}

	got, ok = result.Value.(*[]structs.CheckType)
	if !ok {
		t.Fatalf("fetched value of wrong type, got %T, want *[]structs.CheckType", got)
	}
	want := chkTypes[0:1]
	for i, c := range *got {
		require.Equal(t, c, *want[i])
	}

	// Adding GRPC check should yield both checks in Fetch
	if err := a.AddCheck(*chkTypes[1]); err != nil {
		t.Fatalf("failed to add check: %v", err)
	}
	result2, err := typ.Fetch(
		cache.FetchOptions{LastResult: &result},
		&cachetype.ServiceHTTPChecksRequest{ServiceID: svcState.Service.ID},
	)
	if err != nil {
		t.Fatalf("failed to fetch: %v", err)
	}
	if result2.Index != 2 {
		t.Fatalf("expected index of 2 after second request, got %d", result2.Index)
	}

	got, ok = result2.Value.(*[]structs.CheckType)
	if !ok {
		t.Fatalf("fetched value of wrong type, got %T, want *[]structs.CheckType", got)
	}
	want = chkTypes[0:2]
	for i, c := range *got {
		require.Equal(t, c, *want[i])
	}

	// Removing GRPC check should yield HTTP check in Fetch
	if err := a.RemoveCheck(chkTypes[1].CheckID); err != nil {
		t.Fatalf("failed to add check: %v", err)
	}
	result3, err := typ.Fetch(
		cache.FetchOptions{LastResult: &result2},
		&cachetype.ServiceHTTPChecksRequest{ServiceID: svcState.Service.ID},
	)
	if err != nil {
		t.Fatalf("failed to fetch: %v", err)
	}
	if result3.Index != 3 {
		t.Fatalf("expected index of 3 after third request, got %d", result3.Index)
	}

	got, ok = result3.Value.(*[]structs.CheckType)
	if !ok {
		t.Fatalf("fetched value of wrong type, got %T, want *[]structs.CheckType", got)
	}
	want = chkTypes[0:1]
	for i, c := range *got {
		require.Equal(t, c, *want[i])
	}

	// Fetching again should yield no change in result nor index
	result4, err := typ.Fetch(
		cache.FetchOptions{LastResult: &result3},
		&cachetype.ServiceHTTPChecksRequest{ServiceID: svcState.Service.ID, MaxQueryTime: 100 * time.Millisecond},
	)
	if err != nil {
		t.Fatalf("failed to fetch: %v", err)
	}
	if result4.Index != 3 {
		t.Fatalf("expected index of 3 after fetch timeout, got %d", result4.Index)
	}

	got, ok = result4.Value.(*[]structs.CheckType)
	if !ok {
		t.Fatalf("fetched value of wrong type, got %T, want *[]structs.CheckType", got)
	}
	want = chkTypes[0:1]
	for i, c := range *got {
		require.Equal(t, c, *want[i])
	}
}

type mockAgent struct {
	state   *local.State
	pauseCh <-chan struct{}
	checks  []structs.CheckType
}

func newMockAgent() *mockAgent {
	m := mockAgent{
		state:   local.NewState(local.Config{NodeID: "host"}, nil, new(token.Store)),
		pauseCh: make(chan struct{}),
		checks:  make([]structs.CheckType, 0),
	}
	m.state.TriggerSyncChanges = func() {}
	return &m
}

func (m *mockAgent) ServiceHTTPBasedChecks(id string) []structs.CheckType {
	return m.checks
}

func (m *mockAgent) LocalState() *local.State {
	return m.state
}

func (m *mockAgent) SyncPausedCh() <-chan struct{} {
	return m.pauseCh
}

func (m *mockAgent) AddCheck(check structs.CheckType) error {
	if check.IsHTTP() || check.IsGRPC() {
		m.checks = append(m.checks, check)
	}
	return nil
}

func (m *mockAgent) RemoveCheck(id types.CheckID) error {
	new := make([]structs.CheckType, 0)
	for _, c := range m.checks {
		if c.CheckID != id {
			new = append(new, c)
		}
	}
	m.checks = new
	return nil
}
