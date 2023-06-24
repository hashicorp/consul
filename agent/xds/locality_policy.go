// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package xds

import (
	"fmt"

	"github.com/hashicorp/consul/agent/structs"
)

func groupedEndpoints(locality *structs.Locality, policy *structs.DiscoveryPrioritizeByLocality, csns structs.CheckServiceNodes) ([]structs.CheckServiceNodes, error) {
	switch {
	case policy == nil || policy.Mode == "" || policy.Mode == "none":
		return []structs.CheckServiceNodes{csns}, nil
	case policy.Mode == "failover":
		return prioritizeByLocalityFailover(locality, csns), nil
	default:
		return nil, fmt.Errorf("unexpected priortize-by-locality mode %q", policy.Mode)
	}
}
