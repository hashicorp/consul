// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package rate implements server-side RPC rate limiting.
package rate

import (
	"context"
	"errors"
	"fmt"
	"net"
	"reflect"
	"sync/atomic"

	"github.com/hashicorp/consul/agent/metadata"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"
	"golang.org/x/time/rate"

	"github.com/hashicorp/consul/agent/consul/multilimiter"
	"github.com/hashicorp/consul/agent/structs"
)

var (
	// ErrRetryElsewhere indicates that the operation was not allowed because the
	// rate limit was exhausted, but may succeed on a different server.
	//
	// Results in a RESOURCE_EXHAUSTED or "429 Too Many Requests" response.
	ErrRetryElsewhere = errors.New("rate limit exceeded, try again later or against a different server")

	// ErrRetryLater indicates that the operation was not allowed because the rate
	// limit was exhausted, and trying a different server won't help (e.g. because
	// the operation can only be performed on the leader).
	//
	// Results in an UNAVAILABLE or "503 Service Unavailable" response.
	ErrRetryLater = errors.New("rate limit exceeded for operation that can only be performed by the leader, try again later")
)

// Mode determines the action that will be taken when a rate limit has been
// exhausted (e.g. log and allow, or reject).
type Mode int

const (
	// ModeDisabled causes rate limiting to be bypassed.
	ModeDisabled Mode = iota

	// ModePermissive causes the handler to log the rate-limited operation but
	// still allow it to proceed.
	ModePermissive

	// ModeEnforcing causes the handler to reject the rate-limited operation.
	ModeEnforcing
)

var modeToName = map[Mode]string{
	ModeDisabled:   "disabled",
	ModeEnforcing:  "enforcing",
	ModePermissive: "permissive",
}
var ModeFromName = func() map[string]Mode {
	vals := map[string]Mode{
		"": ModeDisabled,
	}
	for k, v := range modeToName {
		vals[v] = k
	}
	return vals
}()

func (m Mode) String() string {
	return modeToName[m]
}

// RequestLimitsModeFromName will unmarshal the string form of a configMode.
func RequestLimitsModeFromName(name string) (Mode, bool) {
	s, ok := ModeFromName[name]
	return s, ok
}

// RequestLimitsModeFromNameWithDefault will unmarshal the string form of a configMode.
func RequestLimitsModeFromNameWithDefault(name string) Mode {
	s, ok := ModeFromName[name]
	if !ok {
		return ModePermissive
	}
	return s
}

// OperationType is the type of operation the client is attempting to perform.
type OperationType int

type OperationCategory string

type OperationSpec struct {
	Type     OperationType
	Category OperationCategory
}

const (
	// OperationTypeRead represents a read operation.
	OperationTypeRead OperationType = iota

	// OperationTypeWrite represents a write operation.
	OperationTypeWrite

	// OperationTypeExempt represents an operation that is exempt from rate-limiting.
	OperationTypeExempt
)

const (
	OperationCategoryACL             OperationCategory = "ACL"
	OperationCategoryCatalog         OperationCategory = "Catalog"
	OperationCategoryConfigEntry     OperationCategory = "ConfigEntry"
	OperationCategoryConnectCA       OperationCategory = "ConnectCA"
	OperationCategoryCoordinate      OperationCategory = "Coordinate"
	OperationCategoryDiscoveryChain  OperationCategory = "DiscoveryChain"
	OperationCategoryServerDiscovery OperationCategory = "ServerDiscovery"
	OperationCategoryHealth          OperationCategory = "Health"
	OperationCategoryIntention       OperationCategory = "Intention"
	OperationCategoryKV              OperationCategory = "KV"
	OperationCategoryPreparedQuery   OperationCategory = "PreparedQuery"
	OperationCategorySession         OperationCategory = "Session"
	OperationCategoryStatus          OperationCategory = "Status" // not limited
	OperationCategoryTxn             OperationCategory = "Txn"
	OperationCategoryAutoConfig      OperationCategory = "AutoConfig"
	OperationCategoryFederationState OperationCategory = "FederationState"
	OperationCategoryInternal        OperationCategory = "Internal"
	OperationCategoryOperator        OperationCategory = "Operator" // not limited
	OperationCategoryPeerStream      OperationCategory = "PeerStream"
	OperationCategoryPeering         OperationCategory = "Peering"
	OperationCategoryPartition       OperationCategory = "Tenancy"
	OperationCategoryDataPlane       OperationCategory = "DataPlane"
	OperationCategoryDNS             OperationCategory = "DNS"
	OperationCategorySubscribe       OperationCategory = "Subscribe"
	OperationCategoryResource        OperationCategory = "Resource"
)

