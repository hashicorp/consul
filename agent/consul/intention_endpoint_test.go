// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	msgpackrpc "github.com/hashicorp/consul-net-rpc/net-rpc-msgpackrpc"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
)

// Test basic creation
func TestIntentionApply_new(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	waitForLeaderEstablishment(t, s1)

	// Setup a basic record to create
	ixn := structs.IntentionRequest{
		Datacenter: "dc1",
		Op:         structs.IntentionOpCreate,
		Intention: &structs.Intention{
			SourceNS:        structs.IntentionDefaultNamespace,
			SourceName:      "test",
			DestinationNS:   structs.IntentionDefaultNamespace,
			DestinationName: "test",
			Action:          structs.IntentionActionAllow,
			SourceType:      structs.IntentionSourceConsul,
			Meta:            map[string]string{},
		},
	}
	var reply string

	// Record now to check created at time
	now := time.Now()

	// Create
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))
	require.NotEmpty(t, reply)

	// Read
	ixn.Intention.ID = reply
	{
		req := &structs.IntentionQueryRequest{
			Datacenter:  "dc1",
			IntentionID: ixn.Intention.ID,
		}
		var resp structs.IndexedIntentions
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Intention.Get", req, &resp))
		require.Len(t, resp.Intentions, 1)
		actual := resp.Intentions[0]
		require.Equal(t, resp.Index, actual.ModifyIndex)
		require.WithinDuration(t, now, actual.CreatedAt, 5*time.Second)
		require.WithinDuration(t, now, actual.UpdatedAt, 5*time.Second)

		actual.CreateIndex, actual.ModifyIndex = 0, 0
		actual.CreatedAt = ixn.Intention.CreatedAt
		actual.UpdatedAt = ixn.Intention.UpdatedAt
		actual.Hash = ixn.Intention.Hash
		//nolint:staticcheck
		ixn.Intention.UpdatePrecedence()
		// Partition fields will be normalized on Intention.Get
		ixn.Intention.FillPartitionAndNamespace(nil, true)
		require.Equal(t, ixn.Intention, actual)
	}

	// Rename should fail
	t.Run("renaming the destination should fail", func(t *testing.T) {
		// Setup a basic record to create
		ixn2 := structs.IntentionRequest{
			Datacenter: "dc1",
			Op:         structs.IntentionOpUpdate,
			Intention: &structs.Intention{
				ID:              ixn.Intention.ID,
				SourceNS:        structs.IntentionDefaultNamespace,
				SourceName:      "test",
				DestinationNS:   structs.IntentionDefaultNamespace,
				DestinationName: "test-updated",
				Action:          structs.IntentionActionAllow,
				SourceType:      structs.IntentionSourceConsul,
				Meta:            map[string]string{},
			},
		}

		var reply string
		err := msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn2, &reply)
		testutil.RequireErrorContains(t, err, "Cannot modify Destination partition/namespace/name for an intention once it exists.")
	})
}

// Test the source type defaults
func TestIntentionApply_defaultSourceType(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	waitForLeaderEstablishment(t, s1)

	// Setup a basic record to create
	ixn := structs.IntentionRequest{
		Datacenter: "dc1",
		Op:         structs.IntentionOpCreate,
		Intention: &structs.Intention{
			SourceNS:        structs.IntentionDefaultNamespace,
			SourceName:      "test",
			DestinationNS:   structs.IntentionDefaultNamespace,
			DestinationName: "test",
			Action:          structs.IntentionActionAllow,
		},
	}
	var reply string

	// Create
	require.Nil(t, msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))
	require.NotEmpty(t, reply)

	// Read
	ixn.Intention.ID = reply
	{
		req := &structs.IntentionQueryRequest{
			Datacenter:  "dc1",
			IntentionID: ixn.Intention.ID,
		}
		var resp structs.IndexedIntentions
		require.Nil(t, msgpackrpc.CallWithCodec(codec, "Intention.Get", req, &resp))
		require.Len(t, resp.Intentions, 1)
		actual := resp.Intentions[0]
		require.Equal(t, structs.IntentionSourceConsul, actual.SourceType)
	}
}

// Shouldn't be able to create with an ID set
func TestIntentionApply_createWithID(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	waitForLeaderEstablishment(t, s1)

	// Setup a basic record to create
	ixn := structs.IntentionRequest{
		Datacenter: "dc1",
		Op:         structs.IntentionOpCreate,
		Intention: &structs.Intention{
			ID:              generateUUID(),
			SourceName:      "test",
			DestinationName: "test2",
		},
	}
	var reply string

	// Create
	err := msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply)
	require.NotNil(t, err)
	require.Contains(t, err, "ID must be empty")
}

// Test basic updating
func TestIntentionApply_updateGood(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	waitForLeaderEstablishment(t, s1)

	// Setup a basic record to create
	ixn := structs.IntentionRequest{
		Datacenter: "dc1",
		Op:         structs.IntentionOpCreate,
		Intention: &structs.Intention{
			SourceNS:        structs.IntentionDefaultNamespace,
			SourceName:      "test",
			DestinationNS:   structs.IntentionDefaultNamespace,
			DestinationName: "test",
			Action:          structs.IntentionActionAllow,
			SourceType:      structs.IntentionSourceConsul,
			Meta:            map[string]string{},
		},
	}
	var reply string

	// Create
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))
	require.NotEmpty(t, reply)

	// Read CreatedAt
	var createdAt time.Time
	ixn.Intention.ID = reply
	{
		req := &structs.IntentionQueryRequest{
			Datacenter:  "dc1",
			IntentionID: ixn.Intention.ID,
		}
		var resp structs.IndexedIntentions
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Intention.Get", req, &resp))
		require.Len(t, resp.Intentions, 1)
		actual := resp.Intentions[0]
		createdAt = actual.CreatedAt
	}

	// Sleep a bit so that the updated at will definitely be different, not much
	time.Sleep(1 * time.Millisecond)

	// Update
	ixn.Op = structs.IntentionOpUpdate
	ixn.Intention.ID = reply
	ixn.Intention.Description = "updated"
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))

	// Read
	ixn.Intention.ID = reply
	{
		req := &structs.IntentionQueryRequest{
			Datacenter:  "dc1",
			IntentionID: ixn.Intention.ID,
		}
		var resp structs.IndexedIntentions
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Intention.Get", req, &resp))
		require.Len(t, resp.Intentions, 1)
		actual := resp.Intentions[0]
		require.Equal(t, createdAt, actual.CreatedAt)
		require.WithinDuration(t, time.Now(), actual.UpdatedAt, 5*time.Second)

		actual.CreateIndex, actual.ModifyIndex = 0, 0
		actual.CreatedAt = ixn.Intention.CreatedAt
		actual.UpdatedAt = ixn.Intention.UpdatedAt
		actual.Hash = ixn.Intention.Hash
		//nolint:staticcheck
		ixn.Intention.UpdatePrecedence()
		// Partition fields will be normalized on Intention.Get
		ixn.Intention.FillPartitionAndNamespace(nil, true)
		require.Equal(t, ixn.Intention, actual)
	}
}

