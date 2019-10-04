package state

import (
	"context"
	"fmt"

	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
	memdb "github.com/hashicorp/go-memdb"
)

// GetTopicSnapshot returns a snapshot of the given topic based on the SubscribeRequest.
func (s *Store) GetTopicSnapshot(ctx context.Context, eventCh chan stream.Event, req *stream.SubscribeRequest) error {
	var snapshotFunc func(context.Context, chan stream.Event, *stream.SubscribeRequest) error
	switch req.Topic {
	case stream.Topic_ServiceHealth:
		snapshotFunc = s.ServiceHealthSnapshot

	default:
		return fmt.Errorf("only the ServiceHealth topic is supported")
	}

	return snapshotFunc(ctx, eventCh, req)
}

// ServiceHealthSnapshot returns stream.Events that provide a snapshot of the
// current state.
func (s *Store) ServiceHealthSnapshot(ctx context.Context, eventCh chan stream.Event, req *stream.SubscribeRequest) error {
	idx, nodes, err := s.CheckServiceNodes(nil, req.Key)
	if err != nil {
		return err
	}

	checkServiceNodesToServiceHealth(idx, nodes, ctx, eventCh)

	return nil
}

// RegistrationEvents returns stream.Events that correspond to a catalog
// register operation.
func (s *Store) RegistrationEvents(tx *memdb.Txn, idx uint64, node, service string) ([]stream.Event, error) {
	_, services, err := s.nodeServicesTxn(tx, nil, node, service, false)
	if err != nil {
		return nil, err
	}

	idx, nodes, err := s.parseCheckServiceNodes(tx, nil, idx, "", services, nil)
	if err != nil {
		return nil, err
	}

	return checkServiceNodesToServiceHealth(idx, nodes, nil, nil), nil
}

// DeregistrationEvents returns stream.Events that correspond to a catalog
// deregister operation.
func (s *Store) DeregistrationEvents(tx *memdb.Txn, idx uint64, node string) ([]stream.Event, error) {
	events := []stream.Event{
		serviceHealthDeregisterEvent(idx, node),
	}

	return events, nil
}

// TxnEvents returns the stream.Events that correspond to a Txn operation.
func (s *Store) TxnEvents(tx *memdb.Txn, idx uint64, ops structs.TxnOps) ([]stream.Event, error) {
	var events []stream.Event

	// Get the ServiceHealth events.
	serviceHealth, err := txnServiceHealthEvents(s, tx, idx, ops)
	if err != nil {
		return nil, err
	}
	events = append(events, serviceHealth...)

	return events, nil
}

// ACLTokenEvent returns a stream.Event that corresponds to an ACL token operation.
func (s *Store) ACLTokenEvent(idx uint64, token *structs.ACLToken, op stream.ACLOp) stream.Event {
	return stream.Event{
		Topic: stream.Topic_ACLTokens,
		Index: idx,
		Payload: &stream.Event_ACLToken{
			ACLToken: &stream.ACLTokenUpdate{
				Op: op,
				Token: &stream.ACLToken{
					SecretID: token.SecretID,
				},
			},
		},
	}
}

// ACLPolicyEvent returns a stream.Event that corresponds to an ACL policy operation.
func (s *Store) ACLPolicyEvent(idx uint64, id string, op stream.ACLOp) stream.Event {
	return stream.Event{
		Topic: stream.Topic_ACLPolicies,
		Index: idx,
		Payload: &stream.Event_ACLPolicy{
			ACLPolicy: &stream.ACLPolicyUpdate{
				Op:       op,
				PolicyID: id,
			},
		},
	}
}

// ACLRoleEvent returns a stream.Event that corresponds to an ACL role operation.
func (s *Store) ACLRoleEvent(idx uint64, id string, op stream.ACLOp) stream.Event {
	return stream.Event{
		Topic: stream.Topic_ACLRoles,
		Index: idx,
		Payload: &stream.Event_ACLRole{
			ACLRole: &stream.ACLRoleUpdate{
				Op:     op,
				RoleID: id,
			},
		},
	}
}
