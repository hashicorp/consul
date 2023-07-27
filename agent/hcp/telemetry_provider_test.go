package hcp

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/hcp/client"
)

const defaultTestRefreshInterval = 100 * time.Millisecond

type testConfig struct {
	filters         string
	endpoint        *url.URL
	labels          map[string]string
	refreshInterval time.Duration
}

	t.Parallel()
	for name, tc := range map[string]struct {
		a        *testConfig
		b        *testConfig
		expected bool
	}{
		"same": {
			a: &testConfig{
				endpoint: &url.URL{
					Host: "http://test.com/v1/metrics",
				},
				filters:         "state|raft",
				labels:          map[string]string{"test": "123"},
				refreshInterval: 1 * time.Second,
			},
			b: &testConfig{
				endpoint: &url.URL{
					Host: "http://test.com/v1/metrics",
				},
				filters:         "state|raft",
				labels:          map[string]string{"test": "123"},
				refreshInterval: 1 * time.Second,
			},
			expected: true,
		},
		"different": {
			a: &testConfig{
				endpoint: &url.URL{
					Host: "http://test.com/v1/metrics",
				},
				filters:         "state|raft|extra",
				labels:          map[string]string{"test": "12334"},
				refreshInterval: 2 * time.Second,
			},
			b: &testConfig{
				endpoint: &url.URL{
					Host: "http://other-endpoint-test.com/v1/metrics",
				},
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
				labels: map[string]string{
					"test_label": "123",
				},
				filters: "test",
			},
		},
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
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
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
		})
	}
}

func testDynamicCfg(testCfg *testConfig) (*dynamicConfig, error) {
	filters, err := regexp.Compile(testCfg.filters)
	if err != nil {
		return nil, err
	}

	return &dynamicConfig{
		Endpoint:        testCfg.endpoint,
		Filters:         filters,
		Labels:          testCfg.labels,
		RefreshInterval: testCfg.refreshInterval,
	}, nil
}

func newTestTelemetryConfig(t *testing.T, testCfg *testConfig) (*client.TelemetryConfig, error) {
	t.Helper()

	filters, err := regexp.Compile(testCfg.filters)
	if err != nil {
		return nil, fmt.Errorf("failed to compile test filters: %v", filters)
	}

	return &client.TelemetryConfig{
		MetricsConfig: &client.MetricsConfig{
			Endpoint: testCfg.endpoint,
			Filters:  filters,
			Labels:   testCfg.labels,
		},
		RefreshConfig: &client.RefreshConfig{
			RefreshInterval: defaultTestRefreshInterval,
		},
	}, nil
}
