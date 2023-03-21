// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package hcp

import (
	"context"
	"sync"
	"time"

	hcpclient "github.com/hashicorp/consul/agent/hcp/client"
	"github.com/hashicorp/consul/agent/hcp/telemetry"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/version"
	"github.com/hashicorp/go-hclog"
)

var (
	defaultManagerMinInterval = 45 * time.Minute
	defaultManagerMaxInterval = 75 * time.Minute
)

type ManagerConfig struct {
	Client         hcpclient.Client
	NodeID         string
	MetricsBackend lib.MetricsHandler

	StatusFn    StatusCallback
	MinInterval time.Duration
	MaxInterval time.Duration

	Logger hclog.Logger
}

func (cfg *ManagerConfig) enabled() bool {
	return cfg.Client != nil && cfg.StatusFn != nil
}

func (cfg *ManagerConfig) nextHeartbeat() time.Duration {
	min := cfg.MinInterval
	if min == 0 {
		min = defaultManagerMinInterval
	}

	max := cfg.MaxInterval
	if max == 0 {
		max = defaultManagerMaxInterval
	}
	if max < min {
		max = min
	}
	return min + lib.RandomStagger(max-min)
}

type StatusCallback func(context.Context) (hcpclient.ServerStatus, error)

type Manager struct {
	logger hclog.Logger

	cfg   ManagerConfig
	cfgMu sync.RWMutex

	updateCh chan struct{}
	reporter *telemetry.Reporter

	// testUpdateSent is set by unit tests to signal when the manager's status update has triggered
	testUpdateSent chan struct{}
}

// NewManager returns an initialized Manager with a zero configuration. It won't
// do anything until UpdateConfig is called with a config that provides
// credentials to contact HCP.
func NewManager(cfg ManagerConfig) *Manager {
	return &Manager{
		logger: cfg.Logger,
		cfg:    cfg,

		updateCh: make(chan struct{}, 1),
	}
}

// runReporter initializes the metrics reporter by fetching configuration from CCM
// and runs the reporter if configured.
func (m *Manager) runReporter(ctx context.Context) {
	// Make CCM call to obtain configuration.
	telemetryCfg, err := m.cfg.Client.FetchTelemetryConfig(ctx)
	if err != nil || telemetryCfg.Endpoint == "" {
		m.logger.Error("HCP Metrics Collection failed", "error", err, "endpoint", telemetryCfg.Endpoint)
		return
	}

	cfg := telemetry.DefaultConfig()
	cfg.Logger = m.logger
	cfg.Gatherer = m.cfg.MetricsBackend
	cfg.Labels = map[string]string{
		"service.name":        "consul-server",
		"service.version":     version.GetHumanVersion(),
		"service.instance.id": m.cfg.NodeID,
	}
	// cfg.Exporter = NewExporter(telemetryCfg.Endpoint, m.cfg.Client)

	m.reporter = telemetry.NewReporter(cfg)

	m.reporter.Run(ctx)
}

// Run executes the HCP Manager.
func (m *Manager) Run(ctx context.Context) {
	m.logger.Debug("HCP manager starting")

	go m.runReporter(ctx)

	var err error
	// immediately send initial update
	select {
	case <-ctx.Done():
		return
	case <-m.updateCh: // empty the update chan if there is a queued update to prevent repeated update in main loop
		err = m.sendUpdate()
	default:
		err = m.sendUpdate()
	}

	// main loop
	for {
		m.cfgMu.RLock()
		cfg := m.cfg
		m.cfgMu.RUnlock()
		nextUpdate := cfg.nextHeartbeat()
		if err != nil {
			m.logger.Error("failed to send server status to HCP", "err", err, "next_heartbeat", nextUpdate.String())
		}

		select {
		case <-ctx.Done():
			return

		case <-m.updateCh:
			err = m.sendUpdate()

		case <-time.After(nextUpdate):
			err = m.sendUpdate()
		}
	}
}

func (m *Manager) UpdateConfig(cfg ManagerConfig) {
	m.cfgMu.Lock()
	defer m.cfgMu.Unlock()
	old := m.cfg
	m.cfg = cfg
	if old.enabled() || cfg.enabled() {
		// Only log about this if cloud is actually configured or it would be
		// confusing. We check both old and new in case we are disabling cloud or
		// enabling it or just updating it.
		m.logger.Info("updated HCP configuration")
	}

	// Send a new status update since we might have just gotten connection details
	// for the first time.
	m.SendUpdate()
}

func (m *Manager) SendUpdate() {
	m.logger.Debug("HCP triggering status update")
	select {
	case m.updateCh <- struct{}{}:
		// trigger update
	default:
		// if chan is full then there is already an update triggered that will soon
		// be acted on so don't bother blocking.
	}
}

// TODO: we should have retried on failures here with backoff but take into
// account that if a new update is triggered while we are still retrying we
// should not start another retry loop. Something like have a "dirty" flag which
// we mark on first PushUpdate and then a retry timer as well as the interval
// and a "isRetrying" state or something so that we attempt to send update, but
// then fetch fresh info on each attempt to send so if we are already in a retry
// backoff a new push is a no-op.
func (m *Manager) sendUpdate() error {
	m.cfgMu.RLock()
	cfg := m.cfg
	m.cfgMu.RUnlock()

	if !cfg.enabled() {
		return nil
	}

	if m.testUpdateSent != nil {
		defer func() {
			select {
			case m.testUpdateSent <- struct{}{}:
			default:
			}
		}()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s, err := cfg.StatusFn(ctx)
	if err != nil {
		return err
	}

	return m.cfg.Client.PushServerStatus(ctx, &s)
}
