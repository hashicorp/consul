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

	// Wait for the events to come in and forward them to the client.
	var snapshotDone bool
	for !snapshotDone {
		select {
		case <-server.Context().Done():
			return nil
		case event, ok := <-snapshotCh:
			h.srv.logger.Printf("[INFO] consul: sent snapshot event to key %q", req.Key)
			if !ok {
				h.srv.logger.Printf("[INFO] consul: finished sending snapshot")
				snapshotDone = true
				break
			}
			if err := server.Send(&event); err != nil {
				return err
			}
		}
	}

	// Register a subscription on this topic/key with the FSM.
	eventCh := state.Subscribe(req)
	defer state.Unsubscribe(req)

	h.srv.logger.Printf("[INFO] consul: subscribed to key %q", req.Key)

	for {
		select {
		case <-server.Context().Done():
			return nil
		case event := <-eventCh:
			h.srv.logger.Printf("[INFO] consul: sending event for key %q", req.Key)
			if err := server.Send(&event); err != nil {
				return err
			}
		}
	}
}
