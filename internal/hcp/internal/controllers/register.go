// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package controllers

import (
	"github.com/hashicorp/consul/agent/hcp/config"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/hcp/internal/controllers/link"
)

type Dependencies struct {
	CloudConfig            config.CloudConfig
	ResourceApisEnabled    bool
	HCPAllowV2ResourceApis bool
	DataDir                string
}

func Register(mgr *controller.Manager, deps Dependencies) {
	mgr.Register(link.LinkController(
		deps.ResourceApisEnabled,
		deps.HCPAllowV2ResourceApis,
		link.DefaultHCPClientFn,
		deps.CloudConfig,
		deps.DataDir,
	))
}
