// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hcp

import (
	"context"
	"sync"
	"time"

	hcpclient "github.com/hashicorp/consul/agent/hcp/client"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/go-hclog"
)

var (
	defaultManagerMinInterval = 45 * time.Minute
	defaultManagerMaxInterval = 75 * time.Minute
)

type ManagerConfig struct {
	Client hcpclient.Client

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

// Run executes the Manager it's designed to be run in its own goroutine for
// the life of a server agent. It should be run even if HCP is not configured
// yet for servers since a config update might configure it later and
// UpdateConfig called. It will effectively do nothing if there are no HCP
// credentials set other than wait for some to be added.
func (m *Manager) Run(ctx context.Context) {
	var err error
	m.logger.Debug("HCP manager starting")

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
