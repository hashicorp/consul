// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package controllers

import (
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/hcp/internal/controllers/hcclink"
)

type Dependencies struct {
	ResourceApisEnabled              bool
	OverrideResourceApisEnabledCheck bool
}

func Register(mgr *controller.Manager, deps Dependencies) {
	mgr.Register(hcclink.HCCLinkController(
		deps.ResourceApisEnabled,
		deps.OverrideResourceApisEnabledCheck,
	))
}
