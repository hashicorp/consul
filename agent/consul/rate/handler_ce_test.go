// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package rate

import (
	"bytes"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"github.com/hashicorp/consul/agent/consul/multilimiter"
	"github.com/hashicorp/consul/agent/structs"
)

func newTestHandler(t *testing.T) (*Handler, *multilimiter.MockRateLimiter, *bytes.Buffer) {
	t.Helper()

	mockLimiter := multilimiter.NewMockRateLimiter(t)
	mockLimiter.On("UpdateConfig", mock.Anything, mock.Anything).Return()

	var output bytes.Buffer
	logger := hclog.NewInterceptLogger(&hclog.LoggerOptions{
		Level:  hclog.Trace,
		Output: &output,
	})

	handler := NewHandlerWithLimiter(
		HandlerConfig{
			GlobalLimitConfig: GlobalLimitConfig{
				Mode: ModeEnforcing,
				ReadWriteConfig: ReadWriteConfig{
					ReadConfig:  multilimiter.LimiterConfig{},
					WriteConfig: multilimiter.LimiterConfig{},
				},
			},
		},
		mockLimiter,
		logger,
	)

	// Clear initialization calls so we can assert only on UpdateGlobalRateLimitConfig calls
	mockLimiter.Calls = nil

	return handler, mockLimiter, &output
}

// assertUpdateConfigCalled checks that UpdateConfig was called with the expected LimiterConfig
// for a given prefix string (e.g., "global.configentry.read"). We use mock.MatchedBy for the
// prefix because limitedEntity is a named []byte type and testify's AssertCalled can fail
// on type comparison.
func assertUpdateConfigCalled(t *testing.T, mockLimiter *multilimiter.MockRateLimiter, expectedCfg multilimiter.LimiterConfig, prefix string) {
	t.Helper()
	found := false
	for _, call := range mockLimiter.Calls {
		if call.Method != "UpdateConfig" {
			continue
		}
		cfg, ok := call.Arguments.Get(0).(multilimiter.LimiterConfig)
		if !ok {
			continue
		}
		pfx := call.Arguments.Get(1)
		var pfxStr string
		switch v := pfx.(type) {
		case []byte:
			pfxStr = string(v)
		case limitedEntity:
			pfxStr = string(v)
		default:
			continue
		}
		if pfxStr == prefix && cfg == expectedCfg {
			found = true
			break
		}
	}
	require.True(t, found, "expected UpdateConfig call with cfg=%+v prefix=%q not found", expectedCfg, prefix)
}

func TestUpdateGlobalRateLimitConfig_NilConfig(t *testing.T) {
	handler, mockLimiter, _ := newTestHandler(t)

	handler.UpdateGlobalRateLimitConfig(nil)

	// Should store nil and set unlimited rates for both read and write
	stored := handler.globalRateLimitCfg.Load()
	require.Nil(t, stored)

	unlimitedCfg := multilimiter.LimiterConfig{Rate: rate.Limit(rate.Inf), Burst: 0}

	// Should have called UpdateConfig twice: once for read, once for write
	mockLimiter.AssertNumberOfCalls(t, "UpdateConfig", 2)
	assertUpdateConfigCalled(t, mockLimiter, unlimitedCfg, "global.configentry.read")
	assertUpdateConfigCalled(t, mockLimiter, unlimitedCfg, "global.configentry.write")
}

func TestUpdateGlobalRateLimitConfig_BothRatesSet(t *testing.T) {
	handler, mockLimiter, _ := newTestHandler(t)

	cfg := &structs.GlobalRateLimitConfigEntry{
		Name: "test",
		Config: &structs.GlobalRateLimitConfig{
			ReadRate:  float64Ptr(100),
			WriteRate: float64Ptr(200),
		},
	}

	handler.UpdateGlobalRateLimitConfig(cfg)

	stored := handler.globalRateLimitCfg.Load()
	require.NotNil(t, stored)
	require.Equal(t, "test", stored.Name)

	mockLimiter.AssertNumberOfCalls(t, "UpdateConfig", 2)
	assertUpdateConfigCalled(t, mockLimiter, multilimiter.LimiterConfig{
		Rate:  rate.Limit(100),
		Burst: 100,
	}, "global.configentry.read")
	assertUpdateConfigCalled(t, mockLimiter, multilimiter.LimiterConfig{
		Rate:  rate.Limit(200),
		Burst: 200,
	}, "global.configentry.write")
}

