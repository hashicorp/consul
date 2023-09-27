// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
)

func RegisterComputedDestinations(r resource.Registry) {
	r.Register(resource.Registration{
		Type:  pbmesh.ComputedDestinationsType,
		Proto: &pbmesh.ComputedDestinations{},
		Scope: resource.ScopeNamespace,
	})
}
