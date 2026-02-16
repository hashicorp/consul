// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"fmt"

	"github.com/hashicorp/consul/acl"
)

// GlobalRateLimitConfigEntry defines a global rate limit that applies across
// all Consul servers in the cluster. This configuration is stored in Raft and
// automatically replicated to all servers.
type GlobalRateLimitConfigEntry struct {
	// Kind must be "global-rate-limit"
	Kind string

	// Name identifies this rate limit configuration
	Name string

	// GlobalRateLimit contains the rate limiting configuration
	Config *GlobalRateLimitConfig `json:"config,omitempty" alias:"config"`

	Meta map[string]string `json:",omitempty"`
	Hash uint64            `json:",omitempty" hash:"ignore"`

	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	RaftIndex          `bexpr:"-" hash:"ignore"`
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

func (e *GlobalRateLimitConfigEntry) GetKind() string {
	return RateLimit
}

func (e *GlobalRateLimitConfigEntry) GetName() string {
	if e == nil {
		return ""
	}
	return e.Name
}

func (e *GlobalRateLimitConfigEntry) GetMeta() map[string]string {
	if e == nil {
		return nil
	}
	return e.Meta
}

func (e *GlobalRateLimitConfigEntry) GetEnterpriseMeta() *acl.EnterpriseMeta {
	if e == nil {
		return nil
	}
	return &e.EnterpriseMeta
}

func (e *GlobalRateLimitConfigEntry) GetRaftIndex() *RaftIndex {
	if e == nil {
		return nil
	}
	return &e.RaftIndex
}

func (e *GlobalRateLimitConfigEntry) GetHash() uint64 {
	if e == nil {
		return 0
	}
	return e.Hash
}

func (e *GlobalRateLimitConfigEntry) SetHash(h uint64) {
	if e != nil {
		e.Hash = h
	}
}

func (e *GlobalRateLimitConfigEntry) Normalize() error {
	if e == nil {
		return fmt.Errorf("config entry is nil")
	}

	e.Kind = RateLimit
	e.EnterpriseMeta.Normalize()

	h, err := HashConfigEntry(e)
	if err != nil {
		return err
	}
	e.Hash = h
	return nil
}

func (e *GlobalRateLimitConfigEntry) Validate() error {
	if e == nil {
		return fmt.Errorf("config entry is nil")
	}

	if e.Name == "" {
		return fmt.Errorf("Name is required for global-rate-limit config entry")
	}

	if e.Name != "global" {
		return fmt.Errorf("Name for global-rate-limit config entry must be 'global'")
	}

	// Config is required and must be valid
	if e.Config == nil {
		return fmt.Errorf("Config is required for global-rate-limit config entry")
	}

	// Validate read and write rates
	if e.Config.ReadRate != nil && *e.Config.ReadRate < 0 {
		return fmt.Errorf("read_rate must be non-negative, got %f", *e.Config.ReadRate)
	}
	if e.Config.WriteRate != nil && *e.Config.WriteRate < 0 {
		return fmt.Errorf("write_rate must be non-negative, got %f", *e.Config.WriteRate)
	}

	// Validate exclude endpoints - check for empty strings
	for i, endpoint := range e.Config.ExcludeEndpoints {
		if endpoint == "" {
			return fmt.Errorf("exclude_endpoints[%d] cannot be empty", i)
		}
	}

	if err := validateConfigEntryMeta(e.Meta); err != nil {
		return fmt.Errorf("invalid meta: %w", err)
	}

	return nil
}

func (e *GlobalRateLimitConfigEntry) CanRead(authz acl.Authorizer) error {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.ToAllowAuthorizer().OperatorReadAllowed(&authzContext)
}

func (e *GlobalRateLimitConfigEntry) CanWrite(authz acl.Authorizer) error {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.ToAllowAuthorizer().OperatorWriteAllowed(&authzContext)
}
