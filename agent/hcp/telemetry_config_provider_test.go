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
	"github.com/hashicorp/consul/agent/hcp/client"
	"github.com/hashicorp/consul/agent/hcp/config"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestTelemetryConfigProvider_Sucess(t *testing.T) {
	for name, test := range map[string]struct {
		filters      string
		endpoint     string
		labels       map[string]string
		telemetryCfg *client.TelemetryConfig
	}{
		"SuccessNoChanges": {
			filters: "test",
			labels: map[string]string{
				"test_label": "123",
			},
			endpoint: "http://test.com/v1/metrics",
			telemetryCfg: &client.TelemetryConfig{
				Endpoint: "http://test.com/v1/metrics",
				Labels: map[string]string{
					"test_label": "123",
				},
				MetricsConfig: &client.MetricsConfig{
					Filters: []string{"test"},
				},
			},
		},
		"successNewLabels": {
			filters: "test",
			labels: map[string]string{
				"test_label": "123",
			},
			endpoint: "http://test.com/v1/metrics",
			telemetryCfg: &client.TelemetryConfig{
				Endpoint: "http://test.com/v1/metrics",
				Labels: map[string]string{
					"new_label": "1234",
				},
				MetricsConfig: &client.MetricsConfig{
					Filters: []string{"test"},
				},
			},
		},
		"successNewEndpoint": {
			filters: "test",
			labels: map[string]string{
				"test_label": "123",
			},
			endpoint: "http://test.com/v1/metrics",
			telemetryCfg: &client.TelemetryConfig{
				Endpoint: "http://newendpoint.com",
				Labels: map[string]string{
					"test_label": "123",
				},
				MetricsConfig: &client.MetricsConfig{
					Filters: []string{"test"},
				},
			},
		},
		"successNewFilters": {
			filters: "test",
			labels: map[string]string{
				"test_label": "123",
			},
			endpoint: "http://test.com/v1/metrics",
			telemetryCfg: &client.TelemetryConfig{
				Endpoint: "http://test.com",
				Labels: map[string]string{
					"new_label": "1234",
				},
				MetricsConfig: &client.MetricsConfig{
					Filters: []string{"consul"},
				},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			filters, err := regexp.Compile(test.filters)
			require.NoError(t, err)

			endpoint, err := url.Parse(test.endpoint)
			require.NoError(t, err)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			cloudCfg := config.CloudConfig{
				NodeID:   "test_node_id",
				NodeName: "test_node_name",
			}

			mockClient := client.NewMockClient(t)
			mockClient.EXPECT().FetchTelemetryConfig(mock.Anything).Return(test.telemetryCfg, nil)

			opts := &TelemetryConfigProviderOpts{
				ctx:             ctx,
				filters:         filters,
				endpoint:        endpoint,
				labels:          test.labels,
				cloudCfg:        cloudCfg,
				hcpClient:       mockClient,
				refreshInterval: 1 * time.Second,
			}

			configProvider := NewTelemetryConfigProvider(opts)

			// TODO: Can I use a test chan to avoid time.Sleep.
			time.Sleep(2 * time.Second)

			expectedEndpoint, _ := test.telemetryCfg.Enabled()
			expectedEndpointURL, err := url.Parse(expectedEndpoint)
			require.NoError(t, err)

			expectedFilters := test.telemetryCfg.MetricsConfig.Filters
			expectedFiltersRegex, err := regexp.Compile(strings.Join(expectedFilters, "|"))
			require.NoError(t, err)

			expectedLabels := test.telemetryCfg.DefaultLabels(cloudCfg)

			require.Equal(t, expectedEndpointURL, configProvider.GetEndpoint())
			require.Equal(t, expectedFiltersRegex.String(), configProvider.GetFilters().String())
			require.Equal(t, expectedLabels, configProvider.GetLabels())
		})
	}
}

