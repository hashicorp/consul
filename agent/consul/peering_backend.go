package consul

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"

	"google.golang.org/grpc"

	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/rpc/peering"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbpeering"
)

type peeringBackend struct {
	srv      *Server
	connPool GRPCClientConner
	apply    *peeringApply
}

var _ peering.Backend = (*peeringBackend)(nil)

// NewPeeringBackend returns a peering.Backend implementation that is bound to the given server.
func NewPeeringBackend(srv *Server, connPool GRPCClientConner) peering.Backend {
	return &peeringBackend{
		srv:      srv,
		connPool: connPool,
		apply:    &peeringApply{srv: srv},
	}
}

func (b *peeringBackend) Forward(info structs.RPCInfo, f func(*grpc.ClientConn) error) (handled bool, err error) {
	// Only forward the request if the dc in the request matches the server's datacenter.
	if info.RequestDatacenter() != "" && info.RequestDatacenter() != b.srv.config.Datacenter {
		return false, fmt.Errorf("requests to generate peering tokens cannot be forwarded to remote datacenters")
	}
	return b.srv.ForwardGRPC(b.connPool, info, f)
}

// GetAgentCACertificates gets the server's raw CA data from its TLS Configurator.
func (b *peeringBackend) GetAgentCACertificates() ([]string, error) {
	// TODO(peering): handle empty CA pems
	return b.srv.tlsConfigurator.ManualCAPems(), nil
}

// GetServerAddresses looks up server node addresses from the state store.
func (b *peeringBackend) GetServerAddresses() ([]string, error) {
	state := b.srv.fsm.State()
	_, nodes, err := state.ServiceNodes(nil, "consul", structs.DefaultEnterpriseMetaInDefaultPartition(), structs.DefaultPeerKeyword)
	if err != nil {
		return nil, err
	}
	var addrs []string
	for _, node := range nodes {
		addrs = append(addrs, node.Address+":"+strconv.Itoa(node.ServicePort))
	}
	return addrs, nil
}

// GetServerName returns the SNI to be returned in the peering token data which
// will be used by peers when establishing peering connections over TLS.
func (b *peeringBackend) GetServerName() string {
	return b.srv.tlsConfigurator.ServerSNI(b.srv.config.Datacenter, "")
}

// EncodeToken encodes a peering token as a bas64-encoded representation of JSON (for now).
func (b *peeringBackend) EncodeToken(tok *structs.PeeringToken) ([]byte, error) {
	jsonToken, err := json.Marshal(tok)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal token: %w", err)
	}
	return []byte(base64.StdEncoding.EncodeToString(jsonToken)), nil
}

// DecodeToken decodes a peering token from a base64-encoded JSON byte array (for now).
func (b *peeringBackend) DecodeToken(tokRaw []byte) (*structs.PeeringToken, error) {
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

func (s peeringBackend) Subscribe(req *stream.SubscribeRequest) (*stream.Subscription, error) {
	return s.srv.publisher.Subscribe(req)
}

func (b *peeringBackend) Store() peering.Store {
	return b.srv.fsm.State()
}

func (b *peeringBackend) Apply() peering.Apply {
	return b.apply
}

func (b *peeringBackend) EnterpriseCheckPartitions(partition string) error {
	return b.enterpriseCheckPartitions(partition)
}

type peeringApply struct {
	srv *Server
}

func (a *peeringApply) PeeringWrite(req *pbpeering.PeeringWriteRequest) error {
	_, err := a.srv.raftApplyProtobuf(structs.PeeringWriteType, req)
	return err
}

func (a *peeringApply) PeeringDelete(req *pbpeering.PeeringDeleteRequest) error {
	_, err := a.srv.raftApplyProtobuf(structs.PeeringDeleteType, req)
	return err
}

// TODO(peering): This needs RPC metrics interceptor since it's not triggered by an RPC.
func (a *peeringApply) PeeringTerminateByID(req *pbpeering.PeeringTerminateByIDRequest) error {
	_, err := a.srv.raftApplyProtobuf(structs.PeeringTerminateByIDType, req)
	return err
}

func (a *peeringApply) CatalogRegister(req *structs.RegisterRequest) error {
	_, err := a.srv.leaderRaftApply("Catalog.Register", structs.RegisterRequestType, req)
	return err
}

var _ peering.Apply = (*peeringApply)(nil)
