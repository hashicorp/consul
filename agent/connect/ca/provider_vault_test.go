package ca

import (
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	vaultapi "github.com/hashicorp/vault/api"
	vaultconst "github.com/hashicorp/vault/sdk/helper/consts"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

func TestVaultCAProvider_ParseVaultCAConfig(t *testing.T) {
	cases := map[string]struct {
		rawConfig map[string]interface{}
		expConfig *structs.VaultCAProviderConfig
		expError  string
	}{
		"no token and no auth method provided": {
			rawConfig: map[string]interface{}{},
			expError:  "must provide a Vault token or configure a Vault auth method",
		},
		"both token and auth method provided": {
			rawConfig: map[string]interface{}{"Token": "test", "AuthMethod": map[string]interface{}{"Type": "test"}},
			expError:  "only one of Vault token or Vault auth method can be provided, but not both",
		},
		"no root PKI path": {
			rawConfig: map[string]interface{}{"Token": "test"},
			expError:  "must provide a valid path to a root PKI backend",
		},
		"no root intermediate path": {
			rawConfig: map[string]interface{}{"Token": "test", "RootPKIPath": "test"},
			expError:  "must provide a valid path for the intermediate PKI backend",
		},
		"adds a slash to RootPKIPath and IntermediatePKIPath": {
			rawConfig: map[string]interface{}{"Token": "test", "RootPKIPath": "test", "IntermediatePKIPath": "test"},
			expConfig: &structs.VaultCAProviderConfig{
				CommonCAProviderConfig: defaultCommonConfig(),
				Token:                  "test",
				RootPKIPath:            "test/",
				IntermediatePKIPath:    "test/",
			},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			config, err := ParseVaultCAConfig(c.rawConfig)
			if c.expError != "" {
				require.EqualError(t, err, c.expError)
			} else {
				require.NoError(t, err)
				require.Equal(t, c.expConfig, config)
			}
		})
	}
}

func TestVaultCAProvider_configureVaultAuthMethod(t *testing.T) {
	cases := map[string]struct {
		expLoginPath string
		params       map[string]interface{}
		expError     string
	}{
		"alicloud":    {expLoginPath: "auth/alicloud/login"},
		"approle":     {expLoginPath: "auth/approle/login"},
		"aws":         {expLoginPath: "auth/aws/login"},
		"azure":       {expLoginPath: "auth/azure/login"},
		"cf":          {expLoginPath: "auth/cf/login"},
		"github":      {expLoginPath: "auth/github/login"},
		"gcp":         {expLoginPath: "auth/gcp/login"},
		"jwt":         {expLoginPath: "auth/jwt/login"},
		"kerberos":    {expLoginPath: "auth/kerberos/login"},
		"kubernetes":  {expLoginPath: "auth/kubernetes/login", params: map[string]interface{}{"jwt": "fake"}},
		"ldap":        {expLoginPath: "auth/ldap/login/foo", params: map[string]interface{}{"username": "foo"}},
		"oci":         {expLoginPath: "auth/oci/login/foo", params: map[string]interface{}{"role": "foo"}},
		"okta":        {expLoginPath: "auth/okta/login/foo", params: map[string]interface{}{"username": "foo"}},
		"radius":      {expLoginPath: "auth/radius/login/foo", params: map[string]interface{}{"username": "foo"}},
		"cert":        {expLoginPath: "auth/cert/login"},
		"token":       {expError: "'token' auth method is not supported via auth method configuration; please provide the token with the 'token' parameter in the CA configuration"},
		"userpass":    {expLoginPath: "auth/userpass/login/foo", params: map[string]interface{}{"username": "foo"}},
		"unsupported": {expError: "auth method \"unsupported\" is not supported"},
	}

	for authMethodType, c := range cases {
		t.Run(authMethodType, func(t *testing.T) {
			loginPath, err := configureVaultAuthMethod(&structs.VaultAuthMethod{
				Type:   authMethodType,
				Params: c.params,
			})
			if c.expError == "" {
				require.NoError(t, err)
				require.Equal(t, c.expLoginPath, loginPath)
			} else {
				require.EqualError(t, err, c.expError)
			}
		})
	}
}

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

