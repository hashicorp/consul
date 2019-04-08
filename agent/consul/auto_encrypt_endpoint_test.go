package consul

import (
	"crypto/x509"
	"net"
	"net/url"
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

func TestAutoEncryptSign(t *testing.T) {
	t.Parallel()

	dir, s := testServerWithConfig(t, func(c *Config) {
		c.Bootstrap = true
		c.CAFile = "../../test/client_certs/rootca.crt"
		c.CertFile = "../../test/client_certs/server.crt"
		c.KeyFile = "../../test/client_certs/server.key"
	})
	defer os.RemoveAll(dir)
	defer s.Shutdown()
	c := tlsutil.Config{}
	codec := insecureRPCClient(t, s, c)
	defer codec.Close()

	testrpc.WaitForLeader(t, s.RPC, "dc1")

	// Generate a CSR and request signing
	uri, err := url.Parse("spiffe://a.consul/agent/uuid")
	require.NoError(t, err)
	csr, _, err := tlsutil.GenerateCSR(uri, []string{"localhost"}, []net.IP{net.ParseIP("123.234.243.213")})
	require.Nil(t, err)
	require.NotEmpty(t, csr)
	args := &structs.CASignRequest{
		Datacenter: "dc1",
		CSR:        csr,
	}
	var reply structs.SignResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "AutoEncrypt.Sign", args, &reply))

	// Get the current CA
	state := s.fsm.State()
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
	require.Equal(t, "uuid", reply.Agent)
	require.Len(t, reply.RootCAs, 2)
}
