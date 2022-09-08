package servercert

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib/retry"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
)

// Correlation ID for leaf cert watches.
const leafWatchID = "leaf"

// Cache is an interface to represent the necessary methods of the agent/cache.Cache.
// It is used to request and renew the server leaf certificate.
type Cache interface {
	Notify(ctx context.Context, t string, r cache.Request, correlationID string, ch chan<- cache.UpdateEvent) error
}

// TLSConfigurator is an interface to represent the necessary methods of the tlsutil.Configurator.
// It is used to apply the server leaf certificate and trust domain.
type TLSConfigurator interface {
	UpdateAutoTLSCert(pub, priv string) error
	UpdateAutoTLSPeeringServerName(name string)
}

// Store is an interface to represent the necessary methods of the state.Store.
// It is used to fetch the CA Config to store the trust domain in the TLSConfigurator.
type Store interface {
	CAConfig(ws memdb.WatchSet) (uint64, *structs.CAConfiguration, error)
	AbandonCh() <-chan struct{}
}

type Config struct {
	// Datacenter is the datacenter name the server is configured with.
	Datacenter string

	// Token is the ACL token for the server to use in cache requests.
	Token string
}

type Deps struct {
	Config          Config
	Logger          hclog.Logger
	Cache           Cache
	Store           Store
	TlsConfigurator TLSConfigurator
}

type CertManager struct {
	datacenter string
	token      string

	logger          hclog.Logger
	cache           Cache
	cacheUpdateCh   chan cache.UpdateEvent
	store           Store
	tlsConfigurator TLSConfigurator
}

func NewCertManager(deps Deps) *CertManager {
	return &CertManager{
		datacenter:      deps.Config.Datacenter,
		token:           deps.Config.Token,
		logger:          deps.Logger,
		cache:           deps.Cache,
		cacheUpdateCh:   make(chan cache.UpdateEvent),
		store:           deps.Store,
		tlsConfigurator: deps.TlsConfigurator,
	}
}

func (m *CertManager) Start(ctx context.Context) error {
	if err := m.initializeWatches(ctx); err != nil {
		return fmt.Errorf("failed to set up certificate watches: %w", err)
	}

	go m.run(ctx)

	m.logger.Info("initialized server certificate management")
	return nil
}

func (m *CertManager) initializeWatches(ctx context.Context) error {
	if err := m.watchLeafCert(ctx); err != nil {
		return fmt.Errorf("failed to watch leaf certificate: %w", err)
	}
	go m.watchCAConfig(ctx)

	return nil
}

func (m *CertManager) watchLeafCert(ctx context.Context) error {
	req := cachetype.ConnectCALeafRequest{
		Datacenter: m.datacenter,
		Token:      m.token,
		Server:     true,
	}
	if err := m.cache.Notify(ctx, cachetype.ConnectCALeafName, &req, leafWatchID, m.cacheUpdateCh); err != nil {
		return fmt.Errorf("failed to setup notifications: %w", err)
	}
	return nil
}

func (m *CertManager) watchCAConfig(ctx context.Context) {
	waiter := retry.Waiter{
		MinFailures: 5,
		MinWait:     1 * time.Second,
		MaxWait:     5 * time.Second,
		Jitter:      retry.NewJitter(20),
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		waiter.Wait(ctx)

		ws := memdb.NewWatchSet()
		ws.Add(m.store.AbandonCh())
		ws.Add(ctx.Done())

		_, conf, err := m.store.CAConfig(ws)
		if err != nil {
			m.logger.Error("failed to fetch CA configuration from the state store: %w", err)
			continue
		}
		if conf == nil || conf.ClusterID == "" {
			m.logger.Debug("CA has not finished initializing")
			continue
		}

		id := connect.SpiffeIDSigningForCluster(conf.ClusterID)
		name := connect.PeeringServerSAN(m.datacenter, id.Host())

		m.logger.Debug("CA config watch fired - updating auto TLS server name", "name", name)
		m.tlsConfigurator.UpdateAutoTLSPeeringServerName(name)

		ws.WatchCtx(ctx)
	}
}

func (m *CertManager) run(ctx context.Context) {
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
		return fmt.Errorf("leaf watch returned an error: %w", event.Err)
	}
	if event.CorrelationID != leafWatchID {
		return fmt.Errorf("unexpected update correlation ID %q while expecting %q", event.CorrelationID, leafWatchID)
	}

	leaf, ok := event.Result.(*structs.IssuedCert)
	if !ok {
		return fmt.Errorf("received invalid type in leaf cert watch response: %T", event.Result)
	}
	m.logger.Debug("leaf certificate watch fired - updating auto TLS certificate", "uri", leaf.ServerURI)

	if err := m.tlsConfigurator.UpdateAutoTLSCert(leaf.CertPEM, leaf.PrivateKeyPEM); err != nil {
		return fmt.Errorf("failed to update the agent leaf cert: %w", err)
	}
	return nil
}
