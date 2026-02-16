// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

// GlobalRateLimitConfigEntry defines a global rate limit that applies across
// all Consul servers in the cluster.
type GlobalRateLimitConfigEntry struct {
	// Kind must be "rate-limit"
	Kind string

	// Name must be "global" for the single global rate limit config entry
	Name string

	// GlobalRateLimit contains the rate limiting configuration
	Config GlobalRateLimitConfig `json:"config" alias:"config"`

	// Partition is the partition the config entry is associated with.
	// Partitioning is a Consul Enterprise feature.
	Partition string `json:",omitempty"`

	// Namespace is the namespace the config entry is associated with.
	// Namespacing is a Consul Enterprise feature.
	Namespace string `json:",omitempty"`

	// Meta is a map of arbitrary key-value pairs
	Meta map[string]string `json:",omitempty"`

	// CreateIndex is the Raft index this entry was created at. This is a
	// read-only field.
	CreateIndex uint64

	// ModifyIndex is used for the Check-And-Set operations and can also be fed
	// back into the WaitIndex of the QueryOptions in order to perform blocking
	// queries.
	ModifyIndex uint64
}

// GlobalRateLimitConfig contains the actual rate limiting parameters
type GlobalRateLimitConfig struct {
	// RequestLimits allows separate configuration for read and write operations.
	// This follows the same pattern as limits.request_limits in agent configuration.
	// If set, takes precedence over the legacy MaxRPS field.
	// If nil, defaults to infinity (unlimited).
	ReadRate  *float64 `alias:"readRate"`
	WriteRate *float64 `alias:"writeRate"`

	// EmergencyMode enables stricter rate limiting in emergency situations
	Priority bool `json:"priority" alias:"priority"`

	// PriorityEndpoints lists RPC methods that should bypass rate limiting
	// Example: ["Health.Check", "Status.Leader"]
	ExcludeEndpoints []string `json:"exclude_endpoints" alias:"exclude_endpoints"`
}

func (g *GlobalRateLimitConfigEntry) GetKind() string {
	return RateLimit
}

func (g *GlobalRateLimitConfigEntry) GetName() string {
	if g == nil {
		return ""
	}
	return g.Name
}

func (g *GlobalRateLimitConfigEntry) GetPartition() string {
	if g == nil {
		return ""
	}
	return g.Partition
}

func (g *GlobalRateLimitConfigEntry) GetNamespace() string {
	if g == nil {
		return ""
	}
	return g.Namespace
}

func (g *GlobalRateLimitConfigEntry) GetMeta() map[string]string {
	if g == nil {
		return nil
	}
	return g.Meta
}

func (g *GlobalRateLimitConfigEntry) GetCreateIndex() uint64 {
	if g == nil {
		return 0
	}
	return g.CreateIndex
}

func (g *GlobalRateLimitConfigEntry) GetModifyIndex() uint64 {
	if g == nil {
		return 0
	}
	return g.ModifyIndex
}
