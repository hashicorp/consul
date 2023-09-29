// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package servercert

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/leafcert"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib/retry"
)

// Correlation ID for leaf cert watches.
const leafWatchID = "leaf"

// LeafCertManager is an interface to represent the necessary methods of the agent/leafcert.Manager.
// It is used to request and renew the server leaf certificate.
type LeafCertManager interface {
	Notify(ctx context.Context, req *leafcert.ConnectCALeafRequest, correlationID string, ch chan<- cache.UpdateEvent) error
}

// TLSConfigurator is an interface to represent the necessary methods of the tlsutil.Configurator.
// It is used to apply the server leaf certificate and server name.
type TLSConfigurator interface {
	UpdateAutoTLSCert(pub, priv string) error
	UpdateAutoTLSPeeringServerName(name string)
}

// Store is an interface to represent the necessary methods of the state.Store.
// It is used to fetch the CA Config to getStore the trust domain in the TLSConfigurator.
type Store interface {
	CAConfig(ws memdb.WatchSet) (uint64, *structs.CAConfiguration, error)
	SystemMetadataGet(ws memdb.WatchSet, key string) (uint64, *structs.SystemMetadataEntry, error)
	AbandonCh() <-chan struct{}
}

type Config struct {
	// Datacenter is the datacenter name the server is configured with.
	Datacenter string

	// ACLsEnabled indicates whether the ACL system is enabled on this server.
	ACLsEnabled bool
}

type Deps struct {
	Config          Config
	Logger          hclog.Logger
	LeafCertManager LeafCertManager
	GetStore        func() Store
	TLSConfigurator TLSConfigurator
	waiter          retry.Waiter
}

// CertManager is responsible for requesting and renewing the leaf cert for server agents.
// The server certificate is managed internally and used for peering control-plane traffic
// to the TLS-enabled external gRPC port.
type CertManager struct {
	logger hclog.Logger

	// config contains agent configuration necessary for the cert manager to operate.
	config Config

	// leafCerts grants access to request and renew the server leaf cert.
	leafCerts LeafCertManager

	// cacheUpdateCh receives notifications of cache update events for resources watched.
	cacheUpdateCh chan cache.UpdateEvent

	// getStore returns the server state getStore for read-only access.
	getStore func() Store

	// tlsConfigurator receives the leaf cert and peering server name updates from the cert manager.
	tlsConfigurator TLSConfigurator

	// waiter contains the waiter for exponential backoff between retries.
	waiter retry.Waiter
}

func NewCertManager(deps Deps) *CertManager {
	if deps.LeafCertManager == nil {
		panic("LeafCertManager is required")
	}
	return &CertManager{
		config:          deps.Config,
		logger:          deps.Logger,
		leafCerts:       deps.LeafCertManager,
		cacheUpdateCh:   make(chan cache.UpdateEvent, 1),
		getStore:        deps.GetStore,
		tlsConfigurator: deps.TLSConfigurator,
		waiter: retry.Waiter{
			MinFailures: 1,
			MinWait:     1 * time.Second,
			MaxWait:     5 * time.Minute,
			Jitter:      retry.NewJitter(20),
		},
	}
}

func (m *CertManager) Start(ctx context.Context) error {
	if err := m.initializeWatches(ctx); err != nil {
		return fmt.Errorf("failed to set up certificate watches: %w", err)
	}
	go m.handleUpdates(ctx)

	m.logger.Info("initialized server certificate management")
	return nil
}

func (m *CertManager) initializeWatches(ctx context.Context) error {
	if m.config.ACLsEnabled {
		// If ACLs are enabled we need to watch for server token updates and set/reset
		// leaf cert updates as token updates arrive.
		go m.watchServerToken(ctx)
	} else {
		// If ACLs are disabled we set up a single cache notification for leaf certs.
		if err := m.watchLeafCert(ctx); err != nil {
			return fmt.Errorf("failed to watch leaf: %w", err)
		}
	}
	go m.watchCAConfig(ctx)

	return nil
}

