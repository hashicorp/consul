package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-retryablehttp"
	hcpcfg "github.com/hashicorp/hcp-sdk-go/config"
	"github.com/hashicorp/hcp-sdk-go/resource"
	colmetricpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	metricpb "go.opentelemetry.io/proto/otlp/metrics/v1"
	"golang.org/x/oauth2"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/types"
	"github.com/hashicorp/consul/version"
)

const (
	// HTTP Client config
	defaultStreamTimeout = 15 * time.Second

	// Retry config
	// TODO: Eventually, we'd like to configure these values dynamically.
	defaultRetryWaitMin = 1 * time.Second
	defaultRetryWaitMax = 15 * time.Second
	// defaultRetryMax is set to 0 to turn off retry functionality, until dynamic configuration is possible.
	// This is to circumvent any spikes in load that may cause or exacerbate server-side issues for now.
	defaultRetryMax = 0
)

// MetricsClient exports Consul metrics in OTLP format to the HCP Telemetry Gateway.
type MetricsClient interface {
	ExportMetrics(ctx context.Context, protoMetrics *metricpb.ResourceMetrics, endpoint string) error
}

// cloudConfig represents cloud config for TLS abstracted in an interface for easy testing.
type CloudConfig interface {
	HCPConfig(opts ...hcpcfg.HCPConfigOption) (hcpcfg.HCPConfig, error)
	Resource() (resource.Resource, error)
	NodeMeta() (NodeID types.NodeID, NodeName string)
}

// otlpClient is an implementation of MetricsClient with a retryable http client for retries and to honor throttle.
// It also holds default HTTP headers to add to export requests.
type otlpClient struct {
	client *retryablehttp.Client
	header *http.Header
}

// NewMetricsClient returns a configured MetricsClient.
// The current implementation uses otlpClient to provide retry functionality.
func NewMetricsClient(cfg CloudConfig, ctx context.Context) (MetricsClient, error) {
	if cfg == nil {
		return nil, fmt.Errorf("failed to init telemetry client: provide valid cloudCfg (Cloud Configuration for TLS)")
	}

	if ctx == nil {
		return nil, fmt.Errorf("failed to init telemetry client: provide a valid context")
	}

	logger := hclog.FromContext(ctx)

	c, err := newHTTPClient(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to init telemetry client: %v", err)
	}

	r, err := cfg.Resource()
	if err != nil {
		return nil, fmt.Errorf("failed to init telemetry client: %v", err)
	}

	header := make(http.Header)
	header.Set("content-type", "application/x-protobuf")
	header.Set("x-hcp-resource-id", r.String())
	header.Set("x-channel", fmt.Sprintf("consul/%s", version.GetHumanVersion()))

	return &otlpClient{
		client: c,
		header: &header,
	}, nil
}

// newHTTPClient configures the retryable HTTP client.
func newHTTPClient(cloudCfg CloudConfig, logger hclog.Logger) (*retryablehttp.Client, error) {
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
		Logger:       logger.Named("hcp_telemetry_client"),
		RetryWaitMin: defaultRetryWaitMin,
		RetryWaitMax: defaultRetryWaitMax,
		RetryMax:     defaultRetryMax,
		CheckRetry:   retryablehttp.DefaultRetryPolicy,
		Backoff:      retryablehttp.DefaultBackoff,
	}

	return retryClient, nil
}

// ExportMetrics is the single method exposed by MetricsClient to export OTLP metrics to the desired HCP endpoint.
// The endpoint is configurable as the endpoint can change during periodic refresh of CCM telemetry config.
// By configuring the endpoint here, we can re-use the same client and override the endpoint when making a request.
func (o *otlpClient) ExportMetrics(ctx context.Context, protoMetrics *metricpb.ResourceMetrics, endpoint string) error {
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
	req.Header = *o.header

	resp, err := o.client.Do(req.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("failed to post metrics: %w", err)
	}
	defer resp.Body.Close()

	var respData bytes.Buffer
	if _, err := io.Copy(&respData, resp.Body); err != nil {
		return fmt.Errorf("failed to read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to export metrics: code %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
