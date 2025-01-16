// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hcp

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-openapi/runtime"
	"github.com/hashicorp/go-metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/hcp/client"
	"github.com/hashicorp/consul/agent/hcp/config"
	"github.com/hashicorp/consul/version"
	"github.com/hashicorp/go-hclog"
)

const (
	testRefreshInterval      = 100 * time.Millisecond
	testSinkServiceName      = "test.telemetry_config_provider"
	testRaceWriteSampleCount = 100
	testRaceReadSampleCount  = 5000
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
	disabled        bool
}

func TestNewTelemetryConfigProvider_DefaultConfig(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize new provider
	provider := NewHCPProvider(ctx)
	provider.updateConfig(ctx)

	// Assert provider has default configuration and metrics processing is disabled.
	defaultCfg := &dynamicConfig{
		labels:          map[string]string{},
		filters:         client.DefaultMetricFilters,
		refreshInterval: defaultTelemetryConfigRefreshInterval,
		endpoint:        nil,
		disabled:        true,
	}
	require.Equal(t, defaultCfg, provider.cfg)
}

func TestTelemetryConfigProvider_UpdateConfig(t *testing.T) {
	for name, tc := range map[string]struct {
		mockExpect       func(*client.MockClient)
		metricKey        string
		initCfg          *dynamicConfig
		expected         *dynamicConfig
		expectedInterval time.Duration
		skipHCPClient    bool
	}{
		"noChanges": {
			initCfg: testDynamicCfg(&testConfig{
				endpoint: "http://test.com/v1/metrics",
				filters:  "test",
				labels: map[string]string{
					"test_label": "123",
				},
				refreshInterval: testRefreshInterval,
			}),
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
			expected: testDynamicCfg(&testConfig{
				endpoint: "http://test.com/v1/metrics",
				labels: map[string]string{
					"test_label": "123",
				},
				filters:         "test",
				refreshInterval: testRefreshInterval,
			}),
			metricKey:        testMetricKeySuccess,
			expectedInterval: testRefreshInterval,
		},
		"newConfig": {
			initCfg: testDynamicCfg(&testConfig{
				endpoint: "http://test.com/v1/metrics",
				filters:  "test",
				labels: map[string]string{
					"test_label": "123",
				},
				refreshInterval: 2 * time.Second,
			}),
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
			expected: testDynamicCfg(&testConfig{
				endpoint: "http://newendpoint/v1/metrics",
				filters:  "consul",
				labels: map[string]string{
					"new_label": "1234",
				},
				refreshInterval: 2 * time.Second,
			}),
			expectedInterval: 2 * time.Second,
			metricKey:        testMetricKeySuccess,
		},
		"newConfigMetricsDisabled": {
			initCfg: testDynamicCfg(&testConfig{
				endpoint: "http://test.com/v1/metrics",
				filters:  "test",
				labels: map[string]string{
					"test_label": "123",
				},
				refreshInterval: 2 * time.Second,
			}),
			mockExpect: func(m *client.MockClient) {
				mockCfg, _ := testTelemetryCfg(&testConfig{
					endpoint: "",
					filters:  "consul",
					labels: map[string]string{
						"new_label": "1234",
					},
					refreshInterval: 2 * time.Second,
					disabled:        true,
				})
				m.EXPECT().FetchTelemetryConfig(mock.Anything).Return(mockCfg, nil)
			},
			expected: testDynamicCfg(&testConfig{
				endpoint: "",
				filters:  "consul",
				labels: map[string]string{
					"new_label": "1234",
				},
				refreshInterval: 2 * time.Second,
				disabled:        true,
			}),
			metricKey:        testMetricKeySuccess,
			expectedInterval: 2 * time.Second,
		},
		"sameConfigInvalidRefreshInterval": {
			initCfg: testDynamicCfg(&testConfig{
				endpoint: "http://test.com/v1/metrics",
				filters:  "test",
				labels: map[string]string{
					"test_label": "123",
				},
				refreshInterval: testRefreshInterval,
			}),
			mockExpect: func(m *client.MockClient) {
				mockCfg, _ := testTelemetryCfg(&testConfig{
					refreshInterval: 0 * time.Second,
				})
				m.EXPECT().FetchTelemetryConfig(mock.Anything).Return(mockCfg, nil)
			},
			expected: testDynamicCfg(&testConfig{
				endpoint: "http://test.com/v1/metrics",
				labels: map[string]string{
					"test_label": "123",
				},
				filters:         "test",
				refreshInterval: testRefreshInterval,
			}),
			metricKey:        testMetricKeyFailure,
			expectedInterval: 0,
		},
		"sameConfigHCPClientFailure": {
			initCfg: testDynamicCfg(&testConfig{
				endpoint: "http://test.com/v1/metrics",
				filters:  "test",
				labels: map[string]string{
					"test_label": "123",
				},
				refreshInterval: testRefreshInterval,
			}),
			mockExpect: func(m *client.MockClient) {
				m.EXPECT().FetchTelemetryConfig(mock.Anything).Return(nil, fmt.Errorf("failure"))
			},
			expected: testDynamicCfg(&testConfig{
				endpoint: "http://test.com/v1/metrics",
				filters:  "test",
				labels: map[string]string{
					"test_label": "123",
				},
				refreshInterval: testRefreshInterval,
			}),
			metricKey:        testMetricKeyFailure,
			expectedInterval: 0,
		},
		"disableMetrics404": {
			initCfg: testDynamicCfg(&testConfig{
				endpoint: "http://test.com/v1/metrics",
				filters:  "test",
				labels: map[string]string{
					"test_label": "123",
				},
				refreshInterval: testRefreshInterval,
			}),
			mockExpect: func(m *client.MockClient) {
				err := runtime.NewAPIError("404 failure", nil, 404)
				m.EXPECT().FetchTelemetryConfig(mock.Anything).Return(nil, err)
			},
			expected:         defaultDisabledCfg(),
			metricKey:        testMetricKeySuccess,
			expectedInterval: defaultTelemetryConfigRefreshInterval,
		},
		"hcpClientNotConfigured": {
			skipHCPClient: true,
			initCfg: testDynamicCfg(&testConfig{
				endpoint: "http://test.com/v1/metrics",
				filters:  "test",
				labels: map[string]string{
					"test_label": "123",
				},
				refreshInterval: testRefreshInterval,
			}),
			expected:         defaultDisabledCfg(),
			metricKey:        testMetricKeySuccess,
			expectedInterval: defaultTelemetryConfigRefreshInterval,
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			sink := initGlobalSink()
			var mockClient *client.MockClient
			if !tc.skipHCPClient {
				mockClient = client.NewMockClient(t)
				tc.mockExpect(mockClient)
			}

			provider := &hcpProviderImpl{
				hcpClient: mockClient,
				cfg:       tc.initCfg,
				logger:    hclog.NewNullLogger(),
			}

			newInterval := provider.updateConfig(context.Background())
			require.Equal(t, tc.expectedInterval, newInterval)

			// Verify endpoint provider returns correct config values.
			require.Equal(t, tc.expected.endpoint, provider.GetEndpoint())
			require.Equal(t, tc.expected.filters, provider.GetFilters())
			require.Equal(t, tc.expected.labels, provider.GetLabels())
			require.Equal(t, tc.expected.disabled, provider.IsDisabled())

			// Verify count for transform success metric.
			interval := sink.Data()[0]
			require.NotNil(t, interval, 1)
			sv := interval.Counters[tc.metricKey]
			assert.NotNil(t, sv.AggregateSample)
			require.Equal(t, sv.AggregateSample.Count, 1)
		})
	}
}

