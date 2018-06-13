package ca

import (
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/vault/builtin/logical/pki"
	vaulthttp "github.com/hashicorp/vault/http"
	"github.com/hashicorp/vault/logical"
	"github.com/hashicorp/vault/vault"
	"github.com/stretchr/testify/require"
)

func testVaultCluster(t *testing.T) (*VaultProvider, *vault.TestCluster) {
	coreConfig := &vault.CoreConfig{
		LogicalBackends: map[string]logical.Factory{
			"pki": pki.Factory,
		},
	}
	cluster := vault.NewTestCluster(t, coreConfig, &vault.TestClusterOptions{
		HandlerFunc: vaulthttp.Handler,
		NumCores:    1,
	})
	cluster.Start()

	client := cluster.Cores[0].Client

	provider, err := NewVaultProvider(map[string]interface{}{
		"Address":             client.Address(),
		"Token":               cluster.RootToken,
		"RootPKIPath":         "pki-root/",
		"IntermediatePKIPath": "pki-intermediate/",
	}, "asdf", client)
	if err != nil {
		t.Fatal(err)
	}

	return provider, cluster
}

func TestVaultCAProvider_Bootstrap(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	provider, vaultCluster := testVaultCluster(t)
	defer vaultCluster.Cleanup()
	client := vaultCluster.Cores[0].Client

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
		req := client.NewRequest("GET", "v1/"+tc.backendPath+"ca/pem")
		resp, err := client.RawRequest(req)
		require.NoError(err)
		bytes, err := ioutil.ReadAll(resp.Body)
		require.NoError(err)
		require.Equal(cert, string(bytes))

		// Should be a valid cert
		parsed, err := connect.ParseCert(cert)
		require.NoError(err)
		require.Equal(parsed.URIs[0].String(), fmt.Sprintf("spiffe://%s.consul", provider.clusterId))
	}
}
