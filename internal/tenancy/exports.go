// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package tenancy

import (
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/tenancy/internal/bridge"
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

type (
	V2TenancyBridge = bridge.V2TenancyBridge
)

// RegisterTypes adds all resource types within the "tenancy" API group
// to the given type registry
func RegisterTypes(r resource.Registry) {
	types.Register(r)
}

func NewV2TenancyBridge() *V2TenancyBridge {
	return bridge.NewV2TenancyBridge()
}
