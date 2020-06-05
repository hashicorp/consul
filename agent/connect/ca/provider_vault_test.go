package ca

import (
	"crypto/x509"
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

func TestVaultCAProvider_SecondaryActiveIntermediate(t *testing.T) {
	t.Parallel()

	skipIfVaultNotPresent(t)

	provider, testVault := testVaultProviderWithConfig(t, false, nil)
	defer testVault.Stop()
	require := require.New(t)

	cert, err := provider.ActiveIntermediate()
	require.Empty(cert)
	require.NoError(err)
}

func TestVaultCAProvider_Bootstrap(t *testing.T) {
	t.Parallel()

	skipIfVaultNotPresent(t)

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
		require.Equal(fmt.Sprintf("spiffe://%s.consul", provider.clusterID), parsed.URIs[0].String())
	}
}

func assertCorrectKeyType(t *testing.T, want, certPEM string) {
	t.Helper()

	cert, err := connect.ParseCert(certPEM)
	require.NoError(t, err)

	switch want {
	case "ec":
		require.Equal(t, x509.ECDSA, cert.PublicKeyAlgorithm)
	case "rsa":
		require.Equal(t, x509.RSA, cert.PublicKeyAlgorithm)
	default:
		t.Fatal("test doesn't support key type")
	}
}

func TestVaultCAProvider_SignLeaf(t *testing.T) {
	t.Parallel()

	skipIfVaultNotPresent(t)

	for _, tc := range KeyTestCases {
		tc := tc
		t.Run(tc.Desc, func(t *testing.T) {
			require := require.New(t)
			provider, testVault := testVaultProviderWithConfig(t, true, map[string]interface{}{
				"LeafCertTTL":    "1h",
				"PrivateKeyType": tc.KeyType,
				"PrivateKeyBits": tc.KeyBits,
			})
			defer testVault.Stop()

			spiffeService := &connect.SpiffeIDService{
				Host:       "node1",
				Namespace:  "default",
				Datacenter: "dc1",
				Service:    "foo",
			}

			rootPEM, err := provider.ActiveRoot()
			require.NoError(err)
			assertCorrectKeyType(t, tc.KeyType, rootPEM)

			intPEM, err := provider.ActiveIntermediate()
			require.NoError(err)
			assertCorrectKeyType(t, tc.KeyType, intPEM)

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

				// Make sure we can validate the cert as expected.
				require.NoError(connect.ValidateLeaf(rootPEM, cert, []string{intPEM}))
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

				// Make sure we can validate the cert as expected.
				require.NoError(connect.ValidateLeaf(rootPEM, cert, []string{intPEM}))
			}
		})
	}
}

func TestVaultCAProvider_CrossSignCA(t *testing.T) {
	t.Parallel()

	skipIfVaultNotPresent(t)

	tests := CASigningKeyTypeCases()

	for _, tc := range tests {
		tc := tc
		t.Run(tc.Desc, func(t *testing.T) {
			require := require.New(t)

			if tc.SigningKeyType != tc.CSRKeyType {
				// See https://github.com/hashicorp/vault/issues/7709
				t.Skip("Vault doesn't support cross-signing different key types yet.")
			}
			provider1, testVault1 := testVaultProviderWithConfig(t, true, map[string]interface{}{
				"LeafCertTTL":    "1h",
				"PrivateKeyType": tc.SigningKeyType,
				"PrivateKeyBits": tc.SigningKeyBits,
			})
			defer testVault1.Stop()

			{
				rootPEM, err := provider1.ActiveRoot()
				require.NoError(err)
				assertCorrectKeyType(t, tc.SigningKeyType, rootPEM)

				intPEM, err := provider1.ActiveIntermediate()
				require.NoError(err)
				assertCorrectKeyType(t, tc.SigningKeyType, intPEM)
			}

			provider2, testVault2 := testVaultProviderWithConfig(t, true, map[string]interface{}{
				"LeafCertTTL":    "1h",
				"PrivateKeyType": tc.CSRKeyType,
				"PrivateKeyBits": tc.CSRKeyBits,
			})
			defer testVault2.Stop()

			{
				rootPEM, err := provider2.ActiveRoot()
				require.NoError(err)
				assertCorrectKeyType(t, tc.CSRKeyType, rootPEM)

				intPEM, err := provider2.ActiveIntermediate()
				require.NoError(err)
				assertCorrectKeyType(t, tc.CSRKeyType, intPEM)
			}

			testCrossSignProviders(t, provider1, provider2)
		})
	}
}

func TestVaultProvider_SignIntermediate(t *testing.T) {
	t.Parallel()

	skipIfVaultNotPresent(t)

	tests := CASigningKeyTypeCases()

	for _, tc := range tests {
		tc := tc
		t.Run(tc.Desc, func(t *testing.T) {
			provider1, testVault1 := testVaultProviderWithConfig(t, true, map[string]interface{}{
				"LeafCertTTL":    "1h",
				"PrivateKeyType": tc.SigningKeyType,
				"PrivateKeyBits": tc.SigningKeyBits,
			})
			defer testVault1.Stop()

			provider2, testVault2 := testVaultProviderWithConfig(t, false, map[string]interface{}{
				"LeafCertTTL":    "1h",
				"PrivateKeyType": tc.CSRKeyType,
				"PrivateKeyBits": tc.CSRKeyBits,
			})
			defer testVault2.Stop()

			testSignIntermediateCrossDC(t, provider1, provider2)
		})
	}
}

