// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package controller

import (
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// MakeRequests accepts a list of pbresource.ID and pbresource.Reference items,
// and mirrors them into a slice of []controller.Request items where the Type
// of the items has replaced by 'typ'.
func MakeRequests[V resource.ReferenceOrID](
	typ *pbresource.Type,
	refs []V,
) []Request {
	if len(refs) == 0 {
		return nil
	}

	out := make([]Request, 0, len(refs))
	for _, ref := range refs {
		// if type is not provided, we will use the type in the ID or reference.
		if typ == nil {
			typ = ref.GetType()
		}
		out = append(out, Request{
			ID: &pbresource.ID{
				Type:    typ,
				Tenancy: ref.GetTenancy(),
				Name:    ref.GetName(),
			},
		})
	}

	return out
}

// MakeRequestsFromResources accepts a list of items and converts them into a
// slice of []controller.Request items where the Type of the items has replaced
// by 'typ'.
func MakeRequestsFromResources[V interface {
	GetId() *pbresource.ID
}](
	typ *pbresource.Type,
	resources []V,
) []Request {
	if len(resources) == 0 {
		return nil
	}

	out := make([]Request, 0, len(resources))
	for _, res := range resources {
		id := res.GetId()

		// if type is not provided, we will use the type in the ID or reference.
		if typ == nil {
			typ = id.GetType()
		}
		out = append(out, Request{
			ID: &pbresource.ID{
				Type:    typ,
				Tenancy: id.GetTenancy(),
				Name:    id.GetName(),
			},
		})
	}

	return out
}
