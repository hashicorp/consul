package state

import (
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent/agentpb"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/require"
)

func testACLTokenEvent(t *testing.T, idx uint64, n int, delete bool) agentpb.Event {
	t.Helper()
	uuid := strings.ReplaceAll("11111111-????-????-????-????????????", "?",
		strconv.Itoa(n))
	op := agentpb.ACLOp_Update
	if delete {
		op = agentpb.ACLOp_Delete
	}
	return agentpb.Event{
		Topic: agentpb.Topic_ACLTokens,
		Index: idx,
		Payload: &agentpb.Event_ACLToken{
			ACLToken: &agentpb.ACLTokenUpdate{
				Op: op,
				Token: &agentpb.ACLTokenIdentifier{
					AccessorID: uuid,
					SecretID:   uuid,
				},
			},
		},
	}
}

func testACLPolicyEvent(t *testing.T, idx uint64, n int, delete bool) agentpb.Event {
	t.Helper()
	uuid := strings.ReplaceAll("22222222-????-????-????-????????????", "?",
		strconv.Itoa(n))
	op := agentpb.ACLOp_Update
	if delete {
		op = agentpb.ACLOp_Delete
	}
	return agentpb.Event{
		Topic: agentpb.Topic_ACLPolicies,
		Index: idx,
		Payload: &agentpb.Event_ACLPolicy{
			ACLPolicy: &agentpb.ACLPolicyUpdate{
				Op:       op,
				PolicyID: uuid,
			},
		},
	}
}

func testACLRoleEvent(t *testing.T, idx uint64, n int, delete bool) agentpb.Event {
	t.Helper()
	uuid := strings.ReplaceAll("33333333-????-????-????-????????????", "?",
		strconv.Itoa(n))
	op := agentpb.ACLOp_Update
	if delete {
		op = agentpb.ACLOp_Delete
	}
	return agentpb.Event{
		Topic: agentpb.Topic_ACLRoles,
		Index: idx,
		Payload: &agentpb.Event_ACLRole{
			ACLRole: &agentpb.ACLRoleUpdate{
				Op:     op,
				RoleID: uuid,
			},
		},
	}
}

func testToken(t *testing.T, n int) *structs.ACLToken {
	uuid := strings.ReplaceAll("11111111-????-????-????-????????????", "?",
		strconv.Itoa(n))
	return &structs.ACLToken{
		AccessorID: uuid,
		SecretID:   uuid,
	}
}

func testPolicy(t *testing.T, n int) *structs.ACLPolicy {
	numStr := strconv.Itoa(n)
	uuid := strings.ReplaceAll("22222222-????-????-????-????????????", "?", numStr)
	return &structs.ACLPolicy{
		ID:    uuid,
		Name:  "test_policy_" + numStr,
		Rules: `operator = "read"`,
	}
}

func testRole(t *testing.T, n, p int) *structs.ACLRole {
	numStr := strconv.Itoa(n)
	uuid := strings.ReplaceAll("33333333-????-????-????-????????????", "?", numStr)
	policy := testPolicy(t, p)
	return &structs.ACLRole{
		ID:   uuid,
		Name: "test_role_" + numStr,
		Policies: []structs.ACLRolePolicyLink{{
			ID:   policy.ID,
			Name: policy.Name,
		}},
	}
}

