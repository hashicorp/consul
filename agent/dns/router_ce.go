// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package dns

import (
	"fmt"

	"github.com/hashicorp/consul/agent/discovery"
)

// canonicalNameForResult returns the canonical name for a discovery result.
func canonicalNameForResult(result *discovery.Result, domain string) string {
	switch result.Type {
	case discovery.ResultTypeService:
		return fmt.Sprintf("%s.%s.%s.%s", result.Target, "service", result.Tenancy.Datacenter, domain)
	case discovery.ResultTypeNode:
		if result.Tenancy.PeerName != "" {
			// We must return a more-specific DNS name for peering so
			// that there is no ambiguity with lookups.
			return fmt.Sprintf("%s.node.%s.peer.%s",
				result.Target,
				result.Tenancy.PeerName,
				domain)
		}
		// Return a simpler format for non-peering nodes.
		return fmt.Sprintf("%s.node.%s.%s", result.Target, result.Tenancy.Datacenter, domain)
	case discovery.ResultTypeWorkload:
		return fmt.Sprintf("%s.workload.%s", result.Target, domain)
	}
	return ""
}
