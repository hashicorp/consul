package consul

import (
	"os"
	"testing"

	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/consul/testutil"
	"github.com/hashicorp/net-rpc-msgpackrpc"
)

func TestQuery_Apply(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testutil.WaitForLeader(t, s1.RPC, "dc1")

	arg := structs.QueryRequest{
		Datacenter: "dc1",
		Op:         structs.QueryCreate,
		Query: structs.PreparedQuery{
			Service: structs.ServiceQuery{
				Service: "redis",
			},
		},
	}
	var reply string
	if err := msgpackrpc.CallWithCodec(codec, "Query.Apply", &arg, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}
}
