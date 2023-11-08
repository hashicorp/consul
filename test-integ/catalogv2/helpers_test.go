// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package catalogv2

import (
	"strings"

	"github.com/hashicorp/consul/testing/deployer/topology"
)

func clusterPrefixForUpstream(u *topology.Upstream) string {
	if u.Peer == "" {
		return clusterPrefix(u.PortName, u.ID, u.Cluster)
	} else {
		return strings.Join([]string{u.ID.Name, u.ID.Namespace, u.Peer, "external"}, ".")
	}
}

func clusterPrefix(port string, svcID topology.ServiceID, cluster string) string {
	if svcID.PartitionOrDefault() == "default" {
		return strings.Join([]string{port, svcID.Name, svcID.Namespace, cluster, "internal"}, ".")
	} else {
		return strings.Join([]string{port, svcID.Name, svcID.Namespace, svcID.Partition, cluster, "internal-v1"}, ".")
	}
}