func (m *CertManager) watchServerToken(ctx context.Context) {
	// We keep the last iteration's cancel function to reset watches.
	var (
		notifyCtx context.Context
		cancel    context.CancelFunc = func() {}
	)
	retryLoopBackoff(ctx, m.waiter, func() error {
		ws := memdb.NewWatchSet()
		ws.Add(m.getStore().AbandonCh())

		_, token, err := m.getStore().SystemMetadataGet(ws, structs.ServerManagementTokenAccessorID)
		if err != nil {
			return err
		}
		if token == nil {
			m.logger.Debug("ACLs have not finished initializing")
			return nil
		}
		if token.Value == "" {
			// This should never happen. If the leader stored a token with this key it will not be empty.
			return fmt.Errorf("empty token")
		}
		m.logger.Debug("server management token watch fired - resetting leaf cert watch")

		// Cancel existing the leaf cert watch and spin up new one any time the server token changes.
		// The watch needs the current token as set by the leader since certificate signing requests go to the leader.
		cancel()
		notifyCtx, cancel = context.WithCancel(ctx)

		req := leafcert.ConnectCALeafRequest{
			Datacenter: m.config.Datacenter,
			Token:      token.Value,
			Server:     true,
		}
		if err := m.leafCerts.Notify(notifyCtx, &req, leafWatchID, m.cacheUpdateCh); err != nil {
			return fmt.Errorf("failed to setup leaf cert notifications: %w", err)
		}

		ws.WatchCtx(ctx)
		return nil

	}, func(err error) {
		m.logger.Error("failed to watch server management token", "error", err)
	})
}

func (m *CertManager) watchLeafCert(ctx context.Context) error {
	req := leafcert.ConnectCALeafRequest{
		Datacenter: m.config.Datacenter,
		Server:     true,
	}
	if err := m.leafCerts.Notify(ctx, &req, leafWatchID, m.cacheUpdateCh); err != nil {
		return fmt.Errorf("failed to setup leaf cert notifications: %w", err)
	}

	return nil
}

func (m *CertManager) watchCAConfig(ctx context.Context) {
	retryLoopBackoff(ctx, m.waiter, func() error {
		ws := memdb.NewWatchSet()
		ws.Add(m.getStore().AbandonCh())

		_, conf, err := m.getStore().CAConfig(ws)
		if err != nil {
			return fmt.Errorf("failed to fetch CA configuration from the state getStore: %w", err)
		}
		if conf == nil || conf.ClusterID == "" {
			m.logger.Debug("CA has not finished initializing")
			return nil
		}

		id := connect.SpiffeIDSigningForCluster(conf.ClusterID)
		name := connect.PeeringServerSAN(m.config.Datacenter, id.Host())

		m.logger.Debug("CA config watch fired - updating auto TLS server name", "name", name)
		m.tlsConfigurator.UpdateAutoTLSPeeringServerName(name)

		ws.WatchCtx(ctx)
		return nil

	}, func(err error) {
		m.logger.Error("failed to watch CA config", "error", err)
	})
}

func retryLoopBackoff(ctx context.Context, waiter retry.Waiter, loopFn func() error, errorFn func(error)) {
	for {
		if err := waiter.Wait(ctx); err != nil {
			// The error will only be non-nil if the context is canceled.
			return
		}

		if err := loopFn(); err != nil {
			errorFn(err)
			continue
		}

		// Reset the failure count seen by the waiter if there was no error.
		waiter.Reset()
	}
}

func (m *CertManager) handleUpdates(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			m.logger.Debug("context canceled")
			return

		case event := <-m.cacheUpdateCh:
			m.logger.Debug("got cache update event", "correlationID", event.CorrelationID, "error", event.Err)

			if err := m.handleLeafUpdate(event); err != nil {
				m.logger.Error("failed to handle cache update event", "error", err)
			}
		}
	}
}

func (m *CertManager) handleLeafUpdate(event cache.UpdateEvent) error {
	if event.Err != nil {
		return fmt.Errorf("leaf cert watch returned an error: %w", event.Err)
	}
	if event.CorrelationID != leafWatchID {
		return fmt.Errorf("got unexpected update correlation ID %q while expecting %q", event.CorrelationID, leafWatchID)
	}

	leaf, ok := event.Result.(*structs.IssuedCert)
	if !ok {
		return fmt.Errorf("got invalid type in leaf cert watch response: %T", event.Result)
	}

	m.logger.Debug("leaf certificate watch fired - updating auto TLS certificate", "uri", leaf.ServerURI)

	if err := m.tlsConfigurator.UpdateAutoTLSCert(leaf.CertPEM, leaf.PrivateKeyPEM); err != nil {
		return fmt.Errorf("failed to getStore the server leaf cert: %w", err)
	}
	return nil
}
