// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	msgpackrpc "github.com/hashicorp/consul-net-rpc/net-rpc-msgpackrpc"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/connect"
	ca "github.com/hashicorp/consul/agent/connect/ca"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
)

func testParseCert(t *testing.T, pemValue string) *x509.Certificate {
	cert, err := connect.ParseCert(pemValue)
	if err != nil {
		t.Fatal(err)
	}
	return cert
}

// Test listing root CAs.
func TestConnectCARoots(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")

	// Insert some CAs
	state := s1.fsm.State()
	ca1 := connect.TestCA(t, nil)
	ca2 := connect.TestCA(t, nil)
	ca2.Active = false
	idx, _, err := state.CARoots(nil)
	require.NoError(t, err)
	ok, err := state.CARootSetCAS(idx, idx, []*structs.CARoot{ca1, ca2})
	assert.True(t, ok)
	require.NoError(t, err)
	_, caCfg, err := state.CAConfig(nil)
	require.NoError(t, err)

	// Request
	args := &structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	var reply structs.IndexedCARoots
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConnectCA.Roots", args, &reply))

	// Verify
	assert.Equal(t, ca1.ID, reply.ActiveRootID)
	assert.Len(t, reply.Roots, 2)
	for _, r := range reply.Roots {
		// These must never be set, for security
		assert.Equal(t, "", r.SigningCert)
		assert.Equal(t, "", r.SigningKey)
	}
	assert.Equal(t, fmt.Sprintf("%s.consul", caCfg.ClusterID), reply.TrustDomain)
}

func TestConnectCAConfig_GetSet(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")

	// Get the starting config
	{
		args := &structs.DCSpecificRequest{
			Datacenter: "dc1",
		}
		var reply structs.CAConfiguration
		assert.NoError(t, msgpackrpc.CallWithCodec(codec, "ConnectCA.ConfigurationGet", args, &reply))

		actual, err := ca.ParseConsulCAConfig(reply.Config)
		assert.NoError(t, err)
		expected, err := ca.ParseConsulCAConfig(s1.config.CAConfig.Config)
		assert.NoError(t, err)
		assert.Equal(t, reply.Provider, s1.config.CAConfig.Provider)
		assert.Equal(t, actual, expected)
	}

	testState := map[string]string{"foo": "bar"}

	// Update a config value
	newConfig := &structs.CAConfiguration{
		Provider: "consul",
		Config: map[string]interface{}{
			"PrivateKey": "",
			"RootCert":   "",
			// This verifies the state persistence for providers although Consul
			// provider doesn't actually use that mechanism outside of tests.
			"test_state": testState,
		},
	}
	{
		args := &structs.CARequest{
			Datacenter: "dc1",
			Config:     newConfig,
		}
		var reply interface{}
		retry.Run(t, func(r *retry.R) {
			r.Check(msgpackrpc.CallWithCodec(codec, "ConnectCA.ConfigurationSet", args, &reply))
		})
	}

	// Verify the new config was set
	{
		args := &structs.DCSpecificRequest{
			Datacenter: "dc1",
		}
		var reply structs.CAConfiguration
		assert.NoError(t, msgpackrpc.CallWithCodec(codec, "ConnectCA.ConfigurationGet", args, &reply))

		actual, err := ca.ParseConsulCAConfig(reply.Config)
		assert.NoError(t, err)
		expected, err := ca.ParseConsulCAConfig(newConfig.Config)
		assert.NoError(t, err)
		assert.Equal(t, reply.Provider, newConfig.Provider)
		assert.Equal(t, actual, expected)
		assert.Equal(t, testState, reply.State)
	}
}

