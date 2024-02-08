// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package meshindexes

import (
	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller/cache/index"
	"github.com/hashicorp/consul/internal/controller/cache/indexers"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// Cache: reverse CR => SVC[backend]
func ComputedRoutesByBackendServiceIndex() *index.Index {
	return indexers.RefOrIDIndex(
		"computed-routes-by-backend-service",
		func(cr *types.DecodedComputedRoutes) []*pbresource.Reference {
			return GetBackendServiceRefsFromComputedRoutes(cr)
		},
	)
}

func GetBackendServiceRefsFromComputedRoutes(cr *types.DecodedComputedRoutes) []*pbresource.Reference {
	var (
		out  []*pbresource.Reference
		seen = make(map[resource.ReferenceKey]struct{})
	)
	for _, pc := range cr.Data.PortedConfigs {
		for _, target := range pc.Targets {
			ref := target.BackendRef.Ref
			rk := resource.NewReferenceKey(ref)
			if _, ok := seen[rk]; !ok {
				out = append(out, ref)
				seen[rk] = struct{}{}
			}
		}
	}
	return out
}

// Cache: reverse SVC[*] => WI[*]
func ServiceByWorkloadIdentityIndex() *index.Index {
	return indexers.RefOrIDIndex(
		"service-by-workload-identity",
		func(svc *types.DecodedService) []*pbresource.Reference {
			return GetWorkloadIdentitiesFromService(svc.Resource)
		},
	)
}

func GetWorkloadIdentitiesFromService(svc *pbresource.Resource) []*pbresource.Reference {
	ids := catalog.GetBoundIdentities(svc)

	out := make([]*pbresource.Reference, 0, len(ids))
	for _, id := range ids {
		out = append(out, &pbresource.Reference{
			Type:    pbauth.WorkloadIdentityType,
			Name:    id,
			Tenancy: svc.Id.Tenancy,
		})
	}
	return out
}
