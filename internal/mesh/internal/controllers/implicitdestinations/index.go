// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package implicitdestinations

import (
	"golang.org/x/exp/maps"

	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller/cache/index"
	"github.com/hashicorp/consul/internal/controller/cache/indexers"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/storage"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// When an interior (mutable) foreign key pointer on watched data is used to
// determine the resources's applicability in a dependency mapper, it is
// subject to the "orphaned computed resource" problem. When you edit the
// mutable pointer to point elsewhere, the mapper will only witness the NEW
// value and will trigger reconciles for things derived from the NEW pointer,
// but side effects from a prior reconcile using the OLD pointer will be
// orphaned until some other event triggers that reconcile (if ever).
//
// To solve this we need to collect the list of bound references that were
// "ingredients" into a computed resource's output and persist them on the
// newly written resource. Then we load them up and index them such that we can
// use them to AUGMENT a mapper event with additional maps using the OLD data
// as well.
var boundRefsIndex = indexers.BoundRefsIndex[*pbmesh.ComputedImplicitDestinations]("bound-references")

// Cache: reverse SVC[*] => WI[*]
var serviceByWorkloadIdentityIndex = indexers.RefOrIDIndex(
	"service-by-workload-identity",
	func(svc *types.DecodedService) []*pbresource.Reference {
		return getWorkloadIdentitiesFromService(svc)
	},
)

// Cache: reverse CTP => WI[source]
var ctpBySourceWorkloadIdentityIndex = indexers.RefOrIDIndex(
	"ctp-by-source-workload-identity",
	func(ctp *types.DecodedComputedTrafficPermissions) []*pbresource.Reference {
		// We ignore wildcards for this index.
		exact, _, _ := getSourceWorkloadIdentitiesFromCTP(ctp)
		return maps.Values(exact)
	},
)

const ctpByWildcardSourceIndexName = "ctp-by-wildcard-source"

func ctpByWildcardSourceIndexCreator(globalDefaultAllow bool) *index.Index {
	return indexers.DecodedMultiIndexer(
		ctpByWildcardSourceIndexName,
		index.SingleValueFromArgs(func(tn tenantedName) ([]byte, error) {
			return indexFromTenantedName(tn), nil
		}),
		func(r *types.DecodedComputedTrafficPermissions) (bool, [][]byte, error) {
			var vals [][]byte

			if r.Data.IsDefault && globalDefaultAllow {
				// Literally everything can reach it.
				vals = append(vals, indexFromTenantedName(tenantedName{
					Partition: storage.Wildcard,
					Namespace: storage.Wildcard,
					Name:      storage.Wildcard,
				}))
				return true, vals, nil
			}

			for _, perm := range r.Data.AllowPermissions {
				for _, src := range perm.Sources {
					srcType := determineSourceType(src)
					if srcType != sourceTypeLocal {
						// Partition / Peer / SamenessGroup are mututally exclusive.
						continue // Ignore these for now.
					}
					// It is assumed that src.Partition != "" at this point.

					if src.IdentityName != "" {
						// exact
						continue
					} else if src.Namespace != "" {
						// wildcard name
						vals = append(vals, indexFromTenantedName(tenantedName{
							Partition: src.Partition,
							Namespace: src.Namespace,
							Name:      storage.Wildcard,
						}))
					} else {
						// wildcard name+ns
						vals = append(vals, indexFromTenantedName(tenantedName{
							Partition: src.Partition,
							Namespace: storage.Wildcard,
							Name:      storage.Wildcard,
						}))
					}
				}
			}

			return true, vals, nil
		},
	)
}

type tenantedName struct {
	Partition string
	Namespace string
	Name      string
}

func indexFromTenantedName(tn tenantedName) []byte {
	var b index.Builder
	b.String(tn.Partition)
	b.String(tn.Namespace)
	b.String(tn.Name)
	return b.Bytes()
}

// Cache: reverse CR => SVC[backend]
var computedRoutesByBackendServiceIndex = indexers.RefOrIDIndex(
	"computed-routes-by-backend-service",
	func(cr *types.DecodedComputedRoutes) []*pbresource.Reference {
		return getBackendServiceRefsFromComputedRoutes(cr)
	},
)

func getWorkloadIdentitiesFromService(svc *types.DecodedService) []*pbresource.Reference {
	ids := catalog.GetBoundIdentities(svc.Resource)

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

func getSourceWorkloadIdentitiesFromCTP(
	ctp *types.DecodedComputedTrafficPermissions,
) (exact map[resource.ReferenceKey]*pbresource.Reference, wildNames []*pbresource.Tenancy, wildNS []string) {
	var (
		out               = make(map[resource.ReferenceKey]*pbresource.Reference)
		wildNameInNS      = make(map[string]*pbresource.Tenancy)
		wildNSInPartition = make(map[string]struct{})
	)

	for _, perm := range ctp.Data.AllowPermissions {
		for _, src := range perm.Sources {
			srcType := determineSourceType(src)
			if srcType != sourceTypeLocal {
				// Partition / Peer / SamenessGroup are mututally exclusive.
				continue // Ignore these for now.
			}
			// It is assumed that src.Partition != "" at this point.

			if src.IdentityName != "" {
				// exact
				ref := &pbresource.Reference{
					Type: pbauth.WorkloadIdentityType,
					Name: src.IdentityName,
					Tenancy: &pbresource.Tenancy{
						Partition: src.Partition,
						Namespace: src.Namespace,
					},
				}

				rk := resource.NewReferenceKey(ref)
				if _, ok := out[rk]; !ok {
					out[rk] = ref
				}
			} else if src.Namespace != "" {
				// wildcard name
				tenancy := pbauth.SourceToTenancy(src)
				tenancyStr := resource.TenancyToString(tenancy)
				if _, ok := wildNameInNS[tenancyStr]; !ok {
					wildNameInNS[tenancyStr] = tenancy
				}
				continue
			} else {
				// wildcard name+ns
				if _, ok := wildNSInPartition[src.Partition]; !ok {
					wildNSInPartition[src.Partition] = struct{}{}
				}
				continue
			}
		}
	}

	return out, maps.Values(wildNameInNS), maps.Keys(wildNSInPartition)
}

func getBackendServiceRefsFromComputedRoutes(cr *types.DecodedComputedRoutes) []*pbresource.Reference {
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

type sourceType int

const (
	sourceTypeLocal sourceType = iota
	sourceTypePeer
	sourceTypeSamenessGroup
)

// These rules also exist in internal/auth/internal/types during TP validation.
func determineSourceType(src *pbauth.Source) sourceType {
	srcPeer := src.GetPeer()

	switch {
	case srcPeer != "" && srcPeer != "local":
		return sourceTypePeer
	case src.GetSamenessGroup() != "":
		return sourceTypeSamenessGroup
	default:
		return sourceTypeLocal
	}
}
