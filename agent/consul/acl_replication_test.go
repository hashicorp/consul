package consul

import (
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/authmethod/testauth"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/structs/aclfilter"
	tokenStore "github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
)

func TestACLReplication_diffACLPolicies(t *testing.T) {
	diffACLPolicies := func(local structs.ACLPolicies, remote structs.ACLPolicyListStubs, lastRemoteIndex uint64) ([]string, []string) {
		tr := &aclPolicyReplicator{local: local, remote: remote}
		res := diffACLType(tr, lastRemoteIndex)
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
	diffACLTokens := func(
		local structs.ACLTokens,
		remote structs.ACLTokenListStubs,
		lastRemoteIndex uint64,
	) itemDiffResults {
		tr := &aclTokenReplicator{local: local, remote: remote}
		return diffACLType(tr, lastRemoteIndex)
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	client := rpcClient(t, s1)
	defer client.Close()

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc1"
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

	// Create an auth method in the primary that can create global tokens
	// so that we ensure that these replicate correctly.
	testSessionID := testauth.StartSession()
	defer testauth.ResetSession(testSessionID)
	testauth.InstallSessionToken(testSessionID, "fake-token", "default", "demo", "abc123")
	method1, err := upsertTestCustomizedAuthMethod(client, "root", "dc1", func(method *structs.ACLAuthMethod) {
		method.TokenLocality = "global"
		method.Config = map[string]interface{}{
			"SessionID": testSessionID,
		}
	})
	require.NoError(t, err)
	_, err = upsertTestBindingRule(client, "root", "dc1", method1.Name, "", structs.BindingRuleBindTypeService, "demo")
	require.NoError(t, err)

	// Create one token via this process.
	methodToken := structs.ACLToken{}
	require.NoError(t, s1.RPC("ACL.Login", &structs.ACLLoginRequest{
		Auth: &structs.ACLLoginParams{
			AuthMethod:  method1.Name,
			BearerToken: "fake-token",
		},
		Datacenter: "dc1",
	}, &methodToken))
	tokens = append(tokens, &methodToken)

	checkSame := func(t *retry.R) {
		// only account for global tokens - local tokens shouldn't be replicated
		index, remote, err := s1.fsm.State().ACLTokenList(nil, false, true, "", "", "", nil, nil)
		require.NoError(t, err)
		_, local, err := s2.fsm.State().ACLTokenList(nil, false, true, "", "", "", nil, nil)
		require.NoError(t, err)

		require.Len(t, local, len(remote))
		for i, token := range remote {
			require.Equal(t, token.Hash, local[i].Hash)

			if token.AccessorID == methodToken.AccessorID {
				require.Equal(t, method1.Name, token.AuthMethod)
				require.Equal(t, method1.Name, local[i].AuthMethod)
			}
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	client := rpcClient(t, s1)
	defer client.Close()

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc1"
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
	testrpc.WaitForLeader(t, s1.RPC, "dc2", testrpc.WithToken("root"))
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
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
		c.PrimaryDatacenter = "dc1"
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
			TokenID:      aclfilter.RedactedToken,
			TokenIDType:  structs.ACLTokenSecret,
			QueryOptions: structs.QueryOptions{Token: aclfilter.RedactedToken},
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
		require.Equal(r, status.LastErrorMessage, "failed to retrieve unredacted tokens - replication token in use does not grant acl:write")
	})
}

func TestACLReplication_AllTypes(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	client := rpcClient(t, s1)
	defer client.Close()

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc1"
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
