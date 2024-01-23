// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package controllers

import (
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/multicluster/internal/controllers/exportedservices"
	"github.com/hashicorp/consul/internal/multicluster/internal/controllers/v1compat"
)

type Dependencies struct {
	ExportedServicesSamenessGroupsExpander exportedservices.ExportedServicesSamenessGroupExpander
	ConfigEntryExports                     v1compat.AggregatedConfig
}

func Register(mgr *controller.Manager, deps Dependencies) {
	mgr.Register(exportedservices.Controller(deps.ExportedServicesSamenessGroupsExpander))
	mgr.Register(v1compat.Controller(deps.ConfigEntryExports))
}
