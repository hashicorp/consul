package state

import (
	// "reflect"
	"testing"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	// "github.com/hashicorp/go-memdb"
	// "github.com/pascaldekloe/goe/verify"

	"github.com/stretchr/testify/require"
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
	require.NoError(t, s.ACLTokenSet(1, &token, false))
}

func testACLStateStore(t *testing.T) *Store {
	s := testStateStore(t)
	setupGlobalManagement(t, s)
	setupAnonymous(t, s)
	return s
}

func setupExtraPolicies(t *testing.T, s *Store) {
	policies := structs.ACLPolicies{
		&structs.ACLPolicy{
			ID:          "a0625e95-9b3e-42de-a8d6-ceef5b6f3286",
			Name:        "node-read",
			Description: "Allows reading all node information",
			Rules:       `node_prefix "" { policy = "read" }`,
			Syntax:      acl.SyntaxCurrent,
		},
	}

	for _, policy := range policies {
		policy.SetHash(true)
	}

	require.NoError(t, s.ACLPolicyBatchSet(2, policies))
}

func testACLTokensStateStore(t *testing.T) *Store {
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
		// DEPRECATED (ACL-Legacy-Compat) - This is used so that the bootstrap token is still visible via the v1 acl APIs
		Type: structs.ACLTokenTypeManagement,
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
		// DEPRECATED (ACL-Legacy-Compat) - This is used so that the bootstrap token is still visible via the v1 acl APIs
		Type: structs.ACLTokenTypeManagement,
	}

	s := testStateStore(t)
	setupGlobalManagement(t, s)

	canBootstrap, index, err := s.CanBootstrapACLToken()
	require.NoError(t, err)
	require.True(t, canBootstrap)
	require.Equal(t, uint64(0), index)

	// Perform a regular bootstrap.
	require.NoError(t, s.ACLBootstrap(3, 0, token1, false))

	// Make sure we can't bootstrap again
	canBootstrap, index, err = s.CanBootstrapACLToken()
	require.NoError(t, err)
	require.False(t, canBootstrap)
	require.Equal(t, uint64(3), index)

	// Make sure another attempt fails.
	err = s.ACLBootstrap(4, 0, token2, false)
	require.Error(t, err)
	require.Equal(t, structs.ACLBootstrapNotAllowedErr, err)

	// Check that the bootstrap state remains the same.
	canBootstrap, index, err = s.CanBootstrapACLToken()
	require.NoError(t, err)
	require.False(t, canBootstrap)
	require.Equal(t, uint64(3), index)

	// Make sure the ACLs are in an expected state.
	_, tokens, err := s.ACLTokenList(nil, true, true, "")
	require.NoError(t, err)
	require.Len(t, tokens, 1)
	require.Equal(t, token1, tokens[0])

	// bootstrap reset
	err = s.ACLBootstrap(32, index-1, token2, false)
	require.Error(t, err)
	require.Equal(t, structs.ACLBootstrapInvalidResetIndexErr, err)

	// bootstrap reset
	err = s.ACLBootstrap(32, index, token2, false)
	require.NoError(t, err)

	_, tokens, err = s.ACLTokenList(nil, true, true, "")
	require.NoError(t, err)
	require.Len(t, tokens, 2)
}

