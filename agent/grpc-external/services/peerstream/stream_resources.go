package peerstream

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/hashicorp/go-hclog"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"

	"github.com/hashicorp/consul/agent/connect"
	external "github.com/hashicorp/consul/agent/grpc-external"
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
	logger := s.Logger.Named("stream-resources").With("request_id", external.TraceID())

	logger.Trace("Started processing request")
	defer logger.Trace("Finished processing request")

	// NOTE: this code should have similar error handling to the new-request
	// handling code in HandleStream()

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
	if req.ResponseNonce != "" {
		return grpcstatus.Error(codes.InvalidArgument, "initial subscription request must not contain a nonce")
	}
	if req.Error != nil {
		return grpcstatus.Error(codes.InvalidArgument, "initial subscription request must not contain an error")
	}
	if !pbpeerstream.KnownTypeURL(req.ResourceURL) {
		return grpcstatus.Errorf(codes.InvalidArgument, "subscription request to unknown resource URL: %s", req.ResourceURL)
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

	if p.PeerID != "" {
		return grpcstatus.Error(codes.InvalidArgument, "expected PeerID to be empty; the wrong end of peering is being dialed")
	}

	streamReq := HandleStreamRequest{
		LocalID:            p.ID,
		RemoteID:           "",
		PeerName:           p.Name,
		Partition:          p.Partition,
		InitialResourceURL: req.ResourceURL,
		Stream:             stream,
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

	// InitialResourceURL is the ResourceURL from the initial Request.
	InitialResourceURL string

	// Stream is the open stream to the peer cluster.
	Stream BidirectionalStream
}

func (r HandleStreamRequest) WasDialed() bool {
	return r.RemoteID == ""
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
func (s *Server) HandleStream(streamReq HandleStreamRequest) error {
	// TODO: pass logger down from caller?
	logger := s.Logger.Named("stream").
		With("peer_name", streamReq.PeerName).
		With("peer_id", streamReq.LocalID).
		With("dialed", streamReq.WasDialed())
	logger.Trace("handling stream for peer")

	status, err := s.Tracker.Connected(streamReq.LocalID)
	if err != nil {
		return fmt.Errorf("failed to register stream: %v", err)
	}

	// TODO(peering) Also need to clear subscriptions associated with the peer
	defer s.Tracker.Disconnected(streamReq.LocalID)

	var trustDomain string
	if s.ConnectEnabled {
		// Read the TrustDomain up front - we do not allow users to change the ClusterID
		// so reading it once at the beginning of the stream is sufficient.
		trustDomain, err = getTrustDomain(s.GetStore(), logger)
		if err != nil {
			return err
		}
	}

	remoteSubTracker := newResourceSubscriptionTracker()
	if streamReq.InitialResourceURL != "" {
		if remoteSubTracker.Subscribe(streamReq.InitialResourceURL) {
			logger.Info("subscribing to resource type", "resourceURL", streamReq.InitialResourceURL)
		}
	}

	mgr := newSubscriptionManager(
		streamReq.Stream.Context(),
		logger,
		s.Config,
		trustDomain,
		s.Backend,
		s.GetStore,
		remoteSubTracker,
	)
	subCh := mgr.subscribe(streamReq.Stream.Context(), streamReq.LocalID, streamReq.PeerName, streamReq.Partition)

	// We need a mutex to protect against simultaneous sends to the client.
	var sendMutex sync.Mutex

	// streamSend is a helper function that sends msg over the stream
	// respecting the send mutex. It also logs the send and calls status.TrackSendError
	// on error.
	streamSend := func(msg *pbpeerstream.ReplicationMessage) error {
		logTraceSend(logger, msg)

		sendMutex.Lock()
		err := streamReq.Stream.Send(msg)
		sendMutex.Unlock()

		if err != nil {
			status.TrackSendError(err.Error())
		}
		return err
	}

	// Subscribe to all relevant resource types.
	for _, resourceURL := range []string{
		pbpeerstream.TypeURLExportedService,
		pbpeerstream.TypeURLPeeringTrustBundle,
	} {
		sub := makeReplicationRequest(&pbpeerstream.ReplicationMessage_Request{
			ResourceURL: resourceURL,
			PeerID:      streamReq.RemoteID,
		})
		if err := streamSend(sub); err != nil {
			if err == io.EOF {
				logger.Info("stream ended by peer")
				return nil
			}
			// TODO(peering) Test error handling in calls to Send/Recv
			return fmt.Errorf("failed to send subscription for %q to stream: %w", resourceURL, err)
		}
	}

	// TODO(peering): Should this be buffered?
	recvChan := make(chan *pbpeerstream.ReplicationMessage)
	go func() {
		defer close(recvChan)
		for {
			msg, err := streamReq.Stream.Recv()
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
			if err := streamSend(term); err != nil {
				return fmt.Errorf("failed to send to stream: %v", err)
			}

			logger.Trace("deleting stream status")
			s.Tracker.DeleteStatus(streamReq.LocalID)

			return nil

		case msg, open := <-recvChan:
			if !open {
				// The only time we expect the stream to end is when we've received a "Terminated" message.
				// We handle the case of receiving the Terminated message below and then this function exits.
				// So if the channel is closed while this function is still running then we haven't received a Terminated
				// message which means we want to try and reestablish the stream.
				// It's the responsibility of the caller of this function to reestablish the stream on error and so that's
				// why we return an error here.
				return fmt.Errorf("stream ended unexpectedly")
			}

			// NOTE: this code should have similar error handling to the
			// initial handling code in StreamResources()

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
				if !pbpeerstream.KnownTypeURL(req.ResourceURL) {
					return grpcstatus.Errorf(codes.InvalidArgument, "subscription request to unknown resource URL: %s", req.ResourceURL)
				}

				// There are different formats of requests depending upon where in the stream lifecycle we are.
				//
				// 1. Initial Request: This is the first request being received
				//    FROM the establishing peer. This is handled specially in
				//    (*Server).StreamResources BEFORE calling
				//    (*Server).HandleStream. This takes care of determining what
				//    the PeerID is for the stream. This is ALSO treated as (2) below.
				//
				// 2. Subscription Request: This is the first request for a
				//    given ResourceURL within a stream. The Initial Request (1)
				//    is always one of these as well.
				//
				//    These must contain a valid ResourceURL with no Error or
				//    ResponseNonce set.
				//
				//    It is valid to subscribe to the same ResourceURL twice
				//    within the lifetime of a stream, but all duplicate
				//    subscriptions are treated as no-ops upon receipt.
				//
				// 3. ACK Request: This is the message sent in reaction to an
				//    earlier Response to indicate that the response was processed
				//    by the other side successfully.
				//
				//    These must contain a ResponseNonce and no Error.
				//
				// 4. NACK Request: This is the message sent in reaction to an
				//    earlier Response to indicate that the response was NOT
				//    processed by the other side successfully.
				//
				//    These must contain a ResponseNonce and an Error.
				//
				if !remoteSubTracker.IsSubscribed(req.ResourceURL) {
					// This must be a new subscription request to add a new
					// resource type, vet it like a new request.

					if !streamReq.WasDialed() {
						if req.PeerID != "" && req.PeerID != streamReq.RemoteID {
							// Not necessary after the first request from the dialer,
							// but if provided must match.
							return grpcstatus.Errorf(codes.InvalidArgument,
								"initial subscription requests for a resource type must have consistent PeerID values: got=%q expected=%q",
								req.PeerID,
								streamReq.RemoteID,
							)
						}
					}
					if req.ResponseNonce != "" {
						return grpcstatus.Error(codes.InvalidArgument, "initial subscription requests for a resource type must not contain a nonce")
					}
					if req.Error != nil {
						return grpcstatus.Error(codes.InvalidArgument, "initial subscription request for a resource type must not contain an error")
					}

					if remoteSubTracker.Subscribe(req.ResourceURL) {
						logger.Info("subscribing to resource type", "resourceURL", req.ResourceURL)
					}
					status.TrackAck()
					continue
				}

				// At this point we have a valid ResourceURL and we are subscribed to it.

				switch {
				case req.ResponseNonce == "" && req.Error != nil:
					return grpcstatus.Error(codes.InvalidArgument, "initial subscription request for a resource type must not contain an error")

				case req.ResponseNonce != "" && req.Error == nil: // ACK
					// TODO(peering): handle ACK fully
					status.TrackAck()

				case req.ResponseNonce != "" && req.Error != nil: // NACK
					// TODO(peering): handle NACK fully
					logger.Warn("client peer was unable to apply resource", "code", req.Error.Code, "error", req.Error.Message)
					status.TrackNack(fmt.Sprintf("client peer was unable to apply resource: %s", req.Error.Message))

				default:
					// This branch might be dead code, but it could also happen
					// during a stray 're-subscribe' so just ignore the
					// message.
				}

				continue
			}

			if resp := msg.GetResponse(); resp != nil {
				// TODO(peering): Ensure there's a nonce
				reply, err := s.processResponse(streamReq.PeerName, streamReq.Partition, status, resp, logger)
				if err != nil {
					logger.Error("failed to persist resource", "resourceURL", resp.ResourceURL, "resourceID", resp.ResourceID)
					status.TrackReceiveError(err.Error())
				} else {
					status.TrackReceiveSuccess()
				}

				if err := streamSend(reply); err != nil {
					return fmt.Errorf("failed to send to stream: %v", err)
				}

				continue
			}

			if term := msg.GetTerminated(); term != nil {
				logger.Info("peering was deleted by our peer: marking peering as terminated and cleaning up imported resources")

				// Once marked as terminated, a separate deferred deletion routine will clean up imported resources.
				if err := s.Backend.PeeringTerminateByID(&pbpeering.PeeringTerminateByIDRequest{ID: streamReq.LocalID}); err != nil {
					logger.Error("failed to mark peering as terminated: %w", err)
				}
				return nil
			}

		case update := <-subCh:
			var resp *pbpeerstream.ReplicationMessage_Response
			switch {
			case strings.HasPrefix(update.CorrelationID, subExportedService):
				resp, err = makeServiceResponse(logger, status, update)
				if err != nil {
					// Log the error and skip this response to avoid locking up peering due to a bad update event.
					logger.Error("failed to create service response", "error", err)
					continue
				}

			case strings.HasPrefix(update.CorrelationID, subMeshGateway):
				// TODO(Peering): figure out how to sync this separately

			case update.CorrelationID == subCARoot:
				resp, err = makeCARootsResponse(logger, update)
				if err != nil {
					// Log the error and skip this response to avoid locking up peering due to a bad update event.
					logger.Error("failed to create ca roots response", "error", err)
					continue
				}

			default:
				logger.Warn("unrecognized update type from subscription manager: " + update.CorrelationID)
				continue
			}
			if resp == nil {
				continue
			}

			replResp := makeReplicationResponse(resp)
			if err := streamSend(replResp); err != nil {
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

// resourceSubscriptionTracker is used to keep track of the ResourceURLs that a
// stream has subscribed to and can notify you when a subscription comes in by
// closing the channels returned by SubscribedChan.
type resourceSubscriptionTracker struct {
	// notifierMap keeps track of a notification channel for each resourceURL.
	// Keys may exist in here even when they do not exist in 'subscribed' as
	// calling SubscribedChan has to possibly create and and hand out a
	// notification channel in advance of any notification.
	notifierMap map[string]chan struct{}

	// subscribed is a set that keeps track of resourceURLs that are currently
	// subscribed to. Keys are never deleted. If a key is present in this map
	// it is also present in 'notifierMap'.
	subscribed map[string]struct{}
}

func newResourceSubscriptionTracker() *resourceSubscriptionTracker {
	return &resourceSubscriptionTracker{
		subscribed:  make(map[string]struct{}),
		notifierMap: make(map[string]chan struct{}),
	}
}

// IsSubscribed returns true if the given ResourceURL has an active subscription.
func (t *resourceSubscriptionTracker) IsSubscribed(resourceURL string) bool {
	_, ok := t.subscribed[resourceURL]
	return ok
}

// Subscribe subscribes to the given ResourceURL. It will return true if this
// was the FIRST time a subscription occurred. It will also close the
// notification channel associated with this ResourceURL.
func (t *resourceSubscriptionTracker) Subscribe(resourceURL string) bool {
	if _, ok := t.subscribed[resourceURL]; ok {
		return false
	}
	t.subscribed[resourceURL] = struct{}{}

	// and notify
	ch := t.ensureNotifierChan(resourceURL)
	close(ch)

	return true
}

// SubscribedChan returns a channel that will be closed when the ResourceURL is
// subscribed using the Subscribe method.
func (t *resourceSubscriptionTracker) SubscribedChan(resourceURL string) <-chan struct{} {
	return t.ensureNotifierChan(resourceURL)
}

func (t *resourceSubscriptionTracker) ensureNotifierChan(resourceURL string) chan struct{} {
	if ch, ok := t.notifierMap[resourceURL]; ok {
		return ch
	}
	ch := make(chan struct{})
	t.notifierMap[resourceURL] = ch
	return ch
}
