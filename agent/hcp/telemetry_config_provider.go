package hcp

import (
	"context"
	"net/url"
	"regexp"
	"sync"
	"time"

	goMetrics "github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"

	hcpclient "github.com/hashicorp/consul/agent/hcp/client"
	"github.com/hashicorp/consul/agent/hcp/config"
)

const (
	// defaultRefreshInterval is a default duration, after each
	defaultRefreshInterval = 5 * time.Minute
)

var (
	// internalMetricRefreshFailure is a metric to monitors refresh failures.
	internalMetricRefreshFailure []string = []string{"hcp", "telemetry_config_provider", "refresh", "failure"}
)

// TelemetryConfigProviderOpts is used to initialize a telemetryConfigProvider.
type TelemetryConfigProviderOpts struct {
	ctx             context.Context
	endpoint        *url.URL
	labels          map[string]string
	filters         *regexp.Regexp
	refreshInterval time.Duration
	cloudCfg        config.CloudConfig
	hcpClient       hcpclient.Client
}

// metricsConfig is a set of configurable settings for metrics collection, processing and export.
type metricsConfig struct {
	endpoint *url.URL
	labels   map[string]string
	filters  *regexp.Regexp
}

// telemetryConfigProvider holds metrics configuration and settings for its continuous
// fetch of new config from HCP.
type telemetryConfigProvider struct {
	// metricsConfig holds metrics configuration that can be dynamically updated
	// based on updates fetched from HCP.
	metricsConfig *metricsConfig

	// refreshInterval controls the interval at which new configuration is fetched from HCP.
	refreshInterval time.Duration

	// a reader-writer mutex is used as the provider is read heavy, as the OTEL components
	// access metricsConfig, while config is only updated (write) when there are changes.
	rw sync.RWMutex

	logger    hclog.Logger
	cloudCfg  config.CloudConfig
	hcpClient hcpclient.Client
}

func NewTelemetryConfigProvider(opts *TelemetryConfigProviderOpts) *telemetryConfigProvider {
	m := &metricsConfig{
		endpoint: opts.endpoint,
		labels:   opts.labels,
		filters:  opts.filters,
	}

	t := &telemetryConfigProvider{
		metricsConfig:   m,
		logger:          hclog.FromContext(opts.ctx).Named("telemetry_config_provider"),
		refreshInterval: opts.refreshInterval,
		cloudCfg:        opts.cloudCfg,
		hcpClient:       opts.hcpClient,
	}

	go t.run(opts.ctx)

	return t
}

// run continously checks for updates to the telemetry configuration by
// making a request to HCP, and verifying for any updated fields.
func (t *telemetryConfigProvider) run(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(t.refreshInterval))
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if m, hasChanged := t.checkUpdate(ctx); hasChanged {
				// Only update metricsConfig changes are detected
				// to decrease usage of write locks that block read locks.
				t.modifyMetricsConfig(m)
			}
		case <-ctx.Done():
			return
		}
	}
}

// checkUpdate makes a HTTP request to HCP to return a new metrics configuration and true, if config changed.
// checkUpdate does not update the metricsConfig field to prevent acquiring the write lock unnecessarily.
func (t *telemetryConfigProvider) checkUpdate(ctx context.Context) (*metricsConfig, bool) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	telemetryCfg, err := t.hcpClient.FetchTelemetryConfig(ctx)
	if err != nil {
		t.logger.Error("failed to fetch telemetry config from HCP")
		goMetrics.IncrCounter(internalMetricRefreshFailure, 1)
		return nil, false
	}

	t.rw.RLock()
	defer t.rw.RUnlock()

	// TODO: Do we want a enabled config field?
	endpoint, _ := telemetryCfg.Enabled()
	endpointURL, err := url.Parse(endpoint)
	if err != nil {
		t.logger.Error("failed to update config: invalid endpoint URL")
		goMetrics.IncrCounter(internalMetricRefreshFailure, 1)
		return nil, false
	}

	filters, err := telemetryCfg.FilterRegex()
	if err != nil {
		t.logger.Error("failed to update config: invalid filters")
		goMetrics.IncrCounter(internalMetricRefreshFailure, 1)
		return nil, false
	}

	labels := telemetryCfg.DefaultLabels(t.cloudCfg)

	newMetricsConfig := &metricsConfig{
		filters:  filters,
		endpoint: endpointURL,
		labels:   labels,
	}

	newEndpoint := endpoint != t.metricsConfig.endpoint.String()
	newFilters := filters.String() != t.metricsConfig.filters.String()
	newLabels := labelsChanged(t.metricsConfig.labels, labels)

	hasChanged := newEndpoint || newFilters || newLabels

	// TODO: Add refresh interval once added to the protos on the TGW side.
	return newMetricsConfig, hasChanged
}

// labelsChanged returns true if newLabels is different from oldLabels.
func labelsChanged(newLabels map[string]string, oldLabels map[string]string) bool {
	// if length is different, then labels have changed, so return true.
	if len(newLabels) != len(oldLabels) {
		return true
	}

	// If length is the same, we must verify k,v pairs have not changed.
	// If a new key is not in the old labels, return true.
	// If a value has changed, return true.
	for newKey, newValue := range newLabels {
		if oldValue, ok := oldLabels[newKey]; !ok || newValue != oldValue {
			return true
		}
	}

	// labels have not changed.
	return false
}

// modifyMetricsConfig acquires a write lock to modify it with a given metricsConfig object.
func (t *telemetryConfigProvider) modifyMetricsConfig(m *metricsConfig) {
	t.rw.Lock()
	defer t.rw.Unlock()

	t.metricsConfig = m
}

// GetEndpoint acquires a read lock to return endpoint configuration for consumers.
func (t *telemetryConfigProvider) GetEndpoint() *url.URL {
	t.rw.RLock()
	defer t.rw.RUnlock()

	return t.metricsConfig.endpoint
}

// GetFilters acquires a read lock to return filters configuration for consumers.
func (t *telemetryConfigProvider) GetFilters() *regexp.Regexp {
	t.rw.RLock()
	defer t.rw.RUnlock()

	return t.metricsConfig.filters
}

// GetLabels acquires a read lock to return labels configuration for consumers.
func (t *telemetryConfigProvider) GetLabels() map[string]string {
	t.rw.RLock()
	defer t.rw.RUnlock()

	return t.metricsConfig.labels
}
