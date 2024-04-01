// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package tenancy

import (
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/tenancy/internal/bridge"
	"github.com/hashicorp/consul/internal/tenancy/internal/controllers"
	"github.com/hashicorp/consul/internal/tenancy/internal/types"
)

type (
	V2TenancyBridge = bridge.V2TenancyBridge
)

// RegisterTypes adds all resource types within the "tenancy" API group
// to the given type registry
func RegisterTypes(r resource.Registry) {
	types.Register(r)
}

// RegisterControllers registers controllers for the tenancy types with
// the given controller manager.
func RegisterControllers(mgr *controller.Manager, deps Dependencies) {
	controllers.Register(mgr, deps)
}

func NewV2TenancyBridge() *V2TenancyBridge {
	return bridge.NewV2TenancyBridge()
}

type Dependencies = controllers.Dependencies
