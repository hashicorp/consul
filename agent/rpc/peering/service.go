package peering

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/acl/resolver"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/dns"
	external "github.com/hashicorp/consul/agent/grpc-external"
	"github.com/hashicorp/consul/agent/grpc-external/services/peerstream"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
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

// For private/internal gRPC handlers, protoc-gen-rpc-glue generates the
// requisite methods to satisfy the structs.RPCInfo interface using fields
// from the pbcommon package. This service is public, so we can't use those
// fields in our proto definition. Instead, we construct our RPCInfo manually.
var writeRequest struct {
	structs.WriteRequest
	structs.DCSpecificRequest
}

var readRequest struct {
	structs.QueryOptions
	structs.DCSpecificRequest
}

// Server implements pbpeering.PeeringService to provide RPC operations for
// managing peering relationships.
type Server struct {
	Config
}

type Config struct {
	Backend        Backend
	Tracker        *peerstream.Tracker
	Logger         hclog.Logger
	ForwardRPC     func(structs.RPCInfo, func(*grpc.ClientConn) error) (bool, error)
	Datacenter     string
	ConnectEnabled bool
	PeeringEnabled bool
}

func NewServer(cfg Config) *Server {
	requireNotNil(cfg.Backend, "Backend")
	requireNotNil(cfg.Tracker, "Tracker")
	requireNotNil(cfg.Logger, "Logger")
	requireNotNil(cfg.ForwardRPC, "ForwardRPC")
	if cfg.Datacenter == "" {
		panic("Datacenter is required")
	}
	return &Server{
		Config: cfg,
	}
}

func requireNotNil(v interface{}, name string) {
	if v == nil {
		panic(name + " is required")
	}
}

var _ pbpeering.PeeringServiceServer = (*Server)(nil)

func (s *Server) Register(grpcServer *grpc.Server) {
	pbpeering.RegisterPeeringServiceServer(grpcServer, s)
}

// Backend defines the core integrations the Peering endpoint depends on. A
// functional implementation will integrate with various subcomponents of Consul
// such as the State store for reading and writing data, the CA machinery for
// providing access to CA data and the RPC system for forwarding requests to
// other servers.
type Backend interface {
	// ResolveTokenAndDefaultMeta returns an acl.Authorizer which authorizes
	// actions based on the permissions granted to the token.
	// If either entMeta or authzContext are non-nil they will be populated with the
	// partition and namespace from the token.
	ResolveTokenAndDefaultMeta(token string, entMeta *acl.EnterpriseMeta, authzCtx *acl.AuthorizerContext) (resolver.Result, error)

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

	// SetLeaderAddress is called on a raft.LeaderObservation in a go routine
	// in the consul server; see trackLeaderChanges()
	SetLeaderAddress(string)

	// GetLeaderAddress provides the best hint for the current address of the
	// leader. There is no guarantee that this is the actual address of the
	// leader.
	GetLeaderAddress() string

	CheckPeeringUUID(id string) (bool, error)
	PeeringWrite(req *pbpeering.PeeringWriteRequest) error

	Store() Store
}

// Store provides a read-only interface for querying Peering data.
type Store interface {
	PeeringRead(ws memdb.WatchSet, q state.Query) (uint64, *pbpeering.Peering, error)
	PeeringReadByID(ws memdb.WatchSet, id string) (uint64, *pbpeering.Peering, error)
	PeeringList(ws memdb.WatchSet, entMeta acl.EnterpriseMeta) (uint64, []*pbpeering.Peering, error)
	PeeringTrustBundleRead(ws memdb.WatchSet, q state.Query) (uint64, *pbpeering.PeeringTrustBundle, error)
	PeeringTrustBundleList(ws memdb.WatchSet, entMeta acl.EnterpriseMeta) (uint64, []*pbpeering.PeeringTrustBundle, error)
	TrustBundleListByService(ws memdb.WatchSet, service, dc string, entMeta acl.EnterpriseMeta) (uint64, []*pbpeering.PeeringTrustBundle, error)
}

