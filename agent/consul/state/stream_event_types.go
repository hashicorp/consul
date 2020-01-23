package state

import (
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
	memdb "github.com/hashicorp/go-memdb"
)

// unboundSnapFn is a stream.SnapFn with state store as the first argument. This
// is bound to a concrete state store instance in the EventPublisher on startup.
type unboundSnapFn func(*Store, *stream.SubscribeRequest, *stream.EventBuffer) (uint64, error)

type topicHandlers struct {
	Snapshot unboundSnapFn
}

// topicRegistry is a map of topic handlers. It must only be written to during
// init().
var topicRegistry map[stream.Topic]topicHandlers

func init() {
	topicRegistry = map[stream.Topic]topicHandlers{
		stream.Topic_ServiceHealth: topicHandlers{
			Snapshot: (*Store).ServiceHealthSnapshot,
		},
		stream.Topic_ServiceHealthConnect: topicHandlers{
			Snapshot: (*Store).ServiceHealthConnectSnapshot,
		},
	}
}

// ServiceHealthSnapshot returns stream.Events that provide a snapshot of the
// current state.
func (s *Store) ServiceHealthSnapshot(req *stream.SubscribeRequest, buf *stream.EventBuffer) (uint64, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	idx, nodes, err := s.checkServiceNodesTxn(tx, nil, req.Key, false)
	if err != nil {
		return 0, err
	}

	checkServiceNodesToServiceHealth(idx, nodes, buf, false)

	return idx, nil
}

// ServiceHealthSnapshot returns stream.Events that provide a snapshot of the
// current state.
func (s *Store) ServiceHealthConnectSnapshot(req *stream.SubscribeRequest, buf *stream.EventBuffer) (uint64, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	idx, nodes, err := s.checkServiceNodesTxn(tx, nil, req.Key, true)
	if err != nil {
		return 0, err
	}

	checkServiceNodesToServiceHealth(idx, nodes, buf, true)
	return idx, nil
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

	serviceHealthEvents := checkServiceNodesToServiceHealth(idx, nodes, nil, false)
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
					AccessorID: token.AccessorID,
					SecretID:   token.SecretID,
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
