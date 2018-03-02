package consul

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/net-rpc-msgpackrpc"
)

// Test basic creation
func TestIntentionApply_new(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Setup a basic record to create
	ixn := structs.IntentionRequest{
		Datacenter: "dc1",
		Op:         structs.IntentionOpCreate,
		Intention: &structs.Intention{
			SourceName: "test",
		},
	}
	var reply string

	// Create
	if err := msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}
	if reply == "" {
		t.Fatal("reply should be non-empty")
	}

	// Read
	ixn.Intention.ID = reply
	{
		req := &structs.IntentionQueryRequest{
			Datacenter:  "dc1",
			IntentionID: ixn.Intention.ID,
		}
		var resp structs.IndexedIntentions
		if err := msgpackrpc.CallWithCodec(codec, "Intention.Get", req, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(resp.Intentions) != 1 {
			t.Fatalf("bad: %v", resp)
		}
		actual := resp.Intentions[0]
		if resp.Index != actual.ModifyIndex {
			t.Fatalf("bad index: %d", resp.Index)
		}
		actual.CreateIndex, actual.ModifyIndex = 0, 0
		if !reflect.DeepEqual(actual, ixn.Intention) {
			t.Fatalf("bad: %v", actual)
		}
	}
}

// Shouldn't be able to create with an ID set
func TestIntentionApply_createWithID(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Setup a basic record to create
	ixn := structs.IntentionRequest{
		Datacenter: "dc1",
		Op:         structs.IntentionOpCreate,
		Intention: &structs.Intention{
			ID:         generateUUID(),
			SourceName: "test",
		},
	}
	var reply string

	// Create
	err := msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply)
	if err == nil || !strings.Contains(err.Error(), "ID must be empty") {
		t.Fatalf("bad: %v", err)
	}
}

// Test basic updating
func TestIntentionApply_updateGood(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Setup a basic record to create
	ixn := structs.IntentionRequest{
		Datacenter: "dc1",
		Op:         structs.IntentionOpCreate,
		Intention: &structs.Intention{
			SourceName: "test",
		},
	}
	var reply string

	// Create
	if err := msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}
	if reply == "" {
		t.Fatal("reply should be non-empty")
	}

	// Update
	ixn.Op = structs.IntentionOpUpdate
	ixn.Intention.ID = reply
	ixn.Intention.SourceName = "bar"
	if err := msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Read
	ixn.Intention.ID = reply
	{
		req := &structs.IntentionQueryRequest{
			Datacenter:  "dc1",
			IntentionID: ixn.Intention.ID,
		}
		var resp structs.IndexedIntentions
		if err := msgpackrpc.CallWithCodec(codec, "Intention.Get", req, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(resp.Intentions) != 1 {
			t.Fatalf("bad: %v", resp)
		}
		actual := resp.Intentions[0]
		actual.CreateIndex, actual.ModifyIndex = 0, 0
		if !reflect.DeepEqual(actual, ixn.Intention) {
			t.Fatalf("bad: %v", actual)
		}
	}
}

// Shouldn't be able to update a non-existent intention
func TestIntentionApply_updateNonExist(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

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
	if err == nil || !strings.Contains(err.Error(), "Cannot modify non-existent intention") {
		t.Fatalf("bad: %v", err)
	}
}

// Test basic deleting
func TestIntentionApply_deleteGood(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Setup a basic record to create
	ixn := structs.IntentionRequest{
		Datacenter: "dc1",
		Op:         structs.IntentionOpCreate,
		Intention: &structs.Intention{
			SourceName: "test",
		},
	}
	var reply string

	// Create
	if err := msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}
	if reply == "" {
		t.Fatal("reply should be non-empty")
	}

	// Delete
	ixn.Op = structs.IntentionOpDelete
	ixn.Intention.ID = reply
	if err := msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Read
	ixn.Intention.ID = reply
	{
		req := &structs.IntentionQueryRequest{
			Datacenter:  "dc1",
			IntentionID: ixn.Intention.ID,
		}
		var resp structs.IndexedIntentions
		err := msgpackrpc.CallWithCodec(codec, "Intention.Get", req, &resp)
		if err == nil || !strings.Contains(err.Error(), ErrIntentionNotFound.Error()) {
			t.Fatalf("err: %v", err)
		}
	}
}

func TestIntentionList(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	codec := rpcClient(t, s1)
	defer codec.Close()
	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Test with no intentions inserted yet
	{
		req := &structs.DCSpecificRequest{
			Datacenter: "dc1",
		}
		var resp structs.IndexedIntentions
		if err := msgpackrpc.CallWithCodec(codec, "Intention.List", req, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(resp.Intentions) != 0 {
			t.Fatalf("bad: %v", resp)
		}
	}
}

// Test basic matching. We don't need to exhaustively test inputs since this
// is tested in the agent/consul/state package.
func TestIntentionMatch_good(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Create some records
	{
		insert := [][]string{
			{"foo", "*"},
			{"foo", "bar"},
			{"foo", "baz"}, // shouldn't match
			{"bar", "bar"}, // shouldn't match
			{"bar", "*"},   // shouldn't match
			{"*", "*"},
		}

		for _, v := range insert {
			ixn := structs.IntentionRequest{
				Datacenter: "dc1",
				Op:         structs.IntentionOpCreate,
				Intention: &structs.Intention{
					SourceNS:        "default",
					SourceName:      "test",
					DestinationNS:   v[0],
					DestinationName: v[1],
				},
			}

			// Create
			var reply string
			if err := msgpackrpc.CallWithCodec(codec, "Intention.Apply", &ixn, &reply); err != nil {
				t.Fatalf("err: %v", err)
			}
		}
	}

	// Match
	req := &structs.IntentionQueryRequest{
		Datacenter: "dc1",
		Match: &structs.IntentionQueryMatch{
			Type: structs.IntentionMatchDestination,
			Entries: []structs.IntentionMatchEntry{
				{
					Namespace: "foo",
					Name:      "bar",
				},
			},
		},
	}
	var resp structs.IndexedIntentionMatches
	if err := msgpackrpc.CallWithCodec(codec, "Intention.Match", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(resp.Matches) != 1 {
		t.Fatalf("bad: %#v", resp.Matches)
	}

	expected := [][]string{{"foo", "bar"}, {"foo", "*"}, {"*", "*"}}
	var actual [][]string
	for _, ixn := range resp.Matches[0] {
		actual = append(actual, []string{ixn.DestinationNS, ixn.DestinationName})
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("bad (got, wanted):\n\n%#v\n\n%#v", actual, expected)
	}
}
