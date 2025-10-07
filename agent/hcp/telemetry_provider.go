// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hcp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"sync"
	"time"

	"github.com/go-openapi/runtime"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-metrics"
	"github.com/hashicorp/go-retryablehttp"

	"github.com/hashicorp/consul/agent/hcp/client"
	"github.com/hashicorp/consul/agent/hcp/config"
	"github.com/hashicorp/consul/agent/hcp/telemetry"
	"github.com/hashicorp/consul/version"
)

var (
	// internalMetricRefreshFailure is a metric to monitor refresh failures.
	internalMetricRefreshFailure []string = []string{"hcp", "telemetry_config_provider", "refresh", "failure"}
	// internalMetricRefreshSuccess is a metric to monitor refresh successes.
	internalMetricRefreshSuccess []string = []string{"hcp", "telemetry_config_provider", "refresh", "success"}
	// defaultTelemetryConfigRefreshInterval is a default fallback in case the first HCP fetch fails.
	defaultTelemetryConfigRefreshInterval = 1 * time.Minute
)

// Ensure hcpProviderImpl implements telemetry provider interfaces.
var _ TelemetryProvider = &hcpProviderImpl{}
var _ telemetry.ConfigProvider = &hcpProviderImpl{}
var _ telemetry.EndpointProvider = &hcpProviderImpl{}
var _ client.MetricsClientProvider = &hcpProviderImpl{}

// hcpProviderImpl holds telemetry configuration and settings for continuous fetch of new config from HCP.
// it updates configuration, if changes are detected.
type hcpProviderImpl struct {
	// cfg holds configuration that can be dynamically updated.
	cfg *dynamicConfig
	// httpCfg holds configuration for the HTTP client
	httpCfg *httpCfg

	// Reader-writer mutexes are used as the provider is read heavy.
	// OTEL components access telemetryConfig during metrics collection and export (read).
	// Meanwhile, configs are only updated when there are changes (write).
	rw        sync.RWMutex
	httpCfgRW sync.RWMutex

	// running indicates if the HCP telemetry config provider has been started
	running bool

	// stopCh is used to signal that the telemetry config provider should stop running.
	stopCh chan struct{}

	// hcpClient is an authenticated client used to make HTTP requests to HCP.
	hcpClient client.Client

	// logger is the HCP logger for the provider
	logger hclog.Logger

	// testUpdateConfigCh is used by unit tests to signal when an update config has occurred
	testUpdateConfigCh chan struct{}
}

// dynamicConfig is a set of configurable settings for metrics collection, processing and export.
// fields MUST be exported to compute hash for equals method.
type dynamicConfig struct {
	disabled bool
	endpoint *url.URL
	labels   map[string]string
	filters  *regexp.Regexp
	// refreshInterval controls the interval at which configuration is fetched from HCP to refresh config.
	refreshInterval time.Duration
}

// defaultDisabledCfg disables metric collection and contains default config values.
func defaultDisabledCfg() *dynamicConfig {
	return &dynamicConfig{
		labels:          map[string]string{},
		filters:         client.DefaultMetricFilters,
		refreshInterval: defaultTelemetryConfigRefreshInterval,
		endpoint:        nil,
		disabled:        true,
	}
}

// httpCfg is a set of configurable settings for the HTTP client used to export metrics
type httpCfg struct {
	header *http.Header
	client *retryablehttp.Client
}

//go:generate mockery --name TelemetryProvider --with-expecter --inpackage
type TelemetryProvider interface {
	Start(ctx context.Context, c *HCPProviderCfg) error
	Stop()
}

type HCPProviderCfg struct {
	HCPClient client.Client
	HCPConfig config.CloudConfigurer
}

// NewHCPProvider initializes and starts a HCP Telemetry provider.
func NewHCPProvider(ctx context.Context) *hcpProviderImpl {
	h := &hcpProviderImpl{
		// Initialize with default config values.
		cfg:     defaultDisabledCfg(),
		httpCfg: &httpCfg{},
		logger:  hclog.FromContext(ctx),
	}

	return h
}