func TestACLEventsFromChanges(t *testing.T) {
	cases := []struct {
		Name       string
		Setup      func(s *Store, tx *txn) error
		Mutate     func(s *Store, tx *txn) error
		WantEvents []agentpb.Event
		WantErr    bool
	}{
		{
			Name: "token create",
			Mutate: func(s *Store, tx *txn) error {
				if err := s.aclTokenSetTxn(tx, tx.Index, testToken(t, 1), false, false, false, false); err != nil {
					return err
				}
				return nil
			},
			WantEvents: []agentpb.Event{
				testACLTokenEvent(t, 100, 1, false),
			},
			WantErr: false,
		},
		{
			Name: "token update",
			Setup: func(s *Store, tx *txn) error {
				if err := s.aclTokenSetTxn(tx, tx.Index, testToken(t, 1), false, false, false, false); err != nil {
					return err
				}
				return nil
			},
			Mutate: func(s *Store, tx *txn) error {
				// Add a policy to the token (never mind it doesn't exist for now) we
				// allow it in the set command below.
				token := testToken(t, 1)
				token.Policies = []structs.ACLTokenPolicyLink{{ID: "33333333-1111-1111-1111-111111111111"}}
				if err := s.aclTokenSetTxn(tx, tx.Index, token, false, true, false, false); err != nil {
					return err
				}
				return nil
			},
			WantEvents: []agentpb.Event{
				// Should see an event from the update
				testACLTokenEvent(t, 100, 1, false),
			},
			WantErr: false,
		},
		{
			Name: "token delete",
			Setup: func(s *Store, tx *txn) error {
				if err := s.aclTokenSetTxn(tx, tx.Index, testToken(t, 1), false, false, false, false); err != nil {
					return err
				}
				return nil
			},
			Mutate: func(s *Store, tx *txn) error {
				// Delete it
				token := testToken(t, 1)
				if err := s.aclTokenDeleteTxn(tx, tx.Index, token.AccessorID, "id", nil); err != nil {
					return err
				}
				return nil
			},
			WantEvents: []agentpb.Event{
				// Should see a delete event
				testACLTokenEvent(t, 100, 1, true),
			},
			WantErr: false,
		},
		{
			Name: "policy create",
			Mutate: func(s *Store, tx *txn) error {
				if err := s.aclPolicySetTxn(tx, tx.Index, testPolicy(t, 1)); err != nil {
					return err
				}
				return nil
			},
			WantEvents: []agentpb.Event{
				testACLPolicyEvent(t, 100, 1, false),
			},
			WantErr: false,
		},
		{
			Name: "policy update",
			Setup: func(s *Store, tx *txn) error {
				if err := s.aclPolicySetTxn(tx, tx.Index, testPolicy(t, 1)); err != nil {
					return err
				}
				return nil
			},
			Mutate: func(s *Store, tx *txn) error {
				policy := testPolicy(t, 1)
				policy.Rules = `operator = "write"`
				if err := s.aclPolicySetTxn(tx, tx.Index, policy); err != nil {
					return err
				}
				return nil
			},
			WantEvents: []agentpb.Event{
				// Should see an event from the update
				testACLPolicyEvent(t, 100, 1, false),
			},
			WantErr: false,
		},
		{
			Name: "policy delete",
			Setup: func(s *Store, tx *txn) error {
				if err := s.aclPolicySetTxn(tx, tx.Index, testPolicy(t, 1)); err != nil {
					return err
				}
				return nil
			},
			Mutate: func(s *Store, tx *txn) error {
				// Delete it
				policy := testPolicy(t, 1)
				if err := s.aclPolicyDeleteTxn(tx, tx.Index, policy.ID, s.aclPolicyGetByID, nil); err != nil {
					return err
				}
				return nil
			},
			WantEvents: []agentpb.Event{
				// Should see a delete event
				testACLPolicyEvent(t, 100, 1, true),
			},
			WantErr: false,
		},
		{
			Name: "role create",
			Mutate: func(s *Store, tx *txn) error {
				if err := s.aclRoleSetTxn(tx, tx.Index, testRole(t, 1, 1), true); err != nil {
					return err
				}
				return nil
			},
			WantEvents: []agentpb.Event{
				testACLRoleEvent(t, 100, 1, false),
			},
			WantErr: false,
		},
		{
			Name: "role update",
			Setup: func(s *Store, tx *txn) error {
				if err := s.aclRoleSetTxn(tx, tx.Index, testRole(t, 1, 1), true); err != nil {
					return err
				}
				return nil
			},
			Mutate: func(s *Store, tx *txn) error {
				role := testRole(t, 1, 1)
				policy2 := testPolicy(t, 2)
				role.Policies = append(role.Policies, structs.ACLRolePolicyLink{
					ID:   policy2.ID,
					Name: policy2.Name,
				})
				if err := s.aclRoleSetTxn(tx, tx.Index, role, true); err != nil {
					return err
				}
				return nil
			},
			WantEvents: []agentpb.Event{
				// Should see an event from the update
				testACLRoleEvent(t, 100, 1, false),
			},
			WantErr: false,
		},
		{
			Name: "role delete",
			Setup: func(s *Store, tx *txn) error {
				if err := s.aclRoleSetTxn(tx, tx.Index, testRole(t, 1, 1), true); err != nil {
					return err
				}
				return nil
			},
			Mutate: func(s *Store, tx *txn) error {
				// Delete it
				role := testRole(t, 1, 1)
				if err := s.aclRoleDeleteTxn(tx, tx.Index, role.ID, s.aclRoleGetByID, nil); err != nil {
					return err
				}
				return nil
			},
			WantEvents: []agentpb.Event{
				// Should see a delete event
				testACLRoleEvent(t, 100, 1, true),
			},
			WantErr: false,
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
			got, err := s.ACLEventsFromChanges(tx, tx.Changes())
			if tc.WantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Make sure we have the right events, only taking ordering into account
			// where it matters to account for non-determinism.
			requireEventsInCorrectPartialOrder(t, tc.WantEvents, got, func(e agentpb.Event) string {
				// We only care that events affecting the same actual token are ordered
				// with respect ot each other so use it's ID as the key.
				switch v := e.Payload.(type) {
				case *agentpb.Event_ACLToken:
					return "token:" + v.ACLToken.Token.AccessorID
				case *agentpb.Event_ACLPolicy:
					return "policy:" + v.ACLPolicy.PolicyID
				case *agentpb.Event_ACLRole:
					return "role:" + v.ACLRole.RoleID
				}
				return ""
			})
		})
	}
}
