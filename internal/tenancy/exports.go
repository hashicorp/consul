// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package tenancy

import (
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/tenancy/internal/types"
)

var (
	// API Group Information

	APIGroup       = types.GroupName
	VersionV2Beta1 = types.VersionV2Beta1
	CurrentVersion = types.CurrentVersion

	// Resource Kind Names.

	NamespaceKind        = types.NamespaceKind
	NamespaceV2Beta1Type = types.NamespaceV2Beta1Type
)

// RegisterTypes adds all resource types within the "tenancy" API group
// to the given type registry
func RegisterTypes(r resource.Registry) {
	types.Register(r)
}
