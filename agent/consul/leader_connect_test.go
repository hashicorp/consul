package consul

import (
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/connect"
	ca "github.com/hashicorp/consul/agent/connect/ca"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
	tokenStore "github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
	uuid "github.com/hashicorp/go-uuid"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLeader_SecondaryCA_Initialize(t *testing.T) {
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
			masterToken := "8a85f086-dd95-4178-b128-e10902767c5c"

			// Initialize primary as the primary DC
			dir1, s1 := testServerWithConfig(t, func(c *Config) {
				c.Datacenter = "primary"
				c.ACLDatacenter = "primary"
				c.Build = "1.6.0"
				c.ACLsEnabled = true
				c.ACLMasterToken = masterToken
				c.ACLDefaultPolicy = "deny"
				c.CAConfig.Config["PrivateKeyType"] = tc.keyType
				c.CAConfig.Config["PrivateKeyBits"] = tc.keyBits
				c.CAConfig.Config["test_state"] = dc1State
			})
			defer os.RemoveAll(dir1)
			defer s1.Shutdown()

			s1.tokens.UpdateAgentToken(masterToken, token.TokenSourceConfig)

			testrpc.WaitForLeader(t, s1.RPC, "primary")

			// secondary as a secondary DC
			dir2, s2 := testServerWithConfig(t, func(c *Config) {
				c.Datacenter = "secondary"
				c.ACLDatacenter = "primary"
				c.Build = "1.6.0"
				c.ACLsEnabled = true
				c.ACLDefaultPolicy = "deny"
				c.ACLTokenReplication = true
				c.CAConfig.Config["PrivateKeyType"] = tc.keyType
				c.CAConfig.Config["PrivateKeyBits"] = tc.keyBits
				c.CAConfig.Config["test_state"] = dc2State
			})
			defer os.RemoveAll(dir2)
			defer s2.Shutdown()

			s2.tokens.UpdateAgentToken(masterToken, token.TokenSourceConfig)
			s2.tokens.UpdateReplicationToken(masterToken, token.TokenSourceConfig)

			testrpc.WaitForLeader(t, s2.RPC, "secondary")

			// Create the WAN link
			joinWAN(t, s2, s1)

			waitForNewACLs(t, s1)
			waitForNewACLs(t, s2)

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
				_, caRoot = s1.getCAProvider()
				secondaryProvider, _ = s2.getCAProvider()
				intermediatePEM, err = secondaryProvider.ActiveIntermediate()
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

func waitForActiveCARoot(t *testing.T, srv *Server, expect *structs.CARoot) {
	retry.Run(t, func(r *retry.R) {
		_, root := srv.getCAProvider()
		if root == nil {
			r.Fatal("no root")
		}
		if root.ID != expect.ID {
			r.Fatalf("current active root is %s; waiting for %s", root.ID, expect.ID)
		}
	})
}

func TestLeader_SecondaryCA_IntermediateRenew(t *testing.T) {
	// no parallel execution because we change globals
	origInterval := structs.IntermediateCertRenewInterval
	origMinTTL := structs.MinLeafCertTTL
	defer func() {
		structs.IntermediateCertRenewInterval = origInterval
		structs.MinLeafCertTTL = origMinTTL
	}()

	structs.IntermediateCertRenewInterval = time.Millisecond
	structs.MinLeafCertTTL = time.Second
	require := require.New(t)

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.Build = "1.6.0"
		c.CAConfig = &structs.CAConfiguration{
			Provider: "consul",
			Config: map[string]interface{}{
				"PrivateKey":     "",
				"RootCert":       "",
				"RotationPeriod": "2160h",
				"LeafCertTTL":    "5s",
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
	secondaryProvider, _ := s2.getCAProvider()
	intermediatePEM, err := secondaryProvider.ActiveIntermediate()
	require.NoError(err)
	cert, err := connect.ParseCert(intermediatePEM)
	require.NoError(err)
	currentCertSerialNumber := cert.SerialNumber
	currentCertAuthorityKeyId := cert.AuthorityKeyId

	// Capture the current root
	var originalRoot *structs.CARoot
	{
		rootList, activeRoot, err := getTestRoots(s1, "dc1")
		require.NoError(err)
		require.Len(rootList.Roots, 1)
		originalRoot = activeRoot
	}

	waitForActiveCARoot(t, s1, originalRoot)
	waitForActiveCARoot(t, s2, originalRoot)

	// Wait for dc2's intermediate to be refreshed.
	// It is possible that test fails when the blocking query doesn't return.
	// When https://github.com/hashicorp/consul/pull/3777 is merged
	// however, defaultQueryTime will be configurable and we con lower it
	// so that it returns for sure.
	retry.Run(t, func(r *retry.R) {
		secondaryProvider, _ := s2.getCAProvider()
		intermediatePEM, err = secondaryProvider.ActiveIntermediate()
		r.Check(err)
		cert, err := connect.ParseCert(intermediatePEM)
		r.Check(err)
		if cert.SerialNumber.Cmp(currentCertSerialNumber) == 0 || !reflect.DeepEqual(cert.AuthorityKeyId, currentCertAuthorityKeyId) {
			currentCertSerialNumber = cert.SerialNumber
			currentCertAuthorityKeyId = cert.AuthorityKeyId
			r.Fatal("not a renewed intermediate")
		}
	})
	require.NoError(err)

	// Get the new root from dc1 and validate a chain of:
	// dc2 leaf -> dc2 intermediate -> dc1 root
	_, caRoot := s1.getCAProvider()

	// Have dc2 sign a leaf cert and make sure the chain is correct.
	spiffeService := &connect.SpiffeIDService{
		Host:       "node1",
		Namespace:  "default",
		Datacenter: "dc1",
		Service:    "foo",
	}
	raw, _ := connect.TestCSR(t, spiffeService)

	leafCsr, err := connect.ParseCSR(raw)
	require.NoError(err)

	leafPEM, err := secondaryProvider.Sign(leafCsr)
	require.NoError(err)

	cert, err = connect.ParseCert(leafPEM)
	require.NoError(err)

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
	require.NoError(err)
}

func TestLeader_SecondaryCA_IntermediateRefresh(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.Build = "1.6.0"
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
	secondaryProvider, _ := s2.getCAProvider()
	oldIntermediatePEM, err := secondaryProvider.ActiveIntermediate()
	require.NoError(err)
	require.NotEmpty(oldIntermediatePEM)

	// Capture the current root
	var originalRoot *structs.CARoot
	{
		rootList, activeRoot, err := getTestRoots(s1, "dc1")
		require.NoError(err)
		require.Len(rootList.Roots, 1)
		originalRoot = activeRoot
	}

	// Wait for current state to be reflected in both datacenters.
	testrpc.WaitForActiveCARoot(t, s1.RPC, "dc1", originalRoot)
	testrpc.WaitForActiveCARoot(t, s2.RPC, "dc2", originalRoot)

	// Update the provider config to use a new private key, which should
	// cause a rotation.
	_, newKey, err := connect.GeneratePrivateKey()
	require.NoError(err)
	newConfig := &structs.CAConfiguration{
		Provider: "consul",
		Config: map[string]interface{}{
			"PrivateKey":          newKey,
			"RootCert":            "",
			"RotationPeriod":      90 * 24 * time.Hour,
			"IntermediateCertTTL": 72 * 24 * time.Hour,
		},
	}
	{
		args := &structs.CARequest{
			Datacenter: "dc1",
			Config:     newConfig,
		}
		var reply interface{}

		require.NoError(s1.RPC("ConnectCA.ConfigurationSet", args, &reply))
	}

	var updatedRoot *structs.CARoot
	{
		rootList, activeRoot, err := getTestRoots(s1, "dc1")
		require.NoError(err)
		require.Len(rootList.Roots, 2)
		updatedRoot = activeRoot
	}

	testrpc.WaitForActiveCARoot(t, s1.RPC, "dc1", updatedRoot)
	testrpc.WaitForActiveCARoot(t, s2.RPC, "dc2", updatedRoot)

	// Wait for dc2's intermediate to be refreshed.
	var intermediatePEM string
	retry.Run(t, func(r *retry.R) {
		intermediatePEM, err = secondaryProvider.ActiveIntermediate()
		r.Check(err)
		if intermediatePEM == oldIntermediatePEM {
			r.Fatal("not a new intermediate")
		}
	})
	require.NoError(err)

	// Verify the root lists have been rotated in each DC's state store.
	state1 := s1.fsm.State()
	_, primaryRoot, err := state1.CARootActive(nil)
	require.NoError(err)

	state2 := s2.fsm.State()
	_, roots2, err := state2.CARoots(nil)
	require.NoError(err)
	require.Equal(2, len(roots2))

	newRoot := roots2[0]
	oldRoot := roots2[1]
	if roots2[1].Active {
		newRoot = roots2[1]
		oldRoot = roots2[0]
	}
	require.False(oldRoot.Active)
	require.True(newRoot.Active)
	require.Equal(primaryRoot.ID, newRoot.ID)
	require.Equal(primaryRoot.RootCert, newRoot.RootCert)

	// Get the new root from dc1 and validate a chain of:
	// dc2 leaf -> dc2 intermediate -> dc1 root
	_, caRoot := s1.getCAProvider()

	// Have dc2 sign a leaf cert and make sure the chain is correct.
	spiffeService := &connect.SpiffeIDService{
		Host:       "node1",
		Namespace:  "default",
		Datacenter: "dc1",
		Service:    "foo",
	}
	raw, _ := connect.TestCSR(t, spiffeService)

	leafCsr, err := connect.ParseCSR(raw)
	require.NoError(err)

	leafPEM, err := secondaryProvider.Sign(leafCsr)
	require.NoError(err)

	cert, err := connect.ParseCert(leafPEM)
	require.NoError(err)

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
	require.NoError(err)
}

func TestLeader_SecondaryCA_FixSigningKeyID_via_IntermediateRefresh(t *testing.T) {
	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.Build = "1.6.0"
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
		resp, err := s2pre.raftApply(structs.ConnectCARequestType, &structs.CARequest{
			Op:    structs.CAOpSetRoots,
			Index: idx,
			Roots: []*structs.CARoot{activeSecondaryRoot},
		})
		require.NoError(t, err)
		if respErr, ok := resp.(error); ok {
			t.Fatalf("respErr: %v", respErr)
		}
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
		provider, activeRoot := s2.getCAProvider()
		require.NotNil(r, provider)
		require.NotNil(r, activeRoot)

		activeIntermediate, err := provider.ActiveIntermediate()
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

func TestLeader_SecondaryCA_TransitionFromPrimary(t *testing.T) {
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
	require.NoError(t, s2.RPC("ConnectCA.Roots", &args, &dc2PrimaryRoots))
	require.Len(t, dc2PrimaryRoots.Roots, 1)

	// Set the ExternalTrustDomain to a blank string to simulate an old version (pre-1.4.0)
	// it's fine to change the roots struct directly here because the RPC endpoint already
	// makes a copy to return.
	dc2PrimaryRoots.Roots[0].ExternalTrustDomain = ""
	rootSetArgs := structs.CARequest{
		Op:         structs.CAOpSetRoots,
		Datacenter: "dc2",
		Index:      dc2PrimaryRoots.Index,
		Roots:      dc2PrimaryRoots.Roots,
	}
	resp, err := s2.raftApply(structs.ConnectCARequestType, rootSetArgs)
	require.NoError(t, err)
	if respErr, ok := resp.(error); ok {
		t.Fatal(respErr)
	}

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
		require.NoError(r, s1.RPC("ConnectCA.Roots", &args, &dc1Roots))
		require.Len(r, dc1Roots.Roots, 1)

		args = structs.DCSpecificRequest{Datacenter: "dc2"}
		var dc2SecondaryRoots structs.IndexedCARoots
		require.NoError(r, s3.RPC("ConnectCA.Roots", &args, &dc2SecondaryRoots))

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

func TestLeader_SecondaryCA_UpgradeBeforePrimary(t *testing.T) {
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
	secondaryProvider, _ := s2.getCAProvider()
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

		inter, err := secondaryProvider.ActiveIntermediate()
		require.NoError(r, err)
		require.NotEmpty(r, inter, "should have valid intermediate")
	})

	_, caRoot := s1.getCAProvider()
	intermediatePEM, err := secondaryProvider.ActiveIntermediate()
	require.NoError(t, err)

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
	if err := s.RPC("ConnectCA.Roots", rootReq, &rootList); err != nil {
		return nil, nil, err
	}

	var active *structs.CARoot
	for _, root := range rootList.Roots {
		if root.Active {
			active = root
			break
		}
	}

	return &rootList, active, nil
}

func TestLeader_ReplicateIntentions(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	require := require.New(t)
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
		// set the build to ensure all the version checks pass and enable all the connect features that operate cross-dc
		c.Build = "1.6.0"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	s1.tokens.UpdateAgentToken("root", tokenStore.TokenSourceConfig)

	replicationRules := `acl = "read" service_prefix "" { policy = "read" intentions = "read" } operator = "write" `
	// create some tokens
	replToken1, err := upsertTestTokenWithPolicyRules(codec, "root", "dc1", replicationRules)
	require.NoError(err)

	replToken2, err := upsertTestTokenWithPolicyRules(codec, "root", "dc1", replicationRules)
	require.NoError(err)

	// dc2 as a secondary DC
	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLDefaultPolicy = "deny"
		c.ACLTokenReplication = false
		c.Build = "1.6.0"
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	s2.tokens.UpdateAgentToken("root", tokenStore.TokenSourceConfig)

	// start out with one token
	s2.tokens.UpdateReplicationToken(replToken1.SecretID, tokenStore.TokenSourceConfig)

	// Create the WAN link
	joinWAN(t, s2, s1)
	testrpc.WaitForLeader(t, s2.RPC, "dc2")

	// Create an intention in dc1
	ixn := structs.IntentionRequest{
		Datacenter:   "dc1",
		WriteRequest: structs.WriteRequest{Token: "root"},
		Op:           structs.IntentionOpCreate,
		Intention: &structs.Intention{
			SourceNS:        structs.IntentionDefaultNamespace,
			SourceName:      "test",
			DestinationNS:   structs.IntentionDefaultNamespace,
			DestinationName: "test",
			Action:          structs.IntentionActionAllow,
			SourceType:      structs.IntentionSourceConsul,
			Meta:            map[string]string{},
		},
	}
	var reply string
	require.NoError(s1.RPC("Intention.Apply", &ixn, &reply))
	require.NotEmpty(reply)

	// Wait for it to get replicated to dc2
	var createdAt time.Time
	ixn.Intention.ID = reply
	retry.Run(t, func(r *retry.R) {
		req := &structs.IntentionQueryRequest{
			Datacenter:   "dc2",
			QueryOptions: structs.QueryOptions{Token: "root"},
			IntentionID:  ixn.Intention.ID,
		}
		var resp structs.IndexedIntentions
		r.Check(s2.RPC("Intention.Get", req, &resp))
		if len(resp.Intentions) != 1 {
			r.Fatalf("bad: %v", resp.Intentions)
		}
		actual := resp.Intentions[0]
		createdAt = actual.CreatedAt
	})

	// Sleep a bit so that the UpdatedAt field will definitely be different
	time.Sleep(1 * time.Millisecond)

	// delete underlying acl token being used for replication
	require.NoError(deleteTestToken(codec, "root", "dc1", replToken1.AccessorID))

	// switch to the other token
	s2.tokens.UpdateReplicationToken(replToken2.SecretID, tokenStore.TokenSourceConfig)

	// Update the intention in dc1
	ixn.Op = structs.IntentionOpUpdate
	ixn.Intention.ID = reply
	ixn.Intention.SourceName = "*"
	require.NoError(s1.RPC("Intention.Apply", &ixn, &reply))

	// Wait for dc2 to get the update
	ixn.Intention.ID = reply
	var resp structs.IndexedIntentions
	retry.Run(t, func(r *retry.R) {
		req := &structs.IntentionQueryRequest{
			Datacenter:   "dc2",
			QueryOptions: structs.QueryOptions{Token: "root"},
			IntentionID:  ixn.Intention.ID,
		}
		r.Check(s2.RPC("Intention.Get", req, &resp))
		if len(resp.Intentions) != 1 {
			r.Fatalf("bad: %v", resp.Intentions)
		}
		if resp.Intentions[0].SourceName != "*" {
			r.Fatalf("bad: %v", resp.Intentions[0])
		}
	})

	actual := resp.Intentions[0]
	assert.Equal(createdAt, actual.CreatedAt)
	assert.WithinDuration(time.Now(), actual.UpdatedAt, 5*time.Second)

	actual.CreateIndex, actual.ModifyIndex = 0, 0
	actual.CreatedAt = ixn.Intention.CreatedAt
	actual.UpdatedAt = ixn.Intention.UpdatedAt
	ixn.Intention.UpdatePrecedence()
	assert.Equal(ixn.Intention, actual)

	// Delete
	ixn.Op = structs.IntentionOpDelete
	require.NoError(s1.RPC("Intention.Apply", &ixn, &reply))

	// Wait for the delete to be replicated
	retry.Run(t, func(r *retry.R) {
		req := &structs.IntentionQueryRequest{
			Datacenter:   "dc2",
			QueryOptions: structs.QueryOptions{Token: "root"},
			IntentionID:  ixn.Intention.ID,
		}
		var resp structs.IndexedIntentions
		err := s2.RPC("Intention.Get", req, &resp)
		if err == nil || !strings.Contains(err.Error(), ErrIntentionNotFound.Error()) {
			r.Fatalf("expected intention not found")
		}
	})
}

func TestLeader_ReplicateIntentions_forwardToPrimary(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	require := require.New(t)
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// dc2 as a secondary DC
	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc1"
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	// Create the WAN link
	joinWAN(t, s2, s1)
	testrpc.WaitForLeader(t, s2.RPC, "dc2")

	// Create an intention in dc2
	ixn := structs.IntentionRequest{
		Datacenter: "dc2",
		Op:         structs.IntentionOpCreate,
		Intention: &structs.Intention{
			SourceNS:        structs.IntentionDefaultNamespace,
			SourceName:      "test",
			DestinationNS:   structs.IntentionDefaultNamespace,
			DestinationName: "test",
			Action:          structs.IntentionActionAllow,
			SourceType:      structs.IntentionSourceConsul,
			Meta:            map[string]string{},
		},
	}
	var reply string
	require.NoError(s1.RPC("Intention.Apply", &ixn, &reply))
	require.NotEmpty(reply)

	// Make sure it exists in both DCs
	var createdAt time.Time
	ixn.Intention.ID = reply
	retry.Run(t, func(r *retry.R) {
		for _, server := range []*Server{s1, s2} {
			req := &structs.IntentionQueryRequest{
				Datacenter:  server.config.Datacenter,
				IntentionID: ixn.Intention.ID,
			}
			var resp structs.IndexedIntentions
			r.Check(server.RPC("Intention.Get", req, &resp))
			if len(resp.Intentions) != 1 {
				r.Fatalf("bad: %v", resp.Intentions)
			}
			actual := resp.Intentions[0]
			createdAt = actual.CreatedAt
		}
	})

	// Sleep a bit so that the UpdatedAt field will definitely be different
	time.Sleep(1 * time.Millisecond)

	// Update the intention in dc1
	ixn.Op = structs.IntentionOpUpdate
	ixn.Intention.ID = reply
	ixn.Intention.SourceName = "*"
	require.NoError(s1.RPC("Intention.Apply", &ixn, &reply))

	// Wait for dc2 to get the update
	ixn.Intention.ID = reply
	var resp structs.IndexedIntentions
	retry.Run(t, func(r *retry.R) {
		for _, server := range []*Server{s1, s2} {
			req := &structs.IntentionQueryRequest{
				Datacenter:  server.config.Datacenter,
				IntentionID: ixn.Intention.ID,
			}
			r.Check(server.RPC("Intention.Get", req, &resp))
			if len(resp.Intentions) != 1 {
				r.Fatalf("bad: %v", resp.Intentions)
			}
			if resp.Intentions[0].SourceName != "*" {
				r.Fatalf("bad: %v", resp.Intentions[0])
			}
		}
	})

	actual := resp.Intentions[0]
	assert.Equal(createdAt, actual.CreatedAt)
	assert.WithinDuration(time.Now(), actual.UpdatedAt, 5*time.Second)

	actual.CreateIndex, actual.ModifyIndex = 0, 0
	actual.CreatedAt = ixn.Intention.CreatedAt
	actual.UpdatedAt = ixn.Intention.UpdatedAt
	actual.Hash = ixn.Intention.Hash
	ixn.Intention.UpdatePrecedence()
	assert.Equal(ixn.Intention, actual)

	// Delete
	ixn.Op = structs.IntentionOpDelete
	require.NoError(s1.RPC("Intention.Apply", &ixn, &reply))

	// Wait for the delete to be replicated
	retry.Run(t, func(r *retry.R) {
		for _, server := range []*Server{s1, s2} {
			req := &structs.IntentionQueryRequest{
				Datacenter:  server.config.Datacenter,
				IntentionID: ixn.Intention.ID,
			}
			var resp structs.IndexedIntentions
			err := server.RPC("Intention.Get", req, &resp)
			if err == nil || !strings.Contains(err.Error(), ErrIntentionNotFound.Error()) {
				r.Fatalf("expected intention not found")
			}
		}
	})
}

func TestLeader_batchIntentionUpdates(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	ixn1 := structs.TestIntention(t)
	ixn1.ID = "ixn1"
	ixn2 := structs.TestIntention(t)
	ixn2.ID = "ixn2"
	ixnLarge := structs.TestIntention(t)
	ixnLarge.ID = "ixnLarge"
	ixnLarge.Description = strings.Repeat("x", maxIntentionTxnSize-1)

	cases := []struct {
		deletes  structs.Intentions
		updates  structs.Intentions
		expected []structs.TxnOps
	}{
		// 1 deletes, 0 updates
		{
			deletes: structs.Intentions{ixn1},
			expected: []structs.TxnOps{
				structs.TxnOps{
					&structs.TxnOp{
						Intention: &structs.TxnIntentionOp{
							Op:        structs.IntentionOpDelete,
							Intention: ixn1,
						},
					},
				},
			},
		},
		// 0 deletes, 1 updates
		{
			updates: structs.Intentions{ixn1},
			expected: []structs.TxnOps{
				structs.TxnOps{
					&structs.TxnOp{
						Intention: &structs.TxnIntentionOp{
							Op:        structs.IntentionOpUpdate,
							Intention: ixn1,
						},
					},
				},
			},
		},
		// 1 deletes, 1 updates
		{
			deletes: structs.Intentions{ixn1},
			updates: structs.Intentions{ixn2},
			expected: []structs.TxnOps{
				structs.TxnOps{
					&structs.TxnOp{
						Intention: &structs.TxnIntentionOp{
							Op:        structs.IntentionOpDelete,
							Intention: ixn1,
						},
					},
					&structs.TxnOp{
						Intention: &structs.TxnIntentionOp{
							Op:        structs.IntentionOpUpdate,
							Intention: ixn2,
						},
					},
				},
			},
		},
		// 1 large intention update
		{
			updates: structs.Intentions{ixnLarge},
			expected: []structs.TxnOps{
				structs.TxnOps{
					&structs.TxnOp{
						Intention: &structs.TxnIntentionOp{
							Op:        structs.IntentionOpUpdate,
							Intention: ixnLarge,
						},
					},
				},
			},
		},
		// 2 deletes (w/ a large intention), 1 updates
		{
			deletes: structs.Intentions{ixn1, ixnLarge},
			updates: structs.Intentions{ixn2},
			expected: []structs.TxnOps{
				structs.TxnOps{
					&structs.TxnOp{
						Intention: &structs.TxnIntentionOp{
							Op:        structs.IntentionOpDelete,
							Intention: ixn1,
						},
					},
					&structs.TxnOp{
						Intention: &structs.TxnIntentionOp{
							Op:        structs.IntentionOpDelete,
							Intention: ixnLarge,
						},
					},
				},
				structs.TxnOps{
					&structs.TxnOp{
						Intention: &structs.TxnIntentionOp{
							Op:        structs.IntentionOpUpdate,
							Intention: ixn2,
						},
					},
				},
			},
		},
		// 1 deletes , 2 updates (w/ a large intention)
		{
			deletes: structs.Intentions{ixn1},
			updates: structs.Intentions{ixnLarge, ixn2},
			expected: []structs.TxnOps{
				structs.TxnOps{
					&structs.TxnOp{
						Intention: &structs.TxnIntentionOp{
							Op:        structs.IntentionOpDelete,
							Intention: ixn1,
						},
					},
					&structs.TxnOp{
						Intention: &structs.TxnIntentionOp{
							Op:        structs.IntentionOpUpdate,
							Intention: ixnLarge,
						},
					},
				},
				structs.TxnOps{
					&structs.TxnOp{
						Intention: &structs.TxnIntentionOp{
							Op:        structs.IntentionOpUpdate,
							Intention: ixn2,
						},
					},
				},
			},
		},
	}

	for _, tc := range cases {
		actual := batchIntentionUpdates(tc.deletes, tc.updates)
		assert.Equal(tc.expected, actual)
	}
}

func TestLeader_GenerateCASignRequest(t *testing.T) {
	csr := "A"
	s := Server{config: &Config{PrimaryDatacenter: "east"}, tokens: new(token.Store)}
	req := s.generateCASignRequest(csr)
	assert.Equal(t, "east", req.RequestDatacenter())
}

func TestLeader_CARootPruning(t *testing.T) {
	t.Parallel()

	caRootPruneInterval = 200 * time.Millisecond

	require := require.New(t)
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
	require.Nil(msgpackrpc.CallWithCodec(codec, "ConnectCA.Roots", rootReq, &rootList))
	require.Len(rootList.Roots, 1)
	oldRoot := rootList.Roots[0]

	// Update the provider config to use a new private key, which should
	// cause a rotation.
	_, newKey, err := connect.GeneratePrivateKey()
	require.NoError(err)
	newConfig := &structs.CAConfiguration{
		Provider: "consul",
		Config: map[string]interface{}{
			"LeafCertTTL":    "500ms",
			"PrivateKey":     newKey,
			"RootCert":       "",
			"RotationPeriod": "2160h",
			"SkipValidate":   true,
		},
	}
	{
		args := &structs.CARequest{
			Datacenter: "dc1",
			Config:     newConfig,
		}
		var reply interface{}

		require.NoError(msgpackrpc.CallWithCodec(codec, "ConnectCA.ConfigurationSet", args, &reply))
	}

	// Should have 2 roots now.
	_, roots, err := s1.fsm.State().CARoots(nil)
	require.NoError(err)
	require.Len(roots, 2)

	time.Sleep(2 * time.Second)

	// Now the old root should be pruned.
	_, roots, err = s1.fsm.State().CARoots(nil)
	require.NoError(err)
	require.Len(roots, 1)
	require.True(roots[0].Active)
	require.NotEqual(roots[0].ID, oldRoot.ID)
}

func TestLeader_PersistIntermediateCAs(t *testing.T) {
	t.Parallel()

	require := require.New(t)
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
	require.Nil(msgpackrpc.CallWithCodec(codec, "ConnectCA.Roots", rootReq, &rootList))
	require.Len(rootList.Roots, 1)

	// Update the provider config to use a new private key, which should
	// cause a rotation.
	_, newKey, err := connect.GeneratePrivateKey()
	require.NoError(err)
	newConfig := &structs.CAConfiguration{
		Provider: "consul",
		Config: map[string]interface{}{
			"PrivateKey":     newKey,
			"RootCert":       "",
			"RotationPeriod": 90 * 24 * time.Hour,
		},
	}
	{
		args := &structs.CARequest{
			Datacenter: "dc1",
			Config:     newConfig,
		}
		var reply interface{}

		require.NoError(msgpackrpc.CallWithCodec(codec, "ConnectCA.ConfigurationSet", args, &reply))
	}

	// Get the active root before leader change.
	_, root := s1.getCAProvider()
	require.Len(root.IntermediateCerts, 1)

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

		_, newLeaderRoot := leader.getCAProvider()
		if !reflect.DeepEqual(newLeaderRoot, root) {
			r.Fatalf("got %v, want %v", newLeaderRoot, root)
		}
	})
}

func TestLeader_ParseCARoot(t *testing.T) {
	type test struct {
		name             string
		pem              string
		wantSerial       uint64
		wantSigningKeyID string
		wantKeyType      string
		wantKeyBits      int
		wantErr          bool
	}
	// Test certs generated with
	//   go run connect/certgen/certgen.go -out-dir /tmp/connect-certs -key-type ec -key-bits 384
	// for various key types. This does limit the exposure to formats that might
	// exist in external certificates which can be used as Connect CAs.
	// Specifically many other certs will have serial numbers that don't fit into
	// 64 bits but for reasons we truncate down to 64 bits which means our
	// `SerialNumber` will not match the one reported by openssl. We should
	// probably fix that at some point as it seems like a big footgun but it would
	// be a breaking API change to change the type to not be a JSON number and
	// JSON numbers don't even support the full range of a uint64...
	tests := []test{
		{"no cert", "", 0, "", "", 0, true},
		{
			name: "default cert",
			// Watchout for indentations they will break PEM format
			pem: readTestData(t, "cert-with-ec-256-key.pem"),
			// Based on `openssl x509 -noout -text` report from the cert
			wantSerial:       8341954965092507701,
			wantSigningKeyID: "97:4D:17:81:64:F8:B4:AF:05:E8:6C:79:C5:40:3B:0E:3E:8B:C0:AE:38:51:54:8A:2F:05:DB:E3:E8:E4:24:EC",
			wantKeyType:      "ec",
			wantKeyBits:      256,
			wantErr:          false,
		},
		{
			name: "ec 384 cert",
			// Watchout for indentations they will break PEM format
			pem: readTestData(t, "cert-with-ec-384-key.pem"),
			// Based on `openssl x509 -noout -text` report from the cert
			wantSerial:       2935109425518279965,
			wantSigningKeyID: "0B:A0:88:9B:DC:95:31:51:2E:3D:D4:F9:42:D0:6A:A0:62:46:82:D2:7C:22:E7:29:A9:AA:E8:A5:8C:CF:C7:42",
			wantKeyType:      "ec",
			wantKeyBits:      384,
			wantErr:          false,
		},
		{
			name: "rsa 4096 cert",
			// Watchout for indentations they will break PEM format
			pem: readTestData(t, "cert-with-rsa-4096-key.pem"),
			// Based on `openssl x509 -noout -text` report from the cert
			wantSerial:       5186695743100577491,
			wantSigningKeyID: "92:FA:CC:97:57:1E:31:84:A2:33:DD:9B:6A:A8:7C:FC:BE:E2:94:CA:AC:B3:33:17:39:3B:B8:67:9B:DC:C1:08",
			wantKeyType:      "rsa",
			wantKeyBits:      4096,
			wantErr:          false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			root, err := parseCARoot(tt.pem, "consul", "cluster")
			if tt.wantErr {
				require.Error(err)
				return
			}
			require.NoError(err)
			require.Equal(tt.wantSerial, root.SerialNumber)
			require.Equal(strings.ToLower(tt.wantSigningKeyID), root.SigningKeyID)
			require.Equal(tt.wantKeyType, root.PrivateKeyType)
			require.Equal(tt.wantKeyBits, root.PrivateKeyBits)
		})
	}
}

func readTestData(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("testdata", name)
	bs, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatalf("failed reading fixture file %s: %s", name, err)
	}
	return string(bs)
}

func TestLeader_lessThanHalfTimePassed(t *testing.T) {
	now := time.Now()
	require.False(t, lessThanHalfTimePassed(now, now.Add(-10*time.Second), now.Add(-5*time.Second)))
	require.False(t, lessThanHalfTimePassed(now, now.Add(-10*time.Second), now))
	require.False(t, lessThanHalfTimePassed(now, now.Add(-10*time.Second), now.Add(5*time.Second)))
	require.False(t, lessThanHalfTimePassed(now, now.Add(-10*time.Second), now.Add(10*time.Second)))

	require.True(t, lessThanHalfTimePassed(now, now.Add(-10*time.Second), now.Add(20*time.Second)))
}
