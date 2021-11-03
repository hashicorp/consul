// +build !consulent

package consul

import (
	"net"
	"time"

	"github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"
	"github.com/hashicorp/serf/serf"

	"github.com/hashicorp/consul/agent/structs"
)

var SegmentOSSSummaries = []prometheus.SummaryDefinition{
	{
		Name: []string{"leader", "reconcile"},
		Help: "Measures the time spent updating the raft store from the serf member information.",
	},
}

// LANSegmentAddr is used to return the address used for the given LAN segment.
func (s *Server) LANSegmentAddr(name string) string {
	return ""
}

// setupSegmentRPC returns an error if any segments are defined since the OSS
// version of Consul doesn't support them.
func (s *Server) setupSegmentRPC() (map[string]net.Listener, error) {
	if len(s.config.Segments) > 0 {
		return nil, structs.ErrSegmentsNotSupported
	}

	return nil, nil
}

// setupSegments returns an error if any segments are defined since the OSS
// version of Consul doesn't support them.
func (s *Server) setupSegments(config *Config, port int, rpcListeners map[string]net.Listener) error {
	if len(config.Segments) > 0 {
		return structs.ErrSegmentsNotSupported
	}

	return nil
}

// floodSegments is a NOP in the OSS version of Consul.
func (s *Server) floodSegments(config *Config) {
}

func getSerfMemberEnterpriseMeta(member serf.Member) *structs.EnterpriseMeta {
	return structs.NodeEnterpriseMetaInDefaultPartition()
}

// reconcile is used to reconcile the differences between Serf membership and
// what is reflected in our strongly consistent store. Mainly we need to ensure
// all live nodes are registered, all failed nodes are marked as such, and all
// left nodes are deregistered.
func (s *Server) reconcile() (err error) {
	defer metrics.MeasureSince([]string{"leader", "reconcile"}, time.Now())

	members := s.serfLAN.Members()
	knownMembers := make(map[string]struct{})
	for _, member := range members {
		if err := s.reconcileMember(member); err != nil {
			return err
		}
		knownMembers[member.Name] = struct{}{}
	}

	// Reconcile any members that have been reaped while we were not the
	// leader.
	return s.reconcileReaped(knownMembers, nil)
}
