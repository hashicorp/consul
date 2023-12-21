package serverdiscovery

import (
	"context"
	"errors"

	"github.com/hashicorp/go-hclog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/agent/consul/autopilotevents"
	"github.com/hashicorp/consul/agent/consul/stream"
	external "github.com/hashicorp/consul/agent/grpc-external"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto-public/pbserverdiscovery"
)

// WatchServers provides a stream on which you can receive the list of servers
// that are ready to receive incoming requests including stale queries. The
// current set of ready servers are sent immediately at the start of the
// stream and new updates will be sent whenver the set of ready servers changes.
func (s *Server) WatchServers(req *pbserverdiscovery.WatchServersRequest, serverStream pbserverdiscovery.ServerDiscoveryService_WatchServersServer) error {
	logger := s.Logger.Named("watch-servers").With("request_id", external.TraceID())

	logger.Debug("starting stream")
	defer logger.Trace("stream closed")

	options, err := external.QueryOptionsFromContext(serverStream.Context())
	if err != nil {
		return err
	}
	// Serve the ready servers from an EventPublisher subscription. If the subscription is
	// closed due to an ACL change, we'll attempt to re-authorize and resume it to
	// prevent unnecessarily terminating the stream.
	var idx uint64
	for {
		var err error
		idx, err = s.serveReadyServers(options.Token, idx, req, serverStream, logger)
		if errors.Is(err, stream.ErrSubForceClosed) {
			logger.Trace("subscription force-closed due to an ACL change or snapshot restore, will attempt to re-auth and resume")
		} else {
			return err
		}
	}
}

func (s *Server) serveReadyServers(token string, index uint64, req *pbserverdiscovery.WatchServersRequest, serverStream pbserverdiscovery.ServerDiscoveryService_WatchServersServer, logger hclog.Logger) (uint64, error) {
	if err := external.RequireAnyValidACLToken(s.ACLResolver, token); err != nil {
		return 0, err
	}

	// Start the subscription.
	sub, err := s.Publisher.Subscribe(&stream.SubscribeRequest{
		Topic:   autopilotevents.EventTopicReadyServers,
		Subject: stream.SubjectNone,
		Token:   token,
		Index:   index,
	})
	if err != nil {
		logger.Error("failed to subscribe to server discovery events", "error", err)
		return 0, status.Error(codes.Internal, "failed to subscribe to server discovery events")
	}
	defer sub.Unsubscribe()

	for {
		event, err := sub.Next(serverStream.Context())
		switch {
		case errors.Is(err, stream.ErrSubForceClosed):
			return index, err
		case errors.Is(err, context.Canceled):
			return 0, nil
		case err != nil:
			logger.Error("failed to read next event", "error", err)
			return index, status.Error(codes.Internal, err.Error())
		}

		// We do not send framing events (e.g. EndOfSnapshot, NewSnapshotToFollow)
		// because we send a full list of ready servers on every event, rather than expecting
		// clients to maintain a state-machine in the way they do for service health.
		if event.IsFramingEvent() {
			continue
		}

		// Note: this check isn't strictly necessary because the event publishing
		// machinery will ensure the index increases monotonically, but it can be
		// tricky to faithfully reproduce this in tests (e.g. the EventPublisher
		// garbage collects topic buffers and snapshots aggressively when streams
		// disconnect) so this avoids a bunch of confusing setup code.
		if event.Index <= index {
			continue
		}

		index = event.Index

		rsp, err := eventToResponse(req, event)
		if err != nil {
			logger.Error("failed to convert event to response", "error", err)
			return index, status.Error(codes.Internal, err.Error())
		}
		if err := serverStream.Send(rsp); err != nil {
			logger.Error("failed to send response", "error", err)
			return index, err
		}
	}
}

func eventToResponse(req *pbserverdiscovery.WatchServersRequest, event stream.Event) (*pbserverdiscovery.WatchServersResponse, error) {
	readyServers, err := autopilotevents.ExtractEventPayload(event)
	if err != nil {
		return nil, err
	}

	var servers []*pbserverdiscovery.Server

	for _, srv := range readyServers {
		addr := srv.Address

		wanAddr, ok := srv.TaggedAddresses[structs.TaggedAddressWAN]
		if req.Wan && ok {
			addr = wanAddr
		}

		servers = append(servers, &pbserverdiscovery.Server{
			Id:      srv.ID,
			Version: srv.Version,
			Address: addr,
		})
	}

	return &pbserverdiscovery.WatchServersResponse{
		Servers: servers,
	}, nil
}