// TestIntentionApply_NoSourcePeer makes sure that no intention is created with a SourcePeer since this is not supported
func TestIntentionApply_NoSourcePeer(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, s1 := testServer(t)
	codec := rpcClient(t, s1)

	waitForLeaderEstablishment(t, s1)

	// Setup a basic record to create
	ixn := structs.IntentionRequest{
		Datacenter: "dc1",
		Op:         structs.IntentionOpCreate,
		Intention: &structs.Intention{
			SourceNS:        structs.IntentionDefaultNamespace,
			SourceName:      "test",
			SourcePeer:      "peer1",
			DestinationNS:   structs.IntentionDefaultNamespace,
			DestinationName: "test",
			Action:          structs.IntentionActionAllow,
			SourceType:      structs.IntentionSourceConsul,
			Meta:            map[string]string{},
		},
	}
	var reply string
	err := msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply)
	require.Error(t, err)
	require.Contains(t, err, "SourcePeer field is not supported on this endpoint. Use config entries instead")
	require.Empty(t, reply)
}

// Shouldn't be able to update a non-existent intention
func TestIntentionApply_updateNonExist(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	waitForLeaderEstablishment(t, s1)

	// Setup a basic record to create
	ixn := structs.IntentionRequest{
		Datacenter: "dc1",
		Op:         structs.IntentionOpUpdate,
		Intention: &structs.Intention{
			ID:         generateUUID(),
			SourceName: "test",
		},
	}
	var reply string

	// Create
	err := msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply)
	require.NotNil(t, err)
	require.Contains(t, err, "Cannot modify non-existent intention")
}

// Test basic deleting
func TestIntentionApply_deleteGood(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	waitForLeaderEstablishment(t, s1)

	// Setup a basic record to create
	ixn := structs.IntentionRequest{
		Datacenter: "dc1",
		Op:         structs.IntentionOpCreate,
		Intention: &structs.Intention{
			SourceName:      "test",
			DestinationName: "test",
			Action:          structs.IntentionActionAllow,
		},
	}
	var reply string

	// Delete a non existent intention should return an error
	testutil.RequireErrorContains(t, msgpackrpc.CallWithCodec(codec, "Intention.Apply", &structs.IntentionRequest{
		Op: structs.IntentionOpDelete,
		Intention: &structs.Intention{
			ID: generateUUID(),
		},
	}, &reply), "Cannot delete non-existent intention")

	// Create
	require.Nil(t, msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))
	require.NotEmpty(t, reply)

	// Delete
	ixn.Op = structs.IntentionOpDelete
	ixn.Intention.ID = reply
	require.Nil(t, msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))

	// Read
	ixn.Intention.ID = reply
	{
		req := &structs.IntentionQueryRequest{
			Datacenter:  "dc1",
			IntentionID: ixn.Intention.ID,
		}
		var resp structs.IndexedIntentions
		err := msgpackrpc.CallWithCodec(codec, "Intention.Get", req, &resp)
		require.NotNil(t, err)
		require.Contains(t, err, ErrIntentionNotFound.Error())
	}
}

