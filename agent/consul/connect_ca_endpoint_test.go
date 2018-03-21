package consul

import (
	"crypto/x509"
	"os"
	"testing"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/stretchr/testify/assert"
)

// Test listing root CAs.
func TestConnectCARoots(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Insert some CAs
	state := s1.fsm.State()
	ca1 := connect.TestCA(t, nil)
	ca2 := connect.TestCA(t, nil)
	ca2.Active = false
	ok, err := state.CARootSetCAS(1, 0, []*structs.CARoot{ca1, ca2})
	assert.True(ok)
	assert.Nil(err)

	// Request
	args := &structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	var reply structs.IndexedCARoots
	assert.Nil(msgpackrpc.CallWithCodec(codec, "ConnectCA.Roots", args, &reply))

	// Verify
	assert.Equal(ca1.ID, reply.ActiveRootID)
	assert.Len(reply.Roots, 2)
	for _, r := range reply.Roots {
		// These must never be set, for security
		assert.Equal("", r.SigningCert)
		assert.Equal("", r.SigningKey)
	}
}

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
	ca := connect.TestCA(t, nil)
	ok, err := state.CARootSetCAS(1, 0, []*structs.CARoot{ca})
	assert.True(ok)
	assert.Nil(err)

	// Generate a CSR and request signing
	args := &structs.CASignRequest{
		Datacenter: "dc01",
		CSR:        connect.TestCSR(t, connect.TestSpiffeIDService(t, "web")),
	}
	var reply structs.IssuedCert
	assert.Nil(msgpackrpc.CallWithCodec(codec, "ConnectCA.Sign", args, &reply))

	// Verify that the cert is signed by the CA
	roots := x509.NewCertPool()
	assert.True(roots.AppendCertsFromPEM([]byte(ca.RootCert)))
	leaf, err := connect.ParseCert(reply.Cert)
	assert.Nil(err)
	_, err = leaf.Verify(x509.VerifyOptions{
		Roots: roots,
	})
	assert.Nil(err)
}
