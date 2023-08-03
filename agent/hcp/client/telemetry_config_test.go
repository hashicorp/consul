package client

import (
	"context"
	"net/url"
	"regexp"
	"testing"
	"time"

	"github.com/hashicorp/hcp-sdk-go/clients/cloud-consul-telemetry-gateway/preview/2023-04-14/client/consul_telemetry_service"
	"github.com/hashicorp/hcp-sdk-go/clients/cloud-consul-telemetry-gateway/preview/2023-04-14/models"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/hcp/config"
	"github.com/hashicorp/consul/types"
)

func TestValidateAgentTelemetryConfigPayload(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		resp    *consul_telemetry_service.AgentTelemetryConfigOK
		wantErr error
	}{
		"errorsWithNilPayload": {
			resp:    &consul_telemetry_service.AgentTelemetryConfigOK{},
			wantErr: errMissingPayload,
		},
		"errorsWithNilTelemetryConfig": {
			resp: &consul_telemetry_service.AgentTelemetryConfigOK{
				Payload: &models.HashicorpCloudConsulTelemetry20230414AgentTelemetryConfigResponse{
					RefreshConfig: &models.HashicorpCloudConsulTelemetry20230414RefreshConfig{},
				},
			},
			wantErr: errMissingTelemetryConfig,
		},
		"errorsWithNilRefreshConfig": {
			resp: &consul_telemetry_service.AgentTelemetryConfigOK{
				Payload: &models.HashicorpCloudConsulTelemetry20230414AgentTelemetryConfigResponse{
					TelemetryConfig: &models.HashicorpCloudConsulTelemetry20230414TelemetryConfig{},
				},
			},
			wantErr: errMissingRefreshConfig,
		},
		"errorsWithNilMetricsConfig": {
			resp: &consul_telemetry_service.AgentTelemetryConfigOK{
				Payload: &models.HashicorpCloudConsulTelemetry20230414AgentTelemetryConfigResponse{
					TelemetryConfig: &models.HashicorpCloudConsulTelemetry20230414TelemetryConfig{},
					RefreshConfig:   &models.HashicorpCloudConsulTelemetry20230414RefreshConfig{},
				},
			},
			wantErr: errMissingMetricsConfig,
		},
		"success": {
			resp: &consul_telemetry_service.AgentTelemetryConfigOK{
				Payload: &models.HashicorpCloudConsulTelemetry20230414AgentTelemetryConfigResponse{
					TelemetryConfig: &models.HashicorpCloudConsulTelemetry20230414TelemetryConfig{
						Metrics: &models.HashicorpCloudConsulTelemetry20230414TelemetryMetricsConfig{},
					},
					RefreshConfig: &models.HashicorpCloudConsulTelemetry20230414RefreshConfig{},
				},
			},
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := validateAgentTelemetryConfigPayload(tc.resp)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestConvertAgentTelemetryResponse(t *testing.T) {
	validTestURL, err := url.Parse("https://test.com/v1/metrics")
	require.NoError(t, err)

	validTestFilters, err := regexp.Compile("test|consul")
	require.NoError(t, err)

	for name, tc := range map[string]struct {
		resp                 *consul_telemetry_service.AgentTelemetryConfigOK
		expectedTelemetryCfg *TelemetryConfig
		wantErr              error
	}{
		"success": {
			resp: &consul_telemetry_service.AgentTelemetryConfigOK{
				Payload: &models.HashicorpCloudConsulTelemetry20230414AgentTelemetryConfigResponse{
					TelemetryConfig: &models.HashicorpCloudConsulTelemetry20230414TelemetryConfig{
						Endpoint: "https://test.com",
						Labels:   map[string]string{"test": "test"},
						Metrics: &models.HashicorpCloudConsulTelemetry20230414TelemetryMetricsConfig{
							IncludeList: []string{"test", "consul"},
						},
					},
					RefreshConfig: &models.HashicorpCloudConsulTelemetry20230414RefreshConfig{
						RefreshInterval: "2s",
					},
				},
			},
			expectedTelemetryCfg: &TelemetryConfig{
				MetricsConfig: &MetricsConfig{
					Endpoint: validTestURL,
					Labels:   map[string]string{"test": "test"},
					Filters:  validTestFilters,
				},
				RefreshConfig: &RefreshConfig{
					RefreshInterval: 2 * time.Second,
				},
			},
		},
		"successNoEndpoint": {
			resp: &consul_telemetry_service.AgentTelemetryConfigOK{
				Payload: &models.HashicorpCloudConsulTelemetry20230414AgentTelemetryConfigResponse{
					TelemetryConfig: &models.HashicorpCloudConsulTelemetry20230414TelemetryConfig{
						Endpoint: "",
						Labels:   map[string]string{"test": "test"},
						Metrics: &models.HashicorpCloudConsulTelemetry20230414TelemetryMetricsConfig{
							IncludeList: []string{"test", "consul"},
						},
					},
					RefreshConfig: &models.HashicorpCloudConsulTelemetry20230414RefreshConfig{
						RefreshInterval: "2s",
					},
				},
			},
			expectedTelemetryCfg: &TelemetryConfig{
				MetricsConfig: &MetricsConfig{
					Endpoint: nil,
					Labels:   map[string]string{"test": "test"},
					Filters:  validTestFilters,
					Disabled: true,
				},
				RefreshConfig: &RefreshConfig{
					RefreshInterval: 2 * time.Second,
				},
			},
		},
		"successBadFilters": {
			resp: &consul_telemetry_service.AgentTelemetryConfigOK{
				Payload: &models.HashicorpCloudConsulTelemetry20230414AgentTelemetryConfigResponse{
					TelemetryConfig: &models.HashicorpCloudConsulTelemetry20230414TelemetryConfig{
						Endpoint: "https://test.com",
						Labels:   map[string]string{"test": "test"},
						Metrics: &models.HashicorpCloudConsulTelemetry20230414TelemetryMetricsConfig{
							IncludeList: []string{"[", "(*LF)"},
						},
					},
					RefreshConfig: &models.HashicorpCloudConsulTelemetry20230414RefreshConfig{
						RefreshInterval: "2s",
					},
				},
			},
			expectedTelemetryCfg: &TelemetryConfig{
				MetricsConfig: &MetricsConfig{
					Endpoint: validTestURL,
					Labels:   map[string]string{"test": "test"},
					Filters:  DefaultMetricFilters,
				},
				RefreshConfig: &RefreshConfig{
					RefreshInterval: 2 * time.Second,
				},
			},
		},
		"errorsWithInvalidRefreshInterval": {
			resp: &consul_telemetry_service.AgentTelemetryConfigOK{
				Payload: &models.HashicorpCloudConsulTelemetry20230414AgentTelemetryConfigResponse{
					TelemetryConfig: &models.HashicorpCloudConsulTelemetry20230414TelemetryConfig{
						Metrics: &models.HashicorpCloudConsulTelemetry20230414TelemetryMetricsConfig{},
					},
					RefreshConfig: &models.HashicorpCloudConsulTelemetry20230414RefreshConfig{
						RefreshInterval: "300ws",
					},
				},
			},
			wantErr: errInvalidRefreshInterval,
		},
		"errorsWithInvalidEndpoint": {
			resp: &consul_telemetry_service.AgentTelemetryConfigOK{
				Payload: &models.HashicorpCloudConsulTelemetry20230414AgentTelemetryConfigResponse{
					TelemetryConfig: &models.HashicorpCloudConsulTelemetry20230414TelemetryConfig{
						Metrics: &models.HashicorpCloudConsulTelemetry20230414TelemetryMetricsConfig{
							Endpoint: "     ",
						},
					},
					RefreshConfig: &models.HashicorpCloudConsulTelemetry20230414RefreshConfig{
						RefreshInterval: "1s",
					},
				},
			},
			wantErr: errInvalidEndpoint,
		},
	} {
		t.Run(name, func(t *testing.T) {
			telemetryCfg, err := convertAgentTelemetryResponse(context.Background(), tc.resp, config.CloudConfig{})
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				require.Nil(t, telemetryCfg)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expectedTelemetryCfg, telemetryCfg)
		})
	}
}

func TestConvertMetricEndpoint(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		endpoint string
		override string
		expected string
		wantErr  error
	}{
		"success": {
			endpoint: "https://test.com",
			expected: "https://test.com/v1/metrics",
		},
		"successMetricsOverride": {
			endpoint: "https://test.com",
			override: "https://override.com",
			expected: "https://override.com/v1/metrics",
		},
		"noErrorWithEmptyEndpoints": {
			endpoint: "",
			override: "",
			expected: "",
		},
		"errorWithInvalidURL": {
			endpoint: "     ",
			override: "",
			wantErr:  errInvalidEndpoint,
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			u, err := convertMetricEndpoint(tc.endpoint, tc.override)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				require.Empty(t, u)
				return
			}

			if tc.expected == "" {
				require.Nil(t, u)
				require.NoError(t, err)
				return
			}

			require.NotNil(t, u)
			require.NoError(t, err)
			require.Equal(t, tc.expected, u.String())
		})
	}

}