func TestIntentionApply_WithoutIDs(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	waitForLeaderEstablishment(t, s1)

	defaultEntMeta := structs.DefaultEnterpriseMetaInDefaultPartition()

	// Force "test" to be L7-capable.
	{
		args := structs.ConfigEntryRequest{
			Datacenter: "dc1",
			Entry: &structs.ServiceConfigEntry{
				Kind:     structs.ServiceDefaults,
				Name:     "test",
				Protocol: "http",
			},
		}

		var out bool
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &args, &out))
		require.True(t, out)
	}

	opApply := func(req *structs.IntentionRequest) error {
		req.Datacenter = "dc1"
		var ignored string
		return msgpackrpc.CallWithCodec(codec, "Intention.Apply", &req, &ignored)
	}

	opGet := func(req *structs.IntentionQueryRequest) (*structs.IndexedIntentions, error) {
		req.Datacenter = "dc1"
		var resp structs.IndexedIntentions
		if err := msgpackrpc.CallWithCodec(codec, "Intention.Get", req, &resp); err != nil {
			return nil, err
		}
		return &resp, nil
	}

	opList := func() (*structs.IndexedIntentions, error) {
		req := &structs.IntentionListRequest{
			Datacenter:     "dc1",
			EnterpriseMeta: *structs.WildcardEnterpriseMetaInDefaultPartition(),
		}
		var resp structs.IndexedIntentions
		if err := msgpackrpc.CallWithCodec(codec, "Intention.List", req, &resp); err != nil {
			return nil, err
		}
		return &resp, nil
	}

	configEntryUpsert := func(entry *structs.ServiceIntentionsConfigEntry) error {
		req := &structs.ConfigEntryRequest{
			Datacenter: "dc1",
			Op:         structs.ConfigEntryUpsert,
			Entry:      entry,
		}
		var ignored bool
		return msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", req, &ignored)
	}

	getConfigEntry := func(kind, name string) (*structs.ServiceIntentionsConfigEntry, error) {
		state := s1.fsm.State()
		_, entry, err := state.ConfigEntry(nil, kind, name, defaultEntMeta)
		if err != nil {
			return nil, err
		}

		ixn, ok := entry.(*structs.ServiceIntentionsConfigEntry)
		if !ok {
			return nil, fmt.Errorf("unexpected type: %T", entry)
		}
		return ixn, nil
	}

	// Setup a basic record to create
	require.NoError(t, opApply(&structs.IntentionRequest{
		Op: structs.IntentionOpUpsert,
		Intention: &structs.Intention{
			SourceName:      "test",
			DestinationName: "test",
			Action:          structs.IntentionActionAllow,
			Description:     "original",
		},
	}))

	// Read it back.
	{
		resp, err := opGet(&structs.IntentionQueryRequest{
			Exact: &structs.IntentionQueryExact{
				SourceName:      "test",
				DestinationName: "test",
			},
		})
		require.NoError(t, err)

		require.Len(t, resp.Intentions, 1)
		got := resp.Intentions[0]
		require.Equal(t, "original", got.Description)

		// L4
		require.Equal(t, structs.IntentionActionAllow, got.Action)
		require.Empty(t, got.Permissions)

		// Verify it is in the new-style.
		require.Empty(t, got.ID)
		require.True(t, got.CreatedAt.IsZero())
		require.True(t, got.UpdatedAt.IsZero())
	}

	// Double check that there's only 1.
	{
		resp, err := opList()
		require.NoError(t, err)
		require.Len(t, resp.Intentions, 1)
	}

	// Verify the config entry structure is expected.
	{
		entry, err := getConfigEntry(structs.ServiceIntentions, "test")
		require.NoError(t, err)
		require.NotNil(t, entry)

		expect := &structs.ServiceIntentionsConfigEntry{
			Kind:           structs.ServiceIntentions,
			Name:           "test",
			EnterpriseMeta: *defaultEntMeta,
			Sources: []*structs.SourceIntention{
				{
					Name:           "test",
					EnterpriseMeta: *defaultEntMeta,
					Action:         structs.IntentionActionAllow,
					Description:    "original",
					Precedence:     9,
					Type:           structs.IntentionSourceConsul,
				},
			},
			RaftIndex: entry.RaftIndex,
		}

		require.Equal(t, expect, entry)
	}

	// Update in place.
	require.NoError(t, opApply(&structs.IntentionRequest{
		Op: structs.IntentionOpUpsert,
		Intention: &structs.Intention{
			SourceName:      "test",
			DestinationName: "test",
			Action:          structs.IntentionActionAllow,
			Description:     "updated",
		},
	}))

	// Read it back.
	{
		resp, err := opGet(&structs.IntentionQueryRequest{
			Exact: &structs.IntentionQueryExact{
				SourceName:      "test",
				DestinationName: "test",
			},
		})
		require.NoError(t, err)

		require.Len(t, resp.Intentions, 1)
		got := resp.Intentions[0]
		require.Equal(t, "updated", got.Description)

		// L4
		require.Equal(t, structs.IntentionActionAllow, got.Action)
		require.Empty(t, got.Permissions)

		// Verify it is in the new-style.
		require.Empty(t, got.ID)
		require.True(t, got.CreatedAt.IsZero())
		require.True(t, got.UpdatedAt.IsZero())
	}

	// Double check that there's only 1.
	{
		resp, err := opList()
		require.NoError(t, err)
		require.Len(t, resp.Intentions, 1)
	}

	// Create a second one sharing the same destination
	require.NoError(t, opApply(&structs.IntentionRequest{
		Op: structs.IntentionOpUpsert,
		Intention: &structs.Intention{
			SourceName:      "assay",
			DestinationName: "test",
			Description:     "original-2",
			Permissions: []*structs.IntentionPermission{
				{
					Action: structs.IntentionActionAllow,
					HTTP: &structs.IntentionHTTPPermission{
						PathExact: "/foo",
					},
				},
			},
		},
	}))

	// Read it back.
	{
		resp, err := opGet(&structs.IntentionQueryRequest{
			Exact: &structs.IntentionQueryExact{
				SourceName:      "assay",
				DestinationName: "test",
			},
		})
		require.NoError(t, err)

		require.Len(t, resp.Intentions, 1)
		got := resp.Intentions[0]
		require.Equal(t, "original-2", got.Description)

		// L7
		require.Empty(t, got.Action)
		require.Equal(t, []*structs.IntentionPermission{
			{
				Action: structs.IntentionActionAllow,
				HTTP: &structs.IntentionHTTPPermission{
					PathExact: "/foo",
				},
			},
		}, got.Permissions)

		// Verify it is in the new-style.
		require.Empty(t, got.ID)
		require.True(t, got.CreatedAt.IsZero())
		require.True(t, got.UpdatedAt.IsZero())
	}

	// Double check that there's 2 now.
	{
		resp, err := opList()
		require.NoError(t, err)
		require.Len(t, resp.Intentions, 2)
	}

	// Verify the config entry structure is expected.
	{
		entry, err := getConfigEntry(structs.ServiceIntentions, "test")
		require.NoError(t, err)
		require.NotNil(t, entry)

		expect := &structs.ServiceIntentionsConfigEntry{
			Kind:           structs.ServiceIntentions,
			Name:           "test",
			EnterpriseMeta: *defaultEntMeta,
			Sources: []*structs.SourceIntention{
				{
					Name:           "test",
					EnterpriseMeta: *defaultEntMeta,
					Action:         structs.IntentionActionAllow,
					Description:    "updated",
					Precedence:     9,
					Type:           structs.IntentionSourceConsul,
				},
				{
					Name:           "assay",
					EnterpriseMeta: *defaultEntMeta,
					Description:    "original-2",
					Precedence:     9,
					Type:           structs.IntentionSourceConsul,
					Permissions: []*structs.IntentionPermission{
						{
							Action: structs.IntentionActionAllow,
							HTTP: &structs.IntentionHTTPPermission{
								PathExact: "/foo",
							},
						},
					},
				},
			},
			RaftIndex: entry.RaftIndex,
		}

		require.Equal(t, expect, entry)
	}

	// Delete a non existent intention should act like it did work
	require.NoError(t, opApply(&structs.IntentionRequest{
		Op: structs.IntentionOpDelete,
		Intention: &structs.Intention{
			SourceName:      "ghost",
			DestinationName: "phantom",
		},
	}))

	// Delete the original
	require.NoError(t, opApply(&structs.IntentionRequest{
		Op: structs.IntentionOpDelete,
		Intention: &structs.Intention{
			SourceName:      "test",
			DestinationName: "test",
		},
	}))

	// Read it back (not found)
	{
		_, err := opGet(&structs.IntentionQueryRequest{
			Exact: &structs.IntentionQueryExact{
				SourceName:      "test",
				DestinationName: "test",
			},
		})
		testutil.RequireErrorContains(t, err, ErrIntentionNotFound.Error())
	}

	// Double check that there's 1 again.
	{
		resp, err := opList()
		require.NoError(t, err)
		require.Len(t, resp.Intentions, 1)
	}

	// Verify the config entry structure is expected.
	{
		entry, err := getConfigEntry(structs.ServiceIntentions, "test")
		require.NoError(t, err)
		require.NotNil(t, entry)

		expect := &structs.ServiceIntentionsConfigEntry{
			Kind:           structs.ServiceIntentions,
			Name:           "test",
			EnterpriseMeta: *defaultEntMeta,
			Sources: []*structs.SourceIntention{
				{
					Name:           "assay",
					EnterpriseMeta: *defaultEntMeta,
					Description:    "original-2",
					Precedence:     9,
					Type:           structs.IntentionSourceConsul,
					Permissions: []*structs.IntentionPermission{
						{
							Action: structs.IntentionActionAllow,
							HTTP: &structs.IntentionHTTPPermission{
								PathExact: "/foo",
							},
						},
					},
				},
			},
			RaftIndex: entry.RaftIndex,
		}

		require.Equal(t, expect, entry)
	}

	// Set metadata on the config entry directly.
	{
		require.NoError(t, configEntryUpsert(&structs.ServiceIntentionsConfigEntry{
			Kind:           structs.ServiceIntentions,
			Name:           "test",
			EnterpriseMeta: *defaultEntMeta,
			Meta: map[string]string{
				"foo": "bar",
				"zim": "gir",
			},
			Sources: []*structs.SourceIntention{
				{
					Name:           "assay",
					EnterpriseMeta: *defaultEntMeta,
					Action:         structs.IntentionActionDeny,
					Description:    "original-2",
					Precedence:     9,
					Type:           structs.IntentionSourceConsul,
				},
			},
		}))
	}

	// Attempt to create a new intention and set the metadata.
	{
		err := opApply(&structs.IntentionRequest{
			Op: structs.IntentionOpUpsert,
			Intention: &structs.Intention{
				SourceName:      "foo",
				DestinationName: "bar",
				Action:          structs.IntentionActionDeny,
				Meta:            map[string]string{"horseshoe": "crab"},
			},
		})
		testutil.RequireErrorContains(t, err, "Meta must not be specified")
	}

	// Attempt to update an intention and change the metadata.
	{
		err := opApply(&structs.IntentionRequest{
			Op: structs.IntentionOpUpsert,
			Intention: &structs.Intention{
				SourceName:      "assay",
				DestinationName: "test",
				Action:          structs.IntentionActionDeny,
				Description:     "original-3",
				Meta:            map[string]string{"horseshoe": "crab"},
			},
		})
		testutil.RequireErrorContains(t, err, "Meta must not be specified, or should be unchanged during an update.")
	}

	// Try again with the same metadata.
	require.NoError(t, opApply(&structs.IntentionRequest{
		Op: structs.IntentionOpUpsert,
		Intention: &structs.Intention{
			SourceName:      "assay",
			DestinationName: "test",
			Action:          structs.IntentionActionDeny,
			Description:     "original-3",
			Meta: map[string]string{
				"foo": "bar",
				"zim": "gir",
			},
		},
	}))

	// Read it back.
	{
		resp, err := opGet(&structs.IntentionQueryRequest{
			Exact: &structs.IntentionQueryExact{
				SourceName:      "assay",
				DestinationName: "test",
			},
		})
		require.NoError(t, err)

		require.Len(t, resp.Intentions, 1)
		got := resp.Intentions[0]
		require.Equal(t, "original-3", got.Description)
		require.Equal(t, map[string]string{
			"foo": "bar",
			"zim": "gir",
		}, got.Meta)

		// Verify it is in the new-style.
		require.Empty(t, got.ID)
		require.True(t, got.CreatedAt.IsZero())
		require.True(t, got.UpdatedAt.IsZero())
	}

	// Try again with NO metadata.
	require.NoError(t, opApply(&structs.IntentionRequest{
		Op: structs.IntentionOpUpsert,
		Intention: &structs.Intention{
			SourceName:      "assay",
			DestinationName: "test",
			Action:          structs.IntentionActionDeny,
			Description:     "original-4",
		},
	}))

	// Read it back.
	{
		resp, err := opGet(&structs.IntentionQueryRequest{
			Exact: &structs.IntentionQueryExact{
				SourceName:      "assay",
				DestinationName: "test",
			},
		})
		require.NoError(t, err)

		require.Len(t, resp.Intentions, 1)
		got := resp.Intentions[0]
		require.Equal(t, "original-4", got.Description)
		require.Equal(t, map[string]string{
			"foo": "bar",
			"zim": "gir",
		}, got.Meta)

		// Verify it is in the new-style.
		require.Empty(t, got.ID)
		require.True(t, got.CreatedAt.IsZero())
		require.True(t, got.UpdatedAt.IsZero())
	}
}

