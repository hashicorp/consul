// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package sidecarproxymapper

import (
	"context"
	"fmt"

	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/cache/sidecarproxycache"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type Mapper struct {
	destinationsCache   *sidecarproxycache.DestinationsCache
	proxyCfgCache       *sidecarproxycache.ProxyConfigurationCache
	computedRoutesCache *sidecarproxycache.ComputedRoutesCache
}

func New(
	destinationsCache *sidecarproxycache.DestinationsCache,
	proxyCfgCache *sidecarproxycache.ProxyConfigurationCache,
	computedRoutesCache *sidecarproxycache.ComputedRoutesCache,
) *Mapper {
	return &Mapper{
		destinationsCache:   destinationsCache,
		proxyCfgCache:       proxyCfgCache,
		computedRoutesCache: computedRoutesCache,
	}
}

// mapSelectorToProxyStateTemplates returns ProxyStateTemplate requests given a workload
// selector and tenancy. The cacheFunc can be called if the resulting ids need to be cached.
func mapSelectorToProxyStateTemplates(ctx context.Context,
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
		if len(resp.Resources) == 0 {
			return nil, fmt.Errorf("no workloads found")
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