var peeringNotEnabledErr = grpcstatus.Error(codes.FailedPrecondition, "peering must be enabled to use this endpoint")

// GenerateToken implements the PeeringService RPC method to generate a
// peering token which is the initial step in establishing a peering relationship
// with other Consul clusters.
func (s *Server) GenerateToken(
	ctx context.Context,
	req *pbpeering.GenerateTokenRequest,
) (*pbpeering.GenerateTokenResponse, error) {
	if !s.Config.PeeringEnabled {
		return nil, peeringNotEnabledErr
	}

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

	defer metrics.MeasureSince([]string{"peering", "generate_token"}, time.Now())

	resp := &pbpeering.GenerateTokenResponse{}
	handled, err := s.ForwardRPC(&writeRequest, func(conn *grpc.ClientConn) error {
		ctx := external.ForwardMetadataContext(ctx)
		var err error
		resp, err = pbpeering.NewPeeringServiceClient(conn).GenerateToken(ctx, req)
		return err
	})
	if handled || err != nil {
		return resp, err
	}

	var authzCtx acl.AuthorizerContext
	entMeta := structs.DefaultEnterpriseMetaInPartition(req.Partition)
	authz, err := s.Backend.ResolveTokenAndDefaultMeta(external.TokenFromContext(ctx), entMeta, &authzCtx)
	if err != nil {
		return nil, err
	}

	if err := authz.ToAllowAuthorizer().PeeringWriteAllowed(&authzCtx); err != nil {
		return nil, err
	}

	ca, err := s.Backend.GetAgentCACertificates()
	if err != nil {
		return nil, err
	}

	// ServerExternalAddresses must be formatted as addr:port.
	var serverAddrs []string
	if len(req.ServerExternalAddresses) > 0 {
		serverAddrs = req.ServerExternalAddresses
	} else {
		serverAddrs, err = s.Backend.GetServerAddresses()
		if err != nil {
			return nil, err
		}
	}

	peeringOrNil, err := s.getExistingPeering(req.PeerName, entMeta.PartitionOrDefault())
	if err != nil {
		return nil, err
	}

	// validate that this peer name is not being used as a dialer already
	if err = validatePeer(peeringOrNil, false); err != nil {
		return nil, err
	}

	canRetry := true
RETRY_ONCE:
	id, err := s.getExistingOrCreateNewPeerID(req.PeerName, entMeta.PartitionOrDefault())
	if err != nil {
		return nil, err
	}
	writeReq := pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			ID:   id,
			Name: req.PeerName,
			Meta: req.Meta,

			// PartitionOrEmpty is used to avoid writing "default" in OSS.
			Partition: entMeta.PartitionOrEmpty(),
		},
	}
	if err := s.Backend.PeeringWrite(&writeReq); err != nil {
		// There's a possible race where two servers call Generate Token at the
		// same time with the same peer name for the first time. They both
		// generate an ID and try to insert and only one wins. This detects the
		// collision and forces the loser to discard its generated ID and use
		// the one from the other server.
		if canRetry && strings.Contains(err.Error(), "A peering already exists with the name") {
			canRetry = false
			goto RETRY_ONCE
		}
		return nil, fmt.Errorf("failed to write peering: %w", err)
	}

	q := state.Query{
		Value:          strings.ToLower(req.PeerName),
		EnterpriseMeta: *entMeta,
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
func (s *Server) Establish(
	ctx context.Context,
	req *pbpeering.EstablishRequest,
) (*pbpeering.EstablishResponse, error) {
	if !s.Config.PeeringEnabled {
		return nil, peeringNotEnabledErr
	}

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
	handled, err := s.ForwardRPC(&writeRequest, func(conn *grpc.ClientConn) error {
		ctx := external.ForwardMetadataContext(ctx)
		var err error
		resp, err = pbpeering.NewPeeringServiceClient(conn).Establish(ctx, req)
		return err
	})
	if handled || err != nil {
		return resp, err
	}

	defer metrics.MeasureSince([]string{"peering", "establish"}, time.Now())

	var authzCtx acl.AuthorizerContext
	entMeta := structs.DefaultEnterpriseMetaInPartition(req.Partition)
	authz, err := s.Backend.ResolveTokenAndDefaultMeta(external.TokenFromContext(ctx), entMeta, &authzCtx)
	if err != nil {
		return nil, err
	}

	if err := authz.ToAllowAuthorizer().PeeringWriteAllowed(&authzCtx); err != nil {
		return nil, err
	}

	peeringOrNil, err := s.getExistingPeering(req.PeerName, entMeta.PartitionOrDefault())
	if err != nil {
		return nil, err
	}

	// validate that this peer name is not being used as an acceptor already
	if err = validatePeer(peeringOrNil, true); err != nil {
		return nil, err
	}

	var id string
	if peeringOrNil != nil {
		id = peeringOrNil.ID
	} else {
		id, err = lib.GenerateUUID(s.Backend.CheckPeeringUUID)
		if err != nil {
			return nil, err
		}
	}

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
			ID:                  id,
			Name:                req.PeerName,
			PeerCAPems:          tok.CA,
			PeerServerAddresses: serverAddrs,
			PeerServerName:      tok.ServerName,
			PeerID:              tok.PeerID,
			Meta:                req.Meta,
			State:               pbpeering.PeeringState_ESTABLISHING,

			// PartitionOrEmpty is used to avoid writing "default" in OSS.
			Partition: entMeta.PartitionOrEmpty(),
		},
	}
	if err = s.Backend.PeeringWrite(writeReq); err != nil {
		return nil, fmt.Errorf("failed to write peering: %w", err)
	}
	// resp.Status == 0
	return resp, nil
}

