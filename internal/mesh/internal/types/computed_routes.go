// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package types

import (
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	ComputedRoutesKind = "ComputedRoutes"
)

var (
	ComputedRoutesV1Alpha1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV1Alpha1,
		Kind:         ComputedRoutesKind,
	}

	ComputedRoutesType = ComputedRoutesV1Alpha1Type
)

func RegisterComputedRoutes(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     ComputedRoutesV1Alpha1Type,
		Proto:    &pbmesh.ComputedRoutes{},
		Validate: nil,
		Scope:    resource.ScopeNamespace,
	})
}
