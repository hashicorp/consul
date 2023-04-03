package consul

import (
	"container/ring"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/acl/resolver"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/grpc-external/services/peerstream"
	"github.com/hashicorp/consul/agent/rpc/peering"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/ipaddr"
	"github.com/hashicorp/consul/lib"
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

// GetTLSMaterials returns the TLS materials for the dialer to dial the acceptor using TLS.
// It returns the server name to validate, and the CA certificate to validate with.
func (b *PeeringBackend) GetTLSMaterials(generatingToken bool) (string, []string, error) {
	if generatingToken {
		if !b.srv.config.ConnectEnabled {
			return "", nil, fmt.Errorf("connect.enabled must be set to true in the server's configuration when generating peering tokens")
		}
		if b.srv.config.GRPCTLSPort <= 0 && !b.srv.tlsConfigurator.GRPCServerUseTLS() {
			return "", nil, fmt.Errorf("TLS for gRPC must be enabled when generating peering tokens")
		}
	}

	roots, err := b.srv.getCARoots(nil, b.srv.fsm.State())
	if err != nil {
		return "", nil, fmt.Errorf("failed to fetch roots: %w", err)
	}
	if len(roots.Roots) == 0 || roots.TrustDomain == "" {
		return "", nil, fmt.Errorf("CA has not finished initializing")
	}

	serverName := connect.PeeringServerSAN(b.srv.config.Datacenter, roots.TrustDomain)

	var caPems []string
	for _, r := range roots.Roots {
		caPems = append(caPems, lib.EnsureTrailingNewline(r.RootCert))
	}

	return serverName, caPems, nil
}

// GetLocalServerAddresses looks up server or mesh gateway addresses from the state store for a peer to dial.
func (b *PeeringBackend) GetLocalServerAddresses() ([]string, error) {
	store := b.srv.fsm.State()

	useGateways, err := b.PeerThroughMeshGateways(nil)
	if err != nil {
		// For inbound traffic we prefer to fail fast if we can't determine whether we should peer through MGW.
		// This prevents unexpectedly sharing local server addresses when a user only intended to peer through gateways.
		return nil, fmt.Errorf("failed to determine if peering should happen through mesh gateways: %w", err)
	}
	if useGateways {
		return meshGatewayAdresses(store, nil, true)
	}
	return serverAddresses(store)
}

// GetDialAddresses returns: the addresses to cycle through when dialing a peer's servers,
// an optional buffer of just gateway addresses, and an optional error.
// The resulting ring buffer is front-loaded with the local mesh gateway addresses if they are present.
func (b *PeeringBackend) GetDialAddresses(logger hclog.Logger, ws memdb.WatchSet, peerID string) (*ring.Ring, *ring.Ring, error) {
	newRing, err := b.fetchPeerServerAddresses(ws, peerID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to refresh peer server addresses, will continue to use initial addresses: %w", err)
	}

	gatewayRing, err := b.maybeFetchGatewayAddresses(ws)
	if err != nil {
		// If we couldn't fetch the mesh gateway addresses we fall back to dialing the remote server addresses.
		logger.Warn("failed to refresh local gateway addresses, will attempt to dial peer directly", "error", err)
		return newRing, nil, nil
	}
	if gatewayRing != nil {
		// The ordering is important here. We always want to start with the mesh gateway
		// addresses and fallback to the remote addresses, so we append the server addresses
		// in newRing to gatewayRing. We also need a new ring to prevent mixing up pointers
		// with the gateway-only buffer
		compositeRing := ring.New(gatewayRing.Len() + newRing.Len())
		gatewayRing.Do(func(s any) {
			compositeRing.Value = s.(string)
			compositeRing = compositeRing.Next()
		})

		newRing.Do(func(s any) {
			compositeRing.Value = s.(string)
			compositeRing = compositeRing.Next()
		})
		newRing = compositeRing
	}
	return newRing, gatewayRing, nil
}

// fetchPeerServerAddresses will return a ring buffer with the latest peer server addresses.
// If the peering is no longer active or does not have addresses, then we return an error.
func (b *PeeringBackend) fetchPeerServerAddresses(ws memdb.WatchSet, peerID string) (*ring.Ring, error) {
	_, peering, err := b.Store().PeeringReadByID(ws, peerID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch peer %q: %w", peerID, err)
	}
	if peering == nil {
		return nil, fmt.Errorf("unknown peering %q", peerID)
	}
	if peering.DeletedAt != nil && !structs.IsZeroProtoTime(peering.DeletedAt) {
		return nil, fmt.Errorf("peering %q was deleted", peerID)
	}
	return bufferFromAddresses(peering.GetAddressesToDial())
}

// maybeFetchGatewayAddresses will return a ring buffer with the latest gateway addresses if the
// local datacenter is configured to peer through mesh gateways and there are local gateways registered.
// If neither of these are true then we return a nil buffer.
func (b *PeeringBackend) maybeFetchGatewayAddresses(ws memdb.WatchSet) (*ring.Ring, error) {
	useGateways, err := b.PeerThroughMeshGateways(ws)
	if err != nil {
		return nil, fmt.Errorf("failed to determine if peering should happen through mesh gateways: %w", err)
	}
	if useGateways {
		addresses, err := meshGatewayAdresses(b.srv.fsm.State(), ws, false)
		if err != nil {
			return nil, fmt.Errorf("error fetching local mesh gateway addresses: %w", err)
		}
		return bufferFromAddresses(addresses)
	}
	return nil, nil
}

// PeerThroughMeshGateways determines if the config entry to enable peering control plane
// traffic through a mesh gateway is set to enable.
func (b *PeeringBackend) PeerThroughMeshGateways(ws memdb.WatchSet) (bool, error) {
	_, rawEntry, err := b.srv.fsm.State().ConfigEntry(ws, structs.MeshConfig, structs.MeshConfigMesh, acl.DefaultEnterpriseMeta())
	if err != nil {
		return false, fmt.Errorf("failed to read mesh config entry: %w", err)
	}
	mesh, ok := rawEntry.(*structs.MeshConfigEntry)
	if rawEntry != nil && !ok {
		return false, fmt.Errorf("got unexpected type for mesh config entry: %T", rawEntry)
	}
	return mesh.PeerThroughMeshGateways(), nil

}

func bufferFromAddresses(addresses []string) (*ring.Ring, error) {
	// IMPORTANT: The address ring buffer must always be length > 0
	if len(addresses) == 0 {
		return nil, fmt.Errorf("no known addresses")
	}
	ring := ring.New(len(addresses))
	for _, addr := range addresses {
		ring.Value = addr
		ring = ring.Next()
	}
	return ring, nil
}

func meshGatewayAdresses(state *state.Store, ws memdb.WatchSet, wan bool) ([]string, error) {
	_, nodes, err := state.ServiceDump(ws, structs.ServiceKindMeshGateway, true, acl.DefaultEnterpriseMeta(), structs.DefaultPeerKeyword)
	if err != nil {
		return nil, fmt.Errorf("failed to dump gateway addresses: %w", err)
	}

	var addrs []string
	for _, node := range nodes {
		_, addr, port := node.BestAddress(wan)
		addrs = append(addrs, ipaddr.FormatAddressPort(addr, port))
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("servers are configured to PeerThroughMeshGateways, but no mesh gateway instances are registered")
	}
	return addrs, nil
}

func parseNodeAddr(node *structs.ServiceNode) string {
	// Prefer the wan address
	if v, ok := node.TaggedAddresses[structs.TaggedAddressWAN]; ok {
		return v
	}
	return node.Address
}

func serverAddresses(state *state.Store) ([]string, error) {
	_, nodes, err := state.ServiceNodes(nil, structs.ConsulServiceName, structs.DefaultEnterpriseMetaInDefaultPartition(), structs.DefaultPeerKeyword)
	if err != nil {
		return nil, err
	}
	var addrs []string
	for _, node := range nodes {
		addr := parseNodeAddr(node)

		// Prefer the TLS port if it is defined.
		grpcPortStr := node.ServiceMeta["grpc_tls_port"]
		if v, err := strconv.Atoi(grpcPortStr); err == nil && v > 0 {
			addrs = append(addrs, addr+":"+grpcPortStr)
			continue
		}
		// Fallback to the standard port if TLS is not defined.
		grpcPortStr = node.ServiceMeta["grpc_port"]
		if v, err := strconv.Atoi(grpcPortStr); err == nil && v > 0 {
			addrs = append(addrs, addr+":"+grpcPortStr)
			continue
		}
		// Skip node if neither defined.
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("a grpc bind port must be specified in the configuration for all servers")
	}
	return addrs, nil
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
