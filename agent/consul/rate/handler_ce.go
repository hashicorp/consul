// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package rate

import (
	"github.com/hashicorp/consul/agent/consul/multilimiter"
	"github.com/hashicorp/consul/agent/structs"
	"golang.org/x/time/rate"
)

type IPLimitConfig struct{}

func (h *Handler) UpdateIPConfig(cfg IPLimitConfig) {
	// noop
}

func (h *Handler) ipGlobalLimit(op Operation) *limit {
	return nil
}

func (h *Handler) ipCategoryLimit(op Operation) *limit {
	return nil
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