func TestTelemetryConfigProvider_UpdateFailuresWithMetrics(t *testing.T) {
	for name, test := range map[string]struct {
		filters  string
		endpoint string
		labels   map[string]string
		expect   func(*client.MockClient)
	}{
		"failsWithInvalidFilters": {
			filters: "test",
			labels: map[string]string{
				"test_label": "123",
			},
			endpoint: "http://test.com/v1/metrics",
			expect: func(m *client.MockClient) {
				t := &client.TelemetryConfig{
					Endpoint: "http://test.com",
					Labels: map[string]string{
						"new_label": "1234",
					},
					MetricsConfig: &client.MetricsConfig{
						Filters: []string{"["},
					},
				}
				m.EXPECT().FetchTelemetryConfig(mock.Anything).Return(t, nil)
			},
		},
		"failsWithInvalidURL": {
			filters: "test",
			labels: map[string]string{
				"test_label": "123",
			},
			endpoint: "http://test.com/v1/metrics",
			expect: func(m *client.MockClient) {
				t := &client.TelemetryConfig{
					Endpoint: "			",
					Labels: map[string]string{
						"new_label": "1234",
					},
					MetricsConfig: &client.MetricsConfig{
						Filters: []string{"test"},
					},
				}
				m.EXPECT().FetchTelemetryConfig(mock.Anything).Return(t, nil)
			},
		},
		"failsWithHCPClientFailure": {
			filters: "test",
			labels: map[string]string{
				"test_label": "123",
			},
			endpoint: "http://test.com/v1/metrics",
			expect: func(m *client.MockClient) {
				m.EXPECT().FetchTelemetryConfig(mock.Anything).Return(nil, fmt.Errorf("failure"))
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			filters, err := regexp.Compile(test.filters)
			require.NoError(t, err)

			endpoint, err := url.Parse(test.endpoint)
			require.NoError(t, err)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			cloudCfg := config.CloudConfig{
				NodeID:   "test_node_id",
				NodeName: "test_node_name",
			}

			mockClient := client.NewMockClient(t)
			test.expect(mockClient)

			opts := &TelemetryConfigProviderOpts{
				ctx:             ctx,
				filters:         filters,
				endpoint:        endpoint,
				labels:          test.labels,
				cloudCfg:        cloudCfg,
				hcpClient:       mockClient,
				refreshInterval: 1 * time.Second,
			}

			// Init global sink.
			serviceName := "test.telemetry_config_provider"
			cfg := metrics.DefaultConfig(serviceName)
			cfg.EnableHostname = false

			sink := metrics.NewInmemSink(10*time.Second, 10*time.Second)
			metrics.NewGlobal(cfg, sink)

			configProvider := NewTelemetryConfigProvider(opts)

			time.Sleep(2 * time.Second)

			require.Equal(t, endpoint, configProvider.GetEndpoint())
			require.Equal(t, filters.String(), configProvider.GetFilters().String())
			require.Equal(t, test.labels, configProvider.GetLabels())

			// Collect sink metrics.
			intervals := sink.Data()
			require.Len(t, intervals, 1)
			key := serviceName + "." + strings.Join(internalMetricRefreshFailure, ".")
			sv := intervals[0].Counters[key]

			// Verify count for transform failure metric.
			require.NotNil(t, sv)
			require.NotNil(t, sv.AggregateSample)
			require.GreaterOrEqual(t, 1, sv.AggregateSample.Count)
		})
	}
}

func TestLabelsChanged(t *testing.T) {
	for name, test := range map[string]struct {
		newLabels      map[string]string
		oldLabels      map[string]string
		expectedChange bool
	}{
		"noChange": {
			newLabels:      map[string]string{"key1": "test1"},
			oldLabels:      map[string]string{"key1": "test1"},
			expectedChange: false,
		},
		"newLabelsNewKey": {
			newLabels:      map[string]string{"key2": "test1"},
			oldLabels:      map[string]string{"key1": "test1"},
			expectedChange: true,
		},
		"newLabelsSameKey": {
			newLabels:      map[string]string{"key1": "test2"},
			oldLabels:      map[string]string{"key1": "test1"},
			expectedChange: true,
		},
		"newLabelsLonger": {
			newLabels:      map[string]string{"key1": "test1", "key2": "test2", "key3": "test3"},
			oldLabels:      map[string]string{"key1": "test1"},
			expectedChange: true,
		},
		"newLabelsShorter": {
			newLabels:      map[string]string{"key1": "test1", "key2": "test2"},
			oldLabels:      map[string]string{"key1": "test1", "key2": "test2", "key3": "test3"},
			expectedChange: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, test.expectedChange, labelsChanged(test.newLabels, test.oldLabels))
		})
	}
}

// TODO: Add race test
