package hcp

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"sync"
	"time"

	goMetrics "github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/hashstructure/v2"

	hcpclient "github.com/hashicorp/consul/agent/hcp/client"
)

var (
	// internalMetricRefreshFailure is a metric to monitors refresh failures.
	internalMetricRefreshFailure []string = []string{"hcp", "telemetry_config_provider", "refresh", "failure"}
)

// TelemetryConfigProviderOpts is used to initialize a telemetryConfigProvider.
type TelemetryConfigProviderOpts struct {
	Ctx             context.Context
	MetricsConfig   *hcpclient.MetricsConfig
	RefreshInterval time.Duration
	HCPClient       hcpclient.Client
}

// dynamicConfig is a set of configurable settings for metrics collection, processing and export.
type dynamicConfig struct {
	endpoint *url.URL
	labels   map[string]string
	filters  *regexp.Regexp
	// refreshInterval controls the interval at which configuration is fetched from HCP to refresh config.
	refreshInterval time.Duration
}

// telemetryConfigProvider holds metrics configuration and settings for continuous fetch of new config from HCP.
type telemetryConfigProvider struct {
	// telemetryConfig holds configuration that can be dynamically updated
	// based on updates fetched from HCP.
	cfg *dynamicConfig
	// telemetryConfigHash is used to compare two telemetryConfig objects to see if they are the same.
	cfgHash uint64

	// a reader-writer mutex is used as the provider is read heavy, as the OTEL components
	// access telemetryConfig, while config is only updated (write) when there are changes.
	rw        sync.RWMutex
	logger    hclog.Logger
	hcpClient hcpclient.Client
}

func NewTelemetryConfigProvider(opts *TelemetryConfigProviderOpts) (*telemetryConfigProvider, error) {
	if opts.Ctx == nil {
		return nil, fmt.Errorf("missing ctx")
	}

	if opts.HCPClient == nil {
		return nil, fmt.Errorf("missing HCP client")
	}

	if opts.MetricsConfig == nil {
		return nil, fmt.Errorf("missing metrics config")
	}

	if opts.RefreshInterval <= 0 {
		return nil, fmt.Errorf("invalid refresh interval")
	}

	cfg := &dynamicConfig{
		endpoint:        opts.MetricsConfig.Endpoint,
		labels:          opts.MetricsConfig.Labels,
		filters:         opts.MetricsConfig.Filters,
		refreshInterval: opts.RefreshInterval,
	}

	hash, err := calculateHash(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate hash: %w", err)
	}

	t := &telemetryConfigProvider{
		cfg:       cfg,
		cfgHash:   hash,
		logger:    hclog.FromContext(opts.Ctx).Named("telemetry_config_provider"),
		hcpClient: opts.HCPClient,
	}

	go t.run(opts.Ctx, opts.RefreshInterval)

	return t, nil
}

// run continously checks for updates to the telemetry configuration by making a request to HCP.
// Modification of config only occurs if changes are detected to decrease write locks that block read locks.
func (t *telemetryConfigProvider) run(ctx context.Context, refreshInterval time.Duration) {
	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if newCfg, hasChanged := t.checkUpdate(ctx); hasChanged {
				t.modifyTelemetryConfig(newCfg)
				ticker.Reset(newCfg.refreshInterval)
			}
		case <-ctx.Done():
			return
		}
	}
}

// checkUpdate makes a HTTP request to HCP to return a new metrics configuration and true, if config changed.
// checkUpdate does not update the metricsConfig field to prevent acquiring the write lock unnecessarily.
func (t *telemetryConfigProvider) checkUpdate(ctx context.Context) (*dynamicConfig, bool) {
	t.rw.RLock()
	defer t.rw.RUnlock()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	telemetryCfg, err := t.hcpClient.FetchTelemetryConfig(ctx)
	if err != nil {
		t.logger.Error("failed to fetch telemetry config from HCP", "error", err)
		goMetrics.IncrCounter(internalMetricRefreshFailure, 1)
		return nil, false
	}

	newDynamicConfig := &dynamicConfig{
		filters:         telemetryCfg.MetricsConfig.Filters,
		endpoint:        telemetryCfg.MetricsConfig.Endpoint,
		labels:          telemetryCfg.MetricsConfig.Labels,
		refreshInterval: telemetryCfg.RefreshConfig.RefreshInterval,
	}

	newHash, err := calculateHash(newDynamicConfig)
	if err != nil {
		t.logger.Error("failed to calculate hash for new config", "error", err)
		goMetrics.IncrCounter(internalMetricRefreshFailure, 1)
		return nil, false
	}

	return newDynamicConfig, newHash == t.cfgHash
}

// modifynewTelemetryConfig acquires a write lock to modify it with a given newTelemetryConfig object.
func (t *telemetryConfigProvider) modifyTelemetryConfig(newCfg *dynamicConfig) {
	t.rw.Lock()
	defer t.rw.Unlock()

	t.cfg = newCfg
}

// GetEndpoint acquires a read lock to return endpoint configuration for consumers.
func (t *telemetryConfigProvider) GetEndpoint() *url.URL {
	t.rw.RLock()
	defer t.rw.RUnlock()

	return t.cfg.endpoint
}

// GetFilters acquires a read lock to return filters configuration for consumers.
func (t *telemetryConfigProvider) GetFilters() *regexp.Regexp {
	t.rw.RLock()
	defer t.rw.RUnlock()

	return t.cfg.filters
}

// GetLabels acquires a read lock to return labels configuration for consumers.
func (t *telemetryConfigProvider) GetLabels() map[string]string {
	t.rw.RLock()
	defer t.rw.RUnlock()

	return t.cfg.labels
}

// calculateHash returns a uint64 hash for data that can be used for comparisons.
func calculateHash(cfg *dynamicConfig) (uint64, error) {
	return hashstructure.Hash(*cfg, hashstructure.FormatV2, nil)
}
