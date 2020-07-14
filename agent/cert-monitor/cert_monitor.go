package certmon

import (
	"context"
	"fmt"
	"io/ioutil"
	"sync"
	"time"

	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/tlsutil"
	"github.com/hashicorp/go-hclog"
)

const (
	// ID of the roots watch
	rootsWatchID = "roots"

	// ID of the leaf watch
	leafWatchID = "leaf"
)

// Cache is an interface to represent the methods of the
// agent/cache.Cache struct that we care about
type Cache interface {
	Notify(ctx context.Context, t string, r cache.Request, correlationID string, ch chan<- cache.UpdateEvent) error
	Prepopulate(t string, result cache.FetchResult, dc string, token string, key string) error
}

// CertMonitor will setup the proper watches to ensure that
// the Agent's Connect TLS certificate remains up to date
type CertMonitor struct {
	logger          hclog.Logger
	cache           Cache
	tlsConfigurator *tlsutil.Configurator
	tokens          *token.Store
	leafReq         cachetype.ConnectCALeafRequest
	rootsReq        structs.DCSpecificRequest
	fallback        FallbackFunc
	fallbackLeeway  time.Duration
	fallbackRetry   time.Duration

	l       sync.Mutex
	running bool
	// cancel is used to cancel the entire CertMonitor
	// go routine. This is the main field protected
	// by the mutex as it being non-nil indicates that
	// the go routine has been started and is stoppable.
	// note that it doesn't indcate that the go routine
	// is currently running.
	cancel context.CancelFunc

	// cancelWatches is used to cancel the existing
	// cache watches. This is mainly only necessary
	// when the Agent token changes
	cancelWatches context.CancelFunc

	// cacheUpdates is the chan used to have the cache
	// send us back events
	cacheUpdates chan cache.UpdateEvent
	// tokenUpdates is the struct used to receive
	// events from the token store when the Agent
	// token is updated.
	tokenUpdates token.Notifier
}

// New creates a new CertMonitor for automatically rotating
// an Agent's Connect Certificate
func New(config *Config) (*CertMonitor, error) {
	logger := config.Logger
	if logger == nil {
		logger = hclog.New(&hclog.LoggerOptions{
			Level:  0,
			Output: ioutil.Discard,
		})
	}

	if config.FallbackLeeway == 0 {
		config.FallbackLeeway = 10 * time.Second
	}
	if config.FallbackRetry == 0 {
		config.FallbackRetry = time.Minute
	}

	if config.Cache == nil {
		return nil, fmt.Errorf("CertMonitor creation requires a Cache")
	}

	if config.TLSConfigurator == nil {
		return nil, fmt.Errorf("CertMonitor creation requires a TLS Configurator")
	}

	if config.Fallback == nil {
		return nil, fmt.Errorf("CertMonitor creation requires specifying a FallbackFunc")
	}

	if config.Datacenter == "" {
		return nil, fmt.Errorf("CertMonitor creation requires specifying the datacenter")
	}

	if config.NodeName == "" {
		return nil, fmt.Errorf("CertMonitor creation requires specifying the agent's node name")
	}

	if config.Tokens == nil {
		return nil, fmt.Errorf("CertMonitor creation requires specifying a token store")
	}

	return &CertMonitor{
		logger:          logger,
		cache:           config.Cache,
		tokens:          config.Tokens,
		tlsConfigurator: config.TLSConfigurator,
		fallback:        config.Fallback,
		fallbackLeeway:  config.FallbackLeeway,
		fallbackRetry:   config.FallbackRetry,
		rootsReq:        structs.DCSpecificRequest{Datacenter: config.Datacenter},
		leafReq: cachetype.ConnectCALeafRequest{
			Datacenter: config.Datacenter,
			Agent:      config.NodeName,
			DNSSAN:     config.DNSSANs,
			IPSAN:      config.IPSANs,
		},
	}, nil
}