// Test apply with a deny ACL
func TestIntentionApply_aclDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	waitForLeaderEstablishment(t, s1)

	rules := `
service "foobar" {
	policy = "deny"
	intentions = "write"
}`
	token := createToken(t, codec, rules)

	// Setup a basic record to create
	ixn := structs.IntentionRequest{
		Datacenter: "dc1",
		Op:         structs.IntentionOpCreate,
		Intention:  structs.TestIntention(t),
	}
	ixn.Intention.DestinationName = "foobar"

	// Create without a token should error since default deny
	var reply string
	err := msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply)
	require.True(t, acl.IsErrPermissionDenied(err))

	// Now add the token and try again.
	ixn.WriteRequest.Token = token
	require.Nil(t, msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))

	// Read
	ixn.Intention.ID = reply
	{
		req := &structs.IntentionQueryRequest{
			Datacenter:   "dc1",
			IntentionID:  ixn.Intention.ID,
			QueryOptions: structs.QueryOptions{Token: "root"},
		}
		var resp structs.IndexedIntentions
		require.Nil(t, msgpackrpc.CallWithCodec(codec, "Intention.Get", req, &resp))
		require.Len(t, resp.Intentions, 1)
		actual := resp.Intentions[0]
		require.Equal(t, resp.Index, actual.ModifyIndex)

		actual.CreateIndex, actual.ModifyIndex = 0, 0
		actual.CreatedAt = ixn.Intention.CreatedAt
		actual.UpdatedAt = ixn.Intention.UpdatedAt
		actual.Hash = ixn.Intention.Hash
		//nolint:staticcheck
		ixn.Intention.UpdatePrecedence()
		require.Equal(t, ixn.Intention, actual)
	}
}

