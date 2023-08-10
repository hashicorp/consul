package sidecarproxymapper

import (
	"context"

	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/cache/sidecarproxycache"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type Mapper struct {
	cache *sidecarproxycache.Cache
}

func New(c *sidecarproxycache.Cache) *Mapper {
	return &Mapper{
		cache: c,
	}
}

// MapServiceEndpointsToProxyStateTemplate maps catalog.ServiceEndpoints objects to the IDs of
// ProxyStateTemplate.
func (m *Mapper) MapServiceEndpointsToProxyStateTemplate(_ context.Context, _ controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	// This mapper needs to look up workload IDs from service endpoints and replace them with ProxyStateTemplate type.
	var serviceEndpoints pbcatalog.ServiceEndpoints
	err := res.Data.UnmarshalTo(&serviceEndpoints)
	if err != nil {
		return nil, err
	}

	var result []controller.Request

	// First, we need to generate requests from workloads this "endpoints" points to
	// so that we can re-generate proxy state for the sidecar proxy.
	for _, endpoint := range serviceEndpoints.Endpoints {
		// Convert the reference to a workload to a ProxyStateTemplate ID.
		// Because these resources are name and tenancy aligned, we only need to change the type.

		// Skip service endpoints without target refs. These resources would typically be created for
		// services external to Consul, and we don't need to reconcile those as they don't have
		// associated workloads.
		if endpoint.TargetRef != nil {
			id := &pbresource.ID{
				Name:    endpoint.TargetRef.Name,
				Tenancy: endpoint.TargetRef.Tenancy,
				Type:    types.ProxyStateTemplateType,
			}
			result = append(result, controller.Request{
				ID: id,
			})
		}
	}

	// Look up any source proxies for this service and generate updates.
	serviceID := resource.ReplaceType(catalog.ServiceType, res.Id)

	// Second, we need to generate requests for any proxies where this service is a destination.
	if len(serviceEndpoints.Endpoints) > 0 {
		// All port names in the endpoints object should be the same as we filter out to ports that are selected
		// by the service, and so it's sufficient to check just the first endpoint.
		for portName, port := range serviceEndpoints.Endpoints[0].Ports {
			if port.Protocol == pbcatalog.Protocol_PROTOCOL_MESH {
				// Skip mesh ports. These should never be used as destination ports.
				continue
			}
			serviceRef := resource.Reference(serviceID, "")
			if destination, ok := m.cache.ReadDestination(serviceRef, portName); ok {
				for refKey := range destination.SourceProxies {
					result = append(result, controller.Request{ID: refKey.ToID()})
				}
			}
		}
	}

	return result, err
}
