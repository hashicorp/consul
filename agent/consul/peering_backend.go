package consul

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/acl/resolver"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/grpc-external/services/peerstream"
	"github.com/hashicorp/consul/agent/rpc/peering"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbpeering"
)

type PeeringBackend struct {
	// TODO(peering): accept a smaller interface; maybe just funcs from the server that we actually need: DC, IsLeader, etc
	srv *Server

	leaderAddrLock sync.RWMutex
	leaderAddr     string
}

var _ peering.Backend = (*PeeringBackend)(nil)
var _ peerstream.Backend = (*PeeringBackend)(nil)

// NewPeeringBackend returns a peering.Backend implementation that is bound to the given server.
func NewPeeringBackend(srv *Server) *PeeringBackend {
	return &PeeringBackend{
		srv: srv,
	}
}

// SetLeaderAddress is called on a raft.LeaderObservation in a go routine
// in the consul server; see trackLeaderChanges()
func (b *PeeringBackend) SetLeaderAddress(addr string) {
	b.leaderAddrLock.Lock()
	b.leaderAddr = addr
	b.leaderAddrLock.Unlock()
}

// GetLeaderAddress provides the best hint for the current address of the
// leader. There is no guarantee that this is the actual address of the
// leader.
func (b *PeeringBackend) GetLeaderAddress() string {
	b.leaderAddrLock.RLock()
	defer b.leaderAddrLock.RUnlock()
	return b.leaderAddr
}

// GetAgentCACertificates gets the server's raw CA data from its TLS Configurator.
func (b *PeeringBackend) GetAgentCACertificates() ([]string, error) {
	// TODO(peering): handle empty CA pems
	return b.srv.tlsConfigurator.GRPCManualCAPems(), nil
}

func parseNodeAddr(node *structs.ServiceNode) string {
	// Prefer the wan address
	if v, ok := node.TaggedAddresses[structs.TaggedAddressWAN]; ok {
		return v
	}
	return node.Address
}

// GetServerAddresses looks up server node addresses from the state store.
func (b *PeeringBackend) GetServerAddresses() ([]string, error) {
	state := b.srv.fsm.State()
	_, nodes, err := state.ServiceNodes(nil, structs.ConsulServiceName, structs.DefaultEnterpriseMetaInDefaultPartition(), structs.DefaultPeerKeyword)
	if err != nil {
		return nil, err
	}
	var addrs []string
	for _, node := range nodes {
		addr := parseNodeAddr(node)

		grpcPortStr := node.ServiceMeta["grpc_port"]
		if v, err := strconv.Atoi(grpcPortStr); err != nil || v < 1 {
			continue // skip server that isn't exporting public gRPC properly
		}
		addrs = append(addrs, addr+":"+grpcPortStr)
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("a grpc bind port must be specified in the configuration for all servers")
	}
	return addrs, nil
}

// GetServerName returns the SNI to be returned in the peering token data which
// will be used by peers when establishing peering connections over TLS.
func (b *PeeringBackend) GetServerName() string {
	return b.srv.tlsConfigurator.ServerSNI(b.srv.config.Datacenter, "")
}

// EncodeToken encodes a peering token as a bas64-encoded representation of JSON (for now).
func (b *PeeringBackend) EncodeToken(tok *structs.PeeringToken) ([]byte, error) {
	jsonToken, err := json.Marshal(tok)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal token: %w", err)
	}
	return []byte(base64.StdEncoding.EncodeToString(jsonToken)), nil
}

// DecodeToken decodes a peering token from a base64-encoded JSON byte array (for now).
func (b *PeeringBackend) DecodeToken(tokRaw []byte) (*structs.PeeringToken, error) {
	tokJSONRaw, err := base64.StdEncoding.DecodeString(string(tokRaw))
	if err != nil {
		return nil, fmt.Errorf("failed to decode token: %w", err)
	}
	var tok structs.PeeringToken
	if err := json.Unmarshal(tokJSONRaw, &tok); err != nil {
		return nil, err
	}
	return &tok, nil
}

func (s *PeeringBackend) Subscribe(req *stream.SubscribeRequest) (*stream.Subscription, error) {
	return s.srv.publisher.Subscribe(req)
}

func (b *PeeringBackend) Store() peering.Store {
	return b.srv.fsm.State()
}

func (b *PeeringBackend) EnterpriseCheckPartitions(partition string) error {
	return b.enterpriseCheckPartitions(partition)
}

func (b *PeeringBackend) EnterpriseCheckNamespaces(namespace string) error {
	return b.enterpriseCheckNamespaces(namespace)
}

func (b *PeeringBackend) IsLeader() bool {
	return b.srv.IsLeader()
}

func (b *PeeringBackend) CheckPeeringUUID(id string) (bool, error) {
	state := b.srv.fsm.State()
	if _, existing, err := state.PeeringReadByID(nil, id); err != nil {
		return false, err
	} else if existing != nil {
		return false, nil
	}

	return true, nil
}

func (b *PeeringBackend) ValidateProposedPeeringSecret(id string) (bool, error) {
	return b.srv.fsm.State().ValidateProposedPeeringSecretUUID(id)
}

func (b *PeeringBackend) PeeringSecretsWrite(req *pbpeering.SecretsWriteRequest) error {
	_, err := b.srv.raftApplyProtobuf(structs.PeeringSecretsWriteType, req)
	return err
}

func (b *PeeringBackend) PeeringWrite(req *pbpeering.PeeringWriteRequest) error {
	_, err := b.srv.raftApplyProtobuf(structs.PeeringWriteType, req)
	return err
}

// TODO(peering): This needs RPC metrics interceptor since it's not triggered by an RPC.
func (b *PeeringBackend) PeeringTerminateByID(req *pbpeering.PeeringTerminateByIDRequest) error {
	_, err := b.srv.raftApplyProtobuf(structs.PeeringTerminateByIDType, req)
	return err
}

func (b *PeeringBackend) PeeringTrustBundleWrite(req *pbpeering.PeeringTrustBundleWriteRequest) error {
	_, err := b.srv.raftApplyProtobuf(structs.PeeringTrustBundleWriteType, req)
	return err
}

func (b *PeeringBackend) CatalogRegister(req *structs.RegisterRequest) error {
	_, err := b.srv.leaderRaftApply("Catalog.Register", structs.RegisterRequestType, req)
	return err
}

func (b *PeeringBackend) CatalogDeregister(req *structs.DeregisterRequest) error {
	_, err := b.srv.leaderRaftApply("Catalog.Deregister", structs.DeregisterRequestType, req)
	return err
}

func (b *PeeringBackend) ResolveTokenAndDefaultMeta(token string, entMeta *acl.EnterpriseMeta, authzCtx *acl.AuthorizerContext) (resolver.Result, error) {
	return b.srv.ResolveTokenAndDefaultMeta(token, entMeta, authzCtx)
}
