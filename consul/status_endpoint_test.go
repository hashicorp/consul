package consul

import (
	"github.com/ugorji/go/codec"
	"net"
	"net/rpc"
	"os"
	"testing"
	"time"
)

func rpcClient(t *testing.T, s *Server) *rpc.Client {
	addr := s.config.RPCAddr
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// Write the Consul RPC byte to set the mode
	conn.Write([]byte{byte(rpcConsul)})

	cc := codec.GoRpc.ClientCodec(conn, &codec.MsgpackHandle{})
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

	time.Sleep(100 * time.Millisecond)

	if err := client.Call("Status.Leader", arg, &leader); err != nil {
		t.Fatalf("err: %v", err)
	}
	if leader == "" {
		t.Fatalf("no leader")
	}
}