func TestConnectCAConfig_GetSet_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = TestDefaultInitialManagementToken
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	opReadToken, err := upsertTestTokenWithPolicyRules(
		codec, TestDefaultInitialManagementToken, "dc1", `operator = "read"`)
	require.NoError(t, err)

	opWriteToken, err := upsertTestTokenWithPolicyRules(
		codec, TestDefaultInitialManagementToken, "dc1", `operator = "write"`)
	require.NoError(t, err)

	// Update a config value
	newConfig := &structs.CAConfiguration{
		Provider: "consul",
		Config: map[string]interface{}{
			"PrivateKey": `
-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIMoTkpRggp3fqZzFKh82yS4LjtJI+XY+qX/7DefHFrtdoAoGCCqGSM49
AwEHoUQDQgAEADPv1RHVNRfa2VKRAB16b6rZnEt7tuhaxCFpQXPj7M2omb0B9Fav
q5E0ivpNtv1QnFhxtPd7d5k4e+T7SkW1TQ==
-----END EC PRIVATE KEY-----`,
			"RootCert": `
-----BEGIN CERTIFICATE-----
MIICjDCCAjKgAwIBAgIIC5llxGV1gB8wCgYIKoZIzj0EAwIwFDESMBAGA1UEAxMJ
VGVzdCBDQSAyMB4XDTE5MDMyMjEzNTgyNloXDTI5MDMyMjEzNTgyNlowDjEMMAoG
A1UEAxMDd2ViMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEADPv1RHVNRfa2VKR
AB16b6rZnEt7tuhaxCFpQXPj7M2omb0B9Favq5E0ivpNtv1QnFhxtPd7d5k4e+T7
SkW1TaOCAXIwggFuMA4GA1UdDwEB/wQEAwIDuDAdBgNVHSUEFjAUBggrBgEFBQcD
AgYIKwYBBQUHAwEwDAYDVR0TAQH/BAIwADBoBgNVHQ4EYQRfN2Q6MDc6ODc6M2E6
NDA6MTk6NDc6YzM6NWE6YzA6YmE6NjI6ZGY6YWY6NGI6ZDQ6MDU6MjU6NzY6M2Q6
NWE6OGQ6MTY6OGQ6Njc6NWU6MmU6YTA6MzQ6N2Q6ZGM6ZmYwagYDVR0jBGMwYYBf
ZDE6MTE6MTE6YWM6MmE6YmE6OTc6YjI6M2Y6YWM6N2I6YmQ6ZGE6YmU6YjE6OGE6
ZmM6OWE6YmE6YjU6YmM6ODM6ZTc6NWU6NDE6NmY6ZjI6NzM6OTU6NTg6MGM6ZGIw
WQYDVR0RBFIwUIZOc3BpZmZlOi8vMTExMTExMTEtMjIyMi0zMzMzLTQ0NDQtNTU1
NTU1NTU1NTU1LmNvbnN1bC9ucy9kZWZhdWx0L2RjL2RjMS9zdmMvd2ViMAoGCCqG
SM49BAMCA0gAMEUCIGC3TTvvjj76KMrguVyFf4tjOqaSCRie3nmHMRNNRav7AiEA
pY0heYeK9A6iOLrzqxSerkXXQyj5e9bE4VgUnxgPU6g=
-----END CERTIFICATE-----`,
		},
	}

	args := &structs.CARequest{
		Datacenter:   "dc1",
		Config:       newConfig,
		WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
	}
	var reply interface{}
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConnectCA.ConfigurationSet", args, &reply))

	t.Run("deny get with operator:read", func(t *testing.T) {
		args := &structs.DCSpecificRequest{
			Datacenter:   "dc1",
			QueryOptions: structs.QueryOptions{Token: opReadToken.SecretID},
		}

		var reply structs.CAConfiguration
		err = msgpackrpc.CallWithCodec(codec, "ConnectCA.ConfigurationGet", args, &reply)
		assert.True(t, acl.IsErrPermissionDenied(err))
	})

	t.Run("allow get with operator:write", func(t *testing.T) {
		args := &structs.DCSpecificRequest{
			Datacenter:   "dc1",
			QueryOptions: structs.QueryOptions{Token: opWriteToken.SecretID},
		}

		var reply structs.CAConfiguration
		err = msgpackrpc.CallWithCodec(codec, "ConnectCA.ConfigurationGet", args, &reply)
		assert.False(t, acl.IsErrPermissionDenied(err))
		assert.Equal(t, newConfig.Config, reply.Config)
	})
}