func TestConvertMetricFilters(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		filters             []string
		expectedRegexString string
		matches             []string
		wantErr             string
		wantMatch           bool
	}{
		"badFilterRegex": {
			filters:             []string{"(*LF)"},
			expectedRegexString: DefaultMetricFilters.String(),
			matches:             []string{"consul.raft.peers", "consul.mem.heap_size"},
			wantMatch:           true,
		},
		"emptyRegex": {
			filters:             []string{},
			expectedRegexString: DefaultMetricFilters.String(),
			matches:             []string{"consul.raft.peers", "consul.mem.heap_size"},
			wantMatch:           true,
		},
		"matchFound": {
			filters:             []string{"raft.*", "mem.*"},
			expectedRegexString: "raft.*|mem.*",
			matches:             []string{"consul.raft.peers", "consul.mem.heap_size"},
			wantMatch:           true,
		},
		"matchNotFound": {
			filters:             []string{"mem.*"},
			matches:             []string{"consul.raft.peers", "consul.txn.apply"},
			expectedRegexString: "mem.*",
			wantMatch:           false,
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := convertMetricFilters(context.Background(), tc.filters)

			require.Equal(t, tc.expectedRegexString, f.String())
			for _, metric := range tc.matches {
				m := f.MatchString(metric)
				require.Equal(t, tc.wantMatch, m)
			}
		})
	}
}