func TestTelemetryConfigProvider_Start(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	provider := NewHCPProvider(ctx)

	testUpdateConfigCh := make(chan struct{}, 1)
	provider.testUpdateConfigCh = testUpdateConfigCh

	// Configure mocks
	mockClient := client.NewMockClient(t)
	mTelemetryCfg, err := testTelemetryCfg(&testConfig{
		endpoint: "http://test.com/v1/metrics",
		filters:  "test",
		labels: map[string]string{
			"test_label": "123",
		},
		refreshInterval: testRefreshInterval,
	})
	require.NoError(t, err)
	mockClient.EXPECT().FetchTelemetryConfig(mock.Anything).Return(mTelemetryCfg, nil)
	mockHCPCfg := &config.MockCloudCfg{}

	// Run provider
	go provider.Start(context.Background(), &HCPProviderCfg{
		HCPClient: mockClient,
		HCPConfig: mockHCPCfg,
	})

	// Expect at least two update config calls to validate provider is running
	// and has entered the main run loop
	select {
	case <-testUpdateConfigCh:
	case <-time.After(time.Second):
		require.Fail(t, "provider did not attempt to update config in expected time")
	}
	select {
	case <-testUpdateConfigCh:
	case <-time.After(time.Millisecond * 500):
		require.Fail(t, "provider did not attempt to update config in expected time")
	}

	mockClient.AssertExpectations(t)
}

