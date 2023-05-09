package state

import (
	"fmt"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-uuid"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbacl"
)

const (
	testRoleID_A   = "2c74a9b8-271c-4a21-b727-200db397c01c" // from:setupExtraPoliciesAndRoles
	testRoleID_B   = "aeab6b63-08d1-455a-b85b-3458b462b426" // from:setupExtraPoliciesAndRoles
	testPolicyID_A = "a0625e95-9b3e-42de-a8d6-ceef5b6f3286" // from:setupExtraPolicies
	testPolicyID_B = "9386ecae-6677-4686-bcd4-5ab9d86cca1d" // from:setupExtraPolicies
	testPolicyID_C = "2bf7359d-cfde-4769-a9fa-54ff1bb2ae4c" // from:setupExtraPolicies
	testPolicyID_D = "ff807410-2b82-48ae-9a63-6626a90789d0" // from:setupExtraPolicies
	testPolicyID_E = "b4635d48-90aa-4a77-8e1b-9004f68bb3df" // from:setupExtraPolicies
)

func setupGlobalManagement(t *testing.T, s *Store) {
	policy := structs.ACLPolicy{
		ID:          structs.ACLPolicyGlobalManagementID,
		Name:        "global-management",
		Description: "Builtin Policy that grants unlimited access",
		Rules:       structs.ACLPolicyGlobalManagement,
		Syntax:      acl.SyntaxCurrent,
	}
	policy.SetHash(true)
	require.NoError(t, s.ACLPolicySet(1, &policy))
}

func setupAnonymous(t *testing.T, s *Store) {
	token := structs.ACLToken{
		AccessorID:  structs.ACLTokenAnonymousID,
		SecretID:    "anonymous",
		Description: "Anonymous Token",
	}
	token.SetHash(true)
	require.NoError(t, s.ACLTokenSet(1, &token))
}

func testACLStateStore(t *testing.T) *Store {
	s := testStateStore(t)
	setupGlobalManagement(t, s)
	setupAnonymous(t, s)
	return s
}

func setupExtraAuthMethods(t *testing.T, s *Store) {
	methods := structs.ACLAuthMethods{
		&structs.ACLAuthMethod{
			Name:        "test",
			Type:        "testing",
			Description: "test",
		},
	}
	require.NoError(t, s.ACLAuthMethodBatchSet(2, methods))
}

func setupExtraPolicies(t *testing.T, s *Store) {
	policies := structs.ACLPolicies{
		&structs.ACLPolicy{
			ID:          testPolicyID_A,
			Name:        "node-read",
			Description: "Allows reading all node information",
			Rules:       `node_prefix "" { policy = "read" }`,
			Syntax:      acl.SyntaxCurrent,
		},
		&structs.ACLPolicy{
			ID:          testPolicyID_B,
			Name:        "agent-read",
			Description: "Allows reading all node information",
			Rules:       `agent_prefix "" { policy = "read" }`,
			Syntax:      acl.SyntaxCurrent,
		},
		&structs.ACLPolicy{
			ID:          testPolicyID_C,
			Name:        "acl-read",
			Description: "Allows acl read",
			Rules:       `acl = "read"`,
			Syntax:      acl.SyntaxCurrent,
		},
		&structs.ACLPolicy{
			ID:          testPolicyID_D,
			Name:        "acl-write",
			Description: "Allows acl write",
			Rules:       `acl = "write"`,
			Syntax:      acl.SyntaxCurrent,
		},
		&structs.ACLPolicy{
			ID:          testPolicyID_E,
			Name:        "kv-read",
			Description: "Allows kv read",
			Rules:       `key_prefix "" { policy = "read" }`,
			Syntax:      acl.SyntaxCurrent,
		},
	}

	for _, policy := range policies {
		policy.SetHash(true)
	}

	require.NoError(t, s.ACLPolicyBatchSet(2, policies))
}

func setupExtraPoliciesAndRoles(t *testing.T, s *Store) {
	setupExtraPolicies(t, s)

	roles := structs.ACLRoles{
		&structs.ACLRole{
			ID:          testRoleID_A,
			Name:        "node-read-role",
			Description: "Allows reading all node information",
			Policies: []structs.ACLRolePolicyLink{
				{
					ID: testPolicyID_A,
				},
			},
		},
		&structs.ACLRole{
			ID:          testRoleID_B,
			Name:        "agent-read-role",
			Description: "Allows reading all agent information",
			Policies: []structs.ACLRolePolicyLink{
				{
					ID: testPolicyID_B,
				},
			},
		},
	}

	for _, role := range roles {
		role.SetHash(true)
	}

	require.NoError(t, s.ACLRoleBatchSet(3, roles, false))
}

func testACLTokensStateStore(t *testing.T) *Store {
	s := testACLStateStore(t)
	setupExtraPoliciesAndRoles(t, s)
	return s
}

func testACLRolesStateStore(t *testing.T) *Store {
	s := testACLStateStore(t)
	setupExtraPolicies(t, s)
	return s
}

func TestStateStore_ACLBootstrap(t *testing.T) {
	t.Parallel()
	token1 := &structs.ACLToken{
		AccessorID:  "30fca056-9fbb-4455-b94a-bf0e2bc575d6",
		SecretID:    "cbe1c6fd-d865-4034-9d6d-64fef7fb46a9",
		Description: "Bootstrap Token (Global Management)",
		Policies: []structs.ACLTokenPolicyLink{
			{
				ID: structs.ACLPolicyGlobalManagementID,
			},
		},
		CreateTime: time.Now(),
		Local:      false,
	}

	token2 := &structs.ACLToken{
		AccessorID:  "fd5c17fa-1503-4422-a424-dd44cdf35919",
		SecretID:    "7fd776b1-ded1-4d15-931b-db4770fc2317",
		Description: "Bootstrap Token (Global Management)",
		Policies: []structs.ACLTokenPolicyLink{
			{
				ID: structs.ACLPolicyGlobalManagementID,
			},
		},
		CreateTime: time.Now(),
		Local:      false,
	}

	s := testStateStore(t)
	setupGlobalManagement(t, s)

	canBootstrap, index, err := s.CanBootstrapACLToken()
	require.NoError(t, err)
	require.True(t, canBootstrap)
	require.Equal(t, uint64(0), index)

	// Perform a regular bootstrap.
	require.NoError(t, s.ACLBootstrap(3, 0, token1.Clone()))

	// Make sure we can't bootstrap again
	canBootstrap, index, err = s.CanBootstrapACLToken()
	require.NoError(t, err)
	require.False(t, canBootstrap)
	require.Equal(t, uint64(3), index)

	// Make sure another attempt fails.
	err = s.ACLBootstrap(4, 0, token2.Clone())
	require.Error(t, err)
	require.Equal(t, structs.ACLBootstrapNotAllowedErr, err)

	// Check that the bootstrap state remains the same.
	canBootstrap, index, err = s.CanBootstrapACLToken()
	require.NoError(t, err)
	require.False(t, canBootstrap)
	require.Equal(t, uint64(3), index)

	// Make sure the ACLs are in an expected state.
	_, tokens, err := s.ACLTokenList(nil, true, true, "", "", "", nil, nil)
	require.NoError(t, err)
	require.Len(t, tokens, 1)
	compareTokens(t, token1, tokens[0])

	// bootstrap reset
	err = s.ACLBootstrap(32, index-1, token2.Clone())
	require.Error(t, err)
	require.Equal(t, structs.ACLBootstrapInvalidResetIndexErr, err)

	// bootstrap reset
	err = s.ACLBootstrap(32, index, token2.Clone())
	require.NoError(t, err)

	_, tokens, err = s.ACLTokenList(nil, true, true, "", "", "", nil, nil)
	require.NoError(t, err)
	require.Len(t, tokens, 2)
}

func TestStateStore_ACLToken_SetGet(t *testing.T) {
	t.Parallel()
	t.Run("Missing Secret", func(t *testing.T) {
		t.Parallel()
		s := testACLTokensStateStore(t)
		token := &structs.ACLToken{
			AccessorID: "39171632-6f34-4411-827f-9416403687f4",
		}

		err := s.ACLTokenSet(2, token.Clone())
		require.Error(t, err)
		require.Equal(t, ErrMissingACLTokenSecret, err)
	})

	t.Run("Missing Accessor", func(t *testing.T) {
		t.Parallel()
		s := testACLTokensStateStore(t)
		token := &structs.ACLToken{
			SecretID: "39171632-6f34-4411-827f-9416403687f4",
		}

		err := s.ACLTokenSet(2, token.Clone())
		require.Error(t, err)
		require.Equal(t, ErrMissingACLTokenAccessor, err)
	})

	t.Run("Missing Service Identity Fields", func(t *testing.T) {
		t.Parallel()
		s := testACLTokensStateStore(t)
		token := &structs.ACLToken{
			AccessorID: "daf37c07-d04d-4fd5-9678-a8206a57d61a",
			SecretID:   "39171632-6f34-4411-827f-9416403687f4",
			ServiceIdentities: []*structs.ACLServiceIdentity{
				{},
			},
		}

		err := s.ACLTokenSet(2, token)
		require.Error(t, err)
	})

	t.Run("Missing Service Identity Name", func(t *testing.T) {
		t.Parallel()
		s := testACLTokensStateStore(t)
		token := &structs.ACLToken{
			AccessorID: "daf37c07-d04d-4fd5-9678-a8206a57d61a",
			SecretID:   "39171632-6f34-4411-827f-9416403687f4",
			ServiceIdentities: []*structs.ACLServiceIdentity{
				{
					Datacenters: []string{"dc1"},
				},
			},
		}

		err := s.ACLTokenSet(2, token)
		require.Error(t, err)
	})

	t.Run("Missing Policy ID", func(t *testing.T) {
		t.Parallel()
		s := testACLTokensStateStore(t)
		token := &structs.ACLToken{
			AccessorID: "daf37c07-d04d-4fd5-9678-a8206a57d61a",
			SecretID:   "39171632-6f34-4411-827f-9416403687f4",
			Policies: []structs.ACLTokenPolicyLink{
				{
					Name: "no-id",
				},
			},
		}

		err := s.ACLTokenSet(2, token.Clone())
		require.Error(t, err)
	})

	t.Run("Missing Role ID", func(t *testing.T) {
		t.Parallel()
		s := testACLTokensStateStore(t)
		token := &structs.ACLToken{
			AccessorID: "daf37c07-d04d-4fd5-9678-a8206a57d61a",
			SecretID:   "39171632-6f34-4411-827f-9416403687f4",
			Roles: []structs.ACLTokenRoleLink{
				{
					Name: "no-id",
				},
			},
		}

		err := s.ACLTokenSet(2, token)
		require.Error(t, err)
	})

	t.Run("Unresolvable Policy ID", func(t *testing.T) {
		t.Parallel()
		s := testACLTokensStateStore(t)
		token := &structs.ACLToken{
			AccessorID: "daf37c07-d04d-4fd5-9678-a8206a57d61a",
			SecretID:   "39171632-6f34-4411-827f-9416403687f4",
			Policies: []structs.ACLTokenPolicyLink{
				{
					ID: "4f20e379-b496-4b99-9599-19a197126490",
				},
			},
		}

		err := s.ACLTokenSet(2, token.Clone())
		require.Error(t, err)
	})

	t.Run("Unresolvable Role ID", func(t *testing.T) {
		t.Parallel()
		s := testACLTokensStateStore(t)
		token := &structs.ACLToken{
			AccessorID: "daf37c07-d04d-4fd5-9678-a8206a57d61a",
			SecretID:   "39171632-6f34-4411-827f-9416403687f4",
			Roles: []structs.ACLTokenRoleLink{
				{
					ID: "9b2349b6-55d3-4901-b287-347ae725af2f",
				},
			},
		}

		err := s.ACLTokenSet(2, token)
		require.Error(t, err)
	})

	t.Run("Unresolvable AuthMethod", func(t *testing.T) {
		t.Parallel()
		s := testACLTokensStateStore(t)
		token := &structs.ACLToken{
			AccessorID: "daf37c07-d04d-4fd5-9678-a8206a57d61a",
			SecretID:   "39171632-6f34-4411-827f-9416403687f4",
			AuthMethod: "test",
		}

		err := s.ACLTokenSet(2, token)
		require.Error(t, err)
	})

	t.Run("New", func(t *testing.T) {
		t.Parallel()
		s := testACLTokensStateStore(t)
		token := &structs.ACLToken{
			AccessorID: "daf37c07-d04d-4fd5-9678-a8206a57d61a",
			SecretID:   "39171632-6f34-4411-827f-9416403687f4",
			Policies: []structs.ACLTokenPolicyLink{
				{
					ID: testPolicyID_A,
				},
			},
			Roles: []structs.ACLTokenRoleLink{
				{
					ID: testRoleID_A,
				},
			},
			ServiceIdentities: []*structs.ACLServiceIdentity{
				{
					ServiceName: "web",
				},
			},
		}

		require.NoError(t, s.ACLTokenSet(2, token.Clone()))

		idx, rtoken, err := s.ACLTokenGetByAccessor(nil, "daf37c07-d04d-4fd5-9678-a8206a57d61a", nil)
		require.NoError(t, err)
		require.Equal(t, uint64(2), idx)
		require.NotEmpty(t, rtoken.Hash)
		compareTokens(t, token, rtoken)
		require.Equal(t, uint64(2), rtoken.CreateIndex)
		require.Equal(t, uint64(2), rtoken.ModifyIndex)
		require.Len(t, rtoken.Policies, 1)
		require.Equal(t, "node-read", rtoken.Policies[0].Name)
		require.Len(t, rtoken.Roles, 1)
		require.Equal(t, "node-read-role", rtoken.Roles[0].Name)
		require.Len(t, rtoken.ServiceIdentities, 1)
		require.Equal(t, "web", rtoken.ServiceIdentities[0].ServiceName)
	})

	t.Run("Update", func(t *testing.T) {
		t.Parallel()
		s := testACLTokensStateStore(t)
		token := &structs.ACLToken{
			AccessorID: "daf37c07-d04d-4fd5-9678-a8206a57d61a",
			SecretID:   "39171632-6f34-4411-827f-9416403687f4",
			Policies: []structs.ACLTokenPolicyLink{
				{
					ID: testPolicyID_A,
				},
			},
			ServiceIdentities: []*structs.ACLServiceIdentity{
				{
					ServiceName: "web",
				},
			},
		}

		require.NoError(t, s.ACLTokenSet(2, token.Clone()))

		updated := &structs.ACLToken{
			AccessorID: "daf37c07-d04d-4fd5-9678-a8206a57d61a",
			SecretID:   "39171632-6f34-4411-827f-9416403687f4",
			Policies: []structs.ACLTokenPolicyLink{
				{
					ID: structs.ACLPolicyGlobalManagementID,
				},
			},
			Roles: []structs.ACLTokenRoleLink{
				{
					ID: testRoleID_A,
				},
			},
			ServiceIdentities: []*structs.ACLServiceIdentity{
				{
					ServiceName: "db",
				},
			},
		}

		require.NoError(t, s.ACLTokenSet(3, updated.Clone()))

		idx, rtoken, err := s.ACLTokenGetByAccessor(nil, "daf37c07-d04d-4fd5-9678-a8206a57d61a", nil)
		require.NoError(t, err)
		require.Equal(t, uint64(3), idx)
		compareTokens(t, updated, rtoken)
		require.Equal(t, uint64(2), rtoken.CreateIndex)
		require.Equal(t, uint64(3), rtoken.ModifyIndex)
		require.Len(t, rtoken.Policies, 1)
		require.Equal(t, structs.ACLPolicyGlobalManagementID, rtoken.Policies[0].ID)
		require.Equal(t, "global-management", rtoken.Policies[0].Name)
		require.Len(t, rtoken.Roles, 1)
		require.Equal(t, testRoleID_A, rtoken.Roles[0].ID)
		require.Equal(t, "node-read-role", rtoken.Roles[0].Name)
		require.Len(t, rtoken.ServiceIdentities, 1)
		require.Equal(t, "db", rtoken.ServiceIdentities[0].ServiceName)
	})

	t.Run("New with auth method", func(t *testing.T) {
		t.Parallel()
		s := testACLTokensStateStore(t)
		setupExtraAuthMethods(t, s)

		token := &structs.ACLToken{
			AccessorID: "daf37c07-d04d-4fd5-9678-a8206a57d61a",
			SecretID:   "39171632-6f34-4411-827f-9416403687f4",
			AuthMethod: "test",
			Roles: []structs.ACLTokenRoleLink{
				{
					ID: testRoleID_A,
				},
			},
		}

		require.NoError(t, s.ACLTokenSet(2, token.Clone()))

		idx, rtoken, err := s.ACLTokenGetByAccessor(nil, "daf37c07-d04d-4fd5-9678-a8206a57d61a", nil)
		require.NoError(t, err)
		require.Equal(t, uint64(2), idx)
		compareTokens(t, token, rtoken)
		require.Equal(t, uint64(2), rtoken.CreateIndex)
		require.Equal(t, uint64(2), rtoken.ModifyIndex)
		require.Equal(t, "test", rtoken.AuthMethod)
		require.Len(t, rtoken.Policies, 0)
		require.Len(t, rtoken.ServiceIdentities, 0)
		require.Len(t, rtoken.Roles, 1)
		require.Equal(t, "node-read-role", rtoken.Roles[0].Name)
	})
}

