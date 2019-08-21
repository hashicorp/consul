package state

import (
	"context"

	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
	memdb "github.com/hashicorp/go-memdb"
)

// ServiceHealthSnapshot returns stream.Events that provide a snapshot of the
// current state.
func (s *Store) ServiceHealthSnapshot(ctx context.Context, eventCh chan stream.Event, service string) error {
	idx, nodes, err := s.CheckServiceNodes(nil, service)
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

// DeregistrationEvents returns stream.Events that correspond to a catalog
// deregister operation.
func (s *Store) DeregistrationEvents(tx *memdb.Txn, idx uint64, node string) ([]stream.Event, error) {
	events := []stream.Event{
		serviceHealthDeregisterEvent(idx, node),
	}

	return events, nil
}