func TestTelemetryConfigProvider_MultipleRun(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	provider := NewHCPProvider(ctx)

	testUpdateConfigCh := make(chan struct{}, 1)
	provider.testUpdateConfigCh = testUpdateConfigCh

	// Configure mocks
	mockClient := client.NewMockClient(t)
	mTelemetryCfg, err := testTelemetryCfg(&testConfig{
		refreshInterval: 30 * time.Minute,
	})
	require.NoError(t, err)
	mockClient.EXPECT().FetchTelemetryConfig(mock.Anything).Return(mTelemetryCfg, nil)
	mockHCPCfg := &config.MockCloudCfg{}

	// Run provider twice in parallel
	go provider.Start(context.Background(), &HCPProviderCfg{
		HCPClient: mockClient,
		HCPConfig: mockHCPCfg,
	})
	go provider.Start(context.Background(), &HCPProviderCfg{
		HCPClient: mockClient,
		HCPConfig: mockHCPCfg,
	})

	// Expect only one update config call
	select {
	case <-testUpdateConfigCh:
	case <-time.After(time.Second):
		require.Fail(t, "provider did not attempt to update config in expected time")
	}

	select {
	case <-testUpdateConfigCh:
		require.Fail(t, "provider unexpectedly updated config")
	case <-time.After(time.Second):
	}

	// Try calling run again, should not update again
	provider.Start(context.Background(), &HCPProviderCfg{
		HCPClient: mockClient,
		HCPConfig: mockHCPCfg,
	})

	select {
	case <-testUpdateConfigCh:
		require.Fail(t, "provider unexpectedly updated config")
	case <-time.After(time.Second):
	}

	mockClient.AssertExpectations(t)
}

func TestTelemetryConfigProvider_updateHTTPConfig(t *testing.T) {
	for name, test := range map[string]struct {
		wantErr string
		cfg     config.CloudConfigurer
	}{
		"success": {
			cfg: &config.MockCloudCfg{},
		},
		"failsWithoutCloudCfg": {
			wantErr: "must provide valid HCP configuration",
			cfg:     nil,
		},
		"failsHCPConfig": {
			wantErr: "failed to configure telemetry HTTP client",
			cfg: config.MockCloudCfg{
				ConfigErr: fmt.Errorf("test bad hcp config"),
			},
		},
		"failsBadResource": {
			wantErr: "failed set telemetry client headers",
			cfg: config.MockCloudCfg{
				ResourceErr: fmt.Errorf("test bad resource"),
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			provider := NewHCPProvider(context.Background())
			err := provider.updateHTTPConfig(test.cfg)

			if test.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.wantErr)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, provider.GetHTTPClient())

			expectedHeader := make(http.Header)
			expectedHeader.Set("content-type", "application/x-protobuf")
			expectedHeader.Set("x-hcp-resource-id", "organization/test-org/project/test-project/test-type/test-id")
			expectedHeader.Set("x-channel", fmt.Sprintf("consul/%s", version.GetHumanVersion()))
			require.Equal(t, expectedHeader, provider.GetHeader())
		})
	}
}

func TestTelemetryConfigProvider_Stop(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	provider := NewHCPProvider(ctx)

	testUpdateConfigCh := make(chan struct{}, 1)
	provider.testUpdateConfigCh = testUpdateConfigCh

	// Configure mocks
	mockClient := client.NewMockClient(t)
	mTelemetryCfg, err := testTelemetryCfg(&testConfig{
		endpoint: "http://test.com/v1/metrics",
		filters:  "test",
		labels: map[string]string{
			"test_label": "123",
		},
		refreshInterval: testRefreshInterval,
	})
	require.NoError(t, err)
	mockClient.EXPECT().FetchTelemetryConfig(mock.Anything).Return(mTelemetryCfg, nil)
	mockHCPCfg := &config.MockCloudCfg{}

	// Run provider
	provider.Start(context.Background(), &HCPProviderCfg{
		HCPClient: mockClient,
		HCPConfig: mockHCPCfg,
	})

	// Wait for at least two update config calls to ensure provider is running
	// and has entered the main run loop
	select {
	case <-testUpdateConfigCh:
	case <-time.After(time.Second):
		require.Fail(t, "provider did not attempt to update config in expected time")
	}
	select {
	case <-testUpdateConfigCh:
	case <-time.After(time.Millisecond * 500):
		require.Fail(t, "provider did not attempt to update config in expected time")
	}

	// Stop the provider
	provider.Stop()
	require.Equal(t, defaultDisabledCfg(), provider.cfg)
	select {
	case <-testUpdateConfigCh:
		require.Fail(t, "provider should not attempt to update config after stop")
	case <-time.After(time.Second):
		// Success, no updates have happened after stopping
	}

	mockClient.AssertExpectations(t)
}

// mockRaceClient is a mock HCP client that fetches TelemetryConfig.
// The mock TelemetryConfig returned can be manually updated at any time.
// It manages concurrent read/write access to config with a sync.RWMutex.
type mockRaceClient struct {
	client.Client
	cfg *client.TelemetryConfig
	rw  sync.RWMutex
}