func TestStateStore_ACLTokens_UpsertBatchRead(t *testing.T) {
	t.Parallel()

	t.Run("CAS - Deleted", func(t *testing.T) {
		t.Parallel()
		s := testACLTokensStateStore(t)

		// CAS op + nonexistent token should not work. This prevents modifying
		// deleted tokens

		tokens := structs.ACLTokens{
			&structs.ACLToken{
				AccessorID: "a4f68bd6-3af5-4f56-b764-3c6f20247879",
				SecretID:   "00ff4564-dd96-4d1b-8ad6-578a08279f79",
				RaftIndex:  structs.RaftIndex{CreateIndex: 2, ModifyIndex: 3},
			},
		}

		require.NoError(t, s.ACLTokenBatchSet(2, tokens, ACLTokenSetOptions{CAS: true}))

		_, token, err := s.ACLTokenGetByAccessor(nil, tokens[0].AccessorID, nil)
		require.NoError(t, err)
		require.Nil(t, token)
	})

	t.Run("CAS - Updated", func(t *testing.T) {
		t.Parallel()
		s := testACLTokensStateStore(t)

		// CAS op + nonexistent token should not work. This prevents modifying
		// deleted tokens

		tokens := structs.ACLTokens{
			&structs.ACLToken{
				AccessorID: "a4f68bd6-3af5-4f56-b764-3c6f20247879",
				SecretID:   "00ff4564-dd96-4d1b-8ad6-578a08279f79",
			},
		}

		require.NoError(t, s.ACLTokenBatchSet(5, tokens, ACLTokenSetOptions{CAS: true}))

		updated := structs.ACLTokens{
			&structs.ACLToken{
				AccessorID:  "a4f68bd6-3af5-4f56-b764-3c6f20247879",
				SecretID:    "00ff4564-dd96-4d1b-8ad6-578a08279f79",
				Description: "wont update",
				RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 4},
			},
		}

		require.NoError(t, s.ACLTokenBatchSet(6, updated, ACLTokenSetOptions{CAS: true}))

		_, token, err := s.ACLTokenGetByAccessor(nil, tokens[0].AccessorID, nil)
		require.NoError(t, err)
		require.NotNil(t, token)
		require.Equal(t, "", token.Description)
	})

	t.Run("CAS - Already Exists", func(t *testing.T) {
		t.Parallel()
		s := testACLTokensStateStore(t)

		tokens := structs.ACLTokens{
			&structs.ACLToken{
				AccessorID: "a4f68bd6-3af5-4f56-b764-3c6f20247879",
				SecretID:   "00ff4564-dd96-4d1b-8ad6-578a08279f79",
			},
		}

		require.NoError(t, s.ACLTokenBatchSet(5, tokens, ACLTokenSetOptions{CAS: true}))

		updated := structs.ACLTokens{
			&structs.ACLToken{
				AccessorID:  "a4f68bd6-3af5-4f56-b764-3c6f20247879",
				SecretID:    "00ff4564-dd96-4d1b-8ad6-578a08279f79",
				Description: "wont update",
			},
		}

		require.NoError(t, s.ACLTokenBatchSet(6, updated, ACLTokenSetOptions{CAS: true}))

		_, token, err := s.ACLTokenGetByAccessor(nil, tokens[0].AccessorID, nil)
		require.NoError(t, err)
		require.NotNil(t, token)
		require.Equal(t, "", token.Description)
	})

	t.Run("Normal", func(t *testing.T) {
		t.Parallel()
		s := testACLTokensStateStore(t)

		tokens := structs.ACLTokens{
			&structs.ACLToken{
				AccessorID: "a4f68bd6-3af5-4f56-b764-3c6f20247879",
				SecretID:   "00ff4564-dd96-4d1b-8ad6-578a08279f79",
			},
			&structs.ACLToken{
				AccessorID: "a2719052-40b3-4a4b-baeb-f3df1831a217",
				SecretID:   "ff826eaf-4b88-4881-aaef-52b1089e5d5d",
			},
		}

		require.NoError(t, s.ACLTokenBatchSet(2, tokens, ACLTokenSetOptions{}))

		idx, rtokens, err := s.ACLTokenBatchGet(nil, []string{
			"a4f68bd6-3af5-4f56-b764-3c6f20247879",
			"a2719052-40b3-4a4b-baeb-f3df1831a217"})

		require.NoError(t, err)
		require.Equal(t, uint64(2), idx)
		require.Len(t, rtokens, 2)
		require.ElementsMatch(t, tokens, rtokens)
		require.Equal(t, uint64(2), rtokens[0].CreateIndex)
		require.Equal(t, uint64(2), rtokens[0].ModifyIndex)
		require.Equal(t, uint64(2), rtokens[1].CreateIndex)
		require.Equal(t, uint64(2), rtokens[1].ModifyIndex)
	})

	t.Run("Update", func(t *testing.T) {
		t.Parallel()
		s := testACLTokensStateStore(t)

		tokens := structs.ACLTokens{
			&structs.ACLToken{
				AccessorID: "a4f68bd6-3af5-4f56-b764-3c6f20247879",
				SecretID:   "00ff4564-dd96-4d1b-8ad6-578a08279f79",
			},
			&structs.ACLToken{
				AccessorID: "a2719052-40b3-4a4b-baeb-f3df1831a217",
				SecretID:   "ff826eaf-4b88-4881-aaef-52b1089e5d5d",
			},
		}

		require.NoError(t, s.ACLTokenBatchSet(2, tokens, ACLTokenSetOptions{}))

		updates := structs.ACLTokens{
			&structs.ACLToken{
				AccessorID:  "a4f68bd6-3af5-4f56-b764-3c6f20247879",
				SecretID:    "00ff4564-dd96-4d1b-8ad6-578a08279f79",
				Description: "first token",
				Policies: []structs.ACLTokenPolicyLink{
					{
						ID: testPolicyID_A,
					},
				},
			},
			&structs.ACLToken{
				AccessorID:  "a2719052-40b3-4a4b-baeb-f3df1831a217",
				SecretID:    "ff826eaf-4b88-4881-aaef-52b1089e5d5d",
				Description: "second token",
				Policies: []structs.ACLTokenPolicyLink{
					{
						ID: structs.ACLPolicyGlobalManagementID,
					},
				},
			},
		}

		require.NoError(t, s.ACLTokenBatchSet(3, updates, ACLTokenSetOptions{}))

		idx, rtokens, err := s.ACLTokenBatchGet(nil, []string{
			"a4f68bd6-3af5-4f56-b764-3c6f20247879",
			"a2719052-40b3-4a4b-baeb-f3df1831a217"})

		require.NoError(t, err)
		require.Equal(t, uint64(3), idx)
		require.Len(t, rtokens, 2)
		rtokens.Sort()
		require.Equal(t, "a2719052-40b3-4a4b-baeb-f3df1831a217", rtokens[0].AccessorID)
		require.Equal(t, "ff826eaf-4b88-4881-aaef-52b1089e5d5d", rtokens[0].SecretID)
		require.Equal(t, "second token", rtokens[0].Description)
		require.Len(t, rtokens[0].Policies, 1)
		require.Equal(t, structs.ACLPolicyGlobalManagementID, rtokens[0].Policies[0].ID)
		require.Equal(t, "global-management", rtokens[0].Policies[0].Name)
		require.Equal(t, uint64(2), rtokens[0].CreateIndex)
		require.Equal(t, uint64(3), rtokens[0].ModifyIndex)

		require.Equal(t, "a4f68bd6-3af5-4f56-b764-3c6f20247879", rtokens[1].AccessorID)
		require.Equal(t, "00ff4564-dd96-4d1b-8ad6-578a08279f79", rtokens[1].SecretID)
		require.Equal(t, "first token", rtokens[1].Description)
		require.Len(t, rtokens[1].Policies, 1)
		require.Equal(t, testPolicyID_A, rtokens[1].Policies[0].ID)
		require.Equal(t, "node-read", rtokens[1].Policies[0].Name)
		require.Equal(t, uint64(2), rtokens[1].CreateIndex)
		require.Equal(t, uint64(3), rtokens[1].ModifyIndex)
	})

	t.Run("AllowMissing - Policy", func(t *testing.T) {
		t.Parallel()
		s := testACLTokensStateStore(t)

		const fakePolicyID = "0ea7b58a-3d86-4e82-b656-577b63d727f3"

		tokens := structs.ACLTokens{
			&structs.ACLToken{
				AccessorID: "a4f68bd6-3af5-4f56-b764-3c6f20247879",
				SecretID:   "00ff4564-dd96-4d1b-8ad6-578a08279f79",
				Policies: []structs.ACLTokenPolicyLink{
					{
						ID: fakePolicyID,
					},
				},
			},
		}

		require.Error(t, s.ACLTokenBatchSet(2, tokens, ACLTokenSetOptions{}))

		require.NoError(t, s.ACLTokenBatchSet(2, tokens, ACLTokenSetOptions{AllowMissingPolicyAndRoleIDs: true}))

		idx, rtokens, err := s.ACLTokenBatchGet(nil, []string{
			"a4f68bd6-3af5-4f56-b764-3c6f20247879",
		})

		require.NoError(t, err)
		require.Equal(t, uint64(2), idx)
		require.Len(t, rtokens, 1)

		// Persisting invalid IDs will cause them to be masked during read. So
		// before we compare structures strike the dead entries.
		tokens[0].Policies = []structs.ACLTokenPolicyLink{}

		require.Equal(t, tokens[0], rtokens[0])
		require.Equal(t, uint64(2), rtokens[0].CreateIndex)
		require.Equal(t, uint64(2), rtokens[0].ModifyIndex)
	})

	t.Run("AllowMissing - Role", func(t *testing.T) {
		t.Parallel()
		s := testACLTokensStateStore(t)

		const fakeRoleID = "fbd9776e-4403-47a1-8ff1-8d24179ec307"

		tokens := structs.ACLTokens{
			&structs.ACLToken{
				AccessorID: "a4f68bd6-3af5-4f56-b764-3c6f20247879",
				SecretID:   "00ff4564-dd96-4d1b-8ad6-578a08279f79",
				Roles: []structs.ACLTokenRoleLink{
					{
						ID: fakeRoleID,
					},
				},
			},
		}

		require.Error(t, s.ACLTokenBatchSet(2, tokens, ACLTokenSetOptions{}))

		require.NoError(t, s.ACLTokenBatchSet(2, tokens, ACLTokenSetOptions{AllowMissingPolicyAndRoleIDs: true}))

		idx, rtokens, err := s.ACLTokenBatchGet(nil, []string{
			"a4f68bd6-3af5-4f56-b764-3c6f20247879",
		})

		// Persisting invalid IDs will cause them to be masked during read. So
		// before we compare structures strike the dead entries.
		tokens[0].Roles = []structs.ACLTokenRoleLink{}

		require.NoError(t, err)
		require.Equal(t, uint64(2), idx)
		require.Len(t, rtokens, 1)
		require.Equal(t, tokens[0], rtokens[0])
		require.Equal(t, uint64(2), rtokens[0].CreateIndex)
		require.Equal(t, uint64(2), rtokens[0].ModifyIndex)
	})
}

func TestStateStore_ACLTokens_ListUpgradeable(t *testing.T) {
	t.Parallel()
	s := testACLTokensStateStore(t)

	aclTokenSetLegacy := func(idx uint64, token *structs.ACLToken) error {
		tx := s.db.WriteTxn(idx)
		defer tx.Abort()

		opts := ACLTokenSetOptions{Legacy: true}
		if err := aclTokenSetTxn(tx, idx, token, opts); err != nil {
			return err
		}

		return tx.Commit()
	}

	const ACLTokenTypeManagement = "management"

	require.NoError(t, aclTokenSetLegacy(2, &structs.ACLToken{
		SecretID: "34ec8eb3-095d-417a-a937-b439af7a8e8b",
		Type:     ACLTokenTypeManagement,
	}))

	require.NoError(t, aclTokenSetLegacy(3, &structs.ACLToken{
		SecretID: "8de2dd39-134d-4cb1-950b-b7ab96ea20ba",
		Type:     ACLTokenTypeManagement,
	}))

	require.NoError(t, aclTokenSetLegacy(4, &structs.ACLToken{
		SecretID: "548bdb8e-c0d6-477b-bcc4-67fb836e9e61",
		Type:     ACLTokenTypeManagement,
	}))

	require.NoError(t, aclTokenSetLegacy(5, &structs.ACLToken{
		SecretID: "3ee33676-d9b8-4144-bf0b-92618cff438b",
		Type:     ACLTokenTypeManagement,
	}))

	require.NoError(t, aclTokenSetLegacy(6, &structs.ACLToken{
		SecretID: "fa9d658a-6e26-42ab-a5f0-1ea05c893dee",
		Type:     ACLTokenTypeManagement,
	}))

	tokens, _, err := s.ACLTokenListUpgradeable(3)
	require.NoError(t, err)
	require.Len(t, tokens, 3)

	tokens, _, err = s.ACLTokenListUpgradeable(10)
	require.NoError(t, err)
	require.Len(t, tokens, 5)

	updates := structs.ACLTokens{
		&structs.ACLToken{
			AccessorID: "f1093997-b6c7-496d-bfb8-6b1b1895641b",
			SecretID:   "34ec8eb3-095d-417a-a937-b439af7a8e8b",
			Policies: []structs.ACLTokenPolicyLink{
				{
					ID: structs.ACLPolicyGlobalManagementID,
				},
			},
		},
		&structs.ACLToken{
			AccessorID: "54866514-3cf2-4fec-8a8a-710583831834",
			SecretID:   "8de2dd39-134d-4cb1-950b-b7ab96ea20ba",
			Policies: []structs.ACLTokenPolicyLink{
				{
					ID: structs.ACLPolicyGlobalManagementID,
				},
			},
		},
		&structs.ACLToken{
			AccessorID: "47eea4da-bda1-48a6-901c-3e36d2d9262f",
			SecretID:   "548bdb8e-c0d6-477b-bcc4-67fb836e9e61",
			Policies: []structs.ACLTokenPolicyLink{
				{
					ID: structs.ACLPolicyGlobalManagementID,
				},
			},
		},
		&structs.ACLToken{
			AccessorID: "af1dffe5-8ac2-4282-9336-aeed9f7d951a",
			SecretID:   "3ee33676-d9b8-4144-bf0b-92618cff438b",
			Policies: []structs.ACLTokenPolicyLink{
				{
					ID: structs.ACLPolicyGlobalManagementID,
				},
			},
		},
		&structs.ACLToken{
			AccessorID: "511df589-3316-4784-b503-6e25ead4d4e1",
			SecretID:   "fa9d658a-6e26-42ab-a5f0-1ea05c893dee",
			Policies: []structs.ACLTokenPolicyLink{
				{
					ID: structs.ACLPolicyGlobalManagementID,
				},
			},
		},
	}

	require.NoError(t, s.ACLTokenBatchSet(7, updates, ACLTokenSetOptions{}))

	tokens, _, err = s.ACLTokenListUpgradeable(10)
	require.NoError(t, err)
	require.Len(t, tokens, 0)
}

func TestStateStore_ACLToken_List(t *testing.T) {
	t.Parallel()
	s := testACLTokensStateStore(t)
	setupExtraAuthMethods(t, s)

	tokens := structs.ACLTokens{
		// the local token
		&structs.ACLToken{
			AccessorID: "f1093997-b6c7-496d-bfb8-6b1b1895641b",
			SecretID:   "34ec8eb3-095d-417a-a937-b439af7a8e8b",
			Policies: []structs.ACLTokenPolicyLink{
				{
					ID: structs.ACLPolicyGlobalManagementID,
				},
			},
			Local: true,
		},
		// the global token
		&structs.ACLToken{
			AccessorID: "54866514-3cf2-4fec-8a8a-710583831834",
			SecretID:   "8de2dd39-134d-4cb1-950b-b7ab96ea20ba",
			Policies: []structs.ACLTokenPolicyLink{
				{
					ID: structs.ACLPolicyGlobalManagementID,
				},
			},
		},
		// the policy specific token
		&structs.ACLToken{
			AccessorID: "47eea4da-bda1-48a6-901c-3e36d2d9262f",
			SecretID:   "548bdb8e-c0d6-477b-bcc4-67fb836e9e61",
			Policies: []structs.ACLTokenPolicyLink{
				{
					ID: testPolicyID_A,
				},
			},
		},
		// the policy specific token and local
		&structs.ACLToken{
			AccessorID: "4915fc9d-3726-4171-b588-6c271f45eecd",
			SecretID:   "f6998577-fd9b-4e6c-b202-cc3820513d32",
			Policies: []structs.ACLTokenPolicyLink{
				{
					ID: testPolicyID_A,
				},
			},
			Local: true,
		},
		// the role specific token
		&structs.ACLToken{
			AccessorID: "a7715fde-8954-4c92-afbc-d84c6ecdc582",
			SecretID:   "77a2da3a-b479-4025-a83e-bd6b859f0cfe",
			Roles: []structs.ACLTokenRoleLink{
				{
					ID: testRoleID_A,
				},
			},
		},
		// the role specific token and local
		&structs.ACLToken{
			AccessorID: "cadb4f13-f62a-49ab-ab3f-5a7e01b925d9",
			SecretID:   "c432d12b-3c86-4628-b74f-94ddfc7fb3ba",
			Roles: []structs.ACLTokenRoleLink{
				{
					ID: testRoleID_A,
				},
			},
			Local: true,
		},
		// the method specific token
		&structs.ACLToken{
			AccessorID: "74277ae1-6a9b-4035-b444-2370fe6a2cb5",
			SecretID:   "ab8ac834-0d35-4cb7-83c3-168203f986cd",
			AuthMethod: "test",
		},
		// the method specific token and local
		&structs.ACLToken{
			AccessorID: "211f0360-ef53-41d3-9d4d-db84396eb6c0",
			SecretID:   "087a0eb4-366f-4190-ab4c-a4aa3d2562aa",
			AuthMethod: "test",
			Local:      true,
		},
	}

	require.NoError(t, s.ACLTokenBatchSet(2, tokens, ACLTokenSetOptions{}))

	type testCase struct {
		name       string
		local      bool
		global     bool
		policy     string
		role       string
		methodName string
		accessors  []string
	}

	cases := []testCase{
		{
			name:       "Global",
			local:      false,
			global:     true,
			policy:     "",
			role:       "",
			methodName: "",
			accessors: []string{
				structs.ACLTokenAnonymousID,
				"47eea4da-bda1-48a6-901c-3e36d2d9262f", // policy + global
				"54866514-3cf2-4fec-8a8a-710583831834", // mgmt + global
				"74277ae1-6a9b-4035-b444-2370fe6a2cb5", // authMethod + global
				"a7715fde-8954-4c92-afbc-d84c6ecdc582", // role + global
			},
		},
		{
			name:       "Local",
			local:      true,
			global:     false,
			policy:     "",
			role:       "",
			methodName: "",
			accessors: []string{
				"211f0360-ef53-41d3-9d4d-db84396eb6c0", // authMethod + local
				"4915fc9d-3726-4171-b588-6c271f45eecd", // policy + local
				"cadb4f13-f62a-49ab-ab3f-5a7e01b925d9", // role + local
				"f1093997-b6c7-496d-bfb8-6b1b1895641b", // mgmt + local
			},
		},
		{
			name:       "Policy",
			local:      true,
			global:     true,
			policy:     testPolicyID_A,
			role:       "",
			methodName: "",
			accessors: []string{
				"47eea4da-bda1-48a6-901c-3e36d2d9262f", // policy + global
				"4915fc9d-3726-4171-b588-6c271f45eecd", // policy + local
			},
		},
		{
			name:       "Policy - Local",
			local:      true,
			global:     false,
			policy:     testPolicyID_A,
			role:       "",
			methodName: "",
			accessors: []string{
				"4915fc9d-3726-4171-b588-6c271f45eecd", // policy + local
			},
		},
		{
			name:       "Policy - Global",
			local:      false,
			global:     true,
			policy:     testPolicyID_A,
			role:       "",
			methodName: "",
			accessors: []string{
				"47eea4da-bda1-48a6-901c-3e36d2d9262f", // policy + global
			},
		},
		{
			name:       "Role",
			local:      true,
			global:     true,
			policy:     "",
			role:       testRoleID_A,
			methodName: "",
			accessors: []string{
				"a7715fde-8954-4c92-afbc-d84c6ecdc582", // role + global
				"cadb4f13-f62a-49ab-ab3f-5a7e01b925d9", // role + local
			},
		},
		{
			name:       "Role - Local",
			local:      true,
			global:     false,
			policy:     "",
			role:       testRoleID_A,
			methodName: "",
			accessors: []string{
				"cadb4f13-f62a-49ab-ab3f-5a7e01b925d9", // role + local
			},
		},
		{
			name:       "Role - Global",
			local:      false,
			global:     true,
			policy:     "",
			role:       testRoleID_A,
			methodName: "",
			accessors: []string{
				"a7715fde-8954-4c92-afbc-d84c6ecdc582", // role + global
			},
		},
		{
			name:       "AuthMethod - Local",
			local:      true,
			global:     false,
			policy:     "",
			role:       "",
			methodName: "test",
			accessors: []string{
				"211f0360-ef53-41d3-9d4d-db84396eb6c0", // authMethod + local
			},
		},
		{
			name:       "AuthMethod - Global",
			local:      false,
			global:     true,
			policy:     "",
			role:       "",
			methodName: "test",
			accessors: []string{
				"74277ae1-6a9b-4035-b444-2370fe6a2cb5", // authMethod + global
			},
		},
		{
			name:       "All",
			local:      true,
			global:     true,
			policy:     "",
			role:       "",
			methodName: "",
			accessors: []string{
				structs.ACLTokenAnonymousID,
				"211f0360-ef53-41d3-9d4d-db84396eb6c0", // authMethod + local
				"47eea4da-bda1-48a6-901c-3e36d2d9262f", // policy + global
				"4915fc9d-3726-4171-b588-6c271f45eecd", // policy + local
				"54866514-3cf2-4fec-8a8a-710583831834", // mgmt + global
				"74277ae1-6a9b-4035-b444-2370fe6a2cb5", // authMethod + global
				"a7715fde-8954-4c92-afbc-d84c6ecdc582", // role + global
				"cadb4f13-f62a-49ab-ab3f-5a7e01b925d9", // role + local
				"f1093997-b6c7-496d-bfb8-6b1b1895641b", // mgmt + local
			},
		},
	}

	for _, tc := range []struct{ policy, role, methodName string }{
		{testPolicyID_A, testRoleID_A, "test"},
		{"", testRoleID_A, "test"},
		{testPolicyID_A, "", "test"},
		{testPolicyID_A, testRoleID_A, ""},
	} {
		t.Run(fmt.Sprintf("can't filter on more than one: %s/%s/%s", tc.policy, tc.role, tc.methodName), func(t *testing.T) {
			_, _, err := s.ACLTokenList(nil, false, false, tc.policy, tc.role, tc.methodName, nil, nil)
			require.Error(t, err)
		})
	}

	for _, tc := range cases {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, tokens, err := s.ACLTokenList(nil, tc.local, tc.global, tc.policy, tc.role, tc.methodName, nil, nil)
			require.NoError(t, err)
			require.Len(t, tokens, len(tc.accessors))
			tokens.Sort()
			for i, token := range tokens {
				require.Equal(t, tc.accessors[i], token.AccessorID)
			}
		})
	}
}

