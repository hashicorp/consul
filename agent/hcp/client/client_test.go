package client

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-openapi/runtime"
	"github.com/hashicorp/go-hclog"
	hcptelemetry "github.com/hashicorp/hcp-sdk-go/clients/cloud-consul-telemetry-gateway/preview/2023-04-14/client/consul_telemetry_service"
	"github.com/hashicorp/hcp-sdk-go/clients/cloud-consul-telemetry-gateway/preview/2023-04-14/models"
	"github.com/stretchr/testify/require"
)

type mockTGW struct {
	mockResponse *hcptelemetry.AgentTelemetryConfigOK
	mockError    error
}

func (m *mockTGW) AgentTelemetryConfig(params *hcptelemetry.AgentTelemetryConfigParams, authInfo runtime.ClientAuthInfoWriter, opts ...hcptelemetry.ClientOption) (*hcptelemetry.AgentTelemetryConfigOK, error) {
	return m.mockResponse, m.mockError
}
func (m *mockTGW) GetLabelValues(params *hcptelemetry.GetLabelValuesParams, authInfo runtime.ClientAuthInfoWriter, opts ...hcptelemetry.ClientOption) (*hcptelemetry.GetLabelValuesOK, error) {
	return hcptelemetry.NewGetLabelValuesOK(), nil
}
func (m *mockTGW) QueryRangeBatch(params *hcptelemetry.QueryRangeBatchParams, authInfo runtime.ClientAuthInfoWriter, opts ...hcptelemetry.ClientOption) (*hcptelemetry.QueryRangeBatchOK, error) {
	return hcptelemetry.NewQueryRangeBatchOK(), nil
}
func (m *mockTGW) SetTransport(transport runtime.ClientTransport) {}

func TestFetchTelemetryConfig(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		mockResponse *hcptelemetry.AgentTelemetryConfigOK
		mockError    error
		wantErr      string
	}{
		"errorsWithFetchFailure": {
			mockError:    fmt.Errorf("failed to fetch from HCP"),
			mockResponse: nil,
			wantErr:      "failed to fetch from HCP",
		},
		"errorsWithInvalidPayload": {
			mockResponse: &hcptelemetry.AgentTelemetryConfigOK{
				Payload: &models.HashicorpCloudConsulTelemetry20230414AgentTelemetryConfigResponse{},
			},
			mockError: nil,
			wantErr:   "invalid response payload",
		},
		"success:": {
			mockResponse: &hcptelemetry.AgentTelemetryConfigOK{
				Payload: &models.HashicorpCloudConsulTelemetry20230414AgentTelemetryConfigResponse{
					RefreshConfig: &models.HashicorpCloudConsulTelemetry20230414RefreshConfig{
						RefreshInterval: "1s",
					},
					TelemetryConfig: &models.HashicorpCloudConsulTelemetry20230414TelemetryConfig{
						Endpoint: "https://test.com",
						Labels:   map[string]string{"test": "123"},
						Metrics: &models.HashicorpCloudConsulTelemetry20230414TelemetryMetricsConfig{
							IncludeList: []string{"consul"},
						},
					},
				},
			},
			mockError: nil,
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := &hcpClient{
				tgw: &mockTGW{
					mockError:    tc.mockError,
					mockResponse: tc.mockResponse,
				},
				logger: hclog.NewNullLogger(),
			}

			telemetryCfg, err := c.FetchTelemetryConfig(context.Background())

			if tc.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErr)
				require.Nil(t, telemetryCfg)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, telemetryCfg)
		})
	}
}
