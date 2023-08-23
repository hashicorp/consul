package structs

import (
	"net"

	"github.com/hashicorp/raft"
)

// RaftServer has information about a server in the Raft configuration.
type RaftServer struct {
	// ID is the unique ID for the server. These are currently the same
	// as the address, but they will be changed to a real GUID in a future
	// release of Consul.
	ID raft.ServerID

	// Node is the node name of the server, as known by Consul, or this
	// will be set to "(unknown)" otherwise.
	Node string

	// Address is the IP:port of the server, used for Raft communications.
	Address raft.ServerAddress

	// Leader is true if this server is the current cluster leader.
	Leader bool

	// Protocol version is the raft protocol version used by the server
	ProtocolVersion string

	// Voter is true if this server has a vote in the cluster. This might
	// be false if the server is staging and still coming online, or if
	// it's a non-voting server, which will be added in a future release of
	// Consul.
	Voter bool
}

// RaftConfigurationResponse is returned when querying for the current Raft
// configuration.
type RaftConfigurationResponse struct {
	// Servers has the list of servers in the Raft configuration.
	Servers []*RaftServer

	// Index has the Raft index of this configuration.
	Index uint64
}

// RaftRemovePeerRequest is used by the Operator endpoint to apply a Raft
// operation on a specific Raft peer by address in the form of "IP:port".
type RaftRemovePeerRequest struct {
	// Datacenter is the target this request is intended for.
	Datacenter string

	// Address is the peer to remove, in the form "IP:port".
	Address raft.ServerAddress

	// ID is the peer ID to remove.
	ID raft.ServerID

	// WriteRequest holds the ACL token to go along with this request.
	WriteRequest
}

// RequestDatacenter returns the datacenter for a given request.
func (op *RaftRemovePeerRequest) RequestDatacenter() string {
	return op.Datacenter
}

// AutopilotSetConfigRequest is used by the Operator endpoint to update the
// current Autopilot configuration of the cluster.
type AutopilotSetConfigRequest struct {
	// Datacenter is the target this request is intended for.
	Datacenter string

	// Config is the new Autopilot configuration to use.
	Config AutopilotConfig

	// CAS controls whether to use check-and-set semantics for this request.
	CAS bool

	// WriteRequest holds the ACL token to go along with this request.
	WriteRequest
}

// RequestDatacenter returns the datacenter for a given request.
func (op *AutopilotSetConfigRequest) RequestDatacenter() string {
	return op.Datacenter
}

// (Enterprise-only) NetworkSegment is the configuration for a network segment, which is an
// isolated serf group on the LAN.
type NetworkSegment struct {
	// Name is the name of the segment.
	Name string

	// Bind is the bind address for this segment.
	Bind *net.TCPAddr

	// Advertise is the advertise address of this segment.
	Advertise *net.TCPAddr

	// RPCListener is whether to bind a separate RPC listener on the bind address
	// for this segment.
	RPCListener bool
}
