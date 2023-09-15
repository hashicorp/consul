// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"context"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	msgpackrpc "github.com/hashicorp/consul-net-rpc/net-rpc-msgpackrpc"
	"github.com/hashicorp/go-uuid"
	"github.com/stretchr/testify/require"
	"gotest.tools/v3/assert"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/connect/ca"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
)

func TestConnectCA_ConfigurationSet_ChangeKeyConfig_Primary(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	types := []struct {
		keyType string
		keyBits int
	}{
		{connect.DefaultPrivateKeyType, connect.DefaultPrivateKeyBits},
		// {"ec", 256}, skip since values are same as Defaults
		{"ec", 384},
		{"rsa", 2048},
		{"rsa", 4096},
	}

	for _, src := range types {
		for _, dst := range types {
			if src == dst {
				continue // skip
			}
			src := src
			dst := dst
			t.Run(fmt.Sprintf("%s-%d to %s-%d", src.keyType, src.keyBits, dst.keyType, dst.keyBits), func(t *testing.T) {
				t.Parallel()

				providerState := map[string]string{"foo": "dc1-value"}

				// Initialize primary as the primary DC
				_, srv := testServerWithConfig(t, func(c *Config) {
					c.Datacenter = "dc1"
					c.PrimaryDatacenter = "dc1"
					c.Build = "1.6.0"
					c.CAConfig.Config["PrivateKeyType"] = src.keyType
					c.CAConfig.Config["PrivateKeyBits"] = src.keyBits
					c.CAConfig.Config["test_state"] = providerState
				})
				codec := rpcClient(t, srv)

				waitForLeaderEstablishment(t, srv)
				testrpc.WaitForActiveCARoot(t, srv.RPC, "dc1", nil)

				var (
					provider ca.Provider
					caRoot   *structs.CARoot
				)
				retry.Run(t, func(r *retry.R) {
					provider, caRoot = getCAProviderWithLock(srv)
					require.NotNil(r, caRoot)
					// Sanity check CA is using the correct key type
					require.Equal(r, src.keyType, caRoot.PrivateKeyType)
					require.Equal(r, src.keyBits, caRoot.PrivateKeyBits)
				})

				testutil.RunStep(t, "sign leaf cert and make sure chain is correct", func(t *testing.T) {
					spiffeService := &connect.SpiffeIDService{
						Host:       "node1",
						Namespace:  "default",
						Datacenter: "dc1",
						Service:    "foo",
					}
					raw, _ := connect.TestCSR(t, spiffeService)

					leafCsr, err := connect.ParseCSR(raw)
					require.NoError(t, err)

					leafPEM, err := provider.Sign(leafCsr)
					require.NoError(t, err)

					// Check that the leaf signed by the new cert can be verified using the
					// returned cert chain
					require.NoError(t, connect.ValidateLeaf(caRoot.RootCert, leafPEM, []string{}))
				})

				testutil.RunStep(t, "verify persisted state is correct", func(t *testing.T) {
					state := srv.fsm.State()
					_, caConfig, err := state.CAConfig(nil)
					require.NoError(t, err)
					require.Equal(t, providerState, caConfig.State)
				})

				testutil.RunStep(t, "change roots", func(t *testing.T) {
					// Update a config value
					newConfig := &structs.CAConfiguration{
						Provider: "consul",
						Config: map[string]interface{}{
							"PrivateKey":     "",
							"RootCert":       "",
							"PrivateKeyType": dst.keyType,
							"PrivateKeyBits": dst.keyBits,
							// This verifies the state persistence for providers although Consul
							// provider doesn't actually use that mechanism outside of tests.
							"test_state": providerState,
						},
					}

					args := &structs.CARequest{
						Datacenter: "dc1",
						Config:     newConfig,
					}
					var reply interface{}
					require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConnectCA.ConfigurationSet", args, &reply))
				})

				var (
					newProvider ca.Provider
					newCaRoot   *structs.CARoot
				)
				retry.Run(t, func(r *retry.R) {
					newProvider, newCaRoot = getCAProviderWithLock(srv)
					require.NotNil(r, newCaRoot)
					// Sanity check CA is using the correct key type
					require.Equal(r, dst.keyType, newCaRoot.PrivateKeyType)
					require.Equal(r, dst.keyBits, newCaRoot.PrivateKeyBits)
				})

				testutil.RunStep(t, "sign leaf cert and make sure NEW chain is correct", func(t *testing.T) {
					spiffeService := &connect.SpiffeIDService{
						Host:       "node1",
						Namespace:  "default",
						Datacenter: "dc1",
						Service:    "foo",
					}
					raw, _ := connect.TestCSR(t, spiffeService)

					leafCsr, err := connect.ParseCSR(raw)
					require.NoError(t, err)

					leafPEM, err := newProvider.Sign(leafCsr)
					require.NoError(t, err)

					// Check that the leaf signed by the new cert can be verified using the
					// returned cert chain
					require.NoError(t, connect.ValidateLeaf(newCaRoot.RootCert, leafPEM, []string{}))
				})

				testutil.RunStep(t, "verify persisted state is still correct", func(t *testing.T) {
					state := srv.fsm.State()
					_, caConfig, err := state.CAConfig(nil)
					require.NoError(t, err)
					require.Equal(t, providerState, caConfig.State)
				})
			})
		}
	}

}