func TestUpdateGlobalRateLimitConfig_ReadRateNilWriteRateSet(t *testing.T) {
	handler, mockLimiter, _ := newTestHandler(t)

	cfg := &structs.GlobalRateLimitConfigEntry{
		Name: "test",
		Config: &structs.GlobalRateLimitConfig{
			ReadRate:  nil,
			WriteRate: float64Ptr(50),
		},
	}

	handler.UpdateGlobalRateLimitConfig(cfg)

	mockLimiter.AssertNumberOfCalls(t, "UpdateConfig", 2)
	assertUpdateConfigCalled(t, mockLimiter, multilimiter.LimiterConfig{
		Rate:  rate.Limit(rate.Inf),
		Burst: 0,
	}, "global.configentry.read")
	assertUpdateConfigCalled(t, mockLimiter, multilimiter.LimiterConfig{
		Rate:  rate.Limit(50),
		Burst: 50,
	}, "global.configentry.write")
}

func TestUpdateGlobalRateLimitConfig_ReadRateSetWriteRateNil(t *testing.T) {
	handler, mockLimiter, _ := newTestHandler(t)

	cfg := &structs.GlobalRateLimitConfigEntry{
		Name: "test",
		Config: &structs.GlobalRateLimitConfig{
			ReadRate:  float64Ptr(75),
			WriteRate: nil,
		},
	}

	handler.UpdateGlobalRateLimitConfig(cfg)

	mockLimiter.AssertNumberOfCalls(t, "UpdateConfig", 2)
	assertUpdateConfigCalled(t, mockLimiter, multilimiter.LimiterConfig{
		Rate:  rate.Limit(75),
		Burst: 75,
	}, "global.configentry.read")
	assertUpdateConfigCalled(t, mockLimiter, multilimiter.LimiterConfig{
		Rate:  rate.Limit(rate.Inf),
		Burst: 0,
	}, "global.configentry.write")
}

func TestUpdateGlobalRateLimitConfig_BothRatesNil(t *testing.T) {
	handler, mockLimiter, _ := newTestHandler(t)

	cfg := &structs.GlobalRateLimitConfigEntry{
		Name: "test",
		Config: &structs.GlobalRateLimitConfig{
			ReadRate:  nil,
			WriteRate: nil,
		},
	}

	handler.UpdateGlobalRateLimitConfig(cfg)

	mockLimiter.AssertNumberOfCalls(t, "UpdateConfig", 2)
	assertUpdateConfigCalled(t, mockLimiter, multilimiter.LimiterConfig{
		Rate:  rate.Limit(rate.Inf),
		Burst: 0,
	}, "global.configentry.read")
	assertUpdateConfigCalled(t, mockLimiter, multilimiter.LimiterConfig{
		Rate:  rate.Limit(rate.Inf),
		Burst: 0,
	}, "global.configentry.write")
}

func TestUpdateGlobalRateLimitConfig_ZeroRates(t *testing.T) {
	handler, mockLimiter, _ := newTestHandler(t)

	cfg := &structs.GlobalRateLimitConfigEntry{
		Name: "test-zero",
		Config: &structs.GlobalRateLimitConfig{
			ReadRate:  float64Ptr(0),
			WriteRate: float64Ptr(0),
		},
	}

	handler.UpdateGlobalRateLimitConfig(cfg)

	// Zero rates are valid (effectively blocks everything)
	mockLimiter.AssertNumberOfCalls(t, "UpdateConfig", 2)
	assertUpdateConfigCalled(t, mockLimiter, multilimiter.LimiterConfig{
		Rate:  rate.Limit(0),
		Burst: 0,
	}, "global.configentry.read")
	assertUpdateConfigCalled(t, mockLimiter, multilimiter.LimiterConfig{
		Rate:  rate.Limit(0),
		Burst: 0,
	}, "global.configentry.write")
}

