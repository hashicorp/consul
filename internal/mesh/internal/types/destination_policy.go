// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package types

import (
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	DestinationPolicyKind = "DestinationPolicy"
)

var (
	DestinationPolicyV1Alpha1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV1Alpha1,
		Kind:         DestinationPolicyKind,
	}

	DestinationPolicyType = DestinationPolicyV1Alpha1Type
)

func RegisterDestinationPolicy(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     DestinationPolicyV1Alpha1Type,
		Proto:    &pbmesh.DestinationPolicy{},
		Validate: nil,
		Scope:    resource.ScopeNamespace,
	})
}
