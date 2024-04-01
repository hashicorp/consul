// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hcp

import (
	"context"
	"reflect"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"

	hcpclient "github.com/hashicorp/consul/agent/hcp/client"
	"github.com/hashicorp/consul/agent/hcp/config"
	"github.com/hashicorp/consul/agent/hcp/scada"
	"github.com/hashicorp/consul/lib"
)

var (
	defaultManagerMinInterval = 45 * time.Minute
	defaultManagerMaxInterval = 75 * time.Minute
)

var _ Manager = (*HCPManager)(nil)

type ManagerConfig struct {
	Client            hcpclient.Client
	CloudConfig       config.CloudConfig
	SCADAProvider     scada.Provider
	TelemetryProvider TelemetryProvider

	StatusFn StatusCallback
	// Idempotent function to upsert the HCP management token. This will be called periodically in
	// the manager's main loop.
	ManagementTokenUpserterFn ManagementTokenUpserter
	ManagementTokenDeleterFn  ManagementTokenDeleter
	MinInterval               time.Duration
	MaxInterval               time.Duration

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
type ManagementTokenUpserter func(name, secretId string) error
type ManagementTokenDeleter func(secretId string) error

//go:generate mockery --name Manager --with-expecter --inpackage
type Manager interface {
	Start(context.Context) error
	Stop() error
	GetCloudConfig() config.CloudConfig
	UpdateConfig(hcpclient.Client, config.CloudConfig)
}

type HCPManager struct {
	logger hclog.Logger

	running bool
	runLock sync.RWMutex

	cfg   ManagerConfig
	cfgMu sync.RWMutex

	updateCh chan struct{}
	stopCh   chan struct{}

	// testUpdateSent is set by unit tests to signal when the manager's status update has triggered
	testUpdateSent chan struct{}
}

// NewManager returns a Manager initialized with the given configuration.
func NewManager(cfg ManagerConfig) *HCPManager {
	return &HCPManager{
		logger: cfg.Logger,
		cfg:    cfg,

		updateCh: make(chan struct{}, 1),
	}
}

// Start executes the logic for connecting to HCP and sending periodic server updates. If the
// manager has been previously started, it will not start again.
func (m *HCPManager) Start(ctx context.Context) error {
	// Check if the manager has already started
	changed := m.setRunning(true)
	if !changed {
		m.logger.Trace("HCP manager already started")
		return nil
	}

	var err error
	m.logger.Info("HCP manager starting")

	// Update and start the SCADA provider
	err = m.startSCADAProvider()
	if err != nil {
		m.logger.Error("failed to start scada provider", "error", err)
		m.setRunning(false)
		return err
	}

	// Update and start the telemetry provider to enable the HCP metrics sink
	if err := m.startTelemetryProvider(ctx); err != nil {
		m.logger.Error("failed to update telemetry config provider", "error", err)
		m.setRunning(false)
		return err
	}

	// immediately send initial update
	select {
	case <-ctx.Done():
		m.setRunning(false)
		return nil
	case <-m.stopCh:
		return nil
	case <-m.updateCh: // empty the update chan if there is a queued update to prevent repeated update in main loop
		err = m.sendUpdate()
		if err != nil {
			m.setRunning(false)
			return err
		}
	default:
		err = m.sendUpdate()
		if err != nil {
			m.setRunning(false)
			return err
		}
	}

	// main loop
	go func() {
		for {
			m.cfgMu.RLock()
			cfg := m.cfg
			m.cfgMu.RUnlock()

			// Check for configured management token from HCP and upsert it if found
			if hcpManagement := cfg.CloudConfig.ManagementToken; len(hcpManagement) > 0 {
				if cfg.ManagementTokenUpserterFn != nil {
					upsertTokenErr := cfg.ManagementTokenUpserterFn("HCP Management Token", hcpManagement)
					if upsertTokenErr != nil {
						m.logger.Error("failed to upsert HCP management token", "err", upsertTokenErr)
					}
				}
			}

			nextUpdate := cfg.nextHeartbeat()
			if err != nil {
				m.logger.Error("failed to send server status to HCP", "err", err, "next_heartbeat", nextUpdate.String())
			}

			select {
			case <-ctx.Done():
				m.setRunning(false)
				return

			case <-m.stopCh:
				return

			case <-m.updateCh:
				err = m.sendUpdate()

			case <-time.After(nextUpdate):
				err = m.sendUpdate()
			}
		}
	}()

	return err
}

func (m *HCPManager) startSCADAProvider() error {
	provider := m.cfg.SCADAProvider
	if provider == nil {
		return nil
	}

	// Update the SCADA provider configuration with HCP configurations
	m.logger.Debug("updating scada provider with HCP configuration")
	err := provider.UpdateHCPConfig(m.cfg.CloudConfig)
	if err != nil {
		m.logger.Error("failed to update scada provider with HCP configuration", "err", err)
		return err
	}

	// Update the SCADA provider metadata
	provider.UpdateMeta(map[string]string{
		"consul_server_id": string(m.cfg.CloudConfig.NodeID),
	})

	// Start the SCADA provider
	err = provider.Start()
	if err != nil {
		return err
	}
	return nil
}

func (m *HCPManager) startTelemetryProvider(ctx context.Context) error {
	if m.cfg.TelemetryProvider == nil || reflect.ValueOf(m.cfg.TelemetryProvider).IsNil() {
		return nil
	}

	m.cfg.TelemetryProvider.Start(ctx, &HCPProviderCfg{
		HCPClient: m.cfg.Client,
		HCPConfig: &m.cfg.CloudConfig,
	})

	return nil
}

func (m *HCPManager) GetCloudConfig() config.CloudConfig {
	m.cfgMu.RLock()
	defer m.cfgMu.RUnlock()

	return m.cfg.CloudConfig
}

func (m *HCPManager) UpdateConfig(client hcpclient.Client, cloudCfg config.CloudConfig) {
	m.cfgMu.Lock()
	// Save original values
	originalCfg := m.cfg.CloudConfig
	originalClient := m.cfg.Client

	// Update with new values
	m.cfg.Client = client
	m.cfg.CloudConfig = cloudCfg
	m.cfgMu.Unlock()

	// Send update if already running and values were updated
	if m.isRunning() && (originalClient != client || originalCfg != cloudCfg) {
		m.SendUpdate()
	}
}

func (m *HCPManager) SendUpdate() {
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
func (m *HCPManager) sendUpdate() error {
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

	return cfg.Client.PushServerStatus(ctx, &s)
}

func (m *HCPManager) isRunning() bool {
	m.runLock.RLock()
	defer m.runLock.RUnlock()
	return m.running
}

// setRunning sets the running status of the manager to the given value. If the
// given value is the same as the current running status, it returns false. If
// current status is updated to the given status, it returns true.
func (m *HCPManager) setRunning(r bool) bool {
	m.runLock.Lock()
	defer m.runLock.Unlock()

	if m.running == r {
		return false
	}

	// Initialize or close the stop channel depending what running status
	// we're transitioning to. Channel must be initialized on start since
	// a provider can be stopped and started multiple times.
	if r {
		m.stopCh = make(chan struct{})
	} else {
		close(m.stopCh)
	}

	m.running = r
	return true
}

// Stop stops the manager's main loop that sends updates
// and stops the SCADA provider and telemetry provider.
func (m *HCPManager) Stop() error {
	changed := m.setRunning(false)
	if !changed {
		m.logger.Trace("HCP manager already stopped")
		return nil
	}
	m.logger.Info("HCP manager stopping")

	m.cfgMu.RLock()
	defer m.cfgMu.RUnlock()

	if m.cfg.SCADAProvider != nil {
		m.cfg.SCADAProvider.Stop()
	}

	if m.cfg.TelemetryProvider != nil && !reflect.ValueOf(m.cfg.TelemetryProvider).IsNil() {
		m.cfg.TelemetryProvider.Stop()
	}

	if m.cfg.ManagementTokenDeleterFn != nil && m.cfg.CloudConfig.ManagementToken != "" {
		err := m.cfg.ManagementTokenDeleterFn(m.cfg.CloudConfig.ManagementToken)
		if err != nil {
			return err
		}
	}

	m.logger.Info("HCP manager stopped")
	return nil
}
