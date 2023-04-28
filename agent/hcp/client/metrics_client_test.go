package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
	colpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	metricpb "go.opentelemetry.io/proto/otlp/metrics/v1"
	"google.golang.org/protobuf/proto"
)

func TestNewMetricsClient(t *testing.T) {
	for name, test := range map[string]struct {
		wantErr string
		cfg     CloudConfig
		logger  hclog.Logger
	}{
		"success": {
			cfg:    &MockCloudCfg{},
			logger: hclog.NewNullLogger(),
		},
		"failsWithoutCloudCfg": {
			wantErr: "failed to init telemetry client: provide valid cloudCfg (Cloud Configuration for TLS)",
			cfg:     nil,
			logger:  hclog.NewNullLogger(),
		},
		"failsWithoutLogger": {
			wantErr: "failed to init telemetry client: provide a valid logger",
			cfg:     MockCloudCfg{},
			logger:  nil,
		},
		"failsHCPConfig": {
			wantErr: "failed to init telemetry client",
			cfg:     MockErrCloudCfg{},
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

			client, err := NewMetricsClient(MockCloudCfg{}, hclog.NewNullLogger())
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
