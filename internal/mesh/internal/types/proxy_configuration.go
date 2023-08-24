// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	ProxyConfigurationKind = "ProxyConfiguration"
)

var (
	ProxyConfigurationV1Alpha1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV1Alpha1,
		Kind:         ProxyConfigurationKind,
	}

	ProxyConfigurationType = ProxyConfigurationV1Alpha1Type
)

func RegisterProxyConfiguration(r resource.Registry) {
	r.Register(resource.Registration{
		Type:  ProxyConfigurationV1Alpha1Type,
		Proto: &pbmesh.ProxyConfiguration{},
		// TODO(rb): add validation for proxy configuration
		Validate: nil,
		Scope:    resource.ScopeNamespace,
	})
}