func TestCAManager_Initialize_Secondary(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	tests := []struct {
		keyType string
		keyBits int
	}{
		{connect.DefaultPrivateKeyType, connect.DefaultPrivateKeyBits},
		{"rsa", 2048},
	}

	dc1State := map[string]string{"foo": "dc1-value"}
	dc2State := map[string]string{"foo": "dc2-value"}

	for _, tc := range tests {
		tc := tc
		t.Run(fmt.Sprintf("%s-%d", tc.keyType, tc.keyBits), func(t *testing.T) {
			initialManagementToken := "8a85f086-dd95-4178-b128-e10902767c5c"

			// Initialize primary as the primary DC
			dir1, s1 := testServerWithConfig(t, func(c *Config) {
				c.Datacenter = "primary"
				c.PrimaryDatacenter = "primary"
				c.Build = "1.6.0"
				c.ACLsEnabled = true
				c.ACLInitialManagementToken = initialManagementToken
				c.ACLResolverSettings.ACLDefaultPolicy = "deny"
				c.CAConfig.Config["PrivateKeyType"] = tc.keyType
				c.CAConfig.Config["PrivateKeyBits"] = tc.keyBits
				c.CAConfig.Config["test_state"] = dc1State
			})
			defer os.RemoveAll(dir1)
			defer s1.Shutdown()

			s1.tokens.UpdateAgentToken(initialManagementToken, token.TokenSourceConfig)

			testrpc.WaitForLeader(t, s1.RPC, "primary")

			// secondary as a secondary DC
			dir2, s2 := testServerWithConfig(t, func(c *Config) {
				c.Datacenter = "secondary"
				c.PrimaryDatacenter = "primary"
				c.Build = "1.6.0"
				c.ACLsEnabled = true
				c.ACLResolverSettings.ACLDefaultPolicy = "deny"
				c.ACLTokenReplication = true
				c.CAConfig.Config["PrivateKeyType"] = tc.keyType
				c.CAConfig.Config["PrivateKeyBits"] = tc.keyBits
				c.CAConfig.Config["test_state"] = dc2State
			})
			defer os.RemoveAll(dir2)
			defer s2.Shutdown()

			s2.tokens.UpdateAgentToken(initialManagementToken, token.TokenSourceConfig)
			s2.tokens.UpdateReplicationToken(initialManagementToken, token.TokenSourceConfig)

			// Create the WAN link
			joinWAN(t, s2, s1)

			testrpc.WaitForLeader(t, s2.RPC, "secondary")

			// Ensure s2 is authoritative.
			waitForNewACLReplication(t, s2, structs.ACLReplicateTokens, 1, 1, 0)

			// Wait until the providers are fully bootstrapped.
			var (
				caRoot            *structs.CARoot
				secondaryProvider ca.Provider
				intermediatePEM   string
				err               error
			)
			retry.Run(t, func(r *retry.R) {
				_, caRoot = getCAProviderWithLock(s1)
				secondaryProvider, _ = getCAProviderWithLock(s2)
				intermediatePEM, err = secondaryProvider.ActiveLeafSigningCert()
				require.NoError(r, err)

				// Sanity check CA is using the correct key type
				require.Equal(r, tc.keyType, caRoot.PrivateKeyType)
				require.Equal(r, tc.keyBits, caRoot.PrivateKeyBits)

				// Verify the root lists are equal in each DC's state store.
				state1 := s1.fsm.State()
				_, roots1, err := state1.CARoots(nil)
				require.NoError(r, err)

				state2 := s2.fsm.State()
				_, roots2, err := state2.CARoots(nil)
				require.NoError(r, err)
				require.Len(r, roots1, 1)
				require.Len(r, roots2, 1)
				require.Equal(r, roots1[0].ID, roots2[0].ID)
				require.Equal(r, roots1[0].RootCert, roots2[0].RootCert)
				require.Empty(r, roots1[0].IntermediateCerts)
				require.NotEmpty(r, roots2[0].IntermediateCerts)
			})

			// Have secondary sign a leaf cert and make sure the chain is correct.
			spiffeService := &connect.SpiffeIDService{
				Host:       "node1",
				Namespace:  "default",
				Datacenter: "primary",
				Service:    "foo",
			}
			raw, _ := connect.TestCSR(t, spiffeService)

			leafCsr, err := connect.ParseCSR(raw)
			require.NoError(t, err)

			leafPEM, err := secondaryProvider.Sign(leafCsr)
			require.NoError(t, err)

			// Check that the leaf signed by the new cert can be verified using the
			// returned cert chain (signed intermediate + remote root).
			require.NoError(t, connect.ValidateLeaf(caRoot.RootCert, leafPEM, []string{intermediatePEM}))

			// Verify that both primary and secondary persisted state as expected -
			// pass through from the config.
			{
				state := s1.fsm.State()
				_, caConfig, err := state.CAConfig(nil)
				require.NoError(t, err)
				require.Equal(t, dc1State, caConfig.State)
			}
			{
				state := s2.fsm.State()
				_, caConfig, err := state.CAConfig(nil)
				require.NoError(t, err)
				require.Equal(t, dc2State, caConfig.State)
			}

		})
	}
}

func getCAProviderWithLock(s *Server) (ca.Provider, *structs.CARoot) {
	return s.caManager.getCAProvider()
}

func TestCAManager_RenewIntermediate_Vault_Primary(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	ca.SkipIfVaultNotPresent(t)

	// no parallel execution because we change globals
	patchIntermediateCertRenewInterval(t)

	testVault := ca.NewTestVaultServer(t)

	vaultToken := ca.CreateVaultTokenWithAttrs(t, testVault.Client(), &ca.VaultTokenAttributes{
		RootPath:         "pki-root",
		IntermediatePath: "pki-intermediate",
		ConsulManaged:    true,
	})

	_, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.CAConfig = &structs.CAConfiguration{
			Provider: "vault",
			Config: map[string]interface{}{
				"Address":             testVault.Addr,
				"Token":               vaultToken,
				"RootPKIPath":         "pki-root/",
				"IntermediatePKIPath": "pki-intermediate/",
				"LeafCertTTL":         "2s",
				"IntermediateCertTTL": "7s",
			},
		}
	})
	defer func() {
		s1.Shutdown()
		s1.leaderRoutineManager.Wait()
	}()

	testrpc.WaitForActiveCARoot(t, s1.RPC, "dc1", nil)

	store := s1.caManager.delegate.State()
	_, activeRoot, err := store.CARootActive(nil)
	require.NoError(t, err)
	t.Log("original SigningKeyID", activeRoot.SigningKeyID)

	intermediatePEM := s1.caManager.getLeafSigningCertFromRoot(activeRoot)
	intermediateCert, err := connect.ParseCert(intermediatePEM)
	require.NoError(t, err)

	require.Equal(t, connect.HexString(intermediateCert.SubjectKeyId), activeRoot.SigningKeyID)
	require.Equal(t, intermediatePEM, s1.caManager.getLeafSigningCertFromRoot(activeRoot))

	// Wait for dc1's intermediate to be refreshed.
	retry.Run(t, func(r *retry.R) {
		store := s1.caManager.delegate.State()
		_, storedRoot, err := store.CARootActive(nil)
		r.Check(err)

		newIntermediatePEM := s1.caManager.getLeafSigningCertFromRoot(storedRoot)
		if newIntermediatePEM == intermediatePEM {
			r.Fatal("not a renewed intermediate")
		}
		intermediateCert, err = connect.ParseCert(newIntermediatePEM)
		r.Check(err)
		intermediatePEM = newIntermediatePEM
	})

	codec := rpcClient(t, s1)
	roots := structs.IndexedCARoots{}
	err = msgpackrpc.CallWithCodec(codec, "ConnectCA.Roots", &structs.DCSpecificRequest{}, &roots)
	require.NoError(t, err)
	require.Len(t, roots.Roots, 1)

	activeRoot = roots.Active()
	require.Equal(t, connect.HexString(intermediateCert.SubjectKeyId), activeRoot.SigningKeyID)
	require.Equal(t, intermediatePEM, s1.caManager.getLeafSigningCertFromRoot(activeRoot))

	// Have the new intermediate sign a leaf cert and make sure the chain is correct.
	spiffeService := &connect.SpiffeIDService{
		Host:       roots.TrustDomain,
		Namespace:  "default",
		Datacenter: "dc1",
		Service:    "foo",
	}
	csr, _ := connect.TestCSR(t, spiffeService)

	req := structs.CASignRequest{CSR: csr}
	cert := structs.IssuedCert{}
	err = msgpackrpc.CallWithCodec(codec, "ConnectCA.Sign", &req, &cert)
	require.NoError(t, err)
	verifyLeafCert(t, activeRoot, cert.CertPEM)

	// Wait for the primary's old intermediate to be pruned after expiring.
	oldIntermediate := activeRoot.IntermediateCerts[0]
	retry.Run(t, func(r *retry.R) {
		store := s1.caManager.delegate.State()
		_, storedRoot, err := store.CARootActive(nil)
		r.Check(err)

		if storedRoot.IntermediateCerts[0] == oldIntermediate {
			r.Fatal("old intermediate should be gone")
		}
	})
}

func patchIntermediateCertRenewInterval(t *testing.T) {
	origInterval := structs.IntermediateCertRenewInterval
	origMinTTL := structs.MinLeafCertTTL

	structs.IntermediateCertRenewInterval = 200 * time.Millisecond
	structs.MinLeafCertTTL = time.Second

	t.Cleanup(func() {
		structs.IntermediateCertRenewInterval = origInterval
		structs.MinLeafCertTTL = origMinTTL
	})
}

