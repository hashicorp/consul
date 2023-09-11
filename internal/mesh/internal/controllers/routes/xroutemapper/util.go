// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xroutemapper

import (
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func parentRefSliceToRefSlice(parentRefs []*pbmesh.ParentReference) []resource.ReferenceOrID {
	if parentRefs == nil {
		return nil
	}
	parents := make([]resource.ReferenceOrID, 0, len(parentRefs))
	for _, parentRef := range parentRefs {
		if parentRef.Ref != nil && types.IsServiceType(parentRef.Ref.Type) {
			parents = append(parents, parentRef.Ref)
		}
	}
	return parents
}

func backendRefSliceToRefSlice(backendRefs []*pbmesh.BackendReference) []resource.ReferenceOrID {
	if backendRefs == nil {
		return nil
	}
	backends := make([]resource.ReferenceOrID, 0, len(backendRefs))
	for _, backendRef := range backendRefs {
		if backendRef.Ref != nil && types.IsServiceType(backendRef.Ref.Type) {
			backends = append(backends, backendRef.Ref)
		}
	}
	return backends
}

func sliceReplaceType(list []*pbresource.ID, typ *pbresource.Type) []*pbresource.ID {
	if list == nil {
		return nil
	}
	out := make([]*pbresource.ID, 0, len(list))
	for _, id := range list {
		out = append(out, resource.ReplaceType(typ, id))
	}
	return out
}
