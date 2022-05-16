package peering

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/armon/go-metrics"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/dns"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbpeering"
	"github.com/hashicorp/consul/proto/pbservice"
	"github.com/hashicorp/consul/proto/pbstatus"
	"github.com/hashicorp/consul/types"
)

var (
	errPeeringTokenInvalidCA            = errors.New("peering token CA value is invalid")
	errPeeringTokenEmptyServerAddresses = errors.New("peering token server addresses value is empty")
	errPeeringTokenEmptyServerName      = errors.New("peering token server name value is empty")
	errPeeringTokenEmptyPeerID          = errors.New("peering token peer ID value is empty")
)

// errPeeringInvalidServerAddress is returned when an initiate request contains
// an invalid server address.
type errPeeringInvalidServerAddress struct {
	addr string
}

// Error implements the error interface
func (e *errPeeringInvalidServerAddress) Error() string {
	return fmt.Sprintf("%s is not a valid peering server address", e.addr)
}

// Service implements pbpeering.PeeringService to provide RPC operations for
// managing peering relationships.
type Service struct {
	Backend Backend
	logger  hclog.Logger
	streams *streamTracker
}

func NewService(logger hclog.Logger, backend Backend) *Service {
	return &Service{
		Backend: backend,
		logger:  logger,
		streams: newStreamTracker(),
	}
}

var _ pbpeering.PeeringServiceServer = (*Service)(nil)

// Backend defines the core integrations the Peering endpoint depends on. A
// functional implementation will integrate with various subcomponents of Consul
// such as the State store for reading and writing data, the CA machinery for
// providing access to CA data and the RPC system for forwarding requests to
// other servers.
type Backend interface {
	// Forward should forward the request to the leader when necessary.
	Forward(info structs.RPCInfo, f func(*grpc.ClientConn) error) (handled bool, err error)

	// GetAgentCACertificates returns the CA certificate to be returned in the peering token data
	GetAgentCACertificates() ([]string, error)

	// GetServerAddresses returns the addresses used for establishing a peering connection
	GetServerAddresses() ([]string, error)

	// GetServerName returns the SNI to be returned in the peering token data which
	// will be used by peers when establishing peering connections over TLS.
	GetServerName() string

	// EncodeToken packages a peering token into a slice of bytes.
	EncodeToken(tok *structs.PeeringToken) ([]byte, error)

	// DecodeToken unpackages a peering token from a slice of bytes.
	DecodeToken([]byte) (*structs.PeeringToken, error)

	EnterpriseCheckPartitions(partition string) error

	Subscribe(req *stream.SubscribeRequest) (*stream.Subscription, error)

	Store() Store
	Apply() Apply
}

// Store provides a read-only interface for querying Peering data.
type Store interface {
	PeeringRead(ws memdb.WatchSet, q state.Query) (uint64, *pbpeering.Peering, error)
	PeeringReadByID(ws memdb.WatchSet, id string) (uint64, *pbpeering.Peering, error)
	PeeringList(ws memdb.WatchSet, entMeta acl.EnterpriseMeta) (uint64, []*pbpeering.Peering, error)
	PeeringTrustBundleRead(ws memdb.WatchSet, q state.Query) (uint64, *pbpeering.PeeringTrustBundle, error)
	ExportedServicesForPeer(ws memdb.WatchSet, peerID string) (uint64, []structs.ServiceName, error)
	PeeringsForService(ws memdb.WatchSet, serviceName string, entMeta acl.EnterpriseMeta) (uint64, []*pbpeering.Peering, error)
	AbandonCh() <-chan struct{}
}

// Apply provides a write-only interface for persisting Peering data.
type Apply interface {
	PeeringWrite(req *pbpeering.PeeringWriteRequest) error
	PeeringDelete(req *pbpeering.PeeringDeleteRequest) error
	PeeringTerminateByID(req *pbpeering.PeeringTerminateByIDRequest) error
	CatalogRegister(req *structs.RegisterRequest) error
}

