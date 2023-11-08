// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
)

func RegisterComputedProxyConfiguration(r resource.Registry) {
	r.Register(resource.Registration{
		Type:  pbmesh.ComputedProxyConfigurationType,
		Proto: &pbmesh.ComputedProxyConfiguration{},
		Scope: resource.ScopeNamespace,
	})
}
