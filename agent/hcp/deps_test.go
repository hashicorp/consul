package hcp

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/consul/agent/hcp/client"
	"github.com/hashicorp/consul/agent/hcp/telemetry"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestVerifyCCMRegistration(t *testing.T) {
	for name, test := range map[string]struct {
		expect       func(*client.MockClient)
		wantErr      string
		mockCloudCfg client.CloudConfig
		expectedURL  string
	}{
		"failsWithFetchTelemetryFailure": {
			expect: func(mockClient *client.MockClient) {
				mockClient.EXPECT().FetchTelemetryConfig(mock.Anything).Return(nil, fmt.Errorf("FetchTelemetryConfig error"))
			},
			mockCloudCfg: &client.MockCloudCfg{},
			wantErr:      "failed to fetch telemetry config",
		},
		"noSinkWithEmptyEndpoint": {
			expect: func(mockClient *client.MockClient) {
				mockClient.EXPECT().FetchTelemetryConfig(mock.Anything).Return(&client.TelemetryConfig{
					Endpoint: "",
					MetricsOverride: &client.MetricsConfig{
						Endpoint: "",
					},
				}, nil)
			},
			mockCloudCfg: &client.MockCloudCfg{},
			wantErr:      "server not registed with management plane",
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
			mockCloudCfg: &client.MockCloudCfg{},
			wantErr:      "failed to parse url:",
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
			mockCloudCfg: &client.MockCloudCfg{},
			expectedURL:  "https://test.com",
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
			mockCloudCfg: &client.MockCloudCfg{},
			expectedURL:  "https://override.com",
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

func TestInitSink(t *testing.T) {
	for name, test := range map[string]struct {
		sinkOpts   *telemetry.OTELSinkOpts
		clientOpts *client.TelemetryClientCfg
		wantErr    string
	}{
		"failsWithMetricsClientError": {
			clientOpts: &client.TelemetryClientCfg{
				Logger:   hclog.NewNullLogger(),
				CloudCfg: &client.MockErrCloudCfg{},
			},
			sinkOpts: &telemetry.OTELSinkOpts{
				Logger: hclog.NewNullLogger(),
				Ctx:    context.Background(),
			},
			wantErr: "failed to init metrics client",
		},
		"failsWithInvalidSinkOpts": {
			clientOpts: &client.TelemetryClientCfg{
				Logger:   hclog.NewNullLogger(),
				CloudCfg: &client.MockCloudCfg{},
			},
			sinkOpts: &telemetry.OTELSinkOpts{
				Logger: nil,
				Ctx:    context.Background(),
			},
			wantErr: "failed to init OTEL sink: provide valid OTELSinkOpts",
		},
		"success": {
			clientOpts: &client.TelemetryClientCfg{
				Logger:   hclog.NewNullLogger(),
				CloudCfg: &client.MockCloudCfg{},
			},
			sinkOpts: &telemetry.OTELSinkOpts{
				Logger: hclog.NewNullLogger(),
				Ctx:    context.Background(),
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			sink, err := initHCPSink(test.sinkOpts, test.clientOpts, "https://test.com")
			if test.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.wantErr)
				return
			}

			require.NotNil(t, sink)
			require.NoError(t, err)
		})
	}
}
