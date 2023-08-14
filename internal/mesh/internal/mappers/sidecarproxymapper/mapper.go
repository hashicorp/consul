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
	destinationsCache *sidecarproxycache.DestinationsCache
	proxyCfgCache     *sidecarproxycache.ProxyConfigurationCache
}

func New(destinationsCache *sidecarproxycache.DestinationsCache, proxyCfgCache *sidecarproxycache.ProxyConfigurationCache) *Mapper {
	return &Mapper{
		destinationsCache: destinationsCache,
		proxyCfgCache:     proxyCfgCache,
	}
}

// mapWorkloadsBySelector returns ProxyStateTemplate requests given a workload
// selector and tenancy. The cacheFunc can be called if the resulting ids need to be cached.
func mapWorkloadsBySelector(ctx context.Context,
	client pbresource.ResourceServiceClient,
	selector *pbcatalog.WorkloadSelector,
	tenancy *pbresource.Tenancy,
	cacheFunc func(id *pbresource.ID)) ([]controller.Request, error) {
	var result []controller.Request

	for _, prefix := range selector.Prefixes {
		resp, err := client.List(ctx, &pbresource.ListRequest{
			Type:       catalog.WorkloadType,
			Tenancy:    tenancy,
			NamePrefix: prefix,
		})
		if err != nil {
			return nil, err
		}
		for _, r := range resp.Resources {
			id := resource.ReplaceType(types.ProxyStateTemplateType, r.Id)
			result = append(result, controller.Request{
				ID: id,
			})
			cacheFunc(id)
		}
	}

	for _, name := range selector.Names {
		id := &pbresource.ID{
			Name:    name,
			Tenancy: tenancy,
			Type:    types.ProxyStateTemplateType,
		}
		result = append(result, controller.Request{
			ID: id,
		})
		cacheFunc(id)
	}

	return result, nil
}
