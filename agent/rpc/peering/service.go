package peering

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/armon/go-metrics"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/dns"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbpeering"
)

var (
	errPeeringTokenInvalidCA            = errors.New("peering token CA value is invalid")
	errPeeringTokenEmptyServerAddresses = errors.New("peering token server addresses value is empty")
	errPeeringTokenEmptyServerName      = errors.New("peering token server name value is empty")
	errPeeringTokenEmptyPeerID          = errors.New("peering token peer ID value is empty")
)

// errPeeringInvalidServerAddress is returned when an establish request contains
// an invalid server address.
type errPeeringInvalidServerAddress struct {
	addr string
}

// Error implements the error interface
func (e *errPeeringInvalidServerAddress) Error() string {
	return fmt.Sprintf("%s is not a valid peering server address", e.addr)
}

type Config struct {
	Datacenter     string
	ConnectEnabled bool
}

// Service implements pbpeering.PeeringService to provide RPC operations for
// managing peering relationships.
type Service struct {
	Backend Backend
	logger  hclog.Logger
	config  Config
	streams *streamTracker
}

func NewService(logger hclog.Logger, cfg Config, backend Backend) *Service {
	return &Service{
		Backend: backend,
		logger:  logger,
		config:  cfg,
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

	EnterpriseCheckNamespaces(namespace string) error

	Subscribe(req *stream.SubscribeRequest) (*stream.Subscription, error)

	// IsLeader indicates whether the consul server is in a leader state or not.
	IsLeader() bool

	Store() Store
	Apply() Apply
	LeaderAddress() LeaderAddress
}

// LeaderAddress provides a way for the consul server to update the peering service about
// the server's leadership status.
// Server addresses should look like: ip:port
type LeaderAddress interface {
	// Set is called on a raft.LeaderObservation in a go routine in the consul server;
	// see trackLeaderChanges()
	Set(leaderAddr string)

	// Get provides the best hint for the current address of the leader.
	// There is no guarantee that this is the actual address of the leader.
	Get() string
}

// Store provides a read-only interface for querying Peering data.
type Store interface {
	PeeringRead(ws memdb.WatchSet, q state.Query) (uint64, *pbpeering.Peering, error)
	PeeringReadByID(ws memdb.WatchSet, id string) (uint64, *pbpeering.Peering, error)
	PeeringList(ws memdb.WatchSet, entMeta acl.EnterpriseMeta) (uint64, []*pbpeering.Peering, error)
	PeeringTrustBundleRead(ws memdb.WatchSet, q state.Query) (uint64, *pbpeering.PeeringTrustBundle, error)
	PeeringTrustBundleList(ws memdb.WatchSet, entMeta acl.EnterpriseMeta) (uint64, []*pbpeering.PeeringTrustBundle, error)
	ExportedServicesForPeer(ws memdb.WatchSet, peerID string) (uint64, *structs.ExportedServiceList, error)
	ServiceDump(ws memdb.WatchSet, kind structs.ServiceKind, useKind bool, entMeta *acl.EnterpriseMeta, peerName string) (uint64, structs.CheckServiceNodes, error)
	CheckServiceNodes(ws memdb.WatchSet, serviceName string, entMeta *acl.EnterpriseMeta, peerName string) (uint64, structs.CheckServiceNodes, error)
	NodeServices(ws memdb.WatchSet, nodeNameOrID string, entMeta *acl.EnterpriseMeta, peerName string) (uint64, *structs.NodeServices, error)
	CAConfig(ws memdb.WatchSet) (uint64, *structs.CAConfiguration, error)
	TrustBundleListByService(ws memdb.WatchSet, service string, entMeta acl.EnterpriseMeta) (uint64, []*pbpeering.PeeringTrustBundle, error)
	AbandonCh() <-chan struct{}
}

// Apply provides a write-only interface for persisting Peering data.
type Apply interface {
	PeeringWrite(req *pbpeering.PeeringWriteRequest) error
	PeeringTerminateByID(req *pbpeering.PeeringTerminateByIDRequest) error
	PeeringTrustBundleWrite(req *pbpeering.PeeringTrustBundleWriteRequest) error
	CatalogRegister(req *structs.RegisterRequest) error
	CatalogDeregister(req *structs.DeregisterRequest) error
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

// Establish implements the PeeringService RPC method to finalize peering
// registration. Given a valid token output from a peer's GenerateToken endpoint,
// a peering is registered.
func (s *Service) Establish(
	ctx context.Context,
	req *pbpeering.EstablishRequest,
) (*pbpeering.EstablishResponse, error) {
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

	resp := &pbpeering.EstablishResponse{}
	handled, err := s.Backend.Forward(req, func(conn *grpc.ClientConn) error {
		var err error
		resp, err = pbpeering.NewPeeringServiceClient(conn).Establish(ctx, req)
		return err
	})
	if handled || err != nil {
		return resp, err
	}

	defer metrics.MeasureSince([]string{"peering", "establish"}, time.Now())

	// convert ServiceAddress values to strings
	serverAddrs := make([]string, len(tok.ServerAddresses))
	for i, addr := range tok.ServerAddresses {
		serverAddrs[i] = addr
	}

	// as soon as a peering is written with a list of ServerAddresses that is
	// non-empty, the leader routine will see the peering and attempt to
	// establish a connection with the remote peer.
	//
	// This peer now has a record of both the LocalPeerID(ID) and
	// RemotePeerID(PeerID) but at this point the other peer does not.
	writeReq := &pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			Name:                req.PeerName,
			PeerCAPems:          tok.CA,
			PeerServerAddresses: serverAddrs,
			PeerServerName:      tok.ServerName,
			PeerID:              tok.PeerID,
			Meta:                req.Meta,
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

	q := state.Query{
		Value:          strings.ToLower(req.Name),
		EnterpriseMeta: *structs.NodeEnterpriseMetaInPartition(req.Partition),
	}
	_, existing, err := s.Backend.Store().PeeringRead(nil, q)
	if err != nil {
		return nil, err
	}

	if existing == nil || !existing.IsActive() {
		// Return early when the Peering doesn't exist or is already marked for deletion.
		// We don't return nil because the pb will fail to marshal.
		return &pbpeering.PeeringDeleteResponse{}, nil
	}
	// We are using a write request due to needing to perform a deferred deletion.
	// The peering gets marked for deletion by setting the DeletedAt field,
	// and a leader routine will handle deleting the peering.
	writeReq := &pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			// We only need to include the name and partition for the peering to be identified.
			// All other data associated with the peering can be discarded because once marked
			// for deletion the peering is effectively gone.
			Name:      req.Name,
			Partition: req.Partition,
			DeletedAt: structs.TimeToProto(time.Now().UTC()),
		},
	}
	err = s.Backend.Apply().PeeringWrite(writeReq)
	if err != nil {
		return nil, err
	}
	return &pbpeering.PeeringDeleteResponse{}, nil
}