// GenerateToken implements the PeeringService RPC method to generate a
// peering token which is the initial step in establishing a peering relationship
// with other Consul clusters.
func (s *Service) GenerateToken(
	ctx context.Context,
	req *pbpeering.GenerateTokenRequest,
) (*pbpeering.GenerateTokenResponse, error) {
	if err := s.Backend.EnterpriseCheckPartitions(req.Partition); err != nil {
		return nil, grpcstatus.Error(codes.InvalidArgument, err.Error())
	}
	// validate prior to forwarding to the leader, this saves a network hop
	if err := dns.ValidateLabel(req.PeerName); err != nil {
		return nil, fmt.Errorf("%s is not a valid peer name: %w", req.PeerName, err)
	}

	if err := structs.ValidateMetaTags(req.Meta); err != nil {
		return nil, fmt.Errorf("meta tags failed validation: %w", err)
	}

	// TODO(peering): add metrics
	// TODO(peering): add tracing

	resp := &pbpeering.GenerateTokenResponse{}
	handled, err := s.Backend.Forward(req, func(conn *grpc.ClientConn) error {
		var err error
		resp, err = pbpeering.NewPeeringServiceClient(conn).GenerateToken(ctx, req)
		return err
	})
	if handled || err != nil {
		return resp, err
	}

	ca, err := s.Backend.GetAgentCACertificates()
	if err != nil {
		return nil, err
	}

	serverAddrs, err := s.Backend.GetServerAddresses()
	if err != nil {
		return nil, err
	}

	writeReq := pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			Name: req.PeerName,
			// TODO(peering): Normalize from ACL token once this endpoint is guarded by ACLs.
			Partition: req.PartitionOrDefault(),
			Meta:      req.Meta,
		},
	}
	if err := s.Backend.Apply().PeeringWrite(&writeReq); err != nil {
		return nil, fmt.Errorf("failed to write peering: %w", err)
	}

	q := state.Query{
		Value:          strings.ToLower(req.PeerName),
		EnterpriseMeta: *structs.NodeEnterpriseMetaInPartition(req.Partition),
	}
	_, peering, err := s.Backend.Store().PeeringRead(nil, q)
	if err != nil {
		return nil, err
	}
	if peering == nil {
		return nil, fmt.Errorf("peering was deleted while token generation request was in flight")
	}

	tok := structs.PeeringToken{
		// Store the UUID so that we can do a global search when handling inbound streams.
		PeerID:          peering.ID,
		CA:              ca,
		ServerAddresses: serverAddrs,
		ServerName:      s.Backend.GetServerName(),
	}

	encoded, err := s.Backend.EncodeToken(&tok)
	if err != nil {
		return nil, err
	}
	resp.PeeringToken = string(encoded)
	return resp, err
}

// Initiate implements the PeeringService RPC method to finalize peering
// registration. Given a valid token output from a peer's GenerateToken endpoint,
// a peering is registered.
func (s *Service) Initiate(
	ctx context.Context,
	req *pbpeering.InitiateRequest,
) (*pbpeering.InitiateResponse, error) {
	// validate prior to forwarding to the leader, this saves a network hop
	if err := dns.ValidateLabel(req.PeerName); err != nil {
		return nil, fmt.Errorf("%s is not a valid peer name: %w", req.PeerName, err)
	}
	tok, err := s.Backend.DecodeToken([]byte(req.PeeringToken))
	if err != nil {
		return nil, err
	}
	if err := validatePeeringToken(tok); err != nil {
		return nil, err
	}

	if err := structs.ValidateMetaTags(req.Meta); err != nil {
		return nil, fmt.Errorf("meta tags failed validation: %w", err)
	}

	resp := &pbpeering.InitiateResponse{}
	handled, err := s.Backend.Forward(req, func(conn *grpc.ClientConn) error {
		var err error
		resp, err = pbpeering.NewPeeringServiceClient(conn).Initiate(ctx, req)
		return err
	})
	if handled || err != nil {
		return resp, err
	}

	defer metrics.MeasureSince([]string{"peering", "initiate"}, time.Now())

	// convert ServiceAddress values to strings
	serverAddrs := make([]string, len(tok.ServerAddresses))
	for i, addr := range tok.ServerAddresses {
		serverAddrs[i] = addr
	}

	// as soon as a peering is written with a list of ServerAddresses that is
	// non-empty, the leader routine will see the peering and attempt to establish
	// a connection with the remote peer.
	writeReq := &pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			Name:                req.PeerName,
			PeerCAPems:          tok.CA,
			PeerServerAddresses: serverAddrs,
			PeerServerName:      tok.ServerName,
			// uncomment once #1613 lands
			// PeerID: 			 tok.PeerID,
			Meta: req.Meta,
		},
	}
	if err = s.Backend.Apply().PeeringWrite(writeReq); err != nil {
		return nil, fmt.Errorf("failed to write peering: %w", err)
	}
	// resp.Status == 0
	return resp, nil
}