// Update is responsible for priming the cache with the certificates
// as well as injecting them into the TLS configurator
func (m *CertMonitor) Update(certs *structs.SignedResponse) error {
	if certs == nil {
		return nil
	}

	if err := m.populateCache(certs); err != nil {
		return fmt.Errorf("error populating cache with certificates: %w", err)
	}

	connectCAPems := []string{}
	for _, ca := range certs.ConnectCARoots.Roots {
		connectCAPems = append(connectCAPems, ca.RootCert)
	}

	// Note that its expected that the private key be within the IssuedCert in the
	// SignedResponse. This isn't how a server would send back the response and requires
	// that the recipient of the response who also has access to the private key will
	// have filled it in. The Cache definitely does this but auto-encrypt/auto-config
	// will need to ensure the original response is setup this way too.
	err := m.tlsConfigurator.UpdateAutoEncrypt(
		certs.ManualCARoots,
		connectCAPems,
		certs.IssuedCert.CertPEM,
		certs.IssuedCert.PrivateKeyPEM,
		certs.VerifyServerHostname)

	if err != nil {
		return fmt.Errorf("error updating TLS configurator with certificates: %w", err)
	}

	return nil
}

// populateCache is responsible for inserting the certificates into the cache
func (m *CertMonitor) populateCache(resp *structs.SignedResponse) error {
	cert, err := connect.ParseCert(resp.IssuedCert.CertPEM)
	if err != nil {
		return fmt.Errorf("Failed to parse certificate: %w", err)
	}

	// prepolutate roots cache
	rootRes := cache.FetchResult{Value: &resp.ConnectCARoots, Index: resp.ConnectCARoots.QueryMeta.Index}
	// getting the roots doesn't require a token so in order to potentially share the cache with another
	if err := m.cache.Prepopulate(cachetype.ConnectCARootName, rootRes, m.rootsReq.Datacenter, "", m.rootsReq.CacheInfo().Key); err != nil {
		return err
	}

	// copy the template and update the token
	leafReq := m.leafReq
	leafReq.Token = m.tokens.AgentToken()

	// prepolutate leaf cache
	certRes := cache.FetchResult{
		Value: &resp.IssuedCert,
		Index: resp.ConnectCARoots.QueryMeta.Index,
		State: cachetype.ConnectCALeafSuccess(connect.EncodeSigningKeyID(cert.AuthorityKeyId)),
	}
	if err := m.cache.Prepopulate(cachetype.ConnectCALeafName, certRes, leafReq.Datacenter, leafReq.Token, leafReq.Key()); err != nil {
		return err
	}
	return nil
}

// Start spawns the go routine to monitor the certificate and ensure it is
// rotated/renewed as necessary. The chan will indicate once the started
// go routine has exited
func (m *CertMonitor) Start(ctx context.Context) (<-chan struct{}, error) {
	m.l.Lock()
	defer m.l.Unlock()

	if m.running || m.cancel != nil {
		return nil, fmt.Errorf("the CertMonitor is already running")
	}

	// create the top level context to control the go
	// routine executing the `run` method
	ctx, cancel := context.WithCancel(ctx)

	// create the channel to get cache update events through
	// really we should only ever get 10 updates
	m.cacheUpdates = make(chan cache.UpdateEvent, 10)

	// setup the cache watches
	cancelWatches, err := m.setupCacheWatches(ctx)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("error setting up cache watches: %w", err)
	}

	// start the token update notifier
	m.tokenUpdates = m.tokens.Notify(token.TokenKindAgent)

	// store the cancel funcs
	m.cancel = cancel
	m.cancelWatches = cancelWatches

	m.running = true
	exit := make(chan struct{})
	go m.run(ctx, exit)

	return exit, nil
}

// Stop manually stops the go routine spawned by Start and
// returns whether the go routine was still running before
// cancelling.
//
// Note that cancelling the context passed into Start will
// also cause the go routine to stop
func (m *CertMonitor) Stop() bool {
	m.l.Lock()
	defer m.l.Unlock()

	if !m.running {
		return false
	}

	if m.cancel != nil {
		m.cancel()
	}

	return true
}

