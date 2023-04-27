package hcp

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/consul/agent/hcp/client"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestSinkOpts(t *testing.T) {
	for name, test := range map[string]struct {
		expect       func(*client.MockClient)
		mockCloudCfg client.CloudConfig
		wantErr      bool
	}{
		"success": {
			expect: func(mockClient *client.MockClient) {
				mockClient.EXPECT().FetchTelemetryConfig(mock.Anything).Return(&client.TelemetryConfig{
					Endpoint: "test.com",
					MetricsOverride: &client.MetricsConfig{
						Endpoint: "",
					},
				}, nil)
			},
			mockCloudCfg: client.MockCloudCfg{},
		},
		"emptyOptsWhenServerNotRegisteredWithCCM": {
			expect: func(mockClient *client.MockClient) {
				mockClient.EXPECT().FetchTelemetryConfig(mock.Anything).Return(&client.TelemetryConfig{
					Endpoint: "",
					MetricsOverride: &client.MetricsConfig{
						Endpoint: "",
					},
				}, nil)
			},
			mockCloudCfg: client.MockCloudCfg{},
			wantErr:      true,
		},
		"emptyOptsWhenMetricsClientInitFails": {
			expect: func(mockClient *client.MockClient) {
				mockClient.EXPECT().FetchTelemetryConfig(mock.Anything).Return(&client.TelemetryConfig{
					Endpoint: "test.com",
					MetricsOverride: &client.MetricsConfig{
						Endpoint: "",
					},
				}, nil)
			},
			mockCloudCfg: client.MockErrCloudCfg{},
			wantErr:      true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			c := client.NewMockClient(t)
			l := hclog.NewNullLogger()
			test.expect(c)
			sinkOpts := sinkOpts(test.mockCloudCfg, c, l)
			if test.wantErr {
				require.Nil(t, sinkOpts)
				return
			}
			require.NotNil(t, sinkOpts)
		})
	}
}
func TestVerifyCCMRegistration(t *testing.T) {
	for name, test := range map[string]struct {
		expect      func(*client.MockClient)
		wantErr     string
		expectedURL string
	}{
		"failsWithFetchTelemetryFailure": {
			expect: func(mockClient *client.MockClient) {
				mockClient.EXPECT().FetchTelemetryConfig(mock.Anything).Return(nil, fmt.Errorf("FetchTelemetryConfig error"))
			},
			wantErr: "failed to fetch telemetry config",
		},
		"failsWithEmptyEndpoint": {
			expect: func(mockClient *client.MockClient) {
				mockClient.EXPECT().FetchTelemetryConfig(mock.Anything).Return(&client.TelemetryConfig{
					Endpoint: "",
					MetricsOverride: &client.MetricsConfig{
						Endpoint: "",
					},
				}, nil)
			},
			wantErr: "server not registed with management plane",
		},
		"failsWithURLParseErr": {
			expect: func(mockClient *client.MockClient) {
				mockClient.EXPECT().FetchTelemetryConfig(mock.Anything).Return(&client.TelemetryConfig{
					// Minimum 2 chars for a domain to be valid.
					Endpoint: "s",
					MetricsOverride: &client.MetricsConfig{
						// Invalid domain chars
						Endpoint: "			",
					},
				}, nil)
			},
			wantErr: "failed to parse url:",
		},
		"success": {
			expect: func(mockClient *client.MockClient) {
				mockClient.EXPECT().FetchTelemetryConfig(mock.Anything).Return(&client.TelemetryConfig{
					Endpoint: "test.com",
					MetricsOverride: &client.MetricsConfig{
						Endpoint: "",
					},
				}, nil)
			},
			expectedURL: "https://test.com/v1/metrics",
		},
		"successMetricsEndpointOverride": {
			expect: func(mockClient *client.MockClient) {
				mockClient.EXPECT().FetchTelemetryConfig(mock.Anything).Return(&client.TelemetryConfig{
					Endpoint: "test.com",
					MetricsOverride: &client.MetricsConfig{
						Endpoint: "override.com",
					},
				}, nil)
			},
			expectedURL: "https://override.com/v1/metrics",
		},
	} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			mClient := client.NewMockClient(t)
			test.expect(mClient)

			url, err := verifyCCMRegistration(ctx, mClient)
			if test.wantErr != "" {
				require.Empty(t, url)
				require.Error(t, err)
				require.Contains(t, err.Error(), test.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, url, test.expectedURL)
		})
	}
}
