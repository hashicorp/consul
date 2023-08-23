//go:build !consulent
// +build !consulent

package agent

import (
	"github.com/hashicorp/consul/api"
	autopilot "github.com/hashicorp/raft-autopilot"
)

func autopilotToAPIServerEnterprise(_ *autopilot.ServerState, _ *api.AutopilotServer) {
	// noop in oss
}

func autopilotToAPIStateEnterprise(state *autopilot.State, apiState *api.AutopilotState) {
	// without the enterprise features there is no different between these two and we don't want to
	// alarm anyone by leaving this as the zero value.
	apiState.OptimisticFailureTolerance = state.FailureTolerance
}
