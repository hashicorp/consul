// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package catalogv2

import (
	"strings"

	"github.com/hashicorp/consul/testing/deployer/topology"
)

// Deprecated: clusterPrefixForDestination
func clusterPrefixForUpstream(dest *topology.Destination) string {
	return clusterPrefixForDestination(dest)
}

func clusterPrefixForDestination(dest *topology.Destination) string {
	if dest.Peer == "" {
		return clusterPrefix(dest.PortName, dest.ID, dest.Cluster)
	} else {
		return strings.Join([]string{dest.ID.Name, dest.ID.Namespace, dest.Peer, "external"}, ".")
	}
}

func clusterPrefix(port string, svcID topology.ID, cluster string) string {
	if svcID.PartitionOrDefault() == "default" {
		return strings.Join([]string{port, svcID.Name, svcID.Namespace, cluster, "internal"}, ".")
	} else {
		return strings.Join([]string{port, svcID.Name, svcID.Namespace, svcID.Partition, cluster, "internal-v1"}, ".")
	}
}
