package state

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-uuid"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbpeering"
)

func insertTestPeerings(t *testing.T, s *Store) {
	t.Helper()

	tx := s.db.WriteTxn(0)
	defer tx.Abort()

	err := tx.Insert(tablePeering, &pbpeering.Peering{
		Name:        "foo",
		Partition:   structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
		ID:          "9e650110-ac74-4c5a-a6a8-9348b2bed4e9",
		State:       pbpeering.PeeringState_INITIAL,
		CreateIndex: 1,
		ModifyIndex: 1,
	})
	require.NoError(t, err)

	err = tx.Insert(tablePeering, &pbpeering.Peering{
		Name:        "bar",
		Partition:   structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
		ID:          "5ebcff30-5509-4858-8142-a8e580f1863f",
		State:       pbpeering.PeeringState_FAILING,
		CreateIndex: 2,
		ModifyIndex: 2,
	})
	require.NoError(t, err)

	err = tx.Insert(tableIndex, &IndexEntry{
		Key:   tablePeering,
		Value: 2,
	})
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
}

func insertTestPeeringTrustBundles(t *testing.T, s *Store) {
	t.Helper()

	tx := s.db.WriteTxn(0)
	defer tx.Abort()

	err := tx.Insert(tablePeeringTrustBundles, &pbpeering.PeeringTrustBundle{
		TrustDomain: "foo.com",
		PeerName:    "foo",
		Partition:   structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
		RootPEMs:    []string{"foo certificate bundle"},
		CreateIndex: 1,
		ModifyIndex: 1,
	})
	require.NoError(t, err)

	err = tx.Insert(tablePeeringTrustBundles, &pbpeering.PeeringTrustBundle{
		TrustDomain: "bar.com",
		PeerName:    "bar",
		Partition:   structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
		RootPEMs:    []string{"bar certificate bundle"},
		CreateIndex: 2,
		ModifyIndex: 2,
	})
	require.NoError(t, err)

	err = tx.Insert(tableIndex, &IndexEntry{
		Key:   tablePeeringTrustBundles,
		Value: 2,
	})
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
}

