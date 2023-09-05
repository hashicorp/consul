// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package ca

import (
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	vaultapi "github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/api/auth/gcp"
	vaultconst "github.com/hashicorp/vault/sdk/helper/consts"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

const pkiTestPolicyBase = `
path "sys/mounts"
{
	capabilities = ["read"]
}
path "sys/mounts/%[1]s"
{
	capabilities = ["create", "read", "update", "delete", "list"]
}
path "sys/mounts/%[2]s"
{
	capabilities = ["create", "read", "update", "delete", "list"]
}
path "sys/mounts/%[2]s/tune"
{
	capabilities = ["update"]
}
path "%[1]s/*"
{
	capabilities = ["create", "read", "update", "delete", "list"]
}
path "%[2]s/*"
{
	capabilities = ["create", "read", "update", "delete", "list"]
}`

func pkiTestPolicy(rootPath, intermediatePath string) string {
	return fmt.Sprintf(pkiTestPolicyBase, rootPath, intermediatePath)
}

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
		hasLDG       bool
	}{
		"alicloud":    {expLoginPath: "auth/alicloud/login", params: map[string]any{"role": "test-role", "region": "test-region"}, hasLDG: true},
		"approle":     {expLoginPath: "auth/approle/login", params: map[string]any{"role_id_file_path": "test-path"}, hasLDG: true},
		"aws":         {expLoginPath: "auth/aws/login", params: map[string]interface{}{"type": "iam"}, hasLDG: true},
		"azure":       {expLoginPath: "auth/azure/login", params: map[string]interface{}{"role": "test-role", "resource": "test-resource"}, hasLDG: true},
		"cf":          {expLoginPath: "auth/cf/login"},
		"github":      {expLoginPath: "auth/github/login"},
		"gcp":         {expLoginPath: "auth/gcp/login", params: map[string]interface{}{"type": "iam", "role": "test-role"}},
		"jwt":         {expLoginPath: "auth/jwt/login", params: map[string]any{"role": "test-role", "path": "test-path"}, hasLDG: true},
		"kerberos":    {expLoginPath: "auth/kerberos/login"},
		"kubernetes":  {expLoginPath: "auth/kubernetes/login", params: map[string]interface{}{"role": "test-role"}, hasLDG: true},
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
			authMethod := &structs.VaultAuthMethod{
				Type:   authMethodType,
				Params: c.params,
			}
			authIF, err := configureVaultAuthMethod(authMethod)
			if c.expError != "" {
				require.EqualError(t, err, c.expError)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, authIF)

			switch authMethodType {
			case VaultAuthMethodTypeGCP:
				_ = authIF.(*gcp.GCPAuth)
			default:
				auth := authIF.(*VaultAuthClient)
				require.Equal(t, authMethod, auth.AuthMethod)
				require.Equal(t, c.expLoginPath, auth.LoginPath)
				require.Equal(t, c.hasLDG, auth.LoginDataGen != nil)
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
	require.Equal(t, config.CAFile, tlsConfig.CACert)
	require.Equal(t, config.CAPath, tlsConfig.CAPath)
	require.Equal(t, config.CertFile, tlsConfig.ClientCert)
	require.Equal(t, config.KeyFile, tlsConfig.ClientKey)
	require.Equal(t, config.TLSServerName, tlsConfig.TLSServerName)
	require.Equal(t, config.TLSSkipVerify, tlsConfig.Insecure)
}

func TestVaultCAProvider_Configure(t *testing.T) {
	SkipIfVaultNotPresent(t)

	testcases := []struct {
		name          string
		rawConfig     map[string]any
		expectedValue func(t *testing.T, v *VaultProvider)
	}{
		{
			name: "DefaultConfig",
			rawConfig: map[string]any{
				"RootPKIPath":         "pki-root/",
				"IntermediatePKIPath": "pki-intermediate/",
			},
			expectedValue: func(t *testing.T, v *VaultProvider) {
				headers := v.client.Headers()
				require.Equal(t, "", headers.Get(vaultconst.NamespaceHeaderName))
				require.Equal(t, "pki-root/", v.config.RootPKIPath)
				require.Equal(t, "pki-intermediate/", v.config.IntermediatePKIPath)
			},
		},
		{
			name: "TestConfigWithNamespace",
			rawConfig: map[string]any{
				"namespace":           "ns1",
				"RootPKIPath":         "pki-root/",
				"IntermediatePKIPath": "pki-intermediate/",
			},
			expectedValue: func(t *testing.T, v *VaultProvider) {
				h := v.client.Headers()
				require.Equal(t, "ns1", h.Get(vaultconst.NamespaceHeaderName))
			},
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.name, func(t *testing.T) {
			testVault := NewTestVaultServer(t)

			attr := &VaultTokenAttributes{
				RootPath:         "pki-root",
				IntermediatePath: "pki-intermediate",
				ConsulManaged:    true,
			}
			token := CreateVaultTokenWithAttrs(t, testVault.client, attr)

			provider := createVaultProvider(t, true, testVault.Addr, token, testcase.rawConfig)
			testcase.expectedValue(t, provider)
		})
	}

	return
}

func TestVaultCAProvider_SecondaryActiveIntermediate(t *testing.T) {
	SkipIfVaultNotPresent(t)

	t.Parallel()

	testVault := NewTestVaultServer(t)

	attr := &VaultTokenAttributes{
		RootPath:         "pki-root",
		IntermediatePath: "pki-intermediate",
		ConsulManaged:    true,
	}
	token := CreateVaultTokenWithAttrs(t, testVault.client, attr)

	provider := createVaultProvider(t, false, testVault.Addr, token, map[string]any{
		"RootPKIPath":         "pki-root/",
		"IntermediatePKIPath": "pki-intermediate/",
	})

	cert, err := provider.ActiveLeafSigningCert()
	require.Empty(t, cert)
	require.NoError(t, err)
}

func TestVaultCAProvider_RenewToken(t *testing.T) {
	SkipIfVaultNotPresent(t)

	t.Parallel()

	testVault := NewTestVaultServer(t)

	// Create a token with a short TTL to be renewed by the provider.
	ttl := 1 * time.Second
	tcr := &vaultapi.TokenCreateRequest{
		TTL: ttl.String(),
	}
	secret, err := testVault.client.Auth().Token().Create(tcr)
	require.NoError(t, err)
	providerToken := secret.Auth.ClientToken

	_ = createVaultProvider(t, true, testVault.Addr, providerToken, map[string]any{
		"RootPKIPath":         "pki-root/",
		"IntermediatePKIPath": "pki-intermediate/",
	})

	// Check the last renewal time.
	secret, err = testVault.client.Auth().Token().Lookup(providerToken)
	require.NoError(t, err)
	firstRenewal, err := secret.Data["last_renewal_time"].(json.Number).Int64()
	require.NoError(t, err)

	// Retry past the TTL and make sure the token has been renewed.
	retry.Run(t, func(r *retry.R) {
		secret, err = testVault.client.Auth().Token().Lookup(providerToken)
		require.NoError(r, err)
		lastRenewal, err := secret.Data["last_renewal_time"].(json.Number).Int64()
		require.NoError(r, err)
		require.Greater(r, lastRenewal, firstRenewal)
	})
}

func TestVaultCAProvider_RenewTokenStopWatcherOnConfigure(t *testing.T) {
	SkipIfVaultNotPresent(t)

	t.Parallel()

	testVault := NewTestVaultServer(t)

	// Create a token with a short TTL to be renewed by the provider.
	ttl := 1 * time.Second
	tcr := &vaultapi.TokenCreateRequest{
		TTL: ttl.String(),
	}
	secret, err := testVault.client.Auth().Token().Create(tcr)
	require.NoError(t, err)
	providerToken := secret.Auth.ClientToken

	provider := createVaultProvider(t, true, testVault.Addr, providerToken, map[string]any{
		"RootPKIPath":         "pki-root/",
		"IntermediatePKIPath": "pki-intermediate/",
	})

	// overwrite stopWatcher to set flag on stop for testing
	// be sure that original stopWatcher gets called to avoid goroutine leak
	gotStopped := uint32(0)
	realStop := provider.stopWatcher
	provider.stopWatcher = func() {
		atomic.StoreUint32(&gotStopped, 1)
		realStop()
	}

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

	providerConfig := vaultProviderConfig(t, testVault.Addr, providerToken, map[string]any{
		"RootPKIPath":         "pki-root/",
		"IntermediatePKIPath": "pki-intermediate/",
	})

	require.NoError(t, provider.Configure(providerConfig))
	require.Equal(t, uint32(1), atomic.LoadUint32(&gotStopped))
}

func TestVaultCAProvider_Bootstrap(t *testing.T) {
	SkipIfVaultNotPresent(t)

	t.Parallel()

	type testcase struct {
		name                string
		caConfig            map[string]any
		certFunc            func(*VaultProvider) (string, error)
		backendPath         string
		rootCaCreation      bool
		expectedRootCertTTL string
	}

	run := func(t *testing.T, tc testcase) {
		t.Parallel()

		tc.caConfig["RootPKIPath"] = "pki-root/"
		tc.caConfig["IntermediatePKIPath"] = "pki-intermediate/"

		testVault := NewTestVaultServer(t)

		attr := &VaultTokenAttributes{
			RootPath:         "pki-root",
			IntermediatePath: "pki-intermediate",
			ConsulManaged:    true,
		}
		token := CreateVaultTokenWithAttrs(t, testVault.client, attr)

		provider := createVaultProvider(t, true, testVault.Addr, token, tc.caConfig)
		client := testVault.client

		cert, err := tc.certFunc(provider)
		require.NoError(t, err)
		resp, err := client.Logical().ReadRaw(tc.backendPath + "ca/pem")
		require.NoError(t, err)
		bytes, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, cert, string(bytes)+"\n")

		// Should be a valid CA cert
		parsed, err := connect.ParseCert(cert)
		require.NoError(t, err)
		require.True(t, parsed.IsCA)
		require.Len(t, parsed.URIs, 1)
		require.Equal(t, fmt.Sprintf("spiffe://%s.consul", provider.clusterID), parsed.URIs[0].String())

		// test that the root cert ttl as applied
		if tc.rootCaCreation {
			rootCertTTL, err := time.ParseDuration(tc.expectedRootCertTTL)
			require.NoError(t, err)
			expectedNotAfter := time.Now().Add(rootCertTTL).UTC()

			require.WithinDuration(t, expectedNotAfter, parsed.NotAfter, 10*time.Minute, "expected parsed cert ttl to be the same as the value configured")
		}
	}

	cases := []testcase{
		{
			name: "default-root-cert-ttl",
			caConfig: map[string]any{
				"LeafCertTTL": "1h",
			},
			certFunc: func(provider *VaultProvider) (string, error) {
				root, err := provider.GenerateCAChain()
				return root, err
			},
			backendPath:         "pki-root/",
			rootCaCreation:      true,
			expectedRootCertTTL: structs.DefaultRootCertTTL,
		},
		{
			name: "custom-root-cert-ttl",
			caConfig: map[string]any{
				"LeafCertTTL": "1h",
				"RootCertTTL": "8761h",
			},
			certFunc: func(provider *VaultProvider) (string, error) {
				return provider.ActiveLeafSigningCert()
			},
			backendPath:         "pki-intermediate/",
			rootCaCreation:      false,
			expectedRootCertTTL: "8761h",
		},
	}

	// Verify the root and intermediate certs match the ones in the vault backends
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
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

	t.Parallel()

	run := func(t *testing.T, tc KeyTestCase) {
		t.Parallel()

		testVault := NewTestVaultServer(t)

		attr := &VaultTokenAttributes{
			RootPath:         "pki-root",
			IntermediatePath: "pki-intermediate",
			ConsulManaged:    true,
		}
		token := CreateVaultTokenWithAttrs(t, testVault.client, attr)

		provider := createVaultProvider(t, true, testVault.Addr, token, map[string]any{
			"LeafCertTTL":         "1h",
			"PrivateKeyType":      tc.KeyType,
			"PrivateKeyBits":      tc.KeyBits,
			"RootPKIPath":         "pki-root/",
			"IntermediatePKIPath": "pki-intermediate/",
		})

		spiffeService := &connect.SpiffeIDService{
			Host:       "node1",
			Namespace:  "default",
			Datacenter: "dc1",
			Service:    "foo",
		}

		rootPEM, err := provider.GenerateCAChain()
		require.NoError(t, err)
		assertCorrectKeyType(t, tc.KeyType, rootPEM)

		intPEM, err := provider.ActiveLeafSigningCert()
		require.NoError(t, err)
		assertCorrectKeyType(t, tc.KeyType, intPEM)

		// Generate a leaf cert for the service.
		var firstSerial uint64
		{
			raw, _ := connect.TestCSR(t, spiffeService)

			csr, err := connect.ParseCSR(raw)
			require.NoError(t, err)

			cert, err := provider.Sign(csr)
			require.NoError(t, err)

			parsed, err := connect.ParseCert(cert)
			require.NoError(t, err)
			require.Equal(t, parsed.URIs[0], spiffeService.URI())
			firstSerial = parsed.SerialNumber.Uint64()

			// Ensure the cert is valid now and expires within the correct limit.
			now := time.Now()
			require.True(t, parsed.NotAfter.Sub(now) < time.Hour)
			require.True(t, parsed.NotBefore.Before(now))

			// Make sure we can validate the cert as expected.
			require.NoError(t, connect.ValidateLeaf(rootPEM, cert, []string{intPEM}))
			requireTrailingNewline(t, cert)
		}

		// Generate a new cert for another service and make sure
		// the serial number is unique.
		spiffeService.Service = "bar"
		{
			raw, _ := connect.TestCSR(t, spiffeService)

			csr, err := connect.ParseCSR(raw)
			require.NoError(t, err)

			cert, err := provider.Sign(csr)
			require.NoError(t, err)

			parsed, err := connect.ParseCert(cert)
			require.NoError(t, err)
			require.Equal(t, parsed.URIs[0], spiffeService.URI())
			require.NotEqual(t, firstSerial, parsed.SerialNumber.Uint64())

			// Ensure the cert is valid now and expires within the correct limit.
			require.True(t, time.Until(parsed.NotAfter) < time.Hour)
			require.True(t, parsed.NotBefore.Before(time.Now()))

			// Make sure we can validate the cert as expected.
			require.NoError(t, connect.ValidateLeaf(rootPEM, cert, []string{intPEM}))
		}
	}

	for _, tc := range KeyTestCases {
		t.Run(tc.Desc, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestVaultCAProvider_CrossSignCA(t *testing.T) {
	SkipIfVaultNotPresent(t)

	t.Parallel()

	tests := CASigningKeyTypeCases()

	run := func(t *testing.T, tc CASigningKeyTypes, withSudo, expectFailure bool) {
		t.Parallel()

		if tc.SigningKeyType != tc.CSRKeyType {
			// TODO: uncomment since the bug is closed
			// See https://github.com/hashicorp/vault/issues/7709
			t.Skip("Vault doesn't support cross-signing different key types yet.")
		}

		testVault1 := NewTestVaultServer(t)

		attr1 := &VaultTokenAttributes{
			RootPath:         "pki-root",
			IntermediatePath: "pki-intermediate",
			ConsulManaged:    true,
			WithSudo:         withSudo,
		}
		token1 := CreateVaultTokenWithAttrs(t, testVault1.client, attr1)

		provider1 := createVaultProvider(t, true, testVault1.Addr, token1, map[string]any{
			"LeafCertTTL":         "1h",
			"PrivateKeyType":      tc.SigningKeyType,
			"PrivateKeyBits":      tc.SigningKeyBits,
			"RootPKIPath":         "pki-root/",
			"IntermediatePKIPath": "pki-intermediate/",
		})

		testutil.RunStep(t, "init", func(t *testing.T) {
			rootPEM, err := provider1.GenerateCAChain()
			require.NoError(t, err)
			assertCorrectKeyType(t, tc.SigningKeyType, rootPEM)

			intPEM, err := provider1.ActiveLeafSigningCert()
			require.NoError(t, err)
			assertCorrectKeyType(t, tc.SigningKeyType, intPEM)
		})

		testVault2 := NewTestVaultServer(t)

		attr2 := &VaultTokenAttributes{
			RootPath:         "pki-root",
			IntermediatePath: "pki-intermediate",
			ConsulManaged:    true,
			WithSudo:         false, // irrelevant for the new CA provider
		}
		token2 := CreateVaultTokenWithAttrs(t, testVault2.client, attr2)

		provider2 := createVaultProvider(t, true, testVault2.Addr, token2, map[string]any{
			"LeafCertTTL":         "1h",
			"PrivateKeyType":      tc.CSRKeyType,
			"PrivateKeyBits":      tc.CSRKeyBits,
			"RootPKIPath":         "pki-root/",
			"IntermediatePKIPath": "pki-intermediate/",
		})

		testutil.RunStep(t, "swap", func(t *testing.T) {
			rootPEM, err := provider2.GenerateCAChain()
			require.NoError(t, err)
			assertCorrectKeyType(t, tc.CSRKeyType, rootPEM)

			intPEM, err := provider2.ActiveLeafSigningCert()
			require.NoError(t, err)
			assertCorrectKeyType(t, tc.CSRKeyType, intPEM)

			if expectFailure {
				testCrossSignProvidersShouldFail(t, provider1, provider2)
			} else {
				testCrossSignProviders(t, provider1, provider2)
			}
		})
	}

	for _, tc := range tests {
		t.Run(tc.Desc, func(t *testing.T) {
			t.Run("without sudo", func(t *testing.T) {
				run(t, tc, false, true)
			})
			t.Run("with sudo", func(t *testing.T) {
				run(t, tc, true, false)
			})
		})
	}
}

func TestVaultProvider_SignIntermediate(t *testing.T) {
	SkipIfVaultNotPresent(t)

	t.Parallel()

	tests := CASigningKeyTypeCases()

	run := func(t *testing.T, tc CASigningKeyTypes) {
		t.Parallel()

		testVault1 := NewTestVaultServer(t)

		attr1 := &VaultTokenAttributes{
			RootPath:         "pki-root",
			IntermediatePath: "pki-intermediate",
			ConsulManaged:    true,
		}
		token1 := CreateVaultTokenWithAttrs(t, testVault1.client, attr1)

		provider1 := createVaultProvider(t, true, testVault1.Addr, token1, map[string]any{
			"LeafCertTTL":         "1h",
			"PrivateKeyType":      tc.SigningKeyType,
			"PrivateKeyBits":      tc.SigningKeyBits,
			"RootPKIPath":         "pki-root/",
			"IntermediatePKIPath": "pki-intermediate/",
		})

		testVault2 := NewTestVaultServer(t)

		attr2 := &VaultTokenAttributes{
			RootPath:         "pki-root",
			IntermediatePath: "pki-intermediate",
			ConsulManaged:    true,
		}
		token2 := CreateVaultTokenWithAttrs(t, testVault2.client, attr2)

		provider2 := createVaultProvider(t, false, testVault2.Addr, token2, map[string]any{
			"LeafCertTTL":         "1h",
			"PrivateKeyType":      tc.CSRKeyType,
			"PrivateKeyBits":      tc.CSRKeyBits,
			"RootPKIPath":         "pki-root/",
			"IntermediatePKIPath": "pki-intermediate/",
		})

		testSignIntermediateCrossDC(t, provider1, provider2)
	}

	for _, tc := range tests {
		t.Run(tc.Desc, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestVaultProvider_SignIntermediateConsul(t *testing.T) {
	SkipIfVaultNotPresent(t)

	t.Parallel()

	// primary = Vault, secondary = Consul
	t.Run("pri=vault,sec=consul", func(t *testing.T) {
		t.Parallel()

		testVault1 := NewTestVaultServer(t)

		attr1 := &VaultTokenAttributes{
			RootPath:         "pki-root",
			IntermediatePath: "pki-intermediate",
			ConsulManaged:    true,
		}
		token1 := CreateVaultTokenWithAttrs(t, testVault1.client, attr1)

		provider1 := createVaultProvider(t, true, testVault1.Addr, token1, map[string]any{
			"RootPKIPath":         "pki-root/",
			"IntermediatePKIPath": "pki-intermediate/",
		})

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
		t.Parallel()

		conf := testConsulCAConfig()
		delegate := newMockDelegate(t, conf)
		provider1 := TestConsulProvider(t, delegate)
		require.NoError(t, provider1.Configure(testProviderConfig(conf)))
		_, err := provider1.GenerateCAChain()
		require.NoError(t, err)

		// Ensure that we don't configure vault to try and mint leafs that
		// outlive their CA during the test (which hard fails in vault).
		intermediateCertTTL := getIntermediateCertTTL(t, conf)
		leafCertTTL := intermediateCertTTL - 4*time.Hour

		overrideConf := map[string]any{
			"LeafCertTTL":         []uint8(leafCertTTL.String()),
			"RootPKIPath":         "pki-root/",
			"IntermediatePKIPath": "pki-intermediate/",
		}

		testVault2 := NewTestVaultServer(t)

		attr2 := &VaultTokenAttributes{
			RootPath:         "pki-root",
			IntermediatePath: "pki-intermediate",
			ConsulManaged:    true,
		}
		token2 := CreateVaultTokenWithAttrs(t, testVault2.client, attr2)

		provider2 := createVaultProvider(t, false, testVault2.Addr, token2, overrideConf)

		testSignIntermediateCrossDC(t, provider1, provider2)
	})
}

func TestVaultProvider_Cleanup(t *testing.T) {
	SkipIfVaultNotPresent(t)

	t.Parallel()

	testVault := NewTestVaultServer(t)

	t.Run("provider-change", func(t *testing.T) {
		attr := &VaultTokenAttributes{
			RootPath:         "pki-root",
			IntermediatePath: "pki-intermediate",
			ConsulManaged:    true,
		}
		token := CreateVaultTokenWithAttrs(t, testVault.client, attr)

		provider := createVaultProvider(t, true, testVault.Addr, token, map[string]any{
			"RootPKIPath":         "pki-root/",
			"IntermediatePKIPath": "pki-intermediate/",
		})

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
		attr := &VaultTokenAttributes{
			RootPath:         "pki-root",
			IntermediatePath: "pki-intermediate",
			ConsulManaged:    true,
		}
		token := CreateVaultTokenWithAttrs(t, testVault.client, attr)

		provider := createVaultProvider(t, true, testVault.Addr, token, map[string]any{
			"RootPKIPath":         "pki-root/",
			"IntermediatePKIPath": "pki-intermediate/",
		})

		// ensure that the intermediate PKI mount exists
		mounts, err := provider.client.Sys().ListMounts()
		require.NoError(t, err)
		require.Contains(t, mounts, provider.config.IntermediatePKIPath)

		// call cleanup with an intermediate pki path change - this should cause removal of the mount
		require.NoError(t, provider.Cleanup(false, map[string]any{
			"Address":     testVault.Addr,
			"Token":       token,
			"RootPKIPath": "pki-root/",
			//
			"IntermediatePKIPath": "pki-intermediate-2/",
			// Tests duration parsing after msgpack type mangling during raft apply.
			"LeafCertTTL": []uint8("72h"),
		}))

		// verify the mount was removed
		mounts, err = provider.client.Sys().ListMounts()
		require.NoError(t, err)
		require.NotContains(t, mounts, provider.config.IntermediatePKIPath)
	})

	t.Run("pki-path-unchanged", func(t *testing.T) {
		attr := &VaultTokenAttributes{
			RootPath:         "pki-root",
			IntermediatePath: "pki-intermediate",
			ConsulManaged:    true,
		}
		token := CreateVaultTokenWithAttrs(t, testVault.client, attr)

		provider := createVaultProvider(t, true, testVault.Addr, token, map[string]any{
			"RootPKIPath":         "pki-root/",
			"IntermediatePKIPath": "pki-intermediate/",
		})

		// ensure that the intermediate PKI mount exists
		mounts, err := provider.client.Sys().ListMounts()
		require.NoError(t, err)
		require.Contains(t, mounts, provider.config.IntermediatePKIPath)

		// call cleanup with no config changes - this should not cause removal of the intermediate pki path
		require.NoError(t, provider.Cleanup(false, map[string]any{
			"Address":             testVault.Addr,
			"Token":               token,
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
					map[string]interface{}{"password": "foo", "policies": "pki"})
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
				_, err := vaultClient.Logical().Write("auth/approle/role/my-role",
					map[string]interface{}{"token_policies": "pki"})
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

			err = testVault.Client().Sys().PutPolicy("pki", pkiTestPolicy("pki-root", "pki-intermediate"))
			require.NoError(t, err)

			authMethodConf := c.configureAuthMethodFunc(t, testVault.Client())

			conf := map[string]any{
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

	t.Parallel()

	testVault := NewTestVaultServer(t)

	err := testVault.Client().Sys().PutPolicy("pki", pkiTestPolicy("pki-root", "pki-intermediate"))
	require.NoError(t, err)

	err = testVault.Client().Sys().EnableAuthWithOptions("approle", &vaultapi.EnableAuthOptions{Type: "approle"})
	require.NoError(t, err)

	_, err = testVault.Client().Logical().Write("auth/approle/role/my-role",
		map[string]interface{}{
			"token_ttl":              "2s",
			"token_explicit_max_ttl": "2s",
			"token_policies":         "pki",
		})
	require.NoError(t, err)
	resp, err := testVault.Client().Logical().Read("auth/approle/role/my-role/role-id")
	require.NoError(t, err)
	roleID := resp.Data["role_id"]

	resp, err = testVault.Client().Logical().Write("auth/approle/role/my-role/secret-id", nil)
	require.NoError(t, err)
	secretID := resp.Data["secret_id"]

	conf := map[string]any{
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

func TestVaultProvider_ReconfigureIntermediateTTL(t *testing.T) {
	SkipIfVaultNotPresent(t)

	t.Parallel()

	// Set up a standard policy without any sys/mounts/pki-intermediate/tune permissions.
	policy := `
	path "sys/mounts"
	{
		capabilities = ["read"]
	}
	path "sys/mounts/pki-root"
	{
		capabilities = ["create", "read", "update", "delete", "list"]
	}
	path "sys/mounts/pki-intermediate"
	{
		capabilities = ["create", "read", "update", "delete", "list"]
	}
	path "pki-root/*"
	{
		capabilities = ["create", "read", "update", "delete", "list"]
	}
	path "pki-intermediate/*"
	{
		capabilities = ["create", "read", "update", "delete", "list"]
	}`
	testVault := NewTestVaultServer(t)

	err := testVault.Client().Sys().PutPolicy("pki", policy)
	require.NoError(t, err)

	tcr := &vaultapi.TokenCreateRequest{
		Policies: []string{"pki"},
	}
	secret, err := testVault.client.Auth().Token().Create(tcr)
	require.NoError(t, err)
	providerToken := secret.Auth.ClientToken

	makeProviderConfWithTTL := func(ttl string) ProviderConfig {
		conf := map[string]any{
			"Address":             testVault.Addr,
			"RootPKIPath":         "pki-root/",
			"IntermediatePKIPath": "pki-intermediate/",
			"Token":               providerToken,
			"IntermediateCertTTL": ttl,
		}
		cfg := ProviderConfig{
			ClusterID:  connect.TestClusterID,
			Datacenter: "dc1",
			IsPrimary:  true,
			RawConfig:  conf,
		}
		return cfg
	}

	provider := NewVaultProvider(hclog.New(nil))

	// Set up the initial provider config
	t.Cleanup(provider.Stop)
	err = provider.Configure(makeProviderConfWithTTL("222h"))
	require.NoError(t, err)
	_, err = provider.GenerateCAChain()
	require.NoError(t, err)
	_, err = provider.GenerateLeafSigningCert()
	require.NoError(t, err)

	// Attempt to update the ttl without permissions for the tune endpoint - shouldn't
	// return an error.
	err = provider.Configure(makeProviderConfWithTTL("333h"))
	require.NoError(t, err)

	// Intermediate TTL shouldn't have changed
	mountConfig, err := testVault.Client().Sys().MountConfig("pki-intermediate")
	require.NoError(t, err)
	require.Equal(t, 222*3600, mountConfig.MaxLeaseTTL)

	// Update the policy and verify we can reconfigure the TTL properly.
	policy += `
	path "sys/mounts/pki-intermediate/tune"
	{
	  capabilities = ["update"]
	}`
	err = testVault.Client().Sys().PutPolicy("pki", policy)
	require.NoError(t, err)

	err = provider.Configure(makeProviderConfWithTTL("333h"))
	require.NoError(t, err)

	mountConfig, err = testVault.Client().Sys().MountConfig("pki-intermediate")
	require.NoError(t, err)
	require.Equal(t, 333*3600, mountConfig.MaxLeaseTTL)
}

func TestVaultCAProvider_GenerateIntermediate(t *testing.T) {
	SkipIfVaultNotPresent(t)

	t.Parallel()

	testVault := NewTestVaultServer(t)

	attr := &VaultTokenAttributes{
		RootPath:         "pki-root",
		IntermediatePath: "pki-intermediate",
		ConsulManaged:    true,
	}
	token := CreateVaultTokenWithAttrs(t, testVault.client, attr)

	provider := createVaultProvider(t, true, testVault.Addr, token, map[string]any{
		"RootPKIPath":         "pki-root/",
		"IntermediatePKIPath": "pki-intermediate/",
	})

	orig, err := provider.ActiveLeafSigningCert()
	require.NoError(t, err)

	// This test was created to ensure that our calls to Vault
	// returns a new Intermediate certificate and further calls
	// to ActiveLeafSigningCert return the same new cert.
	newLeaf, err := provider.GenerateLeafSigningCert()
	require.NoError(t, err)

	newActive, err := provider.ActiveLeafSigningCert()
	require.NoError(t, err)

	require.Equal(t, newLeaf, newActive)
	require.NotEqual(t, orig, newLeaf)
}

func TestVaultCAProvider_AutoTidyExpiredIssuers(t *testing.T) {
	SkipIfVaultNotPresent(t)
	t.Parallel()

	testVault := NewTestVaultServer(t)
	attr := &VaultTokenAttributes{
		RootPath:         "pki-root",
		IntermediatePath: "pki-intermediate",
		ConsulManaged:    true,
	}
	token := CreateVaultTokenWithAttrs(t, testVault.client, attr)
	provider := createVaultProvider(t, true, testVault.Addr, token,
		map[string]any{
			"RootPKIPath":         "pki-root/",
			"IntermediatePKIPath": "pki-intermediate/",
		})

	version := strings.Split(vaultTestVersion, ".")
	require.Len(t, version, 3)
	minorVersion, err := strconv.Atoi(version[1])
	require.NoError(t, err)
	expIssSet, errStr := provider.autotidyIssuers("pki-intermediate/")
	switch {
	case minorVersion <= 11:
		require.False(t, expIssSet)
		require.Contains(t, errStr, "auto-tidy")
	case minorVersion == 12:
		require.False(t, expIssSet)
		require.Contains(t, errStr, "tidy_expired_issuers")
	default: // Consul 1.13+
		require.True(t, expIssSet)
	}

	// check permission denied
	expIssSet, errStr = provider.autotidyIssuers("pki-bad/")
	require.False(t, expIssSet)
	require.Contains(t, errStr, "permission denied")
}

func TestVaultCAProvider_GenerateIntermediate_inSecondary(t *testing.T) {
	SkipIfVaultNotPresent(t)

	t.Parallel()

	// Primary DC will be a consul provider.
	conf := testConsulCAConfig()
	delegate := newMockDelegate(t, conf)
	primaryProvider := TestConsulProvider(t, delegate)
	require.NoError(t, primaryProvider.Configure(testProviderConfig(conf)))
	_, err := primaryProvider.GenerateCAChain()
	require.NoError(t, err)

	// Ensure that we don't configure vault to try and mint leafs that
	// outlive their CA during the test (which hard fails in vault).
	intermediateCertTTL := getIntermediateCertTTL(t, conf)
	leafCertTTL := intermediateCertTTL - 4*time.Hour

	testVault := NewTestVaultServer(t)

	attr := &VaultTokenAttributes{
		RootPath:         "pki-root",
		IntermediatePath: "pki-intermediate",
		ConsulManaged:    true,
	}
	token := CreateVaultTokenWithAttrs(t, testVault.client, attr)

	provider := createVaultProvider(t, false, testVault.Addr, token, map[string]any{
		"LeafCertTTL":         []uint8(leafCertTTL.String()),
		"RootPKIPath":         "pki-root/",
		"IntermediatePKIPath": "pki-intermediate/",
	})

	var origIntermediate string
	testutil.RunStep(t, "initialize secondary provider", func(t *testing.T) {
		// Get the intermediate CSR from provider.
		csrPEM, issuerID, err := provider.GenerateIntermediateCSR()
		require.NoError(t, err)
		csr, err := connect.ParseCSR(csrPEM)
		require.NoError(t, err)

		// Sign the CSR with primaryProvider.
		intermediatePEM, err := primaryProvider.SignIntermediate(csr)
		require.NoError(t, err)
		rootPEM, err := primaryProvider.GenerateCAChain()
		require.NoError(t, err)

		// Give the new intermediate to provider to use.
		require.NoError(t, provider.SetIntermediate(intermediatePEM, rootPEM, issuerID))

		origIntermediate, err = provider.ActiveLeafSigningCert()
		require.NoError(t, err)
	})

	testutil.RunStep(t, "renew secondary provider", func(t *testing.T) {
		// Get the intermediate CSR from provider.
		csrPEM, issuerID, err := provider.GenerateIntermediateCSR()
		require.NoError(t, err)
		csr, err := connect.ParseCSR(csrPEM)
		require.NoError(t, err)

		// Sign the CSR with primaryProvider.
		intermediatePEM, err := primaryProvider.SignIntermediate(csr)
		require.NoError(t, err)
		rootPEM, err := primaryProvider.GenerateCAChain()
		require.NoError(t, err)

		// Give the new intermediate to provider to use.
		require.NoError(t, provider.SetIntermediate(intermediatePEM, rootPEM, issuerID))

		// This test was created to ensure that our calls to Vault
		// returns a new Intermediate certificate and further calls
		// to ActiveLeafSigningCert return the same new cert.
		newActiveIntermediate, err := provider.ActiveLeafSigningCert()
		require.NoError(t, err)

		require.NotEqual(t, origIntermediate, newActiveIntermediate)
		require.Equal(t, intermediatePEM, newActiveIntermediate)
	})
}

func TestVaultCAProvider_VaultManaged(t *testing.T) {
	SkipIfVaultNotPresent(t)

	const vaultManagedPKIPolicy = `
path "/pki-root/" {
	capabilities = [ "read" ]
}
  
path "/pki-root/root/sign-intermediate" {
	capabilities = [ "update" ]
}
  
path "/pki-intermediate/*" {
	capabilities = [ "create", "read", "update", "delete", "list" ]
}
  
path "auth/token/renew-self" {
	  capabilities = [ "update" ]
}
  
path "auth/token/lookup-self" {
	  capabilities = [ "read" ]
}
`

	testVault := NewTestVaultServer(t)

	client := testVault.Client()

	client.SetToken("root")

	// Mount pki root externally
	require.NoError(t, client.Sys().Mount("pki-root", &vaultapi.MountInput{
		Type:        "pki",
		Description: "root CA backend for Consul Connect",
		Config: vaultapi.MountConfigInput{
			MaxLeaseTTL: "12m",
		},
	}))
	_, err := client.Logical().Write("pki-root/root/generate/internal", map[string]interface{}{
		"common_name": "testconsul",
	})
	require.NoError(t, err)

	// Mount pki intermediate externally
	require.NoError(t, client.Sys().Mount("pki-intermediate", &vaultapi.MountInput{
		Type:        "pki",
		Description: "intermediate CA backend for Consul Connect",
		Config: vaultapi.MountConfigInput{
			MaxLeaseTTL: "6m",
		},
	}))

	// Generate a policy and token for the VaultProvider to use
	require.NoError(t, client.Sys().PutPolicy("consul-ca", vaultManagedPKIPolicy))
	tcr := &vaultapi.TokenCreateRequest{
		Policies: []string{"consul-ca"},
	}
	secret, err := testVault.client.Auth().Token().Create(tcr)
	require.NoError(t, err)
	providerToken := secret.Auth.ClientToken

	// We want to test the provider.Configure() step

	_ = createVaultProvider(t, true, testVault.Addr, providerToken, map[string]any{
		"RootPKIPath":         "pki-root/",
		"IntermediatePKIPath": "pki-intermediate/",
	})
}

func TestVaultCAProvider_ConsulManaged(t *testing.T) {
	SkipIfVaultNotPresent(t)

	testVault := NewTestVaultServer(t)

	client := testVault.Client()

	client.SetToken("root")

	// We do not configure any mounts and instead let Consul
	// be responsible for mounting root and intermediate PKI

	// Generate a policy and token for the VaultProvider to use
	require.NoError(t, client.Sys().PutPolicy("consul-ca", pkiTestPolicy("pki-root", "pki-intermediate")))
	tcr := &vaultapi.TokenCreateRequest{
		Policies: []string{"consul-ca"},
	}
	secret, err := testVault.client.Auth().Token().Create(tcr)
	require.NoError(t, err)
	providerToken := secret.Auth.ClientToken

	// We want to test the provider.Configure() step

	_ = createVaultProvider(t, true, testVault.Addr, providerToken, map[string]any{
		"RootPKIPath":         "pki-root/",
		"IntermediatePKIPath": "pki-intermediate/",
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

func createVaultProvider(t *testing.T, isPrimary bool, addr, token string, rawConf map[string]any) *VaultProvider {
	t.Helper()
	cfg := vaultProviderConfig(t, addr, token, rawConf)

	provider := NewVaultProvider(hclog.New(nil))

	if !isPrimary {
		cfg.IsPrimary = false
		cfg.Datacenter = "dc2"
	}

	t.Cleanup(provider.Stop)
	require.NoError(t, provider.Configure(cfg))
	if isPrimary {
		_, err := provider.GenerateCAChain()
		require.NoError(t, err)
		_, err = provider.GenerateLeafSigningCert()
		require.NoError(t, err)
	}

	return provider
}

func vaultProviderConfig(t *testing.T, addr, token string, rawConf map[string]any) ProviderConfig {
	t.Helper()
	require.NotEmpty(t, rawConf, "config map is required with at least %q and %q set",
		"RootPKIPath",
		"IntermediatePKIPath")

	conf := map[string]any{
		"Address": addr,
		"Token":   token,
		// Tests duration parsing after msgpack type mangling during raft apply.
		"LeafCertTTL": []uint8("72h"),
	}

	hasRequired := false
	if rawConf != nil {
		_, ok1 := rawConf["RootPKIPath"]
		_, ok2 := rawConf["IntermediatePKIPath"]
		hasRequired = ok1 && ok2
	}
	if !hasRequired {
		t.Fatalf("The caller must provide both %q and %q config settings to avoid an incidental collision",
			"RootPKIPath",
			"IntermediatePKIPath")
	}

	for k, v := range rawConf {
		conf[k] = v
	}

	cfg := ProviderConfig{
		ClusterID:  connect.TestClusterID,
		Datacenter: "dc1",
		IsPrimary:  true,
		RawConfig:  conf,
	}

	return cfg
}
