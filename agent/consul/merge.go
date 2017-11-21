package consul

import (
	"fmt"

	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/types"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/serf/serf"
)

// lanMergeDelegate is used to handle a cluster merge on the LAN gossip
// ring. We check that the peers are in the same datacenter and abort the
// merge if there is a mis-match.
type lanMergeDelegate struct {
	dc       string
	nodeID   types.NodeID
	nodeName string
	segment  string
}

// uniqueIDMinVersion is the lowest version where we insist that nodes
// have a unique ID.
var uniqueIDMinVersion = version.Must(version.NewVersion("0.8.5"))

func (md *lanMergeDelegate) NotifyMerge(members []*serf.Member) error {
	nodeMap := make(map[types.NodeID]string)
	for _, m := range members {
		if rawID, ok := m.Tags["id"]; ok && rawID != "" {
			nodeID := types.NodeID(rawID)

			// See if there's another node that conflicts with us.
			if (nodeID == md.nodeID) && (m.Name != md.nodeName) {
				return fmt.Errorf("Member '%s' has conflicting node ID '%s' with this agent's ID",
					m.Name, nodeID)
			}

			// See if there are any two nodes that conflict with each
			// other. This lets us only do joins into a hygienic
			// cluster now that node IDs are critical for operation.
			if other, ok := nodeMap[nodeID]; ok {
				return fmt.Errorf("Member '%s' has conflicting node ID '%s' with member '%s'",
					m.Name, nodeID, other)
			}

			// Only map nodes with a version that's >= than when
			// we made host-based IDs opt-in, which helps prevent
			// chaos when upgrading older clusters. See #3070 for
			// more details.
			if ver, err := metadata.Build(m); err == nil {
				if ver.Compare(uniqueIDMinVersion) >= 0 {
					nodeMap[nodeID] = m.Name
				}
			}
		}

		if ok, dc := isConsulNode(*m); ok {
			if dc != md.dc {
				return fmt.Errorf("Member '%s' part of wrong datacenter '%s'",
					m.Name, dc)
			}
		}

		if ok, parts := metadata.IsConsulServer(*m); ok {
			if parts.Datacenter != md.dc {
				return fmt.Errorf("Member '%s' part of wrong datacenter '%s'",
					m.Name, parts.Datacenter)
			}
		}

		if segment := m.Tags["segment"]; segment != md.segment {
			return fmt.Errorf("Member '%s' part of wrong segment '%s' (expected '%s')",
				m.Name, segment, md.segment)
		}
	}
	return nil
}

// wanMergeDelegate is used to handle a cluster merge on the WAN gossip
// ring. We check that the peers are server nodes and abort the merge
// otherwise.
type wanMergeDelegate struct {
}

func (md *wanMergeDelegate) NotifyMerge(members []*serf.Member) error {
	for _, m := range members {
		ok, _ := metadata.IsConsulServer(*m)
		if !ok {
			return fmt.Errorf("Member '%s' is not a server", m.Name)
		}
	}
	return nil
}
