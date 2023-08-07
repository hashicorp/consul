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
	"github.com/hashicorp/consul/agent/hcp/telemetry"
)

type mockMetricsClient struct {
	telemetry.MetricsClient
}

func TestSink(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		expect       func(*client.MockClient)
		wantErr      string
		expectedSink bool
	}{
		"success": {
			expect: func(mockClient *client.MockClient) {
				u, _ := url.Parse("https://test.com/v1/metrics")
				filters, _ := regexp.Compile("test")
				mt := mockTelemetryConfig(1*time.Second, u, filters)
				mockClient.EXPECT().FetchTelemetryConfig(mock.Anything).Return(mt, nil)
			},
			expectedSink: true,
		},
		"noSinkWhenFetchTelemetryConfigFails": {
			expect: func(mockClient *client.MockClient) {
				mockClient.EXPECT().FetchTelemetryConfig(mock.Anything).Return(nil, fmt.Errorf("fetch failed"))
			},
			wantErr: "failed to fetch telemetry config",
		},
		"noSinkWhenServerNotRegisteredWithCCM": {
			expect: func(mockClient *client.MockClient) {
				mt := mockTelemetryConfig(1*time.Second, nil, nil)
				mockClient.EXPECT().FetchTelemetryConfig(mock.Anything).Return(mt, nil)
			},
		},
		"noSinkWhenTelemetryConfigProviderInitFails": {
			expect: func(mockClient *client.MockClient) {
				u, _ := url.Parse("https://test.com/v1/metrics")
				// Bad refresh interval forces ConfigProvider creation failure.
				mt := mockTelemetryConfig(0*time.Second, u, nil)
				mockClient.EXPECT().FetchTelemetryConfig(mock.Anything).Return(mt, nil)
			},
			wantErr: "failed to init config provider",
		},
	} {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := client.NewMockClient(t)
			mc := mockMetricsClient{}

			test.expect(c)
			ctx := context.Background()

			s, err := sink(ctx, c, mc)

			if test.wantErr != "" {
				require.NotNil(t, err)
				require.Contains(t, err.Error(), test.wantErr)
				require.Nil(t, s)
				return
			}

			if !test.expectedSink {
				require.Nil(t, s)
				require.Nil(t, err)
				return
			}

			require.NotNil(t, s)
		})
	}
}

func mockTelemetryConfig(refreshInterval time.Duration, metricsEndpoint *url.URL, filters *regexp.Regexp) *client.TelemetryConfig {
	return &client.TelemetryConfig{
		MetricsConfig: &client.MetricsConfig{
			Endpoint: metricsEndpoint,
			Filters:  filters,
		},
		RefreshConfig: &client.RefreshConfig{
			RefreshInterval: refreshInterval,
		},
	}
}
