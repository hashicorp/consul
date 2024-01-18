// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"context"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	colpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	metricpb "go.opentelemetry.io/proto/otlp/metrics/v1"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/go-retryablehttp"
)

type mockClientProvider struct {
	client *retryablehttp.Client
	header *http.Header
}

func (m *mockClientProvider) GetHTTPClient() *retryablehttp.Client { return m.client }
func (m *mockClientProvider) GetHeader() http.Header               { return m.header.Clone() }

func newMockClientProvider() *mockClientProvider {
	header := make(http.Header)
	header.Set("content-type", "application/x-protobuf")

	client := retryablehttp.NewClient()

	return &mockClientProvider{
		header: &header,
		client: client,
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
		mutateProvider func(*mockClientProvider)
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
		"failsWithClientNotConfigured": {
			mutateProvider: func(m *mockClientProvider) {
				m.client = nil
			},
			wantErr: "http client not configured",
		},
	} {
		t.Run(name, func(t *testing.T) {
			randomBody := randStringRunes(1000)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, r.Header.Get("content-type"), "application/x-protobuf")

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

			provider := newMockClientProvider()
			if test.mutateProvider != nil {
				test.mutateProvider(provider)
			}
			client := NewMetricsClient(context.Background(), provider)

			ctx := context.Background()
			metrics := &metricpb.ResourceMetrics{}
			err := client.ExportMetrics(ctx, metrics, srv.URL)

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
