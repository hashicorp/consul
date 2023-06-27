package client

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/hcp-sdk-go/clients/cloud-consul-telemetry-gateway/preview/2023-04-14/client/consul_telemetry_service"
	"github.com/hashicorp/hcp-sdk-go/clients/cloud-consul-telemetry-gateway/preview/2023-04-14/models"

	"github.com/hashicorp/consul/agent/hcp/config"
	"github.com/hashicorp/consul/types"
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

func TestConvertTelemetryConfig(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		resp                 *consul_telemetry_service.AgentTelemetryConfigOK
		expectedTelemetryCfg *TelemetryConfig
		wantErr              string
	}{
		"success": {
			resp: &consul_telemetry_service.AgentTelemetryConfigOK{
				Payload: &models.HashicorpCloudConsulTelemetry20230414AgentTelemetryConfigResponse{
					TelemetryConfig: &models.HashicorpCloudConsulTelemetry20230414TelemetryConfig{
						Endpoint: "https://test.com",
						Labels:   map[string]string{"test": "test"},
					},
				},
			},
			expectedTelemetryCfg: &TelemetryConfig{
				Endpoint:      "https://test.com",
				Labels:        map[string]string{"test": "test"},
				MetricsConfig: &MetricsConfig{},
			},
		},
		"successWithMetricsConfig": {
			resp: &consul_telemetry_service.AgentTelemetryConfigOK{
				Payload: &models.HashicorpCloudConsulTelemetry20230414AgentTelemetryConfigResponse{
					TelemetryConfig: &models.HashicorpCloudConsulTelemetry20230414TelemetryConfig{
						Endpoint: "https://test.com",
						Labels:   map[string]string{"test": "test"},
						Metrics: &models.HashicorpCloudConsulTelemetry20230414TelemetryMetricsConfig{
							Endpoint:    "https://metrics-test.com",
							IncludeList: []string{"consul.raft.apply"},
						},
					},
				},
			},
			expectedTelemetryCfg: &TelemetryConfig{
				Endpoint: "https://test.com",
				Labels:   map[string]string{"test": "test"},
				MetricsConfig: &MetricsConfig{
					Endpoint: "https://metrics-test.com",
					Filters:  []string{"consul.raft.apply"},
				},
			},
		},
		"errorsWithNilPayload": {
			resp:    &consul_telemetry_service.AgentTelemetryConfigOK{},
			wantErr: "missing payload",
		},
		"errorsWithNilTelemetryConfig": {
			resp: &consul_telemetry_service.AgentTelemetryConfigOK{
				Payload: &models.HashicorpCloudConsulTelemetry20230414AgentTelemetryConfigResponse{},
			},
			wantErr: "missing telemetry config",
		},
	} {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			telemetryCfg, err := convertTelemetryConfig(test.resp)
			if test.wantErr != "" {
				require.Error(t, err)
				require.Nil(t, telemetryCfg)
				require.Contains(t, err.Error(), test.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, test.expectedTelemetryCfg, telemetryCfg)
		})
	}
}

func Test_DefaultLabels(t *testing.T) {
	for name, tc := range map[string]struct {
		cfg            config.CloudConfig
		expectedLabels map[string]string
	}{
		"Success": {
			cfg: config.CloudConfig{
				NodeID:   types.NodeID("nodeyid"),
				NodeName: "nodey",
			},
			expectedLabels: map[string]string{
				"node_id":   "nodeyid",
				"node_name": "nodey",
			},
		},

		"NoNodeID": {
			cfg: config.CloudConfig{
				NodeID:   types.NodeID(""),
				NodeName: "nodey",
			},
			expectedLabels: map[string]string{
				"node_name": "nodey",
			},
		},
		"NoNodeName": {
			cfg: config.CloudConfig{
				NodeID:   types.NodeID("nodeyid"),
				NodeName: "",
			},
			expectedLabels: map[string]string{
				"node_id": "nodeyid",
			},
		},
		"Empty": {
			cfg: config.CloudConfig{
				NodeID:   "",
				NodeName: "",
			},
			expectedLabels: map[string]string{},
		},
	} {
		t.Run(name, func(t *testing.T) {
			tCfg := &TelemetryConfig{}
			labels := tCfg.DefaultLabels(tc.cfg)
			require.Equal(t, labels, tc.expectedLabels)
		})
	}
}
