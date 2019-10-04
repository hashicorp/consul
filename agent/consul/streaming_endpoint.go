package consul

import (
	"fmt"
	"strings"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/consul/agent/consul/stream"
	bexpr "github.com/hashicorp/go-bexpr"
)

type ConsulGRPCAdapter struct {
	Health
}

// Subscribe opens a long-lived gRPC stream which sends an initial snapshot
// of state for the requested topic, then only sends updates.
func (h *ConsulGRPCAdapter) Subscribe(req *stream.SubscribeRequest, server stream.Consul_SubscribeServer) error {
	metrics.IncrCounter([]string{"subscribe", strings.ToLower(req.Topic.String())}, 1)

	// Resolve the token and create the ACL filter.
	rule, err := h.srv.ResolveToken(req.Token)
	if err != nil {
		return err
	}
	aclFilter := newACLFilter(rule, h.srv.logger, h.srv.config.ACLEnforceVersion8)
	eventFilter := req.EventFilter()

	// Create the boolean expression filter.
	var eval *bexpr.Evaluator
	if req.Filter != "" {
		eval, err = bexpr.CreateEvaluator(req.Filter, nil)
		if err != nil {
			return fmt.Errorf("Failed to create boolean expression evaluator: %v", err)
		}
	}

	// Send an initial snapshot of the state via events if the requested index
	// is lower than the last sent index of the topic.
	state := h.srv.fsm.State()
	lastSentIndex := state.LastTopicIndex(req.Topic)
	sent := make(map[uint32]struct{})
	if req.Index < lastSentIndex || lastSentIndex == 0 {
		snapshotCh := make(chan stream.Event, 32)
		go state.GetTopicSnapshot(server.Context(), snapshotCh, req)

		// Wait for the events to come in and send them to the client.
		for event := range snapshotCh {
			event.SetACLRules()
			if err := sendEvent(event, eventFilter, aclFilter, eval, sent, server); err != nil {
				return err
			}
		}
	}

	// Register a subscription on this topic/key with the FSM.
	eventCh := state.Subscribe(req)
	defer state.Unsubscribe(req)

	for {
		select {
		case <-server.Context().Done():
			return nil
		case event, ok := <-eventCh:
			// If the channel was closed, that means the state store filled it up
			// faster than we could pull events out.
			if !ok {
				return fmt.Errorf("handler could not keep up with events")
			}

			// If we need to reload the stream (because our ACL token was updated)
			// then pass the event along to the client and exit.
			if event.GetReloadStream() {
				if err := server.Send(&event); err != nil {
					return err
				}
				return nil
			}

			if err := sendEvent(event, eventFilter, aclFilter, eval, sent, server); err != nil {
				return err
			}
		}
	}
}

// sendEvent sends the given event along the stream if it passes ACL, boolean, and
// any topic-specific filtering.
func sendEvent(event stream.Event, eventFilter stream.EventFilterFunc, aclFilter *aclFilter,
	eval *bexpr.Evaluator, sent map[uint32]struct{}, server stream.Consul_SubscribeServer) error {
	allowEvent := true

	// Check if the event should be filtered based on the request type.
	/*if eventFilter != nil && !eventFilter(event) {
		allowEvent = false
	}*/

	// Filter by ACL rules.
	if !aclFilter.allowEvent(event) {
		allowEvent = false
	}

	// Apply the user's boolean expression filtering.
	if eval != nil {
		allow, err := eval.Evaluate(event.FilterObject())
		if err != nil {
			return err
		}
		if !allow {
			allowEvent = false
		}
	}

	// If the event would be filtered but the agent needs to know about
	// the delete, change the operation to delete and send the event
	// before removing the ID from the sent map.
	id := event.ID()
	if !allowEvent {
		if _, ok := sent[id]; ok {
			deleteEvent, err := stream.MakeDeleteEvent(&event)
			if err != nil {
				return err
			}

			// Send the delete.
			if err := server.Send(deleteEvent); err != nil {
				return err
			}

			delete(sent, id)
			return nil
		} else {
			// If the event should be filtered and the agent doesn't know about it,
			// just return early and don't send anything.
			return nil
		}
	}

	// Send the event.
	if err := server.Send(&event); err != nil {
		return err
	}
	sent[id] = struct{}{}

	return nil
}