// IsRunning returns whether the go routine to perform certificate monitoring
// is already running.
func (m *CertMonitor) IsRunning() bool {
	m.l.Lock()
	defer m.l.Unlock()
	return m.running
}

// setupCacheWatches will start both the roots and leaf cert watch with a new child
// context and an up to date ACL token. The watches are started with a new child context
// whose CancelFunc is also returned.
func (m *CertMonitor) setupCacheWatches(ctx context.Context) (context.CancelFunc, error) {
	notificationCtx, cancel := context.WithCancel(ctx)

	// copy the request
	rootsReq := m.rootsReq

	err := m.cache.Notify(notificationCtx, cachetype.ConnectCARootName, &rootsReq, rootsWatchID, m.cacheUpdates)
	if err != nil {
		cancel()
		return nil, err
	}

	// copy the request
	leafReq := m.leafReq
	leafReq.Token = m.tokens.AgentToken()

	err = m.cache.Notify(notificationCtx, cachetype.ConnectCALeafName, &leafReq, leafWatchID, m.cacheUpdates)
	if err != nil {
		cancel()
		return nil, err
	}

	return cancel, nil
}

// handleCacheEvent is used to handle event notifications from the cache for the roots
// or leaf cert watches.
func (m *CertMonitor) handleCacheEvent(u cache.UpdateEvent) error {
	switch u.CorrelationID {
	case rootsWatchID:
		m.logger.Debug("roots watch fired - updating CA certificates")
		if u.Err != nil {
			return fmt.Errorf("root watch returned an error: %w", u.Err)
		}

		roots, ok := u.Result.(*structs.IndexedCARoots)
		if !ok {
			return fmt.Errorf("invalid type for roots watch response: %T", u.Result)
		}

		var pems []string
		for _, root := range roots.Roots {
			pems = append(pems, root.RootCert)
		}

		if err := m.tlsConfigurator.UpdateAutoEncryptCA(pems); err != nil {
			return fmt.Errorf("failed to update Connect CA certificates: %w", err)
		}
	case leafWatchID:
		m.logger.Debug("leaf certificate watch fired - updating TLS certificate")
		if u.Err != nil {
			return fmt.Errorf("leaf watch returned an error: %w", u.Err)
		}

		leaf, ok := u.Result.(*structs.IssuedCert)
		if !ok {
			return fmt.Errorf("invalid type for agent leaf cert watch response: %T", u.Result)
		}
		if err := m.tlsConfigurator.UpdateAutoEncryptCert(leaf.CertPEM, leaf.PrivateKeyPEM); err != nil {
			return fmt.Errorf("failed to update the agent leaf cert: %w", err)
		}
	}

	return nil
}

// handleTokenUpdate is used when a notification about the agent token being updated
// is received and various watches need cancelling/restarting to use the new token.
func (m *CertMonitor) handleTokenUpdate(ctx context.Context) error {
	m.logger.Debug("Agent token updated - resetting watches")

	// TODO (autoencrypt) Prepopulate the cache with the new token with
	// the existing cache entry with the old token. The certificate doesn't
	// need to change just because the token has. However there isn't a
	// good way to make that happen and this behavior is benign enough
	// that I am going to push off implementing it.

	// the agent token has been updated so we must update our leaf cert watch.
	// this cancels the current watches before setting up new ones
	m.cancelWatches()

	// recreate the chan for cache updates. This is a precautionary measure to ensure
	// that we don't accidentally get notified for the new watches being setup before
	// a blocking query in the cache returns and sends data to the old chan. In theory
	// the code in agent/cache/watch.go should prevent this where we specifically check
	// for context cancellation prior to sending the event. However we could cancel
	// it after that check and finish setting up the new watches before getting the old
	// events. Both the go routine scheduler and the OS thread scheduler would have to
	// be acting up for this to happen. Regardless the way to ensure we don't get events
	// for the old watches is to simply replace the chan we are expecting them from.
	close(m.cacheUpdates)
	m.cacheUpdates = make(chan cache.UpdateEvent, 10)

	// restart watches - this will be done with the correct token
	cancelWatches, err := m.setupCacheWatches(ctx)
	if err != nil {
		return fmt.Errorf("failed to restart watches after agent token update: %w", err)
	}
	m.cancelWatches = cancelWatches
	return nil
}