// updateCfg acquires a write lock and updates client config to a new value givent a count.
func (m *mockRaceClient) updateCfg(count int) (*client.TelemetryConfig, error) {
	m.rw.Lock()
	defer m.rw.Unlock()

	labels := map[string]string{fmt.Sprintf("label_%d", count): fmt.Sprintf("value_%d", count)}

	filters, err := regexp.Compile(fmt.Sprintf("consul_filter_%d", count))
	if err != nil {
		return nil, err
	}

	endpoint, err := url.Parse(fmt.Sprintf("http://consul-endpoint-%d.com", count))
	if err != nil {
		return nil, err
	}

	cfg := &client.TelemetryConfig{
		MetricsConfig: &client.MetricsConfig{
			Filters:  filters,
			Endpoint: endpoint,
			Labels:   labels,
		},
		RefreshConfig: &client.RefreshConfig{
			RefreshInterval: testRefreshInterval,
		},
	}
	m.cfg = cfg

	return cfg, nil
}

// FetchTelemetryConfig returns the current config held by the mockRaceClient.
func (m *mockRaceClient) FetchTelemetryConfig(ctx context.Context) (*client.TelemetryConfig, error) {
	m.rw.RLock()
	defer m.rw.RUnlock()

	return m.cfg, nil
}

func TestTelemetryConfigProvider_Race(t *testing.T) {
	//todo(achooo): address flaky test
	t.Skip("TODO(flaky): This test fails often in the CI")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	initCfg, err := testTelemetryCfg(&testConfig{
		endpoint:        "test.com",
		filters:         "test",
		labels:          map[string]string{"test_label": "test_value"},
		refreshInterval: testRefreshInterval,
	})
	require.NoError(t, err)

	m := &mockRaceClient{
		cfg: initCfg,
	}

	// Start the provider goroutine, which fetches client TelemetryConfig every RefreshInterval.
	provider := NewHCPProvider(ctx)
	err = provider.Start(context.Background(), &HCPProviderCfg{m, config.MockCloudCfg{}})
	require.NoError(t, err)

	for count := 0; count < testRaceWriteSampleCount; count++ {
		// Force a TelemetryConfig value change in the mockRaceClient.
		newCfg, err := m.updateCfg(count)
		require.NoError(t, err)
		// Force provider to obtain new client TelemetryConfig immediately.
		// This call is necessary to guarantee TelemetryConfig changes to assert on expected values below.
		provider.updateConfig(context.Background())

		// Start goroutines to access label configuration.
		wg := &sync.WaitGroup{}
		kickOff(wg, testRaceReadSampleCount, provider, func(provider *hcpProviderImpl) {
			require.Equal(t, provider.GetLabels(), newCfg.MetricsConfig.Labels)
		})

		// Start goroutines to access endpoint configuration.
		kickOff(wg, testRaceReadSampleCount, provider, func(provider *hcpProviderImpl) {
			require.Equal(t, provider.GetFilters(), newCfg.MetricsConfig.Filters)
		})

		// Start goroutines to access filter configuration.
		kickOff(wg, testRaceReadSampleCount, provider, func(provider *hcpProviderImpl) {
			require.Equal(t, provider.GetEndpoint(), newCfg.MetricsConfig.Endpoint)
		})

		wg.Wait()
	}
}

func kickOff(wg *sync.WaitGroup, count int, provider *hcpProviderImpl, check func(cfgProvider *hcpProviderImpl)) {
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			check(provider)
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
func testDynamicCfg(testCfg *testConfig) *dynamicConfig {
	filters, _ := regexp.Compile(testCfg.filters)

	var endpoint *url.URL
	if testCfg.endpoint != "" {
		endpoint, _ = url.Parse(testCfg.endpoint)
	}
	return &dynamicConfig{
		endpoint:        endpoint,
		filters:         filters,
		labels:          testCfg.labels,
		refreshInterval: testCfg.refreshInterval,
		disabled:        testCfg.disabled,
	}
}

// testTelemetryCfg converts testConfig inputs to a TelemetryConfig to be used in tests.
func testTelemetryCfg(testCfg *testConfig) (*client.TelemetryConfig, error) {
	filters, err := regexp.Compile(testCfg.filters)
	if err != nil {
		return nil, err
	}

	var endpoint *url.URL
	if testCfg.endpoint != "" {
		u, err := url.Parse(testCfg.endpoint)
		if err != nil {
			return nil, err
		}
		endpoint = u
	}

	return &client.TelemetryConfig{
		MetricsConfig: &client.MetricsConfig{
			Endpoint: endpoint,
			Filters:  filters,
			Labels:   testCfg.labels,
			Disabled: testCfg.disabled,
		},
		RefreshConfig: &client.RefreshConfig{
			RefreshInterval: testCfg.refreshInterval,
		},
	}, nil
}
