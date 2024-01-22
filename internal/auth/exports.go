// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package auth

import (
	"github.com/hashicorp/consul/internal/auth/internal/controllers"
	"github.com/hashicorp/consul/internal/auth/internal/controllers/trafficpermissions"
	"github.com/hashicorp/consul/internal/auth/internal/types"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
)

var (
	// Controller statuses

	StatusKey                                  = trafficpermissions.ControllerID
	TrafficPermissionsConditionComputed        = trafficpermissions.ConditionComputed
	TrafficPermissionsConditionFailedToCompute = trafficpermissions.ConditionFailedToCompute
)

// RegisterTypes adds all resource types within the "catalog" API group
// to the given type registry
func RegisterTypes(r resource.Registry) {
	types.Register(r)
}

// RegisterControllers registers controllers for the auth types with
// the given controller manager.
func RegisterControllers(mgr *controller.Manager) {
	controllers.Register(mgr)
}
