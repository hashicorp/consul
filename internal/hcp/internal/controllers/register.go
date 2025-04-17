// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package controllers

import (
	"github.com/hashicorp/consul/agent/hcp/config"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/hcp/internal/controllers/link"
	"github.com/hashicorp/consul/internal/hcp/internal/controllers/telemetrystate"
)

type Dependencies struct {
	CloudConfig config.CloudConfig
}

func Register(mgr *controller.Manager, deps Dependencies) {
	mgr.Register(
		link.LinkController(
			link.DefaultHCPClientFn,
			deps.CloudConfig,
		),
	)

	mgr.Register(telemetrystate.TelemetryStateController(link.DefaultHCPClientFn))
}