// Operation the client is attempting to perform.
type Operation struct {
	// Name of the RPC endpoint (e.g. "Foo.Bar" for net/rpc and "/foo.service/Bar" for gRPC).
	Name string

	// SourceAddr is the client's (or forwarding server's) IP address.
	SourceAddr net.Addr

	// Type of operation to be performed (e.g. read or write).
	Type OperationType

	Category OperationCategory
}

//go:generate mockery --name RequestLimitsHandler --inpackage
type RequestLimitsHandler interface {
	Run(ctx context.Context)
	Allow(op Operation) error
	UpdateConfig(cfg HandlerConfig)
	UpdateIPConfig(cfg IPLimitConfig)
	Register(serversStatusProvider ServersStatusProvider)
}

// Handler enforces rate limits for incoming RPCs.
type Handler struct {
	globalCfg             *atomic.Pointer[HandlerConfig]
	ipCfg                 *atomic.Pointer[IPLimitConfig]
	globalRateLimitCfg    *atomic.Pointer[structs.GlobalRateLimitConfigEntry]
	serversStatusProvider ServersStatusProvider

	limiter multilimiter.RateLimiter

	logger hclog.Logger
}

type ReadWriteConfig struct {
	// WriteConfig configures the global rate limiter for write operations.
	WriteConfig multilimiter.LimiterConfig

	// ReadConfig configures the global rate limiter for read operations.
	ReadConfig multilimiter.LimiterConfig
}

type GlobalLimitConfig struct {
	Mode Mode
	ReadWriteConfig
}

type HandlerConfig struct {
	multilimiter.Config

	GlobalLimitConfig GlobalLimitConfig
}

//go:generate mockery --name ServersStatusProvider --inpackage --filename mock_ServersStatusProvider_test.go
type ServersStatusProvider interface {
	// IsLeader is used to determine whether the operation is being performed
	// against the cluster leader, such that if it can _only_ be performed by
	// the leader (e.g. write operations) we don't tell clients to retry against
	// a different server.
	IsLeader() bool
	IsServer(addr string) bool
}

func isInfRate(cfg multilimiter.LimiterConfig) bool {
	return cfg.Rate == rate.Inf
}

func NewHandlerWithLimiter(
	cfg HandlerConfig,
	limiter multilimiter.RateLimiter,
	logger hclog.Logger) *Handler {

	limiter.UpdateConfig(cfg.GlobalLimitConfig.WriteConfig, globalWrite)

	limiter.UpdateConfig(cfg.GlobalLimitConfig.ReadConfig, globalRead)

	h := &Handler{
		ipCfg:              new(atomic.Pointer[IPLimitConfig]),
		globalCfg:          new(atomic.Pointer[HandlerConfig]),
		globalRateLimitCfg: new(atomic.Pointer[structs.GlobalRateLimitConfigEntry]),
		limiter:            limiter,
		logger:             logger,
	}
	h.globalCfg.Store(&cfg)
	h.ipCfg.Store(&IPLimitConfig{})

	return h
}

// NewHandler creates a new RPC rate limit handler.
func NewHandler(cfg HandlerConfig, logger hclog.Logger) *Handler {
	limiter := multilimiter.NewMultiLimiter(cfg.Config)
	return NewHandlerWithLimiter(cfg, limiter, logger)
}

// Run the limiter cleanup routine until the given context is canceled.
//
// Note: this starts a goroutine.
func (h *Handler) Run(ctx context.Context) {
	h.limiter.Run(ctx)
}

