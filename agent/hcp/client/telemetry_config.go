// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

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
	hcptelemetry "github.com/hashicorp/hcp-sdk-go/clients/cloud-consul-telemetry-gateway/preview/2023-04-14/client/consul_telemetry_service"

	"github.com/hashicorp/consul/agent/hcp/config"
)

var (
	// defaultMetricFilters is a regex that matches all metric names.
	DefaultMetricFilters = regexp.MustCompile(".+")

	// Validation errors for AgentTelemetryConfigOK response.
	errMissingPayload         = errors.New("missing payload")
	errMissingTelemetryConfig = errors.New("missing telemetry config")
	errMissingRefreshConfig   = errors.New("missing refresh config")
	errMissingMetricsConfig   = errors.New("missing metrics config")
	errInvalidRefreshInterval = errors.New("invalid refresh interval")
	errInvalidEndpoint        = errors.New("invalid metrics endpoint")
	errEmptyEndpoint          = errors.New("empty metrics endpoint")
)

// TelemetryConfig contains configuration for telemetry data forwarded by Consul servers
// to the HCP Telemetry gateway.
type TelemetryConfig struct {
	Endpoint      string
	MetricsConfig *MetricsConfig
	RefreshConfig *RefreshConfig
}

// MetricsConfig holds metrics specific configuration within TelemetryConfig.
type MetricsConfig struct {
	Labels   map[string]string
	Filters  *regexp.Regexp
	Endpoint *url.URL
	Disabled bool
}

// RefreshConfig contains configuration for the periodic fetch of configuration from HCP.
type RefreshConfig struct {
	RefreshInterval time.Duration
}

// validateAgentTelemetryConfigPayload ensures the returned payload from HCP is valid.
func validateAgentTelemetryConfigPayload(resp *hcptelemetry.AgentTelemetryConfigOK) error {
	if resp.Payload == nil {
		return errMissingPayload
	}

	if resp.Payload.TelemetryConfig == nil {
		return errMissingTelemetryConfig
	}

	if resp.Payload.RefreshConfig == nil {
		return errMissingRefreshConfig
	}

	if resp.Payload.TelemetryConfig.Metrics == nil {
		return errMissingMetricsConfig
	}

	return nil
}

// convertAgentTelemetryResponse converts an AgentTelemetryConfig payload into a TelemetryConfig object.
func convertAgentTelemetryResponse(ctx context.Context, resp *hcptelemetry.AgentTelemetryConfigOK, cfg config.CloudConfig) (*TelemetryConfig, error) {
	refreshInterval, err := time.ParseDuration(resp.Payload.RefreshConfig.RefreshInterval)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errInvalidRefreshInterval, err)
	}

	telemetryConfig := resp.Payload.TelemetryConfig
	metricsEndpoint, err := convertMetricEndpoint(telemetryConfig.Endpoint, telemetryConfig.Metrics.Endpoint)
	if err != nil {
		return nil, err
	}

	metricsFilters := convertMetricFilters(ctx, telemetryConfig.Metrics.IncludeList)
	metricLabels := convertMetricLabels(telemetryConfig.Labels, cfg)

	return &TelemetryConfig{
		Endpoint: telemetryConfig.Endpoint,
		MetricsConfig: &MetricsConfig{
			Endpoint: metricsEndpoint,
			Labels:   metricLabels,
			Filters:  metricsFilters,
			Disabled: telemetryConfig.Metrics.Disabled,
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

	if endpoint == "" {
		return nil, errEmptyEndpoint
	}

	// Endpoint from CTW has no metrics path, so it must be added.
	rawUrl := endpoint + metricsGatewayPath
	u, err := url.ParseRequestURI(rawUrl)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errInvalidEndpoint, err)
	}

	return u, nil
}

// convertMetricFilters returns a valid regex used to filter metrics.
// if invalid filters are given, a defaults regex that allow all metrics is returned.
func convertMetricFilters(ctx context.Context, payloadFilters []string) *regexp.Regexp {
	logger := hclog.FromContext(ctx)
	validFilters := make([]string, 0, len(payloadFilters))
	for _, filter := range payloadFilters {
		_, err := regexp.Compile(filter)
		if err != nil {
			logger.Error("invalid filter", "error", err)
			continue
		}
		validFilters = append(validFilters, filter)
	}

	if len(validFilters) == 0 {
		logger.Error("no valid filters")
		return DefaultMetricFilters
	}

	// Combine the valid regex strings with OR.
	finalRegex := strings.Join(validFilters, "|")
	composedRegex, err := regexp.Compile(finalRegex)
	if err != nil {
		logger.Error("failed to compile final regex", "error", err)
		return DefaultMetricFilters
	}

	return composedRegex
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
