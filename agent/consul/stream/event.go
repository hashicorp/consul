package stream

import (
	fmt "fmt"
	"hash/fnv"
)

// FilterObject returns the object in the event to use for boolean
// expression filtering.
func (e *Event) FilterObject() interface{} {
	if e == nil || e.Payload == nil {
		return nil
	}

	switch e.Payload.(type) {
	case *Event_ServiceHealth:
		return e.GetServiceHealth().CheckServiceNode
	default:
		return nil
	}
}

// ID returns an identifier for the event based on the contents of the payload.
func (e *Event) ID() uint32 {
	if e == nil || e.Payload == nil {
		return 0
	}

	var id string
	switch e.Payload.(type) {
	case *Event_ServiceHealth:
		node := e.GetServiceHealth().CheckServiceNode
		if node == nil || node.Node == nil || node.Service == nil {
			return 0
		}
		id = fmt.Sprintf("%s/%s", node.Node.Node, node.Service.ID)
	default:
	}

	h := fnv.New32a()
	h.Write([]byte(id))
	return h.Sum32()
}

// MakeDeleteEvent creates a minimal delete event for removing an object
// due to filtering.
func MakeDeleteEvent(e *Event) (*Event, error) {
	deleteEvent := &Event{Topic: e.Topic}

	switch e.Payload.(type) {
	case *Event_ServiceHealth:
		node := e.GetServiceHealth().CheckServiceNode
		if node == nil || node.Node == nil || node.Service == nil {
			return nil, fmt.Errorf("event missing node or service info")
		}

		deleteEvent.Payload = &Event_ServiceHealth{
			ServiceHealth: &ServiceHealthUpdate{
				Op: CatalogOp_Deregister,
				CheckServiceNode: &CheckServiceNode{
					Node:    &Node{Node: node.Node.Node},
					Service: &NodeService{Service: node.Service.Service},
				},
			},
		}
	default:
		return nil, fmt.Errorf("unrecognized payload type")
	}

	return deleteEvent, nil
}