// This test case tests that the logic around forcing a rotation without cross
// signing works when requested (and is denied when not requested). This occurs
// if the current CA is not able to cross sign external CA certificates.
func TestConnectCAConfig_GetSetForceNoCrossSigning(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	// Setup a server with a built-in CA that as artificially disabled cross
	// signing. This is simpler than running tests with external CA dependencies.
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.CAConfig.Config["DisableCrossSigning"] = true
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")

	// Store the current root
	rootReq := &structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	var rootList structs.IndexedCARoots
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConnectCA.Roots", rootReq, &rootList))
	require.Len(t, rootList.Roots, 1)
	oldRoot := rootList.Roots[0]

	// Get the starting config
	{
		args := &structs.DCSpecificRequest{
			Datacenter: "dc1",
		}
		var reply structs.CAConfiguration
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConnectCA.ConfigurationGet", args, &reply))

		actual, err := ca.ParseConsulCAConfig(reply.Config)
		require.NoError(t, err)
		expected, err := ca.ParseConsulCAConfig(s1.config.CAConfig.Config)
		require.NoError(t, err)
		require.Equal(t, reply.Provider, s1.config.CAConfig.Provider)
		require.Equal(t, actual, expected)
	}

	// Update to a new CA with different key. This should fail since the existing
	// CA doesn't support cross signing so can't rotate safely.
	_, newKey, err := connect.GeneratePrivateKey()
	require.NoError(t, err)
	newConfig := &structs.CAConfiguration{
		Provider: "consul",
		Config: map[string]interface{}{
			"PrivateKey": newKey,
		},
	}
	{
		args := &structs.CARequest{
			Datacenter: "dc1",
			Config:     newConfig,
		}
		var reply interface{}
		err := msgpackrpc.CallWithCodec(codec, "ConnectCA.ConfigurationSet", args, &reply)
		require.EqualError(t, err, "The current CA Provider does not support cross-signing. "+
			"You can try again with ForceWithoutCrossSigningSet but this may cause disruption"+
			" - see documentation for more.")
	}

	// Now try again with the force flag set and it should work
	{
		newConfig.ForceWithoutCrossSigning = true
		args := &structs.CARequest{
			Datacenter: "dc1",
			Config:     newConfig,
		}
		var reply interface{}
		err := msgpackrpc.CallWithCodec(codec, "ConnectCA.ConfigurationSet", args, &reply)
		require.NoError(t, err)
	}

	// Make sure the new root has been added but with no cross-signed intermediate
	{
		args := &structs.DCSpecificRequest{
			Datacenter: "dc1",
		}
		var reply structs.IndexedCARoots
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConnectCA.Roots", args, &reply))
		require.Len(t, reply.Roots, 2)

		for _, r := range reply.Roots {
			if r.ID == oldRoot.ID {
				// The old root should no longer be marked as the active root,
				// and none of its other fields should have changed.
				require.False(t, r.Active)
				require.Equal(t, r.Name, oldRoot.Name)
				require.Equal(t, r.RootCert, oldRoot.RootCert)
				require.Equal(t, r.SigningCert, oldRoot.SigningCert)
				require.Equal(t, r.IntermediateCerts, oldRoot.IntermediateCerts)
			} else {
				// The new root should NOT have a valid cross-signed cert from the old
				// root as an intermediate.
				require.True(t, r.Active)
				require.Empty(t, r.IntermediateCerts)
			}
		}
	}
}