func TestStateStore_ACLToken_FixupPolicyLinks(t *testing.T) {
	// This test wants to ensure a couple of things.
	//
	// 1. Doing a token list/get should never modify the data
	//    tracked by memdb
	// 2. Token list/get operations should return an accurate set
	//    of policy links
	t.Parallel()
	s := testACLTokensStateStore(t)

	// the policy specific token
	token := &structs.ACLToken{
		AccessorID: "47eea4da-bda1-48a6-901c-3e36d2d9262f",
		SecretID:   "548bdb8e-c0d6-477b-bcc4-67fb836e9e61",
		Policies: []structs.ACLTokenPolicyLink{
			{
				ID: testPolicyID_A,
			},
		},
	}

	require.NoError(t, s.ACLTokenSet(2, token))

	_, retrieved, err := s.ACLTokenGetByAccessor(nil, token.AccessorID, nil)
	require.NoError(t, err)
	// pointer equality check these should be identical
	require.True(t, token == retrieved)
	require.Len(t, retrieved.Policies, 1)
	require.Equal(t, "node-read", retrieved.Policies[0].Name)

	// rename the policy
	renamed := &structs.ACLPolicy{
		ID:          testPolicyID_A,
		Name:        "node-read-renamed",
		Description: "Allows reading all node information",
		Rules:       `node_prefix "" { policy = "read" }`,
		Syntax:      acl.SyntaxCurrent,
	}
	renamed.SetHash(true)
	require.NoError(t, s.ACLPolicySet(3, renamed))

	// retrieve the token again
	_, retrieved, err = s.ACLTokenGetByAccessor(nil, token.AccessorID, nil)
	require.NoError(t, err)
	// pointer equality check these should be different if we cloned things appropriately
	require.True(t, token != retrieved)
	require.Len(t, retrieved.Policies, 1)
	require.Equal(t, "node-read-renamed", retrieved.Policies[0].Name)

	// list tokens without stale links
	_, tokens, err := s.ACLTokenList(nil, true, true, "", "", "", nil, nil)
	require.NoError(t, err)

	found := false
	for _, tok := range tokens {
		if tok.AccessorID == token.AccessorID {
			// these pointers shouldn't be equal because the link should have been fixed
			require.True(t, tok != token)
			require.Len(t, tok.Policies, 1)
			require.Equal(t, "node-read-renamed", tok.Policies[0].Name)
			found = true
			break
		}
	}
	require.True(t, found)

	// batch get without stale links
	_, tokens, err = s.ACLTokenBatchGet(nil, []string{token.AccessorID})
	require.NoError(t, err)

	found = false
	for _, tok := range tokens {
		if tok.AccessorID == token.AccessorID {
			// these pointers shouldn't be equal because the link should have been fixed
			require.True(t, tok != token)
			require.Len(t, tok.Policies, 1)
			require.Equal(t, "node-read-renamed", tok.Policies[0].Name)
			found = true
			break
		}
	}
	require.True(t, found)

	// delete the policy
	require.NoError(t, s.ACLPolicyDeleteByID(4, testPolicyID_A, nil))

	// retrieve the token again
	_, retrieved, err = s.ACLTokenGetByAccessor(nil, token.AccessorID, nil)
	require.NoError(t, err)
	// pointer equality check these should be different if we cloned things appropriately
	require.True(t, token != retrieved)
	require.Len(t, retrieved.Policies, 0)

	// list tokens without stale links
	_, tokens, err = s.ACLTokenList(nil, true, true, "", "", "", nil, nil)
	require.NoError(t, err)

	found = false
	for _, tok := range tokens {
		if tok.AccessorID == token.AccessorID {
			// these pointers shouldn't be equal because the link should have been fixed
			require.True(t, tok != token)
			require.Len(t, tok.Policies, 0)
			found = true
			break
		}
	}
	require.True(t, found)

	// batch get without stale links
	_, tokens, err = s.ACLTokenBatchGet(nil, []string{token.AccessorID})
	require.NoError(t, err)

	found = false
	for _, tok := range tokens {
		if tok.AccessorID == token.AccessorID {
			// these pointers shouldn't be equal because the link should have been fixed
			require.True(t, tok != token)
			require.Len(t, tok.Policies, 0)
			found = true
			break
		}
	}
	require.True(t, found)
}

func TestStateStore_ACLToken_FixupRoleLinks(t *testing.T) {
	// This test wants to ensure a couple of things.
	//
	// 1. Doing a token list/get should never modify the data
	//    tracked by memdb
	// 2. Token list/get operations should return an accurate set
	//    of role links
	t.Parallel()
	s := testACLTokensStateStore(t)

	// the role specific token
	token := &structs.ACLToken{
		AccessorID: "47eea4da-bda1-48a6-901c-3e36d2d9262f",
		SecretID:   "548bdb8e-c0d6-477b-bcc4-67fb836e9e61",
		Roles: []structs.ACLTokenRoleLink{
			{
				ID: testRoleID_A,
			},
		},
	}

	require.NoError(t, s.ACLTokenSet(2, token))

	_, retrieved, err := s.ACLTokenGetByAccessor(nil, token.AccessorID, nil)
	require.NoError(t, err)
	// pointer equality check these should be identical
	require.True(t, token == retrieved)
	require.Len(t, retrieved.Roles, 1)
	require.Equal(t, "node-read-role", retrieved.Roles[0].Name)

	// rename the role
	renamed := &structs.ACLRole{
		ID:          testRoleID_A,
		Name:        "node-read-role-renamed",
		Description: "Allows reading all node information",
		Policies: []structs.ACLRolePolicyLink{
			{
				ID: testPolicyID_A,
			},
		},
	}
	renamed.SetHash(true)
	require.NoError(t, s.ACLRoleSet(3, renamed))

	// retrieve the token again
	_, retrieved, err = s.ACLTokenGetByAccessor(nil, token.AccessorID, nil)
	require.NoError(t, err)
	// pointer equality check these should be different if we cloned things appropriately
	require.True(t, token != retrieved)
	require.Len(t, retrieved.Roles, 1)
	require.Equal(t, "node-read-role-renamed", retrieved.Roles[0].Name)

	// list tokens without stale links
	_, tokens, err := s.ACLTokenList(nil, true, true, "", "", "", nil, nil)
	require.NoError(t, err)

	found := false
	for _, tok := range tokens {
		if tok.AccessorID == token.AccessorID {
			// these pointers shouldn't be equal because the link should have been fixed
			require.True(t, tok != token)
			require.Len(t, tok.Roles, 1)
			require.Equal(t, "node-read-role-renamed", tok.Roles[0].Name)
			found = true
			break
		}
	}
	require.True(t, found)

	// batch get without stale links
	_, tokens, err = s.ACLTokenBatchGet(nil, []string{token.AccessorID})
	require.NoError(t, err)

	found = false
	for _, tok := range tokens {
		if tok.AccessorID == token.AccessorID {
			// these pointers shouldn't be equal because the link should have been fixed
			require.True(t, tok != token)
			require.Len(t, tok.Roles, 1)
			require.Equal(t, "node-read-role-renamed", tok.Roles[0].Name)
			found = true
			break
		}
	}
	require.True(t, found)

	// delete the role
	require.NoError(t, s.ACLRoleDeleteByID(4, testRoleID_A, nil))

	// retrieve the token again
	_, retrieved, err = s.ACLTokenGetByAccessor(nil, token.AccessorID, nil)
	require.NoError(t, err)
	// pointer equality check these should be different if we cloned things appropriately
	require.True(t, token != retrieved)
	require.Len(t, retrieved.Roles, 0)

	// list tokens without stale links
	_, tokens, err = s.ACLTokenList(nil, true, true, "", "", "", nil, nil)
	require.NoError(t, err)

	found = false
	for _, tok := range tokens {
		if tok.AccessorID == token.AccessorID {
			// these pointers shouldn't be equal because the link should have been fixed
			require.True(t, tok != token)
			require.Len(t, tok.Roles, 0)
			found = true
			break
		}
	}
	require.True(t, found)

	// batch get without stale links
	_, tokens, err = s.ACLTokenBatchGet(nil, []string{token.AccessorID})
	require.NoError(t, err)

	found = false
	for _, tok := range tokens {
		if tok.AccessorID == token.AccessorID {
			// these pointers shouldn't be equal because the link should have been fixed
			require.True(t, tok != token)
			require.Len(t, tok.Roles, 0)
			found = true
			break
		}
	}
	require.True(t, found)
}

func TestStateStore_ACLToken_Delete(t *testing.T) {
	t.Parallel()

	t.Run("Accessor", func(t *testing.T) {
		t.Parallel()
		s := testACLTokensStateStore(t)

		token := &structs.ACLToken{
			AccessorID: "f1093997-b6c7-496d-bfb8-6b1b1895641b",
			SecretID:   "34ec8eb3-095d-417a-a937-b439af7a8e8b",
			Policies: []structs.ACLTokenPolicyLink{
				{
					ID: structs.ACLPolicyGlobalManagementID,
				},
			},
			Local: true,
		}

		require.NoError(t, s.ACLTokenSet(2, token.Clone()))

		_, rtoken, err := s.ACLTokenGetByAccessor(nil, "f1093997-b6c7-496d-bfb8-6b1b1895641b", nil)
		require.NoError(t, err)
		require.NotNil(t, rtoken)

		require.NoError(t, s.ACLTokenDeleteByAccessor(3, "f1093997-b6c7-496d-bfb8-6b1b1895641b", nil))

		_, rtoken, err = s.ACLTokenGetByAccessor(nil, "f1093997-b6c7-496d-bfb8-6b1b1895641b", nil)
		require.NoError(t, err)
		require.Nil(t, rtoken)
	})

	t.Run("Multiple", func(t *testing.T) {
		t.Parallel()
		s := testACLTokensStateStore(t)

		tokens := structs.ACLTokens{
			&structs.ACLToken{
				AccessorID: "f1093997-b6c7-496d-bfb8-6b1b1895641b",
				SecretID:   "34ec8eb3-095d-417a-a937-b439af7a8e8b",
				Policies: []structs.ACLTokenPolicyLink{
					{
						ID: structs.ACLPolicyGlobalManagementID,
					},
				},
				Local: true,
			},
			&structs.ACLToken{
				AccessorID: "a0bfe8d4-b2f3-4b48-b387-f28afb820eab",
				SecretID:   "be444e46-fb95-4ccc-80d5-c873f34e6fa6",
				Policies: []structs.ACLTokenPolicyLink{
					{
						ID: structs.ACLPolicyGlobalManagementID,
					},
				},
				Local: true,
			},
		}

		require.NoError(t, s.ACLTokenBatchSet(2, tokens, ACLTokenSetOptions{}))

		_, rtoken, err := s.ACLTokenGetByAccessor(nil, "f1093997-b6c7-496d-bfb8-6b1b1895641b", nil)
		require.NoError(t, err)
		require.NotNil(t, rtoken)
		_, rtoken, err = s.ACLTokenGetByAccessor(nil, "a0bfe8d4-b2f3-4b48-b387-f28afb820eab", nil)
		require.NoError(t, err)
		require.NotNil(t, rtoken)

		require.NoError(t, s.ACLTokenBatchDelete(2, []string{
			"f1093997-b6c7-496d-bfb8-6b1b1895641b",
			"a0bfe8d4-b2f3-4b48-b387-f28afb820eab"}))

		_, rtoken, err = s.ACLTokenGetByAccessor(nil, "f1093997-b6c7-496d-bfb8-6b1b1895641b", nil)
		require.NoError(t, err)
		require.Nil(t, rtoken)
		_, rtoken, err = s.ACLTokenGetByAccessor(nil, "a0bfe8d4-b2f3-4b48-b387-f28afb820eab", nil)
		require.NoError(t, err)
		require.Nil(t, rtoken)
	})

	t.Run("Anonymous", func(t *testing.T) {
		t.Parallel()
		s := testACLTokensStateStore(t)

		require.Error(t, s.ACLTokenDeleteByAccessor(3, structs.ACLTokenAnonymousID, nil))
	})

	t.Run("Not Found", func(t *testing.T) {
		t.Parallel()
		s := testACLStateStore(t)

		// deletion of non-existent policies is not an error
		require.NoError(t, s.ACLTokenDeleteByAccessor(3, "ea58a09c-2100-4aef-816b-8ee0ade77dcd", nil))
	})
}

