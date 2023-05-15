package hcp

import (
	"fmt"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/hcp/client"
	"github.com/hashicorp/consul/types"
)

func TestSink(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		expect       func(*client.MockClient)
		mockCloudCfg client.CloudConfig
		expectedSink bool
	}{
		"success": {
			expect: func(mockClient *client.MockClient) {
				mockClient.EXPECT().FetchTelemetryConfig(mock.Anything).Return(&client.TelemetryConfig{
					Endpoint: "https://test.com",
					MetricsConfig: &client.MetricsConfig{
						Endpoint: "https://test.com",
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
					Endpoint: "https://test.com",
					MetricsConfig: &client.MetricsConfig{
						Endpoint: "",
					},
				}, nil)
			},
			mockCloudCfg: client.MockErrCloudCfg{},
		},
		"failsWithFetchTelemetryFailure": {
			expect: func(mockClient *client.MockClient) {
				mockClient.EXPECT().FetchTelemetryConfig(mock.Anything).Return(nil, fmt.Errorf("FetchTelemetryConfig error"))
			},
		},
	} {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := client.NewMockClient(t)
			l := hclog.NewNullLogger()
			test.expect(c)
			sinkOpts := sink(c, test.mockCloudCfg, l, types.NodeID("server1234"))
			if !test.expectedSink {
				require.Nil(t, sinkOpts)
				return
			}
			require.NotNil(t, sinkOpts)
		})
	}
}
