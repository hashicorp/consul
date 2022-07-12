package peerstream

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/hashicorp/go-hclog"
	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/grpc/public"
	"github.com/hashicorp/consul/proto/pbpeering"
	"github.com/hashicorp/consul/proto/pbpeerstream"
)

type BidirectionalStream interface {
	Send(*pbpeerstream.ReplicationMessage) error
	Recv() (*pbpeerstream.ReplicationMessage, error)
	Context() context.Context
}

// StreamResources handles incoming streaming connections.
func (s *Server) StreamResources(stream pbpeerstream.PeerStreamService_StreamResourcesServer) error {
	logger := s.Logger.Named("stream-resources").With("request_id", public.TraceID())

	logger.Trace("Started processing request")
	defer logger.Trace("Finished processing request")

	if !s.Backend.IsLeader() {
		// we are not the leader so we will hang up on the dialer

		logger.Error("cannot establish a peering stream on a follower node")

		st, err := grpcstatus.New(codes.FailedPrecondition,
			"cannot establish a peering stream on a follower node").WithDetails(
			&pbpeerstream.LeaderAddress{Address: s.Backend.GetLeaderAddress()})
		if err != nil {
			logger.Error(fmt.Sprintf("failed to marshal the leader address in response; err: %v", err))
			return grpcstatus.Error(codes.FailedPrecondition, "cannot establish a peering stream on a follower node")
		} else {
			return st.Err()
		}
	}

	// Initial message on a new stream must be a new subscription request.
	first, err := stream.Recv()
	if err != nil {
		logger.Error("failed to establish stream", "error", err)
		return err
	}

	// TODO(peering) Make request contain a list of resources, so that roots and services can be
	//  			 subscribed to with a single request. See:
	//               https://github.com/envoyproxy/data-plane-api/blob/main/envoy/service/discovery/v3/discovery.proto#L46
	req := first.GetRequest()
	if req == nil {
		return grpcstatus.Error(codes.InvalidArgument, "first message when initiating a peering must be a subscription request")
	}
	logger.Trace("received initial replication request from peer")
	logTraceRecv(logger, req)

	if req.PeerID == "" {
		return grpcstatus.Error(codes.InvalidArgument, "initial subscription request must specify a PeerID")
	}
	if req.Nonce != "" {
		return grpcstatus.Error(codes.InvalidArgument, "initial subscription request must not contain a nonce")
	}
	if !pbpeerstream.KnownTypeURL(req.ResourceURL) {
		return grpcstatus.Error(codes.InvalidArgument, fmt.Sprintf("subscription request to unknown resource URL: %s", req.ResourceURL))
	}

	_, p, err := s.GetStore().PeeringReadByID(nil, req.PeerID)
	if err != nil {
		logger.Error("failed to look up peer", "peer_id", req.PeerID, "error", err)
		return grpcstatus.Error(codes.Internal, "failed to find PeerID: "+req.PeerID)
	}
	if p == nil {
		return grpcstatus.Error(codes.InvalidArgument, "initial subscription for unknown PeerID: "+req.PeerID)
	}

	// TODO(peering): If the peering is marked as deleted, send a Terminated message and return
	// TODO(peering): Store subscription request so that an event publisher can separately handle pushing messages for it
	logger.Info("accepted initial replication request from peer", "peer_id", p.ID)

	streamReq := HandleStreamRequest{
		LocalID:   p.ID,
		RemoteID:  p.PeerID,
		PeerName:  p.Name,
		Partition: p.Partition,
		Stream:    stream,
	}
	err = s.HandleStream(streamReq)
	// A nil error indicates that the peering was deleted and the stream needs to be gracefully shutdown.
	if err == nil {
		s.DrainStream(streamReq)
		return nil
	}

	logger.Error("error handling stream", "peer_name", p.Name, "peer_id", req.PeerID, "error", err)
	return err
}

type HandleStreamRequest struct {
	// LocalID is the UUID for the peering in the local Consul datacenter.
	LocalID string

	// RemoteID is the UUID for the peering from the perspective of the peer.
	RemoteID string

	// PeerName is the name of the peering.
	PeerName string

	// Partition is the local partition associated with the peer.
	Partition string

	// Stream is the open stream to the peer cluster.
	Stream BidirectionalStream
}