// OPTIMIZE: Handle blocking queries
func (s *Server) PeeringRead(ctx context.Context, req *pbpeering.PeeringReadRequest) (*pbpeering.PeeringReadResponse, error) {
	if !s.Config.PeeringEnabled {
		return nil, peeringNotEnabledErr
	}

	if err := s.Backend.EnterpriseCheckPartitions(req.Partition); err != nil {
		return nil, grpcstatus.Error(codes.InvalidArgument, err.Error())
	}

	var resp *pbpeering.PeeringReadResponse
	handled, err := s.ForwardRPC(&readRequest, func(conn *grpc.ClientConn) error {
		ctx := external.ForwardMetadataContext(ctx)
		var err error
		resp, err = pbpeering.NewPeeringServiceClient(conn).PeeringRead(ctx, req)
		return err
	})
	if handled || err != nil {
		return resp, err
	}

	defer metrics.MeasureSince([]string{"peering", "read"}, time.Now())

	var authzCtx acl.AuthorizerContext
	entMeta := structs.DefaultEnterpriseMetaInPartition(req.Partition)
	authz, err := s.Backend.ResolveTokenAndDefaultMeta(external.TokenFromContext(ctx), entMeta, &authzCtx)
	if err != nil {
		return nil, err
	}

	if err := authz.ToAllowAuthorizer().PeeringReadAllowed(&authzCtx); err != nil {
		return nil, err
	}

	q := state.Query{
		Value:          strings.ToLower(req.Name),
		EnterpriseMeta: *entMeta,
	}
	_, peering, err := s.Backend.Store().PeeringRead(nil, q)
	if err != nil {
		return nil, err
	}
	if peering == nil {
		return &pbpeering.PeeringReadResponse{Peering: nil}, nil
	}

	cp := s.reconcilePeering(peering)
	return &pbpeering.PeeringReadResponse{Peering: cp}, nil
}

