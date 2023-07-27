// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package mesh

import (
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
)

var (
	// API Group Information

	APIGroup        = types.GroupName
	VersionV1Alpha1 = types.VersionV1Alpha1
	CurrentVersion  = types.CurrentVersion

	// Resource Kind Names.

	ProxyConfigurationKind = types.ProxyConfigurationKind
	UpstreamsKind          = types.UpstreamsKind

	// Resource Types for the v1alpha1 version.

	ProxyConfigurationV1Alpha1Type     = types.ProxyConfigurationV1Alpha1Type
	UpstreamsV1Alpha1Type              = types.UpstreamsV1Alpha1Type
	UpstreamsConfigurationV1Alpha1Type = types.UpstreamsConfigurationV1Alpha1Type
)

// RegisterTypes adds all resource types within the "catalog" API group
// to the given type registry
func RegisterTypes(r resource.Registry) {
	types.Register(r)
}
