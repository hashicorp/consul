// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package auth

import (
	"github.com/hashicorp/consul/internal/auth/internal/types"
	"github.com/hashicorp/consul/internal/resource"
)

// RegisterTypes adds all resource types within the "catalog" API group
// to the given type registry
func RegisterTypes(r resource.Registry) {
	types.Register(r)
}
