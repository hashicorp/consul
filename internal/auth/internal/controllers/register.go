// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package controllers

import (
	"github.com/hashicorp/consul/internal/auth/internal/controllers/trafficpermissions"
	"github.com/hashicorp/consul/internal/controller"
)

type Dependencies struct {
	WorkloadIdentityMapper trafficpermissions.TrafficPermissionsMapper
}

func Register(mgr *controller.Manager, deps Dependencies) {
	mgr.Register(trafficpermissions.Controller(deps.WorkloadIdentityMapper))
}
