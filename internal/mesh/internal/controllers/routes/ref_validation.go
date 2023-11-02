// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package routes

import (
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"

	"github.com/hashicorp/consul/internal/mesh/internal/controllers/routes/loader"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// ValidateXRouteReferences examines all of the ParentRefs and BackendRefs of
// xRoutes provided and issues status conditions if anything is unacceptable.
func ValidateXRouteReferences(related *loader.RelatedResources, pending PendingStatuses) {
	related.WalkRoutes(func(
		rk resource.ReferenceKey,
		res *pbresource.Resource,
		route types.XRouteData,
	) {
		parentRefs := route.GetParentRefs()
		backendRefs := route.GetUnderlyingBackendRefs()

		conditions := computeNewRouteRefConditions(related, parentRefs, backendRefs)

		pending.AddConditions(rk, res, conditions)
	})
}

type serviceGetter interface {
	GetService(ref resource.ReferenceOrID) *types.DecodedService
}

func computeNewRouteRefConditions(
	related serviceGetter,
	parentRefs []*pbmesh.ParentReference,
	backendRefs []*pbmesh.BackendReference,
) []*pbresource.Condition {
	var conditions []*pbresource.Condition

	// TODO(rb): handle port numbers here too if we are allowing those instead of the name?

	for _, parentRef := range parentRefs {
		if parentRef.Ref == nil || !resource.EqualType(parentRef.Ref.Type, pbcatalog.ServiceType) {
			continue // not possible due to xRoute validation
		}
		if parentRef.Ref.Section != "" {
			continue // not possible due to xRoute validation
		}
		if svc := related.GetService(parentRef.Ref); svc != nil {
			found := false
			usingMesh := false
			hasMesh := false
			for _, port := range svc.Data.Ports {
				if port.Protocol == pbcatalog.Protocol_PROTOCOL_MESH {
					hasMesh = true
				}
				if port.TargetPort == parentRef.Port {
					found = true
					if port.Protocol == pbcatalog.Protocol_PROTOCOL_MESH {
						usingMesh = true
					}
				}
			}
			switch {
			case !hasMesh:
				conditions = append(conditions, ConditionParentRefOutsideMesh(parentRef.Ref))
			case !found:
				if parentRef.Port != "" {
					conditions = append(conditions, ConditionUnknownParentRefPort(parentRef.Ref, parentRef.Port))
				}
			case usingMesh:
				conditions = append(conditions, ConditionParentRefUsingMeshPort(parentRef.Ref, parentRef.Port))
			}
		} else {
			conditions = append(conditions, ConditionMissingParentRef(parentRef.Ref))
		}
	}

	for _, backendRef := range backendRefs {
		if backendRef.Ref == nil || !resource.EqualType(backendRef.Ref.Type, pbcatalog.ServiceType) {
			continue // not possible due to xRoute validation
		}
		if backendRef.Ref.Section != "" {
			continue // not possible due to xRoute validation
		}
		if svc := related.GetService(backendRef.Ref); svc != nil {
			found := false
			usingMesh := false
			hasMesh := false
			for _, port := range svc.Data.Ports {
				if port.Protocol == pbcatalog.Protocol_PROTOCOL_MESH {
					hasMesh = true
				}
				if port.TargetPort == backendRef.Port {
					found = true
					if port.Protocol == pbcatalog.Protocol_PROTOCOL_MESH {
						usingMesh = true
					}
				}
			}
			switch {
			case !hasMesh:
				conditions = append(conditions, ConditionBackendRefOutsideMesh(backendRef.Ref))
			case !found:
				if backendRef.Port != "" {
					conditions = append(conditions, ConditionUnknownBackendRefPort(backendRef.Ref, backendRef.Port))
				}
			case usingMesh:
				conditions = append(conditions, ConditionBackendRefUsingMeshPort(backendRef.Ref, backendRef.Port))
			}
		} else {
			conditions = append(conditions, ConditionMissingBackendRef(backendRef.Ref))
		}
	}

	return conditions
}
