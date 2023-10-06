// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package intermediate

import (
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// CombinedDestinationRef contains all references we need for a specific
// destination on the mesh.
type CombinedDestinationRef struct {
	// ServiceRef is the reference to the destination service.
	ServiceRef *pbresource.Reference

	// Port is the port name for this destination.
	Port string

	// SourceProxies are the reference keys of source proxy state template resources.
	SourceProxies map[resource.ReferenceKey]struct{}

	// ExplicitDestinationsID is the id of the pbmesh.Destinations resource. For implicit destinations,
	// this should be nil.
	ExplicitDestinationsID *pbresource.ID
}

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