func TestConnectCAConfig_TriggerRotation(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	cases := []struct {
		name     string
		configFn func() (*structs.CAConfiguration, error)
	}{
		{
			name: "new private key provided",
			configFn: func() (*structs.CAConfiguration, error) {
				// Update the provider config to use a new private key, which should
				// cause a rotation.
				_, newKey, err := connect.GeneratePrivateKey()
				if err != nil {
					return nil, err
				}

				return &structs.CAConfiguration{
					Provider: "consul",
					Config: map[string]interface{}{
						"PrivateKey": newKey,
						"RootCert":   "",
					},
				}, nil
			},
		},
		{
			name: "update private key bits",
			configFn: func() (*structs.CAConfiguration, error) {
				return &structs.CAConfiguration{
					Provider: "consul",
					Config: map[string]interface{}{
						"PrivateKeyType": "ec",
						"PrivateKeyBits": 384,
					},
				}, nil
			},
		},
		{
			name: "update private key type",
			configFn: func() (*structs.CAConfiguration, error) {
				return &structs.CAConfiguration{
					Provider: "consul",
					Config: map[string]interface{}{
						"PrivateKeyType": "rsa",
						"PrivateKeyBits": "2048",
					},
				}, nil
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir1, s1 := testServer(t)
			defer os.RemoveAll(dir1)
			defer s1.Shutdown()
			codec := rpcClient(t, s1)
			defer codec.Close()

			testrpc.WaitForTestAgent(t, s1.RPC, "dc1")

			// Store the current root
			rootReq := &structs.DCSpecificRequest{
				Datacenter: "dc1",
			}
			var rootList structs.IndexedCARoots
			require.Nil(t, msgpackrpc.CallWithCodec(codec, "ConnectCA.Roots", rootReq, &rootList))
			assert.Len(t, rootList.Roots, 1)
			oldRoot := rootList.Roots[0]

			newConfig, err := tc.configFn()
			require.NoError(t, err)

			{
				args := &structs.CARequest{
					Datacenter: "dc1",
					Config:     newConfig,
				}
				var reply interface{}

				require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConnectCA.ConfigurationSet", args, &reply))
			}

			// Make sure the new root has been added along with an intermediate
			// cross-signed by the old root.
			var newRootPEM string
			testutil.RunStep(t, "ensure roots look correct", func(t *testing.T) {
				args := &structs.DCSpecificRequest{
					Datacenter: "dc1",
				}
				var reply structs.IndexedCARoots
				require.Nil(t, msgpackrpc.CallWithCodec(codec, "ConnectCA.Roots", args, &reply))
				assert.Len(t, reply.Roots, 2)

				for _, r := range reply.Roots {
					if r.ID == oldRoot.ID {
						// The old root should no longer be marked as the active root,
						// and none of its other fields should have changed.
						assert.False(t, r.Active)
						assert.Equal(t, r.Name, oldRoot.Name)
						assert.Equal(t, r.RootCert, oldRoot.RootCert)
						assert.Equal(t, r.SigningCert, oldRoot.SigningCert)
						assert.Equal(t, r.IntermediateCerts, oldRoot.IntermediateCerts)
					} else {
						newRootPEM = r.RootCert
						// The new root should have a valid cross-signed cert from the old
						// root as an intermediate.
						assert.True(t, r.Active)
						assert.Len(t, r.IntermediateCerts, 1)

						xc := testParseCert(t, r.IntermediateCerts[0])
						oldRootCert := testParseCert(t, oldRoot.RootCert)
						newRootCert := testParseCert(t, r.RootCert)

						// Should have the authority key ID and signature algo of the
						// (old) signing CA.
						assert.Equal(t, xc.AuthorityKeyId, oldRootCert.AuthorityKeyId)
						assert.NotEqual(t, xc.SubjectKeyId, oldRootCert.SubjectKeyId)
						assert.Equal(t, xc.SignatureAlgorithm, oldRootCert.SignatureAlgorithm)

						// The common name and SAN should not have changed.
						assert.Equal(t, xc.Subject.CommonName, newRootCert.Subject.CommonName)
						assert.Equal(t, xc.URIs, newRootCert.URIs)
					}
				}
			})

			testutil.RunStep(t, "verify the new config was set", func(t *testing.T) {
				args := &structs.DCSpecificRequest{
					Datacenter: "dc1",
				}
				var reply structs.CAConfiguration
				require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConnectCA.ConfigurationGet", args, &reply))

				actual, err := ca.ParseConsulCAConfig(reply.Config)
				require.NoError(t, err)
				expected, err := ca.ParseConsulCAConfig(newConfig.Config)
				require.NoError(t, err)
				assert.Equal(t, reply.Provider, newConfig.Provider)
				assert.Equal(t, actual, expected)
			})

			testutil.RunStep(t, "verify that new leaf certs get the cross-signed intermediate bundled", func(t *testing.T) {
				// Generate a CSR and request signing
				spiffeId := connect.TestSpiffeIDService(t, "web")
				csr, _ := connect.TestCSR(t, spiffeId)
				args := &structs.CASignRequest{
					Datacenter: "dc1",
					CSR:        csr,
				}
				var reply structs.IssuedCert
				require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConnectCA.Sign", args, &reply))

				testutil.RunStep(t, "verify that the cert is signed by the new CA", func(t *testing.T) {
					roots := x509.NewCertPool()
					require.True(t, roots.AppendCertsFromPEM([]byte(newRootPEM)))
					leaf, err := connect.ParseCert(reply.CertPEM)
					require.NoError(t, err)
					_, err = leaf.Verify(x509.VerifyOptions{
						Roots: roots,
					})
					require.NoError(t, err)
				})

				testutil.RunStep(t, "and that it validates via the intermediate", func(t *testing.T) {
					roots := x509.NewCertPool()
					assert.True(t, roots.AppendCertsFromPEM([]byte(oldRoot.RootCert)))
					leaf, err := connect.ParseCert(reply.CertPEM)
					require.NoError(t, err)

					// Make sure the intermediate was returned as well as leaf
					_, rest := pem.Decode([]byte(reply.CertPEM))
					require.NotEmpty(t, rest)

					intermediates := x509.NewCertPool()
					require.True(t, intermediates.AppendCertsFromPEM(rest))

					_, err = leaf.Verify(x509.VerifyOptions{
						Roots:         roots,
						Intermediates: intermediates,
					})
					require.NoError(t, err)
				})

				testutil.RunStep(t, "verify other fields", func(t *testing.T) {
					assert.Equal(t, "web", reply.Service)
					assert.Equal(t, spiffeId.URI().String(), reply.ServiceURI)
				})
			})
		})
	}
}

