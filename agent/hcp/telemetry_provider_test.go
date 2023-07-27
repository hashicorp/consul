package hcp

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/armon/go-metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/hcp/client"
)

const (
	testRefreshInterval = 100 * time.Millisecond
	testSinkServiceName = "test.telemetry_config_provider"
	testRaceSampleCount = 5000
)

type testConfig struct {
	filters         string
	endpoint        string
	labels          map[string]string
	refreshInterval time.Duration
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

func TestTelemetryConfigProvider(t *testing.T) {
	for name, tc := range map[string]struct {
		mockExpect func(*client.MockClient)
		metricKey  []string
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
				refreshInterval: testRefreshInterval,
			},
			expected: &testConfig{
				endpoint: "http://test.com/v1/metrics",
				labels: map[string]string{
					"test_label": "123",
				},
				filters: "test",
			},
			metricKey: internalMetricRefreshSuccess,
		},
		"newConfig": {
			optsInputs: &testConfig{
				endpoint: "http://test.com/v1/metrics",
				filters:  "test",
				labels: map[string]string{
					"test_label": "123",
				},
				refreshInterval: 2 * time.Second,
			},
			expected: &testConfig{
				endpoint: "http://newendpoint/v1/metrics",
				filters:  "consul",
				labels: map[string]string{
					"new_label": "1234",
				},
				refreshInterval: 2 * time.Second,
			},
			metricKey: internalMetricRefreshSuccess,
		},
		"sameConfigHCPClientFailure": {
			optsInputs: &testConfig{
				endpoint: "http://test.com/v1/metrics",
				filters:  "test",
				labels: map[string]string{
					"test_label": "123",
				},
				refreshInterval: testRefreshInterval,
			},
			mockExpect: func(m *client.MockClient) {
				m.EXPECT().FetchTelemetryConfig(mock.Anything).Return(nil, fmt.Errorf("failure"))
			},
			expected: &testConfig{
				endpoint: "http://test.com/v1/metrics",
				filters:  "test",
				labels: map[string]string{
					"test_label": "123",
				},
				refreshInterval: testRefreshInterval,
			},
			metricKey: internalMetricRefreshFailure,
		},
	} {
		t.Run(name, func(t *testing.T) {
			sink := initGlobalSink()
			mockClient := client.NewMockClient(t)
			if tc.mockExpect != nil {
				tc.mockExpect(mockClient)
			} else {
				mockCfg, err := testTelemetryCfg(tc.expected)
				require.NoError(t, err)
				mockClient.EXPECT().FetchTelemetryConfig(mock.Anything).Return(mockCfg, nil)
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			dynamicCfg, err := testDynamicCfg(tc.optsInputs)
			require.NoError(t, err)

			provider := &hcpProviderImpl{
				hcpClient: mockClient,
				cfg:       dynamicCfg,
				ticker:    time.NewTicker(testRefreshInterval),
			}

			// Use a time chan to trigger updates manually.
			timeChan := make(chan time.Time, 1)

			go provider.run(ctx, timeChan)

			// Send to the channel twice to ensure we hit the case statement
			// and once again to ensure the getUpdate() function executed.
			timeChan <- time.Now()
			timeChan <- time.Now()

			require.EventuallyWithTf(t, func(collect *assert.CollectT) {
				// Verify endpoint provider returns correct config values.
				assert.Equal(collect, tc.expected.endpoint, provider.GetEndpoint().String())
				assert.Equal(collect, tc.expected.filters, provider.GetFilters().String())
				assert.Equal(collect, tc.expected.labels, provider.GetLabels())

				// Verify count for transform success metric.
				sv := collectSinkMetric(sink, tc.metricKey)
				assert.NotNil(t, sv.AggregateSample)
				if sv.AggregateSample != nil {
					require.GreaterOrEqual(t, sv.AggregateSample.Count, 1)
				}
			}, 5*time.Second, 100*time.Millisecond, "failed to get update in time")
		})
	}
}

