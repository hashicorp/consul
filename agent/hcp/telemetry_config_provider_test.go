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

	hcpclient "github.com/hashicorp/consul/agent/hcp/client"
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
		opts    *TelemetryConfigProviderOpts
		wantErr string
	}{
		"success": {
			opts: &TelemetryConfigProviderOpts{
				Ctx:             context.Background(),
				HCPClient:       hcpclient.NewMockClient(t),
				MetricsConfig:   &hcpclient.MetricsConfig{},
				RefreshInterval: 1 * time.Second,
			},
		},
		"failsWithMissingContext": {
			opts: &TelemetryConfigProviderOpts{
				HCPClient:     hcpclient.NewMockClient(t),
				MetricsConfig: &hcpclient.MetricsConfig{},
			},
			wantErr: "missing ctx",
		},
		"failsWithMissingHCPClient": {
			opts: &TelemetryConfigProviderOpts{
				Ctx:           context.Background(),
				MetricsConfig: &hcpclient.MetricsConfig{},
			},
			wantErr: "missing HCP client",
		},
		"failsWithMissingMetricsConfig": {
			opts: &TelemetryConfigProviderOpts{
				Ctx:       context.Background(),
				HCPClient: hcpclient.NewMockClient(t),
			},
			wantErr: "missing metrics config",
		},
		"failsWithInvalidRefreshInterval": {
			opts: &TelemetryConfigProviderOpts{
				Ctx:             context.Background(),
				HCPClient:       hcpclient.NewMockClient(t),
				MetricsConfig:   &hcpclient.MetricsConfig{},
				RefreshInterval: 0 * time.Second,
			},
			wantErr: "invalid refresh interval",
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cfgProvider, err := NewTelemetryConfigProvider(tc.opts)
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
			// Setup client mock to return the expected config.
			mockClient := hcpclient.NewMockClient(t)

			mockCfg, err := telemetryConfig(tc.expected)
			require.NoError(t, err)

			mockClient.EXPECT().FetchTelemetryConfig(mock.Anything).Return(mockCfg, nil)

			// Setup TelemetryConfigProvider with opts inputs.
			optsCfg, err := telemetryConfig(tc.optsInputs)
			require.NoError(t, err)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			opts := &TelemetryConfigProviderOpts{
				MetricsConfig:   optsCfg.MetricsConfig,
				Ctx:             ctx,
				HCPClient:       mockClient,
				RefreshInterval: defaultTestRefreshInterval,
			}

			configProvider, err := NewTelemetryConfigProvider(opts)
			require.NoError(t, err)

			require.EventuallyWithTf(t, func(c *assert.CollectT) {
				assert.Equal(c, tc.expected.endpoint, configProvider.GetEndpoint().String())
				assert.Equal(c, tc.expected.filters, configProvider.GetFilters().String())
				assert.Equal(c, tc.expected.labels, configProvider.GetLabels())
			}, 2*time.Second, defaultTestRefreshInterval, "failed to update telemetry config expected")
		})
	}
}

func TestTelemetryConfigProvider_UpdateFailuresWithMetrics(t *testing.T) {
	for name, tc := range map[string]struct {
		expected *testConfig
		expect   func(*hcpclient.MockClient)
	}{
		"failsWithHCPClientFailure": {
			expected: &testConfig{
				filters: "test",
				labels: map[string]string{
					"test_label": "123",
				},
				endpoint: "http://test.com/v1/metrics",
			},
			expect: func(m *hcpclient.MockClient) {
				m.EXPECT().FetchTelemetryConfig(mock.Anything).Return(nil, fmt.Errorf("failure"))
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			// Init global metrics sink.
			serviceName := "test.telemetry_config_provider"
			cfg := metrics.DefaultConfig(serviceName)
			cfg.EnableHostname = false

			sink := metrics.NewInmemSink(10*time.Second, 10*time.Second)
			metrics.NewGlobal(cfg, sink)

			telemetryConfig, err := telemetryConfig(tc.expected)
			require.NoError(t, err)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			mockClient := hcpclient.NewMockClient(t)
			tc.expect(mockClient)

			opts := &TelemetryConfigProviderOpts{
				Ctx:             ctx,
				MetricsConfig:   telemetryConfig.MetricsConfig,
				HCPClient:       mockClient,
				RefreshInterval: defaultTestRefreshInterval,
			}

			configProvider, err := NewTelemetryConfigProvider(opts)
			require.NoError(t, err)

			// Eventually tries to run assertions every 100 ms to verify
			// if failure metrics and the dynamic config have been updated as expected.
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
			}, 2*time.Second, defaultTestRefreshInterval, "failed to get expected failure metrics")
		})
	}
}

func telemetryConfig(testCfg *testConfig) (*hcpclient.TelemetryConfig, error) {
	filters, err := regexp.Compile(testCfg.filters)
	if err != nil {
		return nil, err
	}

	endpoint, err := url.Parse(testCfg.endpoint)
	if err != nil {
		return nil, err
	}
	return &hcpclient.TelemetryConfig{
		MetricsConfig: &hcpclient.MetricsConfig{
			Endpoint: endpoint,
			Filters:  filters,
			Labels:   testCfg.labels,
		},
		RefreshConfig: &hcpclient.RefreshConfig{
			RefreshInterval: defaultTestRefreshInterval,
		},
	}, nil
}
