package client

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/hashicorp/go-hclog"
	hcpcfg "github.com/hashicorp/hcp-sdk-go/config"
	"github.com/stretchr/testify/require"
	colpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	metricpb "go.opentelemetry.io/proto/otlp/metrics/v1"
	"golang.org/x/oauth2"
	"google.golang.org/protobuf/proto"
)

type mockHCPCfg struct{}

func (m *mockHCPCfg) Token() (*oauth2.Token, error) {
	return &oauth2.Token{
		AccessToken: "test-token",
	}, nil
}

func (m *mockHCPCfg) APITLSConfig() *tls.Config { return nil }

func (m *mockHCPCfg) SCADAAddress() string { return "" }

func (m *mockHCPCfg) SCADATLSConfig() *tls.Config { return &tls.Config{} }

func (m *mockHCPCfg) APIAddress() string { return "" }

func (m *mockHCPCfg) PortalURL() *url.URL { return &url.URL{} }

type mockCloudCfg struct{}

func (m mockCloudCfg) HCPConfig(opts ...hcpcfg.HCPConfigOption) (hcpcfg.HCPConfig, error) {
	return &mockHCPCfg{}, nil
}

type mockErrCloudCfg struct{}

func (m mockErrCloudCfg) HCPConfig(opts ...hcpcfg.HCPConfigOption) (hcpcfg.HCPConfig, error) {
	return nil, errors.New("test bad HCP config")
}

func TestNewMetricsClient(t *testing.T) {
	for name, test := range map[string]struct {
		wantErr string
		cfg     cloudConfig
		logger  hclog.Logger
	}{
		"success": {
			cfg:    &mockCloudCfg{},
			logger: hclog.NewNullLogger(),
		},
		"failsWithoutCloudCfg": {
			wantErr: "failed to init telemetry client: provide valid cloudCfg (Cloud Configuration for TLS)",
			cfg:     nil,
			logger:  hclog.NewNullLogger(),
		},
		"failsWithoutLogger": {
			wantErr: "failed to init telemetry client: provide a valid logger",
			cfg:     mockCloudCfg{},
			logger:  nil,
		},
		"failsHCPConfig": {
			wantErr: "failed to init telemetry client",
			cfg:     mockErrCloudCfg{},
			logger:  hclog.NewNullLogger(),
		},
	} {
		t.Run(name, func(t *testing.T) {
			client, err := NewMetricsClient(test.cfg, test.logger)
			if test.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.wantErr)
				return
			}

			require.Nil(t, err)
			require.NotNil(t, client)
		})
	}
}

func TestExportMetrics(t *testing.T) {
	for name, test := range map[string]struct {
		wantErr string
		status  int
	}{
		"success": {
			status: http.StatusOK,
		},
		"failsWithNonRetryableError": {
			status:  http.StatusBadRequest,
			wantErr: "failed to export metrics",
		},
	} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, r.Header.Get("Content-Type"), "application/x-protobuf")

				require.Equal(t, r.Header.Get("Authorization"), "Bearer test-token")

				body := colpb.ExportMetricsServiceResponse{}

				if test.wantErr != "" {
					body.PartialSuccess = &colpb.ExportMetricsPartialSuccess{
						ErrorMessage: "partial failure",
					}
				}
				bytes, err := proto.Marshal(&body)

				require.NoError(t, err)

				w.Header().Set("Content-Type", "application/x-protobuf")
				w.WriteHeader(test.status)
				w.Write(bytes)
			}))
			defer srv.Close()

			client, err := NewMetricsClient(mockCloudCfg{}, hclog.NewNullLogger())
			require.NoError(t, err)

			ctx := context.Background()
			metrics := &metricpb.ResourceMetrics{}
			err = client.ExportMetrics(ctx, metrics, srv.URL)

			if test.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.wantErr)
				return
			}

			require.NoError(t, err)
		})
	}

}
