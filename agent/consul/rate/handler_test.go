package rate

import (
	"net"
	"net/netip"
	"testing"

	"github.com/hashicorp/consul/agent/consul/multilimiter"
	"github.com/hashicorp/go-hclog"
	mock "github.com/stretchr/testify/mock"
)

func TestNewHandlerWithLimiter_CallsUpdateConfig(t *testing.T) {
	mockRateLimiter := multilimiter.NewMockRateLimiter(t)
	mockRateLimiter.On("UpdateConfig", mock.Anything, mock.Anything).Return()
	readCfg := multilimiter.LimiterConfig{Rate: 100, Burst: 100}
	writeCfg := multilimiter.LimiterConfig{Rate: 99, Burst: 99}
	cfg := &HandlerConfig{
		GlobalReadConfig:  readCfg,
		GlobalWriteConfig: writeCfg,
		GlobalMode:        ModeEnforcing,
	}

	logger := hclog.NewNullLogger()
	NewHandlerWithLimiter(*cfg, nil, mockRateLimiter, logger)
	mockRateLimiter.AssertNumberOfCalls(t, "UpdateConfig", 2)
}

func TestUpdateConfig(t *testing.T) {
	type testCase struct {
		description   string
		configModFunc func(cfg *HandlerConfig)
		assertFunc    func(mockRateLimiter *multilimiter.MockRateLimiter, cfg *HandlerConfig)
	}
	testCases := []testCase{
		{
			description:   "RateLimiter does not get updated when config does not change.",
			configModFunc: func(cfg *HandlerConfig) {},
			assertFunc: func(mockRateLimiter *multilimiter.MockRateLimiter, cfg *HandlerConfig) {
				mockRateLimiter.AssertNumberOfCalls(t, "UpdateConfig", 0)
			},
		},
		{
			description: "RateLimiter gets updated when GlobalReadConfig changes.",
			configModFunc: func(cfg *HandlerConfig) {
				cfg.GlobalReadConfig.Burst++
			},
			assertFunc: func(mockRateLimiter *multilimiter.MockRateLimiter, cfg *HandlerConfig) {
				mockRateLimiter.AssertNumberOfCalls(t, "UpdateConfig", 1)
				mockRateLimiter.AssertCalled(t, "UpdateConfig", cfg.GlobalReadConfig, []byte("global.read"))
			},
		},
		{
			description: "RateLimiter gets updated when GlobalWriteConfig changes.",
			configModFunc: func(cfg *HandlerConfig) {
				cfg.GlobalWriteConfig.Burst++
			},
			assertFunc: func(mockRateLimiter *multilimiter.MockRateLimiter, cfg *HandlerConfig) {
				mockRateLimiter.AssertNumberOfCalls(t, "UpdateConfig", 1)
				mockRateLimiter.AssertCalled(t, "UpdateConfig", cfg.GlobalWriteConfig, []byte("global.write"))
			},
		},
		{
			description: "RateLimiter does not get updated when GlobalMode changes.",
			configModFunc: func(cfg *HandlerConfig) {
				cfg.GlobalMode = ModePermissive
			},
			assertFunc: func(mockRateLimiter *multilimiter.MockRateLimiter, cfg *HandlerConfig) {
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
			mockRateLimiter.On("UpdateConfig", mock.Anything, mock.Anything).Return()
			logger := hclog.NewNullLogger()
			handler := NewHandlerWithLimiter(*cfg, nil, mockRateLimiter, logger)
			mockRateLimiter.Calls = nil
			tc.configModFunc(cfg)
			handler.UpdateConfig(*cfg)
			tc.assertFunc(mockRateLimiter, cfg)
		})
	}
}

func TestAllow(t *testing.T) {
	readCfg := multilimiter.LimiterConfig{Rate: 100, Burst: 100}
	writeCfg := multilimiter.LimiterConfig{Rate: 99, Burst: 99}

	type testCase struct {
		description        string
		cfg                *HandlerConfig
		expectedAllowCalls int
	}
	testCases := []testCase{
		{
			description: "RateLimiter does not get called when mode is disabled.",
			cfg: &HandlerConfig{
				GlobalReadConfig:  readCfg,
				GlobalWriteConfig: writeCfg,
				GlobalMode:        ModeDisabled,
			},
			expectedAllowCalls: 0,
		},
		{
			description: "RateLimiter gets called when mode is permissive.",
			cfg: &HandlerConfig{
				GlobalReadConfig:  readCfg,
				GlobalWriteConfig: writeCfg,
				GlobalMode:        ModePermissive,
			},
			expectedAllowCalls: 1,
		},
		{
			description: "RateLimiter gets called when mode is enforcing.",
			cfg: &HandlerConfig{
				GlobalReadConfig:  readCfg,
				GlobalWriteConfig: writeCfg,
				GlobalMode:        ModeEnforcing,
			},
			expectedAllowCalls: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			mockRateLimiter := multilimiter.NewMockRateLimiter(t)
			if tc.expectedAllowCalls > 0 {
				mockRateLimiter.On("Allow", mock.Anything).Return(func(entity multilimiter.LimitedEntity) bool { return true })
			}
			mockRateLimiter.On("UpdateConfig", mock.Anything, mock.Anything).Return()
			logger := hclog.NewNullLogger()
			handler := NewHandlerWithLimiter(*tc.cfg, nil, mockRateLimiter, logger)
			addr := net.TCPAddrFromAddrPort(netip.MustParseAddrPort("127.0.0.1:1234"))
			mockRateLimiter.Calls = nil
			handler.Allow(Operation{Name: "test", SourceAddr: addr})
			mockRateLimiter.AssertNumberOfCalls(t, "Allow", tc.expectedAllowCalls)
		})
	}
}
