package consul

import (
	"context"
	"fmt"

	"github.com/hashicorp/consul/agent/consul/stream"
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

	snapshotCh := make(chan stream.Event, 32)
	go snapshotFunc(server.Context(), snapshotCh, req.Key)

	// Resolve the token and create the ACL filter.
	rule, err := h.srv.ResolveToken(req.Token)
	if err != nil {
		return err
	}
	filt := newACLFilter(rule, h.srv.logger, h.srv.config.ACLEnforceVersion8)

	// Wait for the events to come in and forward them to the client.
	for event := range snapshotCh {
		if !filt.allowEvent(event) {
			continue
		}
		if err := server.Send(&event); err != nil {
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

	// Register a subscription on this topic/key with the FSM.
	eventCh := state.Subscribe(req)
	defer state.Unsubscribe(req)

	for {
		select {
		case <-server.Context().Done():
			return nil
		case event, ok := <-eventCh:
			if !ok {
				return fmt.Errorf("handler could not keep up with events")
			}
			if !filt.allowEvent(event) {
				continue
			}
			if err := server.Send(&event); err != nil {
				return err
			}
		}
	}
}
