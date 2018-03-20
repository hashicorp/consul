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

// Test CA signing
//
// NOTE(mitchellh): Just testing the happy path and not all the other validation
// issues because the internals of this method will probably be gutted for the
// CA plugins then we can just test mocks.
func TestConnectCASign(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Insert a CA
	state := s1.fsm.State()
	assert.Nil(state.CARootSet(1, connect.TestCA(t, nil)))

	// Generate a CSR and request signing
	args := &structs.CASignRequest{
		Datacenter: "dc01",
		CSR:        connect.TestCSR(t, connect.TestSpiffeIDService(t, "web")),
	}
	var reply interface{}
	assert.Nil(msgpackrpc.CallWithCodec(codec, "ConnectCA.Sign", args, &reply))
}
