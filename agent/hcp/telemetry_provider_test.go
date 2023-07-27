package hcp

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
<<<<<<< HEAD
	"testing"
	"time"

=======
	"strings"
	"testing"
	"time"

	"github.com/armon/go-metrics"
	"github.com/stretchr/testify/assert"
>>>>>>> cc-4960/hcp-telemetry-periodic-refresh
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/hcp/client"
)

const defaultTestRefreshInterval = 100 * time.Millisecond

type testConfig struct {
	filters         string
<<<<<<< HEAD
	endpoint        *url.URL
=======
	endpoint        string
>>>>>>> cc-4960/hcp-telemetry-periodic-refresh
	labels          map[string]string
	refreshInterval time.Duration
}

func TestDynamicConfigEquals(t *testing.T) {
	t.Parallel()
<<<<<<< HEAD

=======
>>>>>>> cc-4960/hcp-telemetry-periodic-refresh
	for name, tc := range map[string]struct {
		a        *testConfig
		b        *testConfig
		expected bool
	}{
		"same": {
			a: &testConfig{
<<<<<<< HEAD
				endpoint: &url.URL{
					Host: "http://test.com/v1/metrics",
				},
=======
				endpoint:        "test.com",
>>>>>>> cc-4960/hcp-telemetry-periodic-refresh
				filters:         "state|raft",
				labels:          map[string]string{"test": "123"},
				refreshInterval: 1 * time.Second,
			},
			b: &testConfig{
<<<<<<< HEAD
				endpoint: &url.URL{
					Host: "http://test.com/v1/metrics",
				},
=======
				endpoint:        "test.com",
>>>>>>> cc-4960/hcp-telemetry-periodic-refresh
				filters:         "state|raft",
				labels:          map[string]string{"test": "123"},
				refreshInterval: 1 * time.Second,
			},
			expected: true,
		},
		"different": {
			a: &testConfig{
<<<<<<< HEAD
				endpoint: &url.URL{
					Host: "http://test.com/v1/metrics",
				},
=======
				endpoint:        "newendpoint.com",
>>>>>>> cc-4960/hcp-telemetry-periodic-refresh
				filters:         "state|raft|extra",
				labels:          map[string]string{"test": "12334"},
				refreshInterval: 2 * time.Second,
			},
			b: &testConfig{
<<<<<<< HEAD
				endpoint: &url.URL{
					Host: "http://other-endpoint-test.com/v1/metrics",
				},
=======
				endpoint:        "test.com",
>>>>>>> cc-4960/hcp-telemetry-periodic-refresh
				filters:         "state|raft",
				labels:          map[string]string{"test": "123"},
				refreshInterval: 1 * time.Second,
			},
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			aCfg, err := testDynamicCfg(tc.a)
			require.NoError(t, err)
			bCfg, err := testDynamicCfg(tc.b)
			require.NoError(t, err)

			equal, err := aCfg.equals(bCfg)

			require.NoError(t, err)
			require.Equal(t, tc.expected, equal)
		})
	}
}

<<<<<<< HEAD
func TestTelemetryConfigProvider(t *testing.T) {
	for name, tc := range map[string]struct {
		expected *testConfig
		fail     string
	}{
		"validConfig": {
			expected: &testConfig{
				endpoint: &url.URL{
					Host: "http://test.com/v1/metrics",
				},
=======
func TestNewTelemetryConfigProvider(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		opts    *providerParams
		wantErr string
	}{
		"success": {
			opts: &providerParams{
				hcpClient:       client.NewMockClient(t),
				metricsConfig:   &client.MetricsConfig{},
				refreshInterval: 1 * time.Second,
			},
		},
		"failsWithMissingHCPClient": {
			opts: &providerParams{
				metricsConfig: &client.MetricsConfig{},
			},
			wantErr: "missing HCP client",
		},
		"failsWithMissingMetricsConfig": {
			opts: &providerParams{
				hcpClient: client.NewMockClient(t),
			},
			wantErr: "missing metrics config",
		},
		"failsWithInvalidRefreshInterval": {
			opts: &providerParams{
				hcpClient:       client.NewMockClient(t),
				metricsConfig:   &client.MetricsConfig{},
				refreshInterval: 0 * time.Second,
			},
			wantErr: "invalid refresh interval",
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			cfgProvider, err := NewHCPProvider(ctx, tc.opts)
			if tc.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErr)
				require.Nil(t, cfgProvider)
				return
			}
			require.NotNil(t, cfgProvider)
		})
	}
}

