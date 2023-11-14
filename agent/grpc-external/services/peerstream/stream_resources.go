// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package peerstream

import (
	"context"
	"crypto/subtle"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/agent/connect"
	external "github.com/hashicorp/consul/agent/grpc-external"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/proto/private/pbpeering"
	"github.com/hashicorp/consul/proto/private/pbpeerstream"
)

type BidirectionalStream interface {
	Send(*pbpeerstream.ReplicationMessage) error
	Recv() (*pbpeerstream.ReplicationMessage, error)
	Context() context.Context
}

// ExchangeSecret exchanges the one-time secret embedded in a peering token for a
// long-lived secret for use with the peering stream handler. This secret exchange
// prevents peering tokens from being reused.
//
// Note that if the peering secret exchange fails, a peering token may need to be
// re-generated, since the one-time initiation secret may have been invalidated.
func (s *Server) ExchangeSecret(ctx context.Context, req *pbpeerstream.ExchangeSecretRequest) (*pbpeerstream.ExchangeSecretResponse, error) {
	// For private/internal gRPC handlers, protoc-gen-rpc-glue generates the
	// requisite methods to satisfy the structs.RPCInfo interface using fields
	// from the pbcommon package. This service is public, so we can't use those
	// fields in our proto definition. Instead, we construct our RPCInfo manually.
	//
	// Embedding WriteRequest ensures RPCs are forwarded to the leader, embedding
	// DCSpecificRequest adds the RequestDatacenter method (but as we're not
	// setting Datacenter it has the effect of *not* doing DC forwarding).
	var rpcInfo struct {
		structs.WriteRequest
		structs.DCSpecificRequest
	}

	var resp *pbpeerstream.ExchangeSecretResponse
	handled, err := s.ForwardRPC(&rpcInfo, func(conn *grpc.ClientConn) error {
		var err error
		resp, err = pbpeerstream.NewPeerStreamServiceClient(conn).ExchangeSecret(ctx, req)
		return err
	})
	if handled || err != nil {
		return resp, err
	}

	defer metrics.MeasureSince([]string{"peering", "exchange_secret"}, time.Now())

	// Validate the given establishment secret against the one stored on the server.
	existing, err := s.GetStore().PeeringSecretsRead(nil, req.PeerID)
	if err != nil {
		return nil, grpcstatus.Errorf(codes.Internal, "failed to read peering secret: %v", err)
	}
	if existing == nil || subtle.ConstantTimeCompare([]byte(existing.GetEstablishment().GetSecretID()), []byte(req.EstablishmentSecret)) == 0 {
		return nil, grpcstatus.Error(codes.PermissionDenied, "invalid peering establishment secret")
	}

	id, err := s.generateNewStreamSecret()
	if err != nil {
		return nil, grpcstatus.Errorf(codes.Internal, "failed to generate peering stream secret: %v", err)
	}

	writeReq := &pbpeering.SecretsWriteRequest{
		PeerID: req.PeerID,
		Request: &pbpeering.SecretsWriteRequest_ExchangeSecret{
			ExchangeSecret: &pbpeering.SecretsWriteRequest_ExchangeSecretRequest{
				// Pass the given establishment secret to that it can be re-validated at the state store.
				// Validating the establishment secret at the RPC is not enough because there can be
				// concurrent callers with the same establishment secret.
				EstablishmentSecret: req.EstablishmentSecret,

				// Overwrite any existing un-utilized pending stream secret.
				PendingStreamSecret: id,
			},
		},
	}
	err = s.Backend.PeeringSecretsWrite(writeReq)
	if err != nil {
		return nil, grpcstatus.Errorf(codes.Internal, "failed to persist peering secret: %v", err)
	}

	return &pbpeerstream.ExchangeSecretResponse{StreamSecret: id}, nil
}