func TestConnectCAConfig_Vault_TriggerRotation_Fails(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	ca.SkipIfVaultNotPresent(t)

	t.Parallel()

	testVault := ca.NewTestVaultServer(t)

	token1 := ca.CreateVaultTokenWithAttrs(t, testVault.Client(), &ca.VaultTokenAttributes{
		RootPath:         "pki-root",
		IntermediatePath: "pki-primary",
		ConsulManaged:    true,
		WithSudo:         true,
	})

	token2 := ca.CreateVaultTokenWithAttrs(t, testVault.Client(), &ca.VaultTokenAttributes{
		RootPath:         "pki-root",
		IntermediatePath: "pki-intermediate",
		ConsulManaged:    true,
	})

	newConfig := func(token string, keyType string, keyBits int) map[string]any {
		return map[string]any{
			"Address":             testVault.Addr,
			"Token":               token,
			"RootPKIPath":         "pki-root/",
			"IntermediatePKIPath": "pki-intermediate/",
			"PrivateKeyType":      keyType,
			"PrivateKeyBits":      keyBits,
		}
	}

	_, s1 := testServerWithConfig(t, func(c *Config) {
		c.CAConfig = &structs.CAConfiguration{
			Provider: "vault",
			Config:   newConfig(token1, connect.DefaultPrivateKeyType, connect.DefaultPrivateKeyBits),
		}
	})
	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")

	// note: unlike many table tests, the ordering of these cases does matter
	// because any non-errored case will modify the CA config, and any subsequent
	// tests will use the same agent with that new CA config.
	testSteps := []struct {
		name      string
		configFn  func() *structs.CAConfiguration
		expectErr string
	}{
		{
			name: "allow modifying key type and bits from default",
			configFn: func() *structs.CAConfiguration {
				return &structs.CAConfiguration{
					Provider:                 "vault",
					Config:                   newConfig(token2, "rsa", 4096),
					ForceWithoutCrossSigning: true,
				}
			},
		},
		{
			name: "error when trying to modify key bits",
			configFn: func() *structs.CAConfiguration {
				return &structs.CAConfiguration{
					Provider:                 "vault",
					Config:                   newConfig(token2, "rsa", 2048),
					ForceWithoutCrossSigning: true,
				}
			},
			expectErr: `cannot update the PrivateKeyBits field without changing RootPKIPath`,
		},
		{
			name: "error when trying to modify key type",
			configFn: func() *structs.CAConfiguration {
				return &structs.CAConfiguration{
					Provider:                 "vault",
					Config:                   newConfig(token2, "ec", 256),
					ForceWithoutCrossSigning: true,
				}
			},
			expectErr: `cannot update the PrivateKeyType field without changing RootPKIPath`,
		},
		{
			name: "allow update that does not change key type or bits",
			configFn: func() *structs.CAConfiguration {
				return &structs.CAConfiguration{
					Provider:                 "vault",
					Config:                   newConfig(token2, "rsa", 4096),
					ForceWithoutCrossSigning: true,
				}
			},
		},
	}

	for _, tc := range testSteps {
		testutil.RunStep(t, tc.name, func(t *testing.T) {
			args := &structs.CARequest{
				Datacenter: "dc1",
				Config:     tc.configFn(),
			}
			var reply interface{}

			codec := rpcClient(t, s1)
			err := msgpackrpc.CallWithCodec(codec, "ConnectCA.ConfigurationSet", args, &reply)
			if tc.expectErr == "" {
				require.NoError(t, err)
			} else {
				testutil.RequireErrorContains(t, err, tc.expectErr)
			}
		})
	}
}

