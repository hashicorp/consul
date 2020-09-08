package subscribe

import (
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/proto/pbevent"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-uuid"
)

// Server implements a StateChangeSubscriptionServer for accepting SubscribeRequests,
// and sending events to the subscription topic.
type Server struct {
	srv    *Server
	logger hclog.Logger
}

var _ pbevent.StateChangeSubscriptionServer = (*Server)(nil)

func (h *Server) Subscribe(req *pbevent.SubscribeRequest, serverStream pbevent.StateChangeSubscription_SubscribeServer) error {
	// streamID is just used for message correlation in trace logs and not
	// populated normally.
	var streamID string
	var err error

	if h.logger.IsTrace() {
		// TODO(banks) it might be nice one day to replace this with OpenTracing ID
		// if one is set etc. but probably pointless until we support that properly
		// in other places so it's actually propagated properly. For now this just
		// makes lifetime of a stream more traceable in our regular server logs for
		// debugging/dev.
		streamID, err = uuid.GenerateUUID()
		if err != nil {
			return err
		}
	}

	// Forward the request to a remote DC if applicable.
	if req.Datacenter != "" && req.Datacenter != h.srv.config.Datacenter {
		return h.forwardAndProxy(req, serverStream, streamID)
	}

	h.srv.logger.Trace("new subscription",
		"topic", req.Topic.String(),
		"key", req.Key,
		"index", req.Index,
		"stream_id", streamID,
	)

	var sentCount uint64
	defer h.srv.logger.Trace("subscription closed", "stream_id", streamID)

	// Resolve the token and create the ACL filter.
	// TODO: handle token expiry gracefully...
	authz, err := h.srv.ResolveToken(req.Token)
	if err != nil {
		return err
	}
	aclFilter := newACLFilter(authz, h.srv.logger, h.srv.config.ACLEnforceVersion8)

	state := h.srv.fsm.State()

	// Register a subscription on this topic/key with the FSM.
	sub, err := state.Subscribe(serverStream.Context(), req)
	if err != nil {
		return err
	}
	defer state.Unsubscribe(req)

	// Deliver the events
	for {
		events, err := sub.Next()
		if err == stream.ErrSubscriptionReload {
			event := pbevent.Event{
				Payload: &pbevent.Event_ResetStream{ResetStream: true},
			}
			if err := serverStream.Send(&event); err != nil {
				return err
			}
			h.srv.logger.Trace("subscription reloaded",
				"stream_id", streamID,
			)
			return nil
		}
		if err != nil {
			return err
		}

		aclFilter.filterStreamEvents(&events)

		snapshotDone := false
		if len(events) == 1 {
			if events[0].GetEndOfSnapshot() {
				snapshotDone = true
				h.srv.logger.Trace("snapshot complete",
					"index", events[0].Index,
					"sent", sentCount,
					"stream_id", streamID,
				)
			} else if events[0].GetResumeStream() {
				snapshotDone = true
				h.srv.logger.Trace("resuming stream",
					"index", events[0].Index,
					"sent", sentCount,
					"stream_id", streamID,
				)
			} else if snapshotDone {
				// Count this event too in the normal case as "sent" the above cases
				// only show the number of events sent _before_ the snapshot ended.
				h.srv.logger.Trace("sending events",
					"index", events[0].Index,
					"sent", sentCount,
					"batch_size", 1,
					"stream_id", streamID,
				)
			}
			sentCount++
			if err := serverStream.Send(&events[0]); err != nil {
				return err
			}
		} else if len(events) > 1 {
			e := &pbevent.Event{
				Topic: req.Topic,
				Key:   req.Key,
				Index: events[0].Index,
				Payload: &pbevent.Event_EventBatch{
					EventBatch: &pbevent.EventBatch{
						Events: pbevent.EventBatchEventsFromEventSlice(events),
					},
				},
			}
			sentCount += uint64(len(events))
			h.srv.logger.Trace("sending events",
				"index", events[0].Index,
				"sent", sentCount,
				"batch_size", len(events),
				"stream_id", streamID,
			)
			if err := serverStream.Send(e); err != nil {
				return err
			}
		}
	}
}

func (h *Server) forwardAndProxy(
	req *pbevent.SubscribeRequest,
	serverStream pbevent.StateChangeSubscription_SubscribeServer,
	streamID string) error {

	conn, err := h.srv.grpcClient.GRPCConn(req.Datacenter)
	if err != nil {
		return err
	}

	h.logger.Trace("forwarding to another DC",
		"dc", req.Datacenter,
		"topic", req.Topic.String(),
		"key", req.Key,
		"index", req.Index,
		"stream_id", streamID,
	)

	defer func() {
		h.logger.Trace("forwarded stream closed",
			"dc", req.Datacenter,
			"stream_id", streamID,
		)
	}()

	// Open a Subscribe call to the remote DC.
	client := pbevent.NewConsulClient(conn)
	streamHandle, err := client.Subscribe(serverStream.Context(), req)
	if err != nil {
		return err
	}

	// Relay the events back to the client.
	for {
		event, err := streamHandle.Recv()
		if err != nil {
			return err
		}
		if err := serverStream.Send(event); err != nil {
			return err
		}
	}
}