func TestVaultCAProvider_Configure(t *testing.T) {
	SkipIfVaultNotPresent(t)

	testcases := []struct {
		name          string
		rawConfig     map[string]interface{}
		expectedValue func(t *testing.T, v *VaultProvider)
	}{
		{
			name:      "DefaultConfig",
			rawConfig: map[string]interface{}{},
			expectedValue: func(t *testing.T, v *VaultProvider) {
				headers := v.client.Headers()
				require.Equal(t, "", headers.Get(vaultconst.NamespaceHeaderName))
				require.Equal(t, "pki-root/", v.config.RootPKIPath)
				require.Equal(t, "pki-intermediate/", v.config.IntermediatePKIPath)
			},
		},
		{
			name:      "TestConfigWithNamespace",
			rawConfig: map[string]interface{}{"namespace": "ns1"},
			expectedValue: func(t *testing.T, v *VaultProvider) {

				h := v.client.Headers()
				require.Equal(t, "ns1", h.Get(vaultconst.NamespaceHeaderName))
			},
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.name, func(t *testing.T) {
			provider, _ := testVaultProviderWithConfig(t, true, testcase.rawConfig)

			testcase.expectedValue(t, provider)
		})
	}

	return
}

func TestVaultCAProvider_SecondaryActiveIntermediate(t *testing.T) {

	SkipIfVaultNotPresent(t)

	provider, testVault := testVaultProviderWithConfig(t, false, nil)
	defer testVault.Stop()
	require := require.New(t)

	cert, err := provider.ActiveIntermediate()
	require.Empty(cert)
	require.NoError(err)
}

func TestVaultCAProvider_RenewToken(t *testing.T) {

	SkipIfVaultNotPresent(t)

	testVault, err := runTestVault(t)
	require.NoError(t, err)
	testVault.WaitUntilReady(t)

	// Create a token with a short TTL to be renewed by the provider.
	ttl := 1 * time.Second
	tcr := &vaultapi.TokenCreateRequest{
		TTL: ttl.String(),
	}
	secret, err := testVault.client.Auth().Token().Create(tcr)
	require.NoError(t, err)
	providerToken := secret.Auth.ClientToken

	_, err = createVaultProvider(t, true, testVault.Addr, providerToken, nil)
	require.NoError(t, err)

	// Check the last renewal time.
	secret, err = testVault.client.Auth().Token().Lookup(providerToken)
	require.NoError(t, err)
	firstRenewal, err := secret.Data["last_renewal_time"].(json.Number).Int64()
	require.NoError(t, err)

	// Wait past the TTL and make sure the token has been renewed.
	retry.Run(t, func(r *retry.R) {
		secret, err = testVault.client.Auth().Token().Lookup(providerToken)
		require.NoError(r, err)
		lastRenewal, err := secret.Data["last_renewal_time"].(json.Number).Int64()
		require.NoError(r, err)
		require.Greater(r, lastRenewal, firstRenewal)
	})
}