func TestCAManager_RenewIntermediate_Secondary(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	// no parallel execution because we change globals
	patchIntermediateCertRenewInterval(t)

	_, s1 := testServerWithConfig(t, func(c *Config) {
		c.Build = "1.6.0"
		c.CAConfig = &structs.CAConfiguration{
			Provider: "consul",
			Config: map[string]interface{}{
				"PrivateKey":  "",
				"RootCert":    "",
				"LeafCertTTL": "5s",
				// The retry loop only retries for 7sec max and
				// the ttl needs to be below so that it
				// triggers definitely.
				// Since certs are created so that they are
				// valid from 1minute in the past, we need to
				// account for that, otherwise it will be
				// expired immediately.
				"IntermediateCertTTL": time.Minute + (5 * time.Second),
			},
		}
	})
	defer func() {
		s1.Shutdown()
		s1.leaderRoutineManager.Wait()
	}()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// dc2 as a secondary DC
	_, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc1"
	})
	defer func() {
		s2.Shutdown()
		s2.leaderRoutineManager.Wait()
	}()

	// Create the WAN link
	joinWAN(t, s2, s1)
	testrpc.WaitForActiveCARoot(t, s2.RPC, "dc2", nil)

	store := s2.fsm.State()
	_, activeRoot, err := store.CARootActive(nil)
	require.NoError(t, err)
	t.Log("original SigningKeyID", activeRoot.SigningKeyID)

	intermediatePEM := s2.caManager.getLeafSigningCertFromRoot(activeRoot)
	intermediateCert, err := connect.ParseCert(intermediatePEM)
	require.NoError(t, err)

	require.Equal(t, intermediatePEM, s2.caManager.getLeafSigningCertFromRoot(activeRoot))
	require.Equal(t, connect.HexString(intermediateCert.SubjectKeyId), activeRoot.SigningKeyID)

	// Wait for dc2's intermediate to be refreshed.
	retry.Run(t, func(r *retry.R) {
		store := s2.caManager.delegate.State()
		_, storedRoot, err := store.CARootActive(nil)
		r.Check(err)

		newIntermediatePEM := s2.caManager.getLeafSigningCertFromRoot(storedRoot)
		if newIntermediatePEM == intermediatePEM {
			r.Fatal("not a renewed intermediate")
		}
		intermediateCert, err = connect.ParseCert(newIntermediatePEM)
		r.Check(err)
		intermediatePEM = newIntermediatePEM
	})

	codec := rpcClient(t, s2)
	roots := structs.IndexedCARoots{}
	err = msgpackrpc.CallWithCodec(codec, "ConnectCA.Roots", &structs.DCSpecificRequest{}, &roots)
	require.NoError(t, err)
	require.Len(t, roots.Roots, 1)

	_, activeRoot, err = store.CARootActive(nil)
	require.NoError(t, err)
	require.Equal(t, connect.HexString(intermediateCert.SubjectKeyId), activeRoot.SigningKeyID)
	require.Equal(t, intermediatePEM, s2.caManager.getLeafSigningCertFromRoot(activeRoot))

	// Have dc2 sign a leaf cert and make sure the chain is correct.
	spiffeService := &connect.SpiffeIDService{
		Host:       roots.TrustDomain,
		Namespace:  "default",
		Datacenter: "dc2",
		Service:    "foo",
	}
	csr, _ := connect.TestCSR(t, spiffeService)

	req := structs.CASignRequest{CSR: csr}
	cert := structs.IssuedCert{}
	err = msgpackrpc.CallWithCodec(codec, "ConnectCA.Sign", &req, &cert)
	require.NoError(t, err)
	verifyLeafCert(t, activeRoot, cert.CertPEM)

	// Wait for dc2's old intermediate to be pruned after expiring.
	oldIntermediate := activeRoot.IntermediateCerts[0]
	retry.Run(t, func(r *retry.R) {
		store := s2.caManager.delegate.State()
		_, storedRoot, err := store.CARootActive(nil)
		r.Check(err)

		if storedRoot.IntermediateCerts[0] == oldIntermediate {
			r.Fatal("old intermediate should be gone")
		}
	})
}

func TestConnectCA_ConfigurationSet_RootRotation_Secondary(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.Build = "1.6.0"
		c.PrimaryDatacenter = "dc1"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// dc2 as a secondary DC
	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc1"
		c.Build = "1.6.0"
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	// Create the WAN link
	joinWAN(t, s2, s1)
	testrpc.WaitForLeader(t, s2.RPC, "dc2")

	// Get the original intermediate
	secondaryProvider, _ := getCAProviderWithLock(s2)
	oldIntermediatePEM, err := secondaryProvider.ActiveLeafSigningCert()
	require.NoError(t, err)
	require.NotEmpty(t, oldIntermediatePEM)

	// Capture the current root
	var originalRoot *structs.CARoot
	{
		rootList, activeRoot, err := getTestRoots(s1, "dc1")
		require.NoError(t, err)
		require.Len(t, rootList.Roots, 1)
		originalRoot = activeRoot
	}

	// Wait for current state to be reflected in both datacenters.
	testrpc.WaitForActiveCARoot(t, s1.RPC, "dc1", originalRoot)
	testrpc.WaitForActiveCARoot(t, s2.RPC, "dc2", originalRoot)

	// Update the provider config to use a new private key, which should
	// cause a rotation.
	_, newKey, err := connect.GeneratePrivateKey()
	require.NoError(t, err)
	newConfig := &structs.CAConfiguration{
		Provider: "consul",
		Config: map[string]interface{}{
			"PrivateKey":          newKey,
			"RootCert":            "",
			"IntermediateCertTTL": 72 * 24 * time.Hour,
		},
	}
	{
		args := &structs.CARequest{
			Datacenter: "dc1",
			Config:     newConfig,
		}
		var reply interface{}

		require.NoError(t, s1.RPC(context.Background(), "ConnectCA.ConfigurationSet", args, &reply))
	}

	var updatedRoot *structs.CARoot
	{
		rootList, activeRoot, err := getTestRoots(s1, "dc1")
		require.NoError(t, err)
		require.Len(t, rootList.Roots, 2)
		updatedRoot = activeRoot
	}

	testrpc.WaitForActiveCARoot(t, s1.RPC, "dc1", updatedRoot)
	testrpc.WaitForActiveCARoot(t, s2.RPC, "dc2", updatedRoot)

	// Wait for dc2's intermediate to be refreshed.
	var intermediatePEM string
	retry.Run(t, func(r *retry.R) {
		intermediatePEM, err = secondaryProvider.ActiveLeafSigningCert()
		r.Check(err)
		if intermediatePEM == oldIntermediatePEM {
			r.Fatal("not a new intermediate")
		}
	})
	require.NoError(t, err)

	// Verify the root lists have been rotated in each DC's state store.
	state1 := s1.fsm.State()
	_, primaryRoot, err := state1.CARootActive(nil)
	require.NoError(t, err)

	state2 := s2.fsm.State()
	_, roots2, err := state2.CARoots(nil)
	require.NoError(t, err)
	require.Equal(t, 2, len(roots2))

	newRoot := roots2[0]
	oldRoot := roots2[1]
	if roots2[1].Active {
		newRoot = roots2[1]
		oldRoot = roots2[0]
	}
	require.False(t, oldRoot.Active)
	require.True(t, newRoot.Active)
	require.Equal(t, primaryRoot.ID, newRoot.ID)
	require.Equal(t, primaryRoot.RootCert, newRoot.RootCert)

	// Get the new root from dc1 and validate a chain of:
	// dc2 leaf -> dc2 intermediate -> dc1 root
	_, caRoot := getCAProviderWithLock(s1)

	// Have dc2 sign a leaf cert and make sure the chain is correct.
	spiffeService := &connect.SpiffeIDService{
		Host:       "node1",
		Namespace:  "default",
		Datacenter: "dc1",
		Service:    "foo",
	}
	raw, _ := connect.TestCSR(t, spiffeService)

	leafCsr, err := connect.ParseCSR(raw)
	require.NoError(t, err)

	leafPEM, err := secondaryProvider.Sign(leafCsr)
	require.NoError(t, err)

	cert, err := connect.ParseCert(leafPEM)
	require.NoError(t, err)

	// Check that the leaf signed by the new intermediate can be verified using the
	// returned cert chain (signed intermediate + remote root).
	intermediatePool := x509.NewCertPool()
	intermediatePool.AppendCertsFromPEM([]byte(intermediatePEM))
	rootPool := x509.NewCertPool()
	rootPool.AppendCertsFromPEM([]byte(caRoot.RootCert))

	_, err = cert.Verify(x509.VerifyOptions{
		Intermediates: intermediatePool,
		Roots:         rootPool,
	})
	require.NoError(t, err)
}

