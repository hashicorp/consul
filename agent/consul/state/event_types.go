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
	var snapshotFunc func(*memdb.Txn, context.Context, chan stream.Event, *stream.SubscribeRequest) error
	switch req.Topic {
	case stream.Topic_ServiceHealth:
		snapshotFunc = s.ServiceHealthSnapshot

	case stream.Topic_ServiceHealthConnect:
		snapshotFunc = s.ServiceHealthConnectSnapshot

	default:
		return fmt.Errorf("only ServiceHealth and ServiceHealthConnect topics supported")
	}

	// Compute the index to send back with the "end of snapshot" message.
	tx := s.db.Txn(false)
	defer tx.Abort()
	idx, err := s.computeIndexTxn(tx)
	if err != nil {
		return err
	}
	if err := snapshotFunc(tx, ctx, eventCh, req); err != nil {
		return err
	}

	endSnapshotEvent := stream.Event{
		Topic:   req.Topic,
		Index:   idx,
		Payload: &stream.Event_EndOfSnapshot{EndOfSnapshot: true},
	}
	select {
	case eventCh <- endSnapshotEvent:
	case <-ctx.Done():
		return nil
	}

	if eventCh != nil {
		close(eventCh)
	}

	return nil
}

// ServiceHealthSnapshot returns stream.Events that provide a snapshot of the
// current state.
func (s *Store) ServiceHealthSnapshot(tx *memdb.Txn, ctx context.Context, eventCh chan stream.Event, req *stream.SubscribeRequest) error {
	idx, nodes, err := s.checkServiceNodesTxn(tx, nil, req.Key, false)
	if err != nil {
		return err
	}

	checkServiceNodesToServiceHealth(idx, nodes, ctx, eventCh, false)

	return nil
}

// ServiceHealthSnapshot returns stream.Events that provide a snapshot of the
// current state.
func (s *Store) ServiceHealthConnectSnapshot(tx *memdb.Txn, ctx context.Context, eventCh chan stream.Event, req *stream.SubscribeRequest) error {
	idx, nodes, err := s.checkServiceNodesTxn(tx, nil, req.Key, true)
	if err != nil {
		return err
	}

	checkServiceNodesToServiceHealth(idx, nodes, ctx, eventCh, true)

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

	serviceHealthEvents := checkServiceNodesToServiceHealth(idx, nodes, nil, nil, false)
	serviceHealthConnectEvents := serviceHealthToConnectEvents(serviceHealthEvents)

	return append(serviceHealthEvents, serviceHealthConnectEvents...), nil
}

// DeregistrationEvents returns stream.Events that correspond to a catalog
// deregister operation.
func (s *Store) DeregistrationEvents(tx *memdb.Txn, idx uint64, node string) ([]stream.Event, error) {
	serviceHealthEvent := serviceHealthDeregisterEvent(idx, node)
	serviceHealthConnectEvents := serviceHealthToConnectEvents([]stream.Event{serviceHealthEvent})

	events := []stream.Event{
		serviceHealthEvent,
	}
	events = append(events, serviceHealthConnectEvents...)

	return events, nil
}

func serviceHealthToConnectEvents(events []stream.Event) []stream.Event {
	serviceHealthConnectEvents := make([]stream.Event, 0, len(events))
	for _, event := range events {
		node := event.GetServiceHealth().CheckServiceNode
		if node.Service == nil || (node.Service.Kind != structs.ServiceKindConnectProxy && !node.Service.Connect.Native) {
			continue
		}

		connectEvent := event
		connectEvent.Topic = stream.Topic_ServiceHealthConnect

		// If this is a proxy, set the key to the destination service name.
		if node.Service.Kind == structs.ServiceKindConnectProxy {
			connectEvent.Key = node.Service.Proxy.DestinationServiceName
		}

		serviceHealthConnectEvents = append(serviceHealthConnectEvents, connectEvent)
	}

	return serviceHealthConnectEvents
}

// TxnEvents returns the stream.Events that correspond to a Txn operation.
func (s *Store) TxnEvents(tx *memdb.Txn, idx uint64, ops structs.TxnOps) ([]stream.Event, error) {
	var events []stream.Event

	// Get the ServiceHealth and events.
	serviceHealthEvents, err := txnServiceHealthEvents(s, tx, idx, ops)
	if err != nil {
		return nil, err
	}
	events = append(events, serviceHealthEvents...)
	events = append(events, serviceHealthToConnectEvents(serviceHealthEvents)...)

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