func (s *Service) PeeringRead(ctx context.Context, req *pbpeering.PeeringReadRequest) (*pbpeering.PeeringReadResponse, error) {
	if err := s.Backend.EnterpriseCheckPartitions(req.Partition); err != nil {
		return nil, grpcstatus.Error(codes.InvalidArgument, err.Error())
	}

	var resp *pbpeering.PeeringReadResponse
	handled, err := s.Backend.Forward(req, func(conn *grpc.ClientConn) error {
		var err error
		resp, err = pbpeering.NewPeeringServiceClient(conn).PeeringRead(ctx, req)
		return err
	})
	if handled || err != nil {
		return resp, err
	}

	defer metrics.MeasureSince([]string{"peering", "read"}, time.Now())
	// TODO(peering): ACL check request token

	// TODO(peering): handle blocking queries
	q := state.Query{
		Value:          strings.ToLower(req.Name),
		EnterpriseMeta: *structs.NodeEnterpriseMetaInPartition(req.Partition)}
	_, peering, err := s.Backend.Store().PeeringRead(nil, q)
	if err != nil {
		return nil, err
	}
	return &pbpeering.PeeringReadResponse{Peering: peering}, nil
}

func (s *Service) PeeringList(ctx context.Context, req *pbpeering.PeeringListRequest) (*pbpeering.PeeringListResponse, error) {
	if err := s.Backend.EnterpriseCheckPartitions(req.Partition); err != nil {
		return nil, grpcstatus.Error(codes.InvalidArgument, err.Error())
	}

	var resp *pbpeering.PeeringListResponse
	handled, err := s.Backend.Forward(req, func(conn *grpc.ClientConn) error {
		var err error
		resp, err = pbpeering.NewPeeringServiceClient(conn).PeeringList(ctx, req)
		return err
	})
	if handled || err != nil {
		return resp, err
	}

	defer metrics.MeasureSince([]string{"peering", "list"}, time.Now())
	// TODO(peering): ACL check request token

	// TODO(peering): handle blocking queries
	_, peerings, err := s.Backend.Store().PeeringList(nil, *structs.NodeEnterpriseMetaInPartition(req.Partition))
	if err != nil {
		return nil, err
	}
	return &pbpeering.PeeringListResponse{Peerings: peerings}, nil
}

// TODO(peering): As of writing, this method is only used in tests to set up Peerings in the state store.
// Consider removing if we can find another way to populate state store in peering_endpoint_test.go
func (s *Service) PeeringWrite(ctx context.Context, req *pbpeering.PeeringWriteRequest) (*pbpeering.PeeringWriteResponse, error) {
	if err := s.Backend.EnterpriseCheckPartitions(req.Peering.Partition); err != nil {
		return nil, grpcstatus.Error(codes.InvalidArgument, err.Error())
	}

	var resp *pbpeering.PeeringWriteResponse
	handled, err := s.Backend.Forward(req, func(conn *grpc.ClientConn) error {
		var err error
		resp, err = pbpeering.NewPeeringServiceClient(conn).PeeringWrite(ctx, req)
		return err
	})
	if handled || err != nil {
		return resp, err
	}

	defer metrics.MeasureSince([]string{"peering", "write"}, time.Now())
	// TODO(peering): ACL check request token

	// TODO(peering): handle blocking queries
	err = s.Backend.Apply().PeeringWrite(req)
	if err != nil {
		return nil, err
	}
	return &pbpeering.PeeringWriteResponse{}, nil
}

