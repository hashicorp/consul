// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package peerstream

import (
	"time"

	"google.golang.org/grpc"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/acl/resolver"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/private/pbpeering"
	"github.com/hashicorp/consul/proto/private/pbpeerstream"
)

// TODO(peering): fix up these interfaces to be more testable now that they are
// extracted from private peering

const (
	defaultOutgoingHeartbeatInterval = 15 * time.Second
	defaultIncomingHeartbeatTimeout  = 2 * time.Minute
)

type Server struct {
	Config

	Tracker *Tracker
}

type Config struct {
	Backend     Backend
	GetStore    func() StateStore
	Logger      hclog.Logger
	ForwardRPC  func(structs.RPCInfo, func(*grpc.ClientConn) error) (bool, error)
	ACLResolver ACLResolver
	// Datacenter of the Consul server this gRPC server is hosted on
	Datacenter     string
	ConnectEnabled bool

	// outgoingHeartbeatInterval is how often we send a heartbeat.
	outgoingHeartbeatInterval time.Duration

	// incomingHeartbeatTimeout is how long we'll wait between receiving heartbeats before we close the connection.
	incomingHeartbeatTimeout time.Duration
}

//go:generate mockery --name ACLResolver --inpackage
type ACLResolver interface {
	ResolveTokenAndDefaultMeta(string, *acl.EnterpriseMeta, *acl.AuthorizerContext) (resolver.Result, error)
}

func NewServer(cfg Config) *Server {
	requireNotNil(cfg.Backend, "Backend")
	requireNotNil(cfg.GetStore, "GetStore")
	requireNotNil(cfg.Logger, "Logger")
	// requireNotNil(cfg.ACLResolver, "ACLResolver") // TODO(peering): reenable check when ACLs are required
	if cfg.Datacenter == "" {
		panic("Datacenter is required")
	}
	if cfg.outgoingHeartbeatInterval == 0 {
		cfg.outgoingHeartbeatInterval = defaultOutgoingHeartbeatInterval
	}
	if cfg.incomingHeartbeatTimeout == 0 {
		cfg.incomingHeartbeatTimeout = defaultIncomingHeartbeatTimeout
	}
	return &Server{
		Config:  cfg,
		Tracker: NewTracker(cfg.incomingHeartbeatTimeout),
	}
}

func requireNotNil(v interface{}, name string) {
	if v == nil {
		panic(name + " is required")
	}
}

var _ pbpeerstream.PeerStreamServiceServer = (*Server)(nil)

func (s *Server) Register(grpcServer *grpc.Server) {
	pbpeerstream.RegisterPeerStreamServiceServer(grpcServer, s)
}

type Backend interface {
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

	ValidateProposedPeeringSecret(id string) (bool, error)
	PeeringSecretsWrite(req *pbpeering.SecretsWriteRequest) error
	PeeringTerminateByID(req *pbpeering.PeeringTerminateByIDRequest) error
	PeeringTrustBundleWrite(req *pbpeering.PeeringTrustBundleWriteRequest) error
	CatalogRegister(req *structs.RegisterRequest) error
	CatalogDeregister(req *structs.DeregisterRequest) error
	PeeringWrite(req *pbpeering.PeeringWriteRequest) error
}

// StateStore provides a read-only interface for querying Peering data.
type StateStore interface {
	PeeringRead(ws memdb.WatchSet, q state.Query) (uint64, *pbpeering.Peering, error)
	PeeringReadByID(ws memdb.WatchSet, id string) (uint64, *pbpeering.Peering, error)
	PeeringList(ws memdb.WatchSet, entMeta acl.EnterpriseMeta) (uint64, []*pbpeering.Peering, error)
	PeeringTrustBundleRead(ws memdb.WatchSet, q state.Query) (uint64, *pbpeering.PeeringTrustBundle, error)
	PeeringTrustBundleList(ws memdb.WatchSet, entMeta acl.EnterpriseMeta) (uint64, []*pbpeering.PeeringTrustBundle, error)
	PeeringSecretsRead(ws memdb.WatchSet, peerID string) (*pbpeering.PeeringSecrets, error)
	ExportedServicesForPeer(ws memdb.WatchSet, peerID, dc string) (uint64, *structs.ExportedServiceList, error)
	ServiceDump(ws memdb.WatchSet, kind structs.ServiceKind, useKind bool, entMeta *acl.EnterpriseMeta, peerName string) (uint64, structs.CheckServiceNodes, error)
	CheckServiceNodes(ws memdb.WatchSet, serviceName string, entMeta *acl.EnterpriseMeta, peerName string) (uint64, structs.CheckServiceNodes, error)
	NodeServiceList(ws memdb.WatchSet, nodeNameOrID string, entMeta *acl.EnterpriseMeta, peerName string) (uint64, *structs.NodeServiceList, error)
	CAConfig(ws memdb.WatchSet) (uint64, *structs.CAConfiguration, error)
	TrustBundleListByService(ws memdb.WatchSet, service, dc string, entMeta acl.EnterpriseMeta) (uint64, []*pbpeering.PeeringTrustBundle, error)
	ServiceList(ws memdb.WatchSet, entMeta *acl.EnterpriseMeta, peerName string) (uint64, structs.ServiceList, error)
	ConfigEntry(ws memdb.WatchSet, kind, name string, entMeta *acl.EnterpriseMeta) (uint64, structs.ConfigEntry, error)
	AbandonCh() <-chan struct{}
}
