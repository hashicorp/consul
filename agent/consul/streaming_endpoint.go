package consul

import (
	"context"
	"fmt"
	"strings"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/consul/agent/consul/stream"
	bexpr "github.com/hashicorp/go-bexpr"
	"golang.org/x/crypto/blake2b"
)

type ConsulGRPCAdapter struct {
	Health
}

// Subscribe opens a long-lived gRPC stream which sends an initial snapshot
// of state for the requested topic, then only sends updates.
func (h *ConsulGRPCAdapter) Subscribe(req *stream.SubscribeRequest, server stream.Consul_SubscribeServer) error {
	metrics.IncrCounter([]string{"subscribe", strings.ToLower(req.Topic.String())}, 1)

	// Forward the request to a remote DC if applicable.
	if req.Datacenter != "" && req.Datacenter != h.srv.config.Datacenter {
		conn, err := h.srv.grpcClient.GRPCConn(req.Datacenter)
		if err != nil {
			return err
		}

		// Open a Subscribe call to the remote DC.
		client := stream.NewConsulClient(conn)
		streamHandle, err := client.Subscribe(server.Context(), req)
		if err != nil {
			return err
		}

		// Relay the events back to the client.
		for {
			event, err := streamHandle.Recv()
			if err != nil {
				return err
			}
			if err := server.Send(event); err != nil {
				return err
			}
		}
	}

	h.srv.logger.Printf("consul: stream starting in %s", h.srv.config.Datacenter)

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
	state := h.srv.fsm.State()
	lastSentIndex := state.LastTopicIndex(req.Topic)
	sent := make(map[string]struct{})
	if req.Index < lastSentIndex || lastSentIndex == 0 {
		snapshotCh := make(chan stream.Event, 32)
		go state.GetTopicSnapshot(server.Context(), snapshotCh, req)

		// Wait for the events to come in and send them to the client.
		for event := range snapshotCh {
			event.SetACLRules()
			if err := sendEvent(event, aclFilter, eval, sent, server); err != nil {
				return err
			}
		}
	} else {
		// If there wasn't a snapshot, just send an end of snapshot message
		// so the client knows not to wait for one.
		idx, err := state.ComputeIndex()
		if err != nil {
			return err
		}

		endSnapshotEvent := stream.Event{
			Topic:   req.Topic,
			Index:   idx,
			Payload: &stream.Event_EndOfSnapshot{EndOfSnapshot: true},
		}
		if err := server.Send(&endSnapshotEvent); err != nil {
			return err
		}
	}

	// Register a subscription on this topic/key with the FSM.
	eventCh, err := state.Subscribe(req)
	if err != nil {
		return err
	}
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

			if err := sendEvent(event, aclFilter, eval, sent, server); err != nil {
				return err
			}
		}
	}
}

// sendEvent sends the given event along the stream if it passes ACL, boolean, and
// any topic-specific filtering.
func sendEvent(event stream.Event, aclFilter *aclFilter, eval *bexpr.Evaluator,
	sent map[string]struct{}, server stream.Consul_SubscribeServer) error {
	// If it's a special case event, skip the filtering and just send it.
	if event.GetEndOfSnapshot() || event.GetReloadStream() {
		return server.Send(&event)
	}
	allowEvent := true

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

	// Get the unique identifier for the object the event pertains to, and
	// hash it to save space. This is only used to determine if an object has been
	// removed and needs to be deleted from the client's cache so we make the tradeoff
	// of using less memory here at the extremely small risk of a hash collision.
	rawId := event.ID()
	hash, err := blake2b.New256(nil)
	if err != nil {
		return err
	}
	hash.Write([]byte(rawId))
	bytes := hash.Sum(nil)
	idHash := string(bytes)

	// If the event would be filtered but the agent needs to know about
	// the delete, send a delete event before removing the ID from the sent map.
	if !allowEvent {
		if _, ok := sent[idHash]; ok {
			deleteEvent, err := stream.MakeDeleteEvent(&event)
			if err != nil {
				return err
			}

			// Send the delete.
			if err := server.Send(deleteEvent); err != nil {
				return err
			}

			delete(sent, idHash)
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
	sent[idHash] = struct{}{}

	return nil
}

// Test is an internal endpoint used for checking connectivity/balancing logic.
func (h *ConsulGRPCAdapter) Test(ctx context.Context, req *stream.TestRequest) (*stream.TestResponse, error) {
	if req.Datacenter != "" && req.Datacenter != h.srv.config.Datacenter {
		conn, err := h.srv.grpcClient.GRPCConn(req.Datacenter)
		if err != nil {
			return nil, err
		}

		h.srv.logger.Printf("server conn state %s", conn.GetState())

		// Open a Test call to the remote DC.
		client := stream.NewConsulClient(conn)
		return client.Test(ctx, req)
	}

	return &stream.TestResponse{ServerName: h.srv.config.NodeName}, nil
}