// OPTIMIZE: Handle blocking queries
func (s *Server) PeeringList(ctx context.Context, req *pbpeering.PeeringListRequest) (*pbpeering.PeeringListResponse, error) {
	if !s.Config.PeeringEnabled {
		return nil, peeringNotEnabledErr
	}

	if err := s.Backend.EnterpriseCheckPartitions(req.Partition); err != nil {
		return nil, grpcstatus.Error(codes.InvalidArgument, err.Error())
	}

	var resp *pbpeering.PeeringListResponse
	handled, err := s.ForwardRPC(&readRequest, func(conn *grpc.ClientConn) error {
		ctx := external.ForwardMetadataContext(ctx)
		var err error
		resp, err = pbpeering.NewPeeringServiceClient(conn).PeeringList(ctx, req)
		return err
	})
	if handled || err != nil {
		return resp, err
	}

	var authzCtx acl.AuthorizerContext
	entMeta := structs.DefaultEnterpriseMetaInPartition(req.Partition)
	authz, err := s.Backend.ResolveTokenAndDefaultMeta(external.TokenFromContext(ctx), entMeta, &authzCtx)
	if err != nil {
		return nil, err
	}

	if err := authz.ToAllowAuthorizer().PeeringReadAllowed(&authzCtx); err != nil {
		return nil, err
	}

	defer metrics.MeasureSince([]string{"peering", "list"}, time.Now())

	_, peerings, err := s.Backend.Store().PeeringList(nil, *entMeta)
	if err != nil {
		return nil, err
	}

	// reconcile the actual peering state; need to copy over the ds for peering
	var cPeerings []*pbpeering.Peering
	for _, p := range peerings {
		cp := s.reconcilePeering(p)
		cPeerings = append(cPeerings, cp)
	}

	return &pbpeering.PeeringListResponse{Peerings: cPeerings}, nil
}

// TODO(peering): Get rid of this func when we stop using the stream tracker for imported/ exported services and the peering state
// reconcilePeering enriches the peering with the following information:
// -- PeeringState.Active if the peering is active
// -- ImportedServicesCount and ExportedServicesCount
// NOTE: we return a new peering with this additional data
func (s *Server) reconcilePeering(peering *pbpeering.Peering) *pbpeering.Peering {
	streamState, found := s.Tracker.StreamStatus(peering.ID)
	if !found {
		s.Logger.Warn("did not find peer in stream tracker; cannot populate imported and"+
			" exported services count or reconcile peering state", "peerID", peering.ID)
		return peering
	} else {
		cp := copyPeering(peering)

		// reconcile pbpeering.PeeringState_Active
		if streamState.Connected {
			cp.State = pbpeering.PeeringState_ACTIVE
		}

		// add imported & exported services counts
		cp.ImportedServiceCount = streamState.GetImportedServicesCount()
		cp.ExportedServiceCount = streamState.GetExportedServicesCount()

		return cp
	}
}

// TODO(peering): As of writing, this method is only used in tests to set up Peerings in the state store.
// Consider removing if we can find another way to populate state store in peering_endpoint_test.go
func (s *Server) PeeringWrite(ctx context.Context, req *pbpeering.PeeringWriteRequest) (*pbpeering.PeeringWriteResponse, error) {
	if !s.Config.PeeringEnabled {
		return nil, peeringNotEnabledErr
	}

	if err := s.Backend.EnterpriseCheckPartitions(req.Peering.Partition); err != nil {
		return nil, grpcstatus.Error(codes.InvalidArgument, err.Error())
	}

	var resp *pbpeering.PeeringWriteResponse
	handled, err := s.ForwardRPC(&writeRequest, func(conn *grpc.ClientConn) error {
		ctx := external.ForwardMetadataContext(ctx)
		var err error
		resp, err = pbpeering.NewPeeringServiceClient(conn).PeeringWrite(ctx, req)
		return err
	})
	if handled || err != nil {
		return resp, err
	}

	defer metrics.MeasureSince([]string{"peering", "write"}, time.Now())

	var authzCtx acl.AuthorizerContext
	entMeta := structs.DefaultEnterpriseMetaInPartition(req.Peering.Partition)
	authz, err := s.Backend.ResolveTokenAndDefaultMeta(external.TokenFromContext(ctx), entMeta, &authzCtx)
	if err != nil {
		return nil, err
	}

	if err := authz.ToAllowAuthorizer().PeeringWriteAllowed(&authzCtx); err != nil {
		return nil, err
	}

	if req.Peering == nil {
		return nil, fmt.Errorf("missing required peering body")
	}

	id, err := s.getExistingOrCreateNewPeerID(req.Peering.Name, entMeta.PartitionOrDefault())
	if err != nil {
		return nil, err
	}
	req.Peering.ID = id

	err = s.Backend.PeeringWrite(req)
	if err != nil {
		return nil, err
	}
	return &pbpeering.PeeringWriteResponse{}, nil
}

