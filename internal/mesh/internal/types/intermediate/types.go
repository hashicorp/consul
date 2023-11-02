// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package intermediate

import (
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type Destination struct {
	Explicit           *pbmesh.Destination
	Service            *types.DecodedService      // for the name of this destination
	VirtualIPs         []string                   // for the name of this destination
	ComputedPortRoutes *pbmesh.ComputedPortRoutes // for the name of this destination
}

type Status struct {
	ID         *pbresource.ID
	Generation string
	Conditions []*pbresource.Condition
	OldStatus  map[string]*pbresource.Status
}
