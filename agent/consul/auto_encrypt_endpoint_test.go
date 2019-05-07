package consul

import (
	"crypto/x509"
	"fmt"
	"os"
	"strings"
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

	goodRoot := "../../test/ca_path/cert1.crt"
	badRoot := "../../test/ca/root.cer"
	dir, s := testServerWithConfig(t, func(c *Config) {
		c.AutoEncryptAllowTLS = true
		c.Bootstrap = true
		c.CAFile = goodRoot
		c.CertFile = "../../test/key/ourdomain.cer"
		c.KeyFile = "../../test/key/ourdomain.key"
	})
	defer os.RemoveAll(dir)
	defer s.Shutdown()

	type variant struct {
		Config   tlsutil.Config
		RPCError bool
	}

	variants := []variant{
		{Config: tlsutil.Config{AutoEncryptTLS: true}, RPCError: false},
		{Config: tlsutil.Config{AutoEncryptTLS: true, VerifyServerHostname: false, CAFile: goodRoot}, RPCError: false},
		{Config: tlsutil.Config{AutoEncryptTLS: true, VerifyServerHostname: true, CAFile: goodRoot}, RPCError: false},
		{Config: tlsutil.Config{AutoEncryptTLS: true, VerifyServerHostname: false, CAFile: badRoot}, RPCError: false},
		{Config: tlsutil.Config{AutoEncryptTLS: true, VerifyServerHostname: true, CAFile: badRoot}, RPCError: true},
	}

	for i, v := range variants {
		info := fmt.Sprintf("case %d", i)
		codec := insecureRPCClient(t, s, v.Config)
		defer codec.Close()

		testrpc.WaitForLeader(t, s.RPC, "dc1")

		// Generate a CSR and request signing
		id := &connect.SpiffeIDAgent{
			Host:       strings.TrimSuffix("domain", "."),
			Datacenter: "dc1",
			Agent:      "uuid",
		}

		// Create a new private key
		pk, _, err := connect.GeneratePrivateKey()
		require.NoError(t, err, info)

		// Create a CSR.
		csr, err := connect.CreateCSR(id, pk)
		require.NoError(t, err, info)
		require.NotEmpty(t, csr, info)
		args := &structs.CASignRequest{
			Datacenter: "dc1",
			CSR:        csr,
		}
		var reply structs.SignResponse
		err = msgpackrpc.CallWithCodec(codec, "AutoEncrypt.Sign", args, &reply)
		if v.RPCError {
			require.Error(t, err, info)
		} else {
			require.NoError(t, err, info)
		}

		// Get the current CA
		state := s.fsm.State()
		_, ca, err := state.CARootActive(nil)
		require.NoError(t, err, info)

		// Verify that the cert is signed by the CA
		roots := x509.NewCertPool()
		assert.True(t, roots.AppendCertsFromPEM([]byte(ca.RootCert)))
		leaf, err := connect.ParseCert(reply.IssuedCert.CertPEM)
		require.NoError(t, err, info)
		_, err = leaf.Verify(x509.VerifyOptions{
			Roots: roots,
		})
		require.NoError(t, err, info)

		// Verify other fields
		require.Equal(t, "uuid", reply.IssuedCert.Agent, info)
		require.Len(t, reply.ManualCARoots, 1, info)
		require.Len(t, reply.ConnectCARoots.Roots, 1, info)
	}
}
