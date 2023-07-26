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
	hcpTelemetry "github.com/hashicorp/consul/agent/hcp/telemetry"
)

var (
	// internalMetricRefreshFailure is a metric to monitors refresh failures.
	internalMetricRefreshFailure []string = []string{"hcp", "telemetry_config_provider", "refresh", "failure"}
)

// Ensure implementation of the telemetry provider interfaces.
var _ hcpTelemetry.ConfigProvider = &hcpProviderImpl{}
var _ hcpTelemetry.EndpointProvider = &hcpProviderImpl{}

// providerParams is used to initialize a hcpProviderImpl.
type providerParams struct {
	metricsConfig   *hcpclient.MetricsConfig
	refreshInterval time.Duration
	hcpClient       hcpclient.Client
}

// dynamicConfig is a set of configurable settings for metrics collection, processing and export.
type dynamicConfig struct {
	endpoint *url.URL
	labels   map[string]string
	filters  *regexp.Regexp
	// refreshInterval controls the interval at which configuration is fetched from HCP to refresh config.
	refreshInterval time.Duration
}

// hcpProviderImpl holds metrics configuration and settings for continuous fetch of new config from HCP.
type hcpProviderImpl struct {
	// cfg holds configuration that can be dynamically updated
	// based on updates fetched from HCP.
	cfg *dynamicConfig
	// cfgHash is used to compare two telemetryConfig objects to see if they are the same.
	cfgHash uint64

	// a reader-writer mutex is used as the provider is read heavy, as the OTEL components
	// access telemetryConfig, while config is only updated (write) when there are changes.
	rw        sync.RWMutex
	hcpClient hcpclient.Client
}

func NewHCPProviderImpl(ctx context.Context, params *providerParams) (*hcpProviderImpl, error) {
	if params.hcpClient == nil {
		return nil, fmt.Errorf("missing HCP client")
	}

	if params.metricsConfig == nil {
		return nil, fmt.Errorf("missing metrics config")
	}

	// TODO: should this be 0, to disable refresh?
	if params.refreshInterval <= 0 {
		return nil, fmt.Errorf("invalid refresh interval")
	}

	cfg := &dynamicConfig{
		endpoint:        params.metricsConfig.Endpoint,
		labels:          params.metricsConfig.Labels,
		filters:         params.metricsConfig.Filters,
		refreshInterval: params.refreshInterval,
	}

	hash, err := calculateHash(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate hash: %w", err)
	}

	t := &hcpProviderImpl{
		cfg:       cfg,
		cfgHash:   hash,
		hcpClient: params.hcpClient,
	}

	go t.run(ctx)

	return t, nil
}

// run continously checks for updates to the telemetry configuration by making a request to HCP.
// Modification of config only occurs if changes are detected to decrease write locks that block read locks.
func (t *hcpProviderImpl) run(ctx context.Context) {
	ticker := time.NewTicker(t.cfg.refreshInterval)
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
func (t *hcpProviderImpl) checkUpdate(ctx context.Context) (*dynamicConfig, bool) {
	t.rw.RLock()
	defer t.rw.RUnlock()
	logger := hclog.FromContext(ctx).Named("telemetry_config_provider")

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	telemetryCfg, err := t.hcpClient.FetchTelemetryConfig(ctx)
	if err != nil {
		logger.Error("failed to fetch telemetry config from HCP", "error", err)
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
		logger.Error("failed to calculate hash for new config", "error", err)
		goMetrics.IncrCounter(internalMetricRefreshFailure, 1)
		return nil, false
	}

	return newDynamicConfig, newHash == t.cfgHash
}

// modifynewTelemetryConfig acquires a write lock to modify it with a given newTelemetryConfig object.
func (t *hcpProviderImpl) modifyTelemetryConfig(newCfg *dynamicConfig) {
	t.rw.Lock()
	defer t.rw.Unlock()

	t.cfg = newCfg
}

// GetEndpoint acquires a read lock to return endpoint configuration for consumers.
func (t *hcpProviderImpl) GetEndpoint() *url.URL {
	t.rw.RLock()
	defer t.rw.RUnlock()

	return t.cfg.endpoint
}

// GetFilters acquires a read lock to return filters configuration for consumers.
func (t *hcpProviderImpl) GetFilters() *regexp.Regexp {
	t.rw.RLock()
	defer t.rw.RUnlock()

	return t.cfg.filters
}

// GetLabels acquires a read lock to return labels configuration for consumers.
func (t *hcpProviderImpl) GetLabels() map[string]string {
	t.rw.RLock()
	defer t.rw.RUnlock()

	return t.cfg.labels
}

// calculateHash returns a uint64 hash for data that can be used for comparisons.
func calculateHash(cfg *dynamicConfig) (uint64, error) {
	return hashstructure.Hash(*cfg, hashstructure.FormatV2, nil)
}
