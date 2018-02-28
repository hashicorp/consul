package consul

import (
	"os"
	"reflect"
	"testing"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/net-rpc-msgpackrpc"
)

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
