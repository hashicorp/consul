package pbpeering

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/lib"
)

// RequestDatacenter implements structs.RPCInfo
func (req *GenerateTokenRequest) RequestDatacenter() string {
	// Cross-datacenter requests are not allowed for peering actions because
	// they rely on WAN-federation.
	return ""
}

// RequestDatacenter implements structs.RPCInfo
func (req *EstablishRequest) RequestDatacenter() string {
	// Cross-datacenter requests are not allowed for peering actions because
	// they rely on WAN-federation.
	return ""
}

// RequestDatacenter implements structs.RPCInfo
func (req *PeeringReadRequest) RequestDatacenter() string {
	// Cross-datacenter requests are not allowed for peering actions because
	// they rely on WAN-federation.
	return ""
}

// RequestDatacenter implements structs.RPCInfo
func (req *PeeringListRequest) RequestDatacenter() string {
	// Cross-datacenter requests are not allowed for peering actions because
	// they rely on WAN-federation.
	return ""
}

// RequestDatacenter implements structs.RPCInfo
func (req *PeeringWriteRequest) RequestDatacenter() string {
	// Cross-datacenter requests are not allowed for peering actions because
	// they rely on WAN-federation.
	return ""
}

// RequestDatacenter implements structs.RPCInfo
func (req *PeeringDeleteRequest) RequestDatacenter() string {
	// Cross-datacenter requests are not allowed for peering actions because
	// they rely on WAN-federation.
	return ""
}

// RequestDatacenter implements structs.RPCInfo
func (req *TrustBundleReadRequest) RequestDatacenter() string {
	// Cross-datacenter requests are not allowed for peering actions because
	// they rely on WAN-federation.
	return ""
}

// RequestDatacenter implements structs.RPCInfo
func (req *TrustBundleListByServiceRequest) RequestDatacenter() string {
	// Cross-datacenter requests are not allowed for peering actions because
	// they rely on WAN-federation.
	return ""
}

// ShouldDial returns true when the peering was stored via the peering initiation endpoint,
// AND the peering is not marked as terminated by our peer.
// If we generated a token for this peer we did not store our server addresses under PeerServerAddresses or ManualServerAddresses.
// These server addresses are for dialing, and only the peer initiating the peering will do the dialing.
func (p *Peering) ShouldDial() bool {
	return len(p.PeerServerAddresses) > 0 || len(p.ManualServerAddresses) > 0
}

// GetAddressesToDial returns the listing of addresses that should be dialed for the peering.
// It will ensure that manual addresses take precedence, if any are defined.
func (p *Peering) GetAddressesToDial() []string {
	if len(p.ManualServerAddresses) > 0 {
		return p.ManualServerAddresses
	}
	return p.PeerServerAddresses
}

func (x PeeringState) GoString() string {
	return x.String()
}

// ConcatenatedRootPEMs concatenates and returns all PEM-encoded public certificates
// in a peer's trust bundle.
func (b *PeeringTrustBundle) ConcatenatedRootPEMs() string {
	if b == nil {
		return ""
	}

	var rootPEMs string
	for _, pem := range b.RootPEMs {
		rootPEMs += lib.EnsureTrailingNewline(pem)
	}
	return rootPEMs
}

// enumcover:PeeringState
func PeeringStateToAPI(s PeeringState) api.PeeringState {
	switch s {
	case PeeringState_PENDING:
		return api.PeeringStatePending
	case PeeringState_ESTABLISHING:
		return api.PeeringStateEstablishing
	case PeeringState_ACTIVE:
		return api.PeeringStateActive
	case PeeringState_FAILING:
		return api.PeeringStateFailing
	case PeeringState_DELETING:
		return api.PeeringStateDeleting
	case PeeringState_TERMINATED:
		return api.PeeringStateTerminated
	case PeeringState_UNDEFINED:
		fallthrough
	default:
		return api.PeeringStateUndefined
	}
}

// enumcover:api.PeeringState
func PeeringStateFromAPI(t api.PeeringState) PeeringState {
	switch t {
	case api.PeeringStatePending:
		return PeeringState_PENDING
	case api.PeeringStateEstablishing:
		return PeeringState_ESTABLISHING
	case api.PeeringStateActive:
		return PeeringState_ACTIVE
	case api.PeeringStateFailing:
		return PeeringState_FAILING
	case api.PeeringStateDeleting:
		return PeeringState_DELETING
	case api.PeeringStateTerminated:
		return PeeringState_TERMINATED
	case api.PeeringStateUndefined:
		fallthrough
	default:
		return PeeringState_UNDEFINED
	}
}

func StreamStatusToAPI(status *StreamStatus) api.PeeringStreamStatus {
	return api.PeeringStreamStatus{
		ImportedServices: status.ImportedServices,
		ExportedServices: status.ExportedServices,
		LastHeartbeat:    TimePtrFromProto(status.LastHeartbeat),
		LastReceive:      TimePtrFromProto(status.LastReceive),
		LastSend:         TimePtrFromProto(status.LastSend),
	}
}

