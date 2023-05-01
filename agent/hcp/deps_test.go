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

func TestSink(t *testing.T) {
	for name, test := range map[string]struct {
		expect       func(*client.MockClient)
		mockCloudCfg client.CloudConfig
		expectedSink bool
	}{
		"success": {
			expect: func(mockClient *client.MockClient) {
				mockClient.EXPECT().FetchTelemetryConfig(mock.Anything).Return(&client.TelemetryConfig{
					Endpoint: "test.com",
					MetricsConfig: &client.MetricsConfig{
						Endpoint: "test.com",
					},
				}, nil)
			},
			mockCloudCfg: client.MockCloudCfg{},
			expectedSink: true,
		},
		"noSinkWhenServerNotRegisteredWithCCM": {
			expect: func(mockClient *client.MockClient) {
				mockClient.EXPECT().FetchTelemetryConfig(mock.Anything).Return(&client.TelemetryConfig{
					Endpoint: "",
					MetricsConfig: &client.MetricsConfig{
						Endpoint: "",
					},
				}, nil)
			},
			mockCloudCfg: client.MockCloudCfg{},
		},
		"noSinkWhenCCMVerificationFails": {
			expect: func(mockClient *client.MockClient) {
				mockClient.EXPECT().FetchTelemetryConfig(mock.Anything).Return(nil, fmt.Errorf("fetch failed"))
			},
			mockCloudCfg: client.MockCloudCfg{},
		},
		"noSinkWhenMetricsClientInitFails": {
			expect: func(mockClient *client.MockClient) {
				mockClient.EXPECT().FetchTelemetryConfig(mock.Anything).Return(&client.TelemetryConfig{
					Endpoint: "test.com",
					MetricsConfig: &client.MetricsConfig{
						Endpoint: "",
					},
				}, nil)
			},
			mockCloudCfg: client.MockErrCloudCfg{},
		},
	} {
		t.Run(name, func(t *testing.T) {
			c := client.NewMockClient(t)
			l := hclog.NewNullLogger()
			test.expect(c)
			sinkOpts := sink(c, test.mockCloudCfg, l)
			if !test.expectedSink {
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
		"failsWithURLParseErr": {
			expect: func(mockClient *client.MockClient) {
				mockClient.EXPECT().FetchTelemetryConfig(mock.Anything).Return(&client.TelemetryConfig{
					// Minimum 2 chars for a domain to be valid.
					Endpoint: "s",
					MetricsConfig: &client.MetricsConfig{
						// Invalid domain chars
						Endpoint: "			",
					},
				}, nil)
			},
			wantErr: "failed to parse url:",
		},
		"noErrWithEmptyEndpoint": {
			expect: func(mockClient *client.MockClient) {
				mockClient.EXPECT().FetchTelemetryConfig(mock.Anything).Return(&client.TelemetryConfig{
					Endpoint: "",
					MetricsConfig: &client.MetricsConfig{
						Endpoint: "",
					},
				}, nil)
			},
			expectedURL: "",
		},
		"success": {
			expect: func(mockClient *client.MockClient) {
				mockClient.EXPECT().FetchTelemetryConfig(mock.Anything).Return(&client.TelemetryConfig{
					Endpoint: "test.com",
					MetricsConfig: &client.MetricsConfig{
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
					MetricsConfig: &client.MetricsConfig{
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