func TestStateStore_PeeringReadByID(t *testing.T) {
	s := NewStateStore(nil)
	insertTestPeerings(t, s)

	type testcase struct {
		name   string
		id     string
		expect *pbpeering.Peering
	}
	run := func(t *testing.T, tc testcase) {
		_, peering, err := s.PeeringReadByID(nil, tc.id)
		require.NoError(t, err)
		require.Equal(t, tc.expect, peering)
	}
	tcs := []testcase{
		{
			name: "get foo",
			id:   "9e650110-ac74-4c5a-a6a8-9348b2bed4e9",
			expect: &pbpeering.Peering{
				Name:        "foo",
				Partition:   structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
				ID:          "9e650110-ac74-4c5a-a6a8-9348b2bed4e9",
				State:       pbpeering.PeeringState_INITIAL,
				CreateIndex: 1,
				ModifyIndex: 1,
			},
		},
		{
			name: "get bar",
			id:   "5ebcff30-5509-4858-8142-a8e580f1863f",
			expect: &pbpeering.Peering{
				Name:        "bar",
				Partition:   structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
				ID:          "5ebcff30-5509-4858-8142-a8e580f1863f",
				State:       pbpeering.PeeringState_FAILING,
				CreateIndex: 2,
				ModifyIndex: 2,
			},
		},
		{
			name:   "get non-existent",
			id:     "05f54e2f-7813-4d4d-ba03-534554c88a18",
			expect: nil,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestStateStore_PeeringRead(t *testing.T) {
	s := NewStateStore(nil)
	insertTestPeerings(t, s)

	type testcase struct {
		name   string
		query  Query
		expect *pbpeering.Peering
	}
	run := func(t *testing.T, tc testcase) {
		_, peering, err := s.PeeringRead(nil, tc.query)
		require.NoError(t, err)
		require.Equal(t, tc.expect, peering)
	}
	tcs := []testcase{
		{
			name: "get foo",
			query: Query{
				Value: "foo",
			},
			expect: &pbpeering.Peering{
				Name:        "foo",
				Partition:   structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
				ID:          "9e650110-ac74-4c5a-a6a8-9348b2bed4e9",
				State:       pbpeering.PeeringState_INITIAL,
				CreateIndex: 1,
				ModifyIndex: 1,
			},
		},
		{
			name: "get non-existent baz",
			query: Query{
				Value: "baz",
			},
			expect: nil,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestStore_Peering_Watch(t *testing.T) {
	s := NewStateStore(nil)

	var lastIdx uint64
	lastIdx++

	// set up initial write
	err := s.PeeringWrite(lastIdx, &pbpeering.Peering{
		Name: "foo",
	})
	require.NoError(t, err)

	newWatch := func(t *testing.T, q Query) memdb.WatchSet {
		t.Helper()
		// set up a watch
		ws := memdb.NewWatchSet()

		_, _, err := s.PeeringRead(ws, q)
		require.NoError(t, err)

		return ws
	}

	t.Run("insert fires watch", func(t *testing.T) {
		// watch on non-existent bar
		ws := newWatch(t, Query{Value: "bar"})

		lastIdx++
		err := s.PeeringWrite(lastIdx, &pbpeering.Peering{
			Name: "bar",
		})
		require.NoError(t, err)
		require.True(t, watchFired(ws))

		// should find bar peering
		idx, p, err := s.PeeringRead(ws, Query{Value: "bar"})
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.NotNil(t, p)
	})

	t.Run("update fires watch", func(t *testing.T) {
		// watch on existing foo
		ws := newWatch(t, Query{Value: "foo"})

		// unrelated write shouldn't fire watch
		lastIdx++
		err := s.PeeringWrite(lastIdx, &pbpeering.Peering{
			Name: "bar",
		})
		require.NoError(t, err)
		require.False(t, watchFired(ws))

		// foo write should fire watch
		lastIdx++
		err = s.PeeringWrite(lastIdx, &pbpeering.Peering{
			Name:  "foo",
			State: pbpeering.PeeringState_FAILING,
		})
		require.NoError(t, err)
		require.True(t, watchFired(ws))

		// check foo is updated
		idx, p, err := s.PeeringRead(ws, Query{Value: "foo"})
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.Equal(t, pbpeering.PeeringState_FAILING, p.State)
	})

	t.Run("delete fires watch", func(t *testing.T) {
		// watch on existing foo
		ws := newWatch(t, Query{Value: "foo"})

		// delete on bar shouldn't fire watch
		lastIdx++
		require.NoError(t, s.PeeringWrite(lastIdx, &pbpeering.Peering{Name: "bar"}))
		lastIdx++
		require.NoError(t, s.PeeringDelete(lastIdx, Query{Value: "bar"}))
		require.False(t, watchFired(ws))

		// delete on foo should fire watch
		lastIdx++
		err := s.PeeringDelete(lastIdx, Query{Value: "foo"})
		require.NoError(t, err)
		require.True(t, watchFired(ws))

		// check foo is gone
		idx, p, err := s.PeeringRead(ws, Query{Value: "foo"})
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.Nil(t, p)
	})
}

func TestStore_PeeringList(t *testing.T) {
	s := NewStateStore(nil)
	insertTestPeerings(t, s)

	_, pps, err := s.PeeringList(nil, acl.EnterpriseMeta{})
	require.NoError(t, err)
	expect := []*pbpeering.Peering{
		{
			Name:        "foo",
			Partition:   structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
			ID:          "9e650110-ac74-4c5a-a6a8-9348b2bed4e9",
			State:       pbpeering.PeeringState_INITIAL,
			CreateIndex: 1,
			ModifyIndex: 1,
		},
		{
			Name:        "bar",
			Partition:   structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
			ID:          "5ebcff30-5509-4858-8142-a8e580f1863f",
			State:       pbpeering.PeeringState_FAILING,
			CreateIndex: 2,
			ModifyIndex: 2,
		},
	}
	require.ElementsMatch(t, expect, pps)
}

func TestStore_PeeringList_Watch(t *testing.T) {
	s := NewStateStore(nil)

	var lastIdx uint64
	lastIdx++ // start at 1

	// track number of expected peerings in state store
	var count int

	newWatch := func(t *testing.T, entMeta acl.EnterpriseMeta) memdb.WatchSet {
		t.Helper()
		// set up a watch
		ws := memdb.NewWatchSet()

		_, _, err := s.PeeringList(ws, entMeta)
		require.NoError(t, err)

		return ws
	}

	t.Run("insert fires watch", func(t *testing.T) {
		ws := newWatch(t, acl.EnterpriseMeta{})

		lastIdx++
		// insert a peering
		err := s.PeeringWrite(lastIdx, &pbpeering.Peering{
			Name:      "bar",
			Partition: structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
		})
		require.NoError(t, err)
		count++

		require.True(t, watchFired(ws))

		// should find bar peering
		idx, pp, err := s.PeeringList(ws, acl.EnterpriseMeta{})
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.Len(t, pp, count)
	})

	t.Run("update fires watch", func(t *testing.T) {
		// set up initial write
		lastIdx++
		err := s.PeeringWrite(lastIdx, &pbpeering.Peering{
			Name:      "foo",
			Partition: structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
		})
		require.NoError(t, err)
		count++

		ws := newWatch(t, acl.EnterpriseMeta{})

		// update peering
		lastIdx++
		err = s.PeeringWrite(lastIdx, &pbpeering.Peering{
			Name:      "foo",
			State:     pbpeering.PeeringState_FAILING,
			Partition: structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
		})
		require.NoError(t, err)

		require.True(t, watchFired(ws))

		idx, pp, err := s.PeeringList(ws, acl.EnterpriseMeta{})
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.Len(t, pp, count)
	})

	t.Run("delete fires watch", func(t *testing.T) {
		// set up initial write
		lastIdx++
		err := s.PeeringWrite(lastIdx, &pbpeering.Peering{
			Name:      "baz",
			Partition: structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
		})
		require.NoError(t, err)
		count++

		ws := newWatch(t, acl.EnterpriseMeta{})

		// delete peering
		lastIdx++
		err = s.PeeringDelete(lastIdx, Query{Value: "baz"})
		require.NoError(t, err)
		count--

		require.True(t, watchFired(ws))

		idx, pp, err := s.PeeringList(ws, acl.EnterpriseMeta{})
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.Len(t, pp, count)
	})
}

func TestStore_PeeringWrite(t *testing.T) {
	s := NewStateStore(nil)
	insertTestPeerings(t, s)
	type testcase struct {
		name  string
		input *pbpeering.Peering
	}
	run := func(t *testing.T, tc testcase) {
		require.NoError(t, s.PeeringWrite(10, tc.input))

		q := Query{
			Value:          tc.input.Name,
			EnterpriseMeta: *structs.NodeEnterpriseMetaInPartition(tc.input.Partition),
		}
		_, p, err := s.PeeringRead(nil, q)
		require.NoError(t, err)
		require.NotNil(t, p)
		if tc.input.State == 0 {
			require.Equal(t, pbpeering.PeeringState_INITIAL, p.State)
		}
		require.Equal(t, tc.input.Name, p.Name)
	}
	tcs := []testcase{
		{
			name: "create baz",
			input: &pbpeering.Peering{
				Name:      "baz",
				Partition: structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
			},
		},
		{
			name: "update foo",
			input: &pbpeering.Peering{
				Name:      "foo",
				State:     pbpeering.PeeringState_FAILING,
				Partition: structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestStore_PeeringWrite_GenerateUUID(t *testing.T) {
	rand.Seed(1)

	s := NewStateStore(nil)

	entMeta := structs.NodeEnterpriseMetaInDefaultPartition()
	partition := entMeta.PartitionOrDefault()

	for i := 1; i < 11; i++ {
		require.NoError(t, s.PeeringWrite(uint64(i), &pbpeering.Peering{
			Name:      fmt.Sprintf("peering-%d", i),
			Partition: partition,
		}))
	}

	idx, peerings, err := s.PeeringList(nil, *entMeta)
	require.NoError(t, err)
	require.Equal(t, uint64(10), idx)
	require.Len(t, peerings, 10)

	// Ensure that all assigned UUIDs are unique.
	uniq := make(map[string]struct{})
	for _, p := range peerings {
		uniq[p.ID] = struct{}{}
	}
	require.Len(t, uniq, 10)

	// Ensure that the ID of an existing peering cannot be overwritten.
	updated := &pbpeering.Peering{
		Name:      peerings[0].Name,
		Partition: peerings[0].Partition,
	}

	// Attempt to overwrite ID.
	updated.ID, err = uuid.GenerateUUID()
	require.NoError(t, err)
	require.NoError(t, s.PeeringWrite(11, updated))

	q := Query{
		Value:          updated.Name,
		EnterpriseMeta: *entMeta,
	}
	idx, got, err := s.PeeringRead(nil, q)
	require.NoError(t, err)
	require.Equal(t, uint64(11), idx)
	require.Equal(t, peerings[0].ID, got.ID)
}

func TestStore_PeeringDelete(t *testing.T) {
	s := NewStateStore(nil)
	insertTestPeerings(t, s)

	q := Query{Value: "foo"}

	require.NoError(t, s.PeeringDelete(10, q))

	_, p, err := s.PeeringRead(nil, q)
	require.NoError(t, err)
	require.Nil(t, p)
}

func TestStore_PeeringTerminateByID(t *testing.T) {
	s := NewStateStore(nil)
	insertTestPeerings(t, s)

	// id corresponding to default/foo
	id := "9e650110-ac74-4c5a-a6a8-9348b2bed4e9"

	require.NoError(t, s.PeeringTerminateByID(10, id))

	_, p, err := s.PeeringReadByID(nil, id)
	require.NoError(t, err)
	require.Equal(t, pbpeering.PeeringState_TERMINATED, p.State)
}

func TestStateStore_PeeringTrustBundleRead(t *testing.T) {
	s := NewStateStore(nil)
	insertTestPeeringTrustBundles(t, s)

	type testcase struct {
		name   string
		query  Query
		expect *pbpeering.PeeringTrustBundle
	}
	run := func(t *testing.T, tc testcase) {
		_, ptb, err := s.PeeringTrustBundleRead(nil, tc.query)
		require.NoError(t, err)
		require.Equal(t, tc.expect, ptb)
	}

	entMeta := structs.NodeEnterpriseMetaInDefaultPartition()

	tcs := []testcase{
		{
			name: "get foo",
			query: Query{
				Value:          "foo",
				EnterpriseMeta: *entMeta,
			},
			expect: &pbpeering.PeeringTrustBundle{
				TrustDomain: "foo.com",
				PeerName:    "foo",
				Partition:   entMeta.PartitionOrEmpty(),
				RootPEMs:    []string{"foo certificate bundle"},
				CreateIndex: 1,
				ModifyIndex: 1,
			},
		},
		{
			name: "get non-existent baz",
			query: Query{
				Value: "baz",
			},
			expect: nil,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestStore_PeeringTrustBundleWrite(t *testing.T) {
	s := NewStateStore(nil)
	insertTestPeeringTrustBundles(t, s)
	type testcase struct {
		name  string
		input *pbpeering.PeeringTrustBundle
	}
	run := func(t *testing.T, tc testcase) {
		require.NoError(t, s.PeeringTrustBundleWrite(10, tc.input))

		q := Query{
			Value:          tc.input.PeerName,
			EnterpriseMeta: *structs.NodeEnterpriseMetaInPartition(tc.input.Partition),
		}
		_, ptb, err := s.PeeringTrustBundleRead(nil, q)
		require.NoError(t, err)
		require.NotNil(t, ptb)
		require.Equal(t, tc.input.TrustDomain, ptb.TrustDomain)
		require.Equal(t, tc.input.PeerName, ptb.PeerName)
	}
	tcs := []testcase{
		{
			name: "create baz",
			input: &pbpeering.PeeringTrustBundle{
				TrustDomain: "baz.com",
				PeerName:    "baz",
				Partition:   structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
			},
		},
		{
			name: "update foo",
			input: &pbpeering.PeeringTrustBundle{
				TrustDomain: "foo-updated.com",
				PeerName:    "foo",
				Partition:   structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestStore_PeeringTrustBundleDelete(t *testing.T) {
	s := NewStateStore(nil)
	insertTestPeeringTrustBundles(t, s)

	q := Query{Value: "foo"}

	require.NoError(t, s.PeeringTrustBundleDelete(10, q))

	_, ptb, err := s.PeeringRead(nil, q)
	require.NoError(t, err)
	require.Nil(t, ptb)
}

func TestStateStore_ExportedServicesForPeer(t *testing.T) {
	s := NewStateStore(nil)

	var lastIdx uint64

	lastIdx++
	err := s.PeeringWrite(lastIdx, &pbpeering.Peering{
		Name: "my-peering",
	})
	require.NoError(t, err)

	q := Query{Value: "my-peering"}
	_, p, err := s.PeeringRead(nil, q)
	require.NoError(t, err)
	require.NotNil(t, p)

	id := p.ID

	ws := memdb.NewWatchSet()

	runStep(t, "no exported services", func(t *testing.T) {
		idx, exported, err := s.ExportedServicesForPeer(ws, id)
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.Empty(t, exported)
	})

	runStep(t, "config entry with exact service names", func(t *testing.T) {
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
					Name: "redis",
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
		err = s.EnsureConfigEntry(lastIdx, entry)
		require.NoError(t, err)

		require.True(t, watchFired(ws))
		ws = memdb.NewWatchSet()

		expect := []structs.ServiceName{
			{
				Name:           "mysql",
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			},
			{
				Name:           "redis",
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			},
		}
		idx, got, err := s.ExportedServicesForPeer(ws, id)
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.ElementsMatch(t, expect, got)
	})

	runStep(t, "config entry with wildcard service name picks up existing service", func(t *testing.T) {
		lastIdx++
		require.NoError(t, s.EnsureNode(lastIdx, &structs.Node{Node: "foo", Address: "127.0.0.1"}))

		lastIdx++
		require.NoError(t, s.EnsureService(lastIdx, "foo", &structs.NodeService{ID: "billing", Service: "billing", Port: 5000}))

		entry := &structs.ExportedServicesConfigEntry{
			Name: "default",
			Services: []structs.ExportedService{
				{
					Name: "*",
					Consumers: []structs.ServiceConsumer{
						{
							PeerName: "my-peering",
						},
					},
				},
			},
		}
		lastIdx++
		err = s.EnsureConfigEntry(lastIdx, entry)
		require.NoError(t, err)

		require.True(t, watchFired(ws))
		ws = memdb.NewWatchSet()

		expect := []structs.ServiceName{
			{
				Name:           "billing",
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			},
		}
		idx, got, err := s.ExportedServicesForPeer(ws, id)
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.Equal(t, expect, got)
	})

	runStep(t, "config entry with wildcard service names picks up new registrations", func(t *testing.T) {
		lastIdx++
		require.NoError(t, s.EnsureService(lastIdx, "foo", &structs.NodeService{ID: "payments", Service: "payments", Port: 5000}))

		lastIdx++
		proxy := structs.NodeService{
			Kind:    structs.ServiceKindConnectProxy,
			ID:      "payments-proxy",
			Service: "payments-proxy",
			Port:    5000,
		}
		require.NoError(t, s.EnsureService(lastIdx, "foo", &proxy))

		require.True(t, watchFired(ws))
		ws = memdb.NewWatchSet()

		expect := []structs.ServiceName{
			{
				Name:           "billing",
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			},
			{
				Name:           "payments",
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			},
			{
				Name:           "payments-proxy",
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			},
		}
		idx, got, err := s.ExportedServicesForPeer(ws, id)
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.ElementsMatch(t, expect, got)
	})

	runStep(t, "config entry with wildcard service names picks up service deletions", func(t *testing.T) {
		lastIdx++
		require.NoError(t, s.DeleteService(lastIdx, "foo", "billing", nil, ""))

		require.True(t, watchFired(ws))
		ws = memdb.NewWatchSet()

		expect := []structs.ServiceName{
			{
				Name:           "payments",
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			},
			{
				Name:           "payments-proxy",
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			},
		}
		idx, got, err := s.ExportedServicesForPeer(ws, id)
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.ElementsMatch(t, expect, got)
	})

	runStep(t, "deleting the config entry clears exported services", func(t *testing.T) {
		require.NoError(t, s.DeleteConfigEntry(lastIdx, structs.ExportedServices, "default", structs.DefaultEnterpriseMetaInDefaultPartition()))
		idx, exported, err := s.ExportedServicesForPeer(ws, id)
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.Empty(t, exported)
	})
}

func TestStateStore_PeeringsForService(t *testing.T) {
	type testCase struct {
		name      string
		services  []structs.ServiceName
		peerings  []*pbpeering.Peering
		entries   []*structs.ExportedServicesConfigEntry
		query     []string
		expect    [][]*pbpeering.Peering
		expectIdx uint64
	}

	run := func(t *testing.T, tc testCase) {
		s := testStateStore(t)

		var lastIdx uint64
		// Create peerings
		for _, peering := range tc.peerings {
			lastIdx++
			require.NoError(t, s.PeeringWrite(lastIdx, peering))

			// make sure it got created
			q := Query{Value: peering.Name}
			_, p, err := s.PeeringRead(nil, q)
			require.NoError(t, err)
			require.NotNil(t, p)
		}

		// Create a Nodes for services
		svcNode := &structs.Node{Node: "foo", Address: "127.0.0.1"}
		lastIdx++
		require.NoError(t, s.EnsureNode(lastIdx, svcNode))

		// Create the test services
		for _, svc := range tc.services {
			lastIdx++
			require.NoError(t, s.EnsureService(lastIdx, svcNode.Node, &structs.NodeService{
				ID:      svc.Name,
				Service: svc.Name,
				Port:    8080,
			}))
		}

		// Write the config entries.
		for _, entry := range tc.entries {
			lastIdx++
			require.NoError(t, s.EnsureConfigEntry(lastIdx, entry))
		}

		// Query for peers.
		for resultIdx, q := range tc.query {
			tx := s.db.ReadTxn()
			defer tx.Abort()
			idx, peers, err := s.PeeringsForService(nil, q, *acl.DefaultEnterpriseMeta())
			require.NoError(t, err)
			require.Equal(t, tc.expectIdx, idx)

			// Verify the result, ignoring generated fields
			require.Len(t, peers, len(tc.expect[resultIdx]))
			for _, got := range peers {
				got.ID = ""
				got.ModifyIndex = 0
				got.CreateIndex = 0
			}
			require.ElementsMatch(t, tc.expect[resultIdx], peers)
		}
	}

	cases := []testCase{
		{
			name: "no exported services",
			services: []structs.ServiceName{
				{Name: "foo"},
			},
			peerings: []*pbpeering.Peering{},
			entries:  []*structs.ExportedServicesConfigEntry{},
			query:    []string{"foo"},
			expect:   [][]*pbpeering.Peering{{}},
		},
		{
			name: "service does not exist",
			services: []structs.ServiceName{
				{Name: "foo"},
			},
			peerings:  []*pbpeering.Peering{},
			entries:   []*structs.ExportedServicesConfigEntry{},
			query:     []string{"bar"},
			expect:    [][]*pbpeering.Peering{{}},
			expectIdx: uint64(2), // catalog services max index
		},
		{
			name: "config entry with exact service name",
			services: []structs.ServiceName{
				{Name: "foo"},
				{Name: "bar"},
			},
			peerings: []*pbpeering.Peering{
				{Name: "peer1", State: pbpeering.PeeringState_INITIAL},
				{Name: "peer2", State: pbpeering.PeeringState_INITIAL},
			},
			entries: []*structs.ExportedServicesConfigEntry{
				{
					Name: "ce1",
					Services: []structs.ExportedService{
						{
							Name: "foo",
							Consumers: []structs.ServiceConsumer{
								{
									PeerName: "peer1",
								},
							},
						},
						{
							Name: "bar",
							Consumers: []structs.ServiceConsumer{
								{
									PeerName: "peer2",
								},
							},
						},
					},
				},
			},
			query: []string{"foo", "bar"},
			expect: [][]*pbpeering.Peering{
				{
					{Name: "peer1", State: pbpeering.PeeringState_INITIAL},
				},
				{
					{Name: "peer2", State: pbpeering.PeeringState_INITIAL},
				},
			},
			expectIdx: uint64(6), // config	entries max index
		},
		{
			name: "config entry with wildcard service name",
			services: []structs.ServiceName{
				{Name: "foo"},
				{Name: "bar"},
			},
			peerings: []*pbpeering.Peering{
				{Name: "peer1", State: pbpeering.PeeringState_INITIAL},
				{Name: "peer2", State: pbpeering.PeeringState_INITIAL},
				{Name: "peer3", State: pbpeering.PeeringState_INITIAL},
			},
			entries: []*structs.ExportedServicesConfigEntry{
				{
					Name: "ce1",
					Services: []structs.ExportedService{
						{
							Name: "*",
							Consumers: []structs.ServiceConsumer{
								{
									PeerName: "peer1",
								},
								{
									PeerName: "peer2",
								},
							},
						},
						{
							Name: "bar",
							Consumers: []structs.ServiceConsumer{
								{
									PeerName: "peer3",
								},
							},
						},
					},
				},
			},
			query: []string{"foo", "bar"},
			expect: [][]*pbpeering.Peering{
				{
					{Name: "peer1", State: pbpeering.PeeringState_INITIAL},
					{Name: "peer2", State: pbpeering.PeeringState_INITIAL},
				},
				{
					{Name: "peer1", State: pbpeering.PeeringState_INITIAL},
					{Name: "peer2", State: pbpeering.PeeringState_INITIAL},
					{Name: "peer3", State: pbpeering.PeeringState_INITIAL},
				},
			},
			expectIdx: uint64(7),
		},
	}

	for _, tc := range cases {
		runStep(t, tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}
