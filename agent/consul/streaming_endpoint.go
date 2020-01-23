package consul

import (
	"context"

	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/go-uuid"
)

type ConsulGRPCAdapter struct {
	Health
}

// Subscribe opens a long-lived gRPC stream which sends an initial snapshot
// of state for the requested topic, then only sends updates.
func (h *ConsulGRPCAdapter) Subscribe(req *stream.SubscribeRequest, server stream.Consul_SubscribeServer) error {
	// streamID is just used for message correlation in trace logs. Ideally we'd
	// only execute this code while trace logs are enabled but it's not that
	// expensive and theres not a very clean way to do that right now and
	// impending logging changes so I think this makes sense for now.
	streamID, err := uuid.GenerateUUID()
	if err != nil {
		return err
	}

	// Forward the request to a remote DC if applicable.
	if req.Datacenter != "" && req.Datacenter != h.srv.config.Datacenter {
		return h.forwardAndProxy(req, server, streamID)
	}

	h.srv.logger.Printf("[DEBUG] consul: subscribe start topic=%q key=%q "+
		"index=%d streamID=%s", req.Topic.String(), req.Key, req.Index, streamID)

	var sentCount uint64
	defer func() {
		h.srv.logger.Printf("[DEBUG] consul: subscribe stream closed streamID=%s",
			streamID)
	}()

	// Resolve the token and create the ACL filter.
	// TODO: handle token expiry gracefully...
	authz, err := h.srv.ResolveToken(req.Token)
	if err != nil {
		return err
	}
	aclFilter := newACLFilter(authz, h.srv.logger, h.srv.config.ACLEnforceVersion8)

	state := h.srv.fsm.State()

	// Register a subscription on this topic/key with the FSM.
	sub, err := state.Subscribe(req)
	if err != nil {
		return err
	}
	defer state.Unsubscribe(req)

	// Deliver the events
	for {
		events, err := sub.Next(server.Context())
		if err == stream.ErrSubscriptionReload {
			event := stream.Event{
				Payload: &stream.Event_ReloadStream{ReloadStream: true},
			}
			if err := server.Send(&event); err != nil {
				return err
			}
			h.srv.logger.Printf("[DEBUG] consul: subscribe stream reloaded "+
				"streamID=%s", streamID)
			return nil
		}
		if err != nil {
			return err
		}

		filteredEvents, err := aclFilterEvents(events, aclFilter)
		if err != nil {
			return err
		}

		// TODO: bexpr filtering?
		snapshotDone := false
		if len(filteredEvents) == 1 {
			if events[0].GetEndOfSnapshot() {
				snapshotDone = true
				h.srv.logger.Printf("[DEBUG] consul: subscribe snapshot complete "+
					"idx=%d sent=%d streamID=%s", events[0].Index, sentCount,
					streamID)
			} else if events[0].GetResumeStream() {
				snapshotDone = true
				h.srv.logger.Printf("[DEBUG] consul: subscribe resuming stream "+
					"idx=%d sent=%d streamID=%s", events[0].Index, sentCount,
					streamID)
			} else if snapshotDone {
				// Count this event too in the normal case as "sent" the above cases
				// only show the number of events sent _before_ the snapshot ended.
				h.srv.logger.Printf("[DEBUG] consul: subscribe sending event "+
					"idx=%d sent=%d streamID=%s", events[0].Index, sentCount+1,
					streamID)
			}
			sentCount++
			if err := server.Send(&events[0]); err != nil {
				return err
			}
		} else if len(filteredEvents) > 1 {
			e := &stream.Event{
				Topic: req.Topic,
				Key:   req.Key,
				Index: events[0].Index,
				Payload: &stream.Event_EventBatch{
					EventBatch: &stream.EventBatch{
						Events: filteredEvents,
					},
				},
			}
			sentCount += uint64(len(filteredEvents))
			h.srv.logger.Printf("[DEBUG] consul: subscribe sending events "+
				"idx=%d sent=%d batchSize=%d streamID=%s", events[0].Index,
				sentCount, len(filteredEvents), streamID)
			if err := server.Send(e); err != nil {
				return err
			}
		}
	}
}

func aclFilterEvents(events []stream.Event, aclFilter *aclFilter) ([]*stream.Event, error) {
	// We return a slice of pointers since that is what EventBatch needs anyway
	// even if we do no filtering.
	filtered := make([]*stream.Event, 0, len(events))

	for idx := range events {
		// Get pointer to the actual event. We don't use _, event ranging to save to
		// confusion of making a local copy, this is more explicit.
		event := &events[idx]
		if aclFilter.allowEvent(event) {
			filtered = append(filtered, event)
		}
	}
	return filtered, nil
}

func (h *ConsulGRPCAdapter) forwardAndProxy(req *stream.SubscribeRequest,
	server stream.Consul_SubscribeServer, streamID string) error {

	conn, err := h.srv.grpcClient.GRPCConn(req.Datacenter)
	if err != nil {
		return err
	}

	h.srv.logger.Printf("[DEBUG] consul: subscribe forward to dc=%s topic=%q key=%q "+
		"index=%d streamID=%s", req.Datacenter, req.Topic.String(), req.Key,
		req.Index, streamID)

	defer func() {
		h.srv.logger.Printf("[DEBUG] consul: subscribe forwarded stream complete "+
			"streamID=%s", streamID)
	}()

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
