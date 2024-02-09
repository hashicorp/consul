// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package implicitdestinations

import (
	"context"
	"sort"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/controller/cache"
	"github.com/hashicorp/consul/internal/controller/cache/index"
	"github.com/hashicorp/consul/internal/controller/dependency"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/storage"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// Future work: this can be optimized to omit:
//
// - destinations denied due to DENY TP
// - only ports exposed by CR or CTP explicitly

/*
Data Relationships:

Reconcile:
- read WI[source] (ignore)
- list CTPs by WI[source]
  - turn CTP.id -> WI[backend].id
  - list SVCs by WI[backend]
    - list CRs by SVC[backend]
      - turn CR.id -> SVC[dest].id
	  - emit SVC[dest]

DepMappers:
- CR <list> SVC[backend] <list> WI[backend] <align> CTP <list> WI[source] <align> CID
-           SVC[backend] <list> WI[backend] <align> CTP <list> WI[source] <align> CID
-                                                   CTP <list> WI[source] <align> CID
- bound refs for all

*/

func Controller(globalDefaultAllow bool) *controller.Controller {
	m := &mapAndTransformer{globalDefaultAllow: globalDefaultAllow}

	boundRefsMapper := dependency.CacheListMapper(pbmesh.ComputedImplicitDestinationsType, boundRefsIndex.Name())

	return controller.NewController(ControllerID,
		pbmesh.ComputedImplicitDestinationsType,
		boundRefsIndex,
	).
		WithWatch(pbauth.WorkloadIdentityType,
			// BoundRefs: none
			dependency.ReplaceType(pbmesh.ComputedImplicitDestinationsType),
		).
		WithWatch(pbauth.ComputedTrafficPermissionsType,
			// BoundRefs: the WI source refs are interior up-pointers and may change.
			dependency.MultiMapper(boundRefsMapper, m.MapComputedTrafficPermissions),
			ctpBySourceWorkloadIdentityIndex,
			ctpByWildcardSourceIndexCreator(globalDefaultAllow),
		).
		WithWatch(pbcatalog.ServiceType,
			// BoundRefs: the WI slice in the status conds is an interior up-pointer and may change.
			dependency.MultiMapper(boundRefsMapper, m.MapService),
			serviceByWorkloadIdentityIndex,
		).
		WithWatch(pbmesh.ComputedRoutesType,
			// BoundRefs: the backend services are interior up-pointers and may change.
			dependency.MultiMapper(boundRefsMapper, m.MapComputedRoutes),
			computedRoutesByBackendServiceIndex,
		).
		WithReconciler(&reconciler{
			defaultAllow: globalDefaultAllow,
		})
}

type reconciler struct {
	defaultAllow bool
}

func (r *reconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	rt.Logger = rt.Logger.With("resource-id", req.ID, "controller", ControllerID)

	wi := resource.ReplaceType(pbauth.WorkloadIdentityType, req.ID)

	workloadIdentity, err := cache.GetDecoded[*pbauth.WorkloadIdentity](rt.Cache, pbauth.WorkloadIdentityType, "id", wi)
	if err != nil {
		rt.Logger.Error("error retrieving corresponding Workload Identity", "error", err)
		return err
	} else if workloadIdentity == nil {
		rt.Logger.Trace("workload identity has been deleted")
		return nil
	}

	// generate new CID and compare, if different, write new one, if not return without doing anything
	newData, err := r.generateComputedImplicitDestinations(rt, wi)
	if err != nil {
		rt.Logger.Error("error generating computed implicit destinations", "error", err)
		// TODO: update the workload identity with this error as a status condition?
		return err
	}

	oldData, err := resource.GetDecodedResource[*pbmesh.ComputedImplicitDestinations](ctx, rt.Client, req.ID)
	if err != nil {
		rt.Logger.Error("error retrieving computed implicit destinations", "error", err)
		return err
	}
	if oldData != nil && proto.Equal(oldData.Data, newData) {
		rt.Logger.Trace("computed implicit destinations have not changed")
		// there are no changes, and we can return early
		return nil
	}
	rt.Logger.Trace("computed implicit destinations have changed")

	newCID, err := anypb.New(newData)
	if err != nil {
		rt.Logger.Error("error marshalling implicit destination data", "error", err)
		return err
	}
	rt.Logger.Trace("writing computed implicit destinations")

	_, err = rt.Client.Write(ctx, &pbresource.WriteRequest{
		Resource: &pbresource.Resource{
			Id:    req.ID,
			Data:  newCID,
			Owner: workloadIdentity.Resource.Id,
		},
	})
	if err != nil {
		rt.Logger.Error("error writing new computed implicit destinations", "error", err)
		return err
	}
	rt.Logger.Trace("new computed implicit destinations were successfully written")

	return nil
}

// generateComputedImplicitDestinations will use all associated Traffic Permissions to create new ComputedImplicitDestinations data
func (r *reconciler) generateComputedImplicitDestinations(rt controller.Runtime, cid *pbresource.ID) (*pbmesh.ComputedImplicitDestinations, error) {
	wiID := resource.ReplaceType(pbauth.WorkloadIdentityType, cid)

	// Summary: list CTPs by WI[source]
	ctps, err := rt.Cache.List(
		pbauth.ComputedTrafficPermissionsType,
		ctpBySourceWorkloadIdentityIndex.Name(),
		wiID,
	)
	if err != nil {
		return nil, err
	}

	// This covers a foo.bar.* wildcard.
	wildNameCTPs, err := rt.Cache.List(
		pbauth.ComputedTrafficPermissionsType,
		ctpByWildcardSourceIndexName,
		tenantedName{
			Partition: wiID.GetTenancy().GetPartition(),
			Namespace: wiID.GetTenancy().GetNamespace(),
			Name:      storage.Wildcard,
		},
	)
	if err != nil {
		return nil, err
	}
	ctps = append(ctps, wildNameCTPs...)

	// This covers a foo.*.* wildcard.
	wildNamespaceCTPs, err := rt.Cache.List(
		pbauth.ComputedTrafficPermissionsType,
		ctpByWildcardSourceIndexName,
		tenantedName{
			Partition: wiID.GetTenancy().GetPartition(),
			Namespace: storage.Wildcard,
			Name:      storage.Wildcard,
		},
	)
	if err != nil {
		return nil, err
	}
	ctps = append(ctps, wildNamespaceCTPs...)

	// This covers the default-allow + default-CTP option.
	wildPartitionCTPs, err := rt.Cache.List(
		pbauth.ComputedTrafficPermissionsType,
		ctpByWildcardSourceIndexName,
		tenantedName{
			Partition: storage.Wildcard,
			Namespace: storage.Wildcard,
			Name:      storage.Wildcard,
		},
	)
	if err != nil {
		return nil, err
	}
	ctps = append(ctps, wildPartitionCTPs...)

	var (
		out               = &pbmesh.ComputedImplicitDestinations{}
		seenDest          = make(map[resource.ReferenceKey]struct{})
		boundRefCollector = resource.NewBoundReferenceCollector()
	)
	for _, ctp := range ctps {
		// CTP is name aligned with WI[backend].
		backendWorkloadID := resource.ReplaceType(pbauth.WorkloadIdentityType, ctp.Id)

		// Find all services that can reach this WI.
		svcList, err := cache.ListDecoded[*pbcatalog.Service](
			rt.Cache,
			pbcatalog.ServiceType,
			serviceByWorkloadIdentityIndex.Name(),
			backendWorkloadID,
		)
		if err != nil {
			return nil, err
		}

		for _, svc := range svcList {
			// Find all computed routes that have at least one backend target of this service.
			crList, err := rt.Cache.List(
				pbmesh.ComputedRoutesType,
				computedRoutesByBackendServiceIndex.Name(),
				svc.Id,
			)
			if err != nil {
				return nil, err
			}

			// These are name-aligned with the service name that should go
			// directly into the implicit destination list.
			for _, cr := range crList {
				implDestSvcRef := resource.ReplaceType(pbcatalog.ServiceType, cr.Id)

				rk := resource.NewReferenceKey(implDestSvcRef)
				if _, seen := seenDest[rk]; seen {
					continue
				}

				// TODO: populate just the ports allowed by the underlying TPs.
				implDest := &pbmesh.ImplicitDestination{
					DestinationRef: resource.Reference(implDestSvcRef, ""),
				}

				implDestSvc, err := cache.GetDecoded[*pbcatalog.Service](rt.Cache, pbcatalog.ServiceType, "id", implDestSvcRef)
				if err != nil {
					return nil, err
				} else if implDestSvc == nil {
					continue // skip
				}

				inMesh := false
				for _, port := range implDestSvc.Data.Ports {
					if port.Protocol == pbcatalog.Protocol_PROTOCOL_MESH {
						inMesh = true
						continue // skip
					}
					implDest.DestinationPorts = append(implDest.DestinationPorts, port.TargetPort)
				}
				if !inMesh {
					continue // skip
				}

				// Add entire bound-ref lineage at once, since they're only
				// bound if they materially affect the computed resource.
				boundRefCollector.AddRefOrID(ctp.Id)
				boundRefCollector.AddRefOrID(svc.Id)
				boundRefCollector.AddRefOrID(cr.Id)
				boundRefCollector.AddRefOrID(implDestSvcRef)

				sort.Strings(implDest.DestinationPorts)

				out.Destinations = append(out.Destinations, implDest)
				seenDest[rk] = struct{}{}
			}
		}
	}

	// Ensure determinstic sort so we don't get into infinite-reconcile
	sort.Slice(out.Destinations, func(i, j int) bool {
		a, b := out.Destinations[i], out.Destinations[j]
		return resource.LessReference(a.DestinationRef, b.DestinationRef)
	})

	out.BoundReferences = boundRefCollector.List()

	return out, nil
}

func listAllWorkloadIdentities(
	cache cache.ReadOnlyCache,
	tenancy *pbresource.Tenancy,
) ([]*pbresource.Reference, error) {
	// This is the same logic used by the sidecar controller to interpret CTPs. Here we
	// carry it to its logical conclusion and simply include all possible identities.
	iter, err := cache.ListIterator(pbauth.WorkloadIdentityType, "id", &pbresource.Reference{
		Type:    pbauth.WorkloadIdentityType,
		Tenancy: tenancy,
	}, index.IndexQueryOptions{Prefix: true})
	if err != nil {
		return nil, err
	}

	var out []*pbresource.Reference
	for res := iter.Next(); res != nil; res = iter.Next() {
		out = append(out, resource.Reference(res.Id, ""))
	}
	return out, nil
}