func TestIntention_WildcardACLEnforcement(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, srv, codec := testACLServerWithConfig(t, nil, false)
	waitForLeaderEstablishment(t, srv)

	// create some test policies.

	writeToken, err := upsertTestTokenWithPolicyRules(codec, TestDefaultInitialManagementToken, "dc1", `service_prefix "" { policy = "deny" intentions = "write" }`)
	require.NoError(t, err)
	readToken, err := upsertTestTokenWithPolicyRules(codec, TestDefaultInitialManagementToken, "dc1", `service_prefix "" { policy = "deny" intentions = "read" }`)
	require.NoError(t, err)
	exactToken, err := upsertTestTokenWithPolicyRules(codec, TestDefaultInitialManagementToken, "dc1", `service "*" { policy = "deny" intentions = "write" }`)
	require.NoError(t, err)
	wildcardPrefixToken, err := upsertTestTokenWithPolicyRules(codec, TestDefaultInitialManagementToken, "dc1", `service_prefix "*" { policy = "deny" intentions = "write" }`)
	require.NoError(t, err)
	fooToken, err := upsertTestTokenWithPolicyRules(codec, TestDefaultInitialManagementToken, "dc1", `service "foo" { policy = "deny" intentions = "write" }`)
	require.NoError(t, err)
	denyToken, err := upsertTestTokenWithPolicyRules(codec, TestDefaultInitialManagementToken, "dc1", `service_prefix "" { policy = "deny" intentions = "deny" }`)
	require.NoError(t, err)

	doIntentionCreate := func(t *testing.T, token string, dest string, deny bool) string {
		t.Helper()
		ixn := structs.IntentionRequest{
			Datacenter: "dc1",
			Op:         structs.IntentionOpCreate,
			Intention: &structs.Intention{
				SourceNS:        "default",
				SourceName:      "*",
				DestinationNS:   "default",
				DestinationName: dest,
				Action:          structs.IntentionActionAllow,
				SourceType:      structs.IntentionSourceConsul,
			},
			WriteRequest: structs.WriteRequest{Token: token},
		}
		var reply string
		err := msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply)
		if deny {
			require.Error(t, err)
			require.True(t, acl.IsErrPermissionDenied(err))
			return ""
		} else {
			require.NoError(t, err)
			require.NotEmpty(t, reply)
			return reply
		}
	}

	t.Run("deny-write-for-read-token", func(t *testing.T) {
		// This tests ensures that tokens with only read access to all intentions
		// cannot create a wildcard intention
		doIntentionCreate(t, readToken.SecretID, "*", true)
	})

	t.Run("deny-write-for-exact-wildcard-rule", func(t *testing.T) {
		// This test ensures that having a rules like:
		// service "*" {
		//    intentions = "write"
		// }
		// will not actually allow creating an intention with a wildcard service name
		doIntentionCreate(t, exactToken.SecretID, "*", true)
	})

	t.Run("deny-write-for-prefix-wildcard-rule", func(t *testing.T) {
		// This test ensures that having a rules like:
		// service_prefix "*" {
		//    intentions = "write"
		// }
		// will not actually allow creating an intention with a wildcard service name
		doIntentionCreate(t, wildcardPrefixToken.SecretID, "*", true)
	})

	var intentionID string
	allowWriteOk := t.Run("allow-write", func(t *testing.T) {
		// tests that a token with all the required privileges can create
		// intentions with a wildcard destination
		intentionID = doIntentionCreate(t, writeToken.SecretID, "*", false)
	})

	requireAllowWrite := func(t *testing.T) {
		t.Helper()
		if !allowWriteOk {
			t.Skip("Skipping because the allow-write subtest failed")
		}
	}

	doIntentionRead := func(t *testing.T, token string, deny bool) {
		t.Helper()
		requireAllowWrite(t)
		req := &structs.IntentionQueryRequest{
			Datacenter:   "dc1",
			IntentionID:  intentionID,
			QueryOptions: structs.QueryOptions{Token: token},
		}

		var resp structs.IndexedIntentions
		err := msgpackrpc.CallWithCodec(codec, "Intention.Get", req, &resp)
		if deny {
			require.Error(t, err)
			require.True(t, acl.IsErrPermissionDenied(err))
		} else {
			require.NoError(t, err)
			require.Len(t, resp.Intentions, 1)
			require.Equal(t, "*", resp.Intentions[0].DestinationName)
		}
	}

	t.Run("allow-read-for-write-token", func(t *testing.T) {
		doIntentionRead(t, writeToken.SecretID, false)
	})

	t.Run("allow-read-for-read-token", func(t *testing.T) {
		doIntentionRead(t, readToken.SecretID, false)
	})

	t.Run("allow-read-for-exact-wildcard-token", func(t *testing.T) {
		// this is allowed because, the effect of the policy is to grant
		// intention:write on the service named "*". When reading the
		// intention we will validate that the token has read permissions
		// for any intention that would match the wildcard.
		doIntentionRead(t, exactToken.SecretID, false)
	})

	t.Run("allow-read-for-prefix-wildcard-token", func(t *testing.T) {
		// this is allowed for the same reasons as for the
		// exact-wildcard-token case
		doIntentionRead(t, wildcardPrefixToken.SecretID, false)
	})

	t.Run("deny-read-for-deny-token", func(t *testing.T) {
		doIntentionRead(t, denyToken.SecretID, true)
	})

	doIntentionList := func(t *testing.T, token string, deny bool) {
		t.Helper()
		requireAllowWrite(t)
		req := &structs.IntentionListRequest{
			Datacenter:   "dc1",
			QueryOptions: structs.QueryOptions{Token: token},
		}

		var resp structs.IndexedIntentions
		err := msgpackrpc.CallWithCodec(codec, "Intention.List", req, &resp)
		// even with permission denied this should return success but with an empty list
		require.NoError(t, err)
		if deny {
			require.Empty(t, resp.Intentions)
		} else {
			require.Len(t, resp.Intentions, 1)
			require.Equal(t, "*", resp.Intentions[0].DestinationName)
		}
	}

	t.Run("allow-list-for-write-token", func(t *testing.T) {
		doIntentionList(t, writeToken.SecretID, false)
	})

	t.Run("allow-list-for-read-token", func(t *testing.T) {
		doIntentionList(t, readToken.SecretID, false)
	})

	t.Run("allow-list-for-exact-wildcard-token", func(t *testing.T) {
		doIntentionList(t, exactToken.SecretID, false)
	})

	t.Run("allow-list-for-prefix-wildcard-token", func(t *testing.T) {
		doIntentionList(t, wildcardPrefixToken.SecretID, false)
	})

	t.Run("deny-list-for-deny-token", func(t *testing.T) {
		doIntentionList(t, denyToken.SecretID, true)
	})

	doIntentionMatch := func(t *testing.T, token string, deny bool) {
		t.Helper()
		requireAllowWrite(t)
		req := &structs.IntentionQueryRequest{
			Datacenter: "dc1",
			Match: &structs.IntentionQueryMatch{
				Type: structs.IntentionMatchDestination,
				Entries: []structs.IntentionMatchEntry{
					{
						Namespace: "default",
						Name:      "*",
					},
				},
			},
			QueryOptions: structs.QueryOptions{Token: token},
		}

		var resp structs.IndexedIntentionMatches
		err := msgpackrpc.CallWithCodec(codec, "Intention.Match", req, &resp)
		if deny {
			require.Error(t, err)
			require.Empty(t, resp.Matches)
		} else {
			require.NoError(t, err)
			require.Len(t, resp.Matches, 1)
			require.Len(t, resp.Matches[0], 1)
			require.Equal(t, "*", resp.Matches[0][0].DestinationName)
		}
	}

	t.Run("allow-match-for-write-token", func(t *testing.T) {
		doIntentionMatch(t, writeToken.SecretID, false)
	})

	t.Run("allow-match-for-read-token", func(t *testing.T) {
		doIntentionMatch(t, readToken.SecretID, false)
	})

	t.Run("allow-match-for-exact-wildcard-token", func(t *testing.T) {
		doIntentionMatch(t, exactToken.SecretID, false)
	})

	t.Run("allow-match-for-prefix-wildcard-token", func(t *testing.T) {
		doIntentionMatch(t, wildcardPrefixToken.SecretID, false)
	})

	t.Run("deny-match-for-deny-token", func(t *testing.T) {
		doIntentionMatch(t, denyToken.SecretID, true)
	})

	// Since we can't rename the destination, create a new intention for the rest of this test.
	wildIntentionID := intentionID
	fooIntentionID := doIntentionCreate(t, writeToken.SecretID, "foo", false)

	doIntentionUpdate := func(t *testing.T, token string, intentionID, dest, description string, deny bool) {
		t.Helper()
		requireAllowWrite(t)
		ixn := structs.IntentionRequest{
			Datacenter: "dc1",
			Op:         structs.IntentionOpUpdate,
			Intention: &structs.Intention{
				ID:              intentionID,
				SourceNS:        "default",
				SourceName:      "*",
				DestinationNS:   "default",
				DestinationName: dest,
				Description:     description,
				Action:          structs.IntentionActionAllow,
				SourceType:      structs.IntentionSourceConsul,
			},
			WriteRequest: structs.WriteRequest{Token: token},
		}
		var reply string
		err := msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply)
		if deny {
			require.Error(t, err)
			require.True(t, acl.IsErrPermissionDenied(err))
		} else {
			require.NoError(t, err)
		}
	}

	t.Run("deny-update-for-foo-token", func(t *testing.T) {
		doIntentionUpdate(t, fooToken.SecretID, wildIntentionID, "*", "wild-desc", true)
	})

	t.Run("allow-update-for-prefix-token", func(t *testing.T) {
		// This tests that the prefix token can edit wildcard intentions and regular intentions.
		doIntentionUpdate(t, writeToken.SecretID, fooIntentionID, "foo", "foo-desc-two", false)
		doIntentionUpdate(t, writeToken.SecretID, wildIntentionID, "*", "wild-desc-two", false)
	})

	doIntentionDelete := func(t *testing.T, token string, intentionID string, deny bool) {
		t.Helper()
		requireAllowWrite(t)
		ixn := structs.IntentionRequest{
			Datacenter: "dc1",
			Op:         structs.IntentionOpDelete,
			Intention: &structs.Intention{
				ID: intentionID,
			},
			WriteRequest: structs.WriteRequest{Token: token},
		}
		var reply string
		err := msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply)
		if deny {
			require.Error(t, err)
			require.True(t, acl.IsErrPermissionDenied(err))
		} else {
			require.NoError(t, err)
		}
	}

	t.Run("deny-delete-for-read-token", func(t *testing.T) {
		doIntentionDelete(t, readToken.SecretID, fooIntentionID, true)
	})

	t.Run("deny-delete-for-exact-wildcard-rule", func(t *testing.T) {
		// This test ensures that having a rules like:
		// service "*" {
		//    intentions = "write"
		// }
		// will not actually allow deleting an intention with a wildcard service name
		doIntentionDelete(t, exactToken.SecretID, fooIntentionID, true)
	})

	t.Run("deny-delete-for-prefix-wildcard-rule", func(t *testing.T) {
		// This test ensures that having a rules like:
		// service_prefix "*" {
		//    intentions = "write"
		// }
		// will not actually allow creating an intention with a wildcard service name
		doIntentionDelete(t, wildcardPrefixToken.SecretID, fooIntentionID, true)
	})

	t.Run("allow-delete", func(t *testing.T) {
		// tests that a token with all the required privileges can delete
		// intentions with a wildcard destination
		doIntentionDelete(t, writeToken.SecretID, fooIntentionID, false)
	})
}

