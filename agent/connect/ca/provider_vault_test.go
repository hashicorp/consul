package ca

import (
	"io/ioutil"
	"net"
	"testing"

	"github.com/hashicorp/consul/agent/connect"
	vaultapi "github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/builtin/logical/pki"
	vaulthttp "github.com/hashicorp/vault/http"
	"github.com/hashicorp/vault/vault"
	"github.com/stretchr/testify/require"
)

func testVaultCluster(t *testing.T) (*VaultProvider, *vault.Core, net.Listener) {
	if err := vault.AddTestLogicalBackend("pki", pki.Factory); err != nil {
		t.Fatal(err)
	}
	core, _, token := vault.TestCoreUnsealedRaw(t)

	ln, addr := vaulthttp.TestServer(t, core)

	provider, err := NewVaultProvider(map[string]interface{}{
		"Address":             addr,
		"Token":               token,
		"RootPKIPath":         "pki-root/",
		"IntermediatePKIPath": "pki-intermediate/",
	}, "asdf")
	if err != nil {
		t.Fatal(err)
	}

	return provider, core, ln
}

func TestVaultCAProvider_Bootstrap(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	provider, core, listener := testVaultCluster(t)
	defer core.Shutdown()
	defer listener.Close()
	client, err := vaultapi.NewClient(&vaultapi.Config{
		Address: "http://" + listener.Addr().String(),
	})
	require.NoError(err)
	client.SetToken(provider.config.Token)

	cases := []struct {
		certFunc    func() (string, error)
		backendPath string
	}{
		{
			certFunc:    provider.ActiveRoot,
			backendPath: "pki-root/",
		},
		{
			certFunc:    provider.ActiveIntermediate,
			backendPath: "pki-intermediate/",
		},
	}

	// Verify the root and intermediate certs match the ones in the vault backends
	for _, tc := range cases {
		cert, err := tc.certFunc()
		require.NoError(err)
		req := client.NewRequest("GET", "/v1/"+tc.backendPath+"ca/pem")
		resp, err := client.RawRequest(req)
		require.NoError(err)
		bytes, err := ioutil.ReadAll(resp.Body)
		require.NoError(err)
		require.Equal(cert, string(bytes))

		// Should be a valid CA cert
		parsed, err := connect.ParseCert(cert)
		require.NoError(err)
		require.True(parsed.IsCA)
	}
}
