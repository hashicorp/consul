package peering

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbpeering"
	"github.com/hashicorp/consul/proto/pbservice"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

type testSubscriptionBackend struct {
	state.EventPublisher
	store *state.Store
}

func (b *testSubscriptionBackend) Store() Store {
	return b.store
}

func TestSubscriptionManager_RegisterDeregister(t *testing.T) {
	publisher := stream.NewEventPublisher(10 * time.Second)
	store := newStateStore(t, publisher)

	backend := testSubscriptionBackend{
		EventPublisher: publisher,
		store:          store,
	}

	ctx := context.Background()
	mgr := newSubscriptionManager(ctx, hclog.New(nil), &backend)

	// Create a peering
	var lastIdx uint64 = 1
	err := store.PeeringWrite(lastIdx, &pbpeering.Peering{
		Name: "my-peering",
	})
	require.NoError(t, err)

	_, p, err := store.PeeringRead(nil, state.Query{Value: "my-peering"})
	require.NoError(t, err)
	require.NotNil(t, p)

	id := p.ID

	subCh := mgr.subscribe(ctx, id)

	entry := &structs.ExportedServicesConfigEntry{
		Name: "default",
		Services: []structs.ExportedService{
			{
				Name: "mysql",
				Consumers: []structs.ServiceConsumer{
					{
						PeerName: "my-peering",
					},
				},
			},
			{
				Name: "mongo",
				Consumers: []structs.ServiceConsumer{
					{
						PeerName: "my-other-peering",
					},
				},
			},
		},
	}
	lastIdx++
	err = store.EnsureConfigEntry(lastIdx, entry)
	require.NoError(t, err)

	mysql1 := &structs.CheckServiceNode{
		Node:    &structs.Node{Node: "foo", Address: "10.0.0.1"},
		Service: &structs.NodeService{ID: "mysql-1", Service: "mysql", Port: 5000},
		Checks: structs.HealthChecks{
			&structs.HealthCheck{CheckID: "mysql-check", ServiceID: "mysql-1", Node: "foo"},
		},
	}

	testutil.RunStep(t, "registering exported service instance yields update", func(t *testing.T) {

		lastIdx++
		require.NoError(t, store.EnsureNode(lastIdx, mysql1.Node))

		lastIdx++
		require.NoError(t, store.EnsureService(lastIdx, "foo", mysql1.Service))

		lastIdx++
		require.NoError(t, store.EnsureCheck(lastIdx, mysql1.Checks[0]))

		// Receive in a retry loop so that eventually we converge onto the expected CheckServiceNode.
		retry.Run(t, func(r *retry.R) {
			select {
			case update := <-subCh:
				nodes, ok := update.Result.(*pbservice.IndexedCheckServiceNodes)
				require.True(r, ok)
				require.Equal(r, uint64(5), nodes.Index)

				require.Len(r, nodes.Nodes, 1)
				require.Equal(r, "foo", nodes.Nodes[0].Node.Node)
				require.Equal(r, "mysql-1", nodes.Nodes[0].Service.ID)

				require.Len(r, nodes.Nodes[0].Checks, 1)
				require.Equal(r, "mysql-check", nodes.Nodes[0].Checks[0].CheckID)

			default:
				r.Fatalf("invalid update")
			}
		})
	})

	mysql2 := &structs.CheckServiceNode{
		Node:    &structs.Node{Node: "bar", Address: "10.0.0.2"},
		Service: &structs.NodeService{ID: "mysql-2", Service: "mysql", Port: 5000},
		Checks: structs.HealthChecks{
			&structs.HealthCheck{CheckID: "mysql-2-check", ServiceID: "mysql-2", Node: "bar"},
		},
	}

	testutil.RunStep(t, "additional instances are returned when registered", func(t *testing.T) {
		lastIdx++
		require.NoError(t, store.EnsureNode(lastIdx, mysql2.Node))

		lastIdx++
		require.NoError(t, store.EnsureService(lastIdx, "bar", mysql2.Service))

		lastIdx++
		require.NoError(t, store.EnsureCheck(lastIdx, mysql2.Checks[0]))

		retry.Run(t, func(r *retry.R) {
			select {
			case update := <-subCh:
				nodes, ok := update.Result.(*pbservice.IndexedCheckServiceNodes)
				require.True(r, ok)
				require.Equal(r, uint64(8), nodes.Index)

				require.Len(r, nodes.Nodes, 2)
				require.Equal(r, "bar", nodes.Nodes[0].Node.Node)
				require.Equal(r, "mysql-2", nodes.Nodes[0].Service.ID)

				require.Len(r, nodes.Nodes[0].Checks, 1)
				require.Equal(r, "mysql-2-check", nodes.Nodes[0].Checks[0].CheckID)

				require.Equal(r, "foo", nodes.Nodes[1].Node.Node)
				require.Equal(r, "mysql-1", nodes.Nodes[1].Service.ID)

				require.Len(r, nodes.Nodes[1].Checks, 1)
				require.Equal(r, "mysql-check", nodes.Nodes[1].Checks[0].CheckID)

			default:
				r.Fatalf("invalid update")
			}
		})
	})

	testutil.RunStep(t, "no updates are received for services not exported to my-peering", func(t *testing.T) {
		mongo := &structs.CheckServiceNode{
			Node:    &structs.Node{Node: "zip", Address: "10.0.0.3"},
			Service: &structs.NodeService{ID: "mongo", Service: "mongo", Port: 5000},
			Checks: structs.HealthChecks{
				&structs.HealthCheck{CheckID: "mongo-check", ServiceID: "mongo", Node: "zip"},
			},
		}

		lastIdx++
		require.NoError(t, store.EnsureNode(lastIdx, mongo.Node))

		lastIdx++
		require.NoError(t, store.EnsureService(lastIdx, "zip", mongo.Service))

		lastIdx++
		require.NoError(t, store.EnsureCheck(lastIdx, mongo.Checks[0]))

		// Receive from subCh times out. The retry in the last step already consumed all the mysql events.
		select {
		case update := <-subCh:
			nodes, ok := update.Result.(*pbservice.IndexedCheckServiceNodes)

			if ok && len(nodes.Nodes) > 0 && nodes.Nodes[0].Node.Node == "zip" {
				t.Fatalf("received update for mongo node zip")
			}

		case <-time.After(100 * time.Millisecond):
			// Expect this to fire
		}
	})

	testutil.RunStep(t, "deregister an instance and it gets removed from the output", func(t *testing.T) {
		lastIdx++
		require.NoError(t, store.DeleteService(lastIdx, "foo", mysql1.Service.ID, nil, ""))

		select {
		case update := <-subCh:
			nodes, ok := update.Result.(*pbservice.IndexedCheckServiceNodes)
			require.True(t, ok)
			require.Equal(t, uint64(12), nodes.Index)

			require.Len(t, nodes.Nodes, 1)
			require.Equal(t, "bar", nodes.Nodes[0].Node.Node)
			require.Equal(t, "mysql-2", nodes.Nodes[0].Service.ID)

			require.Len(t, nodes.Nodes[0].Checks, 1)
			require.Equal(t, "mysql-2-check", nodes.Nodes[0].Checks[0].CheckID)

		case <-time.After(100 * time.Millisecond):
			t.Fatalf("timed out waiting for update")
		}
	})

	testutil.RunStep(t, "deregister the last instance and the output is empty", func(t *testing.T) {
		lastIdx++
		require.NoError(t, store.DeleteService(lastIdx, "bar", mysql2.Service.ID, nil, ""))

		select {
		case update := <-subCh:
			nodes, ok := update.Result.(*pbservice.IndexedCheckServiceNodes)
			require.True(t, ok)
			require.Equal(t, uint64(13), nodes.Index)
			require.Len(t, nodes.Nodes, 0)

		case <-time.After(100 * time.Millisecond):
			t.Fatalf("timed out waiting for update")
		}
	})
}

