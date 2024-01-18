// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package controllers

import (
	hcpclient "github.com/hashicorp/consul/agent/hcp/client"
	"github.com/hashicorp/consul/agent/hcp/config"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/hcp/internal/controllers/link"
	"github.com/hashicorp/consul/internal/hcp/internal/controllers/telemetrystate"
)

type Dependencies struct {
	CloudConfig            config.CloudConfig
	ResourceApisEnabled    bool
	HCPAllowV2ResourceApis bool
	HCPClient              hcpclient.Client
}

func Register(mgr *controller.Manager, deps Dependencies) {
	mgr.Register(link.LinkController(
		deps.ResourceApisEnabled,
		deps.HCPAllowV2ResourceApis,
		link.DefaultHCPClientFn,
		deps.CloudConfig,
	))

	mgr.Register(telemetrystate.TelemetryStateController(link.DefaultHCPClientFn))
}
