package consul

import (
	"context"
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
	// Start fetching the initial snapshot.
	state := h.srv.fsm.State()

	var snapshotFunc func(context.Context, chan stream.Event, string) error
	switch req.Topic {
	case stream.Topic_ServiceHealth:
		snapshotFunc = state.ServiceHealthSnapshot

	default:
		return fmt.Errorf("only ServiceHealth topic is supported")
	}

	metrics.IncrCounter([]string{"subscribe", strings.ToLower(req.Topic.String())}, 1)

	// Resolve the token and create the ACL filter.
	rule, err := h.srv.ResolveToken(req.Token)
	if err != nil {
		return err
	}
	aclFilter := newACLFilter(rule, h.srv.logger, h.srv.config.ACLEnforceVersion8)

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
	lastSentIndex := state.LastTopicIndex(req.Topic)
	if req.Index < lastSentIndex || lastSentIndex == 0 {
		snapshotCh := make(chan stream.Event, 32)
		go snapshotFunc(server.Context(), snapshotCh, req.Key)

		// Wait for the events to come in and send them to the client.
		for event := range snapshotCh {
			if err := sendEvent(event, aclFilter, eval, server); err != nil {
				return err
			}
		}

		// Send a marker that this is the end of the snapshot.
		endSnapshotEvent := stream.Event{
			Topic:   req.Topic,
			Payload: &stream.Event_EndOfSnapshot{EndOfSnapshot: true},
		}
		if err := server.Send(&endSnapshotEvent); err != nil {
			return err
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

			if err := sendEvent(event, aclFilter, eval, server); err != nil {
				return err
			}
		}
	}
}

// sendEvent sends the given event along the stream if it passes ACL and boolean
// filtering.
func sendEvent(event stream.Event, aclFilter *aclFilter, eval *bexpr.Evaluator, server stream.Consul_SubscribeServer) error {
	// Filter events by ACL rules.
	event.SetACLRules()
	if !aclFilter.allowEvent(event) {
		return nil
	}

	// Apply boolean expression filtering.
	if eval != nil {
		allow, err := eval.Evaluate(event.FilterObject())
		if err != nil {
			return err
		}
		if !allow {
			return nil
		}
	}

	// Send the event.
	if err := server.Send(&event); err != nil {
		return err
	}

	return nil
}
