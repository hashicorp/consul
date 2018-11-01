package consul

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/testutil/retry"
	"github.com/stretchr/testify/require"
)

func TestACLReplication_diffACLPolicies(t *testing.T) {
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
	s2.tokens.UpdateACLReplicationToken("root")
	testrpc.WaitForLeader(t, s2.RPC, "dc2")
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	// Try to join.
	joinWAN(t, s2, s1)
	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	testrpc.WaitForLeader(t, s1.RPC, "dc2")

	// Create a bunch of new tokens and policies
	var tokens structs.ACLTokens
	for i := 0; i < 50; i++ {
		arg := structs.ACLTokenSetRequest{
			Datacenter: "dc1",
			ACLToken: structs.ACLToken{
				Description: fmt.Sprintf("token-%d", i),
				Policies: []structs.ACLTokenPolicyLink{
					structs.ACLTokenPolicyLink{
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

	checkSame := func(t *retry.R) error {
		// only account for global tokens - local tokens shouldn't be replicated
		index, remote, err := s1.fsm.State().ACLTokenList(nil, false, true, "")
		require.NoError(t, err)
		_, local, err := s2.fsm.State().ACLTokenList(nil, false, true, "")
		require.NoError(t, err)

		require.Len(t, local, len(remote))
		for i, token := range remote {
			require.Equal(t, token.Hash, local[i].Hash)
		}

		var status structs.ACLReplicationStatus
		s2.aclReplicationStatusLock.RLock()
		status = s2.aclReplicationStatus
		s2.aclReplicationStatusLock.RUnlock()
		if !status.Enabled || !status.Running ||
			status.ReplicationType != structs.ACLReplicateTokens ||
			status.ReplicatedTokenIndex != index ||
			status.SourceDatacenter != "dc1" {
			return fmt.Errorf("ACL replication status differs")
		}

		return nil
	}
	// Wait for the replica to converge.
	retry.Run(t, func(r *retry.R) {
		checkSame(r)
	})

	// add some local tokens to the secondary DC
	// these shouldn't be deleted by replication
	for i := 0; i < 50; i++ {
		arg := structs.ACLTokenSetRequest{
			Datacenter: "dc2",
			ACLToken: structs.ACLToken{
				Description: fmt.Sprintf("token-%d", i),
				Policies: []structs.ACLTokenPolicyLink{
					structs.ACLTokenPolicyLink{
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
					structs.ACLTokenPolicyLink{
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
					structs.ACLTokenPolicyLink{
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
	_, local, err := s2.fsm.State().ACLTokenList(nil, true, false, "")
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
	s2.tokens.UpdateACLReplicationToken("root")
	testrpc.WaitForLeader(t, s2.RPC, "dc2")
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	// Try to join.
	joinWAN(t, s2, s1)
	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	testrpc.WaitForLeader(t, s1.RPC, "dc2")

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

	checkSame := func(t *retry.R) error {
		// only account for global tokens - local tokens shouldn't be replicated
		index, remote, err := s1.fsm.State().ACLPolicyList(nil)
		require.NoError(t, err)
		_, local, err := s2.fsm.State().ACLPolicyList(nil)
		require.NoError(t, err)

		require.Len(t, local, len(remote))
		for i, policy := range remote {
			require.Equal(t, policy.Hash, local[i].Hash)
		}

		var status structs.ACLReplicationStatus
		s2.aclReplicationStatusLock.RLock()
		status = s2.aclReplicationStatus
		s2.aclReplicationStatusLock.RUnlock()
		if !status.Enabled || !status.Running ||
			status.ReplicationType != structs.ACLReplicatePolicies ||
			status.ReplicatedIndex != index ||
			status.SourceDatacenter != "dc1" {
			return fmt.Errorf("ACL replication status differs")
		}

		return nil
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
