// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package controllers

import (
	"github.com/hashicorp/consul/internal/auth/internal/controllers/trafficpermissions"
	"github.com/hashicorp/consul/internal/auth/internal/controllers/trafficpermissions/expander"
	"github.com/hashicorp/consul/internal/controller"
)

type Dependencies struct {
	TrafficPermissionsMapper trafficpermissions.TrafficPermissionsMapper
}

func Register(mgr *controller.Manager, deps Dependencies) {
	mgr.Register(trafficpermissions.Controller(deps.TrafficPermissionsMapper, expander.GetSamenessGroupExpander()))
}
