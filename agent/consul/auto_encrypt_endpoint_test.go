package consul

import (
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	msgpackrpc "github.com/hashicorp/consul-net-rpc/net-rpc-msgpackrpc"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/tlsutil"
)

func TestAutoEncryptSign(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	type test struct {
		Name      string
		Config    tlsutil.Config
		ConnError bool
		RPCError  bool
		Cert      string
		Key       string
	}
	root := "../../test/ca/root.cer"
	badRoot := "../../test/ca_path/cert1.crt"

	tests := []test{
		{Name: "Works with defaults", Config: tlsutil.Config{}, ConnError: false},
		{Name: "Works with good root", Config: tlsutil.Config{InternalRPC: tlsutil.ProtocolConfig{CAFile: root}}, ConnError: false},
		{Name: "VerifyOutgoing fails because of bad root", Config: tlsutil.Config{InternalRPC: tlsutil.ProtocolConfig{CAFile: badRoot}}, ConnError: true},
		{Name: "VerifyServerHostname fails", Config: tlsutil.Config{InternalRPC: tlsutil.ProtocolConfig{CAFile: root, VerifyServerHostname: true}}, ConnError: false, RPCError: true},
		{Name: "VerifyServerHostname succeeds", Cert: "../../test/key/ourdomain_server.cer", Key: "../../test/key/ourdomain_server.key",
			Config: tlsutil.Config{InternalRPC: tlsutil.ProtocolConfig{VerifyServerHostname: true, CAFile: root}}, ConnError: false, RPCError: false},
	}

	for i, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			cert := test.Cert
			key := test.Key
			if cert == "" {
				cert = "../../test/key/ourdomain.cer"
			}
			if key == "" {
				key = "../../test/key/ourdomain.key"
			}
			dir, s := testServerWithConfig(t, func(c *Config) {
				c.AutoEncryptAllowTLS = true
				c.PrimaryDatacenter = "dc1"
				c.Bootstrap = true
				c.TLSConfig.InternalRPC.CAFile = root
				c.TLSConfig.InternalRPC.VerifyOutgoing = true
				c.TLSConfig.InternalRPC.CertFile = cert
				c.TLSConfig.InternalRPC.KeyFile = key
			})
			defer os.RemoveAll(dir)
			defer s.Shutdown()
			testrpc.WaitForLeader(t, s.RPC, "dc1")

			info := fmt.Sprintf("case %d", i)

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
			dnsNames := []string{"localhost"}
			ipAddresses := []net.IP{net.ParseIP("127.0.0.1")}
			csr, err := connect.CreateCSR(id, pk, dnsNames, ipAddresses)
			require.NoError(t, err, info)
			require.NotEmpty(t, csr, info)
			args := &structs.CASignRequest{
				Datacenter: "dc1",
				CSR:        csr,
			}

			cfg := test.Config
			cfg.AutoTLS = true
			cfg.Domain = "consul"
			codec, err := insecureRPCClient(s, cfg)
			if test.ConnError {
				require.Error(t, err, info)
				return
			}

			require.NoError(t, err, info)
			var reply structs.SignedResponse
			err = msgpackrpc.CallWithCodec(codec, "AutoEncrypt.Sign", args, &reply)
			codec.Close()
			if test.RPCError {
				require.Error(t, err, info)
				return
			}
			require.NoError(t, err, info)

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

			// Verify SANs
			require.Equal(t, dnsNames, leaf.DNSNames)
			require.Len(t, leaf.IPAddresses, 1)
			require.True(t, ipAddresses[0].Equal(leaf.IPAddresses[0]))

			// Verify other fields
			require.Equal(t, "uuid", reply.IssuedCert.Agent, info)
			require.Len(t, reply.ManualCARoots, 1, info)
			require.Len(t, reply.ConnectCARoots.Roots, 1, info)
		})
	}
}
