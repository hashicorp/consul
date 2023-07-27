package client

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
	hcptelemetry "github.com/hashicorp/hcp-sdk-go/clients/cloud-consul-telemetry-gateway/preview/2023-04-14/client/consul_telemetry_service"

	"github.com/hashicorp/consul/agent/hcp/config"
)

// TelemetryConfig contains configuration for telemetry data forwarded by Consul servers
// to the HCP Telemetry gateway.
type TelemetryConfig struct {
	MetricsConfig *MetricsConfig
	RefreshConfig *RefreshConfig
}

// MetricsConfig holds metrics specific configuration within TelemetryConfig.
type MetricsConfig struct {
	Labels   map[string]string
	Filters  *regexp.Regexp
	Endpoint *url.URL
}

// RefreshConfig contains configuration for the periodic fetch of configuration from HCP.
type RefreshConfig struct {
	RefreshInterval time.Duration
}

// MetricsEnabled returns true if metrics export is enabled, i.e. a valid metrics endpoint exists.
func (t *TelemetryConfig) MetricsEnabled() bool {
	return t.MetricsConfig.Endpoint != nil
}

// validateAgentTelemetryConfigPayload ensures the returned payload from HCP is valid.
func validateAgentTelemetryConfigPayload(resp *hcptelemetry.AgentTelemetryConfigOK) error {
	if resp.Payload == nil {
		return errors.New("missing payload")
	}

	if resp.Payload.TelemetryConfig == nil {
		return errors.New("missing telemetry config")
	}

	if resp.Payload.RefreshConfig == nil {
		return errors.New("missing refresh config")
	}

	if resp.Payload.TelemetryConfig.Metrics == nil {
		return errors.New("missing metrics config")
	}

	return nil
}

// convertAgentTelemetryResponse converts an AgentTelemetryConfig payload into a TelemetryConfig object.
func convertAgentTelemetryResponse(ctx context.Context, resp *hcptelemetry.AgentTelemetryConfigOK, cfg config.CloudConfig) (*TelemetryConfig, error) {
	logger := hclog.FromContext(ctx)
	refreshInterval, err := time.ParseDuration(resp.Payload.RefreshConfig.RefreshInterval)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh interval: %w", err)
	}

	telemetryConfig := resp.Payload.TelemetryConfig

	metricsEndpoint, err := convertMetricEndpoint(telemetryConfig.Endpoint, telemetryConfig.Metrics.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse metrics endpoint: %w", err)
	}

	metricsFilters, err := convertMetricFilters(telemetryConfig.Metrics.IncludeList)
	if err != nil {
		// Do not fail on bad regex, as we can update these later on dynamically.
		logger.Error("failed to parse regex filters", "error", err)
	}

	metricLabels := convertMetricLabels(telemetryConfig.Labels, cfg)

	return &TelemetryConfig{
		MetricsConfig: &MetricsConfig{
			Endpoint: metricsEndpoint,
			Labels:   metricLabels,
			Filters:  metricsFilters,
		},
		RefreshConfig: &RefreshConfig{
			RefreshInterval: refreshInterval,
		},
	}, nil
}

// convertMetricEndpoint returns a url for the export of metrics, if a valid endpoint was obtained.
// It returns no error, and no url, if an empty endpoint is retrieved (server not registered with CCM).
// It returns an error, and no url, if a bad endpoint is retrieved.
func convertMetricEndpoint(telemetryEndpoint string, metricsEndpoint string) (*url.URL, error) {
	// Telemetry endpoint overriden by metrics specific endpoint, if given.
	endpoint := telemetryEndpoint
	if metricsEndpoint != "" {
		endpoint = metricsEndpoint
	}

	// If endpoint is empty, server not registered with CCM, no error returned.
	if endpoint == "" {
		return nil, nil
	}

	// Endpoint from CTW has no metrics path, so it must be added.
	rawUrl := endpoint + metricsGatewayPath
	u, err := url.ParseRequestURI(rawUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to parse url: %w", err)
	}

	return u, nil
}

// convertMetricFilters returns a valid regex used to filter metrics.
// It fails if there are 0 valid regex filters given.
func convertMetricFilters(payloadFilters []string) (*regexp.Regexp, error) {
	var mErr error
	filters := payloadFilters
	validFilters := make([]string, 0, len(filters))
	for _, filter := range filters {
		_, err := regexp.Compile(filter)
		if err != nil {
			mErr = multierror.Append(mErr, fmt.Errorf("compilation of filter %q failed: %w", filter, err))
			continue
		}
		validFilters = append(validFilters, filter)
	}

	if len(validFilters) == 0 {
		return nil, multierror.Append(mErr, fmt.Errorf("no valid filters"))
	}

	// Combine the valid regex strings with OR.
	finalRegex := strings.Join(validFilters, "|")
	composedRegex, err := regexp.Compile(finalRegex)
	if err != nil {
		return nil, fmt.Errorf("failed to compile regex: %w", err)
	}

	return composedRegex, nil
}

// convertMetricLabels returns a set of <key, value> string pairs that must be added as attributes to all exported telemetry data.
func convertMetricLabels(payloadLabels map[string]string, cfg config.CloudConfig) map[string]string {
	labels := make(map[string]string)
	nodeID := string(cfg.NodeID)
	if nodeID != "" {
		labels["node_id"] = nodeID
	}

	if cfg.NodeName != "" {
		labels["node_name"] = cfg.NodeName
	}

	for k, v := range payloadLabels {
		labels[k] = v
	}

	return labels
}