func TestSubscriptionManager_InitialSnapshot(t *testing.T) {
	publisher := stream.NewEventPublisher(10 * time.Second)
	store := newStateStore(t, publisher)

	backend := testSubscriptionBackend{
		EventPublisher: publisher,
		store:          store,
	}

	ctx := context.Background()
	mgr := newSubscriptionManager(ctx, hclog.New(nil), &backend)

	// Create a peering
	var lastIdx uint64 = 1
	err := store.PeeringWrite(lastIdx, &pbpeering.Peering{
		Name: "my-peering",
	})
	require.NoError(t, err)

	_, p, err := store.PeeringRead(nil, state.Query{Value: "my-peering"})
	require.NoError(t, err)
	require.NotNil(t, p)

	id := p.ID

	subCh := mgr.subscribe(ctx, id)

	// Register two services that are not yet exported
	mysql := &structs.CheckServiceNode{
		Node:    &structs.Node{Node: "foo", Address: "10.0.0.1"},
		Service: &structs.NodeService{ID: "mysql-1", Service: "mysql", Port: 5000},
	}

	lastIdx++
	require.NoError(t, store.EnsureNode(lastIdx, mysql.Node))

	lastIdx++
	require.NoError(t, store.EnsureService(lastIdx, "foo", mysql.Service))

	mongo := &structs.CheckServiceNode{
		Node:    &structs.Node{Node: "zip", Address: "10.0.0.3"},
		Service: &structs.NodeService{ID: "mongo-1", Service: "mongo", Port: 5000},
	}

	lastIdx++
	require.NoError(t, store.EnsureNode(lastIdx, mongo.Node))

	lastIdx++
	require.NoError(t, store.EnsureService(lastIdx, "zip", mongo.Service))

	// No updates should be received, because neither service is exported.
	select {
	case update := <-subCh:
		nodes, ok := update.Result.(*pbservice.IndexedCheckServiceNodes)

		if ok && len(nodes.Nodes) > 0 {
			t.Fatalf("received unexpected update")
		}

	case <-time.After(100 * time.Millisecond):
		// Expect this to fire
	}

	testutil.RunStep(t, "exporting the two services yields an update for both", func(t *testing.T) {
		entry := &structs.ExportedServicesConfigEntry{
			Name: "default",
			Services: []structs.ExportedService{
				{
					Name: "mysql",
					Consumers: []structs.ServiceConsumer{
						{
							PeerName: "my-peering",
						},
					},
				},
				{
					Name: "mongo",
					Consumers: []structs.ServiceConsumer{
						{
							PeerName: "my-peering",
						},
					},
				},
			},
		}
		lastIdx++
		err = store.EnsureConfigEntry(lastIdx, entry)
		require.NoError(t, err)

		var (
			sawMySQL bool
			sawMongo bool
		)

		retry.Run(t, func(r *retry.R) {
			select {
			case update := <-subCh:
				nodes, ok := update.Result.(*pbservice.IndexedCheckServiceNodes)
				require.True(r, ok)
				require.Len(r, nodes.Nodes, 1)

				switch nodes.Nodes[0].Service.Service {
				case "mongo":
					sawMongo = true
				case "mysql":
					sawMySQL = true
				}
				if !sawMySQL || !sawMongo {
					r.Fatalf("missing an update")
				}
			default:
				r.Fatalf("invalid update")
			}
		})
	})
}

func newStateStore(t *testing.T, publisher *stream.EventPublisher) *state.Store {
	gc, err := state.NewTombstoneGC(time.Second, time.Millisecond)
	require.NoError(t, err)

	store := state.NewStateStoreWithEventPublisher(gc, publisher)
	require.NoError(t, publisher.RegisterHandler(state.EventTopicServiceHealth, store.ServiceHealthSnapshot))
	require.NoError(t, publisher.RegisterHandler(state.EventTopicServiceHealthConnect, store.ServiceHealthSnapshot))
	go publisher.Run(context.Background())

	return store
}