func TestStateStore_ACLPolicy_SetGet(t *testing.T) {
	t.Parallel()

	t.Run("Missing ID", func(t *testing.T) {
		t.Parallel()
		s := testACLStateStore(t)

		policy := structs.ACLPolicy{
			Name:        "test-policy",
			Description: "test",
			Rules:       `keyring = "write"`,
		}

		require.Error(t, s.ACLPolicySet(3, &policy))
	})

	t.Run("Missing Name", func(t *testing.T) {
		t.Parallel()
		s := testACLStateStore(t)

		policy := structs.ACLPolicy{
			ID:          testRoleID_A,
			Description: "test",
			Rules:       `keyring = "write"`,
		}

		require.Error(t, s.ACLPolicySet(3, &policy))
	})

	t.Run("Global Management", func(t *testing.T) {
		t.Parallel()
		s := testACLStateStore(t)

		t.Run("Rules", func(t *testing.T) {
			t.Parallel()

			policy := structs.ACLPolicy{
				ID:          structs.ACLPolicyGlobalManagementID,
				Name:        "global-management",
				Description: "Global Management",
				Rules:       `acl = "write"`,
			}

			require.Error(t, s.ACLPolicySet(3, &policy))
		})

		t.Run("Datacenters", func(t *testing.T) {
			t.Parallel()

			policy := structs.ACLPolicy{
				ID:          structs.ACLPolicyGlobalManagementID,
				Name:        "global-management",
				Description: "Global Management",
				Rules:       structs.ACLPolicyGlobalManagement,
				Datacenters: []string{"dc1"},
			}

			require.Error(t, s.ACLPolicySet(3, &policy))
		})

		t.Run("Change", func(t *testing.T) {
			t.Parallel()

			policy := structs.ACLPolicy{
				ID:          structs.ACLPolicyGlobalManagementID,
				Name:        "management",
				Description: "Modified",
				Rules:       structs.ACLPolicyGlobalManagement,
			}

			require.NoError(t, s.ACLPolicySet(3, &policy))

			_, rpolicy, err := s.ACLPolicyGetByName(nil, "management", nil)
			require.NoError(t, err)
			require.NotNil(t, rpolicy)
			require.Equal(t, structs.ACLPolicyGlobalManagementID, rpolicy.ID)
			require.Equal(t, "management", rpolicy.Name)
			require.Equal(t, "Modified", rpolicy.Description)
			require.Equal(t, uint64(1), rpolicy.CreateIndex)
			require.Equal(t, uint64(3), rpolicy.ModifyIndex)
		})
	})

	t.Run("New", func(t *testing.T) {
		t.Parallel()
		// this actually creates a new policy - we just need to verify it.
		s := testACLStateStore(t)

		policy := structs.ACLPolicy{
			ID:          testPolicyID_A,
			Name:        "node-read",
			Description: "Allows reading all node information",
			Rules:       `node_prefix "" { policy = "read" }`,
			Syntax:      acl.SyntaxCurrent,
			Datacenters: []string{"dc1"},
		}

		require.NoError(t, s.ACLPolicySet(3, &policy))

		idx, rpolicy, err := s.ACLPolicyGetByID(nil, testPolicyID_A, nil)
		require.Equal(t, uint64(3), idx)
		require.NoError(t, err)
		require.NotNil(t, rpolicy)
		require.Equal(t, "node-read", rpolicy.Name)
		require.Equal(t, "Allows reading all node information", rpolicy.Description)
		require.Equal(t, `node_prefix "" { policy = "read" }`, rpolicy.Rules)
		require.Equal(t, acl.SyntaxCurrent, rpolicy.Syntax)
		require.Len(t, rpolicy.Datacenters, 1)
		require.Equal(t, "dc1", rpolicy.Datacenters[0])
		require.Equal(t, uint64(3), rpolicy.CreateIndex)
		require.Equal(t, uint64(3), rpolicy.ModifyIndex)

		// also verify the global management policy that testACLStateStore Set while we are at it.
		idx, rpolicy, err = s.ACLPolicyGetByID(nil, structs.ACLPolicyGlobalManagementID, nil)
		require.Equal(t, uint64(3), idx)
		require.NoError(t, err)
		require.NotNil(t, rpolicy)
		require.Equal(t, "global-management", rpolicy.Name)
		require.Equal(t, "Builtin Policy that grants unlimited access", rpolicy.Description)
		require.Equal(t, structs.ACLPolicyGlobalManagement, rpolicy.Rules)
		require.Equal(t, acl.SyntaxCurrent, rpolicy.Syntax)
		require.Len(t, rpolicy.Datacenters, 0)
		require.Equal(t, uint64(1), rpolicy.CreateIndex)
		require.Equal(t, uint64(1), rpolicy.ModifyIndex)
	})

	t.Run("Update", func(t *testing.T) {
		t.Parallel()
		// this creates the node read policy which we can update
		s := testACLTokensStateStore(t)

		update := &structs.ACLPolicy{
			ID:          testPolicyID_A,
			Name:        "node-read-modified",
			Description: "Modified",
			Rules:       `node_prefix "" { policy = "read" } node "secret" { policy = "deny" }`,
			Syntax:      acl.SyntaxCurrent,
			Datacenters: []string{"dc1", "dc2"},
		}

		require.NoError(t, s.ACLPolicySet(3, update.Clone()))

		expect := update.Clone()
		expect.CreateIndex = 2
		expect.ModifyIndex = 3

		// policy found via id
		idx, rpolicy, err := s.ACLPolicyGetByID(nil, testPolicyID_A, nil)
		require.NoError(t, err)
		require.Equal(t, uint64(3), idx)
		require.Equal(t, expect, rpolicy)

		// policy no longer found via old name
		idx, rpolicy, err = s.ACLPolicyGetByName(nil, "node-read", nil)
		require.Equal(t, uint64(3), idx)
		require.NoError(t, err)
		require.Nil(t, rpolicy)

		// policy is found via new name
		idx, rpolicy, err = s.ACLPolicyGetByName(nil, "node-read-modified", nil)
		require.NoError(t, err)
		require.Equal(t, uint64(3), idx)
		require.Equal(t, expect, rpolicy)
	})
}

func TestStateStore_ACLPolicy_UpsertBatchRead(t *testing.T) {
	t.Parallel()

	t.Run("Normal", func(t *testing.T) {
		t.Parallel()
		s := testACLStateStore(t)

		policies := structs.ACLPolicies{
			&structs.ACLPolicy{
				ID:    "a4f68bd6-3af5-4f56-b764-3c6f20247879",
				Name:  "service-read",
				Rules: `service_prefix "" { policy = "read" }`,
			},
			&structs.ACLPolicy{
				ID:          "a2719052-40b3-4a4b-baeb-f3df1831a217",
				Name:        "acl-write-dc3",
				Description: "Can manage ACLs in dc3",
				Datacenters: []string{"dc3"},
				Rules:       `acl = "write"`,
			},
		}

		require.NoError(t, s.ACLPolicyBatchSet(2, policies))

		idx, rpolicies, err := s.ACLPolicyBatchGet(nil, []string{
			"a4f68bd6-3af5-4f56-b764-3c6f20247879",
			"a2719052-40b3-4a4b-baeb-f3df1831a217"})

		require.NoError(t, err)
		require.Equal(t, uint64(2), idx)
		require.Len(t, rpolicies, 2)
		require.ElementsMatch(t, policies, rpolicies)
		require.Equal(t, uint64(2), rpolicies[0].CreateIndex)
		require.Equal(t, uint64(2), rpolicies[0].ModifyIndex)
		require.Equal(t, uint64(2), rpolicies[1].CreateIndex)
		require.Equal(t, uint64(2), rpolicies[1].ModifyIndex)
	})

	t.Run("Update", func(t *testing.T) {
		t.Parallel()
		s := testACLStateStore(t)

		policies := structs.ACLPolicies{
			&structs.ACLPolicy{
				ID:    "a4f68bd6-3af5-4f56-b764-3c6f20247879",
				Name:  "service-read",
				Rules: `service_prefix "" { policy = "read" }`,
			},
			&structs.ACLPolicy{
				ID:          "a2719052-40b3-4a4b-baeb-f3df1831a217",
				Name:        "acl-write-dc3",
				Description: "Can manage ACLs in dc3",
				Datacenters: []string{"dc3"},
				Rules:       `acl = "write"`,
			},
		}

		require.NoError(t, s.ACLPolicyBatchSet(2, policies))

		updates := structs.ACLPolicies{
			&structs.ACLPolicy{
				ID:          "a4f68bd6-3af5-4f56-b764-3c6f20247879",
				Name:        "service-write",
				Rules:       `service_prefix "" { policy = "write" }`,
				Datacenters: []string{"dc1"},
			},
			&structs.ACLPolicy{
				ID:          "a2719052-40b3-4a4b-baeb-f3df1831a217",
				Name:        "acl-write",
				Description: "Modified",
				Rules:       `acl = "write"`,
			},
		}

		require.NoError(t, s.ACLPolicyBatchSet(3, updates))

		idx, rpolicies, err := s.ACLPolicyBatchGet(nil, []string{
			"a4f68bd6-3af5-4f56-b764-3c6f20247879",
			"a2719052-40b3-4a4b-baeb-f3df1831a217"})

		require.NoError(t, err)
		require.Equal(t, uint64(3), idx)
		require.Len(t, rpolicies, 2)
		rpolicies.Sort()
		require.Equal(t, "a2719052-40b3-4a4b-baeb-f3df1831a217", rpolicies[0].ID)
		require.Equal(t, "acl-write", rpolicies[0].Name)
		require.Equal(t, "Modified", rpolicies[0].Description)
		require.Equal(t, `acl = "write"`, rpolicies[0].Rules)
		require.Empty(t, rpolicies[0].Datacenters)
		require.Equal(t, uint64(2), rpolicies[0].CreateIndex)
		require.Equal(t, uint64(3), rpolicies[0].ModifyIndex)

		require.Equal(t, "a4f68bd6-3af5-4f56-b764-3c6f20247879", rpolicies[1].ID)
		require.Equal(t, "service-write", rpolicies[1].Name)
		require.Equal(t, "", rpolicies[1].Description)
		require.Equal(t, `service_prefix "" { policy = "write" }`, rpolicies[1].Rules)
		require.ElementsMatch(t, []string{"dc1"}, rpolicies[1].Datacenters)
		require.Equal(t, uint64(2), rpolicies[1].CreateIndex)
		require.Equal(t, uint64(3), rpolicies[1].ModifyIndex)
	})
}

func TestStateStore_ACLPolicy_List(t *testing.T) {
	t.Parallel()
	s := testACLStateStore(t)

	policies := structs.ACLPolicies{
		&structs.ACLPolicy{
			ID:    "a4f68bd6-3af5-4f56-b764-3c6f20247879",
			Name:  "service-read",
			Rules: `service_prefix "" { policy = "read" }`,
		},
		&structs.ACLPolicy{
			ID:          "a2719052-40b3-4a4b-baeb-f3df1831a217",
			Name:        "acl-write-dc3",
			Description: "Can manage ACLs in dc3",
			Datacenters: []string{"dc3"},
			Rules:       `acl = "write"`,
		},
	}

	require.NoError(t, s.ACLPolicyBatchSet(2, policies))

	_, policies, err := s.ACLPolicyList(nil, nil)
	require.NoError(t, err)
	require.Len(t, policies, 3)
	policies.Sort()
	require.Equal(t, structs.ACLPolicyGlobalManagementID, policies[0].ID)
	require.Equal(t, "global-management", policies[0].Name)
	require.Equal(t, "Builtin Policy that grants unlimited access", policies[0].Description)
	require.Empty(t, policies[0].Datacenters)
	require.NotEqual(t, []byte{}, policies[0].Hash)
	require.Equal(t, uint64(1), policies[0].CreateIndex)
	require.Equal(t, uint64(1), policies[0].ModifyIndex)

	require.Equal(t, "a2719052-40b3-4a4b-baeb-f3df1831a217", policies[1].ID)
	require.Equal(t, "acl-write-dc3", policies[1].Name)
	require.Equal(t, "Can manage ACLs in dc3", policies[1].Description)
	require.ElementsMatch(t, []string{"dc3"}, policies[1].Datacenters)
	require.Nil(t, policies[1].Hash)
	require.Equal(t, uint64(2), policies[1].CreateIndex)
	require.Equal(t, uint64(2), policies[1].ModifyIndex)

	require.Equal(t, "a4f68bd6-3af5-4f56-b764-3c6f20247879", policies[2].ID)
	require.Equal(t, "service-read", policies[2].Name)
	require.Equal(t, "", policies[2].Description)
	require.Empty(t, policies[2].Datacenters)
	require.Nil(t, policies[2].Hash)
	require.Equal(t, uint64(2), policies[2].CreateIndex)
	require.Equal(t, uint64(2), policies[2].ModifyIndex)
}

func TestStateStore_ACLPolicy_Delete(t *testing.T) {
	t.Parallel()

	t.Run("ID", func(t *testing.T) {
		t.Parallel()
		s := testACLStateStore(t)

		policy := &structs.ACLPolicy{
			ID:    "f1093997-b6c7-496d-bfb8-6b1b1895641b",
			Name:  "test-policy",
			Rules: `acl = "read"`,
		}

		require.NoError(t, s.ACLPolicySet(2, policy))

		_, rpolicy, err := s.ACLPolicyGetByID(nil, "f1093997-b6c7-496d-bfb8-6b1b1895641b", nil)
		require.NoError(t, err)
		require.NotNil(t, rpolicy)

		require.NoError(t, s.ACLPolicyDeleteByID(3, "f1093997-b6c7-496d-bfb8-6b1b1895641b", nil))
		require.NoError(t, err)

		_, rpolicy, err = s.ACLPolicyGetByID(nil, "f1093997-b6c7-496d-bfb8-6b1b1895641b", nil)
		require.NoError(t, err)
		require.Nil(t, rpolicy)
	})

	t.Run("Name", func(t *testing.T) {
		t.Parallel()
		s := testACLStateStore(t)

		policy := &structs.ACLPolicy{
			ID:    "f1093997-b6c7-496d-bfb8-6b1b1895641b",
			Name:  "test-policy",
			Rules: `acl = "read"`,
		}

		require.NoError(t, s.ACLPolicySet(2, policy))

		_, rpolicy, err := s.ACLPolicyGetByName(nil, "test-policy", nil)
		require.NoError(t, err)
		require.NotNil(t, rpolicy)

		require.NoError(t, s.ACLPolicyDeleteByName(3, "test-policy", nil))
		require.NoError(t, err)

		_, rpolicy, err = s.ACLPolicyGetByName(nil, "test-policy", nil)
		require.NoError(t, err)
		require.Nil(t, rpolicy)
	})

	t.Run("Multiple", func(t *testing.T) {
		t.Parallel()
		s := testACLStateStore(t)

		policies := structs.ACLPolicies{
			&structs.ACLPolicy{
				ID:    "f1093997-b6c7-496d-bfb8-6b1b1895641b",
				Name:  "34ec8eb3-095d-417a-a937-b439af7a8e8b",
				Rules: `acl = "read"`,
			},
			&structs.ACLPolicy{
				ID:    "a0bfe8d4-b2f3-4b48-b387-f28afb820eab",
				Name:  "be444e46-fb95-4ccc-80d5-c873f34e6fa6",
				Rules: `acl = "write"`,
			},
		}

		require.NoError(t, s.ACLPolicyBatchSet(2, policies))

		_, rpolicy, err := s.ACLPolicyGetByID(nil, "f1093997-b6c7-496d-bfb8-6b1b1895641b", nil)
		require.NoError(t, err)
		require.NotNil(t, rpolicy)
		_, rpolicy, err = s.ACLPolicyGetByID(nil, "a0bfe8d4-b2f3-4b48-b387-f28afb820eab", nil)
		require.NoError(t, err)
		require.NotNil(t, rpolicy)

		require.NoError(t, s.ACLPolicyBatchDelete(3, []string{
			"f1093997-b6c7-496d-bfb8-6b1b1895641b",
			"a0bfe8d4-b2f3-4b48-b387-f28afb820eab"}))

		_, rpolicy, err = s.ACLPolicyGetByID(nil, "f1093997-b6c7-496d-bfb8-6b1b1895641b", nil)
		require.NoError(t, err)
		require.Nil(t, rpolicy)
		_, rpolicy, err = s.ACLPolicyGetByID(nil, "a0bfe8d4-b2f3-4b48-b387-f28afb820eab", nil)
		require.NoError(t, err)
		require.Nil(t, rpolicy)
	})

	t.Run("Global-Management", func(t *testing.T) {
		t.Parallel()
		s := testACLStateStore(t)

		require.Error(t, s.ACLPolicyDeleteByID(5, structs.ACLPolicyGlobalManagementID, nil))
		require.Error(t, s.ACLPolicyDeleteByName(5, "global-management", nil))
	})

	t.Run("Not Found", func(t *testing.T) {
		t.Parallel()
		s := testACLStateStore(t)

		// deletion of non-existent policies is not an error
		require.NoError(t, s.ACLPolicyDeleteByName(3, "not-found", nil))
		require.NoError(t, s.ACLPolicyDeleteByID(3, "376d0cae-dd50-4213-9668-2c7797a7fb2d", nil))
	})
}

func TestStateStore_ACLRole_SetGet(t *testing.T) {
	t.Parallel()

	t.Run("Missing ID", func(t *testing.T) {
		t.Parallel()
		s := testACLRolesStateStore(t)

		role := structs.ACLRole{
			Name:        "test-role",
			Description: "test",
			Policies: []structs.ACLRolePolicyLink{
				{
					ID: structs.ACLPolicyGlobalManagementID,
				},
			},
		}

		require.Error(t, s.ACLRoleSet(3, &role))
	})

	t.Run("Missing Name", func(t *testing.T) {
		t.Parallel()
		s := testACLRolesStateStore(t)

		role := structs.ACLRole{
			ID:          testRoleID_A,
			Description: "test",
			Policies: []structs.ACLRolePolicyLink{
				{
					ID: structs.ACLPolicyGlobalManagementID,
				},
			},
		}

		require.Error(t, s.ACLRoleSet(3, &role))
	})

	t.Run("Missing Service Identity Fields", func(t *testing.T) {
		t.Parallel()
		s := testACLRolesStateStore(t)

		role := structs.ACLRole{
			ID:          testRoleID_A,
			Description: "test",
			ServiceIdentities: []*structs.ACLServiceIdentity{
				{},
			},
		}

		require.Error(t, s.ACLRoleSet(3, &role))
	})

	t.Run("Missing Service Identity Name", func(t *testing.T) {
		t.Parallel()
		s := testACLRolesStateStore(t)

		role := structs.ACLRole{
			ID:          testRoleID_A,
			Description: "test",
			ServiceIdentities: []*structs.ACLServiceIdentity{
				{
					Datacenters: []string{"dc1"},
				},
			},
		}

		require.Error(t, s.ACLRoleSet(3, &role))
	})

	t.Run("Missing Policy ID", func(t *testing.T) {
		t.Parallel()
		s := testACLRolesStateStore(t)

		role := structs.ACLRole{
			ID:          testRoleID_A,
			Description: "test",
			Policies: []structs.ACLRolePolicyLink{
				{
					Name: "no-id",
				},
			},
		}

		require.Error(t, s.ACLRoleSet(3, &role))
	})

	t.Run("Unresolvable Policy ID", func(t *testing.T) {
		t.Parallel()
		s := testACLRolesStateStore(t)

		role := structs.ACLRole{
			ID:          testRoleID_A,
			Description: "test",
			Policies: []structs.ACLRolePolicyLink{
				{
					ID: "4f20e379-b496-4b99-9599-19a197126490",
				},
			},
		}

		require.Error(t, s.ACLRoleSet(3, &role))
	})

	t.Run("New", func(t *testing.T) {
		t.Parallel()
		s := testACLRolesStateStore(t)

		role := structs.ACLRole{
			ID:          testRoleID_A,
			Name:        "my-new-role",
			Description: "test",
			Policies: []structs.ACLRolePolicyLink{
				{
					ID: testPolicyID_A,
				},
			},
		}

		require.NoError(t, s.ACLRoleSet(3, &role))

		verify := func(idx uint64, rrole *structs.ACLRole, err error) {
			require.Equal(t, uint64(3), idx)
			require.NoError(t, err)
			require.NotNil(t, rrole)
			require.Equal(t, "my-new-role", rrole.Name)
			require.Equal(t, "test", rrole.Description)
			require.Equal(t, uint64(3), rrole.CreateIndex)
			require.Equal(t, uint64(3), rrole.ModifyIndex)
			require.Len(t, rrole.ServiceIdentities, 0)
			// require.ElementsMatch(t, role.Policies, rrole.Policies)
			require.Len(t, rrole.Policies, 1)
			require.Equal(t, "node-read", rrole.Policies[0].Name)
		}

		idx, rpolicy, err := s.ACLRoleGetByID(nil, testRoleID_A, nil)
		verify(idx, rpolicy, err)

		idx, rpolicy, err = s.ACLRoleGetByName(nil, "my-new-role", nil)
		verify(idx, rpolicy, err)
	})

	t.Run("Update", func(t *testing.T) {
		t.Parallel()
		s := testACLRolesStateStore(t)

		// Create the initial role
		role := &structs.ACLRole{
			ID:          testRoleID_A,
			Name:        "node-read-role",
			Description: "Allows reading all node information",
			Policies: []structs.ACLRolePolicyLink{
				{
					ID: testPolicyID_A,
				},
			},
		}
		role.SetHash(true)

		require.NoError(t, s.ACLRoleSet(2, role))

		// Now make sure we can update it
		update := &structs.ACLRole{
			ID:          testRoleID_A,
			Name:        "node-read-role-modified",
			Description: "Modified",
			Policies: []structs.ACLRolePolicyLink{
				{
					ID: structs.ACLPolicyGlobalManagementID,
				},
			},
		}
		update.SetHash(true)

		require.NoError(t, s.ACLRoleSet(3, update))

		verify := func(idx uint64, rrole *structs.ACLRole, err error) {
			require.Equal(t, uint64(3), idx)
			require.NoError(t, err)
			require.NotNil(t, rrole)
			require.Equal(t, "node-read-role-modified", rrole.Name)
			require.Equal(t, "Modified", rrole.Description)
			require.Equal(t, uint64(2), rrole.CreateIndex)
			require.Equal(t, uint64(3), rrole.ModifyIndex)
			require.Len(t, rrole.ServiceIdentities, 0)
			require.Len(t, rrole.Policies, 1)
			require.Equal(t, structs.ACLPolicyGlobalManagementID, rrole.Policies[0].ID)
			require.Equal(t, "global-management", rrole.Policies[0].Name)
		}

		// role found via id
		idx, rrole, err := s.ACLRoleGetByID(nil, testRoleID_A, nil)
		verify(idx, rrole, err)

		// role no longer found via old name
		idx, rrole, err = s.ACLRoleGetByName(nil, "node-read-role", nil)
		require.Equal(t, uint64(3), idx)
		require.NoError(t, err)
		require.Nil(t, rrole)

		// role is found via new name
		idx, rrole, err = s.ACLRoleGetByName(nil, "node-read-role-modified", nil)
		verify(idx, rrole, err)
	})
}

