package autopilot

import (
	"time"

	"github.com/hashicorp/raft"
)

// PromoteStableServers is a basic autopilot promotion policy that promotes any
// server which has been healthy and stable for the duration specified in the
// given Autopilot config.
func PromoteStableServers(autopilotConfig *Config, health OperatorHealthReply, servers []raft.Server) []raft.Server {
	// Find any non-voters eligible for promotion.
	now := time.Now()
	var promotions []raft.Server
	for _, server := range servers {
		if !IsPotentialVoter(server.Suffrage) {
			health := health.ServerHealth(string(server.ID))
			if health.IsStable(now, autopilotConfig) {
				promotions = append(promotions, server)
			}
		}
	}

	return promotions
}
