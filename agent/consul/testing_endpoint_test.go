package consul

import (
	"os"
	"testing"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/stretchr/testify/assert"
)

// Test setting the CAs
func TestTestConnectCASetRoots(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Prepare
	ca1 := connect.TestCA(t, nil)
	ca2 := connect.TestCA(t, nil)
	ca2.Active = false

	// Request
	args := []*structs.CARoot{ca1, ca2}
	var reply interface{}
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Test.ConnectCASetRoots", args, &reply))

	// Verify they're there
	state := s1.fsm.State()
	_, actual, err := state.CARoots(nil)
	assert.Nil(err)
	assert.Len(actual, 2)
}
