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

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/dns"
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

// GenerateToken implements the PeeringService RPC method to generate a
// peering token which is the initial step in establishing a peering relationship
// with other Consul clusters.
func (s *Server) GenerateToken(
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
	handled, err := s.ForwardRPC(req, func(conn *grpc.ClientConn) error {
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

	canRetry := true
RETRY_ONCE:
	id, err := s.getExistingOrCreateNewPeerID(req.PeerName, req.Partition)
	if err != nil {
		return nil, err
	}

	writeReq := pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			ID:   id,
			Name: req.PeerName,
			// TODO(peering): Normalize from ACL token once this endpoint is guarded by ACLs.
			Partition: req.PartitionOrDefault(),
			Meta:      req.Meta,
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
func (s *Server) Establish(
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
	handled, err := s.ForwardRPC(req, func(conn *grpc.ClientConn) error {
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

	id, err := s.getExistingOrCreateNewPeerID(req.PeerName, req.Partition)
	if err != nil {
		return nil, err
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
		},
	}
	if err = s.Backend.PeeringWrite(writeReq); err != nil {
		return nil, fmt.Errorf("failed to write peering: %w", err)
	}
	// resp.Status == 0
	return resp, nil
}

func (s *Server) PeeringRead(ctx context.Context, req *pbpeering.PeeringReadRequest) (*pbpeering.PeeringReadResponse, error) {
	if err := s.Backend.EnterpriseCheckPartitions(req.Partition); err != nil {
		return nil, grpcstatus.Error(codes.InvalidArgument, err.Error())
	}

	var resp *pbpeering.PeeringReadResponse
	handled, err := s.ForwardRPC(req, func(conn *grpc.ClientConn) error {
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
	if peering == nil {
		return &pbpeering.PeeringReadResponse{Peering: nil}, nil
	}

	cp := copyPeeringWithNewState(peering, s.reconciledStreamStateHint(peering.ID, peering.State))

	// add imported services count
	st, found := s.Tracker.StreamStatus(peering.ID)
	if !found {
		s.Logger.Trace("did not find peer in stream tracker when reading peer", "peerID", peering.ID)
	} else {
		cp.ImportedServiceCount = uint64(len(st.ImportedServices))
		cp.ExportedServiceCount = uint64(len(st.ExportedServices))
	}

	return &pbpeering.PeeringReadResponse{Peering: cp}, nil
}

func (s *Server) PeeringList(ctx context.Context, req *pbpeering.PeeringListRequest) (*pbpeering.PeeringListResponse, error) {
	if err := s.Backend.EnterpriseCheckPartitions(req.Partition); err != nil {
		return nil, grpcstatus.Error(codes.InvalidArgument, err.Error())
	}

	var resp *pbpeering.PeeringListResponse
	handled, err := s.ForwardRPC(req, func(conn *grpc.ClientConn) error {
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

	// reconcile the actual peering state; need to copy over the ds for peering
	var cPeerings []*pbpeering.Peering
	for _, p := range peerings {
		cp := copyPeeringWithNewState(p, s.reconciledStreamStateHint(p.ID, p.State))

		// add imported services count
		st, found := s.Tracker.StreamStatus(p.ID)
		if !found {
			s.Logger.Trace("did not find peer in stream tracker when listing peers", "peerID", p.ID)
		} else {
			cp.ImportedServiceCount = uint64(len(st.ImportedServices))
			cp.ExportedServiceCount = uint64(len(st.ExportedServices))
		}

		cPeerings = append(cPeerings, cp)
	}
	return &pbpeering.PeeringListResponse{Peerings: cPeerings}, nil
}

// TODO(peering): Maybe get rid of this when actually monitoring the stream health
// reconciledStreamStateHint peaks into the streamTracker and determines whether a peering should be marked
// as PeeringState.Active or not
func (s *Server) reconciledStreamStateHint(pID string, pState pbpeering.PeeringState) pbpeering.PeeringState {
	streamState, found := s.Tracker.StreamStatus(pID)

	if found && streamState.Connected {
		return pbpeering.PeeringState_ACTIVE
	}

	// default, no reconciliation
	return pState
}

// TODO(peering): As of writing, this method is only used in tests to set up Peerings in the state store.
// Consider removing if we can find another way to populate state store in peering_endpoint_test.go
func (s *Server) PeeringWrite(ctx context.Context, req *pbpeering.PeeringWriteRequest) (*pbpeering.PeeringWriteResponse, error) {
	if err := s.Backend.EnterpriseCheckPartitions(req.Peering.Partition); err != nil {
		return nil, grpcstatus.Error(codes.InvalidArgument, err.Error())
	}

	var resp *pbpeering.PeeringWriteResponse
	handled, err := s.ForwardRPC(req, func(conn *grpc.ClientConn) error {
		var err error
		resp, err = pbpeering.NewPeeringServiceClient(conn).PeeringWrite(ctx, req)
		return err
	})
	if handled || err != nil {
		return resp, err
	}

	defer metrics.MeasureSince([]string{"peering", "write"}, time.Now())
	// TODO(peering): ACL check request token

	if req.Peering == nil {
		return nil, fmt.Errorf("missing required peering body")
	}

	id, err := s.getExistingOrCreateNewPeerID(req.Peering.Name, req.Peering.Partition)
	if err != nil {
		return nil, err
	}
	req.Peering.ID = id

	// TODO(peering): handle blocking queries
	err = s.Backend.PeeringWrite(req)
	if err != nil {
		return nil, err
	}
	return &pbpeering.PeeringWriteResponse{}, nil
}

func (s *Server) PeeringDelete(ctx context.Context, req *pbpeering.PeeringDeleteRequest) (*pbpeering.PeeringDeleteResponse, error) {
	if err := s.Backend.EnterpriseCheckPartitions(req.Partition); err != nil {
		return nil, grpcstatus.Error(codes.InvalidArgument, err.Error())
	}

	var resp *pbpeering.PeeringDeleteResponse
	handled, err := s.ForwardRPC(req, func(conn *grpc.ClientConn) error {
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
			ID:        existing.ID,
			Name:      req.Name,
			Partition: req.Partition,
			State:     pbpeering.PeeringState_DELETING,
			DeletedAt: structs.TimeToProto(time.Now().UTC()),
		},
	}
	err = s.Backend.PeeringWrite(writeReq)
	if err != nil {
		return nil, err
	}
	return &pbpeering.PeeringDeleteResponse{}, nil
}

func (s *Server) TrustBundleRead(ctx context.Context, req *pbpeering.TrustBundleReadRequest) (*pbpeering.TrustBundleReadResponse, error) {
	if err := s.Backend.EnterpriseCheckPartitions(req.Partition); err != nil {
		return nil, grpcstatus.Error(codes.InvalidArgument, err.Error())
	}

	var resp *pbpeering.TrustBundleReadResponse
	handled, err := s.ForwardRPC(req, func(conn *grpc.ClientConn) error {
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
func (s *Server) TrustBundleListByService(ctx context.Context, req *pbpeering.TrustBundleListByServiceRequest) (*pbpeering.TrustBundleListByServiceResponse, error) {
	if err := s.Backend.EnterpriseCheckPartitions(req.Partition); err != nil {
		return nil, grpcstatus.Error(codes.InvalidArgument, err.Error())
	}
	if err := s.Backend.EnterpriseCheckNamespaces(req.Namespace); err != nil {
		return nil, grpcstatus.Error(codes.InvalidArgument, err.Error())
	}

	var resp *pbpeering.TrustBundleListByServiceResponse
	handled, err := s.ForwardRPC(req, func(conn *grpc.ClientConn) error {
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
		idx, bundles, err = s.Backend.Store().TrustBundleListByService(nil, req.ServiceName, s.Datacenter, entMeta)
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

func (s *Server) getExistingOrCreateNewPeerID(peerName, partition string) (string, error) {
	q := state.Query{
		Value:          strings.ToLower(peerName),
		EnterpriseMeta: *structs.NodeEnterpriseMetaInPartition(partition),
	}
	_, peering, err := s.Backend.Store().PeeringRead(nil, q)
	if err != nil {
		return "", err
	}
	if peering != nil {
		return peering.ID, nil
	}

	id, err := lib.GenerateUUID(s.Backend.CheckPeeringUUID)
	if err != nil {
		return "", err
	}
	return id, nil
}

func copyPeeringWithNewState(p *pbpeering.Peering, state pbpeering.PeeringState) *pbpeering.Peering {
	return &pbpeering.Peering{
		ID:                   p.ID,
		Name:                 p.Name,
		Partition:            p.Partition,
		DeletedAt:            p.DeletedAt,
		Meta:                 p.Meta,
		PeerID:               p.PeerID,
		PeerCAPems:           p.PeerCAPems,
		PeerServerAddresses:  p.PeerServerAddresses,
		PeerServerName:       p.PeerServerName,
		CreateIndex:          p.CreateIndex,
		ModifyIndex:          p.ModifyIndex,
		ImportedServiceCount: p.ImportedServiceCount,
		ExportedServiceCount: p.ExportedServiceCount,

		State: state,
	}
}
