// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"fmt"

	"github.com/hashicorp/consul/agent/structs"
)

func CustomizeClusterName(clusterName string, chain *structs.CompiledDiscoveryChain) string {
	if chain == nil || chain.CustomizationHash == "" {
		return clusterName
	}
	// Use a colon to delimit this prefix instead of a dot to avoid a
	// theoretical collision problem with subsets.
	return fmt.Sprintf("%s~%s", chain.CustomizationHash, clusterName)
}
