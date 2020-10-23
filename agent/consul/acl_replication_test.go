package consul

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	tokenStore "github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
	"github.com/stretchr/testify/require"
)

func TestACLReplication_diffACLPolicies(t *testing.T) {
	logger := testutil.Logger(t)
	diffACLPolicies := func(local structs.ACLPolicies, remote structs.ACLPolicyListStubs, lastRemoteIndex uint64) ([]string, []string) {
		tr := &aclPolicyReplicator{local: local, remote: remote}
		res := diffACLType(logger, tr, lastRemoteIndex)
		return res.LocalDeletes, res.LocalUpserts
	}
	local := structs.ACLPolicies{
		&structs.ACLPolicy{
			ID:          "44ef9aec-7654-4401-901b-4d4a8b3c80fc",
			Name:        "policy1",
			Description: "policy1 - already in sync",
			Rules:       `acl = "read"`,
			Syntax:      acl.SyntaxCurrent,
			Datacenters: nil,
			Hash:        []byte{1, 2, 3, 4},
			RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
		},
		&structs.ACLPolicy{
			ID:          "8ea41efb-8519-4091-bc91-c42da0cda9ae",
			Name:        "policy2",
			Description: "policy2 - updated but not changed",
			Rules:       `acl = "read"`,
			Syntax:      acl.SyntaxCurrent,
			Datacenters: nil,
			Hash:        []byte{1, 2, 3, 4},
			RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 25},
		},
		&structs.ACLPolicy{
			ID:          "539f1cb6-40aa-464f-ae66-a900d26bc1b2",
			Name:        "policy3",
			Description: "policy3 - updated and changed",
			Rules:       `acl = "read"`,
			Syntax:      acl.SyntaxCurrent,
			Datacenters: nil,
			Hash:        []byte{1, 2, 3, 4},
			RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 25},
		},
		&structs.ACLPolicy{
			ID:          "e9d33298-6490-4466-99cb-ba93af64fa76",
			Name:        "policy4",
			Description: "policy4 - needs deleting",
			Rules:       `acl = "read"`,
			Syntax:      acl.SyntaxCurrent,
			Datacenters: nil,
			Hash:        []byte{1, 2, 3, 4},
			RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 25},
		},
	}

	remote := structs.ACLPolicyListStubs{
		&structs.ACLPolicyListStub{
			ID:          "44ef9aec-7654-4401-901b-4d4a8b3c80fc",
			Name:        "policy1",
			Description: "policy1 - already in sync",
			Datacenters: nil,
			Hash:        []byte{1, 2, 3, 4},
			CreateIndex: 1,
			ModifyIndex: 2,
		},
		&structs.ACLPolicyListStub{
			ID:          "8ea41efb-8519-4091-bc91-c42da0cda9ae",
			Name:        "policy2",
			Description: "policy2 - updated but not changed",
			Datacenters: nil,
			Hash:        []byte{1, 2, 3, 4},
			CreateIndex: 1,
			ModifyIndex: 50,
		},
		&structs.ACLPolicyListStub{
			ID:          "539f1cb6-40aa-464f-ae66-a900d26bc1b2",
			Name:        "policy3",
			Description: "policy3 - updated and changed",
			Datacenters: nil,
			Hash:        []byte{5, 6, 7, 8},
			CreateIndex: 1,
			ModifyIndex: 50,
		},
		&structs.ACLPolicyListStub{
			ID:          "c6e8fffd-cbd9-4ecd-99fe-ab2f200c7926",
			Name:        "policy5",
			Description: "policy5 - needs adding",
			Datacenters: nil,
			Hash:        []byte{1, 2, 3, 4},
			CreateIndex: 1,
			ModifyIndex: 50,
		},
	}

	// Do the full diff. This full exercises the main body of the loop
	deletions, updates := diffACLPolicies(local, remote, 28)
	require.Len(t, updates, 2)
	require.ElementsMatch(t, updates, []string{
		"c6e8fffd-cbd9-4ecd-99fe-ab2f200c7926",
		"539f1cb6-40aa-464f-ae66-a900d26bc1b2"})

	require.Len(t, deletions, 1)
	require.Equal(t, "e9d33298-6490-4466-99cb-ba93af64fa76", deletions[0])

	deletions, updates = diffACLPolicies(local, nil, 28)
	require.Len(t, updates, 0)
	require.Len(t, deletions, 4)
	require.ElementsMatch(t, deletions, []string{
		"44ef9aec-7654-4401-901b-4d4a8b3c80fc",
		"8ea41efb-8519-4091-bc91-c42da0cda9ae",
		"539f1cb6-40aa-464f-ae66-a900d26bc1b2",
		"e9d33298-6490-4466-99cb-ba93af64fa76"})

	deletions, updates = diffACLPolicies(nil, remote, 28)
	require.Len(t, deletions, 0)
	require.Len(t, updates, 4)
	require.ElementsMatch(t, updates, []string{
		"44ef9aec-7654-4401-901b-4d4a8b3c80fc",
		"8ea41efb-8519-4091-bc91-c42da0cda9ae",
		"539f1cb6-40aa-464f-ae66-a900d26bc1b2",
		"c6e8fffd-cbd9-4ecd-99fe-ab2f200c7926"})
}

