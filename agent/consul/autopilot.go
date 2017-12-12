package consul

import (
	"context"
	"fmt"

	"github.com/hashicorp/consul/agent/consul/autopilot"
	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
)

// AutopilotDelegate is a Consul delegate for autopilot operations.
type AutopilotDelegate struct {
	server *Server
}

func (d *AutopilotDelegate) FetchStats(ctx context.Context, servers []*metadata.Server) map[string]*autopilot.ServerStats {
	return d.server.statsFetcher.Fetch(ctx, servers)
}

func (d *AutopilotDelegate) GetOrCreateAutopilotConfig() (*autopilot.Config, bool) {
	return d.server.getOrCreateAutopilotConfig()
}

func (d *AutopilotDelegate) Raft() *raft.Raft {
	return d.server.raft
}

func (d *AutopilotDelegate) Serf() *serf.Serf {
	return d.server.serfLAN
}

func (d *AutopilotDelegate) NumPeers() (int, error) {
	return d.server.numPeers()
}

func (d *AutopilotDelegate) PromoteNonVoters(conf *autopilot.Config, health autopilot.OperatorHealthReply) ([]raft.Server, error) {
	future := d.server.raft.GetConfiguration()
	if err := future.Error(); err != nil {
		return nil, fmt.Errorf("failed to get raft configuration: %v", err)
	}

	return autopilot.PromoteStableServers(conf, health, future.Configuration().Servers), nil
}
