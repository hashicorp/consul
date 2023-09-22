// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	ProxyConfigurationKind = "ProxyConfiguration"
)

var (
	ProxyConfigurationV2Beta1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV2beta1,
		Kind:         ProxyConfigurationKind,
	}

	ProxyConfigurationType = ProxyConfigurationV2Beta1Type
)

func RegisterProxyConfiguration(r resource.Registry) {
	r.Register(resource.Registration{
		Type:  ProxyConfigurationV2Beta1Type,
		Proto: &pbmesh.ProxyConfiguration{},
		Scope: resource.ScopeNamespace,
		// TODO(rb): add validation for proxy configuration
		Validate: nil,
	})
}
