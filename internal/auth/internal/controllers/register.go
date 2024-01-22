// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package controllers

import (
	"github.com/hashicorp/consul/internal/auth/internal/controllers/trafficpermissions"
	"github.com/hashicorp/consul/internal/controller"
)

func Register(mgr *controller.Manager) {
	mgr.Register(trafficpermissions.Controller())
}