func TestCAManager_Initialize_Vault_KeepOldRoots_Primary(t *testing.T) {
	ca.SkipIfVaultNotPresent(t)

	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	testVault := ca.NewTestVaultServer(t)

	dir1pre, s1pre := testServer(t)
	defer os.RemoveAll(dir1pre)
	defer s1pre.Shutdown()
	codec := rpcClient(t, s1pre)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1pre.RPC, "dc1")

	vaultToken := ca.CreateVaultTokenWithAttrs(t, testVault.Client(), &ca.VaultTokenAttributes{
		RootPath:         "pki-root",
		IntermediatePath: "pki-intermediate",
		ConsulManaged:    true,
	})

	// Update the CA config to use Vault - this should force the generation of a new root cert.
	vaultCAConf := &structs.CAConfiguration{
		Provider: "vault",
		Config: map[string]interface{}{
			"Address":             testVault.Addr,
			"Token":               vaultToken,
			"RootPKIPath":         "pki-root/",
			"IntermediatePKIPath": "pki-intermediate/",
		},
	}

	args := &structs.CARequest{
		Datacenter: "dc1",
		Config:     vaultCAConf,
	}
	var reply interface{}

	require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConnectCA.ConfigurationSet", args, &reply))

	// Should have 2 roots now.
	_, roots, err := s1pre.fsm.State().CARoots(nil)
	require.NoError(t, err)
	require.Len(t, roots, 2)

	// Shutdown s1pre and restart it to trigger the primary CA init.
	s1pre.Shutdown()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.DataDir = s1pre.config.DataDir
		c.NodeName = s1pre.config.NodeName
		c.NodeID = s1pre.config.NodeID
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Roots should be unchanged
	_, rootsAfterRestart, err := s1.fsm.State().CARoots(nil)
	require.NoError(t, err)
	require.Len(t, rootsAfterRestart, 2)
	require.Equal(t, roots[0].ID, rootsAfterRestart[0].ID)
	require.Equal(t, roots[1].ID, rootsAfterRestart[1].ID)
}

func TestCAManager_Initialize_Vault_FixesSigningKeyID_Primary(t *testing.T) {
	ca.SkipIfVaultNotPresent(t)

	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	testVault := ca.NewTestVaultServer(t)

	vaultToken := ca.CreateVaultTokenWithAttrs(t, testVault.Client(), &ca.VaultTokenAttributes{
		RootPath:         "pki-root",
		IntermediatePath: "pki-intermediate",
		ConsulManaged:    true,
	})

	dir1pre, s1pre := testServerWithConfig(t, func(c *Config) {
		c.Build = "1.6.0"
		c.PrimaryDatacenter = "dc1"
		c.CAConfig = &structs.CAConfiguration{
			Provider: "vault",
			Config: map[string]interface{}{
				"Address":             testVault.Addr,
				"Token":               vaultToken,
				"RootPKIPath":         "pki-root/",
				"IntermediatePKIPath": "pki-intermediate/",
			},
		}
	})
	defer os.RemoveAll(dir1pre)
	defer s1pre.Shutdown()

	testrpc.WaitForLeader(t, s1pre.RPC, "dc1")

	// Restore the pre-1.9.3/1.8.8/1.7.12 behavior of the SigningKeyID not being derived
	// from the intermediates in the primary (which only matters for provider=vault).
	var primaryRootSigningKeyID string
	{
		state := s1pre.fsm.State()

		// Get the highest index
		idx, activePrimaryRoot, err := state.CARootActive(nil)
		require.NoError(t, err)
		require.NotNil(t, activePrimaryRoot)

		rootCert, err := connect.ParseCert(activePrimaryRoot.RootCert)
		require.NoError(t, err)

		// Force this to be derived just from the root, not the intermediate.
		primaryRootSigningKeyID = connect.EncodeSigningKeyID(rootCert.SubjectKeyId)
		activePrimaryRoot.SigningKeyID = primaryRootSigningKeyID

		// Store the root cert in raft
		_, err = s1pre.raftApply(structs.ConnectCARequestType, &structs.CARequest{
			Op:    structs.CAOpSetRoots,
			Index: idx,
			Roots: []*structs.CARoot{activePrimaryRoot},
		})
		require.NoError(t, err)
	}

	// Shutdown s1pre and restart it to trigger the secondary CA init to correct
	// the SigningKeyID.
	s1pre.Shutdown()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.DataDir = s1pre.config.DataDir
		c.Datacenter = "dc1"
		c.PrimaryDatacenter = "dc1"
		c.NodeName = s1pre.config.NodeName
		c.NodeID = s1pre.config.NodeID
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Retry since it will take some time to init the primary CA fully and there
	// isn't a super clean way to watch specifically until it's done than polling
	// the CA provider anyway.
	retry.Run(t, func(r *retry.R) {
		// verify that the root is now corrected
		provider, activeRoot := getCAProviderWithLock(s1)
		require.NotNil(r, provider)
		require.NotNil(r, activeRoot)

		activeIntermediate, err := provider.ActiveLeafSigningCert()
		require.NoError(r, err)

		intermediateCert, err := connect.ParseCert(activeIntermediate)
		require.NoError(r, err)

		// Force this to be derived just from the root, not the intermediate.
		expect := connect.EncodeSigningKeyID(intermediateCert.SubjectKeyId)

		// The in-memory representation was saw the correction via a setCAProvider call.
		require.Equal(r, expect, activeRoot.SigningKeyID)

		// The state store saw the correction, too.
		_, activePrimaryRoot, err := s1.fsm.State().CARootActive(nil)
		require.NoError(r, err)
		require.NotNil(r, activePrimaryRoot)
		require.Equal(r, expect, activePrimaryRoot.SigningKeyID)
	})
}

