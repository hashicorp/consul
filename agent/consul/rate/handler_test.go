// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package rate

import (
	"bytes"
	"github.com/hashicorp/consul/agent/metrics"
	"github.com/stretchr/testify/require"
	"net"
	"net/netip"
	"testing"

	"golang.org/x/time/rate"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/mock"

	"github.com/hashicorp/consul/agent/consul/multilimiter"
)

//
// Revisit test when handler.go:189 TODO implemented
//
// func TestHandler_Allow_PanicsWhenLeaderStatusProviderNotRegistered(t *testing.T) {
// 	defer func() {
// 		err := recover()
// 		if err == nil {
// 			t.Fatal("Run should panic")
// 		}
// 	}()

// 	handler := NewHandler(HandlerConfig{}, hclog.NewNullLogger())
// 	handler.Allow(Operation{})
// 	// intentionally skip handler.Register(...)
// }

func TestHandler(t *testing.T) {
	var (
		rpcName    = "Foo.Bar"
		sourceAddr = net.TCPAddrFromAddrPort(netip.MustParseAddrPort("1.2.3.4:5678"))
	)

	type limitCheck struct {
		limit multilimiter.LimitedEntity
		allow bool
	}
	testCases := map[string]struct {
		op                Operation
		globalMode        Mode
		checks            []limitCheck
		isLeader          bool
		expectErr         error
		expectLog         bool
		expectMetric      bool
		expectMetricName  string
		expectMetricCount float64
	}{
		"operation exempt from limiting": {
			op: Operation{
				Type:       OperationTypeExempt,
				Name:       rpcName,
				SourceAddr: sourceAddr,
			},
			globalMode:   ModeEnforcing,
			checks:       []limitCheck{},
			expectErr:    nil,
			expectLog:    false,
			expectMetric: false,
		},
		"global write limit disabled": {
			op: Operation{
				Type:       OperationTypeWrite,
				Name:       rpcName,
				SourceAddr: sourceAddr,
			},
			globalMode:   ModeDisabled,
			checks:       []limitCheck{},
			expectErr:    nil,
			expectLog:    false,
			expectMetric: false,
		},
		"global write limit within allowance": {
			op: Operation{
				Type:       OperationTypeWrite,
				Name:       rpcName,
				SourceAddr: sourceAddr,
			},
			globalMode: ModeEnforcing,
			checks: []limitCheck{
				{limit: globalWrite, allow: true},
			},
			expectErr:    nil,
			expectLog:    false,
			expectMetric: false,
		},
		"global write limit exceeded (permissive)": {
			op: Operation{
				Type:       OperationTypeWrite,
				Name:       rpcName,
				SourceAddr: sourceAddr,
			},
			globalMode: ModePermissive,
			checks: []limitCheck{
				{limit: globalWrite, allow: false},
			},
			expectErr:         nil,
			expectLog:         true,
			expectMetric:      true,
			expectMetricName:  "rpc.rate_limit.exceeded;limit_type=global/write;op=Foo.Bar;mode=permissive",
			expectMetricCount: 1,
		},
		"global write limit exceeded (enforcing, leader)": {
			op: Operation{
				Type:       OperationTypeWrite,
				Name:       rpcName,
				SourceAddr: sourceAddr,
			},
			globalMode: ModeEnforcing,
			checks: []limitCheck{
				{limit: globalWrite, allow: false},
			},
			isLeader:          true,
			expectErr:         ErrRetryLater,
			expectLog:         true,
			expectMetric:      true,
			expectMetricName:  "rpc.rate_limit.exceeded;limit_type=global/write;op=Foo.Bar;mode=enforcing",
			expectMetricCount: 1,
		},
		"global write limit exceeded (enforcing, follower)": {
			op: Operation{
				Type:       OperationTypeWrite,
				Name:       rpcName,
				SourceAddr: sourceAddr,
			},
			globalMode: ModeEnforcing,
			checks: []limitCheck{
				{limit: globalWrite, allow: false},
			},
			isLeader:          false,
			expectErr:         ErrRetryElsewhere,
			expectLog:         true,
			expectMetric:      true,
			expectMetricName:  "rpc.rate_limit.exceeded;limit_type=global/write;op=Foo.Bar;mode=enforcing",
			expectMetricCount: 1,
		},
		"global read limit disabled": {
			op: Operation{
				Type:       OperationTypeRead,
				Name:       rpcName,
				SourceAddr: sourceAddr,
			},
			globalMode:   ModeDisabled,
			checks:       []limitCheck{},
			expectErr:    nil,
			expectLog:    false,
			expectMetric: false,
		},
		"global read limit within allowance": {
			op: Operation{
				Type:       OperationTypeRead,
				Name:       rpcName,
				SourceAddr: sourceAddr,
			},
			globalMode: ModeEnforcing,
			checks: []limitCheck{
				{limit: globalRead, allow: true},
			},
			expectErr:    nil,
			expectLog:    false,
			expectMetric: false,
		},
		"global read limit exceeded (permissive)": {
			op: Operation{
				Type:       OperationTypeRead,
				Name:       rpcName,
				SourceAddr: sourceAddr,
			},
			globalMode: ModePermissive,
			checks: []limitCheck{
				{limit: globalRead, allow: false},
			},
			expectErr:         nil,
			expectLog:         true,
			expectMetric:      true,
			expectMetricName:  "rpc.rate_limit.exceeded;limit_type=global/read;op=Foo.Bar;mode=permissive",
			expectMetricCount: 1,
		},
		"global read limit exceeded (enforcing, leader)": {
			op: Operation{
				Type:       OperationTypeRead,
				Name:       rpcName,
				SourceAddr: sourceAddr,
			},
			globalMode: ModeEnforcing,
			checks: []limitCheck{
				{limit: globalRead, allow: false},
			},
			isLeader:          true,
			expectErr:         ErrRetryElsewhere,
			expectLog:         true,
			expectMetric:      true,
			expectMetricName:  "rpc.rate_limit.exceeded;limit_type=global/read;op=Foo.Bar;mode=enforcing",
			expectMetricCount: 1,
		},
		"global read limit exceeded (enforcing, follower)": {
			op: Operation{
				Type:       OperationTypeRead,
				Name:       rpcName,
				SourceAddr: sourceAddr,
			},
			globalMode: ModeEnforcing,
			checks: []limitCheck{
				{limit: globalRead, allow: false},
			},
			isLeader:          false,
			expectErr:         ErrRetryElsewhere,
			expectLog:         true,
			expectMetric:      true,
			expectMetricName:  "rpc.rate_limit.exceeded;limit_type=global/read;op=Foo.Bar;mode=enforcing",
			expectMetricCount: 1,
		},
	}
	for desc, tc := range testCases {
		t.Run(desc, func(t *testing.T) {
			sink := metrics.TestSetupMetrics(t, "")
			limiter := multilimiter.NewMockRateLimiter(t)
			limiter.On("UpdateConfig", mock.Anything, mock.Anything).Return()
			for _, c := range tc.checks {
				limiter.On("Allow", mock.Anything).Return(c.allow)
			}

			leaderStatusProvider := NewMockLeaderStatusProvider(t)
			leaderStatusProvider.On("IsLeader").Return(tc.isLeader).Maybe()

			var output bytes.Buffer
			logger := hclog.NewInterceptLogger(&hclog.LoggerOptions{
				Level:  hclog.Trace,
				Output: &output,
			})

			handler := NewHandlerWithLimiter(
				HandlerConfig{
					GlobalLimitConfig: GlobalLimitConfig{
						Mode: tc.globalMode,
						ReadWriteConfig: ReadWriteConfig{
							ReadConfig:  multilimiter.LimiterConfig{},
							WriteConfig: multilimiter.LimiterConfig{},
						},
					},
				},
				limiter,
				logger,
			)
			handler.Register(leaderStatusProvider)

			require.Equal(t, tc.expectErr, handler.Allow(tc.op))

			if tc.expectLog {
				require.Contains(t, output.String(), "RPC exceeded allowed rate limit")
			} else {
				require.Zero(t, output.Len(), "expected no logs to be emitted")
			}

			if tc.expectMetric {
				metrics.AssertCounter(t, sink, tc.expectMetricName, tc.expectMetricCount)
			}
		})
	}
}

