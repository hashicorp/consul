// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package rate

import (
	"bytes"
	"net"
	"net/netip"
	"testing"

	"github.com/hashicorp/consul/agent/metrics"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/require"

	"golang.org/x/time/rate"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/mock"

	"github.com/hashicorp/consul/agent/consul/multilimiter"
)

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
		cfg               *structs.GlobalRateLimitConfigEntry
		glbcfg            *HandlerConfig
		checks            []limitCheck
		isLeader          bool
		isServer          bool
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
			glbcfg: &HandlerConfig{
				GlobalLimitConfig: GlobalLimitConfig{
					Mode: ModeEnforcing,
					ReadWriteConfig: ReadWriteConfig{
						ReadConfig: multilimiter.LimiterConfig{Rate: 0, Burst: 0},
					},
				},
			},
			isLeader:          false,
			expectErr:         ErrRetryElsewhere,
			expectLog:         true,
			expectMetric:      true,
			expectMetricName:  "rpc.rate_limit.exceeded;limit_type=global/read;op=Foo.Bar;mode=enforcing",
			expectMetricCount: 1,
		},
		"global read limit exceeded but global config entry rate limits priority is True and its exceeded (enforcing, follower)": {
			op: Operation{
				Type:       OperationTypeRead,
				Name:       rpcName,
				SourceAddr: sourceAddr,
			},
			globalMode: ModeEnforcing,
			checks: []limitCheck{
				{limit: globalRead, allow: false},
			},
			glbcfg: &HandlerConfig{
				GlobalLimitConfig: GlobalLimitConfig{
					Mode: ModeEnforcing,
					ReadWriteConfig: ReadWriteConfig{
						ReadConfig: multilimiter.LimiterConfig{Rate: 0, Burst: 0},
					},
				},
			},
			cfg: &structs.GlobalRateLimitConfigEntry{
				Name: "global",
				Config: &structs.GlobalRateLimitConfig{
					Priority:  true,
					WriteRate: float64Ptr(0),
				},
			},
			isLeader:          false,
			expectErr:         ErrRetryElsewhere,
			expectLog:         true,
			expectMetric:      true,
			expectMetricName:  "rpc.rate_limit.exceeded;limit_type=global.configentry/read;op=Foo.Bar;mode=enforcing",
			expectMetricCount: 1,
		},
		"global read limit exceeded but global config entry rate limits priority is false (enforcing, follower)": {
			op: Operation{
				Type:       OperationTypeRead,
				Name:       rpcName,
				SourceAddr: sourceAddr,
			},
			globalMode: ModeEnforcing,
			checks: []limitCheck{
				{limit: globalRead, allow: false},
			},
			glbcfg: &HandlerConfig{
				GlobalLimitConfig: GlobalLimitConfig{
					Mode: ModeEnforcing,
					ReadWriteConfig: ReadWriteConfig{
						ReadConfig: multilimiter.LimiterConfig{Rate: 100, Burst: 0},
					},
				},
			},
			cfg: &structs.GlobalRateLimitConfigEntry{
				Name: "global",
				Config: &structs.GlobalRateLimitConfig{
					Priority:  false,
					WriteRate: float64Ptr(0),
				},
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

			serversStatusProvider := NewMockServersStatusProvider(t)
			serversStatusProvider.On("IsLeader").Return(tc.isLeader).Maybe()
			serversStatusProvider.On("IsServer", mock.Anything).Return(tc.isServer).Maybe()

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
			handler.Register(serversStatusProvider)
			if tc.glbcfg != nil {
				handler.globalCfg.Store(tc.glbcfg)
			}
			handler.globalRateLimitCfg.Store(tc.cfg)

			require.Equal(t, tc.expectErr, handler.Allow(tc.op))

			if tc.expectLog {
				require.Contains(t, output.String(), "RPC exceeded")
			} else {
				require.NotContains(t, output.String(), "RPC exceeded", "expected no rate limit log to be emitted")
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
				rc := multilimiter.LimiterConfig{Rate: cfg.GlobalLimitConfig.ReadConfig.Rate, Burst: cfg.GlobalLimitConfig.ReadConfig.Burst + 1}
				cfg.GlobalLimitConfig.ReadConfig = rc
			},
			assertFunc: func(mockRateLimiter *multilimiter.MockRateLimiter, cfg *HandlerConfig) {
				mockRateLimiter.AssertNumberOfCalls(t, "UpdateConfig", 1)
				mockRateLimiter.AssertCalled(t, "UpdateConfig", cfg.GlobalLimitConfig.ReadConfig, []byte("global.read"))
			},
		},
		{
			description: "RateLimiter gets updated when GlobalWriteConfig changes.",
			configModFunc: func(cfg *HandlerConfig) {
				wc := multilimiter.LimiterConfig{Rate: cfg.GlobalLimitConfig.WriteConfig.Rate, Burst: cfg.GlobalLimitConfig.WriteConfig.Burst + 1}
				cfg.GlobalLimitConfig.WriteConfig = wc
			},
			assertFunc: func(mockRateLimiter *multilimiter.MockRateLimiter, cfg *HandlerConfig) {
				mockRateLimiter.AssertNumberOfCalls(t, "UpdateConfig", 1)
				mockRateLimiter.AssertCalled(t, "UpdateConfig", cfg.GlobalLimitConfig.WriteConfig, []byte("global.write"))
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
			delegate := NewMockServersStatusProvider(t)
			delegate.On("IsLeader").Return(true).Maybe()
			delegate.On("IsServer", mock.Anything).Return(false).Maybe()
			handler := NewHandlerWithLimiter(*tc.cfg, mockRateLimiter, logger)
			handler.Register(delegate)
			addr := net.TCPAddrFromAddrPort(netip.MustParseAddrPort("127.0.0.1:1234"))
			mockRateLimiter.Calls = nil
			handler.Allow(Operation{Name: "test", SourceAddr: addr})
			mockRateLimiter.AssertExpectations(t)
		})
	}
}