func StreamStatusFromAPI(status api.PeeringStreamStatus) *StreamStatus {
	return &StreamStatus{
		ImportedServices: status.ImportedServices,
		ExportedServices: status.ExportedServices,
		LastHeartbeat:    TimePtrToProto(status.LastHeartbeat),
		LastReceive:      TimePtrToProto(status.LastReceive),
		LastSend:         TimePtrToProto(status.LastSend),
	}
}

func (p *Peering) IsActive() bool {
	if p == nil || p.State == PeeringState_TERMINATED {
		return false
	}
	if p.DeletedAt == nil {
		return true
	}

	// The minimum protobuf timestamp is the Unix epoch rather than go's zero.
	return structs.IsZeroProtoTime(p.DeletedAt)
}

// Validate is a validation helper that checks whether a secret ID is embedded in the container type.
func (s *SecretsWriteRequest) Validate() error {
	if s.PeerID == "" {
		return errors.New("missing peer ID")
	}
	switch r := s.Request.(type) {
	case *SecretsWriteRequest_GenerateToken:
		if r != nil && r.GenerateToken.GetEstablishmentSecret() != "" {
			return nil
		}
	case *SecretsWriteRequest_Establish:
		if r != nil && r.Establish.GetActiveStreamSecret() != "" {
			return nil
		}
	case *SecretsWriteRequest_ExchangeSecret:
		if r != nil && r.ExchangeSecret.GetPendingStreamSecret() != "" {
			return nil
		}
	case *SecretsWriteRequest_PromotePending:
		if r != nil && r.PromotePending.GetActiveStreamSecret() != "" {
			return nil
		}
	default:
		return fmt.Errorf("unexpected request type %T", s.Request)
	}

	return errors.New("missing secret ID")
}

// TLSDialOption returns the gRPC DialOption to secure the transport if CAPems
// ara available. If no CAPems were provided in the peering token then the
// WithInsecure dial option is returned.
func (p *Peering) TLSDialOption() (grpc.DialOption, error) {
	//nolint:staticcheck
	tlsOption := grpc.WithInsecure()

	if len(p.PeerCAPems) > 0 {
		var haveCerts bool
		pool := x509.NewCertPool()
		for _, pem := range p.PeerCAPems {
			if !pool.AppendCertsFromPEM([]byte(pem)) {
				return nil, fmt.Errorf("failed to parse PEM %s", pem)
			}
			if len(pem) > 0 {
				haveCerts = true
			}
		}
		if !haveCerts {
			return nil, fmt.Errorf("failed to build cert pool from peer CA pems")
		}
		cfg := tls.Config{
			ServerName: p.PeerServerName,
			RootCAs:    pool,
		}
		tlsOption = grpc.WithTransportCredentials(credentials.NewTLS(&cfg))
	}
	return tlsOption, nil
}

func (p *Peering) ToAPI() *api.Peering {
	var t api.Peering
	PeeringToAPI(p, &t)
	return &t
}

// TODO consider using mog for this
func (resp *PeeringListResponse) ToAPI() []*api.Peering {
	list := make([]*api.Peering, len(resp.Peerings))
	for i, p := range resp.Peerings {
		list[i] = p.ToAPI()
	}
	return list
}

// TODO consider using mog for this
func (resp *GenerateTokenResponse) ToAPI() *api.PeeringGenerateTokenResponse {
	var t api.PeeringGenerateTokenResponse
	GenerateTokenResponseToAPI(resp, &t)
	return &t
}

// TODO consider using mog for this
func (resp *EstablishResponse) ToAPI() *api.PeeringEstablishResponse {
	var t api.PeeringEstablishResponse
	EstablishResponseToAPI(resp, &t)
	return &t
}

func (r *RemoteInfo) IsEmpty() bool {
	if r == nil {
		return true
	}
	return r.Partition == "" && r.Datacenter == ""
}

// convenience
func NewGenerateTokenRequestFromAPI(req *api.PeeringGenerateTokenRequest) *GenerateTokenRequest {
	if req == nil {
		return nil
	}
	t := &GenerateTokenRequest{}
	GenerateTokenRequestFromAPI(req, t)
	return t
}

// convenience
func NewEstablishRequestFromAPI(req *api.PeeringEstablishRequest) *EstablishRequest {
	if req == nil {
		return nil
	}
	t := &EstablishRequest{}
	EstablishRequestFromAPI(req, t)
	return t
}

func TimePtrFromProto(s *timestamppb.Timestamp) *time.Time {
	if s == nil {
		return nil
	}
	t := s.AsTime()
	return &t
}

func TimePtrToProto(s *time.Time) *timestamppb.Timestamp {
	if s == nil {
		return nil
	}
	return timestamppb.New(*s)
}

// DeepCopy returns a copy of the PeeringTrustBundle that can be passed around
// without worrying about the receiver unsafely modifying it. It is used by the
// generated DeepCopy methods in proxycfg.
func (o *PeeringTrustBundle) DeepCopy() *PeeringTrustBundle {
	cp, ok := proto.Clone(o).(*PeeringTrustBundle)
	if !ok {
		panic(fmt.Sprintf("failed to clone *PeeringTrustBundle, got: %T", cp))
	}
	return cp
}
