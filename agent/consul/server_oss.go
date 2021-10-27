//go:build !consulent
// +build !consulent

package consul

import (
	"github.com/hashicorp/serf/serf"
	"google.golang.org/grpc"

	"github.com/hashicorp/consul/agent/structs"
)

func (s *Server) removeFailedNodeEnterprise(remove func(*serf.Serf, string) error, node, wanNode string) error {
	// nothing to do for oss
	return nil
}

func (s *Server) registerEnterpriseGRPCServices(deps Deps, srv *grpc.Server) {}

// lanPoolAllMembers only returns our own segment or partition's members, because
// OSS servers can't be in multiple segments or partitions.
func (s *Server) lanPoolAllMembers() ([]serf.Member, error) {
	return s.LANMembersInAgentPartition(), nil
}

// LANMembers returns the LAN members for one of:
//
// - the requested partition
// - the requested segment
// - all segments
//
// This is limited to segments and partitions that the node is a member of.
func (s *Server) LANMembers(filter LANMemberFilter) ([]serf.Member, error) {
	if err := filter.Validate(); err != nil {
		return nil, err
	}
	if filter.Segment != "" {
		return nil, structs.ErrSegmentsNotSupported
	}
	return s.LANMembersInAgentPartition(), nil
}