func (s *Service) TrustBundleRead(ctx context.Context, req *pbpeering.TrustBundleReadRequest) (*pbpeering.TrustBundleReadResponse, error) {
	if err := s.Backend.EnterpriseCheckPartitions(req.Partition); err != nil {
		return nil, grpcstatus.Error(codes.InvalidArgument, err.Error())
	}

	var resp *pbpeering.TrustBundleReadResponse
	handled, err := s.Backend.Forward(req, func(conn *grpc.ClientConn) error {
		var err error
		resp, err = pbpeering.NewPeeringServiceClient(conn).TrustBundleRead(ctx, req)
		return err
	})
	if handled || err != nil {
		return resp, err
	}

	defer metrics.MeasureSince([]string{"peering", "trust_bundle_read"}, time.Now())
	// TODO(peering): ACL check request token

	// TODO(peering): handle blocking queries

	idx, trustBundle, err := s.Backend.Store().PeeringTrustBundleRead(nil, state.Query{
		Value:          req.Name,
		EnterpriseMeta: *structs.NodeEnterpriseMetaInPartition(req.Partition),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to read trust bundle for peer %s: %w", req.Name, err)
	}

	return &pbpeering.TrustBundleReadResponse{
		Index:  idx,
		Bundle: trustBundle,
	}, nil
}

// TODO(peering): rename rpc & request/response to drop the "service" part
func (s *Service) TrustBundleListByService(ctx context.Context, req *pbpeering.TrustBundleListByServiceRequest) (*pbpeering.TrustBundleListByServiceResponse, error) {
	if err := s.Backend.EnterpriseCheckPartitions(req.Partition); err != nil {
		return nil, grpcstatus.Error(codes.InvalidArgument, err.Error())
	}
	if err := s.Backend.EnterpriseCheckNamespaces(req.Namespace); err != nil {
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
	// TODO(peering): ACL check request token for service:write on the service name

	// TODO(peering): handle blocking queries

	entMeta := acl.NewEnterpriseMetaWithPartition(req.Partition, req.Namespace)

	var (
		idx     uint64
		bundles []*pbpeering.PeeringTrustBundle
	)

	switch {
	case req.ServiceName != "":
		idx, bundles, err = s.Backend.Store().TrustBundleListByService(nil, req.ServiceName, entMeta)
	case req.Kind == string(structs.ServiceKindMeshGateway):
		idx, bundles, err = s.Backend.Store().PeeringTrustBundleList(nil, entMeta)
	case req.Kind != "":
		return nil, grpcstatus.Error(codes.InvalidArgument, "kind must be mesh-gateway if set")
	default:
		return nil, grpcstatus.Error(codes.InvalidArgument, "one of service or kind is required")
	}

	if err != nil {
		return nil, err
	}
	return &pbpeering.TrustBundleListByServiceResponse{Index: idx, Bundles: bundles}, nil
}

type BidirectionalStream interface {
	Send(*pbpeering.ReplicationMessage) error
	Recv() (*pbpeering.ReplicationMessage, error)
	Context() context.Context
}

// StreamResources handles incoming streaming connections.
func (s *Service) StreamResources(stream pbpeering.PeeringService_StreamResourcesServer) error {
	if !s.Backend.IsLeader() {
		// we are not the leader so we will hang up on the dialer

		s.logger.Error("cannot establish a peering stream on a follower node")

		st, err := grpcstatus.New(codes.FailedPrecondition,
			"cannot establish a peering stream on a follower node").WithDetails(
			&pbpeering.LeaderAddress{Address: s.Backend.LeaderAddress().Get()})
		if err != nil {
			s.logger.Error(fmt.Sprintf("failed to marshal the leader address in response; err: %v", err))
			return grpcstatus.Error(codes.FailedPrecondition, "cannot establish a peering stream on a follower node")
		} else {
			return st.Err()
		}
	}

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
	if !pbpeering.KnownTypeURL(req.ResourceURL) {
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
	s.logger.Info("accepted initial replication request from peer", "peer_id", p.ID)

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

	s.logger.Error("error handling stream", "peer_name", p.Name, "peer_id", req.PeerID, "error", err)
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
func (s *Service) DrainStream(req HandleStreamRequest) {
	for {
		// Ensure that we read until an error, or the peer has nothing more to send.
		if _, err := req.Stream.Recv(); err != nil {
			if err != io.EOF {
				s.logger.Warn("failed to tear down stream gracefully: peer may not have received termination message",
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
func (s *Service) HandleStream(req HandleStreamRequest) error {
	logger := s.logger.Named("stream").With("peer_name", req.PeerName, "peer_id", req.LocalID)
	logger.Trace("handling stream for peer")

	status, err := s.streams.connected(req.LocalID)
	if err != nil {
		return fmt.Errorf("failed to register stream: %v", err)
	}

	// TODO(peering) Also need to clear subscriptions associated with the peer
	defer s.streams.disconnected(req.LocalID)

	var trustDomain string
	if s.config.ConnectEnabled {
		// Read the TrustDomain up front - we do not allow users to change the ClusterID
		// so reading it once at the beginning of the stream is sufficient.
		trustDomain, err = getTrustDomain(s.Backend.Store(), logger)
		if err != nil {
			return err
		}
	}

	mgr := newSubscriptionManager(
		req.Stream.Context(),
		logger,
		s.config,
		trustDomain,
		s.Backend,
	)
	subCh := mgr.subscribe(req.Stream.Context(), req.LocalID, req.PeerName, req.Partition)

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
			if err == nil {
				logTraceRecv(logger, msg)
				recvChan <- msg
				continue
			}

			if err == io.EOF {
				logger.Info("stream ended by peer")
				status.trackReceiveError(err.Error())
				return
			}
			logger.Error("failed to receive from stream", "error", err)
			status.trackReceiveError(err.Error())
			return
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
				logger.Trace("no longer receiving data on the stream")
				return nil
			}

			if !s.Backend.IsLeader() {
				// we are not the leader anymore so we will hang up on the dialer
				logger.Error("node is not a leader anymore; cannot continue streaming")

				st, err := grpcstatus.New(codes.FailedPrecondition,
					"node is not a leader anymore; cannot continue streaming").WithDetails(
					&pbpeering.LeaderAddress{Address: s.Backend.LeaderAddress().Get()})
				if err != nil {
					s.logger.Error(fmt.Sprintf("failed to marshal the leader address in response; err: %v", err))
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
				logger.Info("peering was deleted by our peer: marking peering as terminated and cleaning up imported resources")

				// Once marked as terminated, a separate deferred deletion routine will clean up imported resources.
				if err := s.Backend.Apply().PeeringTerminateByID(&pbpeering.PeeringTerminateByIDRequest{ID: req.LocalID}); err != nil {
					logger.Error("failed to mark peering as terminated: %w", err)
				}
				return nil
			}

		case update := <-subCh:
			var resp *pbpeering.ReplicationMessage
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
				status.trackSendError(err.Error())
				return fmt.Errorf("failed to push data for %q: %w", update.CorrelationID, err)
			}
		}
	}
}

func getTrustDomain(store Store, logger hclog.Logger) (string, error) {
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

func (s *Service) StreamStatus(peer string) (resp StreamStatus, found bool) {
	return s.streams.streamStatus(peer)
}

// ConnectedStreams returns a map of connected stream IDs to the corresponding channel for tearing them down.
func (s *Service) ConnectedStreams() map[string]chan struct{} {
	return s.streams.connectedStreams()
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
