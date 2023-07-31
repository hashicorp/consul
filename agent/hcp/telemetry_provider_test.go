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
	testRaceSampleCount = 5000
	testSinkServiceName = "test.telemetry_config_provider"
)

var (
	// Test constants to verify inmem sink metrics.
	testMetricKeyFailure = testSinkServiceName + "." + strings.Join(internalMetricRefreshFailure, ".")
	testMetricKeySuccess = testSinkServiceName + "." + strings.Join(internalMetricRefreshSuccess, ".")
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
		testInputs *testConfig
		wantErr    string
	}{
		"success": {
			testInputs: &testConfig{
				refreshInterval: 1 * time.Second,
			},
		},
		"failsWithInvalidRefreshInterval": {
			testInputs: &testConfig{
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

			testCfg, err := testTelemetryCfg(tc.testInputs)
			require.NoError(t, err)

			cfgProvider, err := NewHCPProvider(ctx, client.NewMockClient(t), testCfg)
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

func TestTelemetryConfigProviderGetUpdate(t *testing.T) {
	for name, tc := range map[string]struct {
		mockExpect func(*client.MockClient)
		metricKey  string
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
			mockExpect: func(m *client.MockClient) {
				mockCfg, _ := testTelemetryCfg(&testConfig{
					endpoint: "http://test.com/v1/metrics",
					filters:  "test",
					labels: map[string]string{
						"test_label": "123",
					},
					refreshInterval: testRefreshInterval,
				})
				m.EXPECT().FetchTelemetryConfig(mock.Anything).Return(mockCfg, nil)
			},
			expected: &testConfig{
				endpoint: "http://test.com/v1/metrics",
				labels: map[string]string{
					"test_label": "123",
				},
				filters:         "test",
				refreshInterval: testRefreshInterval,
			},
			metricKey: testMetricKeySuccess,
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
			mockExpect: func(m *client.MockClient) {
				mockCfg, _ := testTelemetryCfg(&testConfig{
					endpoint: "http://newendpoint/v1/metrics",
					filters:  "consul",
					labels: map[string]string{
						"new_label": "1234",
					},
					refreshInterval: 2 * time.Second,
				})
				m.EXPECT().FetchTelemetryConfig(mock.Anything).Return(mockCfg, nil)
			},
			expected: &testConfig{
				endpoint: "http://newendpoint/v1/metrics",
				filters:  "consul",
				labels: map[string]string{
					"new_label": "1234",
				},
				refreshInterval: 2 * time.Second,
			},
			metricKey: testMetricKeySuccess,
		},
		"sameConfigInvalidRefreshInterval": {
			optsInputs: &testConfig{
				endpoint: "http://test.com/v1/metrics",
				filters:  "test",
				labels: map[string]string{
					"test_label": "123",
				},
				refreshInterval: testRefreshInterval,
			},
			mockExpect: func(m *client.MockClient) {
				mockCfg, _ := testTelemetryCfg(&testConfig{
					refreshInterval: 0 * time.Second,
				})
				m.EXPECT().FetchTelemetryConfig(mock.Anything).Return(mockCfg, nil)
			},
			expected: &testConfig{
				endpoint: "http://test.com/v1/metrics",
				labels: map[string]string{
					"test_label": "123",
				},
				filters:         "test",
				refreshInterval: testRefreshInterval,
			},
			metricKey: testMetricKeyFailure,
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
			metricKey: testMetricKeyFailure,
		},
	} {
		t.Run(name, func(t *testing.T) {
			sink := initGlobalSink()
			mockClient := client.NewMockClient(t)
			tc.mockExpect(mockClient)

			dynamicCfg, err := testDynamicCfg(tc.optsInputs)
			require.NoError(t, err)

			provider := &hcpProviderImpl{
				hcpClient: mockClient,
				cfg:       dynamicCfg,
			}

			provider.getUpdate(context.Background())

			// Verify endpoint provider returns correct config values.
			require.Equal(t, tc.expected.endpoint, provider.GetEndpoint().String())
			require.Equal(t, tc.expected.filters, provider.GetFilters().String())
			require.Equal(t, tc.expected.labels, provider.GetLabels())

			// Verify count for transform success metric.
			interval := sink.Data()[0]
			require.NotNil(t, interval, 1)
			sv := interval.Counters[tc.metricKey]
			assert.NotNil(t, sv.AggregateSample)
			require.Equal(t, sv.AggregateSample.Count, 1)
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
	mc.counter++
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
	dynamicCfg, err := testDynamicCfg(&testConfig{
		endpoint: "http://test.com/v1/metrics",
		filters:  "test",
		labels: map[string]string{
			"test_label": "123",
		},
		refreshInterval: testRefreshInterval,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	provider := &hcpProviderImpl{
		hcpClient: &mockClientRace{
			defaultEndpoint: dynamicCfg.Endpoint,
			defaultFilters:  dynamicCfg.Filters,
		},
		cfg: dynamicCfg,
	}

	go provider.run(ctx, dynamicCfg.RefreshInterval)

	require.NoError(t, err)

	wg := &sync.WaitGroup{}

	labelErrCh := make(chan error, testRaceSampleCount)
	labelErr := errors.New("expected labels to have one entry")
	// Start 5000 goroutines that try to access label configuration.
	kickOff(wg, labelErrCh, provider, func(provider *hcpProviderImpl) bool {
		return len(provider.GetLabels()) == 1
	}, labelErr)

	expectedEndpoint := dynamicCfg.Endpoint.String()
	endpointErr := fmt.Errorf("expected endpoint to be %s", expectedEndpoint)
	endpointErrCh := make(chan error, testRaceSampleCount)
	// Start 5000 goroutines that try to access endpoint configuration.
	kickOff(wg, endpointErrCh, provider, func(provider *hcpProviderImpl) bool {
		return provider.GetEndpoint().String() == expectedEndpoint
	}, endpointErr)

	expectedFilters := dynamicCfg.Filters.String()
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
			RefreshInterval: testCfg.refreshInterval,
		},
	}, nil
}
