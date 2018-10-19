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

func TestStateStore_ACLBootstrap(t *testing.T) {
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

/*

func TestStateStore_ACLSet_ACLGet(t *testing.T) {
	s := testStateStore(t)

	// Querying ACLs with no results returns nil
	ws := memdb.NewWatchSet()
	idx, res, err := s.ACLGet(ws, "nope")
	if idx != 0 || res != nil || err != nil {
		t.Fatalf("expected (0, nil, nil), got: (%d, %#v, %#v)", idx, res, err)
	}

	// Inserting an ACL with empty ID is disallowed
	if err := s.ACLSet(1, &structs.ACL{}); err == nil {
		t.Fatalf("expected %#v, got: %#v", ErrMissingACLID, err)
	}
	if watchFired(ws) {
		t.Fatalf("bad")
	}

	// Index is not updated if nothing is saved
	if idx := s.maxIndex("acls"); idx != 0 {
		t.Fatalf("bad index: %d", idx)
	}

	// Inserting valid ACL works
	acl := &structs.ACL{
		ID:    "acl1",
		Name:  "First ACL",
		Type:  structs.ACLTokenTypeClient,
		Rules: "rules1",
	}
	if err := s.ACLSet(1, acl); err != nil {
		t.Fatalf("err: %s", err)
	}
	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	// Check that the index was updated
	if idx := s.maxIndex("acls"); idx != 1 {
		t.Fatalf("bad index: %d", idx)
	}

	// Retrieve the ACL again
	ws = memdb.NewWatchSet()
	idx, result, err := s.ACLGet(ws, "acl1")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 1 {
		t.Fatalf("bad index: %d", idx)
	}

	// Check that the ACL matches the result
	expect := &structs.ACL{
		ID:    "acl1",
		Name:  "First ACL",
		Type:  structs.ACLTokenTypeClient,
		Rules: "rules1",
		RaftIndex: structs.RaftIndex{
			CreateIndex: 1,
			ModifyIndex: 1,
		},
	}
	if !reflect.DeepEqual(result, expect) {
		t.Fatalf("bad: %#v", result)
	}

	// Update the ACL
	acl = &structs.ACL{
		ID:    "acl1",
		Name:  "First ACL",
		Type:  structs.ACLTokenTypeClient,
		Rules: "rules2",
	}
	if err := s.ACLSet(2, acl); err != nil {
		t.Fatalf("err: %s", err)
	}
	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	// Index was updated
	if idx := s.maxIndex("acls"); idx != 2 {
		t.Fatalf("bad: %d", idx)
	}

	// ACL was updated and matches expected value
	expect = &structs.ACL{
		ID:    "acl1",
		Name:  "First ACL",
		Type:  structs.ACLTokenTypeClient,
		Rules: "rules2",
		RaftIndex: structs.RaftIndex{
			CreateIndex: 1,
			ModifyIndex: 2,
		},
	}
	if !reflect.DeepEqual(acl, expect) {
		t.Fatalf("bad: %#v", acl)
	}
}

func TestStateStore_ACLList(t *testing.T) {
	s := testStateStore(t)

	// Listing when no ACLs exist returns nil
	ws := memdb.NewWatchSet()
	idx, res, err := s.ACLList(ws)
	if idx != 0 || res != nil || err != nil {
		t.Fatalf("expected (0, nil, nil), got: (%d, %#v, %#v)", idx, res, err)
	}

	// Insert some ACLs
	acls := structs.ACLs{
		&structs.ACL{
			ID:    "acl1",
			Type:  structs.ACLTokenTypeClient,
			Rules: "rules1",
			RaftIndex: structs.RaftIndex{
				CreateIndex: 1,
				ModifyIndex: 1,
			},
		},
		&structs.ACL{
			ID:    "acl2",
			Type:  structs.ACLTokenTypeClient,
			Rules: "rules2",
			RaftIndex: structs.RaftIndex{
				CreateIndex: 2,
				ModifyIndex: 2,
			},
		},
	}
	for _, acl := range acls {
		if err := s.ACLSet(acl.ModifyIndex, acl); err != nil {
			t.Fatalf("err: %s", err)
		}
	}
	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	// Query the ACLs
	idx, res, err = s.ACLList(nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 2 {
		t.Fatalf("bad index: %d", idx)
	}

	// Check that the result matches
	if !reflect.DeepEqual(res, acls) {
		t.Fatalf("bad: %#v", res)
	}
}

func TestStateStore_ACLDelete(t *testing.T) {
	s := testStateStore(t)

	// Calling delete on an ACL which doesn't exist returns nil
	if err := s.ACLDelete(1, "nope"); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Index isn't updated if nothing is deleted
	if idx := s.maxIndex("acls"); idx != 0 {
		t.Fatalf("bad index: %d", idx)
	}

	// Insert an ACL
	if err := s.ACLSet(1, &structs.ACL{ID: "acl1"}); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Delete the ACL and check that the index was updated
	if err := s.ACLDelete(2, "acl1"); err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx := s.maxIndex("acls"); idx != 2 {
		t.Fatalf("bad index: %d", idx)
	}

	tx := s.db.Txn(false)
	defer tx.Abort()

	// Check that the ACL was really deleted
	result, err := tx.First("acls", "id", "acl1")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if result != nil {
		t.Fatalf("expected nil, got: %#v", result)
	}
}
*/

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

	require.NoError(t, s.ACLTokensUpsert(2, tokens, true))

	// Snapshot the ACLs.
	snap := s.Snapshot()
	defer snap.Close()

	// Alter the real state store.
	require.NoError(t, s.ACLTokenDeleteAccessor(3, tokens[0].AccessorID))

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

	require.NoError(t, s.ACLPoliciesUpsert(2, policies))

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
		idx, res, err := s.ACLPolicyList(nil, "")
		require.NoError(t, err)
		require.Equal(t, uint64(2), idx)
		require.ElementsMatch(t, policies, res)
		require.Equal(t, uint64(2), s.maxIndex("acl-policies"))
	}()
}