func TestCAManager_Initialize_FixesSigningKeyID_Secondary(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.Build = "1.6.0"
		c.PrimaryDatacenter = "dc1"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// dc2 as a secondary DC
	dir2pre, s2pre := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc1"
		c.Build = "1.6.0"
	})
	defer os.RemoveAll(dir2pre)
	defer s2pre.Shutdown()

	// Create the WAN link
	joinWAN(t, s2pre, s1)
	testrpc.WaitForLeader(t, s2pre.RPC, "dc2")

	// Restore the pre-1.6.1 behavior of the SigningKeyID not being derived
	// from the intermediates.
	var secondaryRootSigningKeyID string
	{
		state := s2pre.fsm.State()

		// Get the highest index
		idx, activeSecondaryRoot, err := state.CARootActive(nil)
		require.NoError(t, err)
		require.NotNil(t, activeSecondaryRoot)

		rootCert, err := connect.ParseCert(activeSecondaryRoot.RootCert)
		require.NoError(t, err)

		// Force this to be derived just from the root, not the intermediate.
		secondaryRootSigningKeyID = connect.EncodeSigningKeyID(rootCert.SubjectKeyId)
		activeSecondaryRoot.SigningKeyID = secondaryRootSigningKeyID

		// Store the root cert in raft
		_, err = s2pre.raftApply(structs.ConnectCARequestType, &structs.CARequest{
			Op:    structs.CAOpSetRoots,
			Index: idx,
			Roots: []*structs.CARoot{activeSecondaryRoot},
		})
		require.NoError(t, err)
	}

	// Shutdown s2pre and restart it to trigger the secondary CA init to correct
	// the SigningKeyID.
	s2pre.Shutdown()

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.DataDir = s2pre.config.DataDir
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc1"
		c.NodeName = s2pre.config.NodeName
		c.NodeID = s2pre.config.NodeID
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	testrpc.WaitForLeader(t, s2.RPC, "dc2")

	// Retry since it will take some time to init the secondary CA fully and there
	// isn't a super clean way to watch specifically until it's done than polling
	// the CA provider anyway.
	retry.Run(t, func(r *retry.R) {
		// verify that the root is now corrected
		provider, activeRoot := getCAProviderWithLock(s2)
		require.NotNil(r, provider)
		require.NotNil(r, activeRoot)

		activeIntermediate, err := provider.ActiveLeafSigningCert()
		require.NoError(r, err)

		intermediateCert, err := connect.ParseCert(activeIntermediate)
		require.NoError(r, err)

		// Force this to be derived just from the root, not the intermediate.
		expect := connect.EncodeSigningKeyID(intermediateCert.SubjectKeyId)

		// The in-memory representation was saw the correction via a setCAProvider call.
		require.Equal(r, expect, activeRoot.SigningKeyID)

		// The state store saw the correction, too.
		_, activeSecondaryRoot, err := s2.fsm.State().CARootActive(nil)
		require.NoError(r, err)
		require.NotNil(r, activeSecondaryRoot)
		require.Equal(r, expect, activeSecondaryRoot.SigningKeyID)
	})
}

func TestCAManager_Initialize_TransitionFromPrimaryToSecondary(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	// Initialize dc1 as the primary DC
	id1, err := uuid.GenerateUUID()
	require.NoError(t, err)
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.CAConfig.ClusterID = id1
		c.Build = "1.6.0"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// dc2 as a primary DC initially
	id2, err := uuid.GenerateUUID()
	require.NoError(t, err)
	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc2"
		c.CAConfig.ClusterID = id2
		c.Build = "1.6.0"
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	// Get the initial (primary) roots state for the secondary
	testrpc.WaitForLeader(t, s2.RPC, "dc2")
	args := structs.DCSpecificRequest{Datacenter: "dc2"}
	var dc2PrimaryRoots structs.IndexedCARoots
	require.NoError(t, s2.RPC(context.Background(), "ConnectCA.Roots", &args, &dc2PrimaryRoots))
	require.Len(t, dc2PrimaryRoots.Roots, 1)

	// Shutdown s2 and restart it with the dc1 as the primary
	s2.Shutdown()
	dir3, s3 := testServerWithConfig(t, func(c *Config) {
		c.DataDir = s2.config.DataDir
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc1"
		c.NodeName = s2.config.NodeName
		c.NodeID = s2.config.NodeID
	})
	defer os.RemoveAll(dir3)
	defer s3.Shutdown()

	// Create the WAN link
	joinWAN(t, s3, s1)
	testrpc.WaitForLeader(t, s3.RPC, "dc2")

	// Verify the secondary has migrated its TrustDomain and added the new primary's root.
	retry.Run(t, func(r *retry.R) {
		args = structs.DCSpecificRequest{Datacenter: "dc1"}
		var dc1Roots structs.IndexedCARoots
		require.NoError(r, s1.RPC(context.Background(), "ConnectCA.Roots", &args, &dc1Roots))
		require.Len(r, dc1Roots.Roots, 1)

		args = structs.DCSpecificRequest{Datacenter: "dc2"}
		var dc2SecondaryRoots structs.IndexedCARoots
		require.NoError(r, s3.RPC(context.Background(), "ConnectCA.Roots", &args, &dc2SecondaryRoots))

		// dc2's TrustDomain should have changed to the primary's
		require.Equal(r, dc2SecondaryRoots.TrustDomain, dc1Roots.TrustDomain)
		require.NotEqual(r, dc2SecondaryRoots.TrustDomain, dc2PrimaryRoots.TrustDomain)

		// Both roots should be present and correct
		require.Len(r, dc2SecondaryRoots.Roots, 2)
		var oldSecondaryRoot *structs.CARoot
		var newSecondaryRoot *structs.CARoot
		if dc2SecondaryRoots.Roots[0].ID == dc2PrimaryRoots.Roots[0].ID {
			oldSecondaryRoot = dc2SecondaryRoots.Roots[0]
			newSecondaryRoot = dc2SecondaryRoots.Roots[1]
		} else {
			oldSecondaryRoot = dc2SecondaryRoots.Roots[1]
			newSecondaryRoot = dc2SecondaryRoots.Roots[0]
		}

		// The old root should have its TrustDomain filled in as the old domain.
		require.Equal(r, oldSecondaryRoot.ExternalTrustDomain, strings.TrimSuffix(dc2PrimaryRoots.TrustDomain, ".consul"))

		require.Equal(r, oldSecondaryRoot.ID, dc2PrimaryRoots.Roots[0].ID)
		require.Equal(r, oldSecondaryRoot.RootCert, dc2PrimaryRoots.Roots[0].RootCert)
		require.Equal(r, newSecondaryRoot.ID, dc1Roots.Roots[0].ID)
		require.Equal(r, newSecondaryRoot.RootCert, dc1Roots.Roots[0].RootCert)
	})
}

func TestCAManager_Initialize_SecondaryBeforePrimary(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	// Initialize dc1 as the primary DC
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.Build = "1.3.0"
		c.MaxQueryTime = 500 * time.Millisecond
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// dc2 as a secondary DC
	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc1"
		c.Build = "1.6.0"
		c.MaxQueryTime = 500 * time.Millisecond
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	// Create the WAN link
	joinWAN(t, s2, s1)
	testrpc.WaitForLeader(t, s2.RPC, "dc2")

	// ensure all the CA initialization stuff would have already been done
	// this is necessary to ensure that not only has a leader been elected
	// but that it has also finished its establishLeadership call
	retry.Run(t, func(r *retry.R) {
		require.True(r, s1.isReadyForConsistentReads())
		require.True(r, s2.isReadyForConsistentReads())
	})

	// Verify the primary has a root (we faked its version too low but since its the primary it ignores any version checks)
	retry.Run(t, func(r *retry.R) {
		state1 := s1.fsm.State()
		_, roots1, err := state1.CARoots(nil)
		require.NoError(r, err)
		require.Len(r, roots1, 1)
	})

	// Verify the secondary does not have a root - defers initialization until the primary has been upgraded.
	state2 := s2.fsm.State()
	_, roots2, err := state2.CARoots(nil)
	require.NoError(t, err)
	require.Empty(t, roots2)

	// Update the version on the fly so s2 kicks off the secondary DC transition.
	tags := s1.config.SerfWANConfig.Tags
	tags["build"] = "1.6.0"
	s1.serfWAN.SetTags(tags)

	// Wait for the secondary transition to happen and then verify the secondary DC
	// has both roots present.
	retry.Run(t, func(r *retry.R) {
		state1 := s1.fsm.State()
		_, roots1, err := state1.CARoots(nil)
		require.NoError(r, err)
		require.Len(r, roots1, 1)

		state2 := s2.fsm.State()
		_, roots2, err := state2.CARoots(nil)
		require.NoError(r, err)
		require.Len(r, roots2, 1)

		// ensure the roots are the same
		require.Equal(r, roots1[0].ID, roots2[0].ID)
		require.Equal(r, roots1[0].RootCert, roots2[0].RootCert)

		secondaryProvider, _ := getCAProviderWithLock(s2)
		inter, err := secondaryProvider.ActiveLeafSigningCert()
		require.NoError(r, err)
		require.NotEmpty(r, inter, "should have valid intermediate")
	})

	secondaryProvider, _ := getCAProviderWithLock(s2)
	intermediatePEM, err := secondaryProvider.ActiveLeafSigningCert()
	require.NoError(t, err)

	_, caRoot := getCAProviderWithLock(s1)

	// Have dc2 sign a leaf cert and make sure the chain is correct.
	spiffeService := &connect.SpiffeIDService{
		Host:       "node1",
		Namespace:  "default",
		Datacenter: "dc1",
		Service:    "foo",
	}
	raw, _ := connect.TestCSR(t, spiffeService)

	leafCsr, err := connect.ParseCSR(raw)
	require.NoError(t, err)

	leafPEM, err := secondaryProvider.Sign(leafCsr)
	require.NoError(t, err)

	cert, err := connect.ParseCert(leafPEM)
	require.NoError(t, err)

	// Check that the leaf signed by the new cert can be verified using the
	// returned cert chain (signed intermediate + remote root).
	intermediatePool := x509.NewCertPool()
	intermediatePool.AppendCertsFromPEM([]byte(intermediatePEM))
	rootPool := x509.NewCertPool()
	rootPool.AppendCertsFromPEM([]byte(caRoot.RootCert))

	_, err = cert.Verify(x509.VerifyOptions{
		Intermediates: intermediatePool,
		Roots:         rootPool,
	})
	require.NoError(t, err)
}