func TestTelemetryConfigProvider_Success(t *testing.T) {
	for name, tc := range map[string]struct {
		optsInputs *testConfig
		expected   *testConfig
	}{
		"noChanges": {
			optsInputs: &testConfig{
				endpoint: "http://test.com/v1/metrics",
				filters:  "test",
				labels: map[string]string{
					"test_label": "123",
				},
			},
			expected: &testConfig{
				endpoint: "http://test.com/v1/metrics",
>>>>>>> cc-4960/hcp-telemetry-periodic-refresh
				labels: map[string]string{
					"test_label": "123",
				},
				filters: "test",
			},
		},
<<<<<<< HEAD
		"hcpFailure": {
			fail: "client",
		},
		"hcpFailureOnce": {
			fail: "clientOnce",
			expected: &testConfig{
				endpoint: &url.URL{
					Host: "http://test.com/v1/metrics",
				},
				labels: map[string]string{
					"test_label": "123",
				},
				filters: "test",
=======
		"newConfig": {
			optsInputs: &testConfig{
				endpoint: "http://test.com/v1/metrics",
				filters:  "test",
				labels: map[string]string{
					"test_label": "123",
				},
			},
			expected: &testConfig{
				endpoint: "http://newendpoint/v1/metrics",
				filters:  "consul",
				labels: map[string]string{
					"new_label": "1234",
				},
>>>>>>> cc-4960/hcp-telemetry-periodic-refresh
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
<<<<<<< HEAD
			// Setup client mock to return the expected config.
			mockClient := client.NewMockClient(t)

			// Set up mocks.
			func() {
				if tc.fail == "client" {
					mockClient.EXPECT().FetchTelemetryConfig(mock.Anything).Return(nil, fmt.Errorf("failure"))
					return
				}

				if tc.fail == "clientOnce" {
					// Fail the first call to HCP. Tests a start up failure.
					mockClient.EXPECT().FetchTelemetryConfig(mock.Anything).Once().Return(nil, fmt.Errorf("failure"))
				}

				mockCfg, err := newTestTelemetryConfig(t, tc.expected)
				require.NoError(t, err)

				if tc.fail == "contextCancel" {
					// If context cancelled, only a single fetch/update is expected.
					mockClient.EXPECT().FetchTelemetryConfig(mock.Anything).Times(2).Return(nil, fmt.Errorf("failure"))
					return
				}

				mockClient.EXPECT().FetchTelemetryConfig(mock.Anything).Return(mockCfg, nil)
			}()

			// Create a channel to manually execute updates.
			ticker := make(chan time.Time)

			// Create a context, cancel immediately if testing context cancellation.
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Create the provider.
			configProvider := newHCPProviderImpl(ctx, mockClient, ticker)

			// Trigger updates of config.
			ticker <- time.Now()
			ticker <- time.Now()

			// If HCP call fails, expect empty config values.
			if tc.expected == nil {
				endpoint, ok := configProvider.GetEndpoint()
				require.False(t, ok)
				require.Nil(t, endpoint)
				require.Equal(t, defaultTelemetryConfigFilters.String(), configProvider.GetFilters().String())
				require.Empty(t, configProvider.GetLabels())
				return
			}

			endpoint, ok := configProvider.GetEndpoint()
			require.True(t, ok)
			require.Equal(t, tc.expected.endpoint, endpoint)
			require.Equal(t, tc.expected.filters, configProvider.GetFilters().String())
			require.Equal(t, tc.expected.labels, configProvider.GetLabels())
=======
			// Init global metrics sink.
			serviceName := "test.telemetry_config_provider"
			sink := initGlobalSink(serviceName)

			// Setup client mock to return the expected config.
			mockClient := client.NewMockClient(t)

			mockCfg, err := testTelemetryCfg(tc.expected)
			require.NoError(t, err)

			mockClient.EXPECT().FetchTelemetryConfig(mock.Anything).Return(mockCfg, nil)

			// Setup TelemetryConfigProvider with opts inputs.
			optsCfg, err := testTelemetryCfg(tc.optsInputs)
			require.NoError(t, err)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			opts := &providerParams{
				metricsConfig:   optsCfg.MetricsConfig,
				hcpClient:       mockClient,
				refreshInterval: defaultTestRefreshInterval,
			}

			configProvider, err := NewHCPProvider(ctx, opts)
			require.NoError(t, err)

			// TODO: Test this by having access to the ticker directly.
			require.EventuallyWithTf(t, func(c *assert.CollectT) {
				// Collect sink metrics.
				key := serviceName + "." + strings.Join(internalMetricRefreshSuccess, ".")
				intervals := sink.Data()
				sv := intervals[0].Counters[key]

				// Verify count for transform failure metric.
				assert.NotNil(c, sv.AggregateSample)
				// Check for nil, as in some eventually ticks, the AggregateSample isn't populated yet.
				if sv.AggregateSample != nil {
					assert.GreaterOrEqual(c, sv.AggregateSample.Count, 1)
				}

				assert.Equal(c, tc.expected.endpoint, configProvider.GetEndpoint().String())
				assert.Equal(c, tc.expected.filters, configProvider.GetFilters().String())
				assert.Equal(c, tc.expected.labels, configProvider.GetLabels())
			}, 2*time.Second, defaultTestRefreshInterval, "failed to update telemetry config expected")
>>>>>>> cc-4960/hcp-telemetry-periodic-refresh
		})
	}
}

<<<<<<< HEAD
=======
func TestTelemetryConfigProvider_UpdateFailuresWithMetrics(t *testing.T) {
	for name, tc := range map[string]struct {
		expected *testConfig
		expect   func(*client.MockClient)
	}{
		"failsWithHCPClientFailure": {
			expected: &testConfig{
				filters: "test",
				labels: map[string]string{
					"test_label": "123",
				},
				endpoint: "http://test.com/v1/metrics",
			},
			expect: func(m *client.MockClient) {
				m.EXPECT().FetchTelemetryConfig(mock.Anything).Return(nil, fmt.Errorf("failure"))
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			// Init global metrics sink.
			serviceName := "test.telemetry_config_provider"
			sink := initGlobalSink(serviceName)

			testTelemetryCfg, err := testTelemetryCfg(tc.expected)
			require.NoError(t, err)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			mockClient := client.NewMockClient(t)
			tc.expect(mockClient)

			opts := &providerParams{
				metricsConfig:   testTelemetryCfg.MetricsConfig,
				hcpClient:       mockClient,
				refreshInterval: defaultTestRefreshInterval,
			}

			configProvider, err := NewHCPProvider(ctx, opts)
			require.NoError(t, err)

			// Eventually tries to run assertions every 100 ms to verify
			// if failure metrics and the dynamic config have been updated as expected.
			// TODO: Use ticker directly to test this.
			require.EventuallyWithTf(t, func(c *assert.CollectT) {
				// Collect sink metrics.
				key := serviceName + "." + strings.Join(internalMetricRefreshFailure, ".")
				intervals := sink.Data()
				sv := intervals[0].Counters[key]

				// Verify count for transform failure metric.
				assert.NotNil(c, sv.AggregateSample)
				// Check for nil, as in some eventually ticks, the AggregateSample isn't populated yet.
				if sv.AggregateSample != nil {
					assert.GreaterOrEqual(c, sv.AggregateSample.Count, 1)
				}

				// Upon failures, config should not have changed.
				assert.Equal(c, tc.expected.endpoint, configProvider.GetEndpoint().String())
				assert.Equal(c, tc.expected.filters, configProvider.GetFilters().String())
				assert.Equal(c, tc.expected.labels, configProvider.GetLabels())
			}, 3*time.Second, defaultTestRefreshInterval, "failed to get expected failure metrics")
		})
	}
}

// TODO: Add race test.

func initGlobalSink(serviceName string) *metrics.InmemSink {
	cfg := metrics.DefaultConfig(serviceName)
	cfg.EnableHostname = false

	sink := metrics.NewInmemSink(10*time.Second, 10*time.Second)
	metrics.NewGlobal(cfg, sink)

	return sink
}

>>>>>>> cc-4960/hcp-telemetry-periodic-refresh
func testDynamicCfg(testCfg *testConfig) (*dynamicConfig, error) {
	filters, err := regexp.Compile(testCfg.filters)
	if err != nil {
		return nil, err
	}

<<<<<<< HEAD
	return &dynamicConfig{
		Endpoint:        testCfg.endpoint,
=======
	endpoint, err := url.Parse(testCfg.endpoint)
	if err != nil {
		return nil, err
	}
	return &dynamicConfig{
		Endpoint:        endpoint,
>>>>>>> cc-4960/hcp-telemetry-periodic-refresh
		Filters:         filters,
		Labels:          testCfg.labels,
		RefreshInterval: testCfg.refreshInterval,
	}, nil
}

<<<<<<< HEAD
func newTestTelemetryConfig(t *testing.T, testCfg *testConfig) (*client.TelemetryConfig, error) {
	t.Helper()

	filters, err := regexp.Compile(testCfg.filters)
	if err != nil {
		return nil, fmt.Errorf("failed to compile test filters: %v", filters)
	}

	return &client.TelemetryConfig{
		MetricsConfig: &client.MetricsConfig{
			Endpoint: testCfg.endpoint,
=======
func testTelemetryCfg(testCfg *testConfig) (*client.TelemetryConfig, error) {
	filters, err := regexp.Compile(testCfg.filters)
	if err != nil {
		return nil, err
	}

	endpoint, err := url.Parse(testCfg.endpoint)
	if err != nil {
		return nil, err
	}
	return &client.TelemetryConfig{
		MetricsConfig: &client.MetricsConfig{
			Endpoint: endpoint,
>>>>>>> cc-4960/hcp-telemetry-periodic-refresh
			Filters:  filters,
			Labels:   testCfg.labels,
		},
		RefreshConfig: &client.RefreshConfig{
			RefreshInterval: defaultTestRefreshInterval,
		},
	}, nil
}