func TestNewHandlerWithLimiter_CallsUpdateConfig(t *testing.T) {
	mockRateLimiter := multilimiter.NewMockRateLimiter(t)
	mockRateLimiter.On("UpdateConfig", mock.Anything, mock.Anything).Return()
	readCfg := multilimiter.LimiterConfig{Rate: 100, Burst: 100}
	writeCfg := multilimiter.LimiterConfig{Rate: 99, Burst: 99}
	cfg := &HandlerConfig{
		GlobalLimitConfig: GlobalLimitConfig{
			Mode: ModeEnforcing,
			ReadWriteConfig: ReadWriteConfig{
				ReadConfig:  readCfg,
				WriteConfig: writeCfg,
			},
		},
	}
	logger := hclog.NewNullLogger()
	NewHandlerWithLimiter(*cfg, mockRateLimiter, logger)
	mockRateLimiter.AssertNumberOfCalls(t, "UpdateConfig", 2)
}

func infReadRateConfig() ReadWriteConfig {
	return ReadWriteConfig{
		ReadConfig:  multilimiter.LimiterConfig{Rate: rate.Inf},
		WriteConfig: multilimiter.LimiterConfig{Rate: rate.Inf},
	}
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
				rc := multilimiter.LimiterConfig{Rate: cfg.GlobalLimitConfig.ReadWriteConfig.ReadConfig.Rate, Burst: cfg.GlobalLimitConfig.ReadWriteConfig.ReadConfig.Burst + 1}
				cfg.GlobalLimitConfig.ReadWriteConfig.ReadConfig = rc
			},
			assertFunc: func(mockRateLimiter *multilimiter.MockRateLimiter, cfg *HandlerConfig) {
				mockRateLimiter.AssertNumberOfCalls(t, "UpdateConfig", 1)
				mockRateLimiter.AssertCalled(t, "UpdateConfig", cfg.GlobalLimitConfig.ReadWriteConfig.ReadConfig, []byte("global.read"))
			},
		},
		{
			description: "RateLimiter gets updated when GlobalWriteConfig changes.",
			configModFunc: func(cfg *HandlerConfig) {
				wc := multilimiter.LimiterConfig{Rate: cfg.GlobalLimitConfig.ReadWriteConfig.WriteConfig.Rate, Burst: cfg.GlobalLimitConfig.ReadWriteConfig.WriteConfig.Burst + 1}
				cfg.GlobalLimitConfig.ReadWriteConfig.WriteConfig = wc
			},
			assertFunc: func(mockRateLimiter *multilimiter.MockRateLimiter, cfg *HandlerConfig) {
				mockRateLimiter.AssertNumberOfCalls(t, "UpdateConfig", 1)
				mockRateLimiter.AssertCalled(t, "UpdateConfig", cfg.GlobalLimitConfig.ReadWriteConfig.WriteConfig, []byte("global.write"))
			},
		},
		{
			description: "RateLimiter does not get updated when GlobalMode changes.",
			configModFunc: func(cfg *HandlerConfig) {
				cfg.GlobalLimitConfig.Mode = ModePermissive
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
				GlobalLimitConfig: GlobalLimitConfig{
					Mode: ModeEnforcing,
					ReadWriteConfig: ReadWriteConfig{
						ReadConfig:  readCfg,
						WriteConfig: writeCfg,
					},
				},
			}
			mockRateLimiter := multilimiter.NewMockRateLimiter(t)
			mockRateLimiter.On("UpdateConfig", mock.Anything, mock.Anything).Return()
			logger := hclog.NewNullLogger()
			handler := NewHandlerWithLimiter(*cfg, mockRateLimiter, logger)
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
		expectedAllowCalls bool
	}
	testCases := []testCase{
		{
			description: "RateLimiter does not get called when mode is disabled.",
			cfg: &HandlerConfig{
				GlobalLimitConfig: GlobalLimitConfig{
					Mode: ModeDisabled,
					ReadWriteConfig: ReadWriteConfig{
						ReadConfig:  readCfg,
						WriteConfig: writeCfg,
					},
				},
			},
			expectedAllowCalls: false,
		},
		{
			description: "RateLimiter gets called when mode is permissive.",
			cfg: &HandlerConfig{
				GlobalLimitConfig: GlobalLimitConfig{
					Mode: ModePermissive,
					ReadWriteConfig: ReadWriteConfig{
						ReadConfig:  readCfg,
						WriteConfig: writeCfg,
					},
				},
			},
			expectedAllowCalls: true,
		},
		{
			description: "RateLimiter gets called when mode is enforcing.",
			cfg: &HandlerConfig{
				GlobalLimitConfig: GlobalLimitConfig{
					Mode: ModeEnforcing,
					ReadWriteConfig: ReadWriteConfig{
						ReadConfig:  readCfg,
						WriteConfig: writeCfg,
					},
				},
			},
			expectedAllowCalls: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			mockRateLimiter := multilimiter.NewMockRateLimiter(t)
			if tc.expectedAllowCalls {
				mockRateLimiter.On("Allow", mock.Anything).Return(func(entity multilimiter.LimitedEntity) bool { return true })
			}
			mockRateLimiter.On("UpdateConfig", mock.Anything, mock.Anything).Return()
			logger := hclog.NewNullLogger()
			delegate := NewMockLeaderStatusProvider(t)
			delegate.On("IsLeader").Return(true).Maybe()
			handler := NewHandlerWithLimiter(*tc.cfg, mockRateLimiter, logger)
			handler.Register(delegate)
			addr := net.TCPAddrFromAddrPort(netip.MustParseAddrPort("127.0.0.1:1234"))
			mockRateLimiter.Calls = nil
			handler.Allow(Operation{Name: "test", SourceAddr: addr})
			mockRateLimiter.AssertExpectations(t)
		})
	}
}
