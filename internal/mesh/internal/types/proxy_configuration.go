// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	meshapi "github.com/hashicorp/consul/api/mesh/v2beta1"
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
)

func RegisterProxyConfiguration(r resource.Registry) {
	r.Register(resource.Registration{
		Type:  meshapi.ProxyConfigurationV2Beta1Type,
		Proto: &pbmesh.ProxyConfiguration{},
		Scope: resource.ScopeNamespace,
		// TODO(rb): add validation for proxy configuration
		Validate: nil,
	})
}