func TestStateStore_ACLRoles_UpsertBatchRead(t *testing.T) {
	t.Parallel()

	t.Run("Normal", func(t *testing.T) {
		t.Parallel()
		s := testACLRolesStateStore(t)

		roles := structs.ACLRoles{
			&structs.ACLRole{
				ID:          testRoleID_A,
				Name:        "role1",
				Description: "test-role1",
				Policies: []structs.ACLRolePolicyLink{
					{
						ID: testPolicyID_A,
					},
				},
			},
			&structs.ACLRole{
				ID:          testRoleID_B,
				Name:        "role2",
				Description: "test-role2",
				Policies: []structs.ACLRolePolicyLink{
					{
						ID: testPolicyID_B,
					},
				},
			},
		}

		require.NoError(t, s.ACLRoleBatchSet(2, roles, false))

		idx, rroles, err := s.ACLRoleBatchGet(nil, []string{testRoleID_A, testRoleID_B})
		require.NoError(t, err)
		require.Equal(t, uint64(2), idx)
		require.Len(t, rroles, 2)
		rroles.Sort()
		require.ElementsMatch(t, roles, rroles)
		require.Equal(t, uint64(2), rroles[0].CreateIndex)
		require.Equal(t, uint64(2), rroles[0].ModifyIndex)
		require.Equal(t, uint64(2), rroles[1].CreateIndex)
		require.Equal(t, uint64(2), rroles[1].ModifyIndex)
	})

	t.Run("Update", func(t *testing.T) {
		t.Parallel()
		s := testACLRolesStateStore(t)

		// Seed initial data.
		roles := structs.ACLRoles{
			&structs.ACLRole{
				ID:          testRoleID_A,
				Name:        "role1",
				Description: "test-role1",
				Policies: []structs.ACLRolePolicyLink{
					{
						ID: testPolicyID_A,
					},
				},
			},
			&structs.ACLRole{
				ID:          testRoleID_B,
				Name:        "role2",
				Description: "test-role2",
				Policies: []structs.ACLRolePolicyLink{
					{
						ID: testPolicyID_B,
					},
				},
			},
		}

		require.NoError(t, s.ACLRoleBatchSet(2, roles, false))

		// Update two roles at the same time.
		updates := structs.ACLRoles{
			&structs.ACLRole{
				ID:          testRoleID_A,
				Name:        "role1-modified",
				Description: "test-role1-modified",
				Policies: []structs.ACLRolePolicyLink{
					{
						ID: testPolicyID_C,
					},
				},
			},
			&structs.ACLRole{
				ID:   testRoleID_B,
				Name: "role2-modified",
				Policies: []structs.ACLRolePolicyLink{
					{
						ID: testPolicyID_D,
					},
					{
						ID: testPolicyID_E,
					},
				},
			},
		}

		require.NoError(t, s.ACLRoleBatchSet(3, updates, false))

		idx, rroles, err := s.ACLRoleBatchGet(nil, []string{testRoleID_A, testRoleID_B})

		require.NoError(t, err)
		require.Equal(t, uint64(3), idx)
		require.Len(t, rroles, 2)
		rroles.Sort()
		require.Equal(t, testRoleID_A, rroles[0].ID)
		require.Equal(t, "role1-modified", rroles[0].Name)
		require.Equal(t, "test-role1-modified", rroles[0].Description)
		require.ElementsMatch(t, updates[0].Policies, rroles[0].Policies)
		require.Equal(t, uint64(2), rroles[0].CreateIndex)
		require.Equal(t, uint64(3), rroles[0].ModifyIndex)

		require.Equal(t, testRoleID_B, rroles[1].ID)
		require.Equal(t, "role2-modified", rroles[1].Name)
		require.Equal(t, "", rroles[1].Description)
		require.ElementsMatch(t, updates[1].Policies, rroles[1].Policies)
		require.Equal(t, uint64(2), rroles[1].CreateIndex)
		require.Equal(t, uint64(3), rroles[1].ModifyIndex)
	})

	t.Run("AllowMissing - Policy", func(t *testing.T) {
		t.Parallel()
		s := testACLRolesStateStore(t)

		const fakePolicyID = "0ea7b58a-3d86-4e82-b656-577b63d727f3"

		roles := structs.ACLRoles{
			&structs.ACLRole{
				ID:          "d08ca6e3-a000-487e-8d25-e0cb616c221d",
				Name:        "role1",
				Description: "test-role1",
				Policies: []structs.ACLRolePolicyLink{
					{
						ID: fakePolicyID,
					},
				},
			},
		}

		require.Error(t, s.ACLRoleBatchSet(2, roles, false))

		require.NoError(t, s.ACLRoleBatchSet(2, roles, true))

		idx, rroles, err := s.ACLRoleBatchGet(nil, []string{
			"d08ca6e3-a000-487e-8d25-e0cb616c221d",
		})

		require.NoError(t, err)
		require.Equal(t, uint64(2), idx)
		require.Len(t, rroles, 1)

		// Persisting invalid IDs will cause them to be masked during read. So
		// before we compare structures strike the dead entries.
		roles[0].Policies = []structs.ACLRolePolicyLink{}

		require.Equal(t, roles[0], rroles[0])
		require.Equal(t, uint64(2), rroles[0].CreateIndex)
		require.Equal(t, uint64(2), rroles[0].ModifyIndex)
	})
}

func TestStateStore_ACLRole_List(t *testing.T) {
	t.Parallel()
	s := testACLRolesStateStore(t)

	roles := structs.ACLRoles{
		&structs.ACLRole{
			ID:          testRoleID_A,
			Name:        "role1",
			Description: "test-role1",
			Policies: []structs.ACLRolePolicyLink{
				{
					ID: testPolicyID_A,
				},
			},
		},
		&structs.ACLRole{
			ID:          testRoleID_B,
			Name:        "role2",
			Description: "test-role2",
			Policies: []structs.ACLRolePolicyLink{
				{
					ID: testPolicyID_B,
				},
			},
		},
	}

	require.NoError(t, s.ACLRoleBatchSet(2, roles, false))

	type testCase struct {
		name   string
		policy string
		ids    []string
	}

	cases := []testCase{
		{
			name:   "Any",
			policy: "",
			ids: []string{
				testRoleID_A,
				testRoleID_B,
			},
		},
		{
			name:   "Policy A",
			policy: testPolicyID_A,
			ids: []string{
				testRoleID_A,
			},
		},
		{
			name:   "Policy B",
			policy: testPolicyID_B,
			ids: []string{
				testRoleID_B,
			},
		},
	}

	for _, tc := range cases {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			// t.Parallel()
			_, rroles, err := s.ACLRoleList(nil, tc.policy, nil)
			require.NoError(t, err)

			require.Len(t, rroles, len(tc.ids))
			rroles.Sort()
			for i, rrole := range rroles {
				expectID := tc.ids[i]
				require.Equal(t, expectID, rrole.ID)
				switch expectID {
				case testRoleID_A:
					require.Equal(t, testRoleID_A, rrole.ID)
					require.Equal(t, "role1", rrole.Name)
					require.Equal(t, "test-role1", rrole.Description)
					require.ElementsMatch(t, roles[0].Policies, rrole.Policies)
					require.Nil(t, rrole.Hash)
					require.Equal(t, uint64(2), rrole.CreateIndex)
					require.Equal(t, uint64(2), rrole.ModifyIndex)
				case testRoleID_B:
					require.Equal(t, testRoleID_B, rrole.ID)
					require.Equal(t, "role2", rrole.Name)
					require.Equal(t, "test-role2", rrole.Description)
					require.ElementsMatch(t, roles[1].Policies, rrole.Policies)
					require.Nil(t, rrole.Hash)
					require.Equal(t, uint64(2), rrole.CreateIndex)
					require.Equal(t, uint64(2), rrole.ModifyIndex)
				}
			}
		})
	}
}

func TestStateStore_ACLRole_FixupPolicyLinks(t *testing.T) {
	// This test wants to ensure a couple of things.
	//
	// 1. Doing a role list/get should never modify the data
	//    tracked by memdb
	// 2. Role list/get operations should return an accurate set
	//    of policy links
	t.Parallel()
	s := testACLRolesStateStore(t)

	// the policy specific role
	role := &structs.ACLRole{
		ID:   "672537b1-35cb-48fc-a2cd-a1863c301b70",
		Name: "test-role",
		Policies: []structs.ACLRolePolicyLink{
			{
				ID: testPolicyID_A,
			},
		},
	}

	require.NoError(t, s.ACLRoleSet(2, role))

	_, retrieved, err := s.ACLRoleGetByID(nil, role.ID, nil)
	require.NoError(t, err)
	// pointer equality check these should be identical
	require.True(t, role == retrieved)
	require.Len(t, retrieved.Policies, 1)
	require.Equal(t, "node-read", retrieved.Policies[0].Name)

	// rename the policy
	renamed := &structs.ACLPolicy{
		ID:          testPolicyID_A,
		Name:        "node-read-renamed",
		Description: "Allows reading all node information",
		Rules:       `node_prefix "" { policy = "read" }`,
		Syntax:      acl.SyntaxCurrent,
	}
	renamed.SetHash(true)
	require.NoError(t, s.ACLPolicySet(3, renamed))

	// retrieve the role again
	_, retrieved, err = s.ACLRoleGetByID(nil, role.ID, nil)
	require.NoError(t, err)
	// pointer equality check these should be different if we cloned things appropriately
	require.True(t, role != retrieved)
	require.Len(t, retrieved.Policies, 1)
	require.Equal(t, "node-read-renamed", retrieved.Policies[0].Name)

	// list roles without stale links
	_, roles, err := s.ACLRoleList(nil, "", nil)
	require.NoError(t, err)

	found := false
	for _, r := range roles {
		if r.ID == role.ID {
			// these pointers shouldn't be equal because the link should have been fixed
			require.True(t, r != role)
			require.Len(t, r.Policies, 1)
			require.Equal(t, "node-read-renamed", r.Policies[0].Name)
			found = true
			break
		}
	}
	require.True(t, found)

	// batch get without stale links
	_, roles, err = s.ACLRoleBatchGet(nil, []string{role.ID})
	require.NoError(t, err)

	found = false
	for _, r := range roles {
		if r.ID == role.ID {
			// these pointers shouldn't be equal because the link should have been fixed
			require.True(t, r != role)
			require.Len(t, r.Policies, 1)
			require.Equal(t, "node-read-renamed", r.Policies[0].Name)
			found = true
			break
		}
	}
	require.True(t, found)

	// delete the policy
	require.NoError(t, s.ACLPolicyDeleteByID(4, testPolicyID_A, nil))

	// retrieve the role again
	_, retrieved, err = s.ACLRoleGetByID(nil, role.ID, nil)
	require.NoError(t, err)
	// pointer equality check these should be different if we cloned things appropriately
	require.True(t, role != retrieved)
	require.Len(t, retrieved.Policies, 0)

	// list roles without stale links
	_, roles, err = s.ACLRoleList(nil, "", nil)
	require.NoError(t, err)

	found = false
	for _, r := range roles {
		if r.ID == role.ID {
			// these pointers shouldn't be equal because the link should have been fixed
			require.True(t, r != role)
			require.Len(t, r.Policies, 0)
			found = true
			break
		}
	}
	require.True(t, found)

	// batch get without stale links
	_, roles, err = s.ACLRoleBatchGet(nil, []string{role.ID})
	require.NoError(t, err)

	found = false
	for _, r := range roles {
		if r.ID == role.ID {
			// these pointers shouldn't be equal because the link should have been fixed
			require.True(t, r != role)
			require.Len(t, r.Policies, 0)
			found = true
			break
		}
	}
	require.True(t, found)
}

func TestStateStore_ACLRole_Delete(t *testing.T) {
	t.Parallel()

	t.Run("ID", func(t *testing.T) {
		t.Parallel()
		s := testACLRolesStateStore(t)

		role := &structs.ACLRole{
			ID:          testRoleID_A,
			Name:        "role1",
			Description: "test-role1",
			Policies: []structs.ACLRolePolicyLink{
				{
					ID: structs.ACLPolicyGlobalManagementID,
				},
			},
		}

		require.NoError(t, s.ACLRoleSet(2, role))

		_, rrole, err := s.ACLRoleGetByID(nil, testRoleID_A, nil)
		require.NoError(t, err)
		require.NotNil(t, rrole)

		require.NoError(t, s.ACLRoleDeleteByID(3, testRoleID_A, nil))
		require.NoError(t, err)

		_, rrole, err = s.ACLRoleGetByID(nil, testRoleID_A, nil)
		require.NoError(t, err)
		require.Nil(t, rrole)
	})

	t.Run("Name", func(t *testing.T) {
		t.Parallel()
		s := testACLRolesStateStore(t)

		role := &structs.ACLRole{
			ID:          testRoleID_A,
			Name:        "role1",
			Description: "test-role1",
			Policies: []structs.ACLRolePolicyLink{
				{
					ID: structs.ACLPolicyGlobalManagementID,
				},
			},
		}

		require.NoError(t, s.ACLRoleSet(2, role))

		_, rrole, err := s.ACLRoleGetByName(nil, "role1", nil)
		require.NoError(t, err)
		require.NotNil(t, rrole)

		require.NoError(t, s.ACLRoleDeleteByName(3, "role1", nil))
		require.NoError(t, err)

		_, rrole, err = s.ACLRoleGetByName(nil, "role1", nil)
		require.NoError(t, err)
		require.Nil(t, rrole)
	})

	t.Run("Multiple", func(t *testing.T) {
		t.Parallel()
		s := testACLRolesStateStore(t)

		roles := structs.ACLRoles{
			&structs.ACLRole{
				ID:          testRoleID_A,
				Name:        "role1",
				Description: "test-role1",
				Policies: []structs.ACLRolePolicyLink{
					{
						ID: structs.ACLPolicyGlobalManagementID,
					},
				},
			},
			&structs.ACLRole{
				ID:          testRoleID_B,
				Name:        "role2",
				Description: "test-role2",
				Policies: []structs.ACLRolePolicyLink{
					{
						ID: structs.ACLPolicyGlobalManagementID,
					},
				},
			},
		}

		require.NoError(t, s.ACLRoleBatchSet(2, roles, false))

		_, rrole, err := s.ACLRoleGetByID(nil, testRoleID_A, nil)
		require.NoError(t, err)
		require.NotNil(t, rrole)
		_, rrole, err = s.ACLRoleGetByID(nil, testRoleID_B, nil)
		require.NoError(t, err)
		require.NotNil(t, rrole)

		require.NoError(t, s.ACLRoleBatchDelete(3, []string{testRoleID_A, testRoleID_B}))

		_, rrole, err = s.ACLRoleGetByID(nil, testRoleID_A, nil)
		require.NoError(t, err)
		require.Nil(t, rrole)
		_, rrole, err = s.ACLRoleGetByID(nil, testRoleID_B, nil)
		require.NoError(t, err)
		require.Nil(t, rrole)
	})

	t.Run("Not Found", func(t *testing.T) {
		t.Parallel()
		s := testACLStateStore(t)

		// deletion of non-existent roles is not an error
		require.NoError(t, s.ACLRoleDeleteByName(3, "not-found", nil))
		require.NoError(t, s.ACLRoleDeleteByID(3, testRoleID_A, nil))
	})
}

