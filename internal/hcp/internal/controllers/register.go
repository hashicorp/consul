// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package controllers

import (
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/hcp/internal/controllers/link"
)

type Dependencies struct {
	ResourceApisEnabled              bool
	OverrideResourceApisEnabledCheck bool
}

func Register(mgr *controller.Manager, deps Dependencies) {
	mgr.Register(link.LinkController(
		deps.ResourceApisEnabled,
		deps.OverrideResourceApisEnabledCheck,
	))
}
