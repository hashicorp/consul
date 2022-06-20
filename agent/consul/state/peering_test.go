package state

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-uuid"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbpeering"
	"github.com/hashicorp/consul/proto/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
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
			Name:      "foo",
			DeletedAt: structs.TimeToProto(time.Now()),
		})
		require.NoError(t, err)
		require.True(t, watchFired(ws))

		// check foo is updated
		idx, p, err := s.PeeringRead(ws, Query{Value: "foo"})
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.False(t, p.IsActive())
	})

	t.Run("delete fires watch", func(t *testing.T) {
		// watch on existing foo
		ws := newWatch(t, Query{Value: "bar"})

		lastIdx++
		require.NoError(t, s.PeeringDelete(lastIdx, Query{Value: "foo"}))
		require.False(t, watchFired(ws))

		// mark for deletion before actually deleting
		lastIdx++
		err := s.PeeringWrite(lastIdx, &pbpeering.Peering{
			Name:      "bar",
			DeletedAt: structs.TimeToProto(time.Now()),
		})
		require.NoError(t, err)
		require.True(t, watchFired(ws))

		ws = newWatch(t, Query{Value: "bar"})

		// delete on bar should fire watch
		lastIdx++
		err = s.PeeringDelete(lastIdx, Query{Value: "bar"})
		require.NoError(t, err)
		require.True(t, watchFired(ws))

		// check bar is gone
		idx, p, err := s.PeeringRead(ws, Query{Value: "bar"})
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

	testutil.RunStep(t, "insert fires watch", func(t *testing.T) {
		ws := newWatch(t, acl.EnterpriseMeta{})

		lastIdx++
		// insert a peering
		err := s.PeeringWrite(lastIdx, &pbpeering.Peering{
			Name:      "foo",
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

	testutil.RunStep(t, "update fires watch", func(t *testing.T) {
		ws := newWatch(t, acl.EnterpriseMeta{})

		// update peering
		lastIdx++
		require.NoError(t, s.PeeringWrite(lastIdx, &pbpeering.Peering{
			Name:      "foo",
			DeletedAt: structs.TimeToProto(time.Now()),
			Partition: structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
		}))
		require.True(t, watchFired(ws))

		idx, pp, err := s.PeeringList(ws, acl.EnterpriseMeta{})
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.Len(t, pp, count)
	})

	testutil.RunStep(t, "delete fires watch", func(t *testing.T) {
		ws := newWatch(t, acl.EnterpriseMeta{})

		// delete peering
		lastIdx++
		err := s.PeeringDelete(lastIdx, Query{Value: "foo"})
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
	// Note that all test cases in this test share a state store and must be run sequentially.
	// Each case depends on the previous.
	s := NewStateStore(nil)

	type testcase struct {
		name      string
		input     *pbpeering.Peering
		expectErr string
	}
	run := func(t *testing.T, tc testcase) {
		err := s.PeeringWrite(10, tc.input)
		if tc.expectErr != "" {
			testutil.RequireErrorContains(t, err, tc.expectErr)
			return
		}
		require.NoError(t, err)

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
			name: "update baz",
			input: &pbpeering.Peering{
				Name:      "baz",
				State:     pbpeering.PeeringState_FAILING,
				Partition: structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
			},
		},
		{
			name: "mark baz for deletion",
			input: &pbpeering.Peering{
				Name:      "baz",
				State:     pbpeering.PeeringState_TERMINATED,
				DeletedAt: structs.TimeToProto(time.Now()),
				Partition: structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
			},
		},
		{
			name: "cannot update peering marked for deletion",
			input: &pbpeering.Peering{
				Name: "baz",
				// Attempt to add metadata
				Meta: map[string]string{
					"source": "kubernetes",
				},
				Partition: structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
			},
			expectErr: "cannot write to peering that is marked for deletion",
		},
		{
			name: "cannot create peering marked for deletion",
			input: &pbpeering.Peering{
				Name:      "foo",
				DeletedAt: structs.TimeToProto(time.Now()),
				Partition: structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
			},
			expectErr: "cannot create a new peering marked for deletion",
		},
	}
	for _, tc := range tcs {
		testutil.RunStep(t, tc.name, func(t *testing.T) {
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

	testutil.RunStep(t, "cannot delete without marking for deletion", func(t *testing.T) {
		q := Query{Value: "foo"}
		err := s.PeeringDelete(10, q)
		testutil.RequireErrorContains(t, err, "cannot delete a peering without first marking for deletion")
	})

	testutil.RunStep(t, "can delete after marking for deletion", func(t *testing.T) {
		require.NoError(t, s.PeeringWrite(11, &pbpeering.Peering{
			Name:      "foo",
			DeletedAt: structs.TimeToProto(time.Now()),
		}))

		q := Query{Value: "foo"}
		require.NoError(t, s.PeeringDelete(12, q))

		_, p, err := s.PeeringRead(nil, q)
		require.NoError(t, err)
		require.Nil(t, p)
	})
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

func TestStateStore_PeeringTrustBundleList(t *testing.T) {
	s := NewStateStore(nil)
	insertTestPeeringTrustBundles(t, s)

	type testcase struct {
		name    string
		entMeta acl.EnterpriseMeta
		expect  []*pbpeering.PeeringTrustBundle
	}

	entMeta := structs.NodeEnterpriseMetaInDefaultPartition()

	expect := []*pbpeering.PeeringTrustBundle{
		{
			TrustDomain: "bar.com",
			PeerName:    "bar",
			Partition:   entMeta.PartitionOrEmpty(),
			RootPEMs:    []string{"bar certificate bundle"},
			CreateIndex: 2,
			ModifyIndex: 2,
		},
		{
			TrustDomain: "foo.com",
			PeerName:    "foo",
			Partition:   entMeta.PartitionOrEmpty(),
			RootPEMs:    []string{"foo certificate bundle"},
			CreateIndex: 1,
			ModifyIndex: 1,
		},
	}

	_, bundles, err := s.PeeringTrustBundleList(nil, *entMeta)
	require.NoError(t, err)
	prototest.AssertDeepEqual(t, expect, bundles)
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
	require.NoError(t, s.PeeringWrite(lastIdx, &pbpeering.Peering{
		Name: "my-peering",
	}))

	_, p, err := s.PeeringRead(nil, Query{
		Value: "my-peering",
	})
	require.NoError(t, err)
	require.NotNil(t, p)

	id := p.ID

	defaultEntMeta := structs.DefaultEnterpriseMetaInDefaultPartition()

	newSN := func(name string) structs.ServiceName {
		return structs.NewServiceName(name, defaultEntMeta)
	}

	ws := memdb.NewWatchSet()

	ensureConfigEntry := func(t *testing.T, entry structs.ConfigEntry) {
		t.Helper()
		require.NoError(t, entry.Normalize())
		require.NoError(t, entry.Validate())

		lastIdx++
		require.NoError(t, s.EnsureConfigEntry(lastIdx, entry))
	}

	testutil.RunStep(t, "no exported services", func(t *testing.T) {
		expect := &structs.ExportedServiceList{}

		idx, got, err := s.ExportedServicesForPeer(ws, id)
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.Equal(t, expect, got)
	})

	testutil.RunStep(t, "config entry with exact service names", func(t *testing.T) {
		entry := &structs.ExportedServicesConfigEntry{
			Name: "default",
			Services: []structs.ExportedService{
				{
					Name: "mysql",
					Consumers: []structs.ServiceConsumer{
						{PeerName: "my-peering"},
					},
				},
				{
					Name: "redis",
					Consumers: []structs.ServiceConsumer{
						{PeerName: "my-peering"},
					},
				},
				{
					Name: "mongo",
					Consumers: []structs.ServiceConsumer{
						{PeerName: "my-other-peering"},
					},
				},
			},
		}
		ensureConfigEntry(t, entry)

		require.True(t, watchFired(ws))
		ws = memdb.NewWatchSet()

		expect := &structs.ExportedServiceList{
			Services: []structs.ServiceName{
				{
					Name:           "mysql",
					EnterpriseMeta: *defaultEntMeta,
				},
				{
					Name:           "redis",
					EnterpriseMeta: *defaultEntMeta,
				},
			},
			ConnectProtocol: map[structs.ServiceName]string{
				newSN("mysql"): "tcp",
				newSN("redis"): "tcp",
			},
		}

		idx, got, err := s.ExportedServicesForPeer(ws, id)
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.Equal(t, expect, got)
	})

	testutil.RunStep(t, "config entry with wildcard service name picks up existing service", func(t *testing.T) {
		lastIdx++
		require.NoError(t, s.EnsureNode(lastIdx, &structs.Node{
			Node: "foo", Address: "127.0.0.1",
		}))

		lastIdx++
		require.NoError(t, s.EnsureService(lastIdx, "foo", &structs.NodeService{
			ID: "billing", Service: "billing", Port: 5000,
		}))

		entry := &structs.ExportedServicesConfigEntry{
			Name: "default",
			Services: []structs.ExportedService{
				{
					Name: "*",
					Consumers: []structs.ServiceConsumer{
						{PeerName: "my-peering"},
					},
				},
			},
		}
		ensureConfigEntry(t, entry)

		require.True(t, watchFired(ws))
		ws = memdb.NewWatchSet()

		expect := &structs.ExportedServiceList{
			Services: []structs.ServiceName{
				{
					Name:           "billing",
					EnterpriseMeta: *defaultEntMeta,
				},
			},
			ConnectProtocol: map[structs.ServiceName]string{
				newSN("billing"): "tcp",
			},
		}
		idx, got, err := s.ExportedServicesForPeer(ws, id)
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.Equal(t, expect, got)
	})

	testutil.RunStep(t, "config entry with wildcard service names picks up new registrations", func(t *testing.T) {
		lastIdx++
		require.NoError(t, s.EnsureService(lastIdx, "foo", &structs.NodeService{
			ID: "payments", Service: "payments", Port: 5000,
		}))

		// The proxy will be ignored.
		lastIdx++
		require.NoError(t, s.EnsureService(lastIdx, "foo", &structs.NodeService{
			Kind:    structs.ServiceKindConnectProxy,
			ID:      "payments-proxy",
			Service: "payments-proxy",
			Port:    5000,
		}))

		// Ensure everything is L7-capable.
		ensureConfigEntry(t, &structs.ProxyConfigEntry{
			Kind: structs.ProxyDefaults,
			Name: structs.ProxyConfigGlobal,
			Config: map[string]interface{}{
				"protocol": "http",
			},
			EnterpriseMeta: *defaultEntMeta,
		})

		ensureConfigEntry(t, &structs.ServiceRouterConfigEntry{
			Kind:           structs.ServiceRouter,
			Name:           "router",
			EnterpriseMeta: *defaultEntMeta,
		})

		ensureConfigEntry(t, &structs.ServiceSplitterConfigEntry{
			Kind:           structs.ServiceSplitter,
			Name:           "splitter",
			EnterpriseMeta: *defaultEntMeta,
			Splits:         []structs.ServiceSplit{{Weight: 100}},
		})

		ensureConfigEntry(t, &structs.ServiceResolverConfigEntry{
			Kind:           structs.ServiceResolver,
			Name:           "resolver",
			EnterpriseMeta: *defaultEntMeta,
		})

		require.True(t, watchFired(ws))
		ws = memdb.NewWatchSet()

		expect := &structs.ExportedServiceList{
			Services: []structs.ServiceName{
				{
					Name:           "billing",
					EnterpriseMeta: *defaultEntMeta,
				},
				{
					Name:           "payments",
					EnterpriseMeta: *defaultEntMeta,
				},
				// NOTE: no payments-proxy here
			},
			DiscoChains: []structs.ServiceName{
				{
					Name:           "resolver",
					EnterpriseMeta: *defaultEntMeta,
				},
				{
					Name:           "router",
					EnterpriseMeta: *defaultEntMeta,
				},
				{
					Name:           "splitter",
					EnterpriseMeta: *defaultEntMeta,
				},
			},
			ConnectProtocol: map[structs.ServiceName]string{
				newSN("billing"):  "http",
				newSN("payments"): "http",
				newSN("resolver"): "http",
				newSN("router"):   "http",
				newSN("splitter"): "http",
			},
		}
		idx, got, err := s.ExportedServicesForPeer(ws, id)
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.Equal(t, expect, got)
	})

	testutil.RunStep(t, "config entry with wildcard service names picks up service deletions", func(t *testing.T) {
		lastIdx++
		require.NoError(t, s.DeleteService(lastIdx, "foo", "billing", nil, ""))

		lastIdx++
		require.NoError(t, s.DeleteConfigEntry(lastIdx, structs.ServiceSplitter, "splitter", nil))

		require.True(t, watchFired(ws))
		ws = memdb.NewWatchSet()

		expect := &structs.ExportedServiceList{
			Services: []structs.ServiceName{
				{
					Name:           "payments",
					EnterpriseMeta: *defaultEntMeta,
				},
				// NOTE: no payments-proxy here
			},
			DiscoChains: []structs.ServiceName{
				{
					Name:           "resolver",
					EnterpriseMeta: *defaultEntMeta,
				},
				{
					Name:           "router",
					EnterpriseMeta: *defaultEntMeta,
				},
			},
			ConnectProtocol: map[structs.ServiceName]string{
				newSN("payments"): "http",
				newSN("resolver"): "http",
				newSN("router"):   "http",
			},
		}
		idx, got, err := s.ExportedServicesForPeer(ws, id)
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.Equal(t, expect, got)
	})

	testutil.RunStep(t, "deleting the config entry clears exported services", func(t *testing.T) {
		expect := &structs.ExportedServiceList{}

		require.NoError(t, s.DeleteConfigEntry(lastIdx, structs.ExportedServices, "default", defaultEntMeta))
		idx, got, err := s.ExportedServicesForPeer(ws, id)
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.Equal(t, expect, got)
	})
}

func TestStateStore_PeeringsForService(t *testing.T) {
	type testPeering struct {
		peering *pbpeering.Peering
		delete  bool
	}
	type testCase struct {
		name      string
		services  []structs.ServiceName
		peerings  []testPeering
		entry     *structs.ExportedServicesConfigEntry
		query     []string
		expect    [][]*pbpeering.Peering
		expectIdx uint64
	}

	run := func(t *testing.T, tc testCase) {
		s := testStateStore(t)

		var lastIdx uint64
		// Create peerings
		for _, tp := range tc.peerings {
			lastIdx++
			require.NoError(t, s.PeeringWrite(lastIdx, tp.peering))

			// New peerings can't be marked for deletion so there is a two step process
			// of first creating the peering and then marking it for deletion by setting DeletedAt.
			if tp.delete {
				lastIdx++

				copied := pbpeering.Peering{
					Name:      tp.peering.Name,
					DeletedAt: structs.TimeToProto(time.Now()),
				}
				require.NoError(t, s.PeeringWrite(lastIdx, &copied))
			}

			// make sure it got created
			q := Query{Value: tp.peering.Name}
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
		if tc.entry != nil {
			lastIdx++
			require.NoError(t, tc.entry.Normalize())
			require.NoError(t, s.EnsureConfigEntry(lastIdx, tc.entry))
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
			peerings: []testPeering{},
			entry:    nil,
			query:    []string{"foo"},
			expect:   [][]*pbpeering.Peering{{}},
		},
		{
			name: "peerings marked for deletion are excluded",
			services: []structs.ServiceName{
				{Name: "foo"},
			},
			peerings: []testPeering{
				{
					peering: &pbpeering.Peering{
						Name:  "peer1",
						State: pbpeering.PeeringState_INITIAL,
					},
				},
				{
					peering: &pbpeering.Peering{
						Name: "peer2",
					},
					delete: true,
				},
			},
			entry: &structs.ExportedServicesConfigEntry{
				Name: "default",
				Services: []structs.ExportedService{
					{
						Name: "foo",
						Consumers: []structs.ServiceConsumer{
							{
								PeerName: "peer1",
							},
							{
								PeerName: "peer2",
							},
						},
					},
				},
			},
			query: []string{"foo"},
			expect: [][]*pbpeering.Peering{
				{
					{Name: "peer1", State: pbpeering.PeeringState_INITIAL},
				},
			},
			expectIdx: uint64(6), // config	entries max index
		},
		{
			name: "config entry with exact service name",
			services: []structs.ServiceName{
				{Name: "foo"},
				{Name: "bar"},
			},
			peerings: []testPeering{
				{
					peering: &pbpeering.Peering{
						Name:  "peer1",
						State: pbpeering.PeeringState_INITIAL,
					},
				},
				{
					peering: &pbpeering.Peering{
						Name:  "peer2",
						State: pbpeering.PeeringState_INITIAL,
					},
				},
			},
			entry: &structs.ExportedServicesConfigEntry{
				Name: "default",
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
			peerings: []testPeering{
				{
					peering: &pbpeering.Peering{
						Name:  "peer1",
						State: pbpeering.PeeringState_INITIAL,
					},
				},
				{
					peering: &pbpeering.Peering{
						Name:  "peer2",
						State: pbpeering.PeeringState_INITIAL,
					},
				},
				{
					peering: &pbpeering.Peering{
						Name:  "peer3",
						State: pbpeering.PeeringState_INITIAL,
					},
				},
			},
			entry: &structs.ExportedServicesConfigEntry{
				Name: "default",
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
			query: []string{"foo", "bar"},
			expect: [][]*pbpeering.Peering{
				{
					{Name: "peer1", State: pbpeering.PeeringState_INITIAL},
					{Name: "peer2", State: pbpeering.PeeringState_INITIAL},
				},
				{
					{Name: "peer3", State: pbpeering.PeeringState_INITIAL},
				},
			},
			expectIdx: uint64(7),
		},
	}

	for _, tc := range cases {
		testutil.RunStep(t, tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestStore_TrustBundleListByService(t *testing.T) {
	store := testStateStore(t)
	entMeta := *acl.DefaultEnterpriseMeta()

	var lastIdx uint64
	ws := memdb.NewWatchSet()

	testutil.RunStep(t, "no results on initial setup", func(t *testing.T) {
		idx, resp, err := store.TrustBundleListByService(ws, "foo", entMeta)
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.Len(t, resp, 0)
	})

	testutil.RunStep(t, "registering service does not yield trust bundles", func(t *testing.T) {
		lastIdx++
		require.NoError(t, store.EnsureNode(lastIdx, &structs.Node{
			Node:    "my-node",
			Address: "127.0.0.1",
		}))

		lastIdx++
		require.NoError(t, store.EnsureService(lastIdx, "my-node", &structs.NodeService{
			ID:      "foo-1",
			Service: "foo",
			Port:    8000,
		}))

		require.False(t, watchFired(ws))

		idx, resp, err := store.TrustBundleListByService(ws, "foo", entMeta)
		require.NoError(t, err)
		require.Len(t, resp, 0)
		require.Equal(t, lastIdx-2, idx)
	})

	testutil.RunStep(t, "creating peering does not yield trust bundles", func(t *testing.T) {
		lastIdx++
		require.NoError(t, store.PeeringWrite(lastIdx, &pbpeering.Peering{
			Name: "peer1",
		}))

		// The peering is only watched after the service is exported via config entry.
		require.False(t, watchFired(ws))

		idx, resp, err := store.TrustBundleListByService(ws, "foo", entMeta)
		require.NoError(t, err)
		require.Equal(t, uint64(0), idx)
		require.Len(t, resp, 0)
	})

	testutil.RunStep(t, "exporting the service does not yield trust bundles", func(t *testing.T) {
		lastIdx++
		require.NoError(t, store.EnsureConfigEntry(lastIdx, &structs.ExportedServicesConfigEntry{
			Name: "default",
			Services: []structs.ExportedService{
				{
					Name: "foo",
					Consumers: []structs.ServiceConsumer{
						{
							PeerName: "peer1",
						},
					},
				},
			},
		}))

		// The config entry is watched.
		require.True(t, watchFired(ws))
		ws = memdb.NewWatchSet()

		idx, resp, err := store.TrustBundleListByService(ws, "foo", entMeta)
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.Len(t, resp, 0)
	})

	testutil.RunStep(t, "trust bundles are returned after they are created", func(t *testing.T) {
		lastIdx++
		require.NoError(t, store.PeeringTrustBundleWrite(lastIdx, &pbpeering.PeeringTrustBundle{
			TrustDomain: "peer1.com",
			PeerName:    "peer1",
			RootPEMs:    []string{"peer-root-1"},
		}))

		require.True(t, watchFired(ws))
		ws = memdb.NewWatchSet()

		idx, resp, err := store.TrustBundleListByService(ws, "foo", entMeta)
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.Len(t, resp, 1)
		require.Equal(t, []string{"peer-root-1"}, resp[0].RootPEMs)
	})

	testutil.RunStep(t, "trust bundles are not returned after unexporting service", func(t *testing.T) {
		lastIdx++
		require.NoError(t, store.DeleteConfigEntry(lastIdx, structs.ExportedServices, "default", &entMeta))

		require.True(t, watchFired(ws))
		ws = memdb.NewWatchSet()

		idx, resp, err := store.TrustBundleListByService(ws, "foo", entMeta)
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.Len(t, resp, 0)
	})

	testutil.RunStep(t, "trust bundles are returned after config entry is restored", func(t *testing.T) {
		lastIdx++
		require.NoError(t, store.EnsureConfigEntry(lastIdx, &structs.ExportedServicesConfigEntry{
			Name: "default",
			Services: []structs.ExportedService{
				{
					Name: "foo",
					Consumers: []structs.ServiceConsumer{
						{
							PeerName: "peer1",
						},
					},
				},
			},
		}))

		require.True(t, watchFired(ws))
		ws = memdb.NewWatchSet()

		idx, resp, err := store.TrustBundleListByService(ws, "foo", entMeta)
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.Len(t, resp, 1)
		require.Equal(t, []string{"peer-root-1"}, resp[0].RootPEMs)
	})

	testutil.RunStep(t, "bundles for other peers are ignored", func(t *testing.T) {
		lastIdx++
		require.NoError(t, store.PeeringWrite(lastIdx, &pbpeering.Peering{
			Name: "peer2",
		}))

		lastIdx++
		require.NoError(t, store.PeeringTrustBundleWrite(lastIdx, &pbpeering.PeeringTrustBundle{
			TrustDomain: "peer2.com",
			PeerName:    "peer2",
			RootPEMs:    []string{"peer-root-2"},
		}))

		// No relevant changes.
		require.False(t, watchFired(ws))
		ws = memdb.NewWatchSet()

		idx, resp, err := store.TrustBundleListByService(ws, "foo", entMeta)
		require.NoError(t, err)
		require.Equal(t, lastIdx-2, idx)
		require.Len(t, resp, 1)
		require.Equal(t, []string{"peer-root-1"}, resp[0].RootPEMs)
	})

	testutil.RunStep(t, "second bundle is returned when service is exported to that peer", func(t *testing.T) {
		lastIdx++
		require.NoError(t, store.EnsureConfigEntry(lastIdx, &structs.ExportedServicesConfigEntry{
			Name: "default",
			Services: []structs.ExportedService{
				{
					Name: "foo",
					Consumers: []structs.ServiceConsumer{
						{
							PeerName: "peer1",
						},
						{
							PeerName: "peer2",
						},
					},
				},
			},
		}))

		require.True(t, watchFired(ws))
		ws = memdb.NewWatchSet()

		idx, resp, err := store.TrustBundleListByService(ws, "foo", entMeta)
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.Len(t, resp, 2)
		require.Equal(t, []string{"peer-root-1"}, resp[0].RootPEMs)
		require.Equal(t, []string{"peer-root-2"}, resp[1].RootPEMs)
	})

	testutil.RunStep(t, "deleting the peering excludes its trust bundle", func(t *testing.T) {
		lastIdx++
		require.NoError(t, store.PeeringWrite(lastIdx, &pbpeering.Peering{
			Name:      "peer1",
			DeletedAt: structs.TimeToProto(time.Now()),
		}))

		require.True(t, watchFired(ws))
		ws = memdb.NewWatchSet()

		idx, resp, err := store.TrustBundleListByService(ws, "foo", entMeta)
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.Len(t, resp, 1)
		require.Equal(t, []string{"peer-root-2"}, resp[0].RootPEMs)
	})

	testutil.RunStep(t, "deleting the service does not excludes its trust bundle", func(t *testing.T) {
		lastIdx++
		require.NoError(t, store.DeleteService(lastIdx, "my-node", "foo-1", &entMeta, ""))

		require.False(t, watchFired(ws))

		idx, resp, err := store.TrustBundleListByService(ws, "foo", entMeta)
		require.NoError(t, err)
		require.Equal(t, lastIdx-1, idx)
		require.Len(t, resp, 1)
		require.Equal(t, []string{"peer-root-2"}, resp[0].RootPEMs)
	})
}

func TestStateStore_Peering_ListDeleted(t *testing.T) {
	s := testStateStore(t)

	// Insert one active peering and two marked for deletion.
	{
		tx := s.db.WriteTxn(0)
		defer tx.Abort()

		err := tx.Insert(tablePeering, &pbpeering.Peering{
			Name:        "foo",
			Partition:   acl.DefaultPartitionName,
			ID:          "9e650110-ac74-4c5a-a6a8-9348b2bed4e9",
			DeletedAt:   structs.TimeToProto(time.Now()),
			CreateIndex: 1,
			ModifyIndex: 1,
		})
		require.NoError(t, err)

		err = tx.Insert(tablePeering, &pbpeering.Peering{
			Name:        "bar",
			Partition:   acl.DefaultPartitionName,
			ID:          "5ebcff30-5509-4858-8142-a8e580f1863f",
			CreateIndex: 2,
			ModifyIndex: 2,
		})
		require.NoError(t, err)

		err = tx.Insert(tablePeering, &pbpeering.Peering{
			Name:        "baz",
			Partition:   acl.DefaultPartitionName,
			ID:          "432feb2f-5476-4ae2-b33c-e43640ca0e86",
			DeletedAt:   structs.TimeToProto(time.Now()),
			CreateIndex: 3,
			ModifyIndex: 3,
		})
		require.NoError(t, err)

		err = tx.Insert(tableIndex, &IndexEntry{
			Key:   tablePeering,
			Value: 3,
		})
		require.NoError(t, err)
		require.NoError(t, tx.Commit())

	}

	idx, deleted, err := s.PeeringListDeleted(nil)
	require.NoError(t, err)
	require.Equal(t, uint64(3), idx)
	require.Len(t, deleted, 2)

	var names []string
	for _, peering := range deleted {
		names = append(names, peering.Name)
	}

	require.ElementsMatch(t, []string{"foo", "baz"}, names)
}
