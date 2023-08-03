package intermediate

import (
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// todo should it be destination?
// the problem is that it's compiled from different source objects
type CombinedDestinationRef struct {
	// ServiceRef is the reference to the destination service for this upstream
	ServiceRef *pbresource.Reference

	Port string

	// sourceProxies are the IDs of source proxy state template resources.
	SourceProxies map[string]*pbresource.ID

	// explicitUpstreamID is the id of an explicit upstreams resource. For implicit upstreams,
	// this should be nil.
	ExplicitDestinationsID *pbresource.ID
}

type ServiceEndpoints struct {
	Resource  *pbresource.Resource
	Endpoints *pbcatalog.ServiceEndpoints
}

type Destinations struct {
	Resource     *pbresource.Resource
	Destinations *pbmesh.Upstreams
}

type Workload struct {
	Resource *pbresource.Resource
	Workload *pbcatalog.Workload
}

type ProxyStateTemplate struct {
	Resource *pbresource.Resource
	Tmpl     *pbmesh.ProxyStateTemplate
}

type ProxyConfiguration struct {
	Resource *pbresource.Resource
	Cfg      *pbmesh.ProxyConfiguration
}

type Destination struct {
	Explicit         *pbmesh.Upstream
	ServiceEndpoints *ServiceEndpoints
	Identities       []*pbresource.Reference
}

type Status struct {
	ID         *pbresource.ID
	Generation string
	Conditions []*pbresource.Condition
	OldStatus  map[string]*pbresource.Status
}
