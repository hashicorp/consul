// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package controllers

import (
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/multicluster/internal/controllers/exportedservices"
	"github.com/hashicorp/consul/internal/multicluster/internal/controllers/v1compat"
)

type Dependencies struct {
	ConfigEntryExports v1compat.ConfigEntry
}

func Register(mgr *controller.Manager, deps Dependencies) {
	mgr.Register(exportedservices.Controller())
	mgr.Register(v1compat.Controller(deps.ConfigEntryExports))
}
