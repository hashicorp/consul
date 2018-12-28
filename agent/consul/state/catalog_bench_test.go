package state

import (
	"fmt"
	"testing"

	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/types"
)

// results is a package level variable to prevent compiler optimising away the
// call whose results are never visible.
var results structs.CheckServiceNodes

func benchCatalogParseCheckServiceNodes(b *testing.B, instanceCount int, watchSet bool) {
	b.StopTimer()

	// Setup the state store
	s, err := NewStateStore(nil)
	if err != nil {
		b.Fatalf("failed creating state store: %s", err)
	}

	// Setup the service instances
	for i := 0; i < instanceCount; i++ {
		req := &structs.RegisterRequest{
			Datacenter: "dc1",
			ID:         types.NodeID(fmt.Sprintf("11111111-2222-3333-4444-%012d", i)),
			Node:       fmt.Sprintf("node-%d.example.com", i),
			Address:    fmt.Sprintf("10.0.%d.%d", i/256, i%256),
			TaggedAddresses: map[string]string{
				"lan": fmt.Sprintf("10.0.%d.%d", i/256, i%256),
				"wan": fmt.Sprintf("10.0.%d.%d", i/256, i%256),
			},
			Service: &structs.NodeService{
				ID:      fmt.Sprintf("test-%d", i),
				Service: "test",
				Port:    i,
			},
			Checks: structs.HealthChecks{
				// 2 node-level health checks
				&structs.HealthCheck{
					Node:    fmt.Sprintf("node-%d.example.com", i),
					CheckID: types.CheckID(fmt.Sprintf("node-%d-1", i)),
					Name:    fmt.Sprintf("node-%d-1", i),
					Status:  api.HealthPassing,
				},
				&structs.HealthCheck{
					Node:    fmt.Sprintf("node-%d.example.com", i),
					CheckID: types.CheckID(fmt.Sprintf("node-%d-2", i)),
					Name:    fmt.Sprintf("node-%d-2", i),
					Status:  api.HealthPassing,
				},
				// 2 Service level ones
				&structs.HealthCheck{
					Node:        fmt.Sprintf("node-%d.example.com", i),
					CheckID:     types.CheckID(fmt.Sprintf("node-%d-svc-%d-1", i, i)),
					Name:        fmt.Sprintf("node-%d-svc-%d-1", i, i),
					Status:      api.HealthPassing,
					ServiceID:   fmt.Sprintf("test-%d", i),
					ServiceName: "test",
				},
				&structs.HealthCheck{
					Node:        fmt.Sprintf("node-%d.example.com", i),
					CheckID:     types.CheckID(fmt.Sprintf("node-%d-svc-%d-2", i, i)),
					Name:        fmt.Sprintf("node-%d-svc-%d-2", i, i),
					Status:      api.HealthPassing,
					ServiceID:   fmt.Sprintf("test-%d", i),
					ServiceName: "test",
				},
			},
		}
		s.EnsureRegistration(uint64(i+1), req)
	}

	// Fetch some service instances
	_, services, err := s.ServiceNodes(nil, "test")
	if err != nil {
		b.Fatalf("failed fetching services: %s", err)
	}
	if len(services) != instanceCount {
		b.Fatalf("failed fetching services: got %d wanted %d", len(services), instanceCount)
	}

	for n := 0; n < b.N; n++ {
		var ws memdb.WatchSet
		if watchSet {
			ws = memdb.NewWatchSet()
		}
		tx := s.db.Txn(false)

		b.StartTimer()

		_, res, err := s.parseCheckServiceNodes(tx, ws, uint64(n), "test", services, nil)

		b.StopTimer()

		if err != nil {
			b.Fatalf("failed parsing: %s", err)
		}
		// Assign to global to prevent compiler optimising it away.
		results = res

		tx.Abort()
	}
}

func BenchmarkCatalogParseCheckServiceNodes10WatchSet(b *testing.B) {
	benchCatalogParseCheckServiceNodes(b, 10, true)
}

func BenchmarkCatalogParseCheckServiceNodes10NoWatchSet(b *testing.B) {
	benchCatalogParseCheckServiceNodes(b, 10, false)
}

func BenchmarkCatalogParseCheckServiceNodes100WatchSet(b *testing.B) {
	benchCatalogParseCheckServiceNodes(b, 100, true)
}

func BenchmarkCatalogParseCheckServiceNodes100NoWatchSet(b *testing.B) {
	benchCatalogParseCheckServiceNodes(b, 100, false)
}

func BenchmarkCatalogParseCheckServiceNodes1000WatchSet(b *testing.B) {
	benchCatalogParseCheckServiceNodes(b, 1000, true)
}

func BenchmarkCatalogParseCheckServiceNodes1000NoWatchSet(b *testing.B) {
	benchCatalogParseCheckServiceNodes(b, 1000, false)
}

func BenchmarkCatalogEnsureRegistration(b *testing.B) {
	s, err := NewStateStore(nil)
	if err != nil {
		b.Fatalf("failed creating state store: %s", err)
	}

	// First create 100 services on 1 node
	for i := 0; i < 100; i++ {
		req := &structs.RegisterRequest{
			Datacenter: "dc1",
			ID:         types.NodeID("11111111-2222-3333-4444-000000000000"),
			Node:       "node.example.com",
			Address:    "10.0.0.1",
			TaggedAddresses: map[string]string{
				"lan": "10.0.0.1",
				"wan": "10.0.0.1",
			},
			Service: &structs.NodeService{
				ID:      fmt.Sprintf("test-%d", i),
				Service: "test",
				Port:    i,
			},
			Checks: structs.HealthChecks{
				// 2 node-level health checks
				&structs.HealthCheck{
					Node:    "node.example.com",
					CheckID: types.CheckID("node-1"),
					Name:    "node-1",
					Status:  api.HealthPassing,
				},
				&structs.HealthCheck{
					Node:    "node.example.com",
					CheckID: types.CheckID("node-2"),
					Name:    "node-2",
					Status:  api.HealthPassing,
				},
				// 2 Service level ones
				&structs.HealthCheck{
					Node:        "node.example.com",
					CheckID:     types.CheckID(fmt.Sprintf("node-svc-%d-1", i)),
					Name:        fmt.Sprintf("node-svc-%d-1", i),
					Status:      api.HealthPassing,
					ServiceID:   fmt.Sprintf("test-%d", i),
					ServiceName: "test",
				},
				&structs.HealthCheck{
					Node:        "node.example.com",
					CheckID:     types.CheckID(fmt.Sprintf("node-svc-%d-2", i)),
					Name:        fmt.Sprintf("node-svc-%d-2", i),
					Status:      api.HealthPassing,
					ServiceID:   fmt.Sprintf("test-%d", i),
					ServiceName: "test",
				},
			},
		}
		s.EnsureRegistration(uint64(i+1), req)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Keep updating the same service on that one node with different meta
		req := &structs.RegisterRequest{
			Datacenter: "dc1",
			ID:         types.NodeID("11111111-2222-3333-4444-000000000000"),
			Node:       "node.example.com",
			Address:    "10.0.0.1",
			TaggedAddresses: map[string]string{
				"lan": "10.0.0.1",
				"wan": "10.0.0.1",
			},
			Service: &structs.NodeService{
				ID:      "test-0",
				Service: "test",
				Port:    1,
				Tags:    []string{fmt.Sprintf("i:%d", i)},
			},
		}
		s.EnsureRegistration(uint64(i+1), req)
	}
}
