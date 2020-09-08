package subscribe

import (
	"errors"
	"fmt"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/proto/pbservice"
	"github.com/hashicorp/go-uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/proto/pbsubscribe"
)

// Server implements a StateChangeSubscriptionServer for accepting SubscribeRequests,
// and sending events to the subscription topic.
type Server struct {
	Backend Backend
	Logger  Logger
}

type Logger interface {
	IsTrace() bool
	Trace(msg string, args ...interface{})
}

var _ pbsubscribe.StateChangeSubscriptionServer = (*Server)(nil)

type Backend interface {
	ResolveToken(token string) (acl.Authorizer, error)
	Forward(dc string, f func(*grpc.ClientConn) error) (handled bool, err error)
	Subscribe(req *stream.SubscribeRequest) (*stream.Subscription, error)
}

func (h *Server) Subscribe(req *pbsubscribe.SubscribeRequest, serverStream pbsubscribe.StateChangeSubscription_SubscribeServer) error {
	// streamID is just used for message correlation in trace logs and not
	// populated normally.
	var streamID string

	if h.Logger.IsTrace() {
		// TODO(banks) it might be nice one day to replace this with OpenTracing ID
		// if one is set etc. but probably pointless until we support that properly
		// in other places so it's actually propagated properly. For now this just
		// makes lifetime of a stream more traceable in our regular server logs for
		// debugging/dev.
		var err error
		streamID, err = uuid.GenerateUUID()
		if err != nil {
			return err
		}
	}

	// TODO: add fields to logger and pass logger around instead of streamID
	handled, err := h.Backend.Forward(req.Datacenter, h.forwardToDC(req, serverStream, streamID))
	if handled || err != nil {
		return err
	}

	h.Logger.Trace("new subscription",
		"topic", req.Topic.String(),
		"key", req.Key,
		"index", req.Index,
		"stream_id", streamID,
	)

	var sentCount uint64
	defer h.Logger.Trace("subscription closed", "stream_id", streamID)

	// Resolve the token and create the ACL filter.
	// TODO: handle token expiry gracefully...
	authz, err := h.Backend.ResolveToken(req.Token)
	if err != nil {
		return err
	}

	sub, err := h.Backend.Subscribe(toStreamSubscribeRequest(req))
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()

	ctx := serverStream.Context()
	snapshotDone := false
	for {
		events, err := sub.Next(ctx)
		switch {
		// TODO: test case
		case errors.Is(err, stream.ErrSubscriptionClosed):
			h.Logger.Trace("subscription reset by server", "stream_id", streamID)
			return status.Error(codes.Aborted, err.Error())
		case err != nil:
			return err
		}

		events = filterStreamEvents(authz, events)
		if len(events) == 0 {
			continue
		}

		first := events[0]
		switch {
		case first.IsEndOfSnapshot() || first.IsEndOfEmptySnapshot():
			snapshotDone = true
			h.Logger.Trace("snapshot complete",
				"index", first.Index, "sent", sentCount, "stream_id", streamID)
		case snapshotDone:
			h.Logger.Trace("sending events",
				"index", first.Index,
				"sent", sentCount,
				"batch_size", len(events),
				"stream_id", streamID,
			)
		}

		sentCount += uint64(len(events))
		e := newEventFromStreamEvents(req, events)
		if err := serverStream.Send(e); err != nil {
			return err
		}
	}
}

// TODO: can be replaced by mog conversion
func toStreamSubscribeRequest(req *pbsubscribe.SubscribeRequest) *stream.SubscribeRequest {
	return &stream.SubscribeRequest{
		Topic: req.Topic,
		Key:   req.Key,
		Token: req.Token,
		Index: req.Index,
	}
}

func (h *Server) forwardToDC(
	req *pbsubscribe.SubscribeRequest,
	serverStream pbsubscribe.StateChangeSubscription_SubscribeServer,
	streamID string,
) func(conn *grpc.ClientConn) error {
	return func(conn *grpc.ClientConn) error {
		h.Logger.Trace("forwarding to another DC",
			"dc", req.Datacenter,
			"topic", req.Topic.String(),
			"key", req.Key,
			"index", req.Index,
			"stream_id", streamID,
		)

		defer func() {
			h.Logger.Trace("forwarded stream closed",
				"dc", req.Datacenter,
				"stream_id", streamID,
			)
		}()

		client := pbsubscribe.NewStateChangeSubscriptionClient(conn)
		streamHandle, err := client.Subscribe(serverStream.Context(), req)
		if err != nil {
			return err
		}

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
}

// filterStreamEvents to only those allowed by the acl token.
func filterStreamEvents(authz acl.Authorizer, events []stream.Event) []stream.Event {
	// TODO: when is authz nil?
	if authz == nil || len(events) == 0 {
		return events
	}

	// Fast path for the common case of only 1 event since we can avoid slice
	// allocation in the hot path of every single update event delivered in vast
	// majority of cases with this. Note that this is called _per event/item_ when
	// sending snapshots which is a lot worse than being called once on regular
	// result.
	if len(events) == 1 {
		if enforceACL(authz, events[0]) == acl.Allow {
			return events
		}
		return nil
	}

	var filtered []stream.Event
	for idx := range events {
		event := events[idx]
		if enforceACL(authz, event) == acl.Allow {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

func newEventFromStreamEvents(req *pbsubscribe.SubscribeRequest, events []stream.Event) *pbsubscribe.Event {
	e := &pbsubscribe.Event{
		Topic: req.Topic,
		Key:   req.Key,
		Index: events[0].Index,
	}
	if len(events) == 1 {
		setPayload(e, events[0].Payload)
		return e
	}

	e.Payload = &pbsubscribe.Event_EventBatch{
		EventBatch: &pbsubscribe.EventBatch{
			Events: batchEventsFromEventSlice(events),
		},
	}
	return e
}

func setPayload(e *pbsubscribe.Event, payload interface{}) {
	switch p := payload.(type) {
	case state.EventPayloadCheckServiceNode:
		e.Payload = &pbsubscribe.Event_ServiceHealth{
			ServiceHealth: &pbsubscribe.ServiceHealthUpdate{
				Op: p.Op,
				// TODO: this could be cached
				CheckServiceNode: pbservice.NewCheckServiceNodeFromStructs(p.Value),
			},
		}
	default:
		panic(fmt.Sprintf("unexpected payload: %T: %#v", p, p))
	}
}

func batchEventsFromEventSlice(events []stream.Event) []*pbsubscribe.Event {
	result := make([]*pbsubscribe.Event, len(events))
	for i := range events {
		event := events[i]
		result[i] = &pbsubscribe.Event{Key: event.Key, Index: event.Index}
		setPayload(result[i], event.Payload)
	}
	return result
}
