package connectca

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	external "github.com/hashicorp/consul/agent/grpc-external"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto-public/pbconnectca"
)

// WatchRoots provides a stream on which you can receive the list of active
// Connect CA roots. Current roots are sent immediately at the start of the
// stream, and new lists will be sent whenever the roots are rotated.
func (s *Server) WatchRoots(_ *pbconnectca.WatchRootsRequest, serverStream pbconnectca.ConnectCAService_WatchRootsServer) error {
	if err := s.requireConnect(); err != nil {
		return err
	}

	logger := s.Logger.Named("watch-roots").With("request_id", external.TraceID())
	logger.Trace("starting stream")
	defer logger.Trace("stream closed")

	options, err := external.QueryOptionsFromContext(serverStream.Context())
	if err != nil {
		return err
	}

	// Serve the roots from an EventPublisher subscription. If the subscription is
	// closed due to an ACL change, we'll attempt to re-authorize and resume it to
	// prevent unnecessarily terminating the stream.
	var idx uint64
	for {
		var err error
		idx, err = s.serveRoots(options.Token, idx, serverStream, logger)
		if errors.Is(err, stream.ErrSubForceClosed) {
			logger.Trace("subscription force-closed due to an ACL change or snapshot restore, will attempt to re-auth and resume")
		} else {
			return err
		}
	}
}

func (s *Server) serveRoots(
	token string,
	idx uint64,
	serverStream pbconnectca.ConnectCAService_WatchRootsServer,
	logger hclog.Logger,
) (uint64, error) {
	if err := external.RequireAnyValidACLToken(s.ACLResolver, token); err != nil {
		return 0, err
	}

	store := s.GetStore()

	// Read the TrustDomain up front - we do not allow users to change the ClusterID
	// so reading it once at the beginning of the stream is sufficient.
	trustDomain, err := getTrustDomain(store, logger)
	if err != nil {
		return 0, err
	}

	// Start the subscription.
	sub, err := s.Publisher.Subscribe(&stream.SubscribeRequest{
		Topic:   state.EventTopicCARoots,
		Subject: stream.SubjectNone,
		Token:   token,
		Index:   idx,
	})
	if err != nil {
		logger.Error("failed to subscribe to CA Roots events", "error", err)
		return 0, status.Error(codes.Internal, "failed to subscribe to CA Roots events")
	}
	defer sub.Unsubscribe()

	for {
		event, err := sub.Next(serverStream.Context())
		switch {
		case errors.Is(err, stream.ErrSubForceClosed):
			// If the subscription was closed because the state store was abandoned (e.g.
			// following a snapshot restore) reset idx to ensure we don't skip over the
			// new store's events.
			select {
			case <-store.AbandonCh():
				idx = 0
			default:
			}
			return idx, err
		case errors.Is(err, context.Canceled):
			return 0, nil
		case err != nil:
			logger.Error("failed to read next event", "error", err)
			return idx, status.Error(codes.Internal, err.Error())
		}

		// Note: this check isn't strictly necessary because the event publishing
		// machinery will ensure the index increases monotonically, but it can be
		// tricky to faithfully reproduce this in tests (e.g. the EventPublisher
		// garbage collects topic buffers and snapshots aggressively when streams
		// disconnect) so this avoids a bunch of confusing setup code.
		if event.Index <= idx {
			continue
		}

		idx = event.Index

		// We do not send framing events (e.g. EndOfSnapshot, NewSnapshotToFollow)
		// because we send a full list of roots on every event, rather than expecting
		// clients to maintain a state-machine in the way they do for service health.
		if event.IsFramingEvent() {
			continue
		}

		rsp, err := eventToResponse(event, trustDomain)
		if err != nil {
			logger.Error("failed to convert event to response", "error", err)
			return idx, status.Error(codes.Internal, err.Error())
		}
		if err := serverStream.Send(rsp); err != nil {
			logger.Error("failed to send response", "error", err)
			return idx, err
		}
	}
}

func eventToResponse(event stream.Event, trustDomain string) (*pbconnectca.WatchRootsResponse, error) {
	payload, ok := event.Payload.(state.EventPayloadCARoots)
	if !ok {
		return nil, fmt.Errorf("unexpected event payload type: %T", payload)
	}

	var active string
	roots := make([]*pbconnectca.CARoot, 0)

	for _, root := range payload.CARoots {
		if root.Active {
			active = root.ID
		}

		roots = append(roots, &pbconnectca.CARoot{
			Id:                root.ID,
			Name:              root.Name,
			SerialNumber:      root.SerialNumber,
			SigningKeyId:      root.SigningKeyID,
			RootCert:          root.RootCert,
			IntermediateCerts: root.IntermediateCerts,
			Active:            root.Active,
			RotatedOutAt:      timestamppb.New(root.RotatedOutAt),
		})
	}

	return &pbconnectca.WatchRootsResponse{
		TrustDomain:  trustDomain,
		ActiveRootId: active,
		Roots:        roots,
	}, nil
}

func (s *Server) authorize(token string) error {
	// Require the given ACL token to have `service:write` on any service (in any
	// partition and namespace).
	var authzContext acl.AuthorizerContext
	entMeta := structs.WildcardEnterpriseMetaInPartition(structs.WildcardSpecifier)
	authz, err := s.ACLResolver.ResolveTokenAndDefaultMeta(token, entMeta, &authzContext)
	if err != nil {
		return status.Error(codes.Unauthenticated, err.Error())
	}
	if err := authz.ToAllowAuthorizer().ServiceWriteAnyAllowed(&authzContext); err != nil {
		return status.Error(codes.PermissionDenied, err.Error())
	}
	return nil
}

func getTrustDomain(store StateStore, logger hclog.Logger) (string, error) {
	_, cfg, err := store.CAConfig(nil)
	switch {
	case err != nil:
		logger.Error("failed to read Connect CA Config", "error", err)
		return "", status.Error(codes.Internal, "failed to read Connect CA Config")
	case cfg == nil:
		logger.Warn("cannot begin stream because Connect CA is not yet initialized")
		return "", status.Error(codes.FailedPrecondition, "Connect CA is not yet initialized")
	}
	return connect.SpiffeIDSigningForCluster(cfg.ClusterID).Host(), nil
}
