// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package rate

import (
	"math"

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

// burstFromRate computes an appropriate Burst value for a given rate.
// The token bucket's Burst (max tokens) must be >= 1 for any positive rate,
// otherwise rate.Limiter.Allow() will never permit a request.
// For rates >= 1, Burst is set to ceil(rate) to allow natural bursting.
// For rates between 0 and 1 (e.g., 0.5 req/s), Burst is clamped to 1.
// For rate == 0, Burst is 0 (blocks everything).
func burstFromRate(r float64) int {
	if r <= 0 {
		return 0
	}
	burst := int(math.Ceil(r))
	if burst < 1 {
		burst = 1
	}
	return burst
}

// UpdateGlobalRateLimitConfig updates the global rate limit configuration from Raft.
// This should be called when the global-rate-limit config entry changes.
// Note: validation of ReadRate/WriteRate (e.g. rejecting negative values) is handled
// by GlobalRateLimitConfigEntry.Validate(), which is called during ConfigEntry.Apply
// before the entry is written to Raft. The handler trusts the config is already valid.
func (h *Handler) UpdateGlobalRateLimitConfig(cfg *structs.GlobalRateLimitConfigEntry) {
	h.globalRateLimitCfg.Store(cfg)

	if cfg != nil {

		// Update the limiter with separate read and write rates from config entry
		var writeCfg multilimiter.LimiterConfig
		var readCfg multilimiter.LimiterConfig
		if cfg.Config.ReadRate == nil {
			readCfg = multilimiter.LimiterConfig{
				Rate:  rate.Limit(rate.Inf),
				Burst: 0,
			}
		} else {
			readCfg = multilimiter.LimiterConfig{
				Rate:  rate.Limit(*cfg.Config.ReadRate),
				Burst: burstFromRate(*cfg.Config.ReadRate),
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
				Burst: burstFromRate(*cfg.Config.WriteRate),
			}
		}
		h.limiter.UpdateConfig(readCfg, configEntryReadLimit)
		h.limiter.UpdateConfig(writeCfg, configEntryWriteLimit)

	} else {
		// Config entry removed - set to unlimited
		limiterCfg := multilimiter.LimiterConfig{
			Rate:  rate.Limit(rate.Inf),
			Burst: 0,
		}
		h.limiter.UpdateConfig(limiterCfg, configEntryReadLimit)
		h.limiter.UpdateConfig(limiterCfg, configEntryWriteLimit)

	}
}
