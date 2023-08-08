package mapper

import (
	"context"

	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecarproxy/cache"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/mesh/internal/types/intermediate"
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func (m *Mapper) MapDestinationsToProxyStateTemplate(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	var destinations pbmesh.Upstreams
	err := res.Data.UnmarshalTo(&destinations)
	if err != nil {
		return nil, err
	}

	// Look up workloads for this destinations.
	sourceProxyIDs := make(map[string]*pbresource.ID)
	var result []controller.Request
	for _, prefix := range destinations.Workloads.Prefixes {
		resp, err := rt.Client.List(ctx, &pbresource.ListRequest{
			Type:       catalog.WorkloadType,
			Tenancy:    res.Id.Tenancy,
			NamePrefix: prefix,
		})
		if err != nil {
			return nil, err
		}
		for _, r := range resp.Resources {
			proxyID := resource.ReplaceType(types.ProxyStateTemplateType, r.Id)
			sourceProxyIDs[cache.KeyFromID(proxyID)] = proxyID
			result = append(result, controller.Request{
				ID: proxyID,
			})
		}
	}

	for _, name := range destinations.Workloads.Names {
		id := &pbresource.ID{
			Name:    name,
			Tenancy: res.Id.Tenancy,
			Type:    catalog.WorkloadType,
		}
		proxyID := resource.ReplaceType(types.ProxyStateTemplateType, id)
		sourceProxyIDs[cache.KeyFromID(proxyID)] = proxyID
		result = append(result, controller.Request{
			ID: proxyID,
		})
	}

	// Add this destination to cache.
	for _, destination := range destinations.Upstreams {
		destinationRef := &intermediate.CombinedDestinationRef{
			ServiceRef:             destination.DestinationRef,
			Port:                   destination.DestinationPort,
			ExplicitDestinationsID: res.Id,
			SourceProxies:          sourceProxyIDs,
		}
		m.cache.Write(destinationRef)
	}

	return result, nil
}
