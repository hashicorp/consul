package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/protobuf/proto"

	colmetricpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	metricpb "go.opentelemetry.io/proto/otlp/metrics/v1"

	"github.com/hashicorp/consul/agent/hcp/config"
	"github.com/hashicorp/consul/version"
	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-retryablehttp"
)

const (
	// HTTP Client config
	defaultStreamTimeout = 15 * time.Second

	// Retry config
	defaultRetryWaitMin = 15 * time.Second
	defaultRetryWaitMax = 15 * time.Second
	defaultRetryMax     = 4
)

type MetricsClient interface {
	ExportMetrics(ctx context.Context, protoMetrics *metricpb.ResourceMetrics, endpoint string) error
}

type otlpClient struct {
	client  *retryablehttp.Client
	headers map[string]string
}

type TelemetryClientCfg struct {
	cloudCfg config.CloudConfig
	logger   hclog.Logger
}

func NewMetricsClient(cfg TelemetryClientCfg) (MetricsClient, error) {
	c, err := newHTTPClient(cfg.cloudCfg, cfg.logger)
	if err != nil {
		return nil, err
	}

	headers := map[string]string{
		"X-HCP-Source-Channel": fmt.Sprintf("consul %s hcp-go-sdk/%s", version.GetHumanVersion(), version.Version),
		"Content-Type":         "application/x-protobuf",
	}

	return &otlpClient{
		client:  c,
		headers: headers,
	}, nil
}

func newHTTPClient(cloudCfg config.CloudConfig, logger hclog.Logger) (*retryablehttp.Client, error) {
	hcpCfg, err := cloudCfg.HCPConfig()
	if err != nil {
		return nil, err
	}

	tlsTransport := cleanhttp.DefaultPooledTransport()
	tlsTransport.TLSClientConfig = hcpCfg.APITLSConfig()

	var transport http.RoundTripper = &oauth2.Transport{
		Base:   tlsTransport,
		Source: hcpCfg,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   defaultStreamTimeout,
	}

	retryClient := &retryablehttp.Client{
		HTTPClient:   client,
		Logger:       logger,
		RetryWaitMin: defaultRetryWaitMin,
		RetryWaitMax: defaultRetryWaitMax,
		RetryMax:     defaultRetryMax,
		CheckRetry:   retryablehttp.DefaultRetryPolicy,
		Backoff:      retryablehttp.DefaultBackoff,
	}

	return retryClient, nil
}

func (o *otlpClient) ExportMetrics(ctx context.Context, protoMetrics *metricpb.ResourceMetrics, endpoint string) error {
	pbRequest := &colmetricpb.ExportMetricsServiceRequest{
		ResourceMetrics: []*metricpb.ResourceMetrics{protoMetrics},
	}

	body, err := proto.Marshal(pbRequest)
	if err != nil {
		return err
	}

	req, err := retryablehttp.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	for k, v := range o.headers {
		req.Header.Set(k, v)
	}

	resp, err := o.client.Do(req)
	if err != nil {
		return err
	}

	var respData bytes.Buffer
	if _, err := io.Copy(&respData, resp.Body); err != nil {
		return err
	}

	if respData.Len() != 0 {
		var respProto colmetricpb.ExportMetricsServiceResponse
		if err := proto.Unmarshal(respData.Bytes(), &respProto); err != nil {
			return err
		}

		if respProto.PartialSuccess != nil {
			msg := respProto.PartialSuccess.GetErrorMessage()
			return fmt.Errorf("failed to upload metrics: partial success: %s", msg)
		}
	}

	return nil
}
