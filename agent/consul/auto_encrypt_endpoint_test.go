package consul

import (
	"crypto/x509"
	"net"
	"os"
	"testing"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/tlsutil"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test CA signing
func TestAutoEncryptSign(t *testing.T) {
	t.Parallel()

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Generate a CSR and request signing
	id := &connect.SpiffeIDAgent{"a1.consul", "uuid-a1"}
	csr, _, err := tlsutil.GenerateCSR(id, []string{"localhost"}, []net.IP{net.ParseIP("123.234.243.213")})
	require.Nil(t, err)
	require.NotEmpty(t, csr)
	args := &structs.CASignRequest{
		Datacenter: "dc1",
		CSR:        csr,
	}
	var reply structs.IssuedCert
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "AutoEncrypt.Sign", args, &reply))

	// Get the current CA
	state := s1.fsm.State()
	_, ca, err := state.CARootActive(nil)
	require.NoError(t, err)

	// Verify that the cert is signed by the CA
	roots := x509.NewCertPool()
	assert.True(t, roots.AppendCertsFromPEM([]byte(ca.RootCert)))
	leaf, err := connect.ParseCert(reply.CertPEM)
	require.NoError(t, err)
	_, err = leaf.Verify(x509.VerifyOptions{
		Roots: roots,
	})
	require.NoError(t, err)

	// Verify other fields
	require.Empty(t, reply.Service)
}