func TestConnectCAConfig_UpdateSecondary(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	// Initialize primary as the primary DC
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "primary"
		c.PrimaryDatacenter = "primary"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "primary")

	// secondary as a secondary DC
	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "secondary"
		c.PrimaryDatacenter = "primary"
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()
	codec := rpcClient(t, s2)
	defer codec.Close()

	// Create the WAN link
	joinWAN(t, s2, s1)
	testrpc.WaitForLeader(t, s2.RPC, "secondary")

	// Capture the current root
	rootList, activeRoot, err := getTestRoots(s1, "primary")
	require.NoError(t, err)
	require.Len(t, rootList.Roots, 1)
	rootCert := activeRoot

	testrpc.WaitForActiveCARoot(t, s1.RPC, "primary", rootCert)
	testrpc.WaitForActiveCARoot(t, s2.RPC, "secondary", rootCert)

	// Capture the current intermediate
	rootList, activeRoot, err = getTestRoots(s2, "secondary")
	require.NoError(t, err)
	require.Len(t, rootList.Roots, 1)
	require.Len(t, activeRoot.IntermediateCerts, 1)
	oldIntermediatePEM := activeRoot.IntermediateCerts[0]

	// Update the secondary CA config to use a new private key, which should
	// cause a re-signing with a new intermediate.
	_, newKey, err := connect.GeneratePrivateKey()
	assert.NoError(t, err)
	newConfig := &structs.CAConfiguration{
		Provider: "consul",
		Config: map[string]interface{}{
			"PrivateKey": newKey,
			"RootCert":   "",
		},
	}
	{
		args := &structs.CARequest{
			Datacenter: "secondary",
			Config:     newConfig,
		}
		var reply interface{}

		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConnectCA.ConfigurationSet", args, &reply))
	}

	// Make sure the new intermediate has replaced the old one in the active root,
	// and that the root itself hasn't changed.
	var newIntermediatePEM string
	{
		args := &structs.DCSpecificRequest{
			Datacenter: "secondary",
		}
		var reply structs.IndexedCARoots
		require.Nil(t, msgpackrpc.CallWithCodec(codec, "ConnectCA.Roots", args, &reply))
		require.Len(t, reply.Roots, 1)
		require.Len(t, reply.Roots[0].IntermediateCerts, 1)
		newIntermediatePEM = reply.Roots[0].IntermediateCerts[0]
		require.NotEqual(t, oldIntermediatePEM, newIntermediatePEM)
		require.Equal(t, reply.Roots[0].RootCert, rootCert.RootCert)
	}

	// Verify the new config was set.
	{
		args := &structs.DCSpecificRequest{
			Datacenter: "secondary",
		}
		var reply structs.CAConfiguration
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConnectCA.ConfigurationGet", args, &reply))

		actual, err := ca.ParseConsulCAConfig(reply.Config)
		require.NoError(t, err)
		expected, err := ca.ParseConsulCAConfig(newConfig.Config)
		require.NoError(t, err)
		assert.Equal(t, reply.Provider, newConfig.Provider)
		assert.Equal(t, actual, expected)
	}

	// Verify that new leaf certs get the new intermediate bundled
	{
		// Generate a CSR and request signing
		spiffeId := connect.TestSpiffeIDServiceWithHostDC(t, "web", connect.TestClusterID+".consul", "secondary")
		csr, _ := connect.TestCSR(t, spiffeId)
		args := &structs.CASignRequest{
			Datacenter: "secondary",
			CSR:        csr,
		}
		var reply structs.IssuedCert
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConnectCA.Sign", args, &reply))

		// Verify the leaf cert has the new intermediate.
		{
			roots := x509.NewCertPool()
			assert.True(t, roots.AppendCertsFromPEM([]byte(rootCert.RootCert)))
			leaf, err := connect.ParseCert(reply.CertPEM)
			require.NoError(t, err)

			intermediates := x509.NewCertPool()
			require.True(t, intermediates.AppendCertsFromPEM([]byte(newIntermediatePEM)))

			_, err = leaf.Verify(x509.VerifyOptions{
				Roots:         roots,
				Intermediates: intermediates,
			})
			require.NoError(t, err)
		}

		// Verify other fields
		assert.Equal(t, "web", reply.Service)
		assert.Equal(t, spiffeId.URI().String(), reply.ServiceURI)
	}

	// Update a minor field in the config that doesn't trigger an intermediate refresh.
	{
		newConfig := &structs.CAConfiguration{
			Provider: "consul",
			Config: map[string]interface{}{
				"PrivateKey": newKey,
				"RootCert":   "",
			},
		}
		{
			args := &structs.CARequest{
				Datacenter: "secondary",
				Config:     newConfig,
			}
			var reply interface{}

			require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConnectCA.ConfigurationSet", args, &reply))
		}
	}
}

