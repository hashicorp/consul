package consul

import (
	"net"
	"net/rpc"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/pool"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/tlsutil"
	"github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/stretchr/testify/require"
)

func rpcClient(t *testing.T, s *Server) rpc.ClientCodec {
	addr := s.config.RPCAdvertise
	conn, err := net.DialTimeout("tcp", addr.String(), time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Write the Consul RPC byte to set the mode
	conn.Write([]byte{byte(pool.RPCConsul)})
	return msgpackrpc.NewClientCodec(conn)
}

func insecureRPCClient(s *Server, c tlsutil.Config) (rpc.ClientCodec, error) {
	addr := s.config.RPCAdvertise
	configurator, err := tlsutil.NewConfigurator(c, nil)
	if err != nil {
		return nil, err
	}
	wrapper := configurator.OutgoingRPCWrapper()
	if wrapper == nil {
		return nil, err
	}
	conn, _, err := pool.DialTimeoutWithRPCType(s.config.Datacenter, addr, nil, time.Second, true, wrapper, pool.RPCTLSInsecure)
	if err != nil {
		return nil, err
	}
	return msgpackrpc.NewClientCodec(conn), nil
}

func TestStatusLeader(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	arg := struct{}{}
	var leader string
	if err := msgpackrpc.CallWithCodec(codec, "Status.Leader", arg, &leader); err != nil {
		t.Fatalf("err: %v", err)
	}
	if leader != "" {
		t.Fatalf("unexpected leader: %v", leader)
	}

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")

	if err := msgpackrpc.CallWithCodec(codec, "Status.Leader", arg, &leader); err != nil {
		t.Fatalf("err: %v", err)
	}
	if leader == "" {
		t.Fatalf("no leader")
	}
}

func TestStatusLeader_ForwardDC(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerDC(t, "primary")
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	dir2, s2 := testServerDC(t, "secondary")
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	joinWAN(t, s2, s1)

	testrpc.WaitForLeader(t, s1.RPC, "secondary")
	testrpc.WaitForLeader(t, s2.RPC, "primary")

	args := structs.DCSpecificRequest{
		Datacenter: "secondary",
	}

	var out string
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Status.Leader", &args, &out))
	require.Equal(t, s2.config.RPCAdvertise.String(), out)
}

func TestStatusPeers(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	arg := struct{}{}
	var peers []string
	if err := msgpackrpc.CallWithCodec(codec, "Status.Peers", arg, &peers); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(peers) != 1 {
		t.Fatalf("no peers: %v", peers)
	}
}

func TestStatusPeers_ForwardDC(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerDC(t, "primary")
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	dir2, s2 := testServerDC(t, "secondary")
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	joinWAN(t, s2, s1)

	testrpc.WaitForLeader(t, s1.RPC, "secondary")
	testrpc.WaitForLeader(t, s2.RPC, "primary")

	args := structs.DCSpecificRequest{
		Datacenter: "secondary",
	}

	var out []string
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Status.Peers", &args, &out))
	require.Equal(t, []string{s2.config.RPCAdvertise.String()}, out)
}
