// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package naming

import (
	"fmt"

	"github.com/hashicorp/consul/agent/structs"
)

const (
	// OriginalDestinationClusterName is the name we give to the passthrough
	// cluster which redirects transparently-proxied requests to their original
	// destination outside the mesh. This cluster prevents Consul from blocking
	// connections to destinations outside of the catalog when in transparent
	// proxy mode.
	OriginalDestinationClusterName = "original-destination"
	VirtualIPTag                   = "virtual"
)

func CustomizeClusterName(clusterName string, chain *structs.CompiledDiscoveryChain) string {
	if chain == nil || chain.CustomizationHash == "" {
		return clusterName
	}
	// Use a colon to delimit this prefix instead of a dot to avoid a
	// theoretical collision problem with subsets.
	return fmt.Sprintf("%s~%s", chain.CustomizationHash, clusterName)
}
