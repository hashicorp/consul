// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package tenancy

import (
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/tenancy/internal/types"
)

var (
	// API Group Information

	APIGroup        = types.GroupName
	VersionV1Alpha1 = types.VersionV1Alpha1
	CurrentVersion  = types.CurrentVersion

	// Resource Kind Names.

	NamespaceKind         = types.NamespaceKind
	NamespaceV1Alpha1Type = types.NamespaceV1Alpha1Type
)

// RegisterTypes adds all resource types within the "tenancy" API group
// to the given type registry
func RegisterTypes(r resource.Registry) {
	types.Register(r)
}
