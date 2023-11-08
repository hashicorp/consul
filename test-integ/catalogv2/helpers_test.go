// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package catalogv2

import (
	"strings"

	"github.com/hashicorp/consul/testing/deployer/topology"
)

func clusterPrefixForUpstream(u *topology.Upstream) string {
	if u.Peer == "" {
		if u.ID.PartitionOrDefault() == "default" {
			return strings.Join([]string{u.PortName, u.ID.Name, u.ID.Namespace, u.Cluster, "internal"}, ".")
		} else {
			return strings.Join([]string{u.PortName, u.ID.Name, u.ID.Namespace, u.ID.Partition, u.Cluster, "internal-v1"}, ".")
		}
	} else {
		return strings.Join([]string{u.ID.Name, u.ID.Namespace, u.Peer, "external"}, ".")
	}
}