// DrainStream attempts to gracefully drain the stream when the connection is going to be torn down.
// Tearing down the connection too quickly can lead our peer receiving a context cancellation error before the stream termination message.
// Handling the termination message is important to set the expectation that the peering will not be reestablished unless recreated.
func (s *Server) DrainStream(req HandleStreamRequest) {
	for {
		// Ensure that we read until an error, or the peer has nothing more to send.
		if _, err := req.Stream.Recv(); err != nil {
			if err != io.EOF {
				s.Logger.Warn("failed to tear down stream gracefully: peer may not have received termination message",
					"peer_name", req.PeerName, "peer_id", req.LocalID, "error", err)
			}
			break
		}
		// Since the peering is being torn down we discard all replication messages without an error.
		// We want to avoid importing new data at this point.
	}
}

// The localID provided is the locally-generated identifier for the peering.
// The remoteID is an identifier that the remote peer recognizes for the peering.
func (s *Server) HandleStream(req HandleStreamRequest) error {
	// TODO: pass logger down from caller?
	logger := s.Logger.Named("stream").With("peer_name", req.PeerName, "peer_id", req.LocalID)
	logger.Trace("handling stream for peer")

	status, err := s.Tracker.Connected(req.LocalID)
	if err != nil {
		return fmt.Errorf("failed to register stream: %v", err)
	}

	// TODO(peering) Also need to clear subscriptions associated with the peer
	defer s.Tracker.Disconnected(req.LocalID)

	var trustDomain string
	if s.ConnectEnabled {
		// Read the TrustDomain up front - we do not allow users to change the ClusterID
		// so reading it once at the beginning of the stream is sufficient.
		trustDomain, err = getTrustDomain(s.GetStore(), logger)
		if err != nil {
			return err
		}
	}

	mgr := newSubscriptionManager(
		req.Stream.Context(),
		logger,
		s.Config,
		trustDomain,
		s.Backend,
		s.GetStore,
	)
	subCh := mgr.subscribe(req.Stream.Context(), req.LocalID, req.PeerName, req.Partition)

	sub := &pbpeerstream.ReplicationMessage{
		Payload: &pbpeerstream.ReplicationMessage_Request_{
			Request: &pbpeerstream.ReplicationMessage_Request{
				ResourceURL: pbpeerstream.TypeURLService,
				PeerID:      req.RemoteID,
			},
		},
	}
	logTraceSend(logger, sub)

	if err := req.Stream.Send(sub); err != nil {
		if err == io.EOF {
			logger.Info("stream ended by peer")
			status.TrackReceiveError(err.Error())
			return nil
		}
		// TODO(peering) Test error handling in calls to Send/Recv
		status.TrackSendError(err.Error())
		return fmt.Errorf("failed to send to stream: %v", err)
	}

	// TODO(peering): Should this be buffered?
	recvChan := make(chan *pbpeerstream.ReplicationMessage)
	go func() {
		defer close(recvChan)
		for {
			msg, err := req.Stream.Recv()
			if err == nil {
				logTraceRecv(logger, msg)
				recvChan <- msg
				continue
			}

			if err == io.EOF {
				logger.Info("stream ended by peer")
				status.TrackReceiveError(err.Error())
				return
			}
			logger.Error("failed to receive from stream", "error", err)
			status.TrackReceiveError(err.Error())
			return
		}
	}()

	for {
		select {
		// When the doneCh is closed that means that the peering was deleted locally.
		case <-status.Done():
			logger.Info("ending stream")

			term := &pbpeerstream.ReplicationMessage{
				Payload: &pbpeerstream.ReplicationMessage_Terminated_{
					Terminated: &pbpeerstream.ReplicationMessage_Terminated{},
				},
			}
			logTraceSend(logger, term)

			if err := req.Stream.Send(term); err != nil {
				status.TrackSendError(err.Error())
				return fmt.Errorf("failed to send to stream: %v", err)
			}

			logger.Trace("deleting stream status")
			s.Tracker.DeleteStatus(req.LocalID)

			return nil

		case msg, open := <-recvChan:
			if !open {
				logger.Trace("no longer receiving data on the stream")
				return nil
			}

			if !s.Backend.IsLeader() {
				// we are not the leader anymore so we will hang up on the dialer
				logger.Error("node is not a leader anymore; cannot continue streaming")

				st, err := grpcstatus.New(codes.FailedPrecondition,
					"node is not a leader anymore; cannot continue streaming").WithDetails(
					&pbpeerstream.LeaderAddress{Address: s.Backend.GetLeaderAddress()})
				if err != nil {
					logger.Error(fmt.Sprintf("failed to marshal the leader address in response; err: %v", err))
					return grpcstatus.Error(codes.FailedPrecondition, "node is not a leader anymore; cannot continue streaming")
				} else {
					return st.Err()
				}
			}

			if req := msg.GetRequest(); req != nil {
				switch {
				case req.Nonce == "":
					// TODO(peering): This can happen on a client peer since they don't try to receive subscriptions before entering HandleStream.
					//                Should change that behavior or only allow it that one time.

				case req.Error != nil && (req.Error.Code != int32(code.Code_OK) || req.Error.Message != ""):
					logger.Warn("client peer was unable to apply resource", "code", req.Error.Code, "error", req.Error.Message)
					status.TrackNack(fmt.Sprintf("client peer was unable to apply resource: %s", req.Error.Message))

				default:
					status.TrackAck()
				}

				continue
			}

			if resp := msg.GetResponse(); resp != nil {
				// TODO(peering): Ensure there's a nonce
				reply, err := s.processResponse(req.PeerName, req.Partition, resp)
				if err != nil {
					logger.Error("failed to persist resource", "resourceURL", resp.ResourceURL, "resourceID", resp.ResourceID)
					status.TrackReceiveError(err.Error())
				} else {
					status.TrackReceiveSuccess()
				}

				logTraceSend(logger, reply)
				if err := req.Stream.Send(reply); err != nil {
					status.TrackSendError(err.Error())
					return fmt.Errorf("failed to send to stream: %v", err)
				}

				continue
			}

			if term := msg.GetTerminated(); term != nil {
				logger.Info("peering was deleted by our peer: marking peering as terminated and cleaning up imported resources")

				// Once marked as terminated, a separate deferred deletion routine will clean up imported resources.
				if err := s.Backend.PeeringTerminateByID(&pbpeering.PeeringTerminateByIDRequest{ID: req.LocalID}); err != nil {
					logger.Error("failed to mark peering as terminated: %w", err)
				}
				return nil
			}

		case update := <-subCh:
			var resp *pbpeerstream.ReplicationMessage
			switch {
			case strings.HasPrefix(update.CorrelationID, subExportedService):
				resp = makeServiceResponse(logger, update)

			case strings.HasPrefix(update.CorrelationID, subMeshGateway):
				// TODO(Peering): figure out how to sync this separately

			case update.CorrelationID == subCARoot:
				resp = makeCARootsResponse(logger, update)

			default:
				logger.Warn("unrecognized update type from subscription manager: " + update.CorrelationID)
				continue
			}
			if resp == nil {
				continue
			}
			logTraceSend(logger, resp)
			if err := req.Stream.Send(resp); err != nil {
				status.TrackSendError(err.Error())
				return fmt.Errorf("failed to push data for %q: %w", update.CorrelationID, err)
			}
		}
	}
}

