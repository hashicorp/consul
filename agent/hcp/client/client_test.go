package client

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestFetchTelemetryConfig(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		metricsEndpoint string
		expect          func(*MockClient)
		disabled        bool
	}{
		"success": {
			expect: func(mockClient *MockClient) {
				mockClient.EXPECT().FetchTelemetryConfig(mock.Anything).Return(&TelemetryConfig{
					Endpoint: "https://test.com",
					MetricsConfig: &MetricsConfig{
						Endpoint: "",
					},
				}, nil)
			},
			metricsEndpoint: "https://test.com/v1/metrics",
		},
		"overrideMetricsEndpoint": {
			expect: func(mockClient *MockClient) {
				mockClient.EXPECT().FetchTelemetryConfig(mock.Anything).Return(&TelemetryConfig{
					Endpoint: "https://test.com",
					MetricsConfig: &MetricsConfig{
						Endpoint: "https://test.com",
					},
				}, nil)
			},
			metricsEndpoint: "https://test.com/v1/metrics",
		},
		"disabledWithEmptyEndpoint": {
			expect: func(mockClient *MockClient) {
				mockClient.EXPECT().FetchTelemetryConfig(mock.Anything).Return(&TelemetryConfig{
					Endpoint: "",
					MetricsConfig: &MetricsConfig{
						Endpoint: "",
					},
				}, nil)
			},
			disabled: true,
		},
	} {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mock := NewMockClient(t)
			test.expect(mock)

			telemetryCfg, err := mock.FetchTelemetryConfig(context.Background())
			require.NoError(t, err)

			if test.disabled {
				endpoint, ok := telemetryCfg.Enabled()
				require.False(t, ok)
				require.Empty(t, endpoint)
				return
			}

			endpoint, ok := telemetryCfg.Enabled()

			require.True(t, ok)
			require.Equal(t, test.metricsEndpoint, endpoint)
		})
	}
}