func getTestRoots(s *Server, datacenter string) (*structs.IndexedCARoots, *structs.CARoot, error) {
	rootReq := &structs.DCSpecificRequest{
		Datacenter: datacenter,
	}
	var rootList structs.IndexedCARoots
	if err := s.RPC(context.Background(), "ConnectCA.Roots", rootReq, &rootList); err != nil {
		return nil, nil, err
	}

	active := rootList.Active()
	return &rootList, active, nil
}

func TestLeader_CARootPruning(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	// Can not use t.Parallel(), because this modifies a global.
	origPruneInterval := caRootPruneInterval
	caRootPruneInterval = 200 * time.Millisecond
	t.Cleanup(func() {
		// Reset the value of the global prune interval so that it doesn't affect other tests
		caRootPruneInterval = origPruneInterval
	})

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")

	// Get the current root
	rootReq := &structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	var rootList structs.IndexedCARoots
	require.Nil(t, msgpackrpc.CallWithCodec(codec, "ConnectCA.Roots", rootReq, &rootList))
	require.Len(t, rootList.Roots, 1)
	oldRoot := rootList.Roots[0]

	// Update the provider config to use a new private key, which should
	// cause a rotation.
	_, newKey, err := connect.GeneratePrivateKey()
	require.NoError(t, err)
	newConfig := &structs.CAConfiguration{
		Provider: "consul",
		Config: map[string]interface{}{
			"LeafCertTTL":  "500ms",
			"PrivateKey":   newKey,
			"RootCert":     "",
			"SkipValidate": true,
		},
	}
	{
		args := &structs.CARequest{
			Datacenter: "dc1",
			Config:     newConfig,
		}
		var reply interface{}

		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConnectCA.ConfigurationSet", args, &reply))
	}

	// Should have 2 roots now.
	_, roots, err := s1.fsm.State().CARoots(nil)
	require.NoError(t, err)
	require.Len(t, roots, 2)

	time.Sleep(2 * time.Second)

	// Now the old root should be pruned.
	_, roots, err = s1.fsm.State().CARoots(nil)
	require.NoError(t, err)
	require.Len(t, roots, 1)
	require.True(t, roots[0].Active)
	require.NotEqual(t, roots[0].ID, oldRoot.ID)
}

func TestConnectCA_ConfigurationSet_PersistsRoots(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	dir2, s2 := testServerDCBootstrap(t, "dc1", false)
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	dir3, s3 := testServerDCBootstrap(t, "dc1", false)
	defer os.RemoveAll(dir3)
	defer s3.Shutdown()

	joinLAN(t, s2, s1)
	joinLAN(t, s3, s1)

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Get the current root
	rootReq := &structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	var rootList structs.IndexedCARoots
	require.Nil(t, msgpackrpc.CallWithCodec(codec, "ConnectCA.Roots", rootReq, &rootList))
	require.Len(t, rootList.Roots, 1)

	// Update the provider config to use a new private key, which should
	// cause a rotation.
	_, newKey, err := connect.GeneratePrivateKey()
	require.NoError(t, err)
	newConfig := &structs.CAConfiguration{
		Provider: "consul",
		Config: map[string]interface{}{
			"PrivateKey": newKey,
			"RootCert":   "",
		},
	}
	{
		args := &structs.CARequest{
			Datacenter: "dc1",
			Config:     newConfig,
		}
		var reply interface{}

		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConnectCA.ConfigurationSet", args, &reply))
	}

	// Get the active root before leader change.
	_, root := getCAProviderWithLock(s1)
	require.Len(t, root.IntermediateCerts, 1)

	// Force a leader change and make sure the root CA values are preserved.
	s1.Leave()
	s1.Shutdown()

	retry.Run(t, func(r *retry.R) {
		var leader *Server
		for _, s := range []*Server{s2, s3} {
			if s.IsLeader() {
				leader = s
				break
			}
		}
		if leader == nil {
			r.Fatal("no leader")
		}

		_, newLeaderRoot := getCAProviderWithLock(leader)
		if !reflect.DeepEqual(newLeaderRoot, root) {
			r.Fatalf("got %v, want %v", newLeaderRoot, root)
		}
	})
}

