// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package multicluster

import (
	"github.com/hashicorp/consul/internal/multicluster/internal/types"
	"github.com/hashicorp/consul/internal/resource"
)

var (
	// APIGroup Information

	APIGroup        = types.GroupName
	VersionV1Alpha1 = types.VersionV1Alpha1
	CurrentVersion  = types.CurrentVersion
)

// RegisterTypes adds all resource types within the "multicluster" API group
// to the given type registry
func RegisterTypes(r resource.Registry) {
	types.Register(r)
}