func float64Ptr(f float64) *float64 {
	return &f
}

func TestConfigEntryGlobalLimit(t *testing.T) {
	var (
		rpcName    = "Foo.Bar"
		sourceAddr = net.TCPAddrFromAddrPort(netip.MustParseAddrPort("1.2.3.4:5678"))
	)

	type testCase struct {
		description  string
		op           Operation
		cfg          *structs.GlobalRateLimitConfigEntry
		expectNil    bool
		expectMode   Mode
		expectEnt    multilimiter.LimitedEntity
		expectDesc   string
		expectPanic  bool
		expectLogMsg string
	}

	testCases := []testCase{
		{
			description: "exempt operation returns nil",
			op: Operation{
				Type:       OperationTypeExempt,
				Name:       rpcName,
				SourceAddr: sourceAddr,
			},
			cfg: &structs.GlobalRateLimitConfigEntry{
				Name: "global",
				Config: &structs.GlobalRateLimitConfig{
					Priority: true,
					ReadRate: float64Ptr(100),
				},
			},
			expectNil:    true,
			expectLogMsg: "operation exempt from config entry rate limit",
		},
		{
			description: "safety exempt endpoint ConfigEntry.Apply returns nil",
			op: Operation{
				Type:       OperationTypeWrite,
				Name:       "ConfigEntry.Apply",
				SourceAddr: sourceAddr,
			},
			cfg: &structs.GlobalRateLimitConfigEntry{
				Name: "global",
				Config: &structs.GlobalRateLimitConfig{
					Priority:  true,
					WriteRate: float64Ptr(100),
				},
			},
			expectNil:    true,
			expectLogMsg: "operation exempt from config entry rate limit for safety",
		},
		{
			description: "safety exempt endpoint ConfigEntry.Delete returns nil",
			op: Operation{
				Type:       OperationTypeWrite,
				Name:       "ConfigEntry.Delete",
				SourceAddr: sourceAddr,
			},
			cfg: &structs.GlobalRateLimitConfigEntry{
				Name: "global",
				Config: &structs.GlobalRateLimitConfig{
					Priority:  true,
					WriteRate: float64Ptr(100),
				},
			},
			expectNil:    true,
			expectLogMsg: "operation exempt from config entry rate limit for safety",
		},
		{
			description: "nil config returns nil (no rate limit configured)",
			op: Operation{
				Type:       OperationTypeRead,
				Name:       rpcName,
				SourceAddr: sourceAddr,
			},
			cfg:       nil,
			expectNil: true,
		},
		{
			description: "priority disabled still returns limit (priority check moved to Allow)",
			op: Operation{
				Type:       OperationTypeRead,
				Name:       rpcName,
				SourceAddr: sourceAddr,
			},
			cfg: &structs.GlobalRateLimitConfigEntry{
				Name: "global",
				Config: &structs.GlobalRateLimitConfig{
					Priority: false,
					ReadRate: float64Ptr(100),
				},
			},
			expectNil:  false,
			expectMode: ModeEnforcing,
			expectEnt:  configEntryReadLimit,
			expectDesc: "global.configentry/read",
		},
		{
			description: "excluded endpoint bypasses rate limiting",
			op: Operation{
				Type:       OperationTypeRead,
				Name:       "Health.Check",
				SourceAddr: sourceAddr,
			},
			cfg: &structs.GlobalRateLimitConfigEntry{
				Name: "global",
				Config: &structs.GlobalRateLimitConfig{
					Priority:         true,
					ReadRate:         float64Ptr(100),
					ExcludeEndpoints: []string{"Health.Check", "Status.Leader"},
				},
			},
			expectNil:    true,
			expectLogMsg: "operation bypassed rate limiting due to priority endpoint",
		},
		{
			description: "non-excluded endpoint is not bypassed",
			op: Operation{
				Type:       OperationTypeRead,
				Name:       "Catalog.ListServices",
				SourceAddr: sourceAddr,
			},
			cfg: &structs.GlobalRateLimitConfigEntry{
				Name: "global",
				Config: &structs.GlobalRateLimitConfig{
					Priority:         true,
					ReadRate:         float64Ptr(100),
					ExcludeEndpoints: []string{"Health.Check"},
				},
			},
			expectNil:  false,
			expectMode: ModeEnforcing,
			expectEnt:  configEntryReadLimit,
			expectDesc: "global.configentry/read",
		},
		{
			description: "read operation returns read limit",
			op: Operation{
				Type:       OperationTypeRead,
				Name:       rpcName,
				SourceAddr: sourceAddr,
			},
			cfg: &structs.GlobalRateLimitConfigEntry{
				Name: "global",
				Config: &structs.GlobalRateLimitConfig{
					Priority: true,
					ReadRate: float64Ptr(100),
				},
			},
			expectNil:  false,
			expectMode: ModeEnforcing,
			expectEnt:  configEntryReadLimit,
			expectDesc: "global.configentry/read",
		},
		{
			description: "write operation returns write limit",
			op: Operation{
				Type:       OperationTypeWrite,
				Name:       rpcName,
				SourceAddr: sourceAddr,
			},
			cfg: &structs.GlobalRateLimitConfigEntry{
				Name: "global",
				Config: &structs.GlobalRateLimitConfig{
					Priority:  true,
					WriteRate: float64Ptr(200),
				},
			},
			expectNil:  false,
			expectMode: ModeEnforcing,
			expectEnt:  configEntryWriteLimit,
			expectDesc: "global.configentry/write",
		},
		{
			description: "limit always has mode enforcing and applyOnServer true",
			op: Operation{
				Type:       OperationTypeWrite,
				Name:       rpcName,
				SourceAddr: sourceAddr,
			},
			cfg: &structs.GlobalRateLimitConfigEntry{
				Name: "global",
				Config: &structs.GlobalRateLimitConfig{
					Priority:  true,
					WriteRate: float64Ptr(50),
				},
			},
			expectNil:  false,
			expectMode: ModeEnforcing,
			expectEnt:  configEntryWriteLimit,
			expectDesc: "global.configentry/write",
		},
		{
			description: "empty exclude endpoints list does not bypass",
			op: Operation{
				Type:       OperationTypeRead,
				Name:       rpcName,
				SourceAddr: sourceAddr,
			},
			cfg: &structs.GlobalRateLimitConfigEntry{
				Name: "global",
				Config: &structs.GlobalRateLimitConfig{
					Priority:         true,
					ReadRate:         float64Ptr(100),
					ExcludeEndpoints: []string{},
				},
			},
			expectNil:  false,
			expectMode: ModeEnforcing,
			expectEnt:  configEntryReadLimit,
			expectDesc: "global.configentry/read",
		},
		{
			description: "non-safety endpoint is not exempt",
			op: Operation{
				Type:       OperationTypeWrite,
				Name:       "KV.Put",
				SourceAddr: sourceAddr,
			},
			cfg: &structs.GlobalRateLimitConfigEntry{
				Name: "global",
				Config: &structs.GlobalRateLimitConfig{
					Priority:  true,
					WriteRate: float64Ptr(100),
				},
			},
			expectNil:  false,
			expectMode: ModeEnforcing,
			expectEnt:  configEntryWriteLimit,
			expectDesc: "global.configentry/write",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
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

			// Store the config entry
			handler.globalRateLimitCfg.Store(tc.cfg)

			result := handler.configEntryGlobalLimit(tc.op)

			if tc.expectNil {
				require.Nil(t, result)
			} else {
				require.NotNil(t, result)
				require.Equal(t, tc.expectMode, result.mode)
				require.Equal(t, tc.expectEnt, result.ent)
				require.Equal(t, tc.expectDesc, result.desc)
				require.True(t, result.applyOnServer)
			}

			if tc.expectLogMsg != "" {
				require.Contains(t, output.String(), tc.expectLogMsg)
			}
		})
	}
}