func TestACLReplication_diffACLTokens(t *testing.T) {
	logger := testutil.Logger(t)
	diffACLTokens := func(
		local structs.ACLTokens,
		remote structs.ACLTokenListStubs,
		lastRemoteIndex uint64,
	) itemDiffResults {
		tr := &aclTokenReplicator{local: local, remote: remote}
		return diffACLType(logger, tr, lastRemoteIndex)
	}

	local := structs.ACLTokens{
		// When a just-upgraded (1.3->1.4+) secondary DC is replicating from an
		// upgraded primary DC (1.4+), the local state for tokens predating the
		// upgrade will lack AccessorIDs.
		//
		// The primary DC will lazily perform the update to assign AccessorIDs,
		// and that new update will come across the wire locally as a new
		// insert.
		//
		// We simulate that scenario here with 'token0' having no AccessorID in
		// the secondary (local) DC and having an AccessorID assigned in the
		// payload retrieved from the primary (remote) DC.
		&structs.ACLToken{
			AccessorID:  "",
			SecretID:    "5128289f-c22c-4d32-936e-7662443f1a55",
			Description: "token0 - old and not yet upgraded",
			Hash:        []byte{1, 2, 3, 4},
			RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 3},
		},
		&structs.ACLToken{
			AccessorID:  "44ef9aec-7654-4401-901b-4d4a8b3c80fc",
			SecretID:    "44ef9aec-7654-4401-901b-4d4a8b3c80fc",
			Description: "token1 - already in sync",
			Hash:        []byte{1, 2, 3, 4},
			RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
		},
		&structs.ACLToken{
			AccessorID:  "8ea41efb-8519-4091-bc91-c42da0cda9ae",
			SecretID:    "8ea41efb-8519-4091-bc91-c42da0cda9ae",
			Description: "token2 - updated but not changed",
			Hash:        []byte{1, 2, 3, 4},
			RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 25},
		},
		&structs.ACLToken{
			AccessorID:  "539f1cb6-40aa-464f-ae66-a900d26bc1b2",
			SecretID:    "539f1cb6-40aa-464f-ae66-a900d26bc1b2",
			Description: "token3 - updated and changed",
			Hash:        []byte{1, 2, 3, 4},
			RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 25},
		},
		&structs.ACLToken{
			AccessorID:  "e9d33298-6490-4466-99cb-ba93af64fa76",
			SecretID:    "e9d33298-6490-4466-99cb-ba93af64fa76",
			Description: "token4 - needs deleting",
			Hash:        []byte{1, 2, 3, 4},
			RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 25},
		},
	}

	remote := structs.ACLTokenListStubs{
		&structs.ACLTokenListStub{
			AccessorID: "72fac6a3-a014-41c8-9cb2-8d9a5e935f3d",
			//SecretID:    "5128289f-c22c-4d32-936e-7662443f1a55", (formerly)
			Description: "token0 - old and not yet upgraded locally",
			Hash:        []byte{1, 2, 3, 4},
			CreateIndex: 1,
			ModifyIndex: 3,
		},
		&structs.ACLTokenListStub{
			AccessorID:  "44ef9aec-7654-4401-901b-4d4a8b3c80fc",
			Description: "token1 - already in sync",
			Hash:        []byte{1, 2, 3, 4},
			CreateIndex: 1,
			ModifyIndex: 2,
		},
		&structs.ACLTokenListStub{
			AccessorID:  "8ea41efb-8519-4091-bc91-c42da0cda9ae",
			Description: "token2 - updated but not changed",
			Hash:        []byte{1, 2, 3, 4},
			CreateIndex: 1,
			ModifyIndex: 50,
		},
		&structs.ACLTokenListStub{
			AccessorID:  "539f1cb6-40aa-464f-ae66-a900d26bc1b2",
			Description: "token3 - updated and changed",
			Hash:        []byte{5, 6, 7, 8},
			CreateIndex: 1,
			ModifyIndex: 50,
		},
		&structs.ACLTokenListStub{
			AccessorID:  "c6e8fffd-cbd9-4ecd-99fe-ab2f200c7926",
			Description: "token5 - needs adding",
			Hash:        []byte{1, 2, 3, 4},
			CreateIndex: 1,
			ModifyIndex: 50,
		},
		// When a 1.4+ secondary DC is replicating from a 1.4+ primary DC,
		// tokens created using the legacy APIs will not initially have
		// AccessorIDs assigned. That assignment is lazy (but in quick
		// succession).
		//
		// The secondary (local) will see these in the api response as a stub
		// with "" as the AccessorID.
		//
		// We simulate that here to verify that the secondary does the right
		// thing by skipping them until it sees them with nonempty AccessorIDs.
		&structs.ACLTokenListStub{
			AccessorID:  "",
			Description: "token6 - pending async AccessorID assignment",
			Hash:        []byte{1, 2, 3, 4},
			CreateIndex: 51,
			ModifyIndex: 51,
		},
	}

	// Do the full diff. This full exercises the main body of the loop
	t.Run("full-diff", func(t *testing.T) {
		res := diffACLTokens(local, remote, 28)
		require.Equal(t, 1, res.LocalSkipped)
		require.Equal(t, 1, res.RemoteSkipped)
		require.Len(t, res.LocalUpserts, 3)
		require.ElementsMatch(t, res.LocalUpserts, []string{
			"72fac6a3-a014-41c8-9cb2-8d9a5e935f3d",
			"c6e8fffd-cbd9-4ecd-99fe-ab2f200c7926",
			"539f1cb6-40aa-464f-ae66-a900d26bc1b2"})

		require.Len(t, res.LocalDeletes, 1)
		require.Equal(t, "e9d33298-6490-4466-99cb-ba93af64fa76", res.LocalDeletes[0])
	})

	t.Run("only-local", func(t *testing.T) {
		res := diffACLTokens(local, nil, 28)
		require.Equal(t, 1, res.LocalSkipped)
		require.Equal(t, 0, res.RemoteSkipped)
		require.Len(t, res.LocalUpserts, 0)
		require.Len(t, res.LocalDeletes, 4)
		require.ElementsMatch(t, res.LocalDeletes, []string{
			"44ef9aec-7654-4401-901b-4d4a8b3c80fc",
			"8ea41efb-8519-4091-bc91-c42da0cda9ae",
			"539f1cb6-40aa-464f-ae66-a900d26bc1b2",
			"e9d33298-6490-4466-99cb-ba93af64fa76"})
	})

	t.Run("only-remote", func(t *testing.T) {
		res := diffACLTokens(nil, remote, 28)
		require.Equal(t, 0, res.LocalSkipped)
		require.Equal(t, 1, res.RemoteSkipped)
		require.Len(t, res.LocalDeletes, 0)
		require.Len(t, res.LocalUpserts, 5)
		require.ElementsMatch(t, res.LocalUpserts, []string{
			"72fac6a3-a014-41c8-9cb2-8d9a5e935f3d",
			"44ef9aec-7654-4401-901b-4d4a8b3c80fc",
			"8ea41efb-8519-4091-bc91-c42da0cda9ae",
			"539f1cb6-40aa-464f-ae66-a900d26bc1b2",
			"c6e8fffd-cbd9-4ecd-99fe-ab2f200c7926"})
	})
}

