package consul

import (
	"github.com/hashicorp/consul/testutil"
	"github.com/hashicorp/go-msgpack/codec"
	"net"
	"net/rpc"
	"os"
	"testing"
	"time"
)

func rpcClient(t *testing.T, s *Server) *rpc.Client {
	addr := s.config.RPCAddr
	conn, err := net.DialTimeout("tcp", addr.String(), time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// Write the Consul RPC byte to set the mode
	conn.Write([]byte{byte(rpcConsul)})

	cc := codec.GoRpc.ClientCodec(conn, msgpackHandle)
	return rpc.NewClientWithCodec(cc)
}

func TestStatusLeader(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client := rpcClient(t, s1)
	defer client.Close()

	arg := struct{}{}
	var leader string
	if err := client.Call("Status.Leader", arg, &leader); err != nil {
		t.Fatalf("err: %v", err)
	}
	if leader != "" {
		t.Fatalf("unexpected leader: %v", leader)
	}

	testutil.WaitForLeader(t, client.Call, "dc1")

	if err := client.Call("Status.Leader", arg, &leader); err != nil {
		t.Fatalf("err: %v", err)
	}
	if leader == "" {
		t.Fatalf("no leader")
	}
}

func TestStatusPeers(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client := rpcClient(t, s1)
	defer client.Close()

	arg := struct{}{}
	var peers []string
	if err := client.Call("Status.Peers", arg, &peers); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(peers) != 1 {
		t.Fatalf("no peers: %v", peers)
	}
}
