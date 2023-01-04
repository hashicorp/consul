package rate

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/armon/go-metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/go-hclog"

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
			expectMetricName:  "consul.rate_limit;limit_type=global/write;op=Foo.Bar;mode=permissive",
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
			expectMetricName:  "consul.rate_limit;limit_type=global/write;op=Foo.Bar;mode=enforcing",
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
			expectMetricName:  "consul.rate_limit;limit_type=global/write;op=Foo.Bar;mode=enforcing",
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
			expectMetricName:  "consul.rate_limit;limit_type=global/read;op=Foo.Bar;mode=permissive",
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
			expectMetricName:  "consul.rate_limit;limit_type=global/read;op=Foo.Bar;mode=enforcing",
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
			expectMetricName:  "consul.rate_limit;limit_type=global/read;op=Foo.Bar;mode=enforcing",
			expectMetricCount: 1,
		},
	}
	for desc, tc := range testCases {
		t.Run(desc, func(t *testing.T) {
			// Setup metrics to test they are recorded
			sink := testSetupMetrics(t)

			limiter := newMockLimiter(t)
			limiter.On("UpdateConfig", mock.Anything, mock.Anything).Return()
			for _, c := range tc.checks {
				limiter.On("Allow", c.limit).Return(c.allow)
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
					GlobalMode: tc.globalMode,
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
				assertCounter(t, sink, tc.expectMetricName, tc.expectMetricCount)
			}
		})
	}
}

func assertCounter(t *testing.T, sink *metrics.InmemSink, name string, value float64) {
	t.Helper()

	data := sink.Data()

	var got float64
	for _, intv := range data {
		intv.RLock()
		// Note that InMemSink uses SampledValue and treats the _Sum_ not the Count
		// as the entire value.
		if sample, ok := intv.Counters[name]; ok {
			got += sample.Sum
		}
		intv.RUnlock()
	}

	if !assert.Equal(t, value, got) {
		// no nice way to dump this - this is copied from private method in
		// InMemSink used for dumping to stdout on SIGUSR1.
		buf := bytes.NewBuffer(nil)
		for _, intv := range data {
			intv.RLock()
			for name, val := range intv.Gauges {
				fmt.Fprintf(buf, "[%v][G] '%s': %0.3f\n", intv.Interval, name, val.Value)
			}
			for name, vals := range intv.Points {
				for _, val := range vals {
					fmt.Fprintf(buf, "[%v][P] '%s': %0.3f\n", intv.Interval, name, val)
				}
			}
			for name, agg := range intv.Counters {
				fmt.Fprintf(buf, "[%v][C] '%s': %s\n", intv.Interval, name, agg.AggregateSample)
			}
			for name, agg := range intv.Samples {
				fmt.Fprintf(buf, "[%v][S] '%s': %s\n", intv.Interval, name, agg.AggregateSample)
			}
			intv.RUnlock()
		}
		t.Log(buf.String())
	}
}

func testSetupMetrics(t *testing.T) *metrics.InmemSink {
	// Record for ages (5 mins) so we can be confident that our assertions won't
	// fail on silly long test runs due to dropped data.
	s := metrics.NewInmemSink(10*time.Second, 300*time.Second)
	cfg := metrics.DefaultConfig("")
	cfg.EnableHostname = false
	cfg.EnableRuntimeMetrics = false
	metrics.NewGlobal(cfg, s)
	return s
}

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
	NewHandlerWithLimiter(*cfg, mockRateLimiter, logger)
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
			delegate := NewMockLeaderStatusProvider(t)
			delegate.On("IsLeader").Return(true).Maybe()
			handler := NewHandlerWithLimiter(*tc.cfg, mockRateLimiter, logger)
			handler.Register(delegate)
			addr := net.TCPAddrFromAddrPort(netip.MustParseAddrPort("127.0.0.1:1234"))
			mockRateLimiter.Calls = nil
			handler.Allow(Operation{Name: "test", SourceAddr: addr})
			mockRateLimiter.AssertNumberOfCalls(t, "Allow", tc.expectedAllowCalls)
		})
	}
}

var _ multilimiter.RateLimiter = (*mockLimiter)(nil)

func newMockLimiter(t *testing.T) *mockLimiter {
	l := &mockLimiter{}
	l.Mock.Test(t)

	t.Cleanup(func() { l.AssertExpectations(t) })

	return l
}

type mockLimiter struct {
	mock.Mock
}

func (m *mockLimiter) Allow(v multilimiter.LimitedEntity) bool { return m.Called(v).Bool(0) }
func (m *mockLimiter) Run(ctx context.Context)                 { m.Called(ctx) }
func (m *mockLimiter) UpdateConfig(cfg multilimiter.LimiterConfig, prefix []byte) {
	m.Called(cfg, prefix)
}
