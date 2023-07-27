package hcp

import (
	"context"
	"net/url"
	"regexp"
	"sync"
	"time"

	"github.com/armon/go-metrics"
	"github.com/mitchellh/hashstructure/v2"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/hcp/client"
	"github.com/hashicorp/consul/agent/hcp/telemetry"
)

var (
	// internalMetricRefreshFailure is a metric to monitor refresh failures.
	internalMetricRefreshFailure []string = []string{"hcp", "telemetry_config_provider", "refresh", "failure"}
	// internalMetricRefreshFailure is a metric to monitor refresh successes.
	internalMetricRefreshSuccess []string = []string{"hcp", "telemetry_config_provider", "refresh", "success"}

	defaultTelemetryConfigRefreshInterval = 5 * time.Minute
	defaultTelemetryConfigFilters         = regexp.MustCompile(".+")
)

// Ensure hcpProviderImpl implements telemetry provider interfaces.
var _ telemetry.ConfigProvider = &hcpProviderImpl{}
var _ telemetry.EndpointProvider = &hcpProviderImpl{}

// hcpProviderImpl holds telemetry configuration and settings for continuous fetch of new config from HCP.
// it updates configuration, if changes are detected.
type hcpProviderImpl struct {
	// cfg holds configuration that can be dynamically updated.
	cfg *dynamicConfig
	// cfgHash is used to check if two dynamicConfig objects are equal.
	cfgHash uint64

	// A reader-writer mutex is used as the provider is read heavy.
	// OTEL components access telemetryConfig during metrics collection and export (read).
	// Meanwhile, config is only updated when there are changes (write).
	rw sync.RWMutex
	// hcpClient is an authenticated client used to make HTTP requests to HCP.
	hcpClient client.Client
}

// dynamicConfig is a set of configurable settings for metrics collection, processing and export.
type dynamicConfig struct {
	endpoint *url.URL
	labels   map[string]string
	filters  *regexp.Regexp
	// refreshInterval controls the interval at which configuration is fetched from HCP to refresh config.
	refreshInterval time.Duration
}

// providerParams is used to initialize a hcpProviderImpl.
type providerParams struct {
	metricsConfig   *client.MetricsConfig
	refreshInterval time.Duration
	hcpClient       client.Client
}

// NewHCPProviderImpl initializes and starts a HCP Telemetry provider with provided params.
func NewHCPProviderImpl(ctx context.Context, hcpClient client.Client) *hcpProviderImpl {
	t := &hcpProviderImpl{
		hcpClient: hcpClient,
	}
	go t.run(ctx)

	return t
}

// run continously checks for updates to the telemetry configuration by making a request to HCP.
// Modification of config only occurs if changes are detected to decrease write locks that block read locks.
func (t *hcpProviderImpl) run(ctx context.Context) {
	ticker := time.NewTicker(defaultTelemetryConfigRefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if newCfg, newHash, hasChanged := t.checkUpdate(ctx); hasChanged {
				t.modifyTelemetryConfig(newCfg, newHash)
				ticker.Reset(newCfg.refreshInterval)
			}
		case <-ctx.Done():
			return
		}
	}
}

// checkUpdate makes a HTTP request to HCP to return a new metrics configuration and true, if config changed.
// checkUpdate does not update the metricsConfig field to prevent acquiring the write lock unnecessarily.
func (t *hcpProviderImpl) checkUpdate(ctx context.Context) (*dynamicConfig, uint64, bool) {
	logger := hclog.FromContext(ctx).Named("telemetry_config_provider")

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	telemetryCfg, err := t.hcpClient.FetchTelemetryConfig(ctx)
	if err != nil {
		logger.Error("failed to fetch telemetry config from HCP", "error", err)
		metrics.IncrCounter(internalMetricRefreshFailure, 1)
		return nil, 0, false
	}

	newDynamicConfig := &dynamicConfig{
		filters:         telemetryCfg.MetricsConfig.Filters,
		endpoint:        telemetryCfg.MetricsConfig.Endpoint,
		labels:          telemetryCfg.MetricsConfig.Labels,
		refreshInterval: telemetryCfg.RefreshConfig.RefreshInterval,
	}

	newHash, err := calculateHash(newDynamicConfig)
	if err != nil {
		logger.Error("failed to calculate hash for new config", "error", err)
		metrics.IncrCounter(internalMetricRefreshFailure, 1)
		return nil, 0, false
	}

	metrics.IncrCounter(internalMetricRefreshSuccess, 1)

	t.rw.RLock()
	defer t.rw.RUnlock()

	return newDynamicConfig, newHash, newHash == t.cfgHash
}

// modifynewTelemetryConfig acquires a write lock to modify it with a given newTelemetryConfig object.
func (t *hcpProviderImpl) modifyTelemetryConfig(newCfg *dynamicConfig, newHash uint64) {
	t.rw.Lock()
	defer t.rw.Unlock()

	t.cfg = newCfg
	t.cfgHash = newHash
}

// GetEndpoint acquires a read lock to return endpoint configuration for consumers.
func (t *hcpProviderImpl) GetEndpoint() (*url.URL, bool) {
	t.rw.RLock()
	defer t.rw.RUnlock()

	if t.cfg == nil {
		return nil, false
	}

	return t.cfg.endpoint, true
}

// GetFilters acquires a read lock to return filters configuration for consumers.
func (t *hcpProviderImpl) GetFilters() *regexp.Regexp {
	t.rw.RLock()
	defer t.rw.RUnlock()

	if t.cfg == nil {
		return defaultTelemetryConfigFilters
	}

	return t.cfg.filters
}

// GetLabels acquires a read lock to return labels configuration for consumers.
func (t *hcpProviderImpl) GetLabels() map[string]string {
	t.rw.RLock()
	defer t.rw.RUnlock()

	if t.cfg == nil {
		return map[string]string{}
	}

	return t.cfg.labels
}

// calculateHash returns a uint64 hash for data that can be used for comparisons.
func calculateHash(cfg *dynamicConfig) (uint64, error) {
	return hashstructure.Hash(*cfg, hashstructure.FormatV2, nil)
}
