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

	h.srv.logger.Printf("[INFO] consul: new subscriber to %s/%s", req.Topic, req.Key)

	// Wait for the events to come in and forward them to the client.
	var snapshotDone bool
	for !snapshotDone {
		select {
		case <-server.Context().Done():
			return nil
		case event, ok := <-snapshotCh:
			if !ok {
				h.srv.logger.Printf("[INFO] consul: finished sending snapshot to new subscriber")
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

	for {
		select {
		case <-server.Context().Done():
			return nil
		case event := <-eventCh:
			if err := server.Send(&event); err != nil {
				return err
			}
		}
	}
}