func TestVaultProvider_SignIntermediateConsul(t *testing.T) {
	t.Parallel()

	skipIfVaultNotPresent(t)

	// primary = Vault, secondary = Consul
	t.Run("pri=vault,sec=consul", func(t *testing.T) {
		provider1, testVault1 := testVaultProviderWithConfig(t, true, nil)
		defer testVault1.Stop()

		conf := testConsulCAConfig()
		delegate := newMockDelegate(t, conf)
		provider2 := TestConsulProvider(t, delegate)
		cfg := testProviderConfig(conf)
		cfg.IsPrimary = false
		cfg.Datacenter = "dc2"
		require.NoError(t, provider2.Configure(cfg))

		testSignIntermediateCrossDC(t, provider1, provider2)
	})

	// primary = Consul, secondary = Vault
	t.Run("pri=consul,sec=vault", func(t *testing.T) {
		conf := testConsulCAConfig()
		delegate := newMockDelegate(t, conf)
		provider1 := TestConsulProvider(t, delegate)
		require.NoError(t, provider1.Configure(testProviderConfig(conf)))
		require.NoError(t, provider1.GenerateRoot())

		// Ensure that we don't configure vault to try and mint leafs that
		// outlive their CA during the test (which hard fails in vault).
		intermediateCertTTL := getIntermediateCertTTL(t, conf)
		leafCertTTL := intermediateCertTTL - 4*time.Hour

		overrideConf := map[string]interface{}{
			"LeafCertTTL": []uint8(leafCertTTL.String()),
		}

		provider2, testVault2 := testVaultProviderWithConfig(t, false, overrideConf)
		defer testVault2.Stop()

		testSignIntermediateCrossDC(t, provider1, provider2)
	})
}

func getIntermediateCertTTL(t *testing.T, caConf *structs.CAConfiguration) time.Duration {
	t.Helper()

	require.NotNil(t, caConf)
	require.NotNil(t, caConf.Config)

	iface, ok := caConf.Config["IntermediateCertTTL"]
	require.True(t, ok)

	ttlBytes, ok := iface.([]uint8)
	require.True(t, ok)

	ttlString := string(ttlBytes)

	dur, err := time.ParseDuration(ttlString)
	require.NoError(t, err)
	return dur
}

func testVaultProvider(t *testing.T) (*VaultProvider, *testVaultServer) {
	return testVaultProviderWithConfig(t, true, nil)
}

func testVaultProviderWithConfig(t *testing.T, isPrimary bool, rawConf map[string]interface{}) (*VaultProvider, *testVaultServer) {
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

	cfg := ProviderConfig{
		ClusterID:  connect.TestClusterID,
		Datacenter: "dc1",
		IsPrimary:  true,
		RawConfig:  conf,
	}

	if !isPrimary {
		cfg.IsPrimary = false
		cfg.Datacenter = "dc2"
	}

	if err := provider.Configure(cfg); err != nil {
		testVault.Stop()
		t.Fatalf("err: %v", err)
	}
	if isPrimary {
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

// skipIfVaultNotPresent skips the test if the vault binary is not in PATH.
//
// These tests may be skipped in CI. They are run as part of a separate
// integration test suite.
func skipIfVaultNotPresent(t *testing.T) {
	vaultBinaryName := os.Getenv("VAULT_BINARY_NAME")
	if vaultBinaryName == "" {
		vaultBinaryName = "vault"
	}

	path, err := exec.LookPath(vaultBinaryName)
	if err != nil || path == "" {
		t.Skipf("%q not found on $PATH - download and install to run this test", vaultBinaryName)
	}
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

	ports := freeport.MustTake(2)
	returnPortsFn := func() {
		freeport.Return(ports)
	}

	var (
		clientAddr  = fmt.Sprintf("127.0.0.1:%d", ports[0])
		clusterAddr = fmt.Sprintf("127.0.0.1:%d", ports[1])
	)

	const token = "root"

	client, err := vaultapi.NewClient(&vaultapi.Config{
		Address: "http://" + clientAddr,
	})
	if err != nil {
		returnPortsFn()
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
		returnPortsFn()
		return nil, err
	}

	return &testVaultServer{
		rootToken:     token,
		addr:          "http://" + clientAddr,
		cmd:           cmd,
		client:        client,
		returnPortsFn: returnPortsFn,
	}, nil
}

type testVaultServer struct {
	rootToken string
	addr      string
	cmd       *exec.Cmd
	client    *vaultapi.Client

	// returnPortsFn will put the ports claimed for the test back into the
	returnPortsFn func()
}

var printedVaultVersion sync.Once

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
	if err := v.cmd.Wait(); err != nil {
		return err
	}

	if v.returnPortsFn != nil {
		v.returnPortsFn()
	}

	return nil
}