// Start starts a process that continuously checks for updates to the telemetry configuration
// by making a request to HCP. It only starts running if it's not already running.
func (h *hcpProviderImpl) Start(ctx context.Context, c *HCPProviderCfg) error {
	changed := h.setRunning(true)
	if !changed {
		// Provider is already running.
		return nil
	}

	// Update the provider with the HCP configurations
	h.hcpClient = c.HCPClient
	err := h.updateHTTPConfig(c.HCPConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize HCP telemetry provider: %v", err)
	}

	go h.run(ctx)

	return nil
}

// run continuously checks for updates to the telemetry configuration by making a request to HCP.
func (h *hcpProviderImpl) run(ctx context.Context) error {
	h.logger.Debug("starting telemetry config provider")

	// Try to initialize config once before starting periodic fetch.
	h.updateConfig(ctx)

	ticker := time.NewTicker(h.getRefreshInterval())
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if newRefreshInterval := h.updateConfig(ctx); newRefreshInterval > 0 {
				ticker.Reset(newRefreshInterval)
			}
		case <-ctx.Done():
			return nil
		case <-h.stopCh:
			return nil
		}
	}
}

// updateConfig makes a HTTP request to HCP to update metrics configuration held in the provider.
func (h *hcpProviderImpl) updateConfig(ctx context.Context) time.Duration {
	logger := h.logger.Named("telemetry_config_provider")

	if h.testUpdateConfigCh != nil {
		defer func() {
			select {
			case h.testUpdateConfigCh <- struct{}{}:
			default:
			}
		}()
	}

	if h.hcpClient == nil || reflect.ValueOf(h.hcpClient).IsNil() {
		// Disable metrics if HCP client is not configured
		disabledMetricsCfg := defaultDisabledCfg()
		h.modifyDynamicCfg(disabledMetricsCfg)
		return disabledMetricsCfg.refreshInterval
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	logger.Trace("fetching telemetry config")
	telemetryCfg, err := h.hcpClient.FetchTelemetryConfig(ctx)
	if err != nil {
		// Only disable metrics on 404 or 401 to handle the case of an unlinked cluster.
		// For other errors such as 5XX ones, we continue metrics collection, as these are potentially transient server-side errors.
		apiErr, ok := err.(*runtime.APIError)
		if ok && apiErr.IsClientError() {
			disabledMetricsCfg := defaultDisabledCfg()
			h.modifyDynamicCfg(disabledMetricsCfg)
			return disabledMetricsCfg.refreshInterval
		}

		logger.Error("failed to fetch telemetry config from HCP", "error", err)
		metrics.IncrCounter(internalMetricRefreshFailure, 1)
		return 0
	}
	logger.Trace("successfully fetched telemetry config")

	// newRefreshInterval of 0 or less can cause ticker Reset() panic.
	newRefreshInterval := telemetryCfg.RefreshConfig.RefreshInterval
	if newRefreshInterval <= 0 {
		logger.Error("invalid refresh interval duration", "refreshInterval", newRefreshInterval)
		metrics.IncrCounter(internalMetricRefreshFailure, 1)
		return 0
	}

	newCfg := &dynamicConfig{
		filters:         telemetryCfg.MetricsConfig.Filters,
		endpoint:        telemetryCfg.MetricsConfig.Endpoint,
		labels:          telemetryCfg.MetricsConfig.Labels,
		refreshInterval: telemetryCfg.RefreshConfig.RefreshInterval,
		disabled:        telemetryCfg.MetricsConfig.Disabled,
	}

	h.modifyDynamicCfg(newCfg)

	return newCfg.refreshInterval
}

// modifyDynamicCfg acquires a write lock to update new configuration and emits a success metric.
func (h *hcpProviderImpl) modifyDynamicCfg(newCfg *dynamicConfig) {
	h.rw.Lock()
	h.cfg = newCfg
	h.rw.Unlock()

	metrics.IncrCounter(internalMetricRefreshSuccess, 1)
}

func (h *hcpProviderImpl) getRefreshInterval() time.Duration {
	h.rw.RLock()
	defer h.rw.RUnlock()

	return h.cfg.refreshInterval
}

// GetEndpoint acquires a read lock to return endpoint configuration for consumers.
func (h *hcpProviderImpl) GetEndpoint() *url.URL {
	h.rw.RLock()
	defer h.rw.RUnlock()

	return h.cfg.endpoint
}

// GetFilters acquires a read lock to return filters configuration for consumers.
func (h *hcpProviderImpl) GetFilters() *regexp.Regexp {
	h.rw.RLock()
	defer h.rw.RUnlock()

	return h.cfg.filters
}

// GetLabels acquires a read lock to return labels configuration for consumers.
func (h *hcpProviderImpl) GetLabels() map[string]string {
	h.rw.RLock()
	defer h.rw.RUnlock()

	return h.cfg.labels
}

// IsDisabled acquires a read lock and return true if metrics are enabled.
func (h *hcpProviderImpl) IsDisabled() bool {
	h.rw.RLock()
	defer h.rw.RUnlock()

	return h.cfg.disabled
}

// updateHTTPConfig updates the HTTP configuration values that rely on the HCP configuration.
func (h *hcpProviderImpl) updateHTTPConfig(cfg config.CloudConfigurer) error {
	h.httpCfgRW.Lock()
	defer h.httpCfgRW.Unlock()

	if cfg == nil {
		return errors.New("must provide valid HCP configuration")
	}

	// Update headers
	r, err := cfg.Resource()
	if err != nil {
		return fmt.Errorf("failed set telemetry client headers: %v", err)
	}
	header := make(http.Header)
	header.Set("content-type", "application/x-protobuf")
	header.Set("x-hcp-resource-id", r.String())
	header.Set("x-channel", fmt.Sprintf("consul/%s", version.GetHumanVersion()))
	h.httpCfg.header = &header

	// Update HTTP client
	hcpCfg, err := cfg.HCPConfig()
	if err != nil {
		return fmt.Errorf("failed to configure telemetry HTTP client: %v", err)
	}
	h.httpCfg.client = client.NewHTTPClient(hcpCfg.APITLSConfig(), hcpCfg)

	return nil
}

// GetHeader acquires a read lock to return the HTTP request headers needed
// to export metrics.
func (h *hcpProviderImpl) GetHeader() http.Header {
	h.httpCfgRW.RLock()
	defer h.httpCfgRW.RUnlock()

	if h.httpCfg.header == nil {
		return nil
	}

	return h.httpCfg.header.Clone()
}

// GetHTTPClient acquires a read lock to return the retryable HTTP client needed
// to export metrics.
func (h *hcpProviderImpl) GetHTTPClient() *retryablehttp.Client {
	h.httpCfgRW.RLock()
	defer h.httpCfgRW.RUnlock()

	return h.httpCfg.client
}

// setRunning acquires a write lock to set whether the provider is running.
// If the given value is the same as the current running status, it returns
// false. If current status is updated to the given status, it returns true.
func (h *hcpProviderImpl) setRunning(r bool) bool {
	h.rw.Lock()
	defer h.rw.Unlock()

	if h.running == r {
		return false
	}

	// Initialize or close the stop channel depending what running status
	// we're transitioning to. Channel must be initialized on start since
	// a provider can be stopped and started multiple times.
	if r {
		h.stopCh = make(chan struct{})
	} else {
		close(h.stopCh)
	}

	h.running = r

	return true
}

// Stop acquires a write lock to mark the provider as not running and sends a stop signal to the
// main run loop. It also updates the provider with a disabled configuration.
func (h *hcpProviderImpl) Stop() {
	changed := h.setRunning(false)
	if !changed {
		h.logger.Trace("telemetry config provider already stopped")
		return
	}

	h.rw.Lock()
	h.cfg = defaultDisabledCfg()
	h.rw.Unlock()

	h.logger.Debug("telemetry config provider stopped")
}
