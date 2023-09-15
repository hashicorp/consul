// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"fmt"
	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/structs"
)

func groupedEndpoints(logger hclog.Logger, locality *structs.Locality, policy *structs.DiscoveryPrioritizeByLocality, csns structs.CheckServiceNodes) ([]structs.CheckServiceNodes, error) {
	switch {
	case policy == nil || policy.Mode == "" || policy.Mode == "none":
		return []structs.CheckServiceNodes{csns}, nil
	case policy.Mode == "failover":
		log := logger.Named("locality")
		return prioritizeByLocalityFailover(log, locality, csns), nil
	default:
		return nil, fmt.Errorf("unexpected priortize-by-locality mode %q", policy.Mode)
	}
}
