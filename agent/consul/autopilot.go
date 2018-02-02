package consul

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/agent/consul/autopilot"
	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
)

// AutopilotDelegate is a Consul delegate for autopilot operations.
type AutopilotDelegate struct {
	server *Server
}

func (d *AutopilotDelegate) AutopilotConfig() *autopilot.Config {
	return d.server.getOrCreateAutopilotConfig()
}

func (d *AutopilotDelegate) FetchStats(ctx context.Context, servers []serf.Member) map[string]*autopilot.ServerStats {
	return d.server.statsFetcher.Fetch(ctx, servers)
}

func (d *AutopilotDelegate) IsServer(m serf.Member) (*autopilot.ServerInfo, error) {
	if m.Tags["role"] != "consul" {
		return nil, nil
	}

	portStr := m.Tags["port"]
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, err
	}

	buildVersion, err := metadata.Build(&m)
	if err != nil {
		return nil, err
	}

	server := &autopilot.ServerInfo{
		Name:   m.Name,
		ID:     m.Tags["id"],
		Addr:   &net.TCPAddr{IP: m.Addr, Port: port},
		Build:  *buildVersion,
		Status: m.Status,
	}
	return server, nil
}

// Heartbeat a metric for monitoring if we're the leader
func (d *AutopilotDelegate) NotifyHealth(health autopilot.OperatorHealthReply) {
	if d.server.raft.State() == raft.Leader {
		metrics.SetGauge([]string{"consul", "autopilot", "failure_tolerance"}, float32(health.FailureTolerance))
		metrics.SetGauge([]string{"autopilot", "failure_tolerance"}, float32(health.FailureTolerance))
		if health.Healthy {
			metrics.SetGauge([]string{"consul", "autopilot", "healthy"}, 1)
			metrics.SetGauge([]string{"autopilot", "healthy"}, 1)
		} else {
			metrics.SetGauge([]string{"consul", "autopilot", "healthy"}, 0)
			metrics.SetGauge([]string{"autopilot", "healthy"}, 0)
		}
	}
}

func (d *AutopilotDelegate) PromoteNonVoters(conf *autopilot.Config, health autopilot.OperatorHealthReply) ([]raft.Server, error) {
	future := d.server.raft.GetConfiguration()
	if err := future.Error(); err != nil {
		return nil, fmt.Errorf("failed to get raft configuration: %v", err)
	}

	return autopilot.PromoteStableServers(conf, health, future.Configuration().Servers), nil
}

func (d *AutopilotDelegate) Raft() *raft.Raft {
	return d.server.raft
}

func (d *AutopilotDelegate) Serf() *serf.Serf {
	return d.server.serfLAN
}