func TestStateStore_ACLAuthMethod_SetGet(t *testing.T) {
	t.Parallel()

	// The state store only validates key pieces of data, so we only have to
	// care about filling in Name+Type.

	t.Run("Missing Name", func(t *testing.T) {
		t.Parallel()
		s := testACLStateStore(t)

		method := structs.ACLAuthMethod{
			Name:        "",
			Type:        "testing",
			Description: "test",
		}

		require.Error(t, s.ACLAuthMethodSet(3, &method))
	})

	t.Run("Missing Type", func(t *testing.T) {
		t.Parallel()
		s := testACLStateStore(t)

		method := structs.ACLAuthMethod{
			Name:        "test",
			Type:        "",
			Description: "test",
		}

		require.Error(t, s.ACLAuthMethodSet(3, &method))
	})

	t.Run("New", func(t *testing.T) {
		t.Parallel()
		s := testACLStateStore(t)

		method := structs.ACLAuthMethod{
			Name:        "test",
			Type:        "testing",
			Description: "test",
		}

		require.NoError(t, s.ACLAuthMethodSet(3, &method))

		idx, rmethod, err := s.ACLAuthMethodGetByName(nil, "test", nil)
		require.NoError(t, err)
		require.Equal(t, uint64(3), idx)
		require.NotNil(t, rmethod)
		require.Equal(t, "test", rmethod.Name)
		require.Equal(t, "testing", rmethod.Type)
		require.Equal(t, "test", rmethod.Description)
		require.Equal(t, uint64(3), rmethod.CreateIndex)
		require.Equal(t, uint64(3), rmethod.ModifyIndex)
	})

	t.Run("Update", func(t *testing.T) {
		t.Parallel()
		s := testACLStateStore(t)

		// Create the initial method
		method := structs.ACLAuthMethod{
			Name:        "test",
			Type:        "testing",
			Description: "test",
		}

		require.NoError(t, s.ACLAuthMethodSet(2, &method))

		// Now make sure we can update it
		update := structs.ACLAuthMethod{
			Name:        "test",
			Type:        "testing",
			Description: "modified",
			Config: map[string]interface{}{
				"Host": "https://localhost:8443",
			},
		}

		require.NoError(t, s.ACLAuthMethodSet(3, &update))

		idx, rmethod, err := s.ACLAuthMethodGetByName(nil, "test", nil)
		require.NoError(t, err)
		require.Equal(t, uint64(3), idx)
		require.NotNil(t, rmethod)
		require.Equal(t, "test", rmethod.Name)
		require.Equal(t, "testing", rmethod.Type)
		require.Equal(t, "modified", rmethod.Description)
		require.Equal(t, update.Config, rmethod.Config)
		require.Equal(t, uint64(2), rmethod.CreateIndex)
		require.Equal(t, uint64(3), rmethod.ModifyIndex)
	})
}

func TestStateStore_ACLAuthMethod_GlobalNameShadowing_TokenTest(t *testing.T) {
	t.Parallel()

	// This ensures that when a primary DC and secondary DC create identically
	// named auth methods, and the primary instance has a tokenLocality==global
	// that operations in the secondary correctly can target one or the other.

	s := testACLStateStore(t)
	lastIndex := uint64(1)

	// For this test our state machine will simulate the SECONDARY(DC2), so
	// we'll create our auth method here that shadows the global-token-minting
	// one in the primary.

	defaultEntMeta := structs.DefaultEnterpriseMetaInDefaultPartition()

	lastIndex++
	require.NoError(t, s.ACLAuthMethodSet(lastIndex, &structs.ACLAuthMethod{
		Name:           "test",
		Type:           "testing",
		Description:    "test",
		EnterpriseMeta: *defaultEntMeta,
	}))

	const ( // accessors
		methodDC1_tok1 = "6d020c5d-c4fd-4348-ba79-beac37ed0b9c"
		methodDC1_tok2 = "169160dc-34ab-45c6-aba7-ff65e9ace9cb"
		methodDC2_tok1 = "8e14628e-7dde-4573-aca1-6386c0f2095d"
		methodDC2_tok2 = "291e5af9-c68e-4dd3-8824-b2bdfdcc89e6"
	)

	lastIndex++
	require.NoError(t, s.ACLTokenBatchSet(lastIndex, structs.ACLTokens{
		&structs.ACLToken{
			AccessorID:     methodDC2_tok1,
			SecretID:       "d9399b7d-6c34-46bd-a675-c1352fadb6fd",
			Description:    "test-dc2-t1",
			AuthMethod:     "test",
			Local:          true,
			EnterpriseMeta: *defaultEntMeta,
		},
		&structs.ACLToken{
			AccessorID:     methodDC2_tok2,
			SecretID:       "3b72fc27-9230-42ab-a1e8-02cb489ab177",
			Description:    "test-dc2-t2",
			AuthMethod:     "test",
			Local:          true,
			EnterpriseMeta: *defaultEntMeta,
		},
	}, ACLTokenSetOptions{}))

	lastIndex++
	require.NoError(t, s.ACLTokenBatchSet(lastIndex, structs.ACLTokens{
		&structs.ACLToken{
			AccessorID:     methodDC1_tok1,
			SecretID:       "7a1950c6-79dc-441c-acd2-e22cd3db0240",
			Description:    "test-dc1-t1",
			AuthMethod:     "test",
			Local:          false,
			EnterpriseMeta: *defaultEntMeta,
		},
		&structs.ACLToken{
			AccessorID:     methodDC1_tok2,
			SecretID:       "442cee4c-353f-4957-adbb-33db2f9e267f",
			Description:    "test-dc1-t2",
			AuthMethod:     "test",
			Local:          false,
			EnterpriseMeta: *defaultEntMeta,
		},
	}, ACLTokenSetOptions{FromReplication: true}))

	toList := func(tokens structs.ACLTokens) []string {
		var ret []string
		for _, tok := range tokens {
			ret = append(ret, tok.AccessorID)
		}
		return ret
	}

	require.True(t, t.Run("list local only", func(t *testing.T) {
		_, got, err := s.ACLTokenList(nil, true, false, "", "", "test", defaultEntMeta, defaultEntMeta)
		require.NoError(t, err)
		require.ElementsMatch(t, []string{methodDC2_tok1, methodDC2_tok2}, toList(got))
	}))
	require.True(t, t.Run("list global only", func(t *testing.T) {
		_, got, err := s.ACLTokenList(nil, false, true, "", "", "test", defaultEntMeta, defaultEntMeta)
		require.NoError(t, err)
		require.ElementsMatch(t, []string{methodDC1_tok1, methodDC1_tok2}, toList(got))
	}))
	require.True(t, t.Run("list both", func(t *testing.T) {
		_, got, err := s.ACLTokenList(nil, true, true, "", "", "test", defaultEntMeta, defaultEntMeta)
		require.NoError(t, err)
		require.ElementsMatch(t, []string{methodDC1_tok1, methodDC1_tok2, methodDC2_tok1, methodDC2_tok2}, toList(got))
	}))

	lastIndex++
	require.True(t, t.Run("delete dc2 auth method", func(t *testing.T) {
		require.NoError(t, s.ACLAuthMethodDeleteByName(lastIndex, "test", nil))
	}))

	require.True(t, t.Run("list local only (after dc2 delete)", func(t *testing.T) {
		_, got, err := s.ACLTokenList(nil, true, false, "", "", "test", defaultEntMeta, defaultEntMeta)
		require.NoError(t, err)
		require.Empty(t, got)
	}))
	require.True(t, t.Run("list global only (after dc2 delete)", func(t *testing.T) {
		_, got, err := s.ACLTokenList(nil, false, true, "", "", "test", defaultEntMeta, defaultEntMeta)
		require.NoError(t, err)
		require.ElementsMatch(t, []string{methodDC1_tok1, methodDC1_tok2}, toList(got))
	}))
	require.True(t, t.Run("list both (after dc2 delete)", func(t *testing.T) {
		_, got, err := s.ACLTokenList(nil, true, true, "", "", "test", defaultEntMeta, defaultEntMeta)
		require.NoError(t, err)
		require.ElementsMatch(t, []string{methodDC1_tok1, methodDC1_tok2}, toList(got))
	}))
}

func TestStateStore_ACLAuthMethods_UpsertBatchRead(t *testing.T) {
	t.Parallel()

	t.Run("Normal", func(t *testing.T) {
		t.Parallel()
		s := testACLStateStore(t)

		methods := structs.ACLAuthMethods{
			&structs.ACLAuthMethod{
				Name:        "test-1",
				Type:        "testing",
				Description: "test-1",
			},
			&structs.ACLAuthMethod{
				Name:        "test-2",
				Type:        "testing",
				Description: "test-1",
			},
		}

		require.NoError(t, s.ACLAuthMethodBatchSet(2, methods))

		idx, rmethods, err := s.ACLAuthMethodList(nil, nil)
		require.NoError(t, err)
		require.Equal(t, uint64(2), idx)
		require.Len(t, rmethods, 2)
		rmethods.Sort()
		require.ElementsMatch(t, methods, rmethods)
		require.Equal(t, uint64(2), rmethods[0].CreateIndex)
		require.Equal(t, uint64(2), rmethods[0].ModifyIndex)
		require.Equal(t, uint64(2), rmethods[1].CreateIndex)
		require.Equal(t, uint64(2), rmethods[1].ModifyIndex)
	})

	t.Run("Update", func(t *testing.T) {
		t.Parallel()
		s := testACLStateStore(t)

		// Seed initial data.
		methods := structs.ACLAuthMethods{
			&structs.ACLAuthMethod{
				Name:        "test-1",
				Type:        "testing",
				Description: "test-1",
			},
			&structs.ACLAuthMethod{
				Name:        "test-2",
				Type:        "testing",
				Description: "test-2",
			},
		}

		require.NoError(t, s.ACLAuthMethodBatchSet(2, methods))

		// Update two methods at the same time.
		updates := structs.ACLAuthMethods{
			&structs.ACLAuthMethod{
				Name:        "test-1",
				Type:        "testing",
				Description: "test-1 modified",
				Config: map[string]interface{}{
					"Host": "https://localhost:8443",
				},
			},
			&structs.ACLAuthMethod{
				Name:        "test-2",
				Type:        "testing",
				Description: "test-2 modified",
				Config: map[string]interface{}{
					"Host": "https://localhost:8444",
				},
			},
		}

		require.NoError(t, s.ACLAuthMethodBatchSet(3, updates))

		idx, rmethods, err := s.ACLAuthMethodList(nil, nil)
		require.NoError(t, err)
		require.Equal(t, uint64(3), idx)
		require.Len(t, rmethods, 2)
		rmethods.Sort()
		require.ElementsMatch(t, updates, rmethods)
		require.Equal(t, uint64(2), rmethods[0].CreateIndex)
		require.Equal(t, uint64(3), rmethods[0].ModifyIndex)
		require.Equal(t, uint64(2), rmethods[1].CreateIndex)
		require.Equal(t, uint64(3), rmethods[1].ModifyIndex)
	})
}

func TestStateStore_ACLAuthMethod_List(t *testing.T) {
	t.Parallel()
	s := testACLStateStore(t)

	methods := structs.ACLAuthMethods{
		&structs.ACLAuthMethod{
			Name:        "test-1",
			Type:        "testing",
			Description: "test-1",
		},
		&structs.ACLAuthMethod{
			Name:        "test-2",
			Type:        "testing",
			Description: "test-2",
		},
	}

	require.NoError(t, s.ACLAuthMethodBatchSet(2, methods))

	_, rmethods, err := s.ACLAuthMethodList(nil, nil)
	require.NoError(t, err)

	require.Len(t, rmethods, 2)
	rmethods.Sort()

	require.Equal(t, "test-1", rmethods[0].Name)
	require.Equal(t, "testing", rmethods[0].Type)
	require.Equal(t, "test-1", rmethods[0].Description)
	require.Equal(t, uint64(2), rmethods[0].CreateIndex)
	require.Equal(t, uint64(2), rmethods[0].ModifyIndex)

	require.Equal(t, "test-2", rmethods[1].Name)
	require.Equal(t, "testing", rmethods[1].Type)
	require.Equal(t, "test-2", rmethods[1].Description)
	require.Equal(t, uint64(2), rmethods[1].CreateIndex)
	require.Equal(t, uint64(2), rmethods[1].ModifyIndex)
}

func TestStateStore_ACLAuthMethod_Delete(t *testing.T) {
	t.Parallel()

	t.Run("Name", func(t *testing.T) {
		t.Parallel()
		s := testACLStateStore(t)

		method := structs.ACLAuthMethod{
			Name:        "test",
			Type:        "testing",
			Description: "test",
		}

		require.NoError(t, s.ACLAuthMethodSet(2, &method))

		_, rmethod, err := s.ACLAuthMethodGetByName(nil, "test", nil)
		require.NoError(t, err)
		require.NotNil(t, rmethod)

		require.NoError(t, s.ACLAuthMethodDeleteByName(3, "test", nil))
		require.NoError(t, err)

		_, rmethod, err = s.ACLAuthMethodGetByName(nil, "test", nil)
		require.NoError(t, err)
		require.Nil(t, rmethod)
	})

	t.Run("Multiple", func(t *testing.T) {
		t.Parallel()
		s := testACLStateStore(t)

		methods := structs.ACLAuthMethods{
			&structs.ACLAuthMethod{
				Name:        "test-1",
				Type:        "testing",
				Description: "test-1",
			},
			&structs.ACLAuthMethod{
				Name:        "test-2",
				Type:        "testing",
				Description: "test-2",
			},
		}

		require.NoError(t, s.ACLAuthMethodBatchSet(2, methods))

		_, rmethod, err := s.ACLAuthMethodGetByName(nil, "test-1", nil)
		require.NoError(t, err)
		require.NotNil(t, rmethod)
		_, rmethod, err = s.ACLAuthMethodGetByName(nil, "test-2", nil)
		require.NoError(t, err)
		require.NotNil(t, rmethod)

		require.NoError(t, s.ACLAuthMethodBatchDelete(3, []string{"test-1", "test-2"}, nil))

		_, rmethod, err = s.ACLAuthMethodGetByName(nil, "test-1", nil)
		require.NoError(t, err)
		require.Nil(t, rmethod)
		_, rmethod, err = s.ACLAuthMethodGetByName(nil, "test-2", nil)
		require.NoError(t, err)
		require.Nil(t, rmethod)
	})

	t.Run("Not Found", func(t *testing.T) {
		t.Parallel()
		s := testACLStateStore(t)

		// deletion of non-existent methods is not an error
		require.NoError(t, s.ACLAuthMethodDeleteByName(3, "not-found", nil))
	})
}

// Deleting an auth method atomically deletes all rules and tokens as well.
func TestStateStore_ACLAuthMethod_Delete_RuleAndTokenCascade(t *testing.T) {
	t.Parallel()

	s := testACLStateStore(t)

	methods := structs.ACLAuthMethods{
		&structs.ACLAuthMethod{
			Name:        "test-1",
			Type:        "testing",
			Description: "test-1",
		},
		&structs.ACLAuthMethod{
			Name:        "test-2",
			Type:        "testing",
			Description: "test-2",
		},
	}
	require.NoError(t, s.ACLAuthMethodBatchSet(2, methods))

	const (
		method1_rule1 = "dff6f8a3-0115-4b22-8661-04a497ebb23c"
		method1_rule2 = "69e2d304-703d-4889-bd94-4a720c061fc3"
		method2_rule1 = "997ee45c-d6ba-4da1-a98e-aaa012e7d1e2"
		method2_rule2 = "9ebae132-f1f1-4b72-b1d9-a4313ac22075"
	)

	rules := structs.ACLBindingRules{
		&structs.ACLBindingRule{
			ID:          method1_rule1,
			AuthMethod:  "test-1",
			Description: "test-m1-r1",
		},
		&structs.ACLBindingRule{
			ID:          method1_rule2,
			AuthMethod:  "test-1",
			Description: "test-m1-r2",
		},
		&structs.ACLBindingRule{
			ID:          method2_rule1,
			AuthMethod:  "test-2",
			Description: "test-m2-r1",
		},
		&structs.ACLBindingRule{
			ID:          method2_rule2,
			AuthMethod:  "test-2",
			Description: "test-m2-r2",
		},
	}
	require.NoError(t, s.ACLBindingRuleBatchSet(3, rules))

	const ( // accessors
		method1_tok1 = "6d020c5d-c4fd-4348-ba79-beac37ed0b9c"
		method1_tok2 = "169160dc-34ab-45c6-aba7-ff65e9ace9cb"
		method2_tok1 = "8e14628e-7dde-4573-aca1-6386c0f2095d"
		method2_tok2 = "291e5af9-c68e-4dd3-8824-b2bdfdcc89e6"
	)

	tokens := structs.ACLTokens{
		&structs.ACLToken{
			AccessorID:  method1_tok1,
			SecretID:    "7a1950c6-79dc-441c-acd2-e22cd3db0240",
			Description: "test-m1-t1",
			AuthMethod:  "test-1",
			Local:       true,
		},
		&structs.ACLToken{
			AccessorID:  method1_tok2,
			SecretID:    "442cee4c-353f-4957-adbb-33db2f9e267f",
			Description: "test-m1-t2",
			AuthMethod:  "test-1",
			Local:       true,
		},
		&structs.ACLToken{
			AccessorID:  method2_tok1,
			SecretID:    "d9399b7d-6c34-46bd-a675-c1352fadb6fd",
			Description: "test-m2-t1",
			AuthMethod:  "test-2",
			Local:       true,
		},
		&structs.ACLToken{
			AccessorID:  method2_tok2,
			SecretID:    "3b72fc27-9230-42ab-a1e8-02cb489ab177",
			Description: "test-m2-t2",
			AuthMethod:  "test-2",
			Local:       true,
		},
	}
	require.NoError(t, s.ACLTokenBatchSet(4, tokens, ACLTokenSetOptions{}))

	// Delete one method.
	require.NoError(t, s.ACLAuthMethodDeleteByName(4, "test-1", nil))

	// Make sure the method is gone.
	_, rmethod, err := s.ACLAuthMethodGetByName(nil, "test-1", nil)
	require.NoError(t, err)
	require.Nil(t, rmethod)

	// Make sure the rules and tokens are gone.
	for _, ruleID := range []string{method1_rule1, method1_rule2} {
		_, rrule, err := s.ACLBindingRuleGetByID(nil, ruleID, nil)
		require.NoError(t, err)
		require.Nil(t, rrule)
	}
	for _, tokID := range []string{method1_tok1, method1_tok2} {
		_, tok, err := s.ACLTokenGetByAccessor(nil, tokID, nil)
		require.NoError(t, err)
		require.Nil(t, tok)
	}

	// Make sure the rules and tokens for the untouched method are still there.
	for _, ruleID := range []string{method2_rule1, method2_rule2} {
		_, rrule, err := s.ACLBindingRuleGetByID(nil, ruleID, nil)
		require.NoError(t, err)
		require.NotNil(t, rrule)
	}
	for _, tokID := range []string{method2_tok1, method2_tok2} {
		_, tok, err := s.ACLTokenGetByAccessor(nil, tokID, nil)
		require.NoError(t, err)
		require.NotNil(t, tok)
	}
}

