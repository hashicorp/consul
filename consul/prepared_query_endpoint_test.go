package consul

import (
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/consul/testutil"
	"github.com/hashicorp/net-rpc-msgpackrpc"
)

func TestPreparedQuery_Apply(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testutil.WaitForLeader(t, s1.RPC, "dc1")

	// Set up a node and service in the catalog.
	{
		arg := structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "redis",
				Tags:    []string{"master"},
				Port:    8000,
			},
		}
		var reply struct{}

		err := msgpackrpc.CallWithCodec(codec, "Catalog.Register", &arg, &reply)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Set up a bare bones query.
	arg := structs.PreparedQueryRequest{
		Datacenter: "dc1",
		Op:         structs.PreparedQueryCreate,
		Query: structs.PreparedQuery{
			Service: structs.ServiceQuery{
				Service: "redis",
			},
		},
	}
	var reply string

	// Set an ID which should fail the create.
	arg.Query.ID = "nope"
	err := msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &arg, &reply)
	if err == nil || !strings.Contains(err.Error(), "ID must be empty") {
		t.Fatalf("bad: %v", err)
	}

	// Change it to a bogus modify which should also fail.
	arg.Op = structs.PreparedQueryUpdate
	arg.Query.ID = generateUUID()
	err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &arg, &reply)
	if err == nil || !strings.Contains(err.Error(), "Cannot modify non-existent prepared query") {
		t.Fatalf("bad: %v", err)
	}

	// Fix up the ID but invalidate the query itself. This proves we call
	// parseQuery for a create, but that function is checked in detail as
	// part of another test.
	arg.Op = structs.PreparedQueryCreate
	arg.Query.ID = ""
	arg.Query.Service.Failover.NearestN = -1
	err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &arg, &reply)
	if err == nil || !strings.Contains(err.Error(), "Bad NearestN") {
		t.Fatalf("bad: %v", err)
	}

	// Fix that and make sure it propagates an error from the Raft apply.
	arg.Query.Service.Failover.NearestN = 0
	arg.Query.Service.Service = "nope"
	err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &arg, &reply)
	if err == nil || !strings.Contains(err.Error(), "invalid service") {
		t.Fatalf("bad: %v", err)
	}

	// Fix that and make sure the apply goes through.
	arg.Query.Service.Service = "redis"
	if err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &arg, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Capture the new ID and make the op an update. This should go through.
	arg.Op = structs.PreparedQueryUpdate
	arg.Query.ID = reply
	if err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &arg, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Give a bogus op and make sure it fails.
	arg.Op = "nope"
	err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &arg, &reply)
	if err == nil || !strings.Contains(err.Error(), "Unknown prepared query operation:") {
		t.Fatalf("bad: %v", err)
	}

	// Prove that an update also goes through the parseQuery validation.
	arg.Op = structs.PreparedQueryUpdate
	arg.Query.Service.Failover.NearestN = -1
	err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &arg, &reply)
	if err == nil || !strings.Contains(err.Error(), "Bad NearestN") {
		t.Fatalf("bad: %v", err)
	}

	// Sanity check - make sure there's one PQ in there.
	var queries structs.IndexedPreparedQueries
	if err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.List",
		&structs.DCSpecificRequest{Datacenter: "dc1"}, &queries); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(queries.Queries) != 1 {
		t.Fatalf("bad: %v", queries)
	}

	// Now change the op to delete; the bad query field should be ignored
	// because all we care about for a delete op is the ID.
	arg.Op = structs.PreparedQueryDelete
	if err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.Apply", &arg, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make sure there are no longer any queries.
	if err = msgpackrpc.CallWithCodec(codec, "PreparedQuery.List",
		&structs.DCSpecificRequest{Datacenter: "dc1"}, &queries); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(queries.Queries) != 0 {
		t.Fatalf("bad: %v", queries)
	}
}
