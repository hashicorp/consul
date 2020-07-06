package state

import (
	"github.com/hashicorp/consul/agent/consul/state/db"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
	memdb "github.com/hashicorp/go-memdb"
)

// ACLEventsFromChanges returns all the ACL token, policy or role events that
// should be emitted given a set of changes to the state store.
// TODO: Add OpDelete/OpUpdate to the event or payload?
func aclEventsFromChanges(_ db.ReadTxn, changes db.Changes) ([]stream.Event, error) {
	var events []stream.Event

	// TODO: mapping of table->topic?
	for _, change := range changes.Changes {
		switch change.Table {
		case "acl-tokens":
			token := changeObject(change).(*structs.ACLToken)
			e := stream.Event{
				Topic:   stream.Topic_ACLTokens,
				Index:   changes.Index,
				Payload: token,
			}
			events = append(events, e)
		case "acl-policies":
			policy := changeObject(change).(*structs.ACLPolicy)
			e := stream.Event{
				Topic:   stream.Topic_ACLPolicies,
				Index:   changes.Index,
				Payload: policy,
			}
			events = append(events, e)
		case "acl-roles":
			role := changeObject(change).(*structs.ACLRole)
			e := stream.Event{
				Topic:   stream.Topic_ACLRoles,
				Index:   changes.Index,
				Payload: role,
			}
			events = append(events, e)
		}
	}
	return events, nil
}

// changeObject returns the object before it was deleted if the change was a delete,
// otherwise returns the object after the change.
func changeObject(change memdb.Change) interface{} {
	if change.Deleted() {
		return change.Before
	}
	return change.After
}
