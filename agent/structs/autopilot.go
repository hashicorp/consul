package structs

import (
	"time"

	autopilot "github.com/hashicorp/raft-autopilot"
	"github.com/hashicorp/serf/serf"
)

// Autopilotconfig holds the Autopilot configuration for a cluster.
type AutopilotConfig struct {
	// CleanupDeadServers controls whether to remove dead servers when a new
	// server is added to the Raft peers.
	CleanupDeadServers bool

	// LastContactThreshold is the limit on the amount of time a server can go
	// without leader contact before being considered unhealthy.
	LastContactThreshold time.Duration

	// MaxTrailingLogs is the amount of entries in the Raft Log that a server can
	// be behind before being considered unhealthy.
	MaxTrailingLogs uint64

	// MinQuorum sets the minimum number of servers required in a cluster
	// before autopilot can prune dead servers.
	MinQuorum uint

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

func (c *AutopilotConfig) ToAutopilotLibraryConfig() *autopilot.Config {
	if c == nil {
		return nil
	}
	return &autopilot.Config{
		CleanupDeadServers:      c.CleanupDeadServers,
		LastContactThreshold:    c.LastContactThreshold,
		MaxTrailingLogs:         c.MaxTrailingLogs,
		MinQuorum:               c.MinQuorum,
		ServerStabilizationTime: c.ServerStabilizationTime,
		Ext:                     c.autopilotConfigExt(),
	}
}

// AutopilotHealthReply is a representation of the overall health of the cluster
type AutopilotHealthReply struct {
	// Healthy is true if all the servers in the cluster are healthy.
	Healthy bool

	// FailureTolerance is the number of healthy servers that could be lost without
	// an outage occurring.
	FailureTolerance int

	// Servers holds the health of each server.
	Servers []AutopilotServerHealth
}

// ServerHealth is the health (from the leader's point of view) of a server.
type AutopilotServerHealth struct {
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

// RaftStats holds miscellaneous Raft metrics for a server.
type RaftStats struct {
	// LastContact is the time since this node's last contact with the leader.
	LastContact string

	// LastTerm is the highest leader term this server has a record of in its Raft log.
	LastTerm uint64

	// LastIndex is the last log index this server has a record of in its Raft log.
	LastIndex uint64
}

func (s *RaftStats) ToAutopilotServerStats() *autopilot.ServerStats {
	duration, _ := time.ParseDuration(s.LastContact)
	return &autopilot.ServerStats{
		LastContact: duration,
		LastTerm:    s.LastTerm,
		LastIndex:   s.LastIndex,
	}
}
