package agentpb

import (
	fmt "fmt"

	"github.com/hashicorp/consul/acl"
)

// EnforceACL takes an acl.Authorizer and returns the decision for whether the
// event is allowed to be sent to this client or not.
func (e *Event) EnforceACL(authz acl.Authorizer) acl.EnforcementDecision {
	switch v := e.Payload.(type) {
	// For now these ACL types are just used internally so we don't enforce anything for
	// them. To play it safe just always deny until we expose them properly.
	case *Event_ACLPolicy:
		return acl.Deny
	case *Event_ACLRole:
		return acl.Deny
	case *Event_ACLToken:
		return acl.Deny

	// These are protocol messages that are always OK for the subscriber to see as
	// they don't expose any information from the data model.
	case *Event_ResetStream:
		return acl.Allow
	case *Event_ResumeStream:
		return acl.Allow
	case *Event_EndOfSnapshot:
		return acl.Allow
	// EventBatch is a special case of the above. While it does contain other
	// events that might need filtering, we only use it in the transport of other
	// events _after_ they've been filtered currently so we don't need to make it
	// recursively return all the nested event requirements here.
	case *Event_EventBatch:
		return acl.Allow

	// Actual Stream events
	case *Event_ServiceHealth:
		// If it's not populated it's likely a bug so don't send it (or panic on
		// nils). This might catch us out if we ever send partial messages but
		// hopefully test will show that up early.
		if v.ServiceHealth == nil || v.ServiceHealth.CheckServiceNode == nil {
			return acl.Deny
		}
		csn := v.ServiceHealth.CheckServiceNode

		if csn.Node == nil || csn.Service == nil ||
			csn.Node.Node == "" || csn.Service.Service == "" {
			return acl.Deny
		}

		if dec := authz.NodeRead(csn.Node.Node, nil); dec != acl.Allow {
			return acl.Deny
		}

		// TODO(banks): need to actually populate the AuthorizerContext once we add
		// Enterprise support for streaming events - they don't have enough data to
		// populate it yet.
		if dec := authz.ServiceRead(csn.Service.Service, nil); dec != acl.Allow {
			return acl.Deny
		}
		return acl.Allow

	default:
		panic(fmt.Sprintf("Event payload type has no ACL requirements defined: %#v",
			e.Payload))
	}
}

// EventBatchEventsFromEventSlice is a helper to convert a slice of event
// objects as used internally in Consul to a slice of pointer's to the same
// events which the generated EventBatch code needs.
func EventBatchEventsFromEventSlice(events []Event) []*Event {
	ret := make([]*Event, len(events))
	for i := range events {
		ret[i] = &events[i]
	}
	return ret
}