func TestUpdateGlobalRateLimitConfig_FractionalRates(t *testing.T) {
	handler, mockLimiter, _ := newTestHandler(t)

	cfg := &structs.GlobalRateLimitConfigEntry{
		Name: "test-fractional",
		Config: &structs.GlobalRateLimitConfig{
			ReadRate:  float64Ptr(0.5),
			WriteRate: float64Ptr(1.5),
		},
	}

	handler.UpdateGlobalRateLimitConfig(cfg)

	mockLimiter.AssertNumberOfCalls(t, "UpdateConfig", 2)
	assertUpdateConfigCalled(t, mockLimiter, multilimiter.LimiterConfig{
		Rate:  rate.Limit(0.5),
		Burst: 1, // burstFromRate(0.5) = 1 (clamped to min 1 for positive rates)
	}, "global.configentry.read")
	assertUpdateConfigCalled(t, mockLimiter, multilimiter.LimiterConfig{
		Rate:  rate.Limit(1.5),
		Burst: 2, // burstFromRate(1.5) = ceil(1.5) = 2
	}, "global.configentry.write")
}

func TestUpdateGlobalRateLimitConfig_UpdateThenRemove(t *testing.T) {
	handler, mockLimiter, _ := newTestHandler(t)

	// First, set a config
	cfg := &structs.GlobalRateLimitConfigEntry{
		Name: "test",
		Config: &structs.GlobalRateLimitConfig{
			ReadRate:  float64Ptr(100),
			WriteRate: float64Ptr(200),
		},
	}
	handler.UpdateGlobalRateLimitConfig(cfg)

	stored := handler.globalRateLimitCfg.Load()
	require.NotNil(t, stored)

	mockLimiter.Calls = nil

	// Then, remove the config (set to nil)
	handler.UpdateGlobalRateLimitConfig(nil)

	stored = handler.globalRateLimitCfg.Load()
	require.Nil(t, stored)

	// Should reset to unlimited
	mockLimiter.AssertNumberOfCalls(t, "UpdateConfig", 2)
	assertUpdateConfigCalled(t, mockLimiter, multilimiter.LimiterConfig{
		Rate:  rate.Limit(rate.Inf),
		Burst: 0,
	}, "global.configentry.read")
	assertUpdateConfigCalled(t, mockLimiter, multilimiter.LimiterConfig{
		Rate:  rate.Limit(rate.Inf),
		Burst: 0,
	}, "global.configentry.write")
}

func TestUpdateGlobalRateLimitConfig_SuccessiveUpdates(t *testing.T) {
	handler, mockLimiter, _ := newTestHandler(t)

	// First update
	cfg1 := &structs.GlobalRateLimitConfigEntry{
		Name: "first",
		Config: &structs.GlobalRateLimitConfig{
			ReadRate:  float64Ptr(10),
			WriteRate: float64Ptr(20),
		},
	}
	handler.UpdateGlobalRateLimitConfig(cfg1)

	stored := handler.globalRateLimitCfg.Load()
	require.Equal(t, "first", stored.Name)

	mockLimiter.Calls = nil

	// Second update with different values
	cfg2 := &structs.GlobalRateLimitConfigEntry{
		Name: "second",
		Config: &structs.GlobalRateLimitConfig{
			ReadRate:  float64Ptr(500),
			WriteRate: float64Ptr(1000),
		},
	}
	handler.UpdateGlobalRateLimitConfig(cfg2)

	stored = handler.globalRateLimitCfg.Load()
	require.Equal(t, "second", stored.Name)

	mockLimiter.AssertNumberOfCalls(t, "UpdateConfig", 2)
	assertUpdateConfigCalled(t, mockLimiter, multilimiter.LimiterConfig{
		Rate:  rate.Limit(500),
		Burst: 500,
	}, "global.configentry.read")
	assertUpdateConfigCalled(t, mockLimiter, multilimiter.LimiterConfig{
		Rate:  rate.Limit(1000),
		Burst: 1000,
	}, "global.configentry.write")
}

func TestBurstFromRate(t *testing.T) {
	tests := map[string]struct {
		rate     float64
		expected int
	}{
		"zero rate":           {rate: 0, expected: 0},
		"negative rate":       {rate: -1, expected: 0},
		"fractional below 1":  {rate: 0.5, expected: 1},
		"very small fraction": {rate: 0.001, expected: 1},
		"exactly 1":           {rate: 1.0, expected: 1},
		"fractional above 1":  {rate: 1.5, expected: 2},
		"integer value":       {rate: 100, expected: 100},
		"large fractional":    {rate: 99.1, expected: 100},
		"just below integer":  {rate: 9.9999, expected: 10},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := burstFromRate(tc.rate)
			require.Equal(t, tc.expected, result)
		})
	}
}
