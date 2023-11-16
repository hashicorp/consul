// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package topoutil

import (
	"testing"

	"github.com/hashicorp/consul/testing/deployer/topology"
)

// Deprecated: DestinationEndpointStatus
func (a *Asserter) UpstreamEndpointStatus(
	t *testing.T,
	workload *topology.Workload,
	clusterName string,
	healthStatus string,
	count int,
) {
	a.DestinationEndpointStatus(t, workload, clusterName, healthStatus, count)
}

// Deprecated: NewFortioWorkloadWithDefaults
func NewFortioServiceWithDefaults(
	cluster string,
	sid topology.ID,
	nodeVersion topology.NodeVersion,
	mut func(*topology.Workload),
) *topology.Workload {
	return NewFortioWorkloadWithDefaults(cluster, sid, nodeVersion, mut)
}

// Deprecated: NewBlankspaceWorkloadWithDefaults
func NewBlankspaceServiceWithDefaults(
	cluster string,
	sid topology.ID,
	nodeVersion topology.NodeVersion,
	mut func(*topology.Workload),
) *topology.Workload {
	return NewBlankspaceWorkloadWithDefaults(cluster, sid, nodeVersion, mut)
}
