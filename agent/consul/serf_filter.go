package consul

import (
	"fmt"

	"github.com/hashicorp/consul/agent/structs"
)

type LANMemberFilter struct {
	Partition   string
	Segment     string
	AllSegments bool
}

func (f LANMemberFilter) Validate() error {
	if f.AllSegments && f.Segment != "" {
		return fmt.Errorf("cannot specify both allSegments and segment filters")
	}
	if (f.AllSegments || f.Segment != "") && !structs.IsDefaultPartition(f.Partition) {
		return fmt.Errorf("segments do not exist outside of the default partition")
	}
	return nil
}

func (f LANMemberFilter) PartitionOrDefault() string {
	return structs.PartitionOrDefault(f.Partition)
}
