// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package routes

import (
	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/routes/loader"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
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

		conditions := computeNewRouteRefConditions(related, res, parentRefs, backendRefs)

		pending.AddConditions(rk, res, conditions)
	})
}

func computeNewRouteRefConditions(
	related *loader.RelatedResources,
	routeRes *pbresource.Resource,
	parentRefs []*pbmesh.ParentReference,
	backendRefs []*pbmesh.BackendReference,
) []*pbresource.Condition {
	var conditions []*pbresource.Condition

	// TODO: handle port numbers here too? the virtual port

	for _, parentRef := range parentRefs {
		if parentRef.Ref == nil || !resource.EqualType(parentRef.Ref.Type, catalog.ServiceType) {
			continue
		}
		if parentRef.Ref.Section != "" {
			continue
		}
		if svc := related.GetService(parentRef.Ref); svc != nil {
			allowedPorts, hasMesh := getAllowedPorts(svc)
			if hasMesh {
				if parentRef.Port != "" {
					if _, found := allowedPorts[parentRef.Port]; !found {
						conditions = append(conditions, ConditionUnknownParentRefPort(parentRef.Ref, parentRef.Port))
					}
				}
			} else {
				conditions = append(conditions, ConditionParentRefOutsideMesh(parentRef.Ref))
			}
		} else {
			conditions = append(conditions, ConditionMissingParentRef(parentRef.Ref))
		}
	}

	for _, backendRef := range backendRefs {
		if backendRef.Ref == nil || !resource.EqualType(backendRef.Ref.Type, catalog.ServiceType) {
			continue
		}
		if backendRef.Ref.Section != "" {
			continue
		}
		if svc := related.GetService(backendRef.Ref); svc != nil {
			allowedPorts, hasMesh := getAllowedPorts(svc)
			if hasMesh {
				if backendRef.Port != "" {
					if _, found := allowedPorts[backendRef.Port]; !found {
						conditions = append(conditions, ConditionUnknownBackendRefPort(backendRef.Ref, backendRef.Port))
					}
				}
			} else {
				conditions = append(conditions, ConditionBackendRefOutsideMesh(backendRef.Ref))
			}
		} else {
			conditions = append(conditions, ConditionMissingBackendRef(backendRef.Ref))
		}
	}

	return conditions
}

func getAllowedPorts(svc *types.DecodedService) (map[string]pbcatalog.Protocol, bool) {
	allowedPortProtocols := make(map[string]pbcatalog.Protocol)
	hasMesh := false
	for _, port := range svc.Data.Ports {
		if port.Protocol == pbcatalog.Protocol_PROTOCOL_MESH {
			hasMesh = true
			continue // skip
		}
		allowedPortProtocols[port.TargetPort] = port.Protocol
	}
	return allowedPortProtocols, hasMesh
}
