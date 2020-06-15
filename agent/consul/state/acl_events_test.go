package state

import (
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/require"
)

func TestACLEventsFromChanges(t *testing.T) {
	cases := []struct {
		Name     string
		Setup    func(s *Store, tx *txn) error
		Mutate   func(s *Store, tx *txn) error
		expected stream.Event
	}{
		{
			Name: "token create",
			Mutate: func(s *Store, tx *txn) error {
				return s.aclTokenSetTxn(tx, tx.Index, newACLToken(1), false, false, false, false)
			},
			expected: newACLTokenEvent(100, 1),
		},
		{
			Name: "token update",
			Setup: func(s *Store, tx *txn) error {
				return s.aclTokenSetTxn(tx, tx.Index, newACLToken(1), false, false, false, false)
			},
			Mutate: func(s *Store, tx *txn) error {
				// Add a policy to the token (never mind it doesn't exist for now) we
				// allow it in the set command below.
				token := newACLToken(1)
				token.Policies = []structs.ACLTokenPolicyLink{{ID: "33333333-1111-1111-1111-111111111111"}}
				return s.aclTokenSetTxn(tx, tx.Index, token, false, true, false, false)
			},
			expected: newACLTokenEvent(100, 1, structs.ACLTokenPolicyLink{ID: "33333333-1111-1111-1111-111111111111"}),
		},
		{
			Name: "token delete",
			Setup: func(s *Store, tx *txn) error {
				return s.aclTokenSetTxn(tx, tx.Index, newACLToken(1), false, false, false, false)
			},
			Mutate: func(s *Store, tx *txn) error {
				token := newACLToken(1)
				return s.aclTokenDeleteTxn(tx, tx.Index, token.AccessorID, "id", nil)
			},
			expected: newACLTokenEvent(100, 1),
		},
		{
			Name: "policy create",
			Mutate: func(s *Store, tx *txn) error {
				return s.aclPolicySetTxn(tx, tx.Index, newACLPolicy(1))
			},
			expected: newACLPolicyEvent(100, 1),
		},
		{
			Name: "policy update",
			Setup: func(s *Store, tx *txn) error {
				return s.aclPolicySetTxn(tx, tx.Index, newACLPolicy(1))
			},
			Mutate: func(s *Store, tx *txn) error {
				policy := newACLPolicy(1)
				policy.Rules = `operator = "write"`
				return s.aclPolicySetTxn(tx, tx.Index, policy)
			},
			expected: stream.Event{
				Topic: stream.Topic_ACLPolicies,
				Index: 100,
				Payload: &structs.ACLPolicy{
					ID:    "22222222-1111-1111-1111-111111111111",
					Name:  "test_policy_1",
					Rules: `operator = "write"`,
				},
			},
		},
		{
			Name: "policy delete",
			Setup: func(s *Store, tx *txn) error {
				return s.aclPolicySetTxn(tx, tx.Index, newACLPolicy(1))
			},
			Mutate: func(s *Store, tx *txn) error {
				policy := newACLPolicy(1)
				return s.aclPolicyDeleteTxn(tx, tx.Index, policy.ID, s.aclPolicyGetByID, nil)
			},
			expected: newACLPolicyEvent(100, 1),
		},
		{
			Name: "role create",
			Mutate: func(s *Store, tx *txn) error {
				return s.aclRoleSetTxn(tx, tx.Index, newACLRole(1, newACLRolePolicyLink(1)), true)
			},
			expected: newACLRoleEvent(100, 1, newACLRolePolicyLink(1)),
		},
		{
			Name: "role update",
			Setup: func(s *Store, tx *txn) error {
				return s.aclRoleSetTxn(tx, tx.Index, newACLRole(1, newACLRolePolicyLink(1)), true)
			},
			Mutate: func(s *Store, tx *txn) error {
				role := newACLRole(1, newACLRolePolicyLink(1))
				policy2 := newACLPolicy(2)
				role.Policies = append(role.Policies, structs.ACLRolePolicyLink{
					ID:   policy2.ID,
					Name: policy2.Name,
				})
				return s.aclRoleSetTxn(tx, tx.Index, role, true)
			},
			expected: newACLRoleEvent(100, 1, newACLRolePolicyLink(1), newACLRolePolicyLink(2)),
		},
		{
			Name: "role delete",
			Setup: func(s *Store, tx *txn) error {
				return s.aclRoleSetTxn(tx, tx.Index, newACLRole(1, newACLRolePolicyLink(1)), true)
			},
			Mutate: func(s *Store, tx *txn) error {
				role := newACLRole(1, newACLRolePolicyLink(1))
				return s.aclRoleDeleteTxn(tx, tx.Index, role.ID, s.aclRoleGetByID, nil)
			},
			expected: newACLRoleEvent(100, 1, newACLRolePolicyLink(1)),
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			s := testStateStore(t)

			if tc.Setup != nil {
				// Bypass the publish mechanism for this test or we get into odd
				// recursive stuff...
				setupTx := s.db.WriteTxn(10)
				require.NoError(t, tc.Setup(s, setupTx))
				// Commit the underlying transaction without using wrapped Commit so we
				// avoid the whole event publishing system for setup here. It _should_
				// work but it makes debugging test hard as it will call the function
				// under test for the setup data...
				setupTx.Txn.Commit()
			}

			tx := s.db.WriteTxn(100)
			require.NoError(t, tc.Mutate(s, tx))

			// Note we call the func under test directly rather than publishChanges so
			// we can test this in isolation.
			events, err := aclEventsFromChanges(tx, tx.Changes())
			require.NoError(t, err)

			require.Len(t, events, 1)
			actual := events[0]
			// ignore modified and created index because we don't set them in our expected values
			// TODO: gotest.tools/assert would make this easier
			normalizePayload(&actual)
			require.Equal(t, tc.expected, actual)
		})
	}
}