func TestDynamicConfigEquals(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		a        *testConfig
		b        *testConfig
		expected bool
	}{
		"same": {
			a: &testConfig{
				endpoint:        "test.com",
				filters:         "state|raft",
				labels:          map[string]string{"test": "123"},
				refreshInterval: 1 * time.Second,
			},
			b: &testConfig{
				endpoint:        "test.com",
				filters:         "state|raft",
				labels:          map[string]string{"test": "123"},
				refreshInterval: 1 * time.Second,
			},
			expected: true,
		},
		"different": {
			a: &testConfig{
				endpoint:        "newendpoint.com",
				filters:         "state|raft|extra",
				labels:          map[string]string{"test": "12334"},
				refreshInterval: 2 * time.Second,
			},
			b: &testConfig{
				endpoint:        "test.com",
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

// mockClientRace returns new configuration everytime checkUpdate is called
// by creating unique labels using its counter. This allows us to induce
// race conditions with the changing dynamic config in the provider.
type mockClientRace struct {
	counter         int
	defaultEndpoint *url.URL
	defaultFilters  *regexp.Regexp
}

func (mc *mockClientRace) FetchBootstrap(ctx context.Context) (*client.BootstrapConfig, error) {
	return nil, nil
}
func (mc *mockClientRace) PushServerStatus(ctx context.Context, status *client.ServerStatus) error {
	return nil
}
func (mc *mockClientRace) DiscoverServers(ctx context.Context) ([]string, error) {
	return nil, nil
}
func (mc *mockClientRace) FetchTelemetryConfig(ctx context.Context) (*client.TelemetryConfig, error) {
	return &client.TelemetryConfig{
		MetricsConfig: &client.MetricsConfig{
			Endpoint: mc.defaultEndpoint,
			Filters:  mc.defaultFilters,
			// Generate unique labels.
			Labels: map[string]string{fmt.Sprintf("label_%d", mc.counter): fmt.Sprintf("value_%d", mc.counter)},
		},
		RefreshConfig: &client.RefreshConfig{
			RefreshInterval: testRefreshInterval,
		},
	}, nil
}

func TestTelemetryConfigProvider_Race(t *testing.T) {
	initialTelemetryCfg, err := testTelemetryCfg(&testConfig{
		endpoint: "http://test.com/v1/metrics",
		filters:  "test",
		labels: map[string]string{
			"test_label": "123",
		},
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	provider, err := NewHCPProvider(ctx, &providerParams{
		metricsConfig: initialTelemetryCfg.MetricsConfig,
		hcpClient: &mockClientRace{
			defaultEndpoint: initialTelemetryCfg.MetricsConfig.Endpoint,
			defaultFilters:  initialTelemetryCfg.MetricsConfig.Filters,
		},
		refreshInterval: testRefreshInterval,
	})
	require.NoError(t, err)

	wg := &sync.WaitGroup{}

	labelErrCh := make(chan error, testRaceSampleCount)
	labelErr := errors.New("expected labels to have one entry")
	// Start 5000 goroutines that try to access label configuration.
	kickOff(wg, labelErrCh, provider, func(provider *hcpProviderImpl) bool {
		return len(provider.GetLabels()) == 1
	}, labelErr)

	expectedEndpoint := initialTelemetryCfg.MetricsConfig.Endpoint.String()
	endpointErr := fmt.Errorf("expected endpoint to be %s", expectedEndpoint)
	endpointErrCh := make(chan error, testRaceSampleCount)
	// Start 5000 goroutines that try to access endpoint configuration.
	kickOff(wg, endpointErrCh, provider, func(provider *hcpProviderImpl) bool {
		return provider.GetEndpoint().String() == expectedEndpoint
	}, endpointErr)

	expectedFilters := initialTelemetryCfg.MetricsConfig.Filters.String()
	filtersErr := fmt.Errorf("expected filters to be %s", expectedFilters)
	filtersErrCh := make(chan error, testRaceSampleCount)
	// Start 5000 goroutines that try to access filter configuration.
	kickOff(wg, filtersErrCh, provider, func(provider *hcpProviderImpl) bool {
		return provider.GetFilters().String() == expectedFilters
	}, filtersErr)

	wg.Wait()

	require.Empty(t, labelErrCh)
	require.Empty(t, endpointErrCh)
	require.Empty(t, filtersErrCh)
}

func kickOff(wg *sync.WaitGroup, errCh chan error, provider *hcpProviderImpl, check func(cfgProvider *hcpProviderImpl) bool, err error) {
	for i := 0; i < 5000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if !check(provider) {
				errCh <- err
			}
		}()
	}
}

// initGlobalSink is a helper function to initialize a Go metrics inmemsink.
func initGlobalSink() *metrics.InmemSink {
	cfg := metrics.DefaultConfig(testSinkServiceName)
	cfg.EnableHostname = false

	sink := metrics.NewInmemSink(10*time.Second, 10*time.Second)
	metrics.NewGlobal(cfg, sink)

	return sink
}

// collectSinkMetric is a helper function to obtain a measurement from the Go metrics inmemsink.
func collectSinkMetric(sink *metrics.InmemSink, metricKey []string) metrics.SampledValue {
	// Collect sink metrics.
	key := testSinkServiceName + "." + strings.Join(metricKey, ".")
	intervals := sink.Data()
	sv := intervals[0].Counters[key]

	return sv
}

// testDynamicCfg converts testConfig inputs to a dynamicConfig to be used in tests.
func testDynamicCfg(testCfg *testConfig) (*dynamicConfig, error) {
	filters, err := regexp.Compile(testCfg.filters)
	if err != nil {
		return nil, err
	}

	endpoint, err := url.Parse(testCfg.endpoint)
	if err != nil {
		return nil, err
	}
	return &dynamicConfig{
		Endpoint:        endpoint,
		Filters:         filters,
		Labels:          testCfg.labels,
		RefreshInterval: testCfg.refreshInterval,
	}, nil
}

// testTelemetryCfg converts testConfig inputs to a TelemetryConfig to be used in tests.
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
			Filters:  filters,
			Labels:   testCfg.labels,
		},
		RefreshConfig: &client.RefreshConfig{
			RefreshInterval: testRefreshInterval,
		},
	}, nil
}
