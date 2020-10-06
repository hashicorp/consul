package subscribe

import (
	"errors"
	"fmt"

	"github.com/hashicorp/go-hclog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/proto/pbservice"
	"github.com/hashicorp/consul/proto/pbsubscribe"
)

// Server implements a StateChangeSubscriptionServer for accepting SubscribeRequests,
// and sending events to the subscription topic.
type Server struct {
	Backend Backend
	Logger  Logger
}

func NewServer(backend Backend, logger Logger) *Server {
	return &Server{Backend: backend, Logger: logger}
}

type Logger interface {
	Trace(msg string, args ...interface{})
	With(args ...interface{}) hclog.Logger
}

var _ pbsubscribe.StateChangeSubscriptionServer = (*Server)(nil)

type Backend interface {
	// TODO(streaming): Use ResolveTokenAndDefaultMeta instead once SubscribeRequest
	// has an EnterpriseMeta.
	ResolveToken(token string) (acl.Authorizer, error)
	Forward(dc string, f func(*grpc.ClientConn) error) (handled bool, err error)
	Subscribe(req *stream.SubscribeRequest) (*stream.Subscription, error)
}

func (h *Server) Subscribe(req *pbsubscribe.SubscribeRequest, serverStream pbsubscribe.StateChangeSubscription_SubscribeServer) error {
	logger := h.newLoggerForRequest(req)
	handled, err := h.Backend.Forward(req.Datacenter, forwardToDC(req, serverStream, logger))
	if handled || err != nil {
		return err
	}

	logger.Trace("new subscription")
	defer logger.Trace("subscription closed")

	// Resolve the token and create the ACL filter.
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
	elog := &eventLogger{logger: logger}
	for {
		events, err := sub.Next(ctx)
		switch {
		case errors.Is(err, stream.ErrSubscriptionClosed):
			logger.Trace("subscription reset by server")
			return status.Error(codes.Aborted, err.Error())
		case err != nil:
			return err
		}

		events = filterStreamEvents(authz, events)
		if len(events) == 0 {
			continue
		}

		elog.Trace(events)
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

func forwardToDC(
	req *pbsubscribe.SubscribeRequest,
	serverStream pbsubscribe.StateChangeSubscription_SubscribeServer,
	logger Logger,
) func(conn *grpc.ClientConn) error {
	return func(conn *grpc.ClientConn) error {
		logger.Trace("forwarding to another DC")
		defer logger.Trace("forwarded stream closed")

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
	// authz will be nil when ACLs are disabled
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
		event := events[0]
		// TODO: refactor so these are only checked once, instead of 3 times.
		switch {
		case event.IsEndOfSnapshot():
			e.Payload = &pbsubscribe.Event_EndOfSnapshot{EndOfSnapshot: true}
			return e
		case event.IsNewSnapshotToFollow():
			e.Payload = &pbsubscribe.Event_NewSnapshotToFollow{NewSnapshotToFollow: true}
			return e
		}

		setPayload(e, event.Payload)
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
