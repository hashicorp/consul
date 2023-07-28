package hcp

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"sync"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/hashstructure/v2"

	"github.com/hashicorp/consul/agent/hcp/client"
	"github.com/hashicorp/consul/agent/hcp/telemetry"
)

var (
	// internalMetricRefreshFailure is a metric to monitor refresh failures.
	internalMetricRefreshFailure []string = []string{"hcp", "telemetry_config_provider", "refresh", "failure"}
	// internalMetricRefreshSuccess is a metric to monitor refresh successes.
	internalMetricRefreshSuccess []string = []string{"hcp", "telemetry_config_provider", "refresh", "success"}
)

// Ensure hcpProviderImpl implements telemetry provider interfaces.
var _ telemetry.ConfigProvider = &hcpProviderImpl{}
var _ telemetry.EndpointProvider = &hcpProviderImpl{}

// hasher can be implemented to compute the hash of a dynamicConfig.
type hasher interface {
	hash(cfg *dynamicConfig) (uint64, error)
}

// hasherImpl uses the hashstructure library to compute dynamicConfig hash.
type hasherImpl struct{}

// hash returns a uint64 hash value for a dynamicConfig for equality comparisons.
func (h *hasherImpl) hash(cfg *dynamicConfig) (uint64, error) {
	return hashstructure.Hash(*cfg, hashstructure.FormatV2, nil)
}

// hcpProviderImpl holds telemetry configuration and settings for continuous fetch of new config from HCP.
// it updates configuration, if changes are detected.
type hcpProviderImpl struct {
	// cfg holds configuration that can be dynamically updated.
	cfg *dynamicConfig

	// A reader-writer mutex is used as the provider is read heavy.
	// OTEL components access telemetryConfig during metrics collection and export (read).
	// Meanwhile, config is only updated when there are changes (write).
	rw sync.RWMutex
	// hcpClient is an authenticated client used to make HTTP requests to HCP.
	hcpClient client.Client
	// ticker is a reference to the time ticker that can be reset when refreshInterval changes.
	ticker *time.Ticker
	hasher hasher
}

// dynamicConfig is a set of configurable settings for metrics collection, processing and export.
// fields MUST be exported to compute hash for equals method.
type dynamicConfig struct {
	Endpoint *url.URL
	Labels   map[string]string
	Filters  *regexp.Regexp
	// refreshInterval controls the interval at which configuration is fetched from HCP to refresh config.
	RefreshInterval time.Duration
}

// NewHCPProvider initializes and starts a HCP Telemetry provider with provided params.
func NewHCPProvider(ctx context.Context, hcpClient client.Client, telemetryCfg *client.TelemetryConfig) (*hcpProviderImpl, error) {
	refreshInterval := telemetryCfg.RefreshConfig.RefreshInterval
	if refreshInterval <= 0 {
		return nil, fmt.Errorf("invalid refresh interval: %d", refreshInterval)
	}

	cfg := &dynamicConfig{
		Endpoint:        telemetryCfg.MetricsConfig.Endpoint,
		Labels:          telemetryCfg.MetricsConfig.Labels,
		Filters:         telemetryCfg.MetricsConfig.Filters,
		RefreshInterval: refreshInterval,
	}

	t := new(hcpClient, cfg)

	go t.run(ctx, t.ticker.C)

	return t, nil
}

func new(hcpClient client.Client, cfg *dynamicConfig) *hcpProviderImpl {
	return &hcpProviderImpl{
		cfg:       cfg,
		hcpClient: hcpClient,
		hasher:    &hasherImpl{},
		ticker:    time.NewTicker(cfg.RefreshInterval),
	}
}

// run continously checks for updates to the telemetry configuration by making a request to HCP.
// Modification of config only occurs if changes are detected to decrease write locks that block read locks.
func (t *hcpProviderImpl) run(ctx context.Context, tick <-chan time.Time) {
	defer t.ticker.Stop()
	for {
		select {
		case <-tick:
			if newCfg, hasChanged := t.checkUpdate(ctx); hasChanged {
				t.modifyTelemetryConfig(newCfg)
				t.ticker.Reset(newCfg.RefreshInterval)
			}
		case <-ctx.Done():
			return
		}
	}
}

// checkUpdate makes a HTTP request to HCP to return a new metrics configuration and true, if config changed.
// checkUpdate does not update the metricsConfig field to prevent acquiring the write lock unnecessarily.
func (t *hcpProviderImpl) checkUpdate(ctx context.Context) (*dynamicConfig, bool) {
	logger := hclog.FromContext(ctx).Named("telemetry_config_provider")

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	telemetryCfg, err := t.hcpClient.FetchTelemetryConfig(ctx)
	if err != nil {
		logger.Error("failed to fetch telemetry config from HCP", "error", err)
		metrics.IncrCounter(internalMetricRefreshFailure, 1)
		return nil, false
	}

	newDynamicConfig := &dynamicConfig{
		Filters:         telemetryCfg.MetricsConfig.Filters,
		Endpoint:        telemetryCfg.MetricsConfig.Endpoint,
		Labels:          telemetryCfg.MetricsConfig.Labels,
		RefreshInterval: telemetryCfg.RefreshConfig.RefreshInterval,
	}

	t.rw.RLock()
	defer t.rw.RUnlock()

	equal, err := t.equals(newDynamicConfig)
	if err != nil {
		logger.Error("failed to calculate hash for new config", "error", err)
		metrics.IncrCounter(internalMetricRefreshFailure, 1)
		return nil, false
	}

	metrics.IncrCounter(internalMetricRefreshSuccess, 1)

	return newDynamicConfig, !equal
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

	return t.cfg.Endpoint
}

// GetFilters acquires a read lock to return filters configuration for consumers.
func (t *hcpProviderImpl) GetFilters() *regexp.Regexp {
	t.rw.RLock()
	defer t.rw.RUnlock()

	return t.cfg.Filters
}

// GetLabels acquires a read lock to return labels configuration for consumers.
func (t *hcpProviderImpl) GetLabels() map[string]string {
	t.rw.RLock()
	defer t.rw.RUnlock()

	return t.cfg.Labels
}

// equals returns true if the new dynamicConfig is equal to the current config.
func (t *hcpProviderImpl) equals(newCfg *dynamicConfig) (bool, error) {
	currHash, err := t.hasher.hash(t.cfg)
	if err != nil {
		return false, err
	}

	newHash, err := t.hasher.hash(newCfg)
	if err != nil {
		return false, err
	}

	return currHash == newHash, err
}