func TestVaultCAProvider_Bootstrap(t *testing.T) {

	SkipIfVaultNotPresent(t)

	providerWDefaultRootCertTtl, testvault1 := testVaultProviderWithConfig(t, true, map[string]interface{}{
		"LeafCertTTL": "1h",
	})
	defer testvault1.Stop()
	client1 := testvault1.client

	providerCustomRootCertTtl, testvault2 := testVaultProviderWithConfig(t, true, map[string]interface{}{
		"LeafCertTTL": "1h",
		"RootCertTTL": "8761h",
	})
	defer testvault2.Stop()
	client2 := testvault2.client

	require := require.New(t)

	cases := []struct {
		certFunc            func() (string, error)
		backendPath         string
		rootCaCreation      bool
		provider            *VaultProvider
		client              *vaultapi.Client
		expectedRootCertTTL string
	}{
		{
			certFunc:            providerWDefaultRootCertTtl.ActiveRoot,
			backendPath:         "pki-root/",
			rootCaCreation:      true,
			client:              client1,
			provider:            providerWDefaultRootCertTtl,
			expectedRootCertTTL: structs.DefaultRootCertTTL,
		},
		{
			certFunc:            providerCustomRootCertTtl.ActiveIntermediate,
			backendPath:         "pki-intermediate/",
			rootCaCreation:      false,
			provider:            providerCustomRootCertTtl,
			client:              client2,
			expectedRootCertTTL: "8761h",
		},
	}

	// Verify the root and intermediate certs match the ones in the vault backends
	for _, tc := range cases {
		provider := tc.provider
		client := tc.client
		cert, err := tc.certFunc()
		require.NoError(err)
		req := client.NewRequest("GET", "/v1/"+tc.backendPath+"ca/pem")
		resp, err := client.RawRequest(req)
		require.NoError(err)
		bytes, err := ioutil.ReadAll(resp.Body)
		require.NoError(err)
		require.Equal(cert, string(bytes)+"\n")

		// Should be a valid CA cert
		parsed, err := connect.ParseCert(cert)
		require.NoError(err)
		require.True(parsed.IsCA)
		require.Len(parsed.URIs, 1)
		require.Equal(fmt.Sprintf("spiffe://%s.consul", provider.clusterID), parsed.URIs[0].String())

		// test that the root cert ttl as applied
		if tc.rootCaCreation {
			rootCertTTL, err := time.ParseDuration(tc.expectedRootCertTTL)
			require.NoError(err)
			expectedNotAfter := time.Now().Add(rootCertTTL).UTC()

			require.WithinDuration(expectedNotAfter, parsed.NotAfter, 10*time.Minute, "expected parsed cert ttl to be the same as the value configured")
		}
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

	SkipIfVaultNotPresent(t)

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
				requireTrailingNewline(t, cert)
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

	SkipIfVaultNotPresent(t)

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

	SkipIfVaultNotPresent(t)

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

	SkipIfVaultNotPresent(t)

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

func TestVaultProvider_Cleanup(t *testing.T) {

	SkipIfVaultNotPresent(t)

	testVault, err := runTestVault(t)
	require.NoError(t, err)

	testVault.WaitUntilReady(t)

	t.Run("provider-change", func(t *testing.T) {
		provider, err := createVaultProvider(t, true, testVault.Addr, testVault.RootToken, nil)
		require.NoError(t, err)

		// ensure that the intermediate PKI mount exists
		mounts, err := provider.client.Sys().ListMounts()
		require.NoError(t, err)
		require.Contains(t, mounts, provider.config.IntermediatePKIPath)

		// call cleanup with a provider change - this should cause removal of the mount
		require.NoError(t, provider.Cleanup(true, nil))

		// verify the mount was removed
		mounts, err = provider.client.Sys().ListMounts()
		require.NoError(t, err)
		require.NotContains(t, mounts, provider.config.IntermediatePKIPath)
	})

	t.Run("pki-path-change", func(t *testing.T) {
		provider, err := createVaultProvider(t, true, testVault.Addr, testVault.RootToken, nil)
		require.NoError(t, err)

		// ensure that the intermediate PKI mount exists
		mounts, err := provider.client.Sys().ListMounts()
		require.NoError(t, err)
		require.Contains(t, mounts, provider.config.IntermediatePKIPath)

		// call cleanup with an intermediate pki path change - this should cause removal of the mount
		require.NoError(t, provider.Cleanup(false, map[string]interface{}{
			"Address":     testVault.Addr,
			"Token":       testVault.RootToken,
			"RootPKIPath": "pki-root/",
			//
			"IntermediatePKIPath": "pki-intermediate2/",
			// Tests duration parsing after msgpack type mangling during raft apply.
			"LeafCertTTL": []uint8("72h"),
		}))

		// verify the mount was removed
		mounts, err = provider.client.Sys().ListMounts()
		require.NoError(t, err)
		require.NotContains(t, mounts, provider.config.IntermediatePKIPath)
	})

	t.Run("pki-path-unchanged", func(t *testing.T) {
		provider, err := createVaultProvider(t, true, testVault.Addr, testVault.RootToken, nil)
		require.NoError(t, err)

		// ensure that the intermediate PKI mount exists
		mounts, err := provider.client.Sys().ListMounts()
		require.NoError(t, err)
		require.Contains(t, mounts, provider.config.IntermediatePKIPath)

		// call cleanup with no config changes - this should not cause removal of the intermediate pki path
		require.NoError(t, provider.Cleanup(false, map[string]interface{}{
			"Address":             testVault.Addr,
			"Token":               testVault.RootToken,
			"RootPKIPath":         "pki-root/",
			"IntermediatePKIPath": "pki-intermediate/",
			// Tests duration parsing after msgpack type mangling during raft apply.
			"LeafCertTTL": []uint8("72h"),
		}))

		// verify the mount was NOT removed
		mounts, err = provider.client.Sys().ListMounts()
		require.NoError(t, err)
		require.Contains(t, mounts, provider.config.IntermediatePKIPath)
	})
}

func TestVaultProvider_ConfigureWithAuthMethod(t *testing.T) {

	SkipIfVaultNotPresent(t)

	cases := []struct {
		authMethodType          string
		configureAuthMethodFunc func(t *testing.T, vaultClient *vaultapi.Client) map[string]interface{}
	}{
		{
			authMethodType: "userpass",
			configureAuthMethodFunc: func(t *testing.T, vaultClient *vaultapi.Client) map[string]interface{} {
				_, err := vaultClient.Logical().Write("/auth/userpass/users/test",
					map[string]interface{}{"password": "foo", "policies": "admins"})
				require.NoError(t, err)
				return map[string]interface{}{
					"Type": "userpass",
					"Params": map[string]interface{}{
						"username": "test",
						"password": "foo",
					},
				}
			},
		},
		{
			authMethodType: "approle",
			configureAuthMethodFunc: func(t *testing.T, vaultClient *vaultapi.Client) map[string]interface{} {
				_, err := vaultClient.Logical().Write("auth/approle/role/my-role", nil)
				require.NoError(t, err)
				resp, err := vaultClient.Logical().Read("auth/approle/role/my-role/role-id")
				require.NoError(t, err)
				roleID := resp.Data["role_id"]

				resp, err = vaultClient.Logical().Write("auth/approle/role/my-role/secret-id", nil)
				require.NoError(t, err)
				secretID := resp.Data["secret_id"]

				return map[string]interface{}{
					"Type": "approle",
					"Params": map[string]interface{}{
						"role_id":   roleID,
						"secret_id": secretID,
					},
				}
			},
		},
	}

	for _, c := range cases {
		t.Run(c.authMethodType, func(t *testing.T) {
			testVault := NewTestVaultServer(t)

			err := testVault.Client().Sys().EnableAuthWithOptions(c.authMethodType, &vaultapi.EnableAuthOptions{Type: c.authMethodType})
			require.NoError(t, err)

			authMethodConf := c.configureAuthMethodFunc(t, testVault.Client())

			conf := map[string]interface{}{
				"Address":             testVault.Addr,
				"RootPKIPath":         "pki-root/",
				"IntermediatePKIPath": "pki-intermediate/",
				"AuthMethod":          authMethodConf,
			}

			provider := NewVaultProvider(hclog.New(nil))

			cfg := ProviderConfig{
				ClusterID:  connect.TestClusterID,
				Datacenter: "dc1",
				IsPrimary:  true,
				RawConfig:  conf,
			}
			t.Cleanup(provider.Stop)
			err = provider.Configure(cfg)
			require.NoError(t, err)
			require.NotEmpty(t, provider.client.Token())
		})
	}
}

func TestVaultProvider_RotateAuthMethodToken(t *testing.T) {

	SkipIfVaultNotPresent(t)

	testVault := NewTestVaultServer(t)

	err := testVault.Client().Sys().EnableAuthWithOptions("approle", &vaultapi.EnableAuthOptions{Type: "approle"})
	require.NoError(t, err)

	_, err = testVault.Client().Logical().Write("auth/approle/role/my-role",
		map[string]interface{}{"token_ttl": "2s", "token_explicit_max_ttl": "2s"})
	require.NoError(t, err)
	resp, err := testVault.Client().Logical().Read("auth/approle/role/my-role/role-id")
	require.NoError(t, err)
	roleID := resp.Data["role_id"]

	resp, err = testVault.Client().Logical().Write("auth/approle/role/my-role/secret-id", nil)
	require.NoError(t, err)
	secretID := resp.Data["secret_id"]

	conf := map[string]interface{}{
		"Address":             testVault.Addr,
		"RootPKIPath":         "pki-root/",
		"IntermediatePKIPath": "pki-intermediate/",
		"AuthMethod": map[string]interface{}{
			"Type": "approle",
			"Params": map[string]interface{}{
				"role_id":   roleID,
				"secret_id": secretID,
			},
		},
	}

	provider := NewVaultProvider(hclog.New(nil))

	cfg := ProviderConfig{
		ClusterID:  connect.TestClusterID,
		Datacenter: "dc1",
		IsPrimary:  true,
		RawConfig:  conf,
	}
	t.Cleanup(provider.Stop)
	err = provider.Configure(cfg)
	require.NoError(t, err)
	token := provider.client.Token()
	require.NotEmpty(t, token)

	// Check that the token is rotated after max_ttl time has passed.
	require.Eventually(t, func() bool {
		return provider.client.Token() != token
	}, 10*time.Second, 100*time.Millisecond)
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

func testVaultProviderWithConfig(t *testing.T, isPrimary bool, rawConf map[string]interface{}) (*VaultProvider, *TestVaultServer) {
	testVault, err := runTestVault(t)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	testVault.WaitUntilReady(t)

	provider, err := createVaultProvider(t, isPrimary, testVault.Addr, testVault.RootToken, rawConf)
	if err != nil {
		testVault.Stop()
		t.Fatalf("err: %v", err)
	}
	return provider, testVault
}

func createVaultProvider(t *testing.T, isPrimary bool, addr, token string, rawConf map[string]interface{}) (*VaultProvider, error) {
	conf := map[string]interface{}{
		"Address":             addr,
		"Token":               token,
		"RootPKIPath":         "pki-root/",
		"IntermediatePKIPath": "pki-intermediate/",
		// Tests duration parsing after msgpack type mangling during raft apply.
		"LeafCertTTL": []uint8("72h"),
	}
	for k, v := range rawConf {
		conf[k] = v
	}

	provider := NewVaultProvider(hclog.New(nil))

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

	t.Cleanup(provider.Stop)
	require.NoError(t, provider.Configure(cfg))
	if isPrimary {
		require.NoError(t, provider.GenerateRoot())
		_, err := provider.GenerateIntermediate()
		require.NoError(t, err)
	}

	return provider, nil
}