func TestStateStore_ACLBindingRule_SetGet(t *testing.T) {
	t.Parallel()

	// The state store only validates key pieces of data, so we only have to
	// care about filling in ID+AuthMethod.

	t.Run("Missing ID", func(t *testing.T) {
		t.Parallel()
		s := testACLStateStore(t)
		setupExtraAuthMethods(t, s)

		rule := structs.ACLBindingRule{
			ID:          "",
			AuthMethod:  "test",
			Description: "test",
		}

		require.Error(t, s.ACLBindingRuleSet(3, &rule))
	})

	t.Run("Missing AuthMethod", func(t *testing.T) {
		t.Parallel()
		s := testACLStateStore(t)
		setupExtraAuthMethods(t, s)

		rule := structs.ACLBindingRule{
			ID:          "9669b2d7-455c-4d70-b0ac-457fd7969a2e",
			AuthMethod:  "",
			Description: "test",
		}

		require.Error(t, s.ACLBindingRuleSet(3, &rule))
	})

	t.Run("Unknown AuthMethod", func(t *testing.T) {
		t.Parallel()
		s := testACLStateStore(t)
		setupExtraAuthMethods(t, s)

		rule := structs.ACLBindingRule{
			ID:          "9669b2d7-455c-4d70-b0ac-457fd7969a2e",
			AuthMethod:  "unknown",
			Description: "test",
		}

		require.Error(t, s.ACLBindingRuleSet(3, &rule))
	})

	t.Run("New", func(t *testing.T) {
		t.Parallel()
		s := testACLStateStore(t)
		setupExtraAuthMethods(t, s)

		rule := structs.ACLBindingRule{
			ID:          "9669b2d7-455c-4d70-b0ac-457fd7969a2e",
			AuthMethod:  "test",
			Description: "test",
		}

		require.NoError(t, s.ACLBindingRuleSet(3, &rule))

		idx, rrule, err := s.ACLBindingRuleGetByID(nil, rule.ID, nil)
		require.NoError(t, err)
		require.Equal(t, uint64(3), idx)
		require.NotNil(t, rrule)
		require.Equal(t, rule.ID, rrule.ID)
		require.Equal(t, "test", rrule.AuthMethod)
		require.Equal(t, "test", rrule.Description)
		require.Equal(t, uint64(3), rrule.CreateIndex)
		require.Equal(t, uint64(3), rrule.ModifyIndex)
	})

	t.Run("Update", func(t *testing.T) {
		t.Parallel()
		s := testACLStateStore(t)
		setupExtraAuthMethods(t, s)

		// Create the initial rule
		rule := structs.ACLBindingRule{
			ID:          "9669b2d7-455c-4d70-b0ac-457fd7969a2e",
			AuthMethod:  "test",
			Description: "test",
		}

		require.NoError(t, s.ACLBindingRuleSet(2, &rule))

		// Now make sure we can update it
		update := structs.ACLBindingRule{
			ID:          "9669b2d7-455c-4d70-b0ac-457fd7969a2e",
			AuthMethod:  "test",
			Description: "modified",
			BindType:    structs.BindingRuleBindTypeService,
			BindName:    "web",
		}

		require.NoError(t, s.ACLBindingRuleSet(3, &update))

		idx, rrule, err := s.ACLBindingRuleGetByID(nil, rule.ID, nil)
		require.NoError(t, err)
		require.Equal(t, uint64(3), idx)
		require.NotNil(t, rrule)
		require.Equal(t, rule.ID, rrule.ID)
		require.Equal(t, "test", rrule.AuthMethod)
		require.Equal(t, "modified", rrule.Description)
		require.Equal(t, structs.BindingRuleBindTypeService, rrule.BindType)
		require.Equal(t, "web", rrule.BindName)
		require.Equal(t, uint64(2), rrule.CreateIndex)
		require.Equal(t, uint64(3), rrule.ModifyIndex)
	})
}

func TestStateStore_ACLBindingRules_UpsertBatchRead(t *testing.T) {
	t.Parallel()

	t.Run("Normal", func(t *testing.T) {
		t.Parallel()
		s := testACLStateStore(t)
		setupExtraAuthMethods(t, s)

		rules := structs.ACLBindingRules{
			&structs.ACLBindingRule{
				ID:          "9669b2d7-455c-4d70-b0ac-457fd7969a2e",
				AuthMethod:  "test",
				Description: "test-1",
			},
			&structs.ACLBindingRule{
				ID:          "3ebcc27b-f8ba-4611-b385-79a065dfb983",
				AuthMethod:  "test",
				Description: "test-2",
			},
		}

		require.NoError(t, s.ACLBindingRuleBatchSet(2, rules))

		idx, rrules, err := s.ACLBindingRuleList(nil, "test", nil)
		require.NoError(t, err)
		require.Equal(t, uint64(2), idx)
		require.Len(t, rrules, 2)
		rrules.Sort()
		require.ElementsMatch(t, rules, rrules)
		require.Equal(t, uint64(2), rrules[0].CreateIndex)
		require.Equal(t, uint64(2), rrules[0].ModifyIndex)
		require.Equal(t, uint64(2), rrules[1].CreateIndex)
		require.Equal(t, uint64(2), rrules[1].ModifyIndex)
	})

	t.Run("Update", func(t *testing.T) {
		t.Parallel()
		s := testACLStateStore(t)
		setupExtraAuthMethods(t, s)

		// Seed initial data.
		rules := structs.ACLBindingRules{
			&structs.ACLBindingRule{
				ID:          "9669b2d7-455c-4d70-b0ac-457fd7969a2e",
				AuthMethod:  "test",
				Description: "test-1",
			},
			&structs.ACLBindingRule{
				ID:          "3ebcc27b-f8ba-4611-b385-79a065dfb983",
				AuthMethod:  "test",
				Description: "test-2",
			},
		}

		require.NoError(t, s.ACLBindingRuleBatchSet(2, rules))

		// Update two rules at the same time.
		updates := structs.ACLBindingRules{
			&structs.ACLBindingRule{
				ID:          "9669b2d7-455c-4d70-b0ac-457fd7969a2e",
				AuthMethod:  "test",
				Description: "test-1 modified",
				BindType:    structs.BindingRuleBindTypeService,
				BindName:    "web-1",
			},
			&structs.ACLBindingRule{
				ID:          "3ebcc27b-f8ba-4611-b385-79a065dfb983",
				AuthMethod:  "test",
				Description: "test-2 modified",
				BindType:    structs.BindingRuleBindTypeService,
				BindName:    "web-2",
			},
		}

		require.NoError(t, s.ACLBindingRuleBatchSet(3, updates))

		idx, rrules, err := s.ACLBindingRuleList(nil, "test", nil)
		require.NoError(t, err)
		require.Equal(t, uint64(3), idx)
		require.Len(t, rrules, 2)
		rrules.Sort()
		require.ElementsMatch(t, updates, rrules)
		require.Equal(t, uint64(2), rrules[0].CreateIndex)
		require.Equal(t, uint64(3), rrules[0].ModifyIndex)
		require.Equal(t, uint64(2), rrules[1].CreateIndex)
		require.Equal(t, uint64(3), rrules[1].ModifyIndex)
	})
}

func TestStateStore_ACLBindingRule_List(t *testing.T) {
	t.Parallel()
	s := testACLStateStore(t)
	setupExtraAuthMethods(t, s)

	rules := structs.ACLBindingRules{
		&structs.ACLBindingRule{
			ID:          "3ebcc27b-f8ba-4611-b385-79a065dfb983",
			AuthMethod:  "test",
			Description: "test-1",
		},
		&structs.ACLBindingRule{
			ID:          "9669b2d7-455c-4d70-b0ac-457fd7969a2e",
			AuthMethod:  "test",
			Description: "test-2",
		},
	}

	require.NoError(t, s.ACLBindingRuleBatchSet(2, rules))

	_, rrules, err := s.ACLBindingRuleList(nil, "", nil)
	require.NoError(t, err)

	require.Len(t, rrules, 2)
	rrules.Sort()

	require.Equal(t, "3ebcc27b-f8ba-4611-b385-79a065dfb983", rrules[0].ID)
	require.Equal(t, "test", rrules[0].AuthMethod)
	require.Equal(t, "test-1", rrules[0].Description)
	require.Equal(t, uint64(2), rrules[0].CreateIndex)
	require.Equal(t, uint64(2), rrules[0].ModifyIndex)

	require.Equal(t, "9669b2d7-455c-4d70-b0ac-457fd7969a2e", rrules[1].ID)
	require.Equal(t, "test", rrules[1].AuthMethod)
	require.Equal(t, "test-2", rrules[1].Description)
	require.Equal(t, uint64(2), rrules[1].CreateIndex)
	require.Equal(t, uint64(2), rrules[1].ModifyIndex)
}

func TestStateStore_ACLBindingRule_Delete(t *testing.T) {
	t.Parallel()

	t.Run("Name", func(t *testing.T) {
		t.Parallel()
		s := testACLStateStore(t)
		setupExtraAuthMethods(t, s)

		rule := structs.ACLBindingRule{
			ID:          "9669b2d7-455c-4d70-b0ac-457fd7969a2e",
			AuthMethod:  "test",
			Description: "test",
		}

		require.NoError(t, s.ACLBindingRuleSet(2, &rule))

		_, rrule, err := s.ACLBindingRuleGetByID(nil, rule.ID, nil)
		require.NoError(t, err)
		require.NotNil(t, rrule)

		require.NoError(t, s.ACLBindingRuleDeleteByID(3, rule.ID, nil))
		require.NoError(t, err)

		_, rrule, err = s.ACLBindingRuleGetByID(nil, rule.ID, nil)
		require.NoError(t, err)
		require.Nil(t, rrule)
	})

	t.Run("Multiple", func(t *testing.T) {
		t.Parallel()
		s := testACLStateStore(t)
		setupExtraAuthMethods(t, s)

		rules := structs.ACLBindingRules{
			&structs.ACLBindingRule{
				ID:          "3ebcc27b-f8ba-4611-b385-79a065dfb983",
				AuthMethod:  "test",
				Description: "test-1",
			},
			&structs.ACLBindingRule{
				ID:          "9669b2d7-455c-4d70-b0ac-457fd7969a2e",
				AuthMethod:  "test",
				Description: "test-2",
			},
		}

		require.NoError(t, s.ACLBindingRuleBatchSet(2, rules))

		_, rrule, err := s.ACLBindingRuleGetByID(nil, rules[0].ID, nil)
		require.NoError(t, err)
		require.NotNil(t, rrule)
		_, rrule, err = s.ACLBindingRuleGetByID(nil, rules[1].ID, nil)
		require.NoError(t, err)
		require.NotNil(t, rrule)

		require.NoError(t, s.ACLBindingRuleBatchDelete(3, []string{rules[0].ID, rules[1].ID}))

		_, rrule, err = s.ACLBindingRuleGetByID(nil, rules[0].ID, nil)
		require.NoError(t, err)
		require.Nil(t, rrule)
		_, rrule, err = s.ACLBindingRuleGetByID(nil, rules[1].ID, nil)
		require.NoError(t, err)
		require.Nil(t, rrule)
	})

	t.Run("Not Found", func(t *testing.T) {
		t.Parallel()
		s := testACLStateStore(t)

		// deletion of non-existent rules is not an error
		require.NoError(t, s.ACLBindingRuleDeleteByID(3, "ed3ce1b8-3a16-4e2f-b82e-f92e3b92410d", nil))
	})
}

func TestStateStore_ACLTokens_Snapshot_Restore(t *testing.T) {
	s := testStateStore(t)

	policies := structs.ACLPolicies{
		&structs.ACLPolicy{
			ID:          "ca1fc52c-3676-4050-82ed-ca223e38b2c9",
			Name:        "policy1",
			Description: "policy1",
			Rules:       `node_prefix "" { policy = "read" }`,
			Syntax:      acl.SyntaxCurrent,
		},
		&structs.ACLPolicy{
			ID:          "7b70fa0f-58cd-412d-93c3-a0f17bb19a3e",
			Name:        "policy2",
			Description: "policy2",
			Rules:       `acl = "read"`,
			Syntax:      acl.SyntaxCurrent,
		},
	}

	for _, policy := range policies {
		policy.SetHash(true)
	}

	require.NoError(t, s.ACLPolicyBatchSet(2, policies))

	roles := structs.ACLRoles{
		&structs.ACLRole{
			ID:          "1a3a9af9-9cdc-473a-8016-010067b7e424",
			Name:        "role1",
			Description: "role1",
			Policies: []structs.ACLRolePolicyLink{
				{
					ID: "ca1fc52c-3676-4050-82ed-ca223e38b2c9",
				},
			},
		},
		&structs.ACLRole{
			ID:          "4dccc2c7-10f3-4eba-b367-9c09be9a9d67",
			Name:        "role2",
			Description: "role2",
			Policies: []structs.ACLRolePolicyLink{
				{
					ID: "7b70fa0f-58cd-412d-93c3-a0f17bb19a3e",
				},
			},
		},
	}

	for _, role := range roles {
		role.SetHash(true)
	}

	require.NoError(t, s.ACLRoleBatchSet(3, roles, false))

	tokens := structs.ACLTokens{
		&structs.ACLToken{
			AccessorID:  "68016c3d-835b-450c-a6f9-75db9ba740be",
			SecretID:    "838f72b5-5c15-4a9e-aa6d-31734c3a0286",
			Description: "token1",
			Policies: []structs.ACLTokenPolicyLink{
				{
					ID:   "ca1fc52c-3676-4050-82ed-ca223e38b2c9",
					Name: "policy1",
				},
				{
					ID:   "7b70fa0f-58cd-412d-93c3-a0f17bb19a3e",
					Name: "policy2",
				},
			},
			Roles: []structs.ACLTokenRoleLink{
				{
					ID:   "1a3a9af9-9cdc-473a-8016-010067b7e424",
					Name: "role1",
				},
				{
					ID:   "4dccc2c7-10f3-4eba-b367-9c09be9a9d67",
					Name: "role2",
				},
			},
		},
		&structs.ACLToken{
			AccessorID:  "b2125a1b-2a52-41d4-88f3-c58761998a46",
			SecretID:    "ba5d9239-a4ab-49b9-ae09-1f19eed92204",
			Description: "token2",
			Policies: []structs.ACLTokenPolicyLink{
				{
					ID:   "ca1fc52c-3676-4050-82ed-ca223e38b2c9",
					Name: "policy1",
				},
				{
					ID:   "7b70fa0f-58cd-412d-93c3-a0f17bb19a3e",
					Name: "policy2",
				},
			},
			Roles: []structs.ACLTokenRoleLink{
				{
					ID:   "1a3a9af9-9cdc-473a-8016-010067b7e424",
					Name: "role1",
				},
				{
					ID:   "4dccc2c7-10f3-4eba-b367-9c09be9a9d67",
					Name: "role2",
				},
			},
			Hash:      []byte{1, 2, 3, 4},
			RaftIndex: structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
		},
	}

	require.NoError(t, s.ACLTokenBatchSet(4, tokens, ACLTokenSetOptions{}))

	// Snapshot the ACLs.
	snap := s.Snapshot()
	defer snap.Close()

	// Alter the real state store.
	require.NoError(t, s.ACLTokenDeleteByAccessor(3, tokens[0].AccessorID, nil))

	// Verify the snapshot.
	require.Equal(t, uint64(4), snap.LastIndex())

	iter, err := snap.ACLTokens()
	require.NoError(t, err)

	var dump structs.ACLTokens
	for token := iter.Next(); token != nil; token = iter.Next() {
		dump = append(dump, token.(*structs.ACLToken))
	}
	require.ElementsMatch(t, dump, tokens)

	indexes, err := snapshotIndexes(snap)
	require.NoError(t, err)

	// Restore the values into a new state store.
	func() {
		s := testStateStore(t)
		restore := s.Restore()
		for _, token := range dump {
			require.NoError(t, restore.ACLToken(token))
		}
		require.NoError(t, restoreIndexes(indexes, restore))
		restore.Commit()

		// need to ensure we have the policies or else the links will be removed
		require.NoError(t, s.ACLPolicyBatchSet(2, policies))

		// need to ensure we have the roles or else the links will be removed
		require.NoError(t, s.ACLRoleBatchSet(2, roles, false))

		// Read the restored ACLs back out and verify that they match.
		idx, res, err := s.ACLTokenList(nil, true, true, "", "", "", nil, nil)
		require.NoError(t, err)
		require.Equal(t, uint64(4), idx)
		require.ElementsMatch(t, tokens, res)
		require.Equal(t, uint64(4), s.maxIndex(tableACLTokens))
	}()
}

func TestStateStore_ACLPolicies_Snapshot_Restore(t *testing.T) {
	s := testStateStore(t)

	policies := structs.ACLPolicies{
		&structs.ACLPolicy{
			ID:          "68016c3d-835b-450c-a6f9-75db9ba740be",
			Name:        "838f72b5-5c15-4a9e-aa6d-31734c3a0286",
			Description: "policy1",
			Rules:       `acl = "read"`,
			Hash:        []byte{1, 2, 3, 4},
			RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
		},
		&structs.ACLPolicy{
			ID:          "b2125a1b-2a52-41d4-88f3-c58761998a46",
			Name:        "ba5d9239-a4ab-49b9-ae09-1f19eed92204",
			Description: "policy2",
			Rules:       `operator = "read"`,
			Hash:        []byte{1, 2, 3, 4},
			RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
		},
	}

	require.NoError(t, s.ACLPolicyBatchSet(2, policies))

	// Snapshot the ACLs.
	snap := s.Snapshot()
	defer snap.Close()

	// Alter the real state store.
	require.NoError(t, s.ACLPolicyDeleteByID(3, policies[0].ID, nil))

	// Verify the snapshot.
	require.Equal(t, uint64(2), snap.LastIndex())

	iter, err := snap.ACLPolicies()
	require.NoError(t, err)

	var dump structs.ACLPolicies
	for policy := iter.Next(); policy != nil; policy = iter.Next() {
		dump = append(dump, policy.(*structs.ACLPolicy))
	}
	require.ElementsMatch(t, dump, policies)

	indexes, err := snapshotIndexes(snap)
	require.NoError(t, err)

	// Restore the values into a new state store.
	func() {
		s := testStateStore(t)
		restore := s.Restore()
		for _, policy := range dump {
			require.NoError(t, restore.ACLPolicy(policy))
		}
		require.NoError(t, restoreIndexes(indexes, restore))
		restore.Commit()

		// Read the restored ACLs back out and verify that they match.
		idx, res, err := s.ACLPolicyList(nil, nil)
		require.NoError(t, err)
		require.Equal(t, uint64(2), idx)
		require.ElementsMatch(t, policies, res)
		require.Equal(t, uint64(2), s.maxIndex(tableACLPolicies))
	}()
}