func TestStateStore_ACLToken_SetGet_Legacy(t *testing.T) {
	t.Parallel()
	t.Run("Legacy - Existing With Policies", func(t *testing.T) {
		t.Parallel()
		s := testACLTokensStateStore(t)

		token := &structs.ACLToken{
			AccessorID: "c8d0378c-566a-4535-8fc9-c883a8cc9849",
			SecretID:   "6d48ce91-2558-4098-bdab-8737e4e57d5f",
			Policies: []structs.ACLTokenPolicyLink{
				structs.ACLTokenPolicyLink{
					ID: "a0625e95-9b3e-42de-a8d6-ceef5b6f3286",
				},
			},
		}

		require.NoError(t, s.ACLTokenSet(2, token, false))

		// legacy flag is set so it should disallow setting this token
		err := s.ACLTokenSet(3, token, true)
		require.Error(t, err)
	})

	t.Run("Legacy - Empty Type", func(t *testing.T) {
		t.Parallel()
		s := testACLTokensStateStore(t)
		token := &structs.ACLToken{
			AccessorID: "271cd056-0038-4fd3-90e5-f97f50fb3ac8",
			SecretID:   "c0056225-5785-43b3-9b77-3954f06d6aee",
		}

		require.NoError(t, s.ACLTokenSet(2, token, false))

		// legacy flag is set so it should disallow setting this token
		err := s.ACLTokenSet(3, token, true)
		require.Error(t, err)
	})

	t.Run("Legacy - New", func(t *testing.T) {
		t.Parallel()
		s := testACLTokensStateStore(t)
		token := &structs.ACLToken{
			SecretID: "2989e271-6169-4f34-8fec-4618d70008fb",
			Type:     structs.ACLTokenTypeClient,
			Rules:    `service "" { policy = "read" }`,
		}

		require.NoError(t, s.ACLTokenSet(2, token, true))

		idx, rtoken, err := s.ACLTokenGetBySecret(nil, token.SecretID)
		require.NoError(t, err)
		require.Equal(t, uint64(2), idx)
		require.NotNil(t, rtoken)
		require.Equal(t, "", rtoken.AccessorID)
		require.Equal(t, "2989e271-6169-4f34-8fec-4618d70008fb", rtoken.SecretID)
		require.Equal(t, "", rtoken.Description)
		require.Len(t, rtoken.Policies, 0)
		require.Equal(t, structs.ACLTokenTypeClient, rtoken.Type)
		require.Equal(t, uint64(2), rtoken.CreateIndex)
		require.Equal(t, uint64(2), rtoken.ModifyIndex)
	})

	t.Run("Legacy - Update", func(t *testing.T) {
		t.Parallel()
		s := testACLTokensStateStore(t)
		original := &structs.ACLToken{
			SecretID: "2989e271-6169-4f34-8fec-4618d70008fb",
			Type:     structs.ACLTokenTypeClient,
			Rules:    `service "" { policy = "read" }`,
		}

		require.NoError(t, s.ACLTokenSet(2, original, true))

		updatedRules := `service "" { policy = "read" } service "foo" { policy = "deny"}`
		update := &structs.ACLToken{
			SecretID: "2989e271-6169-4f34-8fec-4618d70008fb",
			Type:     structs.ACLTokenTypeClient,
			Rules:    updatedRules,
		}

		require.NoError(t, s.ACLTokenSet(3, update, true))

		idx, rtoken, err := s.ACLTokenGetBySecret(nil, original.SecretID)
		require.NoError(t, err)
		require.Equal(t, uint64(3), idx)
		require.NotNil(t, rtoken)
		require.Equal(t, "", rtoken.AccessorID)
		require.Equal(t, "2989e271-6169-4f34-8fec-4618d70008fb", rtoken.SecretID)
		require.Equal(t, "", rtoken.Description)
		require.Len(t, rtoken.Policies, 0)
		require.Equal(t, structs.ACLTokenTypeClient, rtoken.Type)
		require.Equal(t, updatedRules, rtoken.Rules)
		require.Equal(t, uint64(2), rtoken.CreateIndex)
		require.Equal(t, uint64(3), rtoken.ModifyIndex)
	})
}