// Allow returns an error if the given operation is not allowed to proceed
// because of an exhausted rate-limit.
func (h *Handler) Allow(op Operation) error {

	if h.serversStatusProvider == nil {
		h.logger.Error("serversStatusProvider required to be set via Register(). bailing on rate limiter")
		return nil
		// TODO: panic and make sure to use the server's recovery handler
		// panic("serversStatusProvider required to be set via Register(..)")
	}

	// Check config entry global limit first - this always applies regardless of global mode
	if configEntryLimit := h.configEntryGlobalLimit(op); configEntryLimit != nil {
		isServer := h.serversStatusProvider.IsServer(string(metadata.GetIP(op.SourceAddr)))
		allow, throttledLimits := h.allowAllLimits([]limit{*configEntryLimit}, isServer)
		if !allow {
			for _, l := range throttledLimits {
				enforced := l.mode == ModeEnforcing
				h.logger.Debug("RPC exceeded config entry rate limit",
					"rpc", op.Name,
					"source_addr", op.SourceAddr,
					"limit_type", l.desc,
					"limit_enforced", enforced,
				)

				// Emit metrics for config entry rate limiting
				metrics.IncrCounterWithLabels([]string{"rpc", "rate_limit", "exceeded"}, 1, []metrics.Label{
					{
						Name:  "limit_type",
						Value: l.desc,
					},
					{
						Name:  "op",
						Value: op.Name,
					},
					{
						Name:  "mode",
						Value: l.mode.String(),
					},
					{
						Name:  "source",
						Value: "config_entry",
					},
				})

				if enforced {
					if h.serversStatusProvider.IsLeader() && op.Type == OperationTypeWrite {
						return ErrRetryLater
					}
					return ErrRetryElsewhere
				}
			}
		}
	}

	cfg := h.globalCfg.Load()
	// If global mode is disabled, skip other rate limits
	if cfg.GlobalLimitConfig.Mode == ModeDisabled {
		return nil
	}

	allow, throttledLimits := h.allowAllLimits(h.limits(op), h.serversStatusProvider.IsServer(string(metadata.GetIP(op.SourceAddr))))

	if !allow {
		for _, l := range throttledLimits {
			enforced := l.mode == ModeEnforcing
			h.logger.Debug("RPC exceeded allowed rate limit",
				"rpc", op.Name,
				"source_addr", op.SourceAddr,
				"limit_type", l.desc,
				"limit_enforced", enforced,
			)

			metrics.IncrCounterWithLabels([]string{"rpc", "rate_limit", "exceeded"}, 1, []metrics.Label{
				{
					Name:  "limit_type",
					Value: l.desc,
				},
				{
					Name:  "op",
					Value: op.Name,
				},
				{
					Name:  "mode",
					Value: l.mode.String(),
				},
			})

			if enforced {
				if h.serversStatusProvider.IsLeader() && op.Type == OperationTypeWrite {
					return ErrRetryLater
				}
				return ErrRetryElsewhere
			}
		}
	}
	return nil
}

func (h *Handler) UpdateConfig(cfg HandlerConfig) {
	existingCfg := h.globalCfg.Load()
	h.globalCfg.Store(&cfg)
	if reflect.DeepEqual(existingCfg, &cfg) {
		h.logger.Warn("UpdateConfig called but configuration has not changed.  Skipping updating the server rate limiter configuration.")
		return
	}

	if !reflect.DeepEqual(existingCfg.GlobalLimitConfig.WriteConfig, cfg.GlobalLimitConfig.WriteConfig) {
		h.limiter.UpdateConfig(cfg.GlobalLimitConfig.WriteConfig, globalWrite)
	}

	if !reflect.DeepEqual(existingCfg.GlobalLimitConfig.ReadConfig, cfg.GlobalLimitConfig.ReadConfig) {
		h.limiter.UpdateConfig(cfg.GlobalLimitConfig.ReadConfig, globalRead)
	}

}

func (h *Handler) Register(serversStatusProvider ServersStatusProvider) {
	h.serversStatusProvider = serversStatusProvider
}