func (s *Server) generateNewStreamSecret() (string, error) {
	id, err := lib.GenerateUUID(s.Backend.ValidateProposedPeeringSecret)
	if err != nil {
		return "", err
	}
	return id, nil
}

// StreamResources handles incoming streaming connections.
func (s *Server) StreamResources(stream pbpeerstream.PeerStreamService_StreamResourcesServer) error {
	logger := s.Logger.Named("stream-resources").With("request_id", external.TraceID())

	logger.Trace("Started processing request")
	defer logger.Trace("Finished processing request")

	// NOTE: this code should have similar error handling to the new-request
	// handling code in HandleStream()

	if !s.Backend.IsLeader() {
		// We are not the leader so we will hang up on the dialer.
		logger.Debug("cannot establish a peering stream on a follower node")

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
	req := first.GetOpen()
	if req == nil {
		return grpcstatus.Error(codes.InvalidArgument, "first message when initiating a peering must be: Open")
	}
	logger.Trace("received initial replication request from peer")
	logTraceRecv(logger, req)

	if req.PeerID == "" {
		return grpcstatus.Error(codes.InvalidArgument, "initial subscription request must specify a PeerID")
	}

	var p *pbpeering.Peering
	_, p, err = s.GetStore().PeeringReadByID(nil, req.PeerID)
	if err != nil {
		logger.Error("failed to look up peer", "peer_id", req.PeerID, "error", err)
		return grpcstatus.Error(codes.Internal, "failed to find PeerID: "+req.PeerID)
	}
	if p == nil {
		return grpcstatus.Error(codes.InvalidArgument, "initial subscription for unknown PeerID: "+req.PeerID)
	}
	// Clone the peering because we will modify and rewrite it.
	p, ok := proto.Clone(p).(*pbpeering.Peering)
	if !ok {
		return grpcstatus.Errorf(codes.Internal, "unexpected error while cloning a Peering object.")
	}

	if !p.IsActive() {
		// If peering is terminated, then our peer sent the termination message.
		// For other non-active states, send the termination message.
		if p.State != pbpeering.PeeringState_TERMINATED {
			term := &pbpeerstream.ReplicationMessage{
				Payload: &pbpeerstream.ReplicationMessage_Terminated_{
					Terminated: &pbpeerstream.ReplicationMessage_Terminated{},
				},
			}
			logTraceSend(logger, term)

			// we don't care if send fails; stream will be killed by termination message or grpc error
			_ = stream.Send(term)
		}
		return grpcstatus.Error(codes.Aborted, "peering is marked as deleted: "+req.PeerID)
	}

	secrets, err := s.GetStore().PeeringSecretsRead(nil, req.PeerID)
	if err != nil {
		logger.Error("failed to look up secrets for peering", "peer_id", req.PeerID, "error", err)
		return grpcstatus.Error(codes.Internal, "failed to find peering secrets for PeerID: "+req.PeerID)
	}
	if secrets == nil {
		logger.Error("no known secrets for peering", "peer_id", req.PeerID, "error", err)
		return grpcstatus.Error(codes.Internal, "unable to authorize connection, peering must be re-established")
	}

	// Check the given secret ID against the active stream secret.
	var authorized bool
	if active := secrets.GetStream().GetActiveSecretID(); active != "" {
		if subtle.ConstantTimeCompare([]byte(active), []byte(req.StreamSecretID)) == 1 {
			authorized = true
		}
	}

	// Next check the given stream secret against the locally stored pending stream secret.
	// A pending stream secret is one that has not been seen by this handler.
	if pending := secrets.GetStream().GetPendingSecretID(); pending != "" && !authorized {
		// If the given secret is the currently pending secret, it gets promoted to be the active secret.
		// This is the case where a server recently exchanged for a stream secret.
		if subtle.ConstantTimeCompare([]byte(pending), []byte(req.StreamSecretID)) == 0 {
			return grpcstatus.Error(codes.PermissionDenied, "invalid peering stream secret")
		}
		authorized = true

		promoted := &pbpeering.SecretsWriteRequest{
			PeerID: p.ID,
			Request: &pbpeering.SecretsWriteRequest_PromotePending{
				PromotePending: &pbpeering.SecretsWriteRequest_PromotePendingRequest{
					// Overwrite any existing un-utilized pending stream secret.
					ActiveStreamSecret: pending,
				},
			},
		}

		p.Remote = req.Remote
		err = s.Backend.PeeringWrite(&pbpeering.PeeringWriteRequest{
			Peering:        p,
			SecretsRequest: promoted,
		})
		if err != nil {
			return grpcstatus.Errorf(codes.Internal, "failed to persist peering: %v", err)
		}
	}
	if !authorized {
		return grpcstatus.Error(codes.PermissionDenied, "invalid peering stream secret")
	}

	logger.Info("accepted initial replication request from peer", "peer_id", p.ID)

	if p.PeerID != "" {
		return grpcstatus.Error(codes.InvalidArgument, "expected PeerID to be empty; the wrong end of peering is being dialed")
	}

	streamReq := HandleStreamRequest{
		LocalID:   p.ID,
		RemoteID:  "",
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

func (r HandleStreamRequest) IsAcceptor() bool {
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

func (s *Server) HandleStream(streamReq HandleStreamRequest) error {
	if err := s.realHandleStream(streamReq); err != nil {
		s.Tracker.DisconnectedDueToError(streamReq.LocalID, err.Error())
		return err
	}
	// TODO(peering) Also need to clear subscriptions associated with the peer
	s.Tracker.DisconnectedGracefully(streamReq.LocalID)
	return nil
}

// The localID provided is the locally-generated identifier for the peering.
// The remoteID is an identifier that the remote peer recognizes for the peering.
func (s *Server) realHandleStream(streamReq HandleStreamRequest) error {
	// TODO: pass logger down from caller?
	logger := s.Logger.Named("stream").
		With("peer_name", streamReq.PeerName).
		With("peer_id", streamReq.LocalID).
		With("dialer", !streamReq.IsAcceptor())
	logger.Trace("handling stream for peer")

	// handleStreamCtx is local to this function.
	handleStreamCtx, cancel := context.WithCancel(streamReq.Stream.Context())
	defer cancel()

	status, err := s.Tracker.Connected(streamReq.LocalID)
	if err != nil {
		return fmt.Errorf("failed to register stream: %v", err)
	}

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

		// We only track send successes and errors for response types because this is meant to track
		// resources, not request/ack messages.
		if msg.GetResponse() != nil {
			if err != nil {
				if id := msg.GetResponse().GetResourceID(); id != "" {
					logger.Error("failed to send resource", "resourceID", id, "error", err)
					status.TrackSendError(err.Error())
					return nil
				}
				status.TrackSendError(err.Error())
			} else {
				status.TrackSendSuccess()
			}
		}
		return err
	}

	resources := []string{
		pbpeerstream.TypeURLExportedService,
		pbpeerstream.TypeURLExportedServiceList,
		pbpeerstream.TypeURLPeeringTrustBundle,
	}
	// Acceptors should not subscribe to server address updates, because they should always have an empty list.
	if !streamReq.IsAcceptor() {
		resources = append(resources, pbpeerstream.TypeURLPeeringServerAddresses)
	}

	// Subscribe to all relevant resource types.
	for _, resourceURL := range resources {
		sub := makeReplicationRequest(&pbpeerstream.ReplicationMessage_Request{
			ResourceURL: resourceURL,
			PeerID:      streamReq.RemoteID,
		})
		if err := streamSend(sub); err != nil {
			// TODO(peering) Test error handling in calls to Send/Recv
			return fmt.Errorf("failed to send subscription for %q to stream: %w", resourceURL, err)
		}
	}

	// recvCh sends messages from the gRPC stream.
	recvCh := make(chan *pbpeerstream.ReplicationMessage)
	// recvErrCh sends errors received from the gRPC stream.
	recvErrCh := make(chan error)

	// Start a goroutine to read from the stream and pass to recvCh and recvErrCh.
	// Using a separate goroutine allows us to process sends and receives all in the main for{} loop.
	go func() {
		for {
			msg, err := streamReq.Stream.Recv()
			if err != nil {
				recvErrCh <- err
				return
			}
			logTraceRecv(logger, msg)
			select {
			case recvCh <- msg:
			case <-handleStreamCtx.Done():
				return
			}
		}
	}()

	// Start a goroutine to send heartbeats at a regular interval.
	go func() {
		tick := time.NewTicker(s.outgoingHeartbeatInterval)
		defer tick.Stop()

		for {
			select {
			case <-handleStreamCtx.Done():
				return

			case <-tick.C:
				heartbeat := &pbpeerstream.ReplicationMessage{
					Payload: &pbpeerstream.ReplicationMessage_Heartbeat_{
						Heartbeat: &pbpeerstream.ReplicationMessage_Heartbeat{},
					},
				}
				if err := streamSend(heartbeat); err != nil {
					logger.Warn("error sending heartbeat", "err", err)
				}
			}
		}
	}()

	// incomingHeartbeatCtx will complete if incoming heartbeats time out.
	incomingHeartbeatCtx, incomingHeartbeatCtxCancel :=
		context.WithTimeout(context.Background(), s.incomingHeartbeatTimeout)
	// NOTE: It's important that we wrap the call to cancel in a wrapper func because during the loop we're
	// re-assigning the value of incomingHeartbeatCtxCancel and we want the defer to run on the last assigned
	// value, not the current value.
	defer func() {
		incomingHeartbeatCtxCancel()
	}()

	// The nonce is used to correlate response/(ack|nack) pairs.
	var nonce uint64

	// The main loop that processes sends and receives.
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
				// Nolint directive needed due to bug in govet that doesn't see that the cancel
				// func of the incomingHeartbeatTimer _does_ get called.
				//nolint:govet
				return fmt.Errorf("failed to send to stream: %v", err)
			}

			logger.Trace("deleting stream status")
			s.Tracker.DeleteStatus(streamReq.LocalID)

			return nil

		// Handle errors received from the stream by shutting down our handler.
		case err := <-recvErrCh:
			if err == io.EOF {
				// NOTE: We don't expect to receive an io.EOF error here when the stream is disconnected gracefully.
				// When the peering is deleted locally, status.Done() returns which is handled elsewhere and this method
				// exits. When we receive a Terminated message, that's also handled elsewhere and this method
				// exits. After the method exits this code here won't receive any recv errors and those will be handled
				// by DrainStream().
				err = fmt.Errorf("stream ended unexpectedly")
			} else {
				err = fmt.Errorf("unexpected error receiving from the stream: %w", err)
			}
			status.TrackRecvError(err.Error())
			return err

		// We haven't received a heartbeat within the expected interval. Kill the stream.
		case <-incomingHeartbeatCtx.Done():
			return fmt.Errorf("heartbeat timeout")

		case msg := <-recvCh:
			// NOTE: this code should have similar error handling to the
			// initial handling code in StreamResources()

			if !s.Backend.IsLeader() {
				// We are not the leader anymore, so we will hang up on the dialer.
				logger.Info("node is not a leader anymore; cannot continue streaming")

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
				//    the PeerID is for the stream.
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

					if !streamReq.IsAcceptor() {
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
				case req.Error == nil: // ACK
					// TODO(peering): handle ACK fully
					status.TrackAck()

				case req.Error != nil: // NACK
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
				reply, err := s.processResponse(streamReq.PeerName, streamReq.Partition, status, resp)
				if err != nil {
					logger.Error("failed to persist resource", "resourceURL", resp.ResourceURL, "resourceID", resp.ResourceID)
					status.TrackRecvError(err.Error())
				} else {
					status.TrackRecvResourceSuccess()
				}

				// We are replying ACK or NACK depending on whether we successfully processed the response.
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

			if msg.GetHeartbeat() != nil {
				status.TrackRecvHeartbeat()

				// Reset the heartbeat timeout by creating a new context.
				// We first must cancel the old context so there's no leaks. This is safe to do because we're only
				// reading that context within this for{} loop, and so we won't accidentally trigger the heartbeat
				// timeout.
				incomingHeartbeatCtxCancel()
				// NOTE: IDEs and govet think that the reassigned cancel below never gets
				// called, but it does by the defer when the heartbeat ctx is first created.
				// They just can't trace the execution properly for some reason (possibly golang/go#29587).
				//nolint:govet
				incomingHeartbeatCtx, incomingHeartbeatCtxCancel =
					context.WithTimeout(context.Background(), s.incomingHeartbeatTimeout)
			}

		case update := <-subCh:
			var resp *pbpeerstream.ReplicationMessage_Response
			switch {
			case strings.HasPrefix(update.CorrelationID, subExportedServiceList):
				resp, err = makeExportedServiceListResponse(status, update)
				if err != nil {
					// Log the error and skip this response to avoid locking up peering due to a bad update event.
					logger.Error("failed to create exported service list response", "error", err)
					continue
				}
			case strings.HasPrefix(update.CorrelationID, subExportedService):
				resp, err = makeServiceResponse(update)
				if err != nil {
					// Log the error and skip this response to avoid locking up peering due to a bad update event.
					logger.Error("failed to create service response", "error", err)
					continue
				}

			case update.CorrelationID == subCARoot:
				resp, err = makeCARootsResponse(update)
				if err != nil {
					// Log the error and skip this response to avoid locking up peering due to a bad update event.
					logger.Error("failed to create ca roots response", "error", err)
					continue
				}

			case update.CorrelationID == subServerAddrs:
				resp, err = makeServerAddrsResponse(update)
				if err != nil {
					logger.Error("failed to create server address response", "error", err)
					continue
				}

			default:
				logger.Warn("unrecognized update type from subscription manager: " + update.CorrelationID)
				continue
			}
			if resp == nil {
				continue
			}

			// Assign a new unique nonce to the response.
			nonce++
			resp.Nonce = fmt.Sprintf("%08x", nonce)

			replResp := makeReplicationResponse(resp)
			if err := streamSend(replResp); err != nil {
				// note: govet warns of context leak but it is cleaned up in a defer
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
		return "", grpcstatus.Error(codes.Unavailable, "Connect CA is not yet initialized")
	}
	return connect.SpiffeIDSigningForCluster(cfg.ClusterID).Host(), nil
}

func (s *Server) StreamStatus(peerID string) (resp Status, found bool) {
	return s.Tracker.StreamStatus(peerID)
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

	// Redact the long-lived stream secret to avoid leaking it in trace logs.
	pbToLog := pb
	switch msg := pb.(type) {
	case *pbpeerstream.ReplicationMessage:
		clone := &pbpeerstream.ReplicationMessage{}
		proto.Merge(clone, msg)

		if clone.GetOpen() != nil {
			clone.GetOpen().StreamSecretID = "hidden"
			pbToLog = clone
		}
	case *pbpeerstream.ReplicationMessage_Open:
		clone := &pbpeerstream.ReplicationMessage_Open{}
		proto.Merge(clone, msg)

		clone.StreamSecretID = "hidden"
		pbToLog = clone
	}

	m := protojson.MarshalOptions{
		Indent: "  ",
	}
	out := ""
	outBytes, err := m.Marshal(pbToLog)
	if err != nil {
		out = "<ERROR: " + err.Error() + ">"
	} else {
		out = string(outBytes)
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
