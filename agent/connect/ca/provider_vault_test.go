package ca

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/freeport"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	vaultapi "github.com/hashicorp/vault/api"
	"github.com/stretchr/testify/require"
)

func TestVaultCAProvider_VaultTLSConfig(t *testing.T) {
	config := &structs.VaultCAProviderConfig{
		CAFile:        "/capath/ca.pem",
		CAPath:        "/capath/",
		CertFile:      "/certpath/cert.pem",
		KeyFile:       "/certpath/key.pem",
		TLSServerName: "server.name",
		TLSSkipVerify: true,
	}
	tlsConfig := vaultTLSConfig(config)
	require := require.New(t)
	require.Equal(config.CAFile, tlsConfig.CACert)
	require.Equal(config.CAPath, tlsConfig.CAPath)
	require.Equal(config.CertFile, tlsConfig.ClientCert)
	require.Equal(config.KeyFile, tlsConfig.ClientKey)
	require.Equal(config.TLSServerName, tlsConfig.TLSServerName)
	require.Equal(config.TLSSkipVerify, tlsConfig.Insecure)
}

func TestVaultCAProvider_Bootstrap(t *testing.T) {
	t.Parallel()

	if skipIfVaultNotPresent(t) {
		return
	}

	provider, testVault := testVaultProvider(t)
	defer testVault.Stop()
	client := testVault.client

	require := require.New(t)

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
		require.Len(parsed.URIs, 1)
		require.Equal(parsed.URIs[0].String(), fmt.Sprintf("spiffe://%s.consul", provider.clusterId))
	}
}

func TestVaultCAProvider_SignLeaf(t *testing.T) {
	t.Parallel()

	if skipIfVaultNotPresent(t) {
		return
	}

	require := require.New(t)
	provider, testVault := testVaultProviderWithConfig(t, true, map[string]interface{}{
		"LeafCertTTL": "1h",
	})
	defer testVault.Stop()

	spiffeService := &connect.SpiffeIDService{
		Host:       "node1",
		Namespace:  "default",
		Datacenter: "dc1",
		Service:    "foo",
	}

	// Generate a leaf cert for the service.
	var firstSerial uint64
	{
		raw, _ := connect.TestCSR(t, spiffeService)

		csr, err := connect.ParseCSR(raw)
		require.NoError(err)

		cert, err := provider.Sign(csr)
		require.NoError(err)

		parsed, err := connect.ParseCert(cert)
		require.NoError(err)
		require.Equal(parsed.URIs[0], spiffeService.URI())
		firstSerial = parsed.SerialNumber.Uint64()

		// Ensure the cert is valid now and expires within the correct limit.
		now := time.Now()
		require.True(parsed.NotAfter.Sub(now) < time.Hour)
		require.True(parsed.NotBefore.Before(now))
	}

	// Generate a new cert for another service and make sure
	// the serial number is unique.
	spiffeService.Service = "bar"
	{
		raw, _ := connect.TestCSR(t, spiffeService)

		csr, err := connect.ParseCSR(raw)
		require.NoError(err)

		cert, err := provider.Sign(csr)
		require.NoError(err)

		parsed, err := connect.ParseCert(cert)
		require.NoError(err)
		require.Equal(parsed.URIs[0], spiffeService.URI())
		require.NotEqual(firstSerial, parsed.SerialNumber.Uint64())

		// Ensure the cert is valid now and expires within the correct limit.
		require.True(time.Until(parsed.NotAfter) < time.Hour)
		require.True(parsed.NotBefore.Before(time.Now()))
	}
}

func TestVaultCAProvider_CrossSignCA(t *testing.T) {
	t.Parallel()

	if skipIfVaultNotPresent(t) {
		return
	}

	provider1, testVault1 := testVaultProvider(t)
	defer testVault1.Stop()

	provider2, testVault2 := testVaultProvider(t)
	defer testVault2.Stop()

	testCrossSignProviders(t, provider1, provider2)
}

func TestVaultProvider_SignIntermediate(t *testing.T) {
	t.Parallel()

	if skipIfVaultNotPresent(t) {
		return
	}

	provider1, testVault1 := testVaultProvider(t)
	defer testVault1.Stop()

	provider2, testVault2 := testVaultProviderWithConfig(t, false, nil)
	defer testVault2.Stop()

	testSignIntermediateCrossDC(t, provider1, provider2)
}

func TestVaultProvider_SignIntermediateConsul(t *testing.T) {
	t.Parallel()

	if skipIfVaultNotPresent(t) {
		return
	}

	// primary = Vault, secondary = Consul
	t.Run("pri=vault,sec=consul", func(t *testing.T) {
		provider1, testVault1 := testVaultProviderWithConfig(t, true, nil)
		defer testVault1.Stop()

		conf := testConsulCAConfig()
		delegate := newMockDelegate(t, conf)
		provider2 := &ConsulProvider{Delegate: delegate}
		require.NoError(t, provider2.Configure(conf.ClusterID, false, conf.Config))

		testSignIntermediateCrossDC(t, provider1, provider2)
	})

	// primary = Consul, secondary = Vault
	t.Run("pri=consul,sec=vault", func(t *testing.T) {
		conf := testConsulCAConfig()
		delegate := newMockDelegate(t, conf)
		provider1 := &ConsulProvider{Delegate: delegate}
		require.NoError(t, provider1.Configure(conf.ClusterID, true, conf.Config))
		require.NoError(t, provider1.GenerateRoot())

		provider2, testVault2 := testVaultProviderWithConfig(t, false, nil)
		defer testVault2.Stop()

		testSignIntermediateCrossDC(t, provider1, provider2)
	})
}