// handleFallback is used when the current TLS certificate has expired and the normal
// updating mechanisms have failed to renew it quickly enough. This function will
// use the configured fallback mechanism to retrieve a new cert and start monitoring
// that one.
func (m *CertMonitor) handleFallback(ctx context.Context) error {
	m.logger.Warn("agent's client certificate has expired")
	// Background because the context is mainly useful when the agent is first starting up.
	reply, err := m.fallback(ctx)
	if err != nil {
		return fmt.Errorf("error when getting new agent certificate: %w", err)
	}

	return m.Update(reply)
}

// run is the private method to be spawn by the Start method for
// executing the main monitoring loop.
func (m *CertMonitor) run(ctx context.Context, exit chan struct{}) {
	// The fallbackTimer is used to notify AFTER the agents
	// leaf certificate has expired and where we need
	// to fall back to the less secure RPC endpoint just like
	// if the agent was starting up new.
	//
	// Check 10sec (fallback leeway duration) after cert
	// expires. The agent cache should be handling the expiration
	// and renew it before then.
	//
	// If there is no cert, AutoEncryptCertNotAfter returns
	// a value in the past which immediately triggers the
	// renew, but this case shouldn't happen because at
	// this point, auto_encrypt was just being setup
	// successfully.
	calcFallbackInterval := func() time.Duration {
		certExpiry := m.tlsConfigurator.AutoEncryptCertNotAfter()
		return certExpiry.Add(m.fallbackLeeway).Sub(time.Now())
	}
	fallbackTimer := time.NewTimer(calcFallbackInterval())

	// cleanup for once we are stopped
	defer func() {
		// cancel the go routines performing the cache watches
		m.cancelWatches()
		// ensure we don't leak the timers go routine
		fallbackTimer.Stop()
		// stop receiving notifications for token updates
		m.tokens.StopNotify(m.tokenUpdates)

		m.logger.Debug("certificate monitor has been stopped")

		m.l.Lock()
		m.cancel = nil
		m.running = false
		m.l.Unlock()

		// this should be the final cleanup task as its what notifies
		// the rest of the world that this go routine has exited.
		close(exit)
	}()

	for {
		select {
		case <-ctx.Done():
			m.logger.Debug("stopping the certificate monitor")
			return
		case <-m.tokenUpdates.Ch:
			m.logger.Debug("handling a token update event")

			if err := m.handleTokenUpdate(ctx); err != nil {
				m.logger.Error("error in handling token update event", "error", err)
			}
		case u := <-m.cacheUpdates:
			m.logger.Debug("handling a cache update event", "correlation_id", u.CorrelationID)

			if err := m.handleCacheEvent(u); err != nil {
				m.logger.Error("error in handling cache update event", "error", err)
			}

			// reset the fallback timer as the certificate may have been updated
			fallbackTimer.Stop()
			fallbackTimer = time.NewTimer(calcFallbackInterval())
		case <-fallbackTimer.C:
			// This is a safety net in case the auto_encrypt cert doesn't get renewed
			// in time. The agent would be stuck in that case because the watches
			// never use the AutoEncrypt.Sign endpoint.

			// check auto encrypt client cert expiration
			if m.tlsConfigurator.AutoEncryptCertExpired() {
				if err := m.handleFallback(ctx); err != nil {
					m.logger.Error("error when handling a certificate expiry event", "error", err)
					fallbackTimer = time.NewTimer(m.fallbackRetry)
				} else {
					fallbackTimer = time.NewTimer(calcFallbackInterval())
				}
			} else {
				// this shouldn't be possible. We calculate the timer duration to be the certificate
				// expiration time + some leeway (10s default). So whenever we get here the certificate
				// should be expired. Regardless its probably worth resetting the timer.
				fallbackTimer = time.NewTimer(calcFallbackInterval())
			}
		}
	}
}
