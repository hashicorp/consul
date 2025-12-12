// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

package controller

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
)

func TestMakeRequests(t *testing.T) {
	redType := &pbresource.Type{
		Group:        "colors",
		GroupVersion: "vfake",
		Kind:         "red",
	}
	blueType := &pbresource.Type{
		Group:        "colors",
		GroupVersion: "vfake",
		Kind:         "blue",
	}

	casparID := &pbresource.ID{
		Type:    redType,
		Tenancy: resource.DefaultNamespacedTenancy(),
		Name:    "caspar",
		Uid:     "ignored",
	}
	babypantsID := &pbresource.ID{
		Type:    redType,
		Tenancy: resource.DefaultNamespacedTenancy(),
		Name:    "babypants",
		Uid:     "ignored",
	}
	zimRef := &pbresource.Reference{
		Type:    redType,
		Tenancy: resource.DefaultNamespacedTenancy(),
		Name:    "zim",
		Section: "ignored",
	}
	girRef := &pbresource.Reference{
		Type:    redType,
		Tenancy: resource.DefaultNamespacedTenancy(),
		Name:    "gir",
		Section: "ignored",
	}

	newBlueReq := func(name string) Request {
		return Request{
			ID: &pbresource.ID{
				Type:    blueType,
				Tenancy: resource.DefaultNamespacedTenancy(),
				Name:    name,
			},
		}
	}

	require.Nil(t, MakeRequests[*pbresource.ID](blueType, nil))
	require.Nil(t, MakeRequests[*pbresource.Reference](blueType, nil))

	prototest.AssertElementsMatch(t, []Request{
		newBlueReq("caspar"), newBlueReq("babypants"),
	}, MakeRequests[*pbresource.ID](blueType, []*pbresource.ID{
		casparID, babypantsID,
	}))

	prototest.AssertElementsMatch(t, []Request{
		newBlueReq("gir"), newBlueReq("zim"),
	}, MakeRequests[*pbresource.Reference](blueType, []*pbresource.Reference{
		girRef, zimRef,
	}))
}