func testVaultProvider(t *testing.T) (*VaultProvider, *testVaultServer) {
	return testVaultProviderWithConfig(t, true, nil)
}

func testVaultProviderWithConfig(t *testing.T, isRoot bool, rawConf map[string]interface{}) (*VaultProvider, *testVaultServer) {
	testVault, err := runTestVault()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	testVault.WaitUntilReady(t)

	conf := map[string]interface{}{
		"Address":             testVault.addr,
		"Token":               testVault.rootToken,
		"RootPKIPath":         "pki-root/",
		"IntermediatePKIPath": "pki-intermediate/",
		// Tests duration parsing after msgpack type mangling during raft apply.
		"LeafCertTTL": []uint8("72h"),
	}
	for k, v := range rawConf {
		conf[k] = v
	}

	provider := &VaultProvider{}

	if err := provider.Configure("asdf", isRoot, conf); err != nil {
		testVault.Stop()
		t.Fatalf("err: %v", err)
	}
	if isRoot {
		if err = provider.GenerateRoot(); err != nil {
			testVault.Stop()
			t.Fatalf("err: %v", err)
		}
		if _, err := provider.GenerateIntermediate(); err != nil {
			testVault.Stop()
			t.Fatalf("err: %v", err)
		}
	}

	return provider, testVault
}

var printedVaultVersion sync.Once

// skipIfVaultNotPresent skips the test and returns true if vault is not found
func skipIfVaultNotPresent(t *testing.T) bool {
	vaultBinaryName := os.Getenv("VAULT_BINARY_NAME")
	if vaultBinaryName == "" {
		vaultBinaryName = "vault"
	}

	path, err := exec.LookPath(vaultBinaryName)
	if err != nil || path == "" {
		t.Skipf("%q not found on $PATH - download and install to run this test", vaultBinaryName)
		return true
	}
	return false
}

func runTestVault() (*testVaultServer, error) {
	vaultBinaryName := os.Getenv("VAULT_BINARY_NAME")
	if vaultBinaryName == "" {
		vaultBinaryName = "vault"
	}

	path, err := exec.LookPath(vaultBinaryName)
	if err != nil || path == "" {
		return nil, fmt.Errorf("%q not found on $PATH", vaultBinaryName)
	}

	ports := freeport.Get(2)

	var (
		clientAddr  = fmt.Sprintf("127.0.0.1:%d", ports[0])
		clusterAddr = fmt.Sprintf("127.0.0.1:%d", ports[1])
	)

	const token = "root"

	client, err := vaultapi.NewClient(&vaultapi.Config{
		Address: "http://" + clientAddr,
	})
	if err != nil {
		return nil, err
	}
	client.SetToken(token)

	args := []string{
		"server",
		"-dev",
		"-dev-root-token-id",
		token,
		"-dev-listen-address",
		clientAddr,
		"-address",
		clusterAddr,
	}

	cmd := exec.Command(vaultBinaryName, args...)
	cmd.Stdout = ioutil.Discard
	cmd.Stderr = ioutil.Discard
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return &testVaultServer{
		rootToken: token,
		addr:      "http://" + clientAddr,
		cmd:       cmd,
		client:    client,
	}, nil
}

type testVaultServer struct {
	rootToken string
	addr      string
	cmd       *exec.Cmd
	client    *vaultapi.Client
}

func (v *testVaultServer) WaitUntilReady(t *testing.T) {
	var version string
	retry.Run(t, func(r *retry.R) {
		resp, err := v.client.Sys().Health()
		if err != nil {
			r.Fatalf("err: %v", err)
		}
		if !resp.Initialized {
			r.Fatalf("vault server is not initialized")
		}
		if resp.Sealed {
			r.Fatalf("vault server is sealed")
		}
		version = resp.Version
	})
	printedVaultVersion.Do(func() {
		fmt.Fprintf(os.Stderr, "[INFO] agent/connect/ca: testing with vault server version: %s\n", version)
	})
}

func (v *testVaultServer) Stop() error {
	// There was no process
	if v.cmd == nil {
		return nil
	}

	if v.cmd.Process != nil {
		if err := v.cmd.Process.Signal(os.Interrupt); err != nil {
			return fmt.Errorf("failed to kill vault server: %v", err)
		}
	}

	// wait for the process to exit to be sure that the data dir can be
	// deleted on all platforms.
	return v.cmd.Wait()
}
