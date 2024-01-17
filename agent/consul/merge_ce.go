//go:build !consulent
// +build !consulent

package consul

import (
	"fmt"

	"github.com/hashicorp/serf/serf"
)

func (md *lanMergeDelegate) enterpriseNotifyMergeMember(m *serf.Member) error {
	if memberPartition := m.Tags["ap"]; memberPartition != "" {
		return fmt.Errorf("Member '%s' part of partition '%s'; Partitions are a Consul Enterprise feature",
			m.Name, memberPartition)
	}
	if segment := m.Tags["segment"]; segment != "" {
		return fmt.Errorf("Member '%s' part of segment '%s'; Network Segments are a Consul Enterprise feature",
			m.Name, segment)
	}
	return nil
}
