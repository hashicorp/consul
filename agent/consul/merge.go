// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"fmt"
	"strings"
	"sync"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/serf/serf"

	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/types"
)

// lanMergeDelegate is used to handle a cluster merge on the LAN gossip
// ring. We check that the peers are in the same datacenter and abort the
// merge if there is a mis-match.
type lanMergeDelegate struct {
	dc        string
	nodeID    types.NodeID
	nodeName  string
	segment   string
	server    bool
	partition string
}

// uniqueIDMinVersion is the lowest version where we insist that nodes
// have a unique ID.
var uniqueIDMinVersion = version.Must(version.NewVersion("0.8.5"))

func (md *lanMergeDelegate) NotifyMerge(members []*serf.Member) error {
	nodeMap := make(map[types.NodeID]string)
	for _, m := range members {
		if rawID, ok := m.Tags["id"]; ok && rawID != "" {
			if m.Status == serf.StatusLeft {
				continue
			}

			nodeID := types.NodeID(rawID)

			// See if there's another node that conflicts with us.
			if (nodeID == md.nodeID) && !strings.EqualFold(m.Name, md.nodeName) {
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

		if err := md.enterpriseNotifyMergeMember(m); err != nil {
			return err
		}
	}
	return nil
}

// wanMergeDelegate is used to handle a cluster merge on the WAN gossip
// ring. We check that the peers are server nodes and abort the merge
// otherwise.
type wanMergeDelegate struct {
	localDatacenter string

	federationDisabledLock sync.Mutex
	federationDisabled     bool
}

// SetWANFederationDisabled selectively disables the wan pool from accepting
// non-local members. If the toggle changed the current value it returns true.
func (md *wanMergeDelegate) SetWANFederationDisabled(disabled bool) bool {
	md.federationDisabledLock.Lock()
	prior := md.federationDisabled
	md.federationDisabled = disabled
	md.federationDisabledLock.Unlock()

	return prior != disabled
}

func (md *wanMergeDelegate) NotifyMerge(members []*serf.Member) error {
	// Deliberately hold this lock during the entire merge so calls to
	// SetWANFederationDisabled returning immediately imply that the flag takes
	// effect for all future merges.
	md.federationDisabledLock.Lock()
	defer md.federationDisabledLock.Unlock()

	for _, m := range members {
		ok, srv := metadata.IsConsulServer(*m)
		if !ok {
			return fmt.Errorf("Member '%s' is not a server", m.Name)
		}

		if md.federationDisabled {
			if srv.Datacenter != md.localDatacenter {
				return fmt.Errorf("Member '%s' part of wrong datacenter '%s'; WAN federation is disabled", m.Name, srv.Datacenter)
			}
		}
	}
	return nil
}