// UpdateGlobalRateLimitConfig updates the global rate limit configuration from Raft.
// This should be called when the global-rate-limit config entry changes.
func (h *Handler) UpdateGlobalRateLimitConfig(cfg *structs.GlobalRateLimitConfigEntry) {
	prevCfg := h.globalRateLimitCfg.Load()
	h.globalRateLimitCfg.Store(cfg)

	if cfg != nil {

		// Validate the configuration
		if cfg.Config.ReadRate != nil && *cfg.Config.ReadRate < 0 {
			h.logger.Error("invalid global rate limit config: read_rate is negative",
				"name", cfg.Name,
				"read_rate", *cfg.Config.ReadRate)
			return
		}
		if cfg.Config.WriteRate != nil && *cfg.Config.WriteRate < 0 {
			h.logger.Error("invalid global rate limit config: write_rate is negative",
				"name", cfg.Name,
				"write_rate", *cfg.Config.WriteRate)
			return
		}

		// Update the limiter with separate read and write rates from config entry
		writeCfg := multilimiter.LimiterConfig{}
		readCfg := multilimiter.LimiterConfig{}
		if cfg.Config.ReadRate == nil {
			readCfg = multilimiter.LimiterConfig{
				Rate:  rate.Limit(rate.Inf),
				Burst: 0,
			}
		} else {
			readCfg = multilimiter.LimiterConfig{
				Rate:  rate.Limit(*cfg.Config.ReadRate),
				Burst: int(*cfg.Config.ReadRate),
			}
		}

		if cfg.Config.WriteRate == nil {
			writeCfg = multilimiter.LimiterConfig{
				Rate:  rate.Limit(rate.Inf),
				Burst: 0,
			}
		} else {
			writeCfg = multilimiter.LimiterConfig{
				Rate:  rate.Limit(*cfg.Config.WriteRate),
				Burst: int(*cfg.Config.WriteRate),
			}
		}
		h.limiter.UpdateConfig(readCfg, configEntryReadLimit)
		h.limiter.UpdateConfig(writeCfg, configEntryWriteLimit)

		// Log at appropriate level based on whether this is a change or initial load
		logFields := []interface{}{
			"name", cfg.Name,
			"read_rate", cfg.Config.ReadRate,
			"write_rate", cfg.Config.WriteRate,
			"priority", cfg.Config.Priority,
			"exclude_endpoints_count", len(cfg.Config.ExcludeEndpoints),
			"modify_index", cfg.ModifyIndex,
		}

		if prevCfg == nil {
			h.logger.Info("loaded global rate limit config entry", logFields...)
		} else {
			logFields = append(logFields, "previous_read_rate", prevCfg.Config.ReadRate)
			logFields = append(logFields, "previous_write_rate", prevCfg.Config.WriteRate)
			h.logger.Info("updated global rate limit config entry", logFields...)
		}

		// Debug log the exclude endpoints for troubleshooting
		if len(cfg.Config.ExcludeEndpoints) > 0 {
			h.logger.Debug("global rate limit exclude endpoints configured",
				"endpoints", cfg.Config.ExcludeEndpoints)
		}
	} else {
		// Config entry removed - set to unlimited
		limiterCfg := multilimiter.LimiterConfig{
			Rate:  rate.Limit(rate.Inf),
			Burst: 0,
		}
		h.limiter.UpdateConfig(limiterCfg, configEntryReadLimit)
		h.limiter.UpdateConfig(limiterCfg, configEntryWriteLimit)

		if prevCfg != nil {
			h.logger.Info("removed global rate limit config entry",
				"previous_name", prevCfg.Name,
				"previous_read_rate", prevCfg.Config.ReadRate,
				"previous_write_rate", prevCfg.Config.WriteRate)
		} else {
			h.logger.Debug("global rate limit config entry cleared (was already nil)")
		}
	}
}

type limit struct {
	mode          Mode
	ent           multilimiter.LimitedEntity
	desc          string
	applyOnServer bool
}

func (h *Handler) allowAllLimits(limits []limit, isServer bool) (bool, []limit) {
	allow := true
	throttledLimits := make([]limit, 0)

	for _, l := range limits {
		if l.mode == ModeDisabled {
			continue
		}

		if isServer && !l.applyOnServer {
			continue
		}

		if !h.limiter.Allow(l.ent) {
			throttledLimits = append(throttledLimits, l)
			allow = false
		}
	}
	return allow, throttledLimits
}

// limits returns the limits to check for the given operation (e.g. global +
// ip-based + tenant-based + config-entry-based).
func (h *Handler) limits(op Operation) []limit {
	limits := make([]limit, 0)

	if global := h.globalLimit(op); global != nil {
		limits = append(limits, *global)
	}

	// Check global rate limit from config entry (stored in Raft)
	if configEntryLimit := h.configEntryGlobalLimit(op); configEntryLimit != nil {
		limits = append(limits, *configEntryLimit)
	}

	if ipGlobal := h.ipGlobalLimit(op); ipGlobal != nil {
		limits = append(limits, *ipGlobal)
	}

	if ipCategory := h.ipCategoryLimit(op); ipCategory != nil {
		limits = append(limits, *ipCategory)
	}

	return limits
}

func (h *Handler) globalLimit(op Operation) *limit {
	if op.Type == OperationTypeExempt {
		return nil
	}
	cfg := h.globalCfg.Load()

	lim := &limit{mode: cfg.GlobalLimitConfig.Mode, applyOnServer: true}
	switch op.Type {
	case OperationTypeRead:
		lim.desc = "global/read"
		lim.ent = globalRead
	case OperationTypeWrite:
		lim.desc = "global/write"
		lim.ent = globalWrite
	default:
		panic(fmt.Sprintf("unknown operation type %d", op.Type))
	}
	return lim
}

