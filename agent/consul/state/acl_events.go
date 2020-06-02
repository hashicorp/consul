package state

import (
	"github.com/hashicorp/consul/agent/agentpb"
	"github.com/hashicorp/consul/agent/structs"
	memdb "github.com/hashicorp/go-memdb"
)

// ACLEventsFromChanges returns all the ACL token, policy or role events that
// should be emitted given a set of changes to the state store.
func (s *Store) ACLEventsFromChanges(tx *txn, changes memdb.Changes) ([]agentpb.Event, error) {

	// Don't allocate yet since in majority of update transactions no ACL token
	// will be changed.
	var events []agentpb.Event

	getObj := func(change memdb.Change) interface{} {
		if change.Deleted() {
			return change.Before
		}
		return change.After
	}

	getOp := func(change memdb.Change) agentpb.ACLOp {
		if change.Deleted() {
			return agentpb.ACLOp_Delete
		}
		return agentpb.ACLOp_Update
	}

	for _, change := range changes {
		switch change.Table {
		case "acl-tokens":
			token := getObj(change).(*structs.ACLToken)
			e := agentpb.Event{
				Topic: agentpb.Topic_ACLTokens,
				Index: tx.Index,
				Payload: &agentpb.Event_ACLToken{
					ACLToken: &agentpb.ACLTokenUpdate{
						Op: getOp(change),
						Token: &agentpb.ACLTokenIdentifier{
							AccessorID: token.AccessorID,
							SecretID:   token.SecretID,
						},
					},
				},
			}
			events = append(events, e)
		case "acl-policies":
			policy := getObj(change).(*structs.ACLPolicy)
			e := agentpb.Event{
				Topic: agentpb.Topic_ACLPolicies,
				Index: tx.Index,
				Payload: &agentpb.Event_ACLPolicy{
					ACLPolicy: &agentpb.ACLPolicyUpdate{
						Op:       getOp(change),
						PolicyID: policy.ID,
					},
				},
			}
			events = append(events, e)
		case "acl-roles":
			role := getObj(change).(*structs.ACLRole)
			e := agentpb.Event{
				Topic: agentpb.Topic_ACLRoles,
				Index: tx.Index,
				Payload: &agentpb.Event_ACLRole{
					ACLRole: &agentpb.ACLRoleUpdate{
						Op:     getOp(change),
						RoleID: role.ID,
					},
				},
			}
			events = append(events, e)
		default:
			continue
		}
	}

	return events, nil
}
