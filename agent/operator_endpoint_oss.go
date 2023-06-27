// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package agent

import (
	autopilot "github.com/hashicorp/raft-autopilot"

	"github.com/hashicorp/consul/api"
)

func autopilotToAPIServerEnterprise(_ *autopilot.ServerState, _ *api.AutopilotServer) {
	// noop in oss
}

func autopilotToAPIStateEnterprise(state *autopilot.State, apiState *api.AutopilotState) {
	// without the enterprise features there is no different between these two and we don't want to
	// alarm anyone by leaving this as the zero value.
	apiState.OptimisticFailureTolerance = state.FailureTolerance
}