func (s *Server) PeeringDelete(ctx context.Context, req *pbpeering.PeeringDeleteRequest) (*pbpeering.PeeringDeleteResponse, error) {
	if !s.Config.PeeringEnabled {
		return nil, peeringNotEnabledErr
	}

	if err := s.Backend.EnterpriseCheckPartitions(req.Partition); err != nil {
		return nil, grpcstatus.Error(codes.InvalidArgument, err.Error())
	}

	var resp *pbpeering.PeeringDeleteResponse
	handled, err := s.ForwardRPC(&writeRequest, func(conn *grpc.ClientConn) error {
		ctx := external.ForwardMetadataContext(ctx)
		var err error
		resp, err = pbpeering.NewPeeringServiceClient(conn).PeeringDelete(ctx, req)
		return err
	})
	if handled || err != nil {
		return resp, err
	}

	defer metrics.MeasureSince([]string{"peering", "delete"}, time.Now())

	var authzCtx acl.AuthorizerContext
	entMeta := structs.DefaultEnterpriseMetaInPartition(req.Partition)
	authz, err := s.Backend.ResolveTokenAndDefaultMeta(external.TokenFromContext(ctx), entMeta, &authzCtx)
	if err != nil {
		return nil, err
	}

	if err := authz.ToAllowAuthorizer().PeeringWriteAllowed(&authzCtx); err != nil {
		return nil, err
	}

	q := state.Query{
		Value:          strings.ToLower(req.Name),
		EnterpriseMeta: *entMeta,
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
			ID:        existing.ID,
			Name:      req.Name,
			State:     pbpeering.PeeringState_DELETING,
			DeletedAt: structs.TimeToProto(time.Now().UTC()),

			// PartitionOrEmpty is used to avoid writing "default" in OSS.
			Partition: entMeta.PartitionOrEmpty(),
		},
	}
	err = s.Backend.PeeringWrite(writeReq)
	if err != nil {
		return nil, err
	}
	return &pbpeering.PeeringDeleteResponse{}, nil
}

