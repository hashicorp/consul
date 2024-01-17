// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package controllers

import (
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/multicluster/internal/controllers/exportedservices"
)

type Dependencies struct {
	ExportedServicesSamenessGroupsExpander exportedservices.ExportedServicesSamenessGroupExpander
}

func Register(mgr *controller.Manager, deps Dependencies) {
	mgr.Register(exportedservices.Controller(deps.ExportedServicesSamenessGroupsExpander))
}