// configEntryGlobalLimit checks the global rate limit from the config entry stored in Raft.
// This allows dynamic, cluster-wide rate limiting that's automatically replicated.
func (h *Handler) configEntryGlobalLimit(op Operation) *limit {
	if op.Type == OperationTypeExempt {
		h.logger.Trace("operation exempt from config entry rate limit",
			"rpc", op.Name,
			"operation_type", "exempt")
		return nil
	}

	// Check if this is a safety-critical endpoint that must always be allowed
	// to prevent self-lockout scenarios (e.g., max_rps=0 with emergency_mode=true)
	if h.isSafetyExemptEndpoint(op.Name) {
		h.logger.Trace("operation exempt from config entry rate limit for safety",
			"rpc", op.Name)
		return nil
	}

	cfg := h.globalRateLimitCfg.Load()
	if cfg == nil {
		// No global rate limit config entry configured
		return nil
	}

	// If emergency_mode is false, skip rate limiting entirely
	// (permissive mode only logs, no need to check limits)
	if !cfg.Config.Priority {
		h.logger.Trace("global rate limit config entry exists but emergency_mode is disabled",
			"config_name", cfg.Name,
			"rpc", op.Name)
		return nil
	}

	// Check if this endpoint is in the priority list (bypasses rate limiting)
	for _, endpoint := range cfg.Config.ExcludeEndpoints {
		if endpoint == op.Name {
			h.logger.Debug("operation bypassed rate limiting due to priority endpoint",
				"rpc", op.Name,
				"source_addr", op.SourceAddr,
				"config_name", cfg.Name,
			)
			return nil
		}
	}

	// Priority mode is enabled, so enforce the limit based on operation type
	lim := &limit{
		mode:          ModeEnforcing,
		applyOnServer: true,
	}

	switch op.Type {
	case OperationTypeRead:
		lim.ent = configEntryReadLimit
		lim.desc = fmt.Sprintf("config-entry-read/%s", cfg.Name)
	case OperationTypeWrite:
		lim.ent = configEntryWriteLimit
		lim.desc = fmt.Sprintf("config-entry-write/%s", cfg.Name)
	default:
		panic(fmt.Sprintf("unknown operation type %d", op.Type))
	}

	return lim
}

// safetyExemptEndpoints are RPC endpoints that must always be allowed through
// the config entry rate limiter to prevent self-lockout scenarios.
// For example, if max_rps=0 with emergency_mode=true, operators must still
// be able to update or delete the config entry to recover.
var safetyExemptEndpoints = map[string]struct{}{
	"ConfigEntry.Apply":  {},
	"ConfigEntry.Delete": {},
}

// isSafetyExemptEndpoint checks if an operation is exempt from config entry
// rate limiting for safety reasons (to prevent lockout scenarios).
func (h *Handler) isSafetyExemptEndpoint(opName string) bool {
	_, ok := safetyExemptEndpoints[opName]
	return ok
}

var (
	// globalWrite identifies the global rate limit applied to write operations.
	globalWrite = limitedEntity("global.write")

	// globalRead identifies the global rate limit applied to read operations.
	globalRead = limitedEntity("global.read")

	// globalIPRead identifies the global rate limit applied to read operations.
	globalIPRead = limitedEntity("global.ip.read")

	// globalIPWrite identifies the global rate limit applied to read operations.
	globalIPWrite = limitedEntity("global.ip.write")

	// configEntryReadLimit identifies the global rate limit from config entry for read operations.
	configEntryReadLimit = limitedEntity("config.global.read")

	// configEntryWriteLimit identifies the global rate limit from config entry for write operations.
	configEntryWriteLimit = limitedEntity("config.global.write")
)

// limitedEntity convert the string type to Multilimiter.LimitedEntity
type limitedEntity []byte

// Key satisfies the multilimiter.LimitedEntity interface.
func (prefix limitedEntity) Key() multilimiter.KeyType {
	return multilimiter.Key(prefix, nil)
}

// NullRequestLimitsHandler returns a RequestLimitsHandler that allows every operation.
func NullRequestLimitsHandler() RequestLimitsHandler {
	return nullRequestLimitsHandler{}
}

type nullRequestLimitsHandler struct{}

func (h nullRequestLimitsHandler) UpdateIPConfig(cfg IPLimitConfig) {}

func (nullRequestLimitsHandler) Allow(Operation) error { return nil }

func (nullRequestLimitsHandler) Run(_ context.Context) {}

func (nullRequestLimitsHandler) UpdateConfig(_ HandlerConfig) {}

func (nullRequestLimitsHandler) Register(_ ServersStatusProvider) {}
