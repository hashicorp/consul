// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package intermediate

import (
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
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

	// ExplicitDestinationsID is the id of the pbmesh.Upstreams resource. For implicit destinations,
	// this should be nil.
	ExplicitDestinationsID *pbresource.ID
}

// Deprecated: types.DecodedServiceEndpoints
type ServiceEndpoints struct {
	Resource  *pbresource.Resource
	Endpoints *pbcatalog.ServiceEndpoints
}

// Deprecated: types.DecodedService
type Service struct {
	Resource *pbresource.Resource
	Service  *pbcatalog.Service
}

// Deprecated: types.DecodedDestinations
type Destinations struct {
	Resource     *pbresource.Resource
	Destinations *pbmesh.Upstreams
}

// Deprecated: types.DecodedWorkload
type Workload struct {
	Resource *pbresource.Resource
	Workload *pbcatalog.Workload
}

// Deprecated: types.DecodedProxyStateTemplate
type ProxyStateTemplate struct {
	Resource *pbresource.Resource
	Tmpl     *pbmesh.ProxyStateTemplate
}

// Deprecated: types.DecodedProxyConfiguration
type ProxyConfiguration struct {
	Resource *pbresource.Resource
	Cfg      *pbmesh.ProxyConfiguration
}

type Destination struct {
	Explicit           *pbmesh.Upstream
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