func TestACLReplication_Tokens(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	client := rpcClient(t, s1)
	defer client.Close()

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLTokenReplication = true
		c.ACLReplicationRate = 100
		c.ACLReplicationBurst = 100
		c.ACLReplicationApplyLimit = 1000000
	})
	s2.tokens.UpdateReplicationToken("root", tokenStore.TokenSourceConfig)
	testrpc.WaitForLeader(t, s2.RPC, "dc2")
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	// Try to join.
	joinWAN(t, s2, s1)
	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	testrpc.WaitForLeader(t, s1.RPC, "dc2")

	// Wait for legacy acls to be disabled so we are clear that
	// legacy replication isn't meddling.
	waitForNewACLs(t, s1)
	waitForNewACLs(t, s2)
	waitForNewACLReplication(t, s2, structs.ACLReplicateTokens, 1, 1, 0)

	// Create a bunch of new tokens and policies
	var tokens structs.ACLTokens
	for i := 0; i < 50; i++ {
		arg := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				Description: fmt.Sprintf("token-%d", i),
				Policies: []structs.ACLTokenPolicyLink{
					{
						ID: structs.ACLPolicyGlobalManagementID,
					},
				},
				Local: false,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		var token structs.ACLToken
		require.NoError(t, s1.RPC("ACL.TokenSet", &arg, &token))
		tokens = append(tokens, &token)
	}

	checkSame := func(t *retry.R) {
		// only account for global tokens - local tokens shouldn't be replicated
		index, remote, err := s1.fsm.State().ACLTokenList(nil, false, true, "", "", "", nil, nil)
		require.NoError(t, err)
		_, local, err := s2.fsm.State().ACLTokenList(nil, false, true, "", "", "", nil, nil)
		require.NoError(t, err)

		require.Len(t, local, len(remote))
		for i, token := range remote {
			require.Equal(t, token.Hash, local[i].Hash)
		}

		s2.aclReplicationStatusLock.RLock()
		status := s2.aclReplicationStatus
		s2.aclReplicationStatusLock.RUnlock()

		require.True(t, status.Enabled)
		require.True(t, status.Running)
		require.Equal(t, status.ReplicationType, structs.ACLReplicateTokens)
		require.Equal(t, status.ReplicatedTokenIndex, index)
		require.Equal(t, status.SourceDatacenter, "dc1")
	}
	// Wait for the replica to converge.
	retry.Run(t, func(r *retry.R) {
		checkSame(r)
	})

	// Wait for s2 global-management policy
	retry.Run(t, func(r *retry.R) {
		_, policy, err := s2.fsm.State().ACLPolicyGetByID(nil, structs.ACLPolicyGlobalManagementID, nil)
		require.NoError(r, err)
		require.NotNil(r, policy)
	})

	// add some local tokens to the secondary DC
	// these shouldn't be deleted by replication
	for i := 0; i < 50; i++ {
		arg := structs.ACLTokenSetRequest{
			Datacenter: "dc2",
			ACLToken: structs.ACLToken{
				Description: fmt.Sprintf("token-%d", i),
				Policies: []structs.ACLTokenPolicyLink{
					{
						ID: structs.ACLPolicyGlobalManagementID,
					},
				},
				Local: true,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		var token structs.ACLToken
		require.NoError(t, s2.RPC("ACL.TokenSet", &arg, &token))
	}

	// add some local tokens to the primary DC
	// these shouldn't be replicated to the secondary DC
	for i := 0; i < 50; i++ {
		arg := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				Description: fmt.Sprintf("token-%d", i),
				Policies: []structs.ACLTokenPolicyLink{
					{
						ID: structs.ACLPolicyGlobalManagementID,
					},
				},
				Local: true,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		var token structs.ACLToken
		require.NoError(t, s1.RPC("ACL.TokenSet", &arg, &token))
	}

	// Update those other tokens
	for i := 0; i < 50; i++ {
		arg := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				AccessorID:  tokens[i].AccessorID,
				SecretID:    tokens[i].SecretID,
				Description: fmt.Sprintf("token-%d-modified", i),
				Policies: []structs.ACLTokenPolicyLink{
					{
						ID: structs.ACLPolicyGlobalManagementID,
					},
				},
				Local: false,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		var token structs.ACLToken
		require.NoError(t, s1.RPC("ACL.TokenSet", &arg, &token))
	}

	// Wait for the replica to converge.
	// this time it also verifies the local tokens from the primary were not replicated.
	retry.Run(t, func(r *retry.R) {
		checkSame(r)
	})

	// verify dc2 local tokens didn't get blown away
	_, local, err := s2.fsm.State().ACLTokenList(nil, true, false, "", "", "", nil, nil)
	require.NoError(t, err)
	require.Len(t, local, 50)

	for _, token := range tokens {
		arg := structs.ACLTokenDeleteRequest{
			Datacenter:   "dc1",
			TokenID:      token.AccessorID,
			WriteRequest: structs.WriteRequest{Token: "root"},
		}

		var dontCare string
		require.NoError(t, s1.RPC("ACL.TokenDelete", &arg, &dontCare))
	}

	// Wait for the replica to converge.
	retry.Run(t, func(r *retry.R) {
		checkSame(r)
	})
}

func TestACLReplication_Policies(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	client := rpcClient(t, s1)
	defer client.Close()

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLTokenReplication = false
		c.ACLReplicationRate = 100
		c.ACLReplicationBurst = 100
		c.ACLReplicationApplyLimit = 1000000
	})
	s2.tokens.UpdateReplicationToken("root", tokenStore.TokenSourceConfig)
	testrpc.WaitForLeader(t, s2.RPC, "dc2")
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	// Try to join.
	joinWAN(t, s2, s1)
	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	testrpc.WaitForLeader(t, s1.RPC, "dc2")

	// Wait for legacy acls to be disabled so we are clear that
	// legacy replication isn't meddling.
	waitForNewACLs(t, s1)
	waitForNewACLs(t, s2)
	waitForNewACLReplication(t, s2, structs.ACLReplicatePolicies, 1, 0, 0)

	// Create a bunch of new policies
	var policies structs.ACLPolicies
	for i := 0; i < 50; i++ {
		arg := structs.ACLPolicySetRequest{
			Datacenter: "dc1",
			Policy: structs.ACLPolicy{
				Name:        fmt.Sprintf("token-%d", i),
				Description: fmt.Sprintf("token-%d", i),
				Rules:       fmt.Sprintf(`service "app-%d" { policy = "read" }`, i),
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		var policy structs.ACLPolicy
		require.NoError(t, s1.RPC("ACL.PolicySet", &arg, &policy))
		policies = append(policies, &policy)
	}

	checkSame := func(t *retry.R) {
		// only account for global tokens - local tokens shouldn't be replicated
		index, remote, err := s1.fsm.State().ACLPolicyList(nil, nil)
		require.NoError(t, err)
		_, local, err := s2.fsm.State().ACLPolicyList(nil, nil)
		require.NoError(t, err)

		require.Len(t, local, len(remote))
		for i, policy := range remote {
			require.Equal(t, policy.Hash, local[i].Hash)
		}

		s2.aclReplicationStatusLock.RLock()
		status := s2.aclReplicationStatus
		s2.aclReplicationStatusLock.RUnlock()

		require.True(t, status.Enabled)
		require.True(t, status.Running)
		require.Equal(t, status.ReplicationType, structs.ACLReplicatePolicies)
		require.Equal(t, status.ReplicatedIndex, index)
		require.Equal(t, status.SourceDatacenter, "dc1")
	}
	// Wait for the replica to converge.
	retry.Run(t, func(r *retry.R) {
		checkSame(r)
	})

	// Update those policies
	for i := 0; i < 50; i++ {
		arg := structs.ACLPolicySetRequest{
			Datacenter: "dc1",
			Policy: structs.ACLPolicy{
				ID:          policies[i].ID,
				Name:        fmt.Sprintf("token-%d-modified", i),
				Description: fmt.Sprintf("token-%d-modified", i),
				Rules:       policies[i].Rules,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		var policy structs.ACLPolicy
		require.NoError(t, s1.RPC("ACL.PolicySet", &arg, &policy))
	}

	// Wait for the replica to converge.
	// this time it also verifies the local tokens from the primary were not replicated.
	retry.Run(t, func(r *retry.R) {
		checkSame(r)
	})

	for _, policy := range policies {
		arg := structs.ACLPolicyDeleteRequest{
			Datacenter:   "dc1",
			PolicyID:     policy.ID,
			WriteRequest: structs.WriteRequest{Token: "root"},
		}

		var dontCare string
		require.NoError(t, s1.RPC("ACL.PolicyDelete", &arg, &dontCare))
	}

	// Wait for the replica to converge.
	retry.Run(t, func(r *retry.R) {
		checkSame(r)
	})
}

func TestACLReplication_TokensRedacted(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	client := rpcClient(t, s1)
	defer client.Close()

	// Create the ACL Write Policy
	policyArg := structs.ACLPolicySetRequest{
		Datacenter: "dc1",
		Policy: structs.ACLPolicy{
			Name:        "token-replication-redacted",
			Description: "token-replication-redacted",
			Rules:       `acl = "write"`,
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	var policy structs.ACLPolicy
	require.NoError(t, s1.RPC("ACL.PolicySet", &policyArg, &policy))

	// Create the dc2 replication token
	tokenArg := structs.ACLTokenSetRequest{
		Datacenter: "dc1",
		ACLToken: structs.ACLToken{
			Description: "dc2-replication",
			Policies: []structs.ACLTokenPolicyLink{
				{
					ID: policy.ID,
				},
			},
			Local: false,
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}

	var token structs.ACLToken
	require.NoError(t, s1.RPC("ACL.TokenSet", &tokenArg, &token))

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLTokenReplication = true
		c.ACLReplicationRate = 100
		c.ACLReplicationBurst = 100
		c.ACLReplicationApplyLimit = 1000000
	})
	s2.tokens.UpdateReplicationToken(token.SecretID, tokenStore.TokenSourceConfig)
	testrpc.WaitForLeader(t, s2.RPC, "dc2")
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	// Try to join.
	joinWAN(t, s2, s1)
	testrpc.WaitForLeader(t, s2.RPC, "dc2")
	testrpc.WaitForLeader(t, s2.RPC, "dc1")
	waitForNewACLs(t, s2)

	// ensures replication is working ok
	retry.Run(t, func(r *retry.R) {
		var tokenResp structs.ACLTokenResponse
		req := structs.ACLTokenGetRequest{
			Datacenter:   "dc2",
			TokenID:      "root",
			TokenIDType:  structs.ACLTokenSecret,
			QueryOptions: structs.QueryOptions{Token: "root"},
		}
		err := s2.RPC("ACL.TokenRead", &req, &tokenResp)
		require.NoError(r, err)
		require.NotNil(r, tokenResp.Token)
		require.Equal(r, "root", tokenResp.Token.SecretID)

		var status structs.ACLReplicationStatus
		statusReq := structs.DCSpecificRequest{
			Datacenter: "dc2",
		}
		require.NoError(r, s2.RPC("ACL.ReplicationStatus", &statusReq, &status))
		// ensures that tokens are not being synced
		require.True(r, status.ReplicatedTokenIndex > 0, "ReplicatedTokenIndex not greater than 0")

	})

	// modify the replication policy to change to only granting read privileges
	policyArg = structs.ACLPolicySetRequest{
		Datacenter: "dc1",
		Policy: structs.ACLPolicy{
			ID:          policy.ID,
			Name:        "token-replication-redacted",
			Description: "token-replication-redacted",
			Rules:       `acl = "read"`,
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	require.NoError(t, s1.RPC("ACL.PolicySet", &policyArg, &policy))

	// Create the another token so that replication will attempt to read it.
	tokenArg = structs.ACLTokenSetRequest{
		Datacenter: "dc1",
		ACLToken: structs.ACLToken{
			Description: "management",
			Policies: []structs.ACLTokenPolicyLink{
				{
					ID: structs.ACLPolicyGlobalManagementID,
				},
			},
			Local: false,
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	var token2 structs.ACLToken

	// record the time right before we are touching the token
	minErrorTime := time.Now()
	require.NoError(t, s1.RPC("ACL.TokenSet", &tokenArg, &token2))

	retry.Run(t, func(r *retry.R) {
		var tokenResp structs.ACLTokenResponse
		req := structs.ACLTokenGetRequest{
			Datacenter:   "dc2",
			TokenID:      redactedToken,
			TokenIDType:  structs.ACLTokenSecret,
			QueryOptions: structs.QueryOptions{Token: redactedToken},
		}
		err := s2.RPC("ACL.TokenRead", &req, &tokenResp)
		// its not an error for the secret to not be found.
		require.NoError(r, err)
		require.Nil(r, tokenResp.Token)

		var status structs.ACLReplicationStatus
		statusReq := structs.DCSpecificRequest{
			Datacenter: "dc2",
		}
		require.NoError(r, s2.RPC("ACL.ReplicationStatus", &statusReq, &status))
		// ensures that tokens are not being synced
		require.True(r, status.ReplicatedTokenIndex < token2.CreateIndex, "ReplicatedTokenIndex is not less than the token2s create index")
		// ensures that token replication is erroring
		require.True(r, status.LastError.After(minErrorTime), "Replication LastError not after the minErrorTime")
	})
}

func TestACLReplication_AllTypes(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	client := rpcClient(t, s1)
	defer client.Close()

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLTokenReplication = true
		c.ACLReplicationRate = 100
		c.ACLReplicationBurst = 25
		c.ACLReplicationApplyLimit = 1000000
	})
	s2.tokens.UpdateReplicationToken("root", tokenStore.TokenSourceConfig)
	testrpc.WaitForLeader(t, s2.RPC, "dc2")
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	// Try to join.
	joinWAN(t, s2, s1)
	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	testrpc.WaitForLeader(t, s1.RPC, "dc2")

	// Wait for legacy acls to be disabled so we are clear that
	// legacy replication isn't meddling.
	waitForNewACLs(t, s1)
	waitForNewACLs(t, s2)
	waitForNewACLReplication(t, s2, structs.ACLReplicateTokens, 1, 1, 0)

	const (
		numItems             = 50
		numItemsThatAreLocal = 10
	)

	// Create some data.
	policyIDs, roleIDs, tokenIDs := createACLTestData(t, s1, "b1", numItems, numItemsThatAreLocal)

	checkSameTokens := func(t *retry.R) {
		// only account for global tokens - local tokens shouldn't be replicated
		index, remote, err := s1.fsm.State().ACLTokenList(nil, false, true, "", "", "", nil, nil)
		require.NoError(t, err)
		// Query for all of them, so that we can prove that no globals snuck in.
		_, local, err := s2.fsm.State().ACLTokenList(nil, true, true, "", "", "", nil, nil)
		require.NoError(t, err)

		require.Len(t, remote, len(local))
		for i, token := range remote {
			require.Equal(t, token.Hash, local[i].Hash)
		}

		s2.aclReplicationStatusLock.RLock()
		status := s2.aclReplicationStatus
		s2.aclReplicationStatusLock.RUnlock()

		require.True(t, status.Enabled)
		require.True(t, status.Running)
		require.Equal(t, status.ReplicationType, structs.ACLReplicateTokens)
		require.Equal(t, status.ReplicatedTokenIndex, index)
		require.Equal(t, status.SourceDatacenter, "dc1")
	}
	checkSamePolicies := func(t *retry.R) {
		index, remote, err := s1.fsm.State().ACLPolicyList(nil, nil)
		require.NoError(t, err)
		_, local, err := s2.fsm.State().ACLPolicyList(nil, nil)
		require.NoError(t, err)

		require.Len(t, remote, len(local))
		for i, policy := range remote {
			require.Equal(t, policy.Hash, local[i].Hash)
		}

		s2.aclReplicationStatusLock.RLock()
		status := s2.aclReplicationStatus
		s2.aclReplicationStatusLock.RUnlock()

		require.True(t, status.Enabled)
		require.True(t, status.Running)
		require.Equal(t, status.ReplicationType, structs.ACLReplicateTokens)
		require.Equal(t, status.ReplicatedIndex, index)
		require.Equal(t, status.SourceDatacenter, "dc1")
	}
	checkSameRoles := func(t *retry.R) {
		index, remote, err := s1.fsm.State().ACLRoleList(nil, "", nil)
		require.NoError(t, err)
		_, local, err := s2.fsm.State().ACLRoleList(nil, "", nil)
		require.NoError(t, err)

		require.Len(t, remote, len(local))
		for i, role := range remote {
			require.Equal(t, role.Hash, local[i].Hash)
		}

		s2.aclReplicationStatusLock.RLock()
		status := s2.aclReplicationStatus
		s2.aclReplicationStatusLock.RUnlock()

		require.True(t, status.Enabled)
		require.True(t, status.Running)
		require.Equal(t, status.ReplicationType, structs.ACLReplicateTokens)
		require.Equal(t, status.ReplicatedRoleIndex, index)
		require.Equal(t, status.SourceDatacenter, "dc1")
	}
	checkSame := func(t *retry.R) {
		checkSameTokens(t)
		checkSamePolicies(t)
		checkSameRoles(t)
	}
	// Wait for the replica to converge.
	retry.Run(t, func(r *retry.R) {
		checkSame(r)
	})

	// Create additional data to replicate.
	_, _, _ = createACLTestData(t, s1, "b2", numItems, numItemsThatAreLocal)

	// Wait for the replica to converge.
	retry.Run(t, func(r *retry.R) {
		checkSame(r)
	})

	// Delete one piece of each type of data from batch 1.
	const itemToDelete = numItems - 1
	{
		id := tokenIDs[itemToDelete]

		arg := structs.ACLTokenDeleteRequest{
			Datacenter:   "dc1",
			TokenID:      id,
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		var dontCare string
		if err := s1.RPC("ACL.TokenDelete", &arg, &dontCare); err != nil {
			t.Fatalf("err: %v", err)
		}
	}
	{
		id := roleIDs[itemToDelete]

		arg := structs.ACLRoleDeleteRequest{
			Datacenter:   "dc1",
			RoleID:       id,
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		var dontCare string
		if err := s1.RPC("ACL.RoleDelete", &arg, &dontCare); err != nil {
			t.Fatalf("err: %v", err)
		}
	}
	{
		id := policyIDs[itemToDelete]

		arg := structs.ACLPolicyDeleteRequest{
			Datacenter:   "dc1",
			PolicyID:     id,
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		var dontCare string
		if err := s1.RPC("ACL.PolicyDelete", &arg, &dontCare); err != nil {
			t.Fatalf("err: %v", err)
		}
	}
	// Wait for the replica to converge.
	retry.Run(t, func(r *retry.R) {
		checkSame(r)
	})
}

func createACLTestData(t *testing.T, srv *Server, namePrefix string, numObjects, numItemsThatAreLocal int) (policyIDs, roleIDs, tokenIDs []string) {
	require.True(t, numItemsThatAreLocal <= numObjects, 0, "numItemsThatAreLocal <= numObjects")

	// Create some policies.
	for i := 0; i < numObjects; i++ {
		str := strconv.Itoa(i)
		arg := structs.ACLPolicySetRequest{
			Datacenter: "dc1",
			Policy: structs.ACLPolicy{
				Name:        namePrefix + "-policy-" + str,
				Description: namePrefix + "-policy " + str,
				Rules:       testACLPolicyNew,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		var out structs.ACLPolicy
		if err := srv.RPC("ACL.PolicySet", &arg, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
		policyIDs = append(policyIDs, out.ID)
	}

	// Create some roles.
	for i := 0; i < numObjects; i++ {
		str := strconv.Itoa(i)
		arg := structs.ACLRoleSetRequest{
			Datacenter: "dc1",
			Role: structs.ACLRole{
				Name:        namePrefix + "-role-" + str,
				Description: namePrefix + "-role " + str,
				Policies: []structs.ACLRolePolicyLink{
					{ID: policyIDs[i]},
				},
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		var out structs.ACLRole
		if err := srv.RPC("ACL.RoleSet", &arg, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
		roleIDs = append(roleIDs, out.ID)
	}

	// Create a bunch of new tokens.
	for i := 0; i < numObjects; i++ {
		str := strconv.Itoa(i)
		arg := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				Description: namePrefix + "-token " + str,
				Policies: []structs.ACLTokenPolicyLink{
					{ID: policyIDs[i]},
				},
				Roles: []structs.ACLTokenRoleLink{
					{ID: roleIDs[i]},
				},
				Local: (i < numItemsThatAreLocal),
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		var out structs.ACLToken
		if err := srv.RPC("ACL.TokenSet", &arg, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
		tokenIDs = append(tokenIDs, out.AccessorID)
	}

	return policyIDs, roleIDs, tokenIDs
}

func TestACLReplication_AllTypes_CorrectedAfterUpgrade(t *testing.T) {
	// If during an in-flight cross-datacenter consul version upgrade the
	// primary datacenter is updated first, and an operator elects to persist a
	// field into a Token/Role/Policy that did not exist in the prior consul
	// version then it can lead to the secondary datacenters replicating all
	// but the field it doesn't understand, but it does persist the hash of the
	// new fields (since that's computed by the primary and persisted).
	//
	// There used to be a bug whereby after the secondary is finally upgraded
	// it would never go back and figure out that it was missing some data.
	//
	// This tests that the bug is fixed. Because we're running on a single
	// version of consul we make a mirror universe version of the buggy
	// behavior where we persist an incorrect Hash in the primary's state
	// machine so that when it replicates the secondary gets into a similar
	// situation.
	//
	// We'll use edits to the Description as a proxy for "new fields".

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	client := rpcClient(t, s1)
	defer client.Close()

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc1"
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLTokenReplication = true
		c.ACLReplicationRate = 100
		c.ACLReplicationBurst = 25
		c.ACLReplicationApplyLimit = 1000000
	})
	s2.tokens.UpdateReplicationToken("root", tokenStore.TokenSourceConfig)
	testrpc.WaitForLeader(t, s2.RPC, "dc2")
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	// Try to join.
	joinWAN(t, s2, s1)
	testrpc.WaitForLeader(t, s2.RPC, "dc2")
	testrpc.WaitForLeader(t, s2.RPC, "dc1")

	// Wait for legacy acls to be disabled so we are clear that
	// legacy replication isn't meddling.
	waitForNewACLs(t, s1)
	waitForNewACLs(t, s2)
	waitForNewACLReplication(t, s2, structs.ACLReplicateTokens, 1, 1, 0)

	// Create one of each type of data the proper way, simulating a write + replication
	// of pre-upgrade data on both sides.
	var policyID string
	{
		arg := structs.ACLPolicySetRequest{
			Datacenter: "dc1",
			Policy: structs.ACLPolicy{
				Name:  "test-policy",
				Rules: `key_prefix "" { policy = "deny" }`,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		var out structs.ACLPolicy
		require.NoError(t, s1.RPC("ACL.PolicySet", &arg, &out))
		policyID = out.ID
	}
	t.Logf("created policy with id=%q", policyID)

	var roleID string
	{
		arg := structs.ACLRoleSetRequest{
			Datacenter: "dc1",
			Role: structs.ACLRole{
				Name: "test-role",
				Policies: []structs.ACLRolePolicyLink{
					{ID: policyID},
				},
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		var out structs.ACLRole
		require.NoError(t, s1.RPC("ACL.RoleSet", &arg, &out))
		roleID = out.ID
	}
	t.Logf("created role with id=%q", roleID)

	var tokenAccessor string
	{
		arg := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				Description: "test-token",
				Policies: []structs.ACLTokenPolicyLink{
					{ID: policyID},
				},
				Roles: []structs.ACLTokenRoleLink{
					{ID: roleID},
				},
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		var out structs.ACLToken
		require.NoError(t, s1.RPC("ACL.TokenSet", &arg, &out))
		tokenAccessor = out.AccessorID
	}
	t.Logf("created token with accessor=%q", tokenAccessor)

	checkSame := func(r *retry.R, s2 *Server) {
		{
			_, expect, err := s1.fsm.State().ACLPolicyGetByID(nil, policyID, nil)
			require.NoError(r, err)
			_, got, err := s2.fsm.State().ACLPolicyGetByID(nil, policyID, nil)
			require.NoError(r, err)

			require.NotNil(r, expect)
			require.NotNil(r, got)

			require.Equal(r, expect.ID, got.ID)
			require.Equal(r, expect.Hash, got.Hash)
			require.Equal(r, expect.Description, got.Description) // is our new field correct?
		}
		{
			_, expect, err := s1.fsm.State().ACLRoleGetByID(nil, roleID, nil)
			require.NoError(r, err)
			_, got, err := s2.fsm.State().ACLRoleGetByID(nil, roleID, nil)
			require.NoError(r, err)

			require.NotNil(r, expect)
			require.NotNil(r, got)

			require.Equal(r, expect.ID, got.ID)
			require.Equal(r, expect.Hash, got.Hash)
			require.Equal(r, expect.Description, got.Description) // is our new field correct?
		}
		{
			_, expect, err := s1.fsm.State().ACLTokenGetByAccessor(nil, tokenAccessor, nil)
			require.NoError(r, err)
			_, got, err := s2.fsm.State().ACLTokenGetByAccessor(nil, tokenAccessor, nil)
			require.NoError(r, err)

			require.NotNil(r, expect)
			require.NotNil(r, got)

			require.Equal(r, expect.AccessorID, got.AccessorID)
			require.Equal(r, expect.Hash, got.Hash)
			require.Equal(r, expect.Description, got.Description) // is our new field correct?
		}
	}

	// Wait for each of the 3 data types to replicate with identical hashes.
	retry.Run(t, func(r *retry.R) {
		checkSame(r, s2)
	})

	// OK, now let's simulate a primary upgrade followed immediately by a new
	// field edit.
	//
	// First we'll bypass replication safeties with a direct raftApply and
	// update the hashes to the new value in the secondary first. This is like
	// setting a new field in the primary that is factored into the hash and
	// letting it replicate using OLD CODE from before this bugfix.
	//
	// Then we'll shutdown the secondary. While it's shutdown we'll simulate
	// the new field and updated hash both being visible in the primary, which
	// is an approximation of what a newly upgraded secondary would "see".
	//
	// Then we power the secondary back on and let the patched replicators do
	// their job on startup and correct it.

	var (
		policyJustHashSecondary, policyFull *structs.ACLPolicy
	)
	{
		_, policy, err := s1.fsm.State().ACLPolicyGetByID(nil, policyID, nil)
		require.NoError(t, err)
		require.NotNil(t, policy)
		_, policy2, err := s2.fsm.State().ACLPolicyGetByID(nil, policyID, nil)
		require.NoError(t, err)
		require.NotNil(t, policy2)

		policyFull = policy.Clone()
		policyFull.Description = "edited"
		policyFull.SetHash(true)

		policyJustHashSecondary = policy2.Clone()
		policyJustHashSecondary.Hash = policyFull.Hash

		// Double check some hashes
		require.NotEqual(t, policy.Hash, policyFull.Hash)
		require.Equal(t, policyFull.Hash, policyJustHashSecondary.Hash)

		// Double check the descriptions.
		require.NotEqual(t, policy.Description, policyFull.Description)
		require.Equal(t, policy.Description, policyJustHashSecondary.Description)
	}

	var (
		roleJustHashSecondary, roleFull *structs.ACLRole
	)
	{
		_, role, err := s1.fsm.State().ACLRoleGetByID(nil, roleID, nil)
		require.NoError(t, err)
		require.NotNil(t, role)
		_, role2, err := s2.fsm.State().ACLRoleGetByID(nil, roleID, nil)
		require.NoError(t, err)
		require.NotNil(t, role2)

		roleFull = role.Clone()
		roleFull.Description = "edited"
		roleFull.SetHash(true)

		roleJustHashSecondary = role2.Clone()
		roleJustHashSecondary.Hash = roleFull.Hash

		// Double check some hashes
		require.NotEqual(t, role.Hash, roleFull.Hash)
		require.Equal(t, roleFull.Hash, roleJustHashSecondary.Hash)

		// Double check the descriptions.
		require.NotEqual(t, role.Description, roleFull.Description)
		require.Equal(t, role.Description, roleJustHashSecondary.Description)
	}

	var (
		tokenJustHashSecondary, tokenFull *structs.ACLToken
	)
	{
		_, token, err := s1.fsm.State().ACLTokenGetByAccessor(nil, tokenAccessor, nil)
		require.NoError(t, err)
		require.NotNil(t, token)
		_, token2, err := s2.fsm.State().ACLTokenGetByAccessor(nil, tokenAccessor, nil)
		require.NoError(t, err)
		require.NotNil(t, token2)

		tokenFull = token.Clone()
		tokenFull.Description = "edited"
		tokenFull.SetHash(true)

		tokenJustHashSecondary = token2.Clone()
		tokenJustHashSecondary.Hash = tokenFull.Hash

		// Double check some hashes
		require.NotEqual(t, token.Hash, tokenFull.Hash)
		require.Equal(t, tokenFull.Hash, tokenJustHashSecondary.Hash)

		// Double check the descriptions.
		require.NotEqual(t, token.Description, tokenFull.Description)
		require.Equal(t, token.Description, tokenJustHashSecondary.Description)
	}

	setPolicy := func(t *testing.T, srv *Server, policy *structs.ACLPolicy) {
		req := &structs.ACLPolicyBatchSetRequest{
			Policies: structs.ACLPolicies{policy},
		}

		resp, err := srv.raftApply(structs.ACLPolicySetRequestType, req)
		require.NoError(t, err)
		if respErr, ok := resp.(error); ok {
			t.Fatalf("err: %v", respErr)
		}
	}
	setRole := func(t *testing.T, srv *Server, role *structs.ACLRole) {
		req := &structs.ACLRoleBatchSetRequest{
			Roles: structs.ACLRoles{role},
		}

		resp, err := srv.raftApply(structs.ACLRoleSetRequestType, req)
		require.NoError(t, err)
		if respErr, ok := resp.(error); ok {
			t.Fatalf("err: %v", respErr)
		}
	}
	setToken := func(t *testing.T, srv *Server, token *structs.ACLToken) {
		req := &structs.ACLTokenBatchSetRequest{
			Tokens: structs.ACLTokens{token},
			CAS:    false,
		}

		resp, err := srv.raftApply(structs.ACLTokenSetRequestType, req)
		require.NoError(t, err)
		if respErr, ok := resp.(error); ok {
			t.Fatalf("err: %v", respErr)
		}
	}

	// Force the weird hash-only update in the secondary.
	setPolicy(t, s2, policyJustHashSecondary)
	setRole(t, s2, roleJustHashSecondary)
	setToken(t, s2, tokenJustHashSecondary)

	// Stop the secondary so we know the replication routines will restart completely.
	s2.Shutdown()

	// To avoid any previous replicator goroutines from manipulating the data
	// directory while it's shutting down (and thus seeing our actual write
	// below, rather than letting the NEXT server instance see it) we'll copy
	// the data directory and feed that one to the new server rather than
	// having them possibly share.
	newDataDir := testutil.TempDir(t, "datadir-s2-restart")
	require.NoError(t, testCopyDir(s2.config.DataDir, newDataDir))

	// While s2 isn't running, re-introduce the edited Description field to
	// simulate being able to "see" the new field.
	setPolicy(t, s1, policyFull)
	setRole(t, s1, roleFull)
	setToken(t, s1, tokenFull)

	// Now restart it, simulating the restart after an upgrade.
	dir2_restart, s2_restart := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc1"
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLTokenReplication = true
		c.ACLReplicationRate = 100
		c.ACLReplicationBurst = 25
		c.ACLReplicationApplyLimit = 1000000
		// use the cloned data
		c.DataDir = newDataDir
		// use the same name
		c.NodeName = s2.config.NodeName
		c.NodeID = s2.config.NodeID
		// use the same ports
		c.SerfLANConfig.MemberlistConfig = s2.config.SerfLANConfig.MemberlistConfig
		c.SerfWANConfig.MemberlistConfig = s2.config.SerfWANConfig.MemberlistConfig
		c.RPCAddr = s2.config.RPCAddr
		c.RPCAdvertise = s2.config.RPCAdvertise
	})
	defer os.RemoveAll(dir2_restart)
	defer s2_restart.Shutdown()
	s2_restart.tokens.UpdateReplicationToken("root", tokenStore.TokenSourceConfig)
	testrpc.WaitForLeader(t, s2_restart.RPC, "dc2")

	// If the bug is gone, they should be identical.
	retry.Run(t, func(r *retry.R) {
		checkSame(r, s2_restart)
	})
}

// testCopyFile is roughly the same as "cp -a src dst", and only works for
// directories and regular files recursively.
//
// It's a heavily trimmed down version of "CopyDir" from
// https://github.com/moby/moby/blob/master/daemon/graphdriver/copy/copy.go
func testCopyDir(srcDir, dstDir string) error {
	testCopyFile := func(srcPath, dstPath string) error {
		srcFile, err := os.Open(srcPath)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		dstFile, err := os.Create(dstPath)
		if err != nil {
			return err
		}

		if _, err = io.Copy(dstFile, srcFile); err != nil {
			_ = dstFile.Close()
			return err
		}

		return dstFile.Close()
	}

	return filepath.Walk(srcDir, func(srcPath string, f os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Rebase path
		relPath, err := filepath.Rel(srcDir, srcPath)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dstDir, relPath)

		switch mode := f.Mode(); {
		case mode.IsRegular():
			if err2 := testCopyFile(srcPath, dstPath); err2 != nil {
				return err2
			}
		case mode.IsDir():
			if err := os.Mkdir(dstPath, f.Mode()); err != nil && !os.IsExist(err) {
				return err
			}
		default:
			return fmt.Errorf("unknown file type (%d / %s) for %s", f.Mode(), f.Mode().String(), srcPath)
		}

		if err := os.Chmod(dstPath, f.Mode()); err != nil {
			return err
		}

		return nil
	})
}