// Test apply with delete and a default deny ACL
func TestIntentionApply_aclDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	waitForLeaderEstablishment(t, s1)

	rules := `
service "foobar" {
	policy = "deny"
	intentions = "write"
}`
	token := createToken(t, codec, rules)

	// Setup a basic record to create
	ixn := structs.IntentionRequest{
		Datacenter: "dc1",
		Op:         structs.IntentionOpCreate,
		Intention:  structs.TestIntention(t),
	}
	ixn.Intention.DestinationName = "foobar"
	ixn.WriteRequest.Token = token

	// Create
	var reply string
	require.Nil(t, msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))

	// Try to do a delete with no token; this should get rejected.
	ixn.Op = structs.IntentionOpDelete
	ixn.Intention.ID = reply
	ixn.WriteRequest.Token = ""
	err := msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply)
	require.True(t, acl.IsErrPermissionDenied(err))

	// Try again with the original token. This should go through.
	ixn.WriteRequest.Token = token
	require.Nil(t, msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))

	// Verify it is gone
	{
		req := &structs.IntentionQueryRequest{
			Datacenter:  "dc1",
			IntentionID: ixn.Intention.ID,
		}
		var resp structs.IndexedIntentions
		err := msgpackrpc.CallWithCodec(codec, "Intention.Get", req, &resp)
		require.NotNil(t, err)
		require.Contains(t, err.Error(), ErrIntentionNotFound.Error())
	}
}

// Test apply with update and a default deny ACL
func TestIntentionApply_aclUpdate(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	waitForLeaderEstablishment(t, s1)

	rules := `
service "foobar" {
	policy = "deny"
	intentions = "write"
}`
	token := createToken(t, codec, rules)

	// Setup a basic record to create
	ixn := structs.IntentionRequest{
		Datacenter: "dc1",
		Op:         structs.IntentionOpCreate,
		Intention:  structs.TestIntention(t),
	}
	ixn.Intention.DestinationName = "foobar"
	ixn.WriteRequest.Token = token

	// Create
	var reply string
	require.Nil(t, msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))

	// Try to do an update without a token; this should get rejected.
	ixn.Op = structs.IntentionOpUpdate
	ixn.Intention.ID = reply
	ixn.WriteRequest.Token = ""
	err := msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply)
	require.True(t, acl.IsErrPermissionDenied(err))

	// Try again with the original token; this should go through.
	ixn.WriteRequest.Token = token
	require.Nil(t, msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))
}

// Test apply with a management token
func TestIntentionApply_aclManagement(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	waitForLeaderEstablishment(t, s1)

	// Setup a basic record to create
	ixn := structs.IntentionRequest{
		Datacenter: "dc1",
		Op:         structs.IntentionOpCreate,
		Intention:  structs.TestIntention(t),
	}
	ixn.Intention.DestinationName = "foobar"
	ixn.WriteRequest.Token = "root"

	// Create
	var reply string
	require.Nil(t, msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))
	ixn.Intention.ID = reply

	// Update
	ixn.Op = structs.IntentionOpUpdate
	require.Nil(t, msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))

	// Delete
	ixn.Op = structs.IntentionOpDelete
	require.Nil(t, msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))
}

// Test update changing the name where an ACL won't allow it
func TestIntentionApply_aclUpdateChange(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	waitForLeaderEstablishment(t, s1)

	rules := `
service "foobar" {
	policy = "deny"
	intentions = "write"
}`
	token := createToken(t, codec, rules)

	// Setup a basic record to create
	ixn := structs.IntentionRequest{
		Datacenter: "dc1",
		Op:         structs.IntentionOpCreate,
		Intention:  structs.TestIntention(t),
	}
	ixn.Intention.DestinationName = "bar"
	ixn.WriteRequest.Token = "root"

	// Create
	var reply string
	require.Nil(t, msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))

	// Try to do an update without a token; this should get rejected.
	ixn.Op = structs.IntentionOpUpdate
	ixn.Intention.ID = reply
	ixn.Intention.DestinationName = "foo"
	ixn.WriteRequest.Token = token
	err := msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply)
	require.True(t, acl.IsErrPermissionDenied(err))
}

// Test reading with ACLs
func TestIntentionGet_acl(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	waitForLeaderEstablishment(t, s1)

	// Create an ACL with service write permissions. This will grant
	// intentions read on either end of an intention.
	token, err := upsertTestTokenWithPolicyRules(codec, "root", "dc1", `
	service "foobar" {
		policy = "write"
	}`)
	require.NoError(t, err)

	// Setup a basic record to create
	ixn := structs.IntentionRequest{
		Datacenter: "dc1",
		Op:         structs.IntentionOpCreate,
		Intention:  structs.TestIntention(t),
	}
	ixn.Intention.DestinationName = "foobar"
	ixn.WriteRequest.Token = "root"

	// Create
	var reply string
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))
	ixn.Intention.ID = reply

	t.Run("Read by ID without token should be error", func(t *testing.T) {
		req := &structs.IntentionQueryRequest{
			Datacenter:  "dc1",
			IntentionID: ixn.Intention.ID,
		}

		var resp structs.IndexedIntentions
		err := msgpackrpc.CallWithCodec(codec, "Intention.Get", req, &resp)
		require.True(t, acl.IsErrPermissionDenied(err))
		require.Len(t, resp.Intentions, 0)
	})

	t.Run("Read by ID with token should work", func(t *testing.T) {
		req := &structs.IntentionQueryRequest{
			Datacenter:   "dc1",
			IntentionID:  ixn.Intention.ID,
			QueryOptions: structs.QueryOptions{Token: token.SecretID},
		}

		var resp structs.IndexedIntentions
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Intention.Get", req, &resp))
		require.Len(t, resp.Intentions, 1)
	})

	t.Run("Read by Exact without token should be error", func(t *testing.T) {
		req := &structs.IntentionQueryRequest{
			Datacenter: "dc1",
			Exact: &structs.IntentionQueryExact{
				SourceNS:        structs.IntentionDefaultNamespace,
				SourceName:      "api",
				DestinationNS:   structs.IntentionDefaultNamespace,
				DestinationName: "foobar",
			},
		}

		var resp structs.IndexedIntentions
		err := msgpackrpc.CallWithCodec(codec, "Intention.Get", req, &resp)
		require.True(t, acl.IsErrPermissionDenied(err))
		require.Len(t, resp.Intentions, 0)
	})

	t.Run("Read by Exact with token should work", func(t *testing.T) {
		req := &structs.IntentionQueryRequest{
			Datacenter: "dc1",
			Exact: &structs.IntentionQueryExact{
				SourceNS:        structs.IntentionDefaultNamespace,
				SourceName:      "api",
				DestinationNS:   structs.IntentionDefaultNamespace,
				DestinationName: "foobar",
			},
			QueryOptions: structs.QueryOptions{Token: token.SecretID},
		}

		var resp structs.IndexedIntentions
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Intention.Get", req, &resp))
		require.Len(t, resp.Intentions, 1)
	})
}

func TestIntentionList(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	codec := rpcClient(t, s1)
	defer codec.Close()
	waitForLeaderEstablishment(t, s1)

	// Test with no intentions inserted yet
	{
		req := &structs.IntentionListRequest{
			Datacenter: "dc1",
		}
		var resp structs.IndexedIntentions
		require.Nil(t, msgpackrpc.CallWithCodec(codec, "Intention.List", req, &resp))
		require.NotNil(t, resp.Intentions)
		require.Len(t, resp.Intentions, 0)
	}
}