// Test CA signing
func TestConnectCASign(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	tests := []struct {
		caKeyType string
		caKeyBits int
	}{
		{
			caKeyType: connect.DefaultPrivateKeyType,
			caKeyBits: connect.DefaultPrivateKeyBits,
		},
		{
			// Ensure that an RSA Keyed CA can sign EC leaves and they validate.
			caKeyType: "rsa",
			caKeyBits: 2048,
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s-%d", tt.caKeyType, tt.caKeyBits), func(t *testing.T) {
			dir1, s1 := testServerWithConfig(t, func(cfg *Config) {
				cfg.PrimaryDatacenter = "dc1"
				cfg.CAConfig.Config["PrivateKeyType"] = tt.caKeyType
				cfg.CAConfig.Config["PrivateKeyBits"] = tt.caKeyBits
			})
			defer os.RemoveAll(dir1)
			defer s1.Shutdown()
			codec := rpcClient(t, s1)
			defer codec.Close()

			testrpc.WaitForLeader(t, s1.RPC, "dc1")

			// Generate a CSR and request signing
			spiffeId := connect.TestSpiffeIDService(t, "web")

			// TestCSR will always generate a CSR with an EC key currently.
			csr, _ := connect.TestCSR(t, spiffeId)
			args := &structs.CASignRequest{
				Datacenter: "dc1",
				CSR:        csr,
			}
			var reply structs.IssuedCert
			require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConnectCA.Sign", args, &reply))

			// Generate a second CSR and request signing
			spiffeId2 := connect.TestSpiffeIDService(t, "web2")
			csr, _ = connect.TestCSR(t, spiffeId2)
			args = &structs.CASignRequest{
				Datacenter: "dc1",
				CSR:        csr,
			}

			var reply2 structs.IssuedCert
			require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConnectCA.Sign", args, &reply2))
			require.True(t, reply2.ModifyIndex > reply.ModifyIndex)

			// Get the current CA
			state := s1.fsm.State()
			_, ca, err := state.CARootActive(nil)
			require.NoError(t, err)

			// Verify that the cert is signed by the CA
			require.NoError(t, connect.ValidateLeaf(ca.RootCert, reply.CertPEM, nil))

			// Verify other fields
			assert.Equal(t, "web", reply.Service)
			assert.Equal(t, spiffeId.URI().String(), reply.ServiceURI)
		})
	}
}

// Bench how long Signing RPC takes. This was used to ballpark reasonable
// default rate limit to protect servers from thundering herds of signing
// requests on root rotation.
func BenchmarkConnectCASign(b *testing.B) {
	t := &testing.T{}

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Generate a CSR and request signing
	spiffeID := connect.TestSpiffeIDService(b, "web")
	csr, _ := connect.TestCSR(b, spiffeID)
	args := &structs.CASignRequest{
		Datacenter: "dc1",
		CSR:        csr,
	}
	var reply structs.IssuedCert

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		if err := msgpackrpc.CallWithCodec(codec, "ConnectCA.Sign", args, &reply); err != nil {
			b.Fatalf("err: %v", err)
		}
	}
}

func TestConnectCASign_rateLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.PrimaryDatacenter = "dc1"
		c.Bootstrap = true
		c.CAConfig.Config = map[string]interface{}{
			// It actually doesn't work as expected with some higher values because
			// the token bucket is initialized with max(10%, 1) burst which for small
			// values is 1 and then the test completes so fast it doesn't actually
			// replenish any tokens so you only get the burst allowed through. This is
			// OK, running the test slower is likely to be more brittle anyway since
			// it will become more timing dependent whether the actual rate the
			// requests are made matches the expectation from the sleeps etc.
			"CSRMaxPerSecond": 1,
		}
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Generate a CSR and request signing a few times in a loop.
	spiffeID := connect.TestSpiffeIDService(t, "web")
	csr, _ := connect.TestCSR(t, spiffeID)
	args := &structs.CASignRequest{
		Datacenter: "dc1",
		CSR:        csr,
	}
	var reply structs.IssuedCert

	errs := make([]error, 10)
	for i := 0; i < len(errs); i++ {
		errs[i] = msgpackrpc.CallWithCodec(codec, "ConnectCA.Sign", args, &reply)
	}

	limitedCount := 0
	successCount := 0
	for _, err := range errs {
		if err == nil {
			successCount++
		} else if err.Error() == ErrRateLimited.Error() {
			limitedCount++
		} else {
			require.NoError(t, err)
		}
	}
	// I've only ever seen this as 1/9 however if the test runs slowly on an
	// over-subscribed CPU (e.g. in CI) it's possible that later requests could
	// have had their token replenished and succeed so we allow a little slack -
	// the test here isn't really the exact token bucket response more a sanity
	// check that some limiting is being applied. Note that we can't just measure
	// the time it took to send them all and infer how many should have succeeded
	// without some complex modeling of the token bucket algorithm.
	require.Truef(t, successCount >= 1, "at least 1 CSRs should have succeeded, got %d", successCount)
	require.Truef(t, limitedCount >= 7, "at least 7 CSRs should have been rate limited, got %d", limitedCount)
}