func TestTokenPoliciesIndex(t *testing.T) {

	idIndex := &memdb.IndexSchema{
		Name:         "id",
		AllowMissing: false,
		Unique:       true,
		Indexer:      &memdb.StringFieldIndex{Field: "AccessorID", Lowercase: false},
	}
	globalIndex := &memdb.IndexSchema{
		Name:         "global",
		AllowMissing: true,
		Unique:       false,
		Indexer: indexerSingle[*TimeQuery, *structs.ACLToken]{
			readIndex:  indexFromTimeQuery,
			writeIndex: indexExpiresGlobalFromACLToken,
		},
	}
	localIndex := &memdb.IndexSchema{
		Name:         "local",
		AllowMissing: true,
		Unique:       false,
		Indexer: indexerSingle[*TimeQuery, *structs.ACLToken]{
			readIndex:  indexFromTimeQuery,
			writeIndex: indexExpiresLocalFromACLToken,
		},
	}
	schema := &memdb.DBSchema{
		Tables: map[string]*memdb.TableSchema{
			"test": {
				Name: "test",
				Indexes: map[string]*memdb.IndexSchema{
					"id":     idIndex,
					"global": globalIndex,
					"local":  localIndex,
				},
			},
		},
	}

	knownUUIDs := make(map[string]struct{})
	newUUID := func() string {
		for {
			ret, err := uuid.GenerateUUID()
			require.NoError(t, err)
			if _, ok := knownUUIDs[ret]; !ok {
				knownUUIDs[ret] = struct{}{}
				return ret
			}
		}
	}

	baseTime := time.Date(2010, 12, 31, 11, 30, 7, 0, time.UTC)

	newToken := func(local bool, desc string, expTime time.Time) *structs.ACLToken {
		return &structs.ACLToken{
			AccessorID:     newUUID(),
			SecretID:       newUUID(),
			Description:    desc,
			Local:          local,
			ExpirationTime: &expTime,
			CreateTime:     baseTime,
			RaftIndex: structs.RaftIndex{
				CreateIndex: 9,
				ModifyIndex: 10,
			},
		}
	}

	db, err := memdb.NewMemDB(schema)
	require.NoError(t, err)

	dumpItems := func(index string) ([]string, error) {
		tx := db.Txn(false)
		defer tx.Abort()

		iter, err := tx.Get("test", index)
		if err != nil {
			return nil, err
		}

		var out []string
		for raw := iter.Next(); raw != nil; raw = iter.Next() {
			tok := raw.(*structs.ACLToken)
			out = append(out, tok.Description)
		}

		return out, nil
	}

	{ // insert things with no expiration time
		tx := db.Txn(true)
		for i := 0; i < 10; i++ {
			tok := newToken(i%2 != 1, "tok["+strconv.Itoa(i)+"]", time.Time{})

			require.NoError(t, tx.Insert("test", tok))
		}
		tx.Commit()
	}

	t.Run("no expiration", func(t *testing.T) {
		dump, err := dumpItems("local")
		require.NoError(t, err)
		require.Len(t, dump, 0)

		dump, err = dumpItems("global")
		require.NoError(t, err)
		require.Len(t, dump, 0)
	})

	{ // insert things with laddered expiration time, inserted in random order
		var tokens []*structs.ACLToken
		for i := 0; i < 10; i++ {
			expTime := baseTime.Add(time.Duration(i+1) * time.Minute)
			tok := newToken(i%2 == 0, "exp-tok["+strconv.Itoa(i)+"]", expTime)
			tokens = append(tokens, tok)
		}
		rand.Shuffle(len(tokens), func(i, j int) {
			tokens[i], tokens[j] = tokens[j], tokens[i]
		})

		tx := db.Txn(true)
		for _, tok := range tokens {
			require.NoError(t, tx.Insert("test", tok))
		}
		tx.Commit()
	}

	t.Run("mixed expiration", func(t *testing.T) {
		dump, err := dumpItems("local")
		require.NoError(t, err)
		require.ElementsMatch(t, []string{
			"exp-tok[0]",
			"exp-tok[2]",
			"exp-tok[4]",
			"exp-tok[6]",
			"exp-tok[8]",
		}, dump)

		dump, err = dumpItems("global")
		require.NoError(t, err)
		require.ElementsMatch(t, []string{
			"exp-tok[1]",
			"exp-tok[3]",
			"exp-tok[5]",
			"exp-tok[7]",
			"exp-tok[9]",
		}, dump)
	})
}

func stripIrrelevantTokenFields(token *structs.ACLToken) *structs.ACLToken {
	tokenCopy := token.Clone()
	// When comparing the tokens disregard the policy link names.  This
	// data is not cleanly updated in a variety of scenarios and should not
	// be relied upon.
	for i := range tokenCopy.Policies {
		tokenCopy.Policies[i].Name = ""
	}
	// Also do the same for Role links.
	for i := range tokenCopy.Roles {
		tokenCopy.Roles[i].Name = ""
	}
	// The raft indexes won't match either because the requester will not
	// have access to that.
	tokenCopy.RaftIndex = structs.RaftIndex{}

	// nil out the hash - this is a computed field and we should assert
	// elsewhere that its not empty when expected
	tokenCopy.Hash = nil
	return tokenCopy
}

func compareTokens(t *testing.T, expected, actual *structs.ACLToken) {
	require.Equal(t, stripIrrelevantTokenFields(expected), stripIrrelevantTokenFields(actual))
}

func TestStateStore_ACLRoles_Snapshot_Restore(t *testing.T) {
	s := testStateStore(t)

	policies := structs.ACLPolicies{
		&structs.ACLPolicy{
			ID:          "ca1fc52c-3676-4050-82ed-ca223e38b2c9",
			Name:        "policy1",
			Description: "policy1",
			Rules:       `node_prefix "" { policy = "read" }`,
			Syntax:      acl.SyntaxCurrent,
		},
		&structs.ACLPolicy{
			ID:          "7b70fa0f-58cd-412d-93c3-a0f17bb19a3e",
			Name:        "policy2",
			Description: "policy2",
			Rules:       `acl = "read"`,
			Syntax:      acl.SyntaxCurrent,
		},
	}

	for _, policy := range policies {
		policy.SetHash(true)
	}

	require.NoError(t, s.ACLPolicyBatchSet(2, policies))

	roles := structs.ACLRoles{
		&structs.ACLRole{
			ID:          "68016c3d-835b-450c-a6f9-75db9ba740be",
			Name:        "838f72b5-5c15-4a9e-aa6d-31734c3a0286",
			Description: "policy1",
			Policies: []structs.ACLRolePolicyLink{
				{
					ID:   "ca1fc52c-3676-4050-82ed-ca223e38b2c9",
					Name: "policy1",
				},
				{
					ID:   "7b70fa0f-58cd-412d-93c3-a0f17bb19a3e",
					Name: "policy2",
				},
			},
			Hash:      []byte{1, 2, 3, 4},
			RaftIndex: structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
		},
		&structs.ACLRole{
			ID:          "b2125a1b-2a52-41d4-88f3-c58761998a46",
			Name:        "ba5d9239-a4ab-49b9-ae09-1f19eed92204",
			Description: "policy2",
			Policies: []structs.ACLRolePolicyLink{
				{
					ID:   "ca1fc52c-3676-4050-82ed-ca223e38b2c9",
					Name: "policy1",
				},
				{
					ID:   "7b70fa0f-58cd-412d-93c3-a0f17bb19a3e",
					Name: "policy2",
				},
			},
			Hash:      []byte{1, 2, 3, 4},
			RaftIndex: structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
		},
	}

	require.NoError(t, s.ACLRoleBatchSet(2, roles, false))

	// Snapshot the ACLs.
	snap := s.Snapshot()
	defer snap.Close()

	// Alter the real state store.
	require.NoError(t, s.ACLRoleDeleteByID(3, roles[0].ID, nil))

	// Verify the snapshot.
	require.Equal(t, uint64(2), snap.LastIndex())

	iter, err := snap.ACLRoles()
	require.NoError(t, err)

	var dump structs.ACLRoles
	for role := iter.Next(); role != nil; role = iter.Next() {
		dump = append(dump, role.(*structs.ACLRole))
	}
	require.ElementsMatch(t, dump, roles)

	indexes, err := snapshotIndexes(snap)
	require.NoError(t, err)

	// Restore the values into a new state store.
	func() {
		s := testStateStore(t)
		restore := s.Restore()
		for _, role := range dump {
			require.NoError(t, restore.ACLRole(role))
		}
		require.NoError(t, restoreIndexes(indexes, restore))
		restore.Commit()

		// need to ensure we have the policies or else the links will be removed
		require.NoError(t, s.ACLPolicyBatchSet(2, policies))

		// Read the restored ACLs back out and verify that they match.
		idx, res, err := s.ACLRoleList(nil, "", nil)
		require.NoError(t, err)
		require.Equal(t, uint64(2), idx)
		require.ElementsMatch(t, roles, res)
		require.Equal(t, uint64(2), s.maxIndex(tableACLRoles))
	}()
}

func TestStateStore_ACLAuthMethods_Snapshot_Restore(t *testing.T) {
	s := testACLStateStore(t)

	methods := structs.ACLAuthMethods{
		&structs.ACLAuthMethod{
			Name:        "test-1",
			Type:        "testing",
			Description: "test-1",
			RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
		},
		&structs.ACLAuthMethod{
			Name:        "test-2",
			Type:        "testing",
			Description: "test-2",
			RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
		},
	}

	require.NoError(t, s.ACLAuthMethodBatchSet(2, methods))

	// Snapshot the ACLs.
	snap := s.Snapshot()
	defer snap.Close()

	// Alter the real state store.
	require.NoError(t, s.ACLAuthMethodDeleteByName(3, "test-1", nil))

	// Verify the snapshot.
	require.Equal(t, uint64(2), snap.LastIndex())

	iter, err := snap.ACLAuthMethods()
	require.NoError(t, err)

	var dump structs.ACLAuthMethods
	for method := iter.Next(); method != nil; method = iter.Next() {
		dump = append(dump, method.(*structs.ACLAuthMethod))
	}
	require.ElementsMatch(t, dump, methods)

	indexes, err := snapshotIndexes(snap)
	require.NoError(t, err)

	// Restore the values into a new state store.
	func() {
		s := testStateStore(t)
		restore := s.Restore()
		for _, method := range dump {
			require.NoError(t, restore.ACLAuthMethod(method))
		}
		require.NoError(t, restoreIndexes(indexes, restore))
		restore.Commit()

		// Read the restored methods back out and verify that they match.
		idx, res, err := s.ACLAuthMethodList(nil, nil)
		require.NoError(t, err)
		require.Equal(t, uint64(2), idx)
		require.ElementsMatch(t, methods, res)
		require.Equal(t, uint64(2), s.maxIndex(tableACLAuthMethods))
	}()
}

func TestStateStore_ACLBindingRules_Snapshot_Restore(t *testing.T) {
	s := testACLStateStore(t)
	setupExtraAuthMethods(t, s)

	rules := structs.ACLBindingRules{
		&structs.ACLBindingRule{
			ID:          "9669b2d7-455c-4d70-b0ac-457fd7969a2e",
			AuthMethod:  "test",
			Description: "test-1",
			RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
		},
		&structs.ACLBindingRule{
			ID:          "3ebcc27b-f8ba-4611-b385-79a065dfb983",
			AuthMethod:  "test",
			Description: "test-2",
			RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
		},
	}

	require.NoError(t, s.ACLBindingRuleBatchSet(2, rules))

	// Snapshot the ACLs.
	snap := s.Snapshot()
	defer snap.Close()

	// Alter the real state store.
	require.NoError(t, s.ACLBindingRuleDeleteByID(3, rules[0].ID, nil))

	// Verify the snapshot.
	require.Equal(t, uint64(2), snap.LastIndex())

	iter, err := snap.ACLBindingRules()
	require.NoError(t, err)

	var dump structs.ACLBindingRules
	for rule := iter.Next(); rule != nil; rule = iter.Next() {
		dump = append(dump, rule.(*structs.ACLBindingRule))
	}
	require.ElementsMatch(t, dump, rules)

	indexes, err := snapshotIndexes(snap)
	require.NoError(t, err)

	// Restore the values into a new state store.
	func() {
		s := testStateStore(t)
		setupExtraAuthMethods(t, s)

		restore := s.Restore()
		for _, rule := range dump {
			require.NoError(t, restore.ACLBindingRule(rule))
		}
		require.NoError(t, restoreIndexes(indexes, restore))
		restore.Commit()

		// Read the restored rules back out and verify that they match.
		idx, res, err := s.ACLBindingRuleList(nil, "", nil)
		require.NoError(t, err)
		require.Equal(t, uint64(2), idx)
		require.ElementsMatch(t, rules, res)
		require.Equal(t, uint64(2), s.maxIndex(tableACLBindingRules))
	}()
}

func TestStateStore_resolveACLLinks(t *testing.T) {
	t.Parallel()

	t.Run("missing link id", func(t *testing.T) {
		t.Parallel()
		s := testStateStore(t)

		tx := s.db.Txn(false)
		defer tx.Abort()

		links := []*pbacl.ACLLink{
			{
				Name: "foo",
			},
		}

		_, err := resolveACLLinks(tx, links, func(ReadTxn, string) (string, error) {
			err := fmt.Errorf("Should not be attempting to resolve an empty id")
			require.Fail(t, err.Error())
			return "", err
		})

		require.Error(t, err)
		require.Contains(t, err.Error(), "Encountered an ACL resource linked by Name in the state store")
	})

	t.Run("typical", func(t *testing.T) {
		t.Parallel()
		s := testStateStore(t)

		tx := s.db.Txn(false)
		defer tx.Abort()

		links := []*pbacl.ACLLink{
			{
				ID: "b985e082-25d3-45a9-9dd8-fd1a41b83b0d",
			},
			{
				ID: "e81887b4-836b-4053-a1fa-7e8305902be9",
			},
		}

		numValid, err := resolveACLLinks(tx, links, func(_ ReadTxn, linkID string) (string, error) {
			switch linkID {
			case "e81887b4-836b-4053-a1fa-7e8305902be9":
				return "foo", nil
			case "b985e082-25d3-45a9-9dd8-fd1a41b83b0d":
				return "bar", nil
			default:
				return "", fmt.Errorf("No such id")
			}
		})

		require.NoError(t, err)
		require.Equal(t, "bar", links[0].Name)
		require.Equal(t, "foo", links[1].Name)
		require.Equal(t, 2, numValid)
	})

	t.Run("unresolvable", func(t *testing.T) {
		t.Parallel()
		s := testStateStore(t)

		tx := s.db.Txn(false)
		defer tx.Abort()

		links := []*pbacl.ACLLink{
			{
				ID: "b985e082-25d3-45a9-9dd8-fd1a41b83b0d",
			},
		}

		numValid, err := resolveACLLinks(tx, links, func(_ ReadTxn, linkID string) (string, error) {
			require.Equal(t, "b985e082-25d3-45a9-9dd8-fd1a41b83b0d", linkID)
			return "", nil
		})

		require.NoError(t, err)
		require.Empty(t, links[0].Name)
		require.Equal(t, 0, numValid)
	})
}

func TestStateStore_fixupACLLinks(t *testing.T) {
	t.Parallel()

	links := []*pbacl.ACLLink{
		{
			ID:   "40b57f86-97ea-40e4-a99a-c399cc81f4dd",
			Name: "foo",
		},
		{
			ID:   "8f024f92-1f8e-42ea-a3c3-55fb0c8670bc",
			Name: "bar",
		},
		{
			ID:   "c91afed1-e474-4cd2-98aa-cd57dd9377e9",
			Name: "baz",
		},
		{
			ID:   "c1585be7-ab0e-4973-b572-ba9afda86e07",
			Name: "four",
		},
	}

	t.Run("unaltered", func(t *testing.T) {
		t.Parallel()
		s := testStateStore(t)

		tx := s.db.Txn(false)
		defer tx.Abort()

		newLinks, cloned, err := fixupACLLinks(tx, links, func(_ ReadTxn, linkID string) (string, error) {
			switch linkID {
			case "40b57f86-97ea-40e4-a99a-c399cc81f4dd":
				return "foo", nil
			case "8f024f92-1f8e-42ea-a3c3-55fb0c8670bc":
				return "bar", nil
			case "c91afed1-e474-4cd2-98aa-cd57dd9377e9":
				return "baz", nil
			case "c1585be7-ab0e-4973-b572-ba9afda86e07":
				return "four", nil
			default:
				return "", nil
			}
		})

		require.NoError(t, err)
		require.False(t, cloned)
		require.Equal(t, links, newLinks)
	})

	t.Run("renamed", func(t *testing.T) {
		t.Parallel()
		s := testStateStore(t)

		tx := s.db.Txn(false)
		defer tx.Abort()

		newLinks, cloned, err := fixupACLLinks(tx, links, func(_ ReadTxn, linkID string) (string, error) {
			switch linkID {
			case "40b57f86-97ea-40e4-a99a-c399cc81f4dd":
				return "foo", nil
			case "8f024f92-1f8e-42ea-a3c3-55fb0c8670bc":
				return "bart", nil
			case "c91afed1-e474-4cd2-98aa-cd57dd9377e9":
				return "bazzy", nil
			case "c1585be7-ab0e-4973-b572-ba9afda86e07":
				return "four", nil
			default:
				return "", nil
			}
		})

		require.NoError(t, err)
		require.True(t, cloned)
		require.Equal(t, links[0], newLinks[0])
		require.Equal(t, links[1].ID, newLinks[1].ID)
		require.Equal(t, "bart", newLinks[1].Name)
		require.Equal(t, links[2].ID, newLinks[2].ID)
		require.Equal(t, "bazzy", newLinks[2].Name)
		require.Equal(t, links[3], newLinks[3])
	})

	t.Run("deleted", func(t *testing.T) {
		t.Parallel()
		s := testStateStore(t)

		tx := s.db.Txn(false)
		defer tx.Abort()

		newLinks, cloned, err := fixupACLLinks(tx, links, func(_ ReadTxn, linkID string) (string, error) {
			switch linkID {
			case "40b57f86-97ea-40e4-a99a-c399cc81f4dd":
				return "foo", nil
			case "c91afed1-e474-4cd2-98aa-cd57dd9377e9":
				return "baz", nil
			case "c1585be7-ab0e-4973-b572-ba9afda86e07":
				return "four", nil
			default:
				return "", nil
			}
		})

		require.NoError(t, err)
		require.True(t, cloned)
		require.Equal(t, links[0], newLinks[0])
		require.Equal(t, links[2], newLinks[1])
		require.Equal(t, links[3], newLinks[2])
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()
		s := testStateStore(t)

		tx := s.db.Txn(false)
		defer tx.Abort()

		_, _, err := fixupACLLinks(tx, links, func(ReadTxn, string) (string, error) {
			return "", fmt.Errorf("Resolver Error")
		})

		require.Error(t, err)
		require.Equal(t, err.Error(), "Resolver Error")
	})
}
