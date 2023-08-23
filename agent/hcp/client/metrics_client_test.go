package client

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	colpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	metricpb "go.opentelemetry.io/proto/otlp/metrics/v1"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/version"
)

func TestNewMetricsClient(t *testing.T) {
	for name, test := range map[string]struct {
		wantErr string
		cfg     CloudConfig
		ctx     context.Context
	}{
		"success": {
			cfg: &MockCloudCfg{},
			ctx: context.Background(),
		},
		"failsWithoutCloudCfg": {
			wantErr: "failed to init telemetry client: provide valid cloudCfg (Cloud Configuration for TLS)",
			cfg:     nil,
			ctx:     context.Background(),
		},
		"failsWithoutContext": {
			wantErr: "failed to init telemetry client: provide a valid context",
			cfg:     MockCloudCfg{},
			ctx:     nil,
		},
		"failsHCPConfig": {
			wantErr: "failed to init telemetry client",
			cfg: MockCloudCfg{
				ConfigErr: fmt.Errorf("test bad hcp config"),
			},
			ctx: context.Background(),
		},
		"failsBadResource": {
			wantErr: "failed to init telemetry client",
			cfg: MockCloudCfg{
				ResourceErr: fmt.Errorf("test bad resource"),
			},
			ctx: context.Background(),
		},
	} {
		t.Run(name, func(t *testing.T) {
			client, err := NewMetricsClient(test.ctx, test.cfg)
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

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZäöüÄÖÜ世界")

func randStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func TestExportMetrics(t *testing.T) {
	for name, test := range map[string]struct {
		wantErr        string
		status         int
		largeBodyError bool
	}{
		"success": {
			status: http.StatusOK,
		},
		"failsWithNonRetryableError": {
			status:  http.StatusBadRequest,
			wantErr: "failed to export metrics: code 400",
		},
		"failsWithNonRetryableErrorWithLongError": {
			status:         http.StatusBadRequest,
			wantErr:        "failed to export metrics: code 400",
			largeBodyError: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			randomBody := randStringRunes(1000)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, r.Header.Get("content-type"), "application/x-protobuf")
				require.Equal(t, r.Header.Get("x-hcp-resource-id"), testResourceID)
				require.Equal(t, r.Header.Get("x-channel"), fmt.Sprintf("consul/%s", version.GetHumanVersion()))
				require.Equal(t, r.Header.Get("Authorization"), "Bearer test-token")

				body := colpb.ExportMetricsServiceResponse{}
				bytes, err := proto.Marshal(&body)

				require.NoError(t, err)

				w.Header().Set("Content-Type", "application/x-protobuf")
				w.WriteHeader(test.status)
				if test.largeBodyError {
					w.Write([]byte(randomBody))
				} else {
					w.Write(bytes)
				}

			}))
			defer srv.Close()

			client, err := NewMetricsClient(context.Background(), MockCloudCfg{})
			require.NoError(t, err)

			ctx := context.Background()
			metrics := &metricpb.ResourceMetrics{}
			err = client.ExportMetrics(ctx, metrics, srv.URL)

			if test.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.wantErr)
				if test.largeBodyError {
					truncatedBody := truncate(randomBody, defaultErrRespBodyLength)
					require.Contains(t, err.Error(), truncatedBody)
				}
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestTruncate(t *testing.T) {
	for name, tc := range map[string]struct {
		body         string
		expectedSize int
	}{
		"ZeroSize": {
			body:         "",
			expectedSize: 0,
		},
		"LessThanSize": {
			body:         "foobar",
			expectedSize: 6,
		},
		"defaultSize": {
			body:         "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Duis vel tincidunt nunc, sed tristique risu",
			expectedSize: 100,
		},
		"greaterThanSize": {
			body:         "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Duis vel tincidunt nunc, sed tristique risus",
			expectedSize: 103,
		},
		"greaterThanSizeWithUnicode": {
			body:         randStringRunes(1000),
			expectedSize: 103,
		},
	} {
		t.Run(name, func(t *testing.T) {
			truncatedBody := truncate(tc.body, defaultErrRespBodyLength)
			truncatedRunes := []rune(truncatedBody)
			require.Equal(t, len(truncatedRunes), tc.expectedSize)
		})
	}
}
