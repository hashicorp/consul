package state

import (
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/require"
)

func TestACLChangeUnsubscribeEvent(t *testing.T) {
	cases := []struct {
		Name     string
		Setup    func(tx *txn) error
		Mutate   func(tx *txn) error
		expected stream.Event
	}{
		{
			Name: "token create",
			Mutate: func(tx *txn) error {
				return aclTokenSetTxn(tx, tx.Index, newACLToken(1), ACLTokenSetOptions{})
			},
			expected: stream.NewCloseSubscriptionEvent(newSecretIDs(1)),
		},
		{
			Name: "token update",
			Setup: func(tx *txn) error {
				return aclTokenSetTxn(tx, tx.Index, newACLToken(1), ACLTokenSetOptions{})
			},
			Mutate: func(tx *txn) error {
				// Add a policy to the token (never mind it doesn't exist for now) we
				// allow it in the set command below.
				token := newACLToken(1)
				token.Policies = []structs.ACLTokenPolicyLink{{ID: "33333333-1111-1111-1111-111111111111"}}
				return aclTokenSetTxn(tx, tx.Index, token, ACLTokenSetOptions{AllowMissingPolicyAndRoleIDs: true})
			},
			expected: stream.NewCloseSubscriptionEvent(newSecretIDs(1)),
		},
		{
			Name: "token delete",
			Setup: func(tx *txn) error {
				return aclTokenSetTxn(tx, tx.Index, newACLToken(1), ACLTokenSetOptions{})
			},
			Mutate: func(tx *txn) error {
				token := newACLToken(1)
				return aclTokenDeleteTxn(tx, tx.Index, token.AccessorID, "id", nil)
			},
			expected: stream.NewCloseSubscriptionEvent(newSecretIDs(1)),
		},
		{
			Name:   "policy create",
			Mutate: newACLPolicyWithSingleToken,
			// two identical tokens, because Mutate has two changes
			expected: stream.NewCloseSubscriptionEvent(newSecretIDs(1, 1)),
		},
		{
			Name:  "policy update",
			Setup: newACLPolicyWithSingleToken,
			Mutate: func(tx *txn) error {
				policy := newACLPolicy(1)
				policy.Rules = `operator = "write"`
				return aclPolicySetTxn(tx, tx.Index, policy)
			},
			expected: stream.NewCloseSubscriptionEvent(newSecretIDs(1)),
		},
		{
			Name:  "policy delete",
			Setup: newACLPolicyWithSingleToken,
			Mutate: func(tx *txn) error {
				policy := newACLPolicy(1)
				return aclPolicyDeleteTxn(tx, tx.Index, policy.ID, aclPolicyGetByID, nil)
			},
			expected: stream.NewCloseSubscriptionEvent(newSecretIDs(1)),
		},
		{
			Name:   "role create",
			Mutate: newACLRoleWithSingleToken,
			// Two tokens with the same ID, because there are two changes in Mutate
			expected: stream.NewCloseSubscriptionEvent(newSecretIDs(1, 1)),
		},
		{
			Name:  "role update",
			Setup: newACLRoleWithSingleToken,
			Mutate: func(tx *txn) error {
				role := newACLRole(1, newACLRolePolicyLink(1))
				policy2 := newACLPolicy(2)
				role.Policies = append(role.Policies, structs.ACLRolePolicyLink{
					ID:   policy2.ID,
					Name: policy2.Name,
				})
				return aclRoleSetTxn(tx, tx.Index, role, true)
			},
			expected: stream.NewCloseSubscriptionEvent(newSecretIDs(1)),
		},
		{
			Name:  "role delete",
			Setup: newACLRoleWithSingleToken,
			Mutate: func(tx *txn) error {
				role := newACLRole(1, newACLRolePolicyLink(1))
				return aclRoleDeleteTxn(tx, tx.Index, role.ID, aclRoleGetByID, nil)
			},
			expected: stream.NewCloseSubscriptionEvent(newSecretIDs(1)),
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
				require.NoError(t, tc.Setup(setupTx))
				// Commit the underlying transaction without using wrapped Commit so we
				// avoid the whole event publishing system for setup here. It _should_
				// work but it makes debugging test hard as it will call the function
				// under test for the setup data...
				setupTx.Txn.Commit()
			}

			tx := s.db.WriteTxn(100)
			require.NoError(t, tc.Mutate(tx))

			// Note we call the func under test directly rather than publishChanges so
			// we can test this in isolation.
			events, err := aclChangeUnsubscribeEvent(tx, Changes{Index: 100, Changes: tx.Changes()})
			require.NoError(t, err)

			require.Len(t, events, 1)
			actual := events[0]
			require.Equal(t, tc.expected, actual)
		})
	}
}

func newACLRoleWithSingleToken(tx *txn) error {
	role := newACLRole(1, newACLRolePolicyLink(1))
	if err := aclRoleSetTxn(tx, tx.Index, role, true); err != nil {
		return err
	}
	token := newACLToken(1)
	token.Roles = append(token.Roles, structs.ACLTokenRoleLink{ID: role.ID})
	return aclTokenSetTxn(tx, tx.Index, token, ACLTokenSetOptions{})
}

func newACLPolicyWithSingleToken(tx *txn) error {
	policy := newACLPolicy(1)
	if err := aclPolicySetTxn(tx, tx.Index, policy); err != nil {
		return err
	}
	token := newACLToken(1)
	token.Policies = append(token.Policies, structs.ACLTokenPolicyLink{ID: policy.ID})
	return aclTokenSetTxn(tx, tx.Index, token, ACLTokenSetOptions{})
}

func newSecretIDs(ids ...int) []string {
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		uuid := strings.ReplaceAll("11111111-????-????-????-????????????", "?", strconv.Itoa(id))
		result = append(result, uuid)
	}
	return result
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
