package stream

import (
	fmt "fmt"
	"hash/fnv"

	"github.com/hashicorp/consul/agent/structs"
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

// EventFilterFunc returns true if the given event should be sent.
type EventFilterFunc func(Event) bool

// EventFilter returns a function used to apply event filtering
// to the Subscribe call based on the request.
func (r *SubscribeRequest) EventFilter() EventFilterFunc {
	if r == nil || r.TopicFilters == nil {
		return nil
	}

	switch r.Topic {
	case Topic_ServiceHealth:
		if r.TopicFilters.Connect {
			return serviceConnectFilter
		} else if len(r.TopicFilters.Tags) > 0 {
			return r.serviceTagsFilter
		}
	default:
	}

	return nil
}

// serviceConnectFilter returns whether the event relates to a Connect-enabled service.
func serviceConnectFilter(e Event) bool {
	svc := e.GetServiceHealth()
	if svc == nil || svc.CheckServiceNode == nil || svc.CheckServiceNode.Service == nil {
		return false
	}

	service := svc.CheckServiceNode.Service
	return service.Connect.Native == true || service.Kind == structs.ServiceKindConnectProxy
}

// serviceTagsFilter returns whether the event's service contains the required tags.
func (r *SubscribeRequest) serviceTagsFilter(e Event) bool {
	svc := e.GetServiceHealth()
	if svc == nil || svc.CheckServiceNode == nil || svc.CheckServiceNode.Service == nil {
		return false
	}

	return !structs.ServiceTagsFilter(svc.CheckServiceNode.Service.Tags, r.TopicFilters.Tags)
}
