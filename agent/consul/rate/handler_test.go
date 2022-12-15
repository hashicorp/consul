package rate

import (
	"testing"

	"github.com/hashicorp/consul/agent/consul/multilimiter"
	mock "github.com/stretchr/testify/mock"
)

func TestNewHandlerWithLimiter_CallsUpdateConfig(t *testing.T) {
	mockRateLimiter := multilimiter.NewMockRateLimiter(t)
	mockRateLimiter.On("UpdateConfig", mock.Anything, mock.Anything).Return(
		func(c multilimiter.LimiterConfig, prefix []byte) {})
	readCfg := multilimiter.LimiterConfig{Rate: 100, Burst: 100}
	writeCfg := multilimiter.LimiterConfig{Rate: 99, Burst: 99}
	cfg := &HandlerConfig{
		GlobalReadConfig:  readCfg,
		GlobalWriteConfig: writeCfg,
		GlobalMode:        ModeEnforcing,
	}
	NewHandlerWithLimiter(*cfg, nil, mockRateLimiter)
	mockRateLimiter.AssertNumberOfCalls(t, "UpdateConfig", 2)
}

func TestUpdateConfig_DoesNotUpdateRateLimiterIfNoConfigChange(t *testing.T) {
	mockRateLimiter := multilimiter.NewMockRateLimiter(t)
	mockRateLimiter.On("UpdateConfig", mock.Anything, mock.Anything).Return(
		func(c multilimiter.LimiterConfig, prefix []byte) {})
	readCfg := multilimiter.LimiterConfig{Rate: 100, Burst: 100}
	writeCfg := multilimiter.LimiterConfig{Rate: 99, Burst: 99}
	cfg := &HandlerConfig{
		GlobalReadConfig:  readCfg,
		GlobalWriteConfig: writeCfg,
		GlobalMode:        ModeEnforcing,
	}
	handler := NewHandlerWithLimiter(*cfg, nil, mockRateLimiter)
	handler.UpdateConfig(*cfg)
	mockRateLimiter.AssertNumberOfCalls(t, "UpdateConfig", 2)
}

func TestUpdateConfig(t *testing.T) {
	type testCase struct {
		description   string
		configModFunc func(cfg *HandlerConfig)
		assertFunc    func(mockRateLimiter *multilimiter.MockRateLimiter)
	}
	testCases := []testCase{
		{
			description:   "RateLimiter does not get updated when config does not change.",
			configModFunc: func(cfg *HandlerConfig) {},
			assertFunc: func(mockRateLimiter *multilimiter.MockRateLimiter) {
				mockRateLimiter.AssertNumberOfCalls(t, "UpdateConfig", 0)
			},
		},
		{
			description: "RateLimiter gets updated when GlobalReadConfig changes.",
			configModFunc: func(cfg *HandlerConfig) {
				cfg.GlobalReadConfig.Burst++
			},
			assertFunc: func(mockRateLimiter *multilimiter.MockRateLimiter) {
				mockRateLimiter.AssertNumberOfCalls(t, "UpdateConfig", 1)
			},
		},
		{
			description: "RateLimiter gets updated when GlobalWriteConfig changes.",
			configModFunc: func(cfg *HandlerConfig) {
				cfg.GlobalWriteConfig.Burst++
			},
			assertFunc: func(mockRateLimiter *multilimiter.MockRateLimiter) {
				mockRateLimiter.AssertNumberOfCalls(t, "UpdateConfig", 1)
			},
		},
		{
			description: "RateLimiter does not get updated when GlobalMode changes.",
			configModFunc: func(cfg *HandlerConfig) {
				cfg.GlobalMode = ModePermissive
			},
			assertFunc: func(mockRateLimiter *multilimiter.MockRateLimiter) {
				mockRateLimiter.AssertNumberOfCalls(t, "UpdateConfig", 0)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			readCfg := multilimiter.LimiterConfig{Rate: 100, Burst: 100}
			writeCfg := multilimiter.LimiterConfig{Rate: 99, Burst: 99}
			cfg := &HandlerConfig{
				GlobalReadConfig:  readCfg,
				GlobalWriteConfig: writeCfg,
				GlobalMode:        ModeEnforcing,
			}
			mockRateLimiter := multilimiter.NewMockRateLimiter(t)
			mockRateLimiter.On("UpdateConfig", mock.Anything, mock.Anything).Return(
				func(c multilimiter.LimiterConfig, prefix []byte) {})
			handler := NewHandlerWithLimiter(*cfg, nil, mockRateLimiter)
			mockRateLimiter.Calls = nil
			tc.configModFunc(cfg)
			handler.UpdateConfig(*cfg)
			tc.assertFunc(mockRateLimiter)
		})
	}
}