func normalizePayload(s *stream.Event) {
	switch s := s.Payload.(type) {
	case *structs.ACLToken:
		s.ModifyIndex = 0
		s.CreateIndex = 0
		s.Hash = nil
	case *structs.ACLPolicy:
		s.ModifyIndex = 0
		s.CreateIndex = 0
	case *structs.ACLRole:
		s.ModifyIndex = 0
		s.CreateIndex = 0
	}
}

func newACLTokenEvent(idx uint64, n int, policies ...structs.ACLTokenPolicyLink) stream.Event {
	uuid := strings.ReplaceAll("11111111-????-????-????-????????????", "?", strconv.Itoa(n))
	return stream.Event{
		Topic: stream.Topic_ACLTokens,
		Index: idx,
		Payload: &structs.ACLToken{
			AccessorID: uuid,
			SecretID:   uuid,
			Policies:   policies,
		},
	}
}

func newACLPolicyEvent(idx uint64, n int) stream.Event {
	return stream.Event{
		Topic:   stream.Topic_ACLPolicies,
		Index:   idx,
		Payload: newACLPolicy(n),
	}
}

func newACLRoleEvent(idx uint64, n int, policies ...structs.ACLRolePolicyLink) stream.Event {
	return stream.Event{
		Topic:   stream.Topic_ACLRoles,
		Index:   idx,
		Payload: newACLRole(n, policies...),
	}
}

func newACLToken(n int) *structs.ACLToken {
	uuid := strings.ReplaceAll("11111111-????-????-????-????????????", "?", strconv.Itoa(n))
	return &structs.ACLToken{
		AccessorID: uuid,
		SecretID:   uuid,
	}
}

func newACLPolicy(n int) *structs.ACLPolicy {
	numStr := strconv.Itoa(n)
	uuid := strings.ReplaceAll("22222222-????-????-????-????????????", "?", numStr)
	return &structs.ACLPolicy{
		ID:    uuid,
		Name:  "test_policy_" + numStr,
		Rules: `operator = "read"`,
	}
}

func newACLRole(n int, policies ...structs.ACLRolePolicyLink) *structs.ACLRole {
	numStr := strconv.Itoa(n)
	uuid := strings.ReplaceAll("33333333-????-????-????-????????????", "?", numStr)
	return &structs.ACLRole{
		ID:       uuid,
		Name:     "test_role_" + numStr,
		Policies: policies,
	}
}

func newACLRolePolicyLink(n int) structs.ACLRolePolicyLink {
	policy := newACLPolicy(n)
	return structs.ACLRolePolicyLink{
		ID:   policy.ID,
		Name: policy.Name,
	}
}