func TestNewCARoot(t *testing.T) {
	type testCase struct {
		name            string
		pem             string
		intermediatePem string
		expected        *structs.CARoot
		expectedErr     string
	}

	run := func(t *testing.T, tc testCase) {
		root, err := newCARoot(
			tc.pem,
			"provider-name", "cluster-id")
		if tc.intermediatePem != "" {
			setLeafSigningCert(root, tc.intermediatePem)
		}
		if tc.expectedErr != "" {
			testutil.RequireErrorContains(t, err, tc.expectedErr)
			return
		}
		require.NoError(t, err)
		assert.DeepEqual(t, tc.expected, root)
	}

	// Test certs can be generated with
	//   go run connect/certgen/certgen.go -out-dir /tmp/connect-certs -key-type ec -key-bits 384
	// serial generated with:
	//   openssl x509 -noout -text
	testCases := []testCase{
		{
			name:        "no cert",
			expectedErr: "no PEM-encoded data found",
		},
		{
			name: "type=ec bits=256",
			pem:  readTestData(t, "cert-with-ec-256-key.pem"),
			expected: &structs.CARoot{
				ID:                  "c9:1b:24:e0:89:63:1a:ba:22:01:f4:cf:bc:f1:c0:36:b2:6b:6c:3d",
				Name:                "Provider-Name CA Primary Cert",
				SerialNumber:        8341954965092507701,
				SigningKeyID:        "97:4d:17:81:64:f8:b4:af:05:e8:6c:79:c5:40:3b:0e:3e:8b:c0:ae:38:51:54:8a:2f:05:db:e3:e8:e4:24:ec",
				ExternalTrustDomain: "cluster-id",
				NotBefore:           time.Date(2019, 10, 17, 11, 46, 29, 0, time.UTC),
				NotAfter:            time.Date(2029, 10, 17, 11, 46, 29, 0, time.UTC),
				RootCert:            readTestData(t, "cert-with-ec-256-key.pem"),
				Active:              true,
				PrivateKeyType:      "ec",
				PrivateKeyBits:      256,
			},
		},
		{
			name: "type=ec bits=384",
			pem:  readTestData(t, "cert-with-ec-384-key.pem"),
			expected: &structs.CARoot{
				ID:                  "29:69:c4:0f:aa:8f:bd:07:31:0d:51:3b:45:62:3d:c0:b2:fc:c6:3f",
				Name:                "Provider-Name CA Primary Cert",
				SerialNumber:        2935109425518279965,
				SigningKeyID:        "0b:a0:88:9b:dc:95:31:51:2e:3d:d4:f9:42:d0:6a:a0:62:46:82:d2:7c:22:e7:29:a9:aa:e8:a5:8c:cf:c7:42",
				ExternalTrustDomain: "cluster-id",
				NotBefore:           time.Date(2019, 10, 17, 11, 55, 18, 0, time.UTC),
				NotAfter:            time.Date(2029, 10, 17, 11, 55, 18, 0, time.UTC),
				RootCert:            readTestData(t, "cert-with-ec-384-key.pem"),
				Active:              true,
				PrivateKeyType:      "ec",
				PrivateKeyBits:      384,
			},
		},
		{
			name: "type=rsa bits=4096",
			pem:  readTestData(t, "cert-with-rsa-4096-key.pem"),
			expected: &structs.CARoot{
				ID:                  "3a:6a:e3:e2:2d:44:85:5a:e9:44:3b:ef:d2:90:78:83:7f:61:a2:84",
				Name:                "Provider-Name CA Primary Cert",
				SerialNumber:        5186695743100577491,
				SigningKeyID:        "92:fa:cc:97:57:1e:31:84:a2:33:dd:9b:6a:a8:7c:fc:be:e2:94:ca:ac:b3:33:17:39:3b:b8:67:9b:dc:c1:08",
				ExternalTrustDomain: "cluster-id",
				NotBefore:           time.Date(2019, 10, 17, 11, 53, 15, 0, time.UTC),
				NotAfter:            time.Date(2029, 10, 17, 11, 53, 15, 0, time.UTC),
				RootCert:            readTestData(t, "cert-with-rsa-4096-key.pem"),
				Active:              true,
				PrivateKeyType:      "rsa",
				PrivateKeyBits:      4096,
			},
		},
		{
			name: "two certs in pem",
			pem:  readTestData(t, "pem-with-two-certs.pem"),
			expected: &structs.CARoot{
				ID:                  "42:43:10:1f:71:6b:21:21:d1:10:49:d1:f0:41:78:8c:0a:77:ef:c0",
				Name:                "Provider-Name CA Primary Cert",
				SerialNumber:        17692800288680335732,
				SigningKeyID:        "9d:5c:27:43:ce:58:7b:ca:3e:7d:c4:fb:b6:2e:b7:13:e9:a1:68:3e",
				ExternalTrustDomain: "cluster-id",
				NotBefore:           time.Date(2022, 1, 5, 23, 22, 12, 0, time.UTC),
				NotAfter:            time.Date(2022, 4, 7, 15, 22, 42, 0, time.UTC),
				RootCert:            readTestData(t, "pem-with-two-certs.pem"),
				Active:              true,
				PrivateKeyType:      "ec",
				PrivateKeyBits:      256,
			},
		},
		{
			name: "three certs in pem",
			pem:  readTestData(t, "pem-with-three-certs.pem"),
			expected: &structs.CARoot{
				ID:                  "42:43:10:1f:71:6b:21:21:d1:10:49:d1:f0:41:78:8c:0a:77:ef:c0",
				Name:                "Provider-Name CA Primary Cert",
				SerialNumber:        17692800288680335732,
				SigningKeyID:        "9d:5c:27:43:ce:58:7b:ca:3e:7d:c4:fb:b6:2e:b7:13:e9:a1:68:3e",
				ExternalTrustDomain: "cluster-id",
				NotBefore:           time.Date(2022, 1, 5, 23, 22, 12, 0, time.UTC),
				NotAfter:            time.Date(2022, 4, 7, 15, 22, 42, 0, time.UTC),
				RootCert:            readTestData(t, "pem-with-three-certs.pem"),
				Active:              true,
				PrivateKeyType:      "ec",
				PrivateKeyBits:      256,
			},
		},
		{
			// Although the intermediate pem doesn't have pem as the issuer
			// as in a real certificate chain, we are testing that the IntermediateCerts
			// are being populated and that the signing key is from the intermediatePem.
			name:            "pem with intermediate pem",
			pem:             readTestData(t, "cert-with-rsa-4096-key.pem"),
			intermediatePem: readTestData(t, "cert-with-ec-256-key.pem"),
			expected: &structs.CARoot{
				ID:                  "3a:6a:e3:e2:2d:44:85:5a:e9:44:3b:ef:d2:90:78:83:7f:61:a2:84",
				Name:                "Provider-Name CA Primary Cert",
				SerialNumber:        5186695743100577491,
				SigningKeyID:        "97:4d:17:81:64:f8:b4:af:05:e8:6c:79:c5:40:3b:0e:3e:8b:c0:ae:38:51:54:8a:2f:05:db:e3:e8:e4:24:ec",
				ExternalTrustDomain: "cluster-id",
				NotBefore:           time.Date(2019, 10, 17, 11, 53, 15, 0, time.UTC),
				NotAfter:            time.Date(2029, 10, 17, 11, 53, 15, 0, time.UTC),
				RootCert:            readTestData(t, "cert-with-rsa-4096-key.pem"),
				IntermediateCerts:   []string{readTestData(t, "cert-with-ec-256-key.pem")},
				Active:              true,
				PrivateKeyType:      "rsa",
				PrivateKeyBits:      4096,
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func readTestData(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("testdata", name)
	bs, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed reading fixture file %s: %s", name, err)
	}
	return string(bs)
}

func TestLessThanHalfTimePassed(t *testing.T) {
	now := time.Now()
	require.False(t, lessThanHalfTimePassed(now, now.Add(-10*time.Second), now.Add(-5*time.Second)))
	require.False(t, lessThanHalfTimePassed(now, now.Add(-10*time.Second), now))
	require.False(t, lessThanHalfTimePassed(now, now.Add(-10*time.Second), now.Add(5*time.Second)))
	require.False(t, lessThanHalfTimePassed(now, now.Add(-10*time.Second), now.Add(10*time.Second)))

	require.True(t, lessThanHalfTimePassed(now, now.Add(-10*time.Second), now.Add(20*time.Second)))
}

func TestRetryLoopBackoffHandleSuccess(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	type test struct {
		desc     string
		loopFn   func() error
		abort    bool
		timedOut bool
	}
	success := func() error {
		return nil
	}
	failure := func() error {
		return fmt.Errorf("test error")
	}
	tests := []test{
		{"loop without error and no abortOnSuccess keeps running", success, false, true},
		{"loop with error and no abortOnSuccess keeps running", failure, false, true},
		{"loop without error and abortOnSuccess is stopped", success, true, false},
		{"loop with error and abortOnSuccess keeps running", failure, true, true},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()

			retryLoopBackoffHandleSuccess(ctx, tc.loopFn, func(_ error) {}, tc.abort)
			select {
			case <-ctx.Done():
				if !tc.timedOut {
					t.Fatal("should not have timed out")
				}
			default:
				if tc.timedOut {
					t.Fatal("should have timed out")
				}
			}
		})
	}
}

func TestCAManager_Initialize_Vault_BadCAConfigDoesNotPreventLeaderEstablishment(t *testing.T) {
	ca.SkipIfVaultNotPresent(t)

	testVault := ca.NewTestVaultServer(t)
	defer testVault.Stop()

	_, s1 := testServerWithConfig(t, func(c *Config) {
		c.Build = "1.9.1"
		c.PrimaryDatacenter = "dc1"
		c.CAConfig = &structs.CAConfiguration{
			Provider: "vault",
			Config: map[string]interface{}{
				"Address":             testVault.Addr,
				"Token":               "not-the-root",
				"RootPKIPath":         "pki-root/",
				"IntermediatePKIPath": "pki-intermediate/",
			},
		}
	})
	defer s1.Shutdown()

	waitForLeaderEstablishment(t, s1)

	rootsList, activeRoot, err := getTestRoots(s1, "dc1")
	require.NoError(t, err)
	require.Empty(t, rootsList.Roots)
	require.Nil(t, activeRoot)

	goodVaultToken := ca.CreateVaultTokenWithAttrs(t, testVault.Client(), &ca.VaultTokenAttributes{
		RootPath:         "pki-root",
		IntermediatePath: "pki-intermediate",
		ConsulManaged:    true,
	})

	// Now that the leader is up and we have verified that there are no roots / CA init failed,
	// verify that we can reconfigure away from the bad configuration.
	newConfig := &structs.CAConfiguration{
		Provider: "vault",
		Config: map[string]interface{}{
			"Address":             testVault.Addr,
			"Token":               goodVaultToken,
			"RootPKIPath":         "pki-root/",
			"IntermediatePKIPath": "pki-intermediate/",
		},
	}
	{
		args := &structs.CARequest{
			Datacenter: "dc1",
			Config:     newConfig,
		}
		var reply interface{}

		retry.Run(t, func(r *retry.R) {
			require.NoError(r, s1.RPC(context.Background(), "ConnectCA.ConfigurationSet", args, &reply))
		})
	}

	rootsList, activeRoot, err = getTestRoots(s1, "dc1")
	require.NoError(t, err)
	require.NotEmpty(t, rootsList.Roots)
	require.NotNil(t, activeRoot)
}

func TestCAManager_Initialize_BadCAConfigDoesNotPreventLeaderEstablishment(t *testing.T) {
	_, s1 := testServerWithConfig(t, func(c *Config) {
		c.Build = "1.9.1"
		c.PrimaryDatacenter = "dc1"
		c.CAConfig = &structs.CAConfiguration{
			Provider: "consul",
			Config: map[string]interface{}{
				"RootCert": "garbage",
			},
		}
	})
	defer s1.Shutdown()

	waitForLeaderEstablishment(t, s1)

	rootsList, activeRoot, err := getTestRoots(s1, "dc1")
	require.NoError(t, err)
	require.Empty(t, rootsList.Roots)
	require.Nil(t, activeRoot)

	newConfig := &structs.CAConfiguration{
		Provider: "consul",
		Config:   map[string]interface{}{},
	}
	{
		args := &structs.CARequest{
			Datacenter: "dc1",
			Config:     newConfig,
		}
		var reply interface{}

		retry.Run(t, func(r *retry.R) {
			require.NoError(r, s1.RPC(context.Background(), "ConnectCA.ConfigurationSet", args, &reply))
		})
	}

	rootsList, activeRoot, err = getTestRoots(s1, "dc1")
	require.NoError(t, err)
	require.NotEmpty(t, rootsList.Roots)
	require.NotNil(t, activeRoot)
}

func TestConnectCA_ConfigurationSet_ForceWithoutCrossSigning(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	waitForLeaderEstablishment(t, s1)

	// Get the current root
	rootReq := &structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	var rootList structs.IndexedCARoots
	require.Nil(t, msgpackrpc.CallWithCodec(codec, "ConnectCA.Roots", rootReq, &rootList))
	require.Len(t, rootList.Roots, 1)
	oldRoot := rootList.Roots[0]

	// Update the provider config to use a new private key, which should
	// cause a rotation.
	_, newKey, err := connect.GeneratePrivateKey()
	require.NoError(t, err)
	newConfig := &structs.CAConfiguration{
		Provider: "consul",
		Config: map[string]interface{}{
			"LeafCertTTL":  "500ms",
			"PrivateKey":   newKey,
			"RootCert":     "",
			"SkipValidate": true,
		},
		ForceWithoutCrossSigning: true,
	}
	{
		args := &structs.CARequest{
			Datacenter: "dc1",
			Config:     newConfig,
		}
		var reply interface{}

		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConnectCA.ConfigurationSet", args, &reply))
	}

	// Old root should no longer be active.
	_, roots, err := s1.fsm.State().CARoots(nil)
	require.NoError(t, err)
	require.Len(t, roots, 2)
	for _, r := range roots {
		if r.ID == oldRoot.ID {
			require.False(t, r.Active)
		} else {
			require.True(t, r.Active)
		}
	}
}

func TestConnectCA_ConfigurationSet_Vault_ForceWithoutCrossSigning(t *testing.T) {
	ca.SkipIfVaultNotPresent(t)

	testVault := ca.NewTestVaultServer(t)

	vaultToken1 := ca.CreateVaultTokenWithAttrs(t, testVault.Client(), &ca.VaultTokenAttributes{
		RootPath:         "pki-root",
		IntermediatePath: "pki-intermediate",
		ConsulManaged:    true,
		WithSudo:         true,
	})

	_, s1 := testServerWithConfig(t, func(c *Config) {
		c.Build = "1.9.1"
		c.PrimaryDatacenter = "dc1"
		c.CAConfig = &structs.CAConfiguration{
			Provider: "vault",
			Config: map[string]interface{}{
				"Address":             testVault.Addr,
				"Token":               vaultToken1,
				"RootPKIPath":         "pki-root/",
				"IntermediatePKIPath": "pki-intermediate/",
			},
		}
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	waitForLeaderEstablishment(t, s1)

	// Get the current root
	rootReq := &structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	var rootList structs.IndexedCARoots
	require.Nil(t, msgpackrpc.CallWithCodec(codec, "ConnectCA.Roots", rootReq, &rootList))
	require.Len(t, rootList.Roots, 1)
	oldRoot := rootList.Roots[0]

	vaultToken2 := ca.CreateVaultTokenWithAttrs(t, testVault.Client(), &ca.VaultTokenAttributes{
		RootPath:         "pki-root-2",
		IntermediatePath: "pki-intermediate",
		ConsulManaged:    true,
	})

	// Update the provider config to use a new PKI path, which should
	// cause a rotation.
	newConfig := &structs.CAConfiguration{
		Provider: "vault",
		Config: map[string]interface{}{
			"Address":             testVault.Addr,
			"Token":               vaultToken2,
			"RootPKIPath":         "pki-root-2/",
			"IntermediatePKIPath": "pki-intermediate/",
		},
		ForceWithoutCrossSigning: true,
	}
	{
		args := &structs.CARequest{
			Datacenter: "dc1",
			Config:     newConfig,
		}
		var reply interface{}

		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConnectCA.ConfigurationSet", args, &reply))
	}

	// Old root should no longer be active.
	_, roots, err := s1.fsm.State().CARoots(nil)
	require.NoError(t, err)
	require.Len(t, roots, 2)
	for _, r := range roots {
		if r.ID == oldRoot.ID {
			require.False(t, r.Active)
		} else {
			require.True(t, r.Active)
		}
	}
}