func (s *Service) PeeringDelete(ctx context.Context, req *pbpeering.PeeringDeleteRequest) (*pbpeering.PeeringDeleteResponse, error) {
	if err := s.Backend.EnterpriseCheckPartitions(req.Partition); err != nil {
		return nil, grpcstatus.Error(codes.InvalidArgument, err.Error())
	}

	var resp *pbpeering.PeeringDeleteResponse
	handled, err := s.Backend.Forward(req, func(conn *grpc.ClientConn) error {
		var err error
		resp, err = pbpeering.NewPeeringServiceClient(conn).PeeringDelete(ctx, req)
		return err
	})
	if handled || err != nil {
		return resp, err
	}

	defer metrics.MeasureSince([]string{"peering", "delete"}, time.Now())
	// TODO(peering): ACL check request token

	// TODO(peering): handle blocking queries
	err = s.Backend.Apply().PeeringDelete(req)
	if err != nil {
		return nil, err
	}
	return &pbpeering.PeeringDeleteResponse{}, nil
}

func (s *Service) TrustBundleListByService(ctx context.Context, req *pbpeering.TrustBundleListByServiceRequest) (*pbpeering.TrustBundleListByServiceResponse, error) {
	if err := s.Backend.EnterpriseCheckPartitions(req.Partition); err != nil {
		return nil, grpcstatus.Error(codes.InvalidArgument, err.Error())
	}

	var resp *pbpeering.TrustBundleListByServiceResponse
	handled, err := s.Backend.Forward(req, func(conn *grpc.ClientConn) error {
		var err error
		resp, err = pbpeering.NewPeeringServiceClient(conn).TrustBundleListByService(ctx, req)
		return err
	})
	if handled || err != nil {
		return resp, err
	}

	defer metrics.MeasureSince([]string{"peering", "trust_bundle_list_by_service"}, time.Now())
	// TODO(peering): ACL check request token

	// TODO(peering): handle blocking queries

	entMeta := *structs.NodeEnterpriseMetaInPartition(req.Partition)
	// TODO(peering): we're throwing away the index here that would tell us how to execute a blocking query
	_, peers, err := s.Backend.Store().PeeringsForService(nil, req.ServiceName, entMeta)
	if err != nil {
		return nil, fmt.Errorf("failed to get peers for service %s: %v", req.ServiceName, err)
	}

	trustBundles := []*pbpeering.PeeringTrustBundle{}
	for _, peer := range peers {
		q := state.Query{
			Value:          strings.ToLower(peer.Name),
			EnterpriseMeta: *structs.NodeEnterpriseMetaInPartition(req.Partition),
		}
		_, trustBundle, err := s.Backend.Store().PeeringTrustBundleRead(nil, q)
		if err != nil {
			return nil, fmt.Errorf("failed to read trust bundle for peer %s: %v", peer.Name, err)
		}

		if trustBundle != nil {
			trustBundles = append(trustBundles, trustBundle)
		}
	}
	return &pbpeering.TrustBundleListByServiceResponse{Bundles: trustBundles}, nil
}

type BidirectionalStream interface {
	Send(*pbpeering.ReplicationMessage) error
	Recv() (*pbpeering.ReplicationMessage, error)
	Context() context.Context
}