func TestStateStore_ACLToken_SetGet(t *testing.T) {
	t.Parallel()
	t.Run("Missing Secret", func(t *testing.T) {
		t.Parallel()
		s := testACLTokensStateStore(t)
		token := &structs.ACLToken{
			AccessorID: "39171632-6f34-4411-827f-9416403687f4",
		}

		err := s.ACLTokenSet(2, token, false)
		require.Error(t, err)
		require.Equal(t, ErrMissingACLTokenSecret, err)
	})

	t.Run("Missing Accessor", func(t *testing.T) {
		t.Parallel()
		s := testACLTokensStateStore(t)
		token := &structs.ACLToken{
			SecretID: "39171632-6f34-4411-827f-9416403687f4",
		}

		err := s.ACLTokenSet(2, token, false)
		require.Error(t, err)
		require.Equal(t, ErrMissingACLTokenAccessor, err)
	})

	t.Run("Missing Policy ID", func(t *testing.T) {
		t.Parallel()
		s := testACLTokensStateStore(t)
		token := &structs.ACLToken{
			AccessorID: "daf37c07-d04d-4fd5-9678-a8206a57d61a",
			SecretID:   "39171632-6f34-4411-827f-9416403687f4",
			Policies: []structs.ACLTokenPolicyLink{
				structs.ACLTokenPolicyLink{
					Name: "no-id",
				},
			},
		}

		err := s.ACLTokenSet(2, token, false)
		require.Error(t, err)
	})

	t.Run("Unresolvable Policy ID", func(t *testing.T) {
		t.Parallel()
		s := testACLTokensStateStore(t)
		token := &structs.ACLToken{
			AccessorID: "daf37c07-d04d-4fd5-9678-a8206a57d61a",
			SecretID:   "39171632-6f34-4411-827f-9416403687f4",
			Policies: []structs.ACLTokenPolicyLink{
				structs.ACLTokenPolicyLink{
					ID: "4f20e379-b496-4b99-9599-19a197126490",
				},
			},
		}

		err := s.ACLTokenSet(2, token, false)
		require.Error(t, err)
	})

	t.Run("New", func(t *testing.T) {
		t.Parallel()
		s := testACLTokensStateStore(t)
		token := &structs.ACLToken{
			AccessorID: "daf37c07-d04d-4fd5-9678-a8206a57d61a",
			SecretID:   "39171632-6f34-4411-827f-9416403687f4",
			Policies: []structs.ACLTokenPolicyLink{
				structs.ACLTokenPolicyLink{
					ID: "a0625e95-9b3e-42de-a8d6-ceef5b6f3286",
				},
			},
		}

		require.NoError(t, s.ACLTokenSet(2, token, false))

		idx, rtoken, err := s.ACLTokenGetByAccessor(nil, "daf37c07-d04d-4fd5-9678-a8206a57d61a")
		require.NoError(t, err)
		require.Equal(t, uint64(2), idx)
		// pointer equality
		require.True(t, rtoken == token)
		require.Equal(t, uint64(2), rtoken.CreateIndex)
		require.Equal(t, uint64(2), rtoken.ModifyIndex)
		require.Len(t, rtoken.Policies, 1)
		require.Equal(t, "node-read", rtoken.Policies[0].Name)
	})

	t.Run("Update", func(t *testing.T) {
		t.Parallel()
		s := testACLTokensStateStore(t)
		token := &structs.ACLToken{
			AccessorID: "daf37c07-d04d-4fd5-9678-a8206a57d61a",
			SecretID:   "39171632-6f34-4411-827f-9416403687f4",
			Policies: []structs.ACLTokenPolicyLink{
				structs.ACLTokenPolicyLink{
					ID: "a0625e95-9b3e-42de-a8d6-ceef5b6f3286",
				},
			},
		}

		require.NoError(t, s.ACLTokenSet(2, token, false))

		updated := &structs.ACLToken{
			AccessorID: "daf37c07-d04d-4fd5-9678-a8206a57d61a",
			SecretID:   "39171632-6f34-4411-827f-9416403687f4",
			Policies: []structs.ACLTokenPolicyLink{
				structs.ACLTokenPolicyLink{
					ID: structs.ACLPolicyGlobalManagementID,
				},
			},
		}

		require.NoError(t, s.ACLTokenSet(3, updated, false))

		idx, rtoken, err := s.ACLTokenGetByAccessor(nil, "daf37c07-d04d-4fd5-9678-a8206a57d61a")
		require.NoError(t, err)
		require.Equal(t, uint64(3), idx)
		// pointer equality
		require.True(t, rtoken == updated)
		require.Equal(t, uint64(2), rtoken.CreateIndex)
		require.Equal(t, uint64(3), rtoken.ModifyIndex)
		require.Len(t, rtoken.Policies, 1)
		require.Equal(t, structs.ACLPolicyGlobalManagementID, rtoken.Policies[0].ID)
		require.Equal(t, "global-management", rtoken.Policies[0].Name)
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

		require.NoError(t, s.ACLTokenBatchSet(2, tokens, true))

		_, token, err := s.ACLTokenGetByAccessor(nil, tokens[0].AccessorID)
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

		require.NoError(t, s.ACLTokenBatchSet(5, tokens, true))

		updated := structs.ACLTokens{
			&structs.ACLToken{
				AccessorID:  "a4f68bd6-3af5-4f56-b764-3c6f20247879",
				SecretID:    "00ff4564-dd96-4d1b-8ad6-578a08279f79",
				Description: "wont update",
				RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 4},
			},
		}

		require.NoError(t, s.ACLTokenBatchSet(6, updated, true))

		_, token, err := s.ACLTokenGetByAccessor(nil, tokens[0].AccessorID)
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

		require.NoError(t, s.ACLTokenBatchSet(5, tokens, true))

		updated := structs.ACLTokens{
			&structs.ACLToken{
				AccessorID:  "a4f68bd6-3af5-4f56-b764-3c6f20247879",
				SecretID:    "00ff4564-dd96-4d1b-8ad6-578a08279f79",
				Description: "wont update",
			},
		}

		require.NoError(t, s.ACLTokenBatchSet(6, updated, true))

		_, token, err := s.ACLTokenGetByAccessor(nil, tokens[0].AccessorID)
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

		require.NoError(t, s.ACLTokenBatchSet(2, tokens, false))

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

		require.NoError(t, s.ACLTokenBatchSet(2, tokens, false))

		updates := structs.ACLTokens{
			&structs.ACLToken{
				AccessorID:  "a4f68bd6-3af5-4f56-b764-3c6f20247879",
				SecretID:    "00ff4564-dd96-4d1b-8ad6-578a08279f79",
				Description: "first token",
				Policies: []structs.ACLTokenPolicyLink{
					structs.ACLTokenPolicyLink{
						ID: "a0625e95-9b3e-42de-a8d6-ceef5b6f3286",
					},
				},
			},
			&structs.ACLToken{
				AccessorID:  "a2719052-40b3-4a4b-baeb-f3df1831a217",
				SecretID:    "ff826eaf-4b88-4881-aaef-52b1089e5d5d",
				Description: "second token",
				Policies: []structs.ACLTokenPolicyLink{
					structs.ACLTokenPolicyLink{
						ID: structs.ACLPolicyGlobalManagementID,
					},
				},
			},
		}

		require.NoError(t, s.ACLTokenBatchSet(3, updates, false))

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
		require.Equal(t, "a0625e95-9b3e-42de-a8d6-ceef5b6f3286", rtokens[1].Policies[0].ID)
		require.Equal(t, "node-read", rtokens[1].Policies[0].Name)
		require.Equal(t, uint64(2), rtokens[1].CreateIndex)
		require.Equal(t, uint64(3), rtokens[1].ModifyIndex)
	})
}

