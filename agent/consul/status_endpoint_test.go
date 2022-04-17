package consul

import (
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	msgpackrpc "github.com/hashicorp/consul-net-rpc/net-rpc-msgpackrpc"
	"github.com/hashicorp/consul-net-rpc/net/rpc"

	"github.com/hashicorp/consul/agent/pool"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/tlsutil"
)

func rpcClient(t *testing.T, s *Server) rpc.ClientCodec {
	codec, err := rpcClientNoClose(s)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	t.Cleanup(func() { codec.Close() })
	return codec
}

func rpcClientNoClose(s *Server) (rpc.ClientCodec, error) {
	addr := s.config.RPCAdvertise
	conn, err := net.DialTimeout("tcp", addr.String(), time.Second)
	if err != nil {
		return nil, err
	}

	// Write the Consul RPC byte to set the mode
	conn.Write([]byte{byte(pool.RPCConsul)})
	codec := msgpackrpc.NewCodecFromHandle(true, true, conn, structs.MsgpackHandle)
	return codec, nil
}

func insecureRPCClient(s *Server, c tlsutil.Config) (rpc.ClientCodec, error) {
	addr := s.config.RPCAdvertise
	configurator, err := tlsutil.NewConfigurator(c, s.logger)
	if err != nil {
		return nil, err
	}
	wrapper := configurator.OutgoingRPCWrapper()
	if wrapper == nil {
		return nil, err
	}
	d := &net.Dialer{Timeout: time.Second}
	conn, err := d.Dial("tcp", addr.String())
	if err != nil {
		return nil, err
	}
	// Switch the connection into TLS mode
	if _, err = conn.Write([]byte{byte(pool.RPCTLSInsecure)}); err != nil {
		conn.Close()
		return nil, err
	}

	// Wrap the connection in a TLS client
	tlsConn, err := wrapper(s.config.Datacenter, conn)
	if err != nil {
		conn.Close()
		return nil, err
	}
	conn = tlsConn

	return msgpackrpc.NewCodecFromHandle(true, true, conn, structs.MsgpackHandle), nil
}

func TestStatusLeader(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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