func TestConnectCASign_concurrencyLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.PrimaryDatacenter = "dc1"
		c.Bootstrap = true
		c.CAConfig.Config = map[string]interface{}{
			// Must disable the rate limit since it takes precedence
			"CSRMaxPerSecond":  0,
			"CSRMaxConcurrent": 1,
		}
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Generate a CSR and request signing a few times in a loop.
	spiffeID := connect.TestSpiffeIDService(t, "web")
	csr, _ := connect.TestCSR(t, spiffeID)
	args := &structs.CASignRequest{
		Datacenter: "dc1",
		CSR:        csr,
	}

	var wg sync.WaitGroup

	errs := make(chan error, 10)
	times := make(chan time.Duration, cap(errs))
	start := time.Now()
	for i := 0; i < cap(errs); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codec := rpcClient(t, s1)
			defer codec.Close()
			var reply structs.IssuedCert
			errs <- msgpackrpc.CallWithCodec(codec, "ConnectCA.Sign", args, &reply)
			times <- time.Since(start)
		}()
	}

	wg.Wait()
	close(errs)

	limitedCount := 0
	successCount := 0
	var minTime, maxTime time.Duration
	for err := range errs {
		elapsed := <-times
		if elapsed < minTime || minTime == 0 {
			minTime = elapsed
		}
		if elapsed > maxTime {
			maxTime = elapsed
		}
		if err == nil {
			successCount++
		} else if err.Error() == ErrRateLimited.Error() {
			limitedCount++
		} else {
			require.NoError(t, err)
		}
	}

	// These are very hand wavy - on my mac times look like this:
	//     2.776009ms
	//     3.705813ms
	//     4.527212ms
	//     5.267755ms
	//     6.119809ms
	//     6.958083ms
	//     7.869179ms
	//     8.675058ms
	//     9.512281ms
	//     10.238183ms
	//
	// But it's indistinguishable from noise - even if you disable the concurrency
	// limiter you get pretty much the same pattern/spread.
	//
	// On the other hand it's only timing that stops us from not hitting the 500ms
	// timeout. On highly CPU constrained CI box this could be brittle if we
	// assert that we never get rate limited.
	//
	// So this test is not super strong - but it's a sanity check at least that
	// things don't break when configured this way, and through manual
	// inspection/debug logging etc. we can verify it's actually doing the
	// concurrency limit thing. If you add a 100ms sleep into the sign endpoint
	// after the rate limit code for example it makes it much more obvious:
	//
	//   With 100ms sleep an no concurrency limit:
	//     min=109ms, max=118ms
	//   With concurrency limit of 1:
	//     min=106ms, max=538ms (with ~half hitting the 500ms timeout)
	//
	// Without instrumenting the endpoint to make the RPC take an artificially
	// long time it's hard to know what else we can do to actively detect that the
	// requests were serialized.
	t.Logf("min=%s, max=%s", minTime, maxTime)
	//t.Fail() // Uncomment to see the time spread logged
	require.Truef(t, successCount >= 1, "at least 1 CSRs should have succeeded, got %d", successCount)
}

func TestConnectCASignValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	webToken := createToken(t, codec, `service "web" { policy = "write" }`)
	testWebID := connect.TestSpiffeIDService(t, "web")

	tests := []struct {
		name    string
		id      connect.CertURI
		wantErr string
	}{
		{
			name: "different cluster",
			id: &connect.SpiffeIDService{
				Host:       "55555555-4444-3333-2222-111111111111.consul",
				Namespace:  testWebID.Namespace,
				Datacenter: testWebID.Datacenter,
				Service:    testWebID.Service,
			},
			wantErr: "different trust domain",
		},
		{
			name:    "same cluster should validate",
			id:      testWebID,
			wantErr: "",
		},
		{
			name: "same cluster, CSR for a different DC should NOT validate",
			id: &connect.SpiffeIDService{
				Host:       testWebID.Host,
				Namespace:  testWebID.Namespace,
				Datacenter: "dc2",
				Service:    testWebID.Service,
			},
			wantErr: "different datacenter",
		},
		{
			name: "same cluster and DC, different service should not have perms",
			id: &connect.SpiffeIDService{
				Host:       testWebID.Host,
				Namespace:  testWebID.Namespace,
				Datacenter: testWebID.Datacenter,
				Service:    "db",
			},
			wantErr: "Permission denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			csr, _ := connect.TestCSR(t, tt.id)
			args := &structs.CASignRequest{
				Datacenter:   "dc1",
				CSR:          csr,
				WriteRequest: structs.WriteRequest{Token: webToken},
			}
			var reply structs.IssuedCert
			err := msgpackrpc.CallWithCodec(codec, "ConnectCA.Sign", args, &reply)
			if tt.wantErr == "" {
				require.NoError(t, err)
				// No other validation that is handled in different tests
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}
