package autopilot

import (
	"time"

	"github.com/hashicorp/serf/serf"
)

// Config holds the Autopilot configuration for a cluster.
type Config struct {
	// CleanupDeadServers controls whether to remove dead servers when a new
	// server is added to the Raft peers.
	CleanupDeadServers bool

	// LastContactThreshold is the limit on the amount of time a server can go
	// without leader contact before being considered unhealthy.
	LastContactThreshold time.Duration

	// MaxTrailingLogs is the amount of entries in the Raft Log that a server can
	// be behind before being considered unhealthy.
	MaxTrailingLogs uint64

	// ServerStabilizationTime is the minimum amount of time a server must be
	// in a stable, healthy state before it can be added to the cluster. Only
	// applicable with Raft protocol version 3 or higher.
	ServerStabilizationTime time.Duration

	// (Enterprise-only) RedundancyZoneTag is the node tag to use for separating
	// servers into zones for redundancy. If left blank, this feature will be disabled.
	RedundancyZoneTag string

	// (Enterprise-only) DisableUpgradeMigration will disable Autopilot's upgrade migration
	// strategy of waiting until enough newer-versioned servers have been added to the
	// cluster before promoting them to voters.
	DisableUpgradeMigration bool

	// (Enterprise-only) UpgradeVersionTag is the node tag to use for version info when
	// performing upgrade migrations. If left blank, the Consul version will be used.
	UpgradeVersionTag string

	// CreateIndex/ModifyIndex store the create/modify indexes of this configuration.
	CreateIndex uint64
	ModifyIndex uint64
}

// ServerHealth is the health (from the leader's point of view) of a server.
type ServerHealth struct {
	// ID is the raft ID of the server.
	ID string

	// Name is the node name of the server.
	Name string

	// Address is the address of the server.
	Address string

	// The status of the SerfHealth check for the server.
	SerfStatus serf.MemberStatus

	// Version is the version of the server.
	Version string

	// Leader is whether this server is currently the leader.
	Leader bool

	// LastContact is the time since this node's last contact with the leader.
	LastContact time.Duration

	// LastTerm is the highest leader term this server has a record of in its Raft log.
	LastTerm uint64

	// LastIndex is the last log index this server has a record of in its Raft log.
	LastIndex uint64

	// Healthy is whether or not the server is healthy according to the current
	// Autopilot config.
	Healthy bool

	// Voter is whether this is a voting server.
	Voter bool

	// StableSince is the last time this server's Healthy value changed.
	StableSince time.Time
}

// IsHealthy determines whether this ServerHealth is considered healthy
// based on the given Autopilot config
func (h *ServerHealth) IsHealthy(lastTerm uint64, leaderLastIndex uint64, autopilotConf *Config) bool {
	if h.SerfStatus != serf.StatusAlive {
		return false
	}

	if h.LastContact > autopilotConf.LastContactThreshold || h.LastContact < 0 {
		return false
	}

	if h.LastTerm != lastTerm {
		return false
	}

	if leaderLastIndex > autopilotConf.MaxTrailingLogs && h.LastIndex < leaderLastIndex-autopilotConf.MaxTrailingLogs {
		return false
	}

	return true
}

// IsStable returns true if the ServerHealth shows a stable, passing state
// according to the given AutopilotConfig
func (h *ServerHealth) IsStable(now time.Time, conf *Config) bool {
	if h == nil {
		return false
	}

	if !h.Healthy {
		return false
	}

	if now.Sub(h.StableSince) < conf.ServerStabilizationTime {
		return false
	}

	return true
}

// ServerStats holds miscellaneous Raft metrics for a server
type ServerStats struct {
	// LastContact is the time since this node's last contact with the leader.
	LastContact string

	// LastTerm is the highest leader term this server has a record of in its Raft log.
	LastTerm uint64

	// LastIndex is the last log index this server has a record of in its Raft log.
	LastIndex uint64
}

// OperatorHealthReply is a representation of the overall health of the cluster
type OperatorHealthReply struct {
	// Healthy is true if all the servers in the cluster are healthy.
	Healthy bool

	// FailureTolerance is the number of healthy servers that could be lost without
	// an outage occurring.
	FailureTolerance int

	// Servers holds the health of each server.
	Servers []ServerHealth
}

func (o *OperatorHealthReply) ServerHealth(id string) *ServerHealth {
	for _, health := range o.Servers {
		if health.ID == id {
			return &health
		}
	}
	return nil
}