// StreamResources handles incoming streaming connections.
func (s *Service) StreamResources(stream pbpeering.PeeringService_StreamResourcesServer) error {
	// Initial message on a new stream must be a new subscription request.
	first, err := stream.Recv()
	if err != nil {
		s.logger.Error("failed to establish stream", "error", err)
		return err
	}

	// TODO(peering) Make request contain a list of resources, so that roots and services can be
	//  			 subscribed to with a single request. See:
	//               https://github.com/envoyproxy/data-plane-api/blob/main/envoy/service/discovery/v3/discovery.proto#L46
	req := first.GetRequest()
	if req == nil {
		return grpcstatus.Error(codes.InvalidArgument, "first message when initiating a peering must be a subscription request")
	}
	s.logger.Trace("received initial replication request from peer")
	logTraceRecv(s.logger, req)

	if req.PeerID == "" {
		return grpcstatus.Error(codes.InvalidArgument, "initial subscription request must specify a PeerID")
	}
	if req.Nonce != "" {
		return grpcstatus.Error(codes.InvalidArgument, "initial subscription request must not contain a nonce")
	}
	if req.ResourceURL != pbpeering.TypeURLService {
		return grpcstatus.Error(codes.InvalidArgument, fmt.Sprintf("subscription request to unknown resource URL: %s", req.ResourceURL))
	}

	_, p, err := s.Backend.Store().PeeringReadByID(nil, req.PeerID)
	if err != nil {
		s.logger.Error("failed to look up peer", "peer_id", req.PeerID, "error", err)
		return grpcstatus.Error(codes.Internal, "failed to find PeerID: "+req.PeerID)
	}
	if p == nil {
		return grpcstatus.Error(codes.InvalidArgument, "initial subscription for unknown PeerID: "+req.PeerID)
	}

	// TODO(peering): If the peering is marked as deleted, send a Terminated message and return
	// TODO(peering): Store subscription request so that an event publisher can separately handle pushing messages for it
	s.logger.Info("accepted initial replication request from peer", "peer_id", req.PeerID)

	// For server peers both of these ID values are the same, because we generated a token with a local ID,
	// and the client peer dials using that same ID.
	return s.HandleStream(HandleStreamRequest{
		LocalID:   req.PeerID,
		RemoteID:  req.PeerID,
		PeerName:  p.Name,
		Partition: p.Partition,
		Stream:    stream,
	})
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

// The localID provided is the locally-generated identifier for the peering.
// The remoteID is an identifier that the remote peer recognizes for the peering.
func (s *Service) HandleStream(req HandleStreamRequest) error {
	logger := s.logger.Named("stream").With("peer_id", req.LocalID)
	logger.Trace("handling stream for peer")

	status, err := s.streams.connected(req.LocalID)
	if err != nil {
		return fmt.Errorf("failed to register stream: %v", err)
	}

	// TODO(peering) Also need to clear subscriptions associated with the peer
	defer s.streams.disconnected(req.LocalID)

	mgr := newSubscriptionManager(req.Stream.Context(), logger, s.Backend)
	subCh := mgr.subscribe(req.Stream.Context(), req.LocalID)

	sub := &pbpeering.ReplicationMessage{
		Payload: &pbpeering.ReplicationMessage_Request_{
			Request: &pbpeering.ReplicationMessage_Request{
				ResourceURL: pbpeering.TypeURLService,
				PeerID:      req.RemoteID,
			},
		},
	}
	logTraceSend(logger, sub)

	if err := req.Stream.Send(sub); err != nil {
		if err == io.EOF {
			logger.Info("stream ended by peer")
			status.trackReceiveError(err.Error())
			return nil
		}
		// TODO(peering) Test error handling in calls to Send/Recv
		status.trackSendError(err.Error())
		return fmt.Errorf("failed to send to stream: %v", err)
	}

	// TODO(peering): Should this be buffered?
	recvChan := make(chan *pbpeering.ReplicationMessage)
	go func() {
		defer close(recvChan)
		for {
			msg, err := req.Stream.Recv()
			if err == io.EOF {
				logger.Info("stream ended by peer")
				status.trackReceiveError(err.Error())
				return
			}
			if e, ok := grpcstatus.FromError(err); ok {
				// Cancelling the stream is not an error, that means we or our peer intended to terminate the peering.
				if e.Code() == codes.Canceled {
					return
				}
			}
			if err != nil {
				logger.Error("failed to receive from stream", "error", err)
				status.trackReceiveError(err.Error())
				return
			}

			logTraceRecv(logger, msg)
			recvChan <- msg
		}
	}()

	for {
		select {
		// When the doneCh is closed that means that the peering was deleted locally.
		case <-status.doneCh:
			logger.Info("ending stream")

			term := &pbpeering.ReplicationMessage{
				Payload: &pbpeering.ReplicationMessage_Terminated_{
					Terminated: &pbpeering.ReplicationMessage_Terminated{},
				},
			}
			logTraceSend(logger, term)

			if err := req.Stream.Send(term); err != nil {
				status.trackSendError(err.Error())
				return fmt.Errorf("failed to send to stream: %v", err)
			}

			logger.Trace("deleting stream status")
			s.streams.deleteStatus(req.LocalID)

			return nil

		case msg, open := <-recvChan:
			if !open {
				// No longer receiving data on the stream.
				return nil
			}

			if req := msg.GetRequest(); req != nil {
				switch {
				case req.Nonce == "":
					// TODO(peering): This can happen on a client peer since they don't try to receive subscriptions before entering HandleStream.
					//                Should change that behavior or only allow it that one time.

				case req.Error != nil && (req.Error.Code != int32(code.Code_OK) || req.Error.Message != ""):
					logger.Warn("client peer was unable to apply resource", "code", req.Error.Code, "error", req.Error.Message)
					status.trackNack(fmt.Sprintf("client peer was unable to apply resource: %s", req.Error.Message))

				default:
					status.trackAck()
				}

				continue
			}

			if resp := msg.GetResponse(); resp != nil {
				// TODO(peering): Ensure there's a nonce
				reply, err := s.processResponse(req.PeerName, req.Partition, resp)
				if err != nil {
					logger.Error("failed to persist resource", "resourceURL", resp.ResourceURL, "resourceID", resp.ResourceID)
					status.trackReceiveError(err.Error())
				} else {
					status.trackReceiveSuccess()
				}

				logTraceSend(logger, reply)
				if err := req.Stream.Send(reply); err != nil {
					status.trackSendError(err.Error())
					return fmt.Errorf("failed to send to stream: %v", err)
				}

				continue
			}

			if term := msg.GetTerminated(); term != nil {
				logger.Info("received peering termination message, cleaning up imported resources")

				// Once marked as terminated, a separate deferred deletion routine will clean up imported resources.
				if err := s.Backend.Apply().PeeringTerminateByID(&pbpeering.PeeringTerminateByIDRequest{ID: req.LocalID}); err != nil {
					return err
				}
				return nil
			}

		case update := <-subCh:
			switch {
			case strings.HasPrefix(update.CorrelationID, subExportedService):
				if err := pushServiceResponse(logger, req.Stream, status, update); err != nil {
					return fmt.Errorf("failed to push data for %q: %w", update.CorrelationID, err)
				}

			default:
				logger.Warn("unrecognized update type from subscription manager: " + update.CorrelationID)
				continue
			}
		}
	}
}

// pushService response handles sending exported service instance updates to the peer cluster.
// Each cache.UpdateEvent will contain all instances for a service name.
// If there are no instances in the event, we consider that to be a de-registration.
func pushServiceResponse(logger hclog.Logger, stream BidirectionalStream, status *lockableStreamStatus, update cache.UpdateEvent) error {
	csn, ok := update.Result.(*pbservice.IndexedCheckServiceNodes)
	if !ok {
		logger.Error(fmt.Sprintf("invalid type for response: %T, expected *pbservice.IndexedCheckServiceNodes", update.Result))

		// Skip this update to avoid locking up peering due to a bad service update.
		return nil
	}
	serviceName := strings.TrimPrefix(update.CorrelationID, subExportedService)

	// If no nodes are present then it's due to one of:
	// 1. The service is newly registered or exported and yielded a transient empty update.
	// 2. All instances of the service were de-registered.
	// 3. The service was un-exported.
	//
	// We don't distinguish when these three things occurred, but it's safe to send a DELETE Op in all cases, so we do that.
	// Case #1 is a no-op for the importing peer.
	if len(csn.Nodes) == 0 {
		resp := &pbpeering.ReplicationMessage{
			Payload: &pbpeering.ReplicationMessage_Response_{
				Response: &pbpeering.ReplicationMessage_Response{
					ResourceURL: pbpeering.TypeURLService,
					// TODO(peering): Nonce management
					Nonce:      "",
					ResourceID: serviceName,
					Operation:  pbpeering.ReplicationMessage_Response_DELETE,
				},
			},
		}
		logTraceSend(logger, resp)
		if err := stream.Send(resp); err != nil {
			status.trackSendError(err.Error())
			return fmt.Errorf("failed to send to stream: %v", err)
		}
		return nil
	}

	// If there are nodes in the response, we push them as an UPSERT operation.
	any, err := ptypes.MarshalAny(csn)
	if err != nil {
		// Log the error and skip this response to avoid locking up peering due to a bad update event.
		logger.Error("failed to marshal service endpoints", "error", err)
		return nil
	}
	resp := &pbpeering.ReplicationMessage{
		Payload: &pbpeering.ReplicationMessage_Response_{
			Response: &pbpeering.ReplicationMessage_Response{
				ResourceURL: pbpeering.TypeURLService,
				// TODO(peering): Nonce management
				Nonce:      "",
				ResourceID: serviceName,
				Operation:  pbpeering.ReplicationMessage_Response_UPSERT,
				Resource:   any,
			},
		},
	}
	logTraceSend(logger, resp)
	if err := stream.Send(resp); err != nil {
		status.trackSendError(err.Error())
		return fmt.Errorf("failed to send to stream: %v", err)
	}
	return nil
}

func (s *Service) StreamStatus(peer string) (resp StreamStatus, found bool) {
	return s.streams.streamStatus(peer)
}

// ConnectedStreams returns a map of connected stream IDs to the corresponding channel for tearing them down.
func (s *Service) ConnectedStreams() map[string]chan struct{} {
	return s.streams.connectedStreams()
}

func makeReply(resourceURL, nonce string, errCode code.Code, errMsg string) *pbpeering.ReplicationMessage {
	var rpcErr *pbstatus.Status
	if errCode != code.Code_OK || errMsg != "" {
		rpcErr = &pbstatus.Status{
			Code:    int32(errCode),
			Message: errMsg,
		}
	}

	msg := &pbpeering.ReplicationMessage{
		Payload: &pbpeering.ReplicationMessage_Request_{
			Request: &pbpeering.ReplicationMessage_Request{
				ResourceURL: resourceURL,
				Nonce:       nonce,
				Error:       rpcErr,
			},
		},
	}
	return msg
}

func (s *Service) processResponse(peerName string, partition string, resp *pbpeering.ReplicationMessage_Response) (*pbpeering.ReplicationMessage, error) {
	var (
		err     error
		errCode code.Code
		errMsg  string
	)

	if resp.ResourceURL != pbpeering.TypeURLService {
		errCode = code.Code_INVALID_ARGUMENT
		err = fmt.Errorf("received response for unknown resource type %q", resp.ResourceURL)
		return makeReply(resp.ResourceURL, resp.Nonce, errCode, err.Error()), err
	}

	switch resp.Operation {
	case pbpeering.ReplicationMessage_Response_UPSERT:
		if resp.Resource == nil {
			break
		}
		err = s.handleUpsert(peerName, partition, resp.ResourceURL, resp.ResourceID, resp.Resource)
		if err != nil {
			errCode = code.Code_INTERNAL
			errMsg = err.Error()
		}

	case pbpeering.ReplicationMessage_Response_DELETE:
		err = handleDelete(resp.ResourceURL, resp.ResourceID)
		if err != nil {
			errCode = code.Code_INTERNAL
			errMsg = err.Error()
		}

	default:
		errCode = code.Code_INVALID_ARGUMENT

		op := pbpeering.ReplicationMessage_Response_Operation_name[int32(resp.Operation)]
		if op == "" {
			op = strconv.FormatInt(int64(resp.Operation), 10)
		}
		errMsg = fmt.Sprintf("unsupported operation: %q", op)

		err = errors.New(errMsg)
	}

	return makeReply(resp.ResourceURL, resp.Nonce, errCode, errMsg), err
}

func (s *Service) handleUpsert(peerName string, partition string, resourceURL string, resourceID string, resource *anypb.Any) error {
	csn := &pbservice.IndexedCheckServiceNodes{}
	err := ptypes.UnmarshalAny(resource, csn)
	if err != nil {
		return fmt.Errorf("failed to unmarshal resource, ResourceURL: %q, ResourceID: %q, err: %w", resourceURL, resourceID, err)
	}
	if csn == nil || len(csn.Nodes) == 0 {
		return nil
	}

	type checkTuple struct {
		checkID   types.CheckID
		serviceID string
		nodeID    types.NodeID

		acl.EnterpriseMeta
	}

	var (
		nodes    = make(map[types.NodeID]*structs.Node)
		services = make(map[types.NodeID][]*structs.NodeService)
		checks   = make(map[types.NodeID]map[checkTuple]*structs.HealthCheck)
	)

	for _, pbinstance := range csn.Nodes {
		instance, err := pbservice.CheckServiceNodeToStructs(pbinstance)
		if err != nil {
			return fmt.Errorf("failed to convert instance, ResourceURL: %q, ResourceID: %q, err: %w", resourceURL, resourceID, err)
		}

		nodes[instance.Node.ID] = instance.Node
		services[instance.Node.ID] = append(services[instance.Node.ID], instance.Service)

		if _, ok := checks[instance.Node.ID]; !ok {
			checks[instance.Node.ID] = make(map[checkTuple]*structs.HealthCheck)
		}
		for _, c := range instance.Checks {
			tuple := checkTuple{
				checkID:        c.CheckID,
				serviceID:      c.ServiceID,
				nodeID:         instance.Node.ID,
				EnterpriseMeta: c.EnterpriseMeta,
			}
			checks[instance.Node.ID][tuple] = c
		}
	}

	for nodeID, node := range nodes {
		// For all nodes, services, and checks we override the peer name and partition to be
		// the local partition and local name for the peer.
		node.PeerName, node.Partition = peerName, partition

		// First register the node
		req := node.ToRegisterRequest()
		if err := s.Backend.Apply().CatalogRegister(&req); err != nil {
			return fmt.Errorf("failed to register, ResourceURL: %q, ResourceID: %q, err: %w", resourceURL, resourceID, err)
		}

		// Then register all services on that node
		for _, svc := range services[nodeID] {
			svc.PeerName = peerName
			svc.OverridePartition(partition)

			req.Service = svc
			if err := s.Backend.Apply().CatalogRegister(&req); err != nil {
				return fmt.Errorf("failed to register, ResourceURL: %q, ResourceID: %q, err: %w", resourceURL, resourceID, err)
			}
		}
		req.Service = nil

		// Then register all checks on that node
		var chks structs.HealthChecks
		for _, c := range checks[nodeID] {
			c.PeerName = peerName
			c.OverridePartition(partition)

			chks = append(chks, c)
		}

		req.Checks = chks
		if err := s.Backend.Apply().CatalogRegister(&req); err != nil {
			return fmt.Errorf("failed to register, ResourceURL: %q, ResourceID: %q, err: %w", resourceURL, resourceID, err)
		}
	}
	return nil
}

func handleDelete(resourceURL string, resourceID string) error {
	// TODO(peering): implement
	return nil
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