func TestStateStore_ACLTokens_ListUpgradeable(t *testing.T) {
	t.Parallel()
	s := testACLTokensStateStore(t)

	require.NoError(t, s.ACLTokenSet(2, &structs.ACLToken{
		SecretID: "34ec8eb3-095d-417a-a937-b439af7a8e8b",
		Type:     structs.ACLTokenTypeManagement,
	}, true))

	require.NoError(t, s.ACLTokenSet(3, &structs.ACLToken{
		SecretID: "8de2dd39-134d-4cb1-950b-b7ab96ea20ba",
		Type:     structs.ACLTokenTypeManagement,
	}, true))

	require.NoError(t, s.ACLTokenSet(4, &structs.ACLToken{
		SecretID: "548bdb8e-c0d6-477b-bcc4-67fb836e9e61",
		Type:     structs.ACLTokenTypeManagement,
	}, true))

	require.NoError(t, s.ACLTokenSet(5, &structs.ACLToken{
		SecretID: "3ee33676-d9b8-4144-bf0b-92618cff438b",
		Type:     structs.ACLTokenTypeManagement,
	}, true))

	require.NoError(t, s.ACLTokenSet(6, &structs.ACLToken{
		SecretID: "fa9d658a-6e26-42ab-a5f0-1ea05c893dee",
		Type:     structs.ACLTokenTypeManagement,
	}, true))

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
				structs.ACLTokenPolicyLink{
					ID: structs.ACLPolicyGlobalManagementID,
				},
			},
		},
		&structs.ACLToken{
			AccessorID: "54866514-3cf2-4fec-8a8a-710583831834",
			SecretID:   "8de2dd39-134d-4cb1-950b-b7ab96ea20ba",
			Policies: []structs.ACLTokenPolicyLink{
				structs.ACLTokenPolicyLink{
					ID: structs.ACLPolicyGlobalManagementID,
				},
			},
		},
		&structs.ACLToken{
			AccessorID: "47eea4da-bda1-48a6-901c-3e36d2d9262f",
			SecretID:   "548bdb8e-c0d6-477b-bcc4-67fb836e9e61",
			Policies: []structs.ACLTokenPolicyLink{
				structs.ACLTokenPolicyLink{
					ID: structs.ACLPolicyGlobalManagementID,
				},
			},
		},
		&structs.ACLToken{
			AccessorID: "af1dffe5-8ac2-4282-9336-aeed9f7d951a",
			SecretID:   "3ee33676-d9b8-4144-bf0b-92618cff438b",
			Policies: []structs.ACLTokenPolicyLink{
				structs.ACLTokenPolicyLink{
					ID: structs.ACLPolicyGlobalManagementID,
				},
			},
		},
		&structs.ACLToken{
			AccessorID: "511df589-3316-4784-b503-6e25ead4d4e1",
			SecretID:   "fa9d658a-6e26-42ab-a5f0-1ea05c893dee",
			Policies: []structs.ACLTokenPolicyLink{
				structs.ACLTokenPolicyLink{
					ID: structs.ACLPolicyGlobalManagementID,
				},
			},
		},
	}

	require.NoError(t, s.ACLTokenBatchSet(7, updates, false))

	tokens, _, err = s.ACLTokenListUpgradeable(10)
	require.NoError(t, err)
	require.Len(t, tokens, 0)
}