func getTrustDomain(store StateStore, logger hclog.Logger) (string, error) {
	_, cfg, err := store.CAConfig(nil)
	switch {
	case err != nil:
		logger.Error("failed to read Connect CA Config", "error", err)
		return "", grpcstatus.Error(codes.Internal, "failed to read Connect CA Config")
	case cfg == nil:
		logger.Warn("cannot begin stream because Connect CA is not yet initialized")
		return "", grpcstatus.Error(codes.FailedPrecondition, "Connect CA is not yet initialized")
	}
	return connect.SpiffeIDSigningForCluster(cfg.ClusterID).Host(), nil
}

func (s *Server) StreamStatus(peer string) (resp Status, found bool) {
	return s.Tracker.StreamStatus(peer)
}

// ConnectedStreams returns a map of connected stream IDs to the corresponding channel for tearing them down.
func (s *Server) ConnectedStreams() map[string]chan struct{} {
	return s.Tracker.ConnectedStreams()
}

func logTraceRecv(logger hclog.Logger, pb proto.Message) {
	logTraceProto(logger, pb, true)
}

func logTraceSend(logger hclog.Logger, pb proto.Message) {
	logTraceProto(logger, pb, false)
}

func logTraceProto(logger hclog.Logger, pb proto.Message, received bool) {
	if !logger.IsTrace() {
		return
	}

	dir := "sent"
	if received {
		dir = "received"
	}

	m := jsonpb.Marshaler{
		Indent: "  ",
	}
	out, err := m.MarshalToString(pb)
	if err != nil {
		out = "<ERROR: " + err.Error() + ">"
	}

	logger.Trace("replication message", "direction", dir, "protobuf", out)
}
