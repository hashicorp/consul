// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
)

type DecodedComputedExplicitDestinations = resource.DecodedResource[*pbmesh.ComputedExplicitDestinations]

func RegisterComputedExplicitDestinations(r resource.Registry) {
	r.Register(resource.Registration{
		Type:  pbmesh.ComputedExplicitDestinationsType,
		Proto: &pbmesh.ComputedExplicitDestinations{},
		Scope: resource.ScopeNamespace,
	})
}
