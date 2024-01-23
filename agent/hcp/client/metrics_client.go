// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/go-retryablehttp"
	colmetricpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	metricpb "go.opentelemetry.io/proto/otlp/metrics/v1"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/agent/hcp/telemetry"
)

const (
	// defaultErrRespBodyLength refers to the max character length of the body on a failure to export metrics.
	// anything beyond we will truncate.
	defaultErrRespBodyLength = 100
)

// MetricsClientProvider provides the retryable HTTP client and headers to use for exporting metrics
// by the metrics client.
type MetricsClientProvider interface {
	GetHTTPClient() *retryablehttp.Client
	GetHeader() http.Header
}

// otlpClient is an implementation of MetricsClient with a retryable http client for retries and to honor throttle.
// It also holds default HTTP headers to add to export requests.
type otlpClient struct {
	provider MetricsClientProvider
}

// NewMetricsClient returns a configured MetricsClient.
// The current implementation uses otlpClient to provide retry functionality.
func NewMetricsClient(ctx context.Context, provider MetricsClientProvider) telemetry.MetricsClient {
	return &otlpClient{
		provider: provider,
	}
}

// ExportMetrics is the single method exposed by MetricsClient to export OTLP metrics to the desired HCP endpoint.
// The endpoint is configurable as the endpoint can change during periodic refresh of CCM telemetry config.
// By configuring the endpoint here, we can re-use the same client and override the endpoint when making a request.
func (o *otlpClient) ExportMetrics(ctx context.Context, protoMetrics *metricpb.ResourceMetrics, endpoint string) error {
	client := o.provider.GetHTTPClient()
	if client == nil {
		return errors.New("http client not configured")
	}

	pbRequest := &colmetricpb.ExportMetricsServiceRequest{
		ResourceMetrics: []*metricpb.ResourceMetrics{protoMetrics},
	}

	body, err := proto.Marshal(pbRequest)
	if err != nil {
		return fmt.Errorf("failed to marshal the request: %w", err)
	}

	req, err := retryablehttp.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header = o.provider.GetHeader()

	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("failed to post metrics: %w", err)
	}
	defer resp.Body.Close()

	var respData bytes.Buffer
	if _, err := io.Copy(&respData, resp.Body); err != nil {
		return fmt.Errorf("failed to read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		truncatedBody := truncate(respData.String(), defaultErrRespBodyLength)
		return fmt.Errorf("failed to export metrics: code %d: %s", resp.StatusCode, truncatedBody)
	}

	return nil
}

func truncate(text string, width uint) string {
	if len(text) <= int(width) {
		return text
	}
	r := []rune(text)
	trunc := r[:width]
	return string(trunc) + "..."
}
