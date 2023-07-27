package hcp

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/armon/go-metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/hcp/client"
)

const defaultTestRefreshInterval = 100 * time.Millisecond

type testConfig struct {
	filters  string
	endpoint string
	labels   map[string]string
}

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
			cfgProvider := NewHCPProviderImpl(ctx, tc.opts.hcpClient)
			if tc.wantErr != "" {
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
				labels: map[string]string{
					"test_label": "123",
				},
				filters: "test",
			},
		},
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
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			// Init global metrics sink.
			serviceName := "test.telemetry_config_provider"
			sink := initGlobalSink(serviceName)

			// Setup client mock to return the expected config.
			mockClient := client.NewMockClient(t)

			mockCfg, err := telemetryConfig(tc.expected)
			require.NoError(t, err)

			mockClient.EXPECT().FetchTelemetryConfig(mock.Anything).Return(mockCfg, nil)

			// Setup TelemetryConfigProvider with opts inputs.
			optsCfg, err := telemetryConfig(tc.optsInputs)
			require.NoError(t, err)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			opts := &providerParams{
				metricsConfig:   optsCfg.MetricsConfig,
				hcpClient:       mockClient,
				refreshInterval: defaultTestRefreshInterval,
			}

			configProvider := NewHCPProviderImpl(ctx, opts.hcpClient)

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

				endpoint, _ := configProvider.GetEndpoint()
				assert.Equal(c, tc.expected.endpoint, endpoint)
				assert.Equal(c, tc.expected.filters, configProvider.GetFilters().String())
				assert.Equal(c, tc.expected.labels, configProvider.GetLabels())
			}, 2*time.Second, defaultTestRefreshInterval, "failed to update telemetry config expected")
		})
	}
}

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

			telemetryConfig, err := telemetryConfig(tc.expected)
			require.NoError(t, err)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			mockClient := client.NewMockClient(t)
			tc.expect(mockClient)

			opts := &providerParams{
				metricsConfig:   telemetryConfig.MetricsConfig,
				hcpClient:       mockClient,
				refreshInterval: defaultTestRefreshInterval,
			}

			configProvider := NewHCPProviderImpl(ctx, opts.hcpClient)

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
				// assert.Equal(c, tc.expected.endpoint, configProvider.GetEndpoint().String())
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

func telemetryConfig(testCfg *testConfig) (*client.TelemetryConfig, error) {
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
			Filters:  filters,
			Labels:   testCfg.labels,
		},
		RefreshConfig: &client.RefreshConfig{
			RefreshInterval: defaultTestRefreshInterval,
		},
	}, nil
}
