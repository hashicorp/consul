package consul

import (
	"context"
	"fmt"

	"github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"
	"github.com/hashicorp/raft"
	autopilot "github.com/hashicorp/raft-autopilot"
	"github.com/hashicorp/serf/serf"
	"math"

	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/types"
)

var AutopilotGauges = []prometheus.GaugeDefinition{
	{
		Name: []string{"autopilot", "failure_tolerance"},
		Help: "Tracks the number of voting servers that the cluster can lose while continuing to function.",
	},
	{
		Name: []string{"autopilot", "healthy"},
		Help: "Tracks the overall health of the local server cluster. 1 if all servers are healthy, 0 if one or more are unhealthy.",
	},
}

// AutopilotDelegate is a Consul delegate for autopilot operations.
type AutopilotDelegate struct {
	server *Server
}

func (d *AutopilotDelegate) AutopilotConfig() *autopilot.Config {
	return d.server.getOrCreateAutopilotConfig().ToAutopilotLibraryConfig()
}

func (d *AutopilotDelegate) KnownServers() map[raft.ServerID]*autopilot.Server {
	return d.server.autopilotServers()
}

func (d *AutopilotDelegate) FetchServerStats(ctx context.Context, servers map[raft.ServerID]*autopilot.Server) map[raft.ServerID]*autopilot.ServerStats {
	return d.server.statsFetcher.Fetch(ctx, servers)
}

func (d *AutopilotDelegate) NotifyState(state *autopilot.State) {
	// emit metrics if we are the leader regarding overall healthiness and the failure tolerance
	if d.server.raft.State() == raft.Leader {
		metrics.SetGauge([]string{"autopilot", "failure_tolerance"}, float32(state.FailureTolerance))
		if state.Healthy {
			metrics.SetGauge([]string{"autopilot", "healthy"}, 1)
		} else {
			metrics.SetGauge([]string{"autopilot", "healthy"}, 0)
		}
	} else {

		// if we are not a leader, emit NaN per
		// https://www.consul.io/docs/agent/telemetry#autopilot
		metrics.SetGauge([]string{"autopilot", "healthy"}, float32(math.NaN()))

		// also emit NaN for failure tolerance to be backwards compatible
		metrics.SetGauge([]string{"autopilot", "failure_tolerance"}, float32(math.NaN()))

	}
}

func (d *AutopilotDelegate) RemoveFailedServer(srv *autopilot.Server) {
	go func() {
		if err := d.server.RemoveFailedNode(srv.Name, false); err != nil {
			d.server.logger.Error("failed to remove server", "name", srv.Name, "id", srv.ID, "error", err)
		}
	}()
}

func (s *Server) initAutopilot(config *Config) {
	apDelegate := &AutopilotDelegate{s}

	s.autopilot = autopilot.New(
		s.raft,
		apDelegate,
		autopilot.WithLogger(s.logger),
		autopilot.WithReconcileInterval(config.AutopilotInterval),
		autopilot.WithUpdateInterval(config.ServerHealthInterval),
		autopilot.WithPromoter(s.autopilotPromoter()),
	)

	metrics.SetGauge([]string{"autopilot", "healthy"}, float32(math.NaN()))
	metrics.SetGauge([]string{"autopilot", "failure_tolerance"}, float32(math.NaN()))
}

func (s *Server) autopilotServers() map[raft.ServerID]*autopilot.Server {
	servers := make(map[raft.ServerID]*autopilot.Server)
	for _, member := range s.serfLAN.Members() {
		srv, err := s.autopilotServer(member)
		if err != nil {
			s.logger.Warn("Error parsing server info", "name", member.Name, "error", err)
			continue
		} else if srv == nil {
			// this member was a client
			continue
		}

		servers[srv.ID] = srv
	}

	return servers
}

func (s *Server) autopilotServer(m serf.Member) (*autopilot.Server, error) {
	ok, srv := metadata.IsConsulServer(m)
	if !ok {
		return nil, nil
	}

	return s.autopilotServerFromMetadata(srv)
}

func (s *Server) autopilotServerFromMetadata(srv *metadata.Server) (*autopilot.Server, error) {
	server := &autopilot.Server{
		Name:        srv.ShortName,
		ID:          raft.ServerID(srv.ID),
		Address:     raft.ServerAddress(srv.Addr.String()),
		Version:     srv.Build.String(),
		RaftVersion: srv.RaftVersion,
		Ext:         s.autopilotServerExt(srv),
	}

	switch srv.Status {
	case serf.StatusLeft:
		server.NodeStatus = autopilot.NodeLeft
	case serf.StatusAlive, serf.StatusLeaving:
		// we want to treat leaving as alive to prevent autopilot from
		// prematurely removing the node.
		server.NodeStatus = autopilot.NodeAlive
	case serf.StatusFailed:
		server.NodeStatus = autopilot.NodeFailed
	default:
		server.NodeStatus = autopilot.NodeUnknown
	}

	// populate the node meta if there is any. When a node first joins or if
	// there are ACL issues then this could be empty if the server has not
	// yet been able to register itself in the catalog
	_, node, err := s.fsm.State().GetNodeID(types.NodeID(srv.ID), structs.NodeEnterpriseMetaInDefaultPartition())
	if err != nil {
		return nil, fmt.Errorf("error retrieving node from state store: %w", err)
	}

	if node != nil {
		server.Meta = node.Meta
	}

	return server, nil
}
