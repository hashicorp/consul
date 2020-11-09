// +build !consulent

package agent

import (
	"github.com/hashicorp/consul/api"
	autopilot "github.com/hashicorp/raft-autopilot"
)

func autopilotToAPIServerEnterprise(_ *autopilot.ServerState, _ *api.AutopilotServer) {
	// noop in oss
}

func autopilotToAPIStateEnterprise(_ *autopilot.State, _ *api.AutopilotState) {
	// noop in oss
}