func TestStateStore_ACLToken_List(t *testing.T) {
	t.Parallel()
	s := testACLTokensStateStore(t)

	tokens := structs.ACLTokens{
		// the local token
		&structs.ACLToken{
			AccessorID: "f1093997-b6c7-496d-bfb8-6b1b1895641b",
			SecretID:   "34ec8eb3-095d-417a-a937-b439af7a8e8b",
			Policies: []structs.ACLTokenPolicyLink{
				structs.ACLTokenPolicyLink{
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
				structs.ACLTokenPolicyLink{
					ID: structs.ACLPolicyGlobalManagementID,
				},
			},
		},
		// the policy specific token
		&structs.ACLToken{
			AccessorID: "47eea4da-bda1-48a6-901c-3e36d2d9262f",
			SecretID:   "548bdb8e-c0d6-477b-bcc4-67fb836e9e61",
			Policies: []structs.ACLTokenPolicyLink{
				structs.ACLTokenPolicyLink{
					ID: "a0625e95-9b3e-42de-a8d6-ceef5b6f3286",
				},
			},
		},
		// the policy specific token and local
		&structs.ACLToken{
			AccessorID: "4915fc9d-3726-4171-b588-6c271f45eecd",
			SecretID:   "f6998577-fd9b-4e6c-b202-cc3820513d32",
			Policies: []structs.ACLTokenPolicyLink{
				structs.ACLTokenPolicyLink{
					ID: "a0625e95-9b3e-42de-a8d6-ceef5b6f3286",
				},
			},
			Local: true,
		},
	}

	require.NoError(t, s.ACLTokenBatchSet(2, tokens, false))

	type testCase struct {
		name      string
		local     bool
		global    bool
		policy    string
		accessors []string
	}

	cases := []testCase{
		{
			name:   "Global",
			local:  false,
			global: true,
			policy: "",
			accessors: []string{
				structs.ACLTokenAnonymousID,
				"47eea4da-bda1-48a6-901c-3e36d2d9262f",
				"54866514-3cf2-4fec-8a8a-710583831834",
			},
		},
		{
			name:   "Local",
			local:  true,
			global: false,
			policy: "",
			accessors: []string{
				"4915fc9d-3726-4171-b588-6c271f45eecd",
				"f1093997-b6c7-496d-bfb8-6b1b1895641b",
			},
		},
		{
			name:   "Policy",
			local:  true,
			global: true,
			policy: "a0625e95-9b3e-42de-a8d6-ceef5b6f3286",
			accessors: []string{
				"47eea4da-bda1-48a6-901c-3e36d2d9262f",
				"4915fc9d-3726-4171-b588-6c271f45eecd",
			},
		},
		{
			name:   "Policy - Local",
			local:  true,
			global: false,
			policy: "a0625e95-9b3e-42de-a8d6-ceef5b6f3286",
			accessors: []string{
				"4915fc9d-3726-4171-b588-6c271f45eecd",
			},
		},
		{
			name:   "Policy - Global",
			local:  false,
			global: true,
			policy: "a0625e95-9b3e-42de-a8d6-ceef5b6f3286",
			accessors: []string{
				"47eea4da-bda1-48a6-901c-3e36d2d9262f",
				"4915fc9d-3726-4171-b588-6c271f45eecd",
			},
		},
		{
			name:   "All",
			local:  true,
			global: true,
			policy: "",
			accessors: []string{
				structs.ACLTokenAnonymousID,
				"47eea4da-bda1-48a6-901c-3e36d2d9262f",
				"4915fc9d-3726-4171-b588-6c271f45eecd",
				"54866514-3cf2-4fec-8a8a-710583831834",
				"f1093997-b6c7-496d-bfb8-6b1b1895641b",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, tokens, err := s.ACLTokenList(nil, tc.local, tc.global, tc.policy)
			require.NoError(t, err)
			require.Len(t, tokens, len(tc.accessors))
			tokens.Sort()
			for i, token := range tokens {
				require.Equal(t, tc.accessors[i], token.AccessorID)
			}
		})
	}
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
				structs.ACLTokenPolicyLink{
					ID: structs.ACLPolicyGlobalManagementID,
				},
			},
			Local: true,
		}

		require.NoError(t, s.ACLTokenSet(2, token, false))

		_, rtoken, err := s.ACLTokenGetByAccessor(nil, "f1093997-b6c7-496d-bfb8-6b1b1895641b")
		require.NoError(t, err)
		require.NotNil(t, rtoken)

		require.NoError(t, s.ACLTokenDeleteByAccessor(3, "f1093997-b6c7-496d-bfb8-6b1b1895641b"))

		_, rtoken, err = s.ACLTokenGetByAccessor(nil, "f1093997-b6c7-496d-bfb8-6b1b1895641b")
		require.NoError(t, err)
		require.Nil(t, rtoken)
	})

	t.Run("Secret", func(t *testing.T) {
		t.Parallel()
		s := testACLTokensStateStore(t)

		token := &structs.ACLToken{
			AccessorID: "f1093997-b6c7-496d-bfb8-6b1b1895641b",
			SecretID:   "34ec8eb3-095d-417a-a937-b439af7a8e8b",
			Policies: []structs.ACLTokenPolicyLink{
				structs.ACLTokenPolicyLink{
					ID: structs.ACLPolicyGlobalManagementID,
				},
			},
			Local: true,
		}

		require.NoError(t, s.ACLTokenSet(2, token, false))

		_, rtoken, err := s.ACLTokenGetByAccessor(nil, "f1093997-b6c7-496d-bfb8-6b1b1895641b")
		require.NoError(t, err)
		require.NotNil(t, rtoken)

		require.NoError(t, s.ACLTokenDeleteBySecret(3, "34ec8eb3-095d-417a-a937-b439af7a8e8b"))

		_, rtoken, err = s.ACLTokenGetByAccessor(nil, "f1093997-b6c7-496d-bfb8-6b1b1895641b")
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
					structs.ACLTokenPolicyLink{
						ID: structs.ACLPolicyGlobalManagementID,
					},
				},
				Local: true,
			},
			&structs.ACLToken{
				AccessorID: "a0bfe8d4-b2f3-4b48-b387-f28afb820eab",
				SecretID:   "be444e46-fb95-4ccc-80d5-c873f34e6fa6",
				Policies: []structs.ACLTokenPolicyLink{
					structs.ACLTokenPolicyLink{
						ID: structs.ACLPolicyGlobalManagementID,
					},
				},
				Local: true,
			},
		}

		require.NoError(t, s.ACLTokenBatchSet(2, tokens, false))

		_, rtoken, err := s.ACLTokenGetByAccessor(nil, "f1093997-b6c7-496d-bfb8-6b1b1895641b")
		require.NoError(t, err)
		require.NotNil(t, rtoken)
		_, rtoken, err = s.ACLTokenGetByAccessor(nil, "a0bfe8d4-b2f3-4b48-b387-f28afb820eab")
		require.NoError(t, err)
		require.NotNil(t, rtoken)

		require.NoError(t, s.ACLTokenBatchDelete(2, []string{
			"f1093997-b6c7-496d-bfb8-6b1b1895641b",
			"a0bfe8d4-b2f3-4b48-b387-f28afb820eab"}))

		_, rtoken, err = s.ACLTokenGetByAccessor(nil, "f1093997-b6c7-496d-bfb8-6b1b1895641b")
		require.NoError(t, err)
		require.Nil(t, rtoken)
		_, rtoken, err = s.ACLTokenGetByAccessor(nil, "a0bfe8d4-b2f3-4b48-b387-f28afb820eab")
		require.NoError(t, err)
		require.Nil(t, rtoken)
	})

	t.Run("Anonymous", func(t *testing.T) {
		t.Parallel()
		s := testACLTokensStateStore(t)

		require.Error(t, s.ACLTokenDeleteByAccessor(3, structs.ACLTokenAnonymousID))
		require.Error(t, s.ACLTokenDeleteBySecret(3, "anonymous"))
	})

	t.Run("Not Found", func(t *testing.T) {
		t.Parallel()
		s := testACLStateStore(t)

		// deletion of non-existant policies is not an error
		require.NoError(t, s.ACLTokenDeleteByAccessor(3, "ea58a09c-2100-4aef-816b-8ee0ade77dcd"))
		require.NoError(t, s.ACLTokenDeleteBySecret(3, "376d0cae-dd50-4213-9668-2c7797a7fb2d"))
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
			ID:          "2c74a9b8-271c-4a21-b727-200db397c01c",
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

			_, rpolicy, err := s.ACLPolicyGetByName(nil, "management")
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
			ID:          "a0625e95-9b3e-42de-a8d6-ceef5b6f3286",
			Name:        "node-read",
			Description: "Allows reading all node information",
			Rules:       `node_prefix "" { policy = "read" }`,
			Syntax:      acl.SyntaxCurrent,
			Datacenters: []string{"dc1"},
		}

		require.NoError(t, s.ACLPolicySet(3, &policy))

		idx, rpolicy, err := s.ACLPolicyGetByID(nil, "a0625e95-9b3e-42de-a8d6-ceef5b6f3286")
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
		idx, rpolicy, err = s.ACLPolicyGetByID(nil, structs.ACLPolicyGlobalManagementID)
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

		update := structs.ACLPolicy{
			ID:          "a0625e95-9b3e-42de-a8d6-ceef5b6f3286",
			Name:        "node-read-modified",
			Description: "Modified",
			Rules:       `node_prefix "" { policy = "read" } node "secret" { policy = "deny" }`,
			Syntax:      acl.SyntaxCurrent,
			Datacenters: []string{"dc1", "dc2"},
		}

		require.NoError(t, s.ACLPolicySet(3, &update))

		idx, rpolicy, err := s.ACLPolicyGetByID(nil, "a0625e95-9b3e-42de-a8d6-ceef5b6f3286")
		require.Equal(t, uint64(3), idx)
		require.NoError(t, err)
		require.NotNil(t, rpolicy)
		require.Equal(t, "node-read-modified", rpolicy.Name)
		require.Equal(t, "Modified", rpolicy.Description)
		require.Equal(t, `node_prefix "" { policy = "read" } node "secret" { policy = "deny" }`, rpolicy.Rules)
		require.Equal(t, acl.SyntaxCurrent, rpolicy.Syntax)
		require.ElementsMatch(t, []string{"dc1", "dc2"}, rpolicy.Datacenters)
		require.Equal(t, uint64(2), rpolicy.CreateIndex)
		require.Equal(t, uint64(3), rpolicy.ModifyIndex)
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

	_, policies, err := s.ACLPolicyList(nil)
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

		_, rpolicy, err := s.ACLPolicyGetByID(nil, "f1093997-b6c7-496d-bfb8-6b1b1895641b")
		require.NoError(t, err)
		require.NotNil(t, rpolicy)

		require.NoError(t, s.ACLPolicyDeleteByID(3, "f1093997-b6c7-496d-bfb8-6b1b1895641b"))
		require.NoError(t, err)

		_, rpolicy, err = s.ACLPolicyGetByID(nil, "f1093997-b6c7-496d-bfb8-6b1b1895641b")
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

		_, rpolicy, err := s.ACLPolicyGetByName(nil, "test-policy")
		require.NoError(t, err)
		require.NotNil(t, rpolicy)

		require.NoError(t, s.ACLPolicyDeleteByName(3, "test-policy"))
		require.NoError(t, err)

		_, rpolicy, err = s.ACLPolicyGetByName(nil, "test-policy")
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

		_, rpolicy, err := s.ACLPolicyGetByID(nil, "f1093997-b6c7-496d-bfb8-6b1b1895641b")
		require.NoError(t, err)
		require.NotNil(t, rpolicy)
		_, rpolicy, err = s.ACLPolicyGetByID(nil, "a0bfe8d4-b2f3-4b48-b387-f28afb820eab")
		require.NoError(t, err)
		require.NotNil(t, rpolicy)

		require.NoError(t, s.ACLPolicyBatchDelete(3, []string{
			"f1093997-b6c7-496d-bfb8-6b1b1895641b",
			"a0bfe8d4-b2f3-4b48-b387-f28afb820eab"}))

		_, rpolicy, err = s.ACLPolicyGetByID(nil, "f1093997-b6c7-496d-bfb8-6b1b1895641b")
		require.NoError(t, err)
		require.Nil(t, rpolicy)
		_, rpolicy, err = s.ACLPolicyGetByID(nil, "a0bfe8d4-b2f3-4b48-b387-f28afb820eab")
		require.NoError(t, err)
		require.Nil(t, rpolicy)
	})

	t.Run("Global-Management", func(t *testing.T) {
		t.Parallel()
		s := testACLStateStore(t)

		require.Error(t, s.ACLPolicyDeleteByID(5, structs.ACLPolicyGlobalManagementID))
		require.Error(t, s.ACLPolicyDeleteByName(5, "global-management"))
	})

	t.Run("Not Found", func(t *testing.T) {
		t.Parallel()
		s := testACLStateStore(t)

		// deletion of non-existant policies is not an error
		require.NoError(t, s.ACLPolicyDeleteByName(3, "not-found"))
		require.NoError(t, s.ACLPolicyDeleteByID(3, "376d0cae-dd50-4213-9668-2c7797a7fb2d"))
	})
}