// Test listing with ACLs
func TestIntentionList_acl(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServerWithConfig(t, testServerACLConfig)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	waitForLeaderEstablishment(t, s1)

	token, err := upsertTestTokenWithPolicyRules(codec, TestDefaultInitialManagementToken, "dc1", `service_prefix "foo" { policy = "write" }`)
	require.NoError(t, err)

	// Create a few records
	for _, name := range []string{"foobar", "bar", "baz"} {
		ixn := structs.IntentionRequest{
			Datacenter: "dc1",
			Op:         structs.IntentionOpCreate,
			Intention:  structs.TestIntention(t),
		}
		ixn.Intention.SourceNS = "default"
		ixn.Intention.DestinationNS = "default"
		ixn.Intention.DestinationName = name
		ixn.WriteRequest.Token = TestDefaultInitialManagementToken

		// Create
		var reply string
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))
	}

	// Test with no token
	t.Run("no-token", func(t *testing.T) {
		req := &structs.IntentionListRequest{
			Datacenter: "dc1",
		}
		var resp structs.IndexedIntentions
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Intention.List", req, &resp))
		require.Len(t, resp.Intentions, 0)
		require.False(t, resp.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be false")
	})

	// Test with management token
	t.Run("initial-management-token", func(t *testing.T) {
		req := &structs.IntentionListRequest{
			Datacenter:   "dc1",
			QueryOptions: structs.QueryOptions{Token: TestDefaultInitialManagementToken},
		}
		var resp structs.IndexedIntentions
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Intention.List", req, &resp))
		require.Len(t, resp.Intentions, 3)
		require.False(t, resp.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be false")
	})

	// Test with user token
	t.Run("user-token", func(t *testing.T) {
		req := &structs.IntentionListRequest{
			Datacenter:   "dc1",
			QueryOptions: structs.QueryOptions{Token: token.SecretID},
		}
		var resp structs.IndexedIntentions
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Intention.List", req, &resp))
		require.Len(t, resp.Intentions, 1)
		require.True(t, resp.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be true")
	})

	t.Run("filtered", func(t *testing.T) {
		req := &structs.IntentionListRequest{
			Datacenter: "dc1",
			QueryOptions: structs.QueryOptions{
				Token:  TestDefaultInitialManagementToken,
				Filter: "DestinationName == foobar",
			},
		}

		var resp structs.IndexedIntentions
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Intention.List", req, &resp))
		require.Len(t, resp.Intentions, 1)
		require.False(t, resp.QueryMeta.ResultsFilteredByACLs, "ResultsFilteredByACLs should be false")
	})
}

// Test basic matching. We don't need to exhaustively test inputs since this
// is tested in the agent/consul/state package.
func TestIntentionMatch_good(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	waitForLeaderEstablishment(t, s1)

	// Create some records
	{
		insert := [][]string{
			{"default", "*", "default", "*"},
			{"default", "*", "default", "bar"},
			{"default", "*", "default", "baz"}, // shouldn't match
		}

		for _, v := range insert {
			ixn := structs.IntentionRequest{
				Datacenter: "dc1",
				Op:         structs.IntentionOpCreate,
				Intention: &structs.Intention{
					SourceNS:        v[0],
					SourceName:      v[1],
					DestinationNS:   v[2],
					DestinationName: v[3],
					Action:          structs.IntentionActionAllow,
				},
			}

			// Create
			var reply string
			require.Nil(t, msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))
		}
	}

	// Match
	req := &structs.IntentionQueryRequest{
		Datacenter: "dc1",
		Match: &structs.IntentionQueryMatch{
			Type: structs.IntentionMatchDestination,
			Entries: []structs.IntentionMatchEntry{
				{Name: "bar"},
			},
		},
	}
	var resp structs.IndexedIntentionMatches
	require.Nil(t, msgpackrpc.CallWithCodec(codec, "Intention.Match", req, &resp))
	require.Len(t, resp.Matches, 1)

	expected := [][]string{
		{"default", "*", "default", "bar"},
		{"default", "*", "default", "*"},
	}
	var actual [][]string
	for _, ixn := range resp.Matches[0] {
		actual = append(actual, []string{
			ixn.SourceNS,
			ixn.SourceName,
			ixn.DestinationNS,
			ixn.DestinationName,
		})
	}
	require.Equal(t, expected, actual)
}

func TestIntentionMatch_BlockOnNoChange(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, s1 := testServerWithConfig(t, func(c *Config) {
		c.DevMode = true // keep it in ram to make it 10x faster on macos
	})

	codec := rpcClient(t, s1)

	waitForLeaderEstablishment(t, s1)

	run := func(t *testing.T, dataPrefix string, expectMatches int) {
		rpcBlockingQueryTestHarness(t,
			func(minQueryIndex uint64) (*structs.QueryMeta, <-chan error) {
				args := &structs.IntentionQueryRequest{
					Datacenter: "dc1",
					Match: &structs.IntentionQueryMatch{
						Type: structs.IntentionMatchDestination,
						Entries: []structs.IntentionMatchEntry{
							{Name: "bar"},
						},
					},
				}
				args.QueryOptions.MinQueryIndex = minQueryIndex

				var out structs.IndexedIntentionMatches
				errCh := channelCallRPC(s1, "Intention.Match", args, &out, func() error {
					if len(out.Matches) != 1 {
						return fmt.Errorf("expected 1 match got %d", len(out.Matches))
					}
					if len(out.Matches[0]) != expectMatches {
						return fmt.Errorf("expected %d inner matches got %d", expectMatches, len(out.Matches[0]))
					}
					return nil
				})
				return &out.QueryMeta, errCh
			},
			func(i int) <-chan error {
				var out string
				return channelCallRPC(s1, "Intention.Apply", &structs.IntentionRequest{
					Datacenter: "dc1",
					Op:         structs.IntentionOpCreate,
					Intention: &structs.Intention{
						// {"default", "*", "default", "baz"}, // shouldn't match
						SourceNS:        "default",
						SourceName:      "*",
						DestinationNS:   "default",
						DestinationName: fmt.Sprintf(dataPrefix+"%d", i),
						Action:          structs.IntentionActionAllow,
					},
				}, &out, nil)
			},
		)
	}

	testutil.RunStep(t, "test the errNotFound path", func(t *testing.T) {
		run(t, "other", 0)
	})

	// Create some records
	{
		insert := [][]string{
			{"default", "*", "default", "*"},
			{"default", "*", "default", "bar"},
			{"default", "*", "default", "baz"}, // shouldn't match
		}

		for _, v := range insert {
			var out string
			require.NoError(t, msgpackrpc.CallWithCodec(codec, "Intention.Apply", &structs.IntentionRequest{
				Datacenter: "dc1",
				Op:         structs.IntentionOpCreate,
				Intention: &structs.Intention{
					SourceNS:        v[0],
					SourceName:      v[1],
					DestinationNS:   v[2],
					DestinationName: v[3],
					Action:          structs.IntentionActionAllow,
				},
			}, &out))
		}
	}

	testutil.RunStep(t, "test the errNotChanged path", func(t *testing.T) {
		run(t, "completely-different-other", 2)
	})
}

// Test matching with ACLs
func TestIntentionMatch_acl(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, srv, codec := testACLServerWithConfig(t, nil, false)
	waitForLeaderEstablishment(t, srv)

	token, err := upsertTestTokenWithPolicyRules(codec, TestDefaultInitialManagementToken, "dc1", `service "bar" { policy = "write" }`)
	require.NoError(t, err)

	// Create some records
	{
		insert := []string{
			"*",
			"bar",
			"baz",
		}

		for _, v := range insert {
			ixn := structs.IntentionRequest{
				Datacenter: "dc1",
				Op:         structs.IntentionOpCreate,
				Intention:  structs.TestIntention(t),
			}
			ixn.Intention.DestinationName = v
			ixn.WriteRequest.Token = TestDefaultInitialManagementToken

			// Create
			var reply string
			require.Nil(t, msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))
		}
	}

	// Test with no token
	{
		req := &structs.IntentionQueryRequest{
			Datacenter: "dc1",
			Match: &structs.IntentionQueryMatch{
				Type: structs.IntentionMatchDestination,
				Entries: []structs.IntentionMatchEntry{
					{
						Namespace: "default",
						Name:      "bar",
					},
				},
			},
		}
		var resp structs.IndexedIntentionMatches
		err := msgpackrpc.CallWithCodec(codec, "Intention.Match", req, &resp)
		require.Error(t, err)
		require.True(t, acl.IsErrPermissionDenied(err))
		require.Len(t, resp.Matches, 0)
	}

	// Test with proper token
	{
		req := &structs.IntentionQueryRequest{
			Datacenter: "dc1",
			Match: &structs.IntentionQueryMatch{
				Type: structs.IntentionMatchDestination,
				Entries: []structs.IntentionMatchEntry{
					{
						Namespace: "default",
						Name:      "bar",
					},
				},
			},
			QueryOptions: structs.QueryOptions{Token: token.SecretID},
		}
		var resp structs.IndexedIntentionMatches
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Intention.Match", req, &resp))
		require.Len(t, resp.Matches, 1)

		expected := []string{"bar", "*"}
		var actual []string
		for _, ixn := range resp.Matches[0] {
			actual = append(actual, ixn.DestinationName)
		}

		require.ElementsMatch(t, expected, actual)
	}
}

