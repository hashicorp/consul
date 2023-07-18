package hcp

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/consul/agent/hcp/config"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/hcp/client"
	"github.com/hashicorp/consul/types"
)

func TestSink(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		expect       func(*client.MockClient)
		cloudCfg     config.CloudConfig
		expectedSink bool
	}{
		"success": {
			expect: func(mockClient *client.MockClient) {
				mockClient.EXPECT().FetchTelemetryConfig(mock.Anything).Return(&client.TelemetryConfig{
					Endpoint: "https://test.com",
					MetricsConfig: &client.MetricsConfig{
						Endpoint: "https://test.com",
						Filters:  []string{"test"},
					},
				}, nil)
			},
			cloudCfg: config.CloudConfig{
				NodeID:   types.NodeID("nodeyid"),
				NodeName: "nodey",
			},
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
			cloudCfg: config.CloudConfig{},
		},
		"noSinkWhenCCMVerificationFails": {
			expect: func(mockClient *client.MockClient) {
				mockClient.EXPECT().FetchTelemetryConfig(mock.Anything).Return(nil, fmt.Errorf("fetch failed"))
			},
			cloudCfg: config.CloudConfig{},
		},
		"failsWithFetchTelemetryFailure": {
			expect: func(mockClient *client.MockClient) {
				mockClient.EXPECT().FetchTelemetryConfig(mock.Anything).Return(nil, fmt.Errorf("FetchTelemetryConfig error"))
			},
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
		},
	} {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := client.NewMockClient(t)
			mc := client.MockMetricsClient{}

			test.expect(c)
			ctx := context.Background()

			s := sink(ctx, c, mc, test.cloudCfg)
			if !test.expectedSink {
				require.Nil(t, s)
				return
			}
			require.NotNil(t, s)
		})
	}
}
