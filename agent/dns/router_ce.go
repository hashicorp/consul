// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package dns

import (
	"fmt"

	"github.com/hashicorp/consul/agent/discovery"
)

// canonicalNameForResult returns the canonical name for a discovery result.
func canonicalNameForResult(resultType discovery.ResultType, target, domain string,
	tenancy discovery.ResultTenancy, portName string) string {
	switch resultType {
	case discovery.ResultTypeService:
		return fmt.Sprintf("%s.%s.%s.%s", target, "service", tenancy.Datacenter, domain)
	case discovery.ResultTypeNode:
		if tenancy.PeerName != "" {
			// We must return a more-specific DNS name for peering so
			// that there is no ambiguity with lookups.
			return fmt.Sprintf("%s.node.%s.peer.%s",
				target,
				tenancy.PeerName,
				domain)
		}
		// Return a simpler format for non-peering nodes.
		return fmt.Sprintf("%s.node.%s.%s", target, tenancy.Datacenter, domain)
	case discovery.ResultTypeWorkload:
		if portName != "" {
			return fmt.Sprintf("%s.port.%s.workload.%s", portName, target, domain)
		}
		return fmt.Sprintf("%s.workload.%s", target, domain)
	}
	return ""
}

// getDefaultPartitionName returns the default partition name.
func getDefaultPartitionName() string {
	return ""
}
