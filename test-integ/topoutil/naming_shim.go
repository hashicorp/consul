// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package topoutil

import (
	"github.com/hashicorp/consul/testing/deployer/topology"
)

// Deprecated: NewFortioWorkloadWithDefaults
func NewFortioServiceWithDefaults(
	cluster string,
	sid topology.ID,
	mut func(*topology.Workload),
) *topology.Workload {
	return NewFortioWorkloadWithDefaults(cluster, sid, mut)
}

// Deprecated: NewBlankspaceWorkloadWithDefaults
func NewBlankspaceServiceWithDefaults(
	cluster string,
	sid topology.ID,
	mut func(*topology.Workload),
) *topology.Workload {
	return NewBlankspaceWorkloadWithDefaults(cluster, sid, mut)
}