func TestStateStore_ACLTokens_Snapshot_Restore(t *testing.T) {
	s := testStateStore(t)

	tokens := structs.ACLTokens{
		&structs.ACLToken{
			AccessorID:  "68016c3d-835b-450c-a6f9-75db9ba740be",
			SecretID:    "838f72b5-5c15-4a9e-aa6d-31734c3a0286",
			Description: "token1",
			Policies: []structs.ACLTokenPolicyLink{
				structs.ACLTokenPolicyLink{
					ID:   "ca1fc52c-3676-4050-82ed-ca223e38b2c9",
					Name: "policy1",
				},
				structs.ACLTokenPolicyLink{
					ID:   "7b70fa0f-58cd-412d-93c3-a0f17bb19a3e",
					Name: "policy2",
				},
			},
			Hash:      []byte{1, 2, 3, 4},
			RaftIndex: structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
		},
		&structs.ACLToken{
			AccessorID:  "b2125a1b-2a52-41d4-88f3-c58761998a46",
			SecretID:    "ba5d9239-a4ab-49b9-ae09-1f19eed92204",
			Description: "token2",
			Policies: []structs.ACLTokenPolicyLink{
				structs.ACLTokenPolicyLink{
					ID:   "ca1fc52c-3676-4050-82ed-ca223e38b2c9",
					Name: "policy1",
				},
				structs.ACLTokenPolicyLink{
					ID:   "7b70fa0f-58cd-412d-93c3-a0f17bb19a3e",
					Name: "policy2",
				},
			},
			Hash:      []byte{1, 2, 3, 4},
			RaftIndex: structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
		},
	}

	require.NoError(t, s.ACLTokenBatchSet(2, tokens, false))

	// Snapshot the ACLs.
	snap := s.Snapshot()
	defer snap.Close()

	// Alter the real state store.
	require.NoError(t, s.ACLTokenDeleteByAccessor(3, tokens[0].AccessorID))

	// Verify the snapshot.
	require.Equal(t, uint64(2), snap.LastIndex())

	iter, err := snap.ACLTokens()
	require.NoError(t, err)

	var dump structs.ACLTokens
	for token := iter.Next(); token != nil; token = iter.Next() {
		dump = append(dump, token.(*structs.ACLToken))
	}
	require.ElementsMatch(t, dump, tokens)

	// Restore the values into a new state store.
	func() {
		s := testStateStore(t)
		restore := s.Restore()
		for _, token := range dump {
			require.NoError(t, restore.ACLToken(token))
		}
		restore.Commit()

		// Read the restored ACLs back out and verify that they match.
		idx, res, err := s.ACLTokenList(nil, true, true, "")
		require.NoError(t, err)
		require.Equal(t, uint64(2), idx)
		require.ElementsMatch(t, tokens, res)
		require.Equal(t, uint64(2), s.maxIndex("acl-tokens"))
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
	require.NoError(t, s.ACLPolicyDeleteByID(3, policies[0].ID))

	// Verify the snapshot.
	require.Equal(t, uint64(2), snap.LastIndex())

	iter, err := snap.ACLPolicies()
	require.NoError(t, err)

	var dump structs.ACLPolicies
	for policy := iter.Next(); policy != nil; policy = iter.Next() {
		dump = append(dump, policy.(*structs.ACLPolicy))
	}
	require.ElementsMatch(t, dump, policies)

	// Restore the values into a new state store.
	func() {
		s := testStateStore(t)
		restore := s.Restore()
		for _, policy := range dump {
			require.NoError(t, restore.ACLPolicy(policy))
		}
		restore.Commit()

		// Read the restored ACLs back out and verify that they match.
		idx, res, err := s.ACLPolicyList(nil)
		require.NoError(t, err)
		require.Equal(t, uint64(2), idx)
		require.ElementsMatch(t, policies, res)
		require.Equal(t, uint64(2), s.maxIndex("acl-policies"))
	}()
}
