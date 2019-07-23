package consul

import (
	"crypto/x509"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
	uuid "github.com/hashicorp/go-uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLeader_SecondaryCA_Initialize(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	masterToken := "8a85f086-dd95-4178-b128-e10902767c5c"

	// Initialize primary as the primary DC
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "primary"
		c.PrimaryDatacenter = "primary"
		c.Build = "1.4.0"
		c.ACLsEnabled = true
		c.ACLMasterToken = masterToken
		c.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	s1.tokens.UpdateAgentToken(masterToken, token.TokenSourceConfig)

	testrpc.WaitForLeader(t, s1.RPC, "primary")

	// secondary as a secondary DC
	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "secondary"
		c.PrimaryDatacenter = "primary"
		c.Build = "1.4.0"
		c.ACLsEnabled = true
		c.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	s2.tokens.UpdateAgentToken(masterToken, token.TokenSourceConfig)
	s2.tokens.UpdateReplicationToken(masterToken, token.TokenSourceConfig)

	// Create the WAN link
	joinWAN(t, s2, s1)
	testrpc.WaitForLeader(t, s2.RPC, "secondary")

	_, caRoot := s1.getCAProvider()
	secondaryProvider, _ := s2.getCAProvider()
	intermediatePEM, err := secondaryProvider.ActiveIntermediate()
	require.NoError(err)

	// Verify the root lists are equal in each DC's state store.
	state1 := s1.fsm.State()
	_, roots1, err := state1.CARoots(nil)
	require.NoError(err)

	state2 := s2.fsm.State()
	_, roots2, err := state2.CARoots(nil)
	require.NoError(err)
	require.Equal(roots1[0].ID, roots2[0].ID)
	require.Equal(roots1[0].RootCert, roots2[0].RootCert)
	require.Equal(1, len(roots1))
	require.Equal(len(roots1), len(roots2))

	// Have secondary sign a leaf cert and make sure the chain is correct.
	spiffeService := &connect.SpiffeIDService{
		Host:       "node1",
		Namespace:  "default",
		Datacenter: "primary",
		Service:    "foo",
	}
	raw, _ := connect.TestCSR(t, spiffeService)

	leafCsr, err := connect.ParseCSR(raw)
	require.NoError(err)

	leafPEM, err := secondaryProvider.Sign(leafCsr)
	require.NoError(err)

	cert, err := connect.ParseCert(leafPEM)
	require.NoError(err)

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
	require.NoError(err)
}

func TestLeader_SecondaryCA_IntermediateRefresh(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.Build = "1.4.0"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// dc2 as a secondary DC
	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc1"
		c.Build = "1.4.0"
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

	// Store the current root
	rootReq := &structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	var rootList structs.IndexedCARoots
	require.NoError(s1.RPC("ConnectCA.Roots", rootReq, &rootList))
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

		require.NoError(s1.RPC("ConnectCA.ConfigurationSet", args, &reply))
	}

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

func TestLeader_SecondaryCA_TransitionFromPrimary(t *testing.T) {
	t.Parallel()

	// Initialize dc1 as the primary DC
	id1, err := uuid.GenerateUUID()
	require.NoError(t, err)
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.CAConfig.ClusterID = id1
		c.Build = "1.4.0"
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
		c.Build = "1.4.0"
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

	require := require.New(t)
	maxRootsQueryTime = 500 * time.Millisecond

	// Initialize dc1 as the primary DC
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.Build = "1.3.0"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// dc2 as a secondary DC
	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc1"
		c.Build = "1.4.0"
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	// Create the WAN link
	joinWAN(t, s2, s1)
	testrpc.WaitForLeader(t, s2.RPC, "dc2")

	// Verify the root lists are different in each DC's state store.
	var oldSecondaryRootID string
	{
		state1 := s1.fsm.State()
		_, roots1, err := state1.CARoots(nil)
		require.NoError(err)

		state2 := s2.fsm.State()
		_, roots2, err := state2.CARoots(nil)
		require.NoError(err)
		require.Equal(1, len(roots1))
		require.Equal(1, len(roots2))
		require.NotEqual(roots1[0].ID, roots2[0].ID)
		require.NotEqual(roots1[0].RootCert, roots2[0].RootCert)
		oldSecondaryRootID = roots2[0].ID
	}

	// Update the version on the fly so s2 kicks off the secondary DC transition.
	tags := s1.config.SerfLANConfig.Tags
	tags["build"] = "1.4.0"
	s1.serfLAN.SetTags(tags)

	// Wait for the secondary transition to happen and then verify the secondary DC
	// has both roots present.
	secondaryProvider, _ := s2.getCAProvider()
	retry.Run(t, func(r *retry.R) {
		state := s2.fsm.State()
		_, roots, err := state.CARoots(nil)
		r.Check(err)
		if len(roots) != 2 {
			r.Fatalf("should have 2 roots: %v", roots)
		}
		inter, err := secondaryProvider.ActiveIntermediate()
		r.Check(err)
		if inter == "" {
			r.Fatal("should have valid intermediate")
		}
	})
	{
		state1 := s1.fsm.State()
		_, roots1, err := state1.CARoots(nil)
		require.NoError(err)

		state2 := s2.fsm.State()
		_, roots2, err := state2.CARoots(nil)
		require.NoError(err)
		require.Equal(1, len(roots1))
		require.Equal(2, len(roots2))
		var oldSecondaryRoot *structs.CARoot
		var newSecondaryRoot *structs.CARoot
		if roots2[0].ID == oldSecondaryRootID {
			oldSecondaryRoot = roots2[0]
			newSecondaryRoot = roots2[1]
		} else {
			oldSecondaryRoot = roots2[1]
			newSecondaryRoot = roots2[0]
		}
		require.Equal(roots1[0].ID, newSecondaryRoot.ID)
		require.Equal(roots1[0].RootCert, newSecondaryRoot.RootCert)
		require.NotEqual(newSecondaryRoot.ID, oldSecondaryRoot.ID)
		require.NotEqual(newSecondaryRoot.RootCert, oldSecondaryRoot.RootCert)
	}

	_, caRoot := s1.getCAProvider()
	intermediatePEM, err := secondaryProvider.ActiveIntermediate()
	require.NoError(err)

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
	require.NoError(err)
}

func TestLeader_ReplicateIntentions(t *testing.T) {
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

	// Create an intention in dc1
	ixn := structs.IntentionRequest{
		Datacenter: "dc1",
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

	// Wait for it to get replicated to dc2
	var createdAt time.Time
	ixn.Intention.ID = reply
	retry.Run(t, func(r *retry.R) {
		req := &structs.IntentionQueryRequest{
			Datacenter:  "dc2",
			IntentionID: ixn.Intention.ID,
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
			Datacenter:  "dc2",
			IntentionID: ixn.Intention.ID,
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
			Datacenter:  "dc2",
			IntentionID: ixn.Intention.ID,
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