func TestConvertMetricLabels(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		payloadLabels  map[string]string
		cfg            config.CloudConfig
		expectedLabels map[string]string
	}{
		"Success": {
			payloadLabels: map[string]string{
				"ctw_label": "test",
			},
			cfg: config.CloudConfig{
				NodeID:   types.NodeID("nodeyid"),
				NodeName: "nodey",
			},
			expectedLabels: map[string]string{
				"ctw_label": "test",
				"node_id":   "nodeyid",
				"node_name": "nodey",
			},
		},

		"NoNodeID": {
			payloadLabels: map[string]string{
				"ctw_label": "test",
			},
			cfg: config.CloudConfig{
				NodeID:   types.NodeID(""),
				NodeName: "nodey",
			},
			expectedLabels: map[string]string{
				"ctw_label": "test",
				"node_name": "nodey",
			},
		},
		"NoNodeName": {
			payloadLabels: map[string]string{
				"ctw_label": "test",
			},
			cfg: config.CloudConfig{
				NodeID:   types.NodeID("nodeyid"),
				NodeName: "",
			},
			expectedLabels: map[string]string{
				"ctw_label": "test",
				"node_id":   "nodeyid",
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
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			labels := convertMetricLabels(tc.payloadLabels, tc.cfg)
			require.Equal(t, labels, tc.expectedLabels)
		})
	}
}