// OPTIMIZE: Handle blocking queries
func (s *Server) TrustBundleRead(ctx context.Context, req *pbpeering.TrustBundleReadRequest) (*pbpeering.TrustBundleReadResponse, error) {
	if !s.Config.PeeringEnabled {
		return nil, peeringNotEnabledErr
	}

	if err := s.Backend.EnterpriseCheckPartitions(req.Partition); err != nil {
		return nil, grpcstatus.Error(codes.InvalidArgument, err.Error())
	}

	var resp *pbpeering.TrustBundleReadResponse
	handled, err := s.ForwardRPC(&readRequest, func(conn *grpc.ClientConn) error {
		ctx := external.ForwardMetadataContext(ctx)
		var err error
		resp, err = pbpeering.NewPeeringServiceClient(conn).TrustBundleRead(ctx, req)
		return err
	})
	if handled || err != nil {
		return resp, err
	}

	defer metrics.MeasureSince([]string{"peering", "trust_bundle_read"}, time.Now())

	var authzCtx acl.AuthorizerContext
	entMeta := structs.DefaultEnterpriseMetaInPartition(req.Partition)
	authz, err := s.Backend.ResolveTokenAndDefaultMeta(external.TokenFromContext(ctx), entMeta, &authzCtx)
	if err != nil {
		return nil, err
	}

	if err := authz.ToAllowAuthorizer().ServiceWriteAnyAllowed(&authzCtx); err != nil {
		return nil, err
	}

	idx, trustBundle, err := s.Backend.Store().PeeringTrustBundleRead(nil, state.Query{
		Value:          req.Name,
		EnterpriseMeta: *entMeta,
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
// OPTIMIZE: Handle blocking queries
func (s *Server) TrustBundleListByService(ctx context.Context, req *pbpeering.TrustBundleListByServiceRequest) (*pbpeering.TrustBundleListByServiceResponse, error) {
	if !s.Config.PeeringEnabled {
		return nil, peeringNotEnabledErr
	}

	if err := s.Backend.EnterpriseCheckPartitions(req.Partition); err != nil {
		return nil, grpcstatus.Error(codes.InvalidArgument, err.Error())
	}
	if err := s.Backend.EnterpriseCheckNamespaces(req.Namespace); err != nil {
		return nil, grpcstatus.Error(codes.InvalidArgument, err.Error())
	}
	if req.ServiceName == "" {
		return nil, errors.New("missing service name")
	}

	var resp *pbpeering.TrustBundleListByServiceResponse
	handled, err := s.ForwardRPC(&readRequest, func(conn *grpc.ClientConn) error {
		ctx := external.ForwardMetadataContext(ctx)
		var err error
		resp, err = pbpeering.NewPeeringServiceClient(conn).TrustBundleListByService(ctx, req)
		return err
	})
	if handled || err != nil {
		return resp, err
	}

	defer metrics.MeasureSince([]string{"peering", "trust_bundle_list_by_service"}, time.Now())

	var authzCtx acl.AuthorizerContext
	entMeta := acl.NewEnterpriseMetaWithPartition(req.Partition, req.Namespace)
	authz, err := s.Backend.ResolveTokenAndDefaultMeta(external.TokenFromContext(ctx), &entMeta, &authzCtx)
	if err != nil {
		return nil, err
	}

	if err := authz.ToAllowAuthorizer().ServiceWriteAllowed(req.ServiceName, &authzCtx); err != nil {
		return nil, err
	}

	var (
		idx     uint64
		bundles []*pbpeering.PeeringTrustBundle
	)

	switch {
	case req.Kind == string(structs.ServiceKindMeshGateway):
		idx, bundles, err = s.Backend.Store().PeeringTrustBundleList(nil, entMeta)
	case req.ServiceName != "":
		idx, bundles, err = s.Backend.Store().TrustBundleListByService(nil, req.ServiceName, s.Datacenter, entMeta)
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

func (s *Server) getExistingOrCreateNewPeerID(peerName, partition string) (string, error) {
	peeringOrNil, err := s.getExistingPeering(peerName, partition)
	if err != nil {
		return "", err
	}
	if peeringOrNil != nil {
		return peeringOrNil.ID, nil
	}

	id, err := lib.GenerateUUID(s.Backend.CheckPeeringUUID)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (s *Server) getExistingPeering(peerName, partition string) (*pbpeering.Peering, error) {
	q := state.Query{
		Value:          strings.ToLower(peerName),
		EnterpriseMeta: *structs.NodeEnterpriseMetaInPartition(partition),
	}
	_, peering, err := s.Backend.Store().PeeringRead(nil, q)
	if err != nil {
		return nil, err
	}

	return peering, nil
}

// validatePeer enforces the following rule for an existing peering:
// - if a peering already exists, it can only be used as an acceptor or dialer
//
// We define a DIALER as a peering that has server addresses (or a peering that is created via the Establish endpoint)
// Conversely, we define an ACCEPTOR as a peering that is created via the GenerateToken endpoint
func validatePeer(peering *pbpeering.Peering, allowedToDial bool) error {
	if peering != nil && peering.ShouldDial() != allowedToDial {
		if allowedToDial {
			return fmt.Errorf("cannot create peering with name: %q; there is an existing peering expecting to be dialed", peering.Name)
		} else {
			return fmt.Errorf("cannot create peering with name: %q; there is already an established peering", peering.Name)
		}
	}

	return nil
}

func copyPeering(p *pbpeering.Peering) *pbpeering.Peering {
	var copyP pbpeering.Peering
	proto.Merge(&copyP, p)

	return &copyP
}
