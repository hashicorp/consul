package controllers

import (
	"github.com/hashicorp/consul/internal/auth/internal/controllers/trafficpermissions"
	"github.com/hashicorp/consul/internal/controller"
)

type Dependencies struct {
	ComputedTrafficPermissionsMapper trafficpermissions.Mapper
}

func Register(mgr *controller.Manager, deps Dependencies) {
	mgr.Register(trafficpermissions.Controller(deps.ComputedTrafficPermissionsMapper))
}
