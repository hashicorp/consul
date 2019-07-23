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

// checkServiceNodesToServiceHealth converts a list of CheckServiceNodes to
// ServiceHealth events for streaming. If a non-nil channel and context are passed,
// the events will be sent to the channel instead of appended to a slice.
func checkServiceNodesToServiceHealth(idx uint64, nodes structs.CheckServiceNodes,
	ctx context.Context, eventCh chan stream.Event) []stream.Event {
	var events []stream.Event
	for _, n := range nodes {
		event := stream.Event{
			Topic: stream.Topic_ServiceHealth,
			Index: idx,
		}

		if n.Service != nil {
			event.Key = n.Service.Service
		}

		event.Payload = &stream.Event_ServiceHealth{
			ServiceHealth: &stream.ServiceHealthUpdate{
				Op:          stream.CatalogOp_Register,
				ServiceNode: stream.ToCheckServiceNode(&n),
			},
		}

		// Send the event on the channel if one was provided.
		if eventCh != nil {
			select {
			case <-ctx.Done():
				return nil
			case eventCh <- event:
			}
		} else {
			events = append(events, event)
		}
	}

	if eventCh != nil {
		close(eventCh)
	}

	return events
}

// DeregistrationEvents returns stream.Events that correspond to a catalog
// deregister operation.
func (s *Store) DeregistrationEvents(tx *memdb.Txn, idx uint64, node string) ([]stream.Event, error) {
	events := []stream.Event{
		stream.Event{
			Topic: stream.Topic_ServiceHealth,
			Index: idx,
			Payload: &stream.Event_ServiceHealth{
				ServiceHealth: &stream.ServiceHealthUpdate{
					Op: stream.CatalogOp_Deregister,
					ServiceNode: &stream.CheckServiceNode{
						Node: &stream.Node{Node: node},
					},
				},
			},
		},
	}

	return events, nil
}