// Test the Check method defaults to allow with no ACL set.
func TestIntentionCheck_defaultNoACL(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	waitForLeaderEstablishment(t, s1)

	// Test
	req := &structs.IntentionQueryRequest{
		Datacenter: "dc1",
		Check: &structs.IntentionQueryCheck{
			SourceName:      "bar",
			DestinationName: "qux",
			SourceType:      structs.IntentionSourceConsul,
		},
	}
	var resp structs.IntentionQueryCheckResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Intention.Check", req, &resp))
	require.True(t, resp.Allowed)
}

// Test the Check method defaults to deny with allowlist ACLs.
func TestIntentionCheck_defaultACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	waitForLeaderEstablishment(t, s1)

	// Check
	req := &structs.IntentionQueryRequest{
		Datacenter: "dc1",
		Check: &structs.IntentionQueryCheck{
			SourceName:      "bar",
			DestinationName: "qux",
			SourceType:      structs.IntentionSourceConsul,
		},
	}
	req.Token = "root"
	var resp structs.IntentionQueryCheckResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Intention.Check", req, &resp))
	require.False(t, resp.Allowed)
}

// Test the Check method defaults to deny with denylist ACLs.
func TestIntentionCheck_defaultACLAllow(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "allow"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	waitForLeaderEstablishment(t, s1)

	// Check
	req := &structs.IntentionQueryRequest{
		Datacenter: "dc1",
		Check: &structs.IntentionQueryCheck{
			SourceName:      "bar",
			DestinationName: "qux",
			SourceType:      structs.IntentionSourceConsul,
		},
	}
	req.Token = "root"
	var resp structs.IntentionQueryCheckResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Intention.Check", req, &resp))
	require.True(t, resp.Allowed)
}

// Test the Check method requires service:read permission.
func TestIntentionCheck_aclDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	waitForLeaderEstablishment(t, s1)

	rules := `
service "bar" {
	policy = "read"
}`
	token := createToken(t, codec, rules)

	// Check
	req := &structs.IntentionQueryRequest{
		Datacenter: "dc1",
		Check: &structs.IntentionQueryCheck{
			SourceName:      "qux",
			DestinationName: "baz",
			SourceType:      structs.IntentionSourceConsul,
		},
	}
	req.Token = token
	var resp structs.IntentionQueryCheckResponse
	err := msgpackrpc.CallWithCodec(codec, "Intention.Check", req, &resp)
	require.True(t, acl.IsErrPermissionDenied(err))
}

// Test the Check method returns allow/deny properly.
func TestIntentionCheck_match(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, srv, codec := testACLServerWithConfig(t, nil, false)
	waitForLeaderEstablishment(t, srv)

	token, err := upsertTestTokenWithPolicyRules(codec, TestDefaultInitialManagementToken, "dc1", `service "api" { policy = "read" }`)
	require.NoError(t, err)

	// Create some intentions
	{
		insert := [][]string{
			{"web", "db"},
			{"api", "db"},
			{"web", "api"},
		}

		for _, v := range insert {
			ixn := structs.IntentionRequest{
				Datacenter: "dc1",
				Op:         structs.IntentionOpCreate,
				Intention: &structs.Intention{
					SourceNS:        "default",
					SourceName:      v[0],
					DestinationNS:   "default",
					DestinationName: v[1],
					Action:          structs.IntentionActionAllow,
				},
				WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
			}
			// Create
			var reply string
			require.NoError(t, msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply))
		}
	}

	// Check
	req := &structs.IntentionQueryRequest{
		Datacenter: "dc1",
		Check: &structs.IntentionQueryCheck{
			SourceNS:        "default",
			SourceName:      "web",
			DestinationNS:   "default",
			DestinationName: "api",
			SourceType:      structs.IntentionSourceConsul,
		},
		QueryOptions: structs.QueryOptions{Token: token.SecretID},
	}
	var resp structs.IntentionQueryCheckResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Intention.Check", req, &resp))
	require.True(t, resp.Allowed)

	// Test no match for sanity
	{
		req := &structs.IntentionQueryRequest{
			Datacenter: "dc1",
			Check: &structs.IntentionQueryCheck{
				SourceNS:        "default",
				SourceName:      "db",
				DestinationNS:   "default",
				DestinationName: "api",
				SourceType:      structs.IntentionSourceConsul,
			},
			QueryOptions: structs.QueryOptions{Token: token.SecretID},
		}
		var resp structs.IntentionQueryCheckResponse
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Intention.Check", req, &resp))
		require.False(t, resp.Allowed)
	}
}

func TestEqualStringMaps(t *testing.T) {
	m1 := map[string]string{
		"foo": "a",
	}
	m2 := map[string]string{
		"foo": "a",
		"bar": "b",
	}
	var m3 map[string]string

	m4 := map[string]string{
		"dog": "",
	}

	m5 := map[string]string{
		"cat": "",
	}

	tests := []struct {
		a      map[string]string
		b      map[string]string
		result bool
	}{
		{m1, m1, true},
		{m2, m2, true},
		{m1, m2, false},
		{m2, m1, false},
		{m2, m2, true},
		{m3, m1, false},
		{m3, m3, true},
		{m4, m5, false},
	}

	for i, test := range tests {
		actual := equalStringMaps(test.a, test.b)
		if actual != test.result {
			t.Fatalf("case %d, expected %v, got %v", i, test.result, actual)
		}
	}
}
