package structs

import (
	"github.com/hashicorp/raft"
)

// RaftConfigrationResponse is returned when querying for the current Raft
// configuration. This has the low-level Raft structure, as well as some
// supplemental information from Consul.
type RaftConfigurationResponse struct {
	// Configuration is the low-level Raft configuration structure.
	Configuration raft.Configuration

	// NodeMap maps IDs in the Raft configuration to node names known by
	// Consul. It's possible that not all configuration entries may have
	// an entry here if the node isn't known to Consul. Given how this is
	// generated, this may also contain entries that aren't present in the
	// Raft configuration.
	NodeMap map[raft.ServerID]string

	// Leader is the ID of the current Raft leader. This may be blank if
	// there isn't one.
	Leader raft.ServerID
}

// RaftPeerByAddressRequest is used by the Operator endpoint to apply a Raft
// operation on a specific Raft peer by address in the form of "IP:port".
type RaftPeerByAddressRequest struct {
	// Datacenter is the target this request is intended for.
	Datacenter string

	// Address is the peer to remove, in the form "IP:port".
	Address raft.ServerAddress

	// WriteRequest holds the ACL token to go along with this request.
	WriteRequest
}

// RequestDatacenter returns the datacenter for a given request.
func (op *RaftPeerByAddressRequest) RequestDatacenter() string {
	return op.Datacenter
}
