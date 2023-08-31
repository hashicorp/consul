package autoconf

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/structs"
)

// handleCacheEvent is used to handle event notifications from the cache for the roots
// or leaf cert watches.
func (ac *AutoConfig) handleCacheEvent(u cache.UpdateEvent) error {
	switch u.CorrelationID {
	case rootsWatchID:
		ac.logger.Debug("roots watch fired - updating CA certificates")
		if u.Err != nil {
			return fmt.Errorf("root watch returned an error: %w", u.Err)
		}

		roots, ok := u.Result.(*structs.IndexedCARoots)
		if !ok {
			return fmt.Errorf("invalid type for roots watch response: %T", u.Result)
		}

		return ac.updateCARoots(roots)
	case leafWatchID:
		ac.logger.Debug("leaf certificate watch fired - updating TLS certificate")
		if u.Err != nil {
			return fmt.Errorf("leaf watch returned an error: %w", u.Err)
		}

		leaf, ok := u.Result.(*structs.IssuedCert)
		if !ok {
			return fmt.Errorf("invalid type for agent leaf cert watch response: %T", u.Result)
		}

		return ac.updateLeafCert(leaf)
	}

	return nil
}

// handleTokenUpdate is used when a notification about the agent token being updated
// is received and various watches need cancelling/restarting to use the new token.
func (ac *AutoConfig) handleTokenUpdate(ctx context.Context) error {
	ac.logger.Debug("Agent token updated - resetting watches")

	// TODO (autoencrypt) Prepopulate the cache with the new token with
	// the existing cache entry with the old token. The certificate doesn't
	// need to change just because the token has. However there isn't a
	// good way to make that happen and this behavior is benign enough
	// that I am going to push off implementing it.

	// the agent token has been updated so we must update our leaf cert watch.
	// this cancels the current watches before setting up new ones
	ac.cancelWatches()

	// recreate the chan for cache updates. This is a precautionary measure to ensure
	// that we don't accidentally get notified for the new watches being setup before
	// a blocking query in the cache returns and sends data to the old chan. In theory
	// the code in agent/cache/watch.go should prevent this where we specifically check
	// for context cancellation prior to sending the event. However we could cancel
	// it after that check and finish setting up the new watches before getting the old
	// events. Both the go routine scheduler and the OS thread scheduler would have to
	// be acting up for this to happen. Regardless the way to ensure we don't get events
	// for the old watches is to simply replace the chan we are expecting them from.
	close(ac.cacheUpdates)
	ac.cacheUpdates = make(chan cache.UpdateEvent, 10)

	// restart watches - this will be done with the correct token
	cancelWatches, err := ac.setupCertificateCacheWatches(ctx)
	if err != nil {
		return fmt.Errorf("failed to restart watches after agent token update: %w", err)
	}
	ac.cancelWatches = cancelWatches
	return nil
}

// handleFallback is used when the current TLS certificate has expired and the normal
// updating mechanisms have failed to renew it quickly enough. This function will
// use the configured fallback mechanism to retrieve a new cert and start monitoring
// that one.
func (ac *AutoConfig) handleFallback(ctx context.Context) error {
	ac.logger.Warn("agent's client certificate has expired")
	// Background because the context is mainly useful when the agent is first starting up.
	switch {
	case ac.config.AutoConfig.Enabled:
		resp, err := ac.getInitialConfiguration(ctx)
		if err != nil {
			return fmt.Errorf("error while retrieving new agent certificates via auto-config: %w", err)
		}

		return ac.recordInitialConfiguration(resp)
	case ac.config.AutoEncryptTLS:
		reply, err := ac.autoEncryptInitialCerts(ctx)
		if err != nil {
			return fmt.Errorf("error while retrieving new agent certificate via auto-encrypt: %w", err)
		}
		return ac.setInitialTLSCertificates(reply)
	default:
		return fmt.Errorf("logic error: either auto-encrypt or auto-config must be enabled")
	}
}

// run is the private method to be spawn by the Start method for
// executing the main monitoring loop.
func (ac *AutoConfig) run(ctx context.Context, exit chan struct{}) {
	// The fallbackTimer is used to notify AFTER the agents
	// leaf certificate has expired and where we need
	// to fall back to the less secure RPC endpoint just like
	// if the agent was starting up new.
	//
	// Check 10sec (fallback leeway duration) after cert
	// expires. The agent cache should be handling the expiration
	// and renew it before then.
	//
	// If there is no cert, use a value which immediately triggers the
	// renew, but this case shouldn't happen because at
	// this point, auto_encrypt was just being setup
	// successfully.
	calcFallbackInterval := func() time.Duration {
		cert := ac.acConfig.TLSConfigurator.AutoEncryptCert()
		if cert == nil {
			return -1
		}
		expiry := cert.NotAfter.Add(ac.acConfig.FallbackLeeway)
		return expiry.Sub(time.Now())
	}
	fallbackTimer := time.NewTimer(calcFallbackInterval())

	// cleanup for once we are stopped
	defer func() {
		// cancel the go routines performing the cache watches
		ac.cancelWatches()
		// ensure we don't leak the timers go routine
		fallbackTimer.Stop()
		// stop receiving notifications for token updates
		ac.acConfig.Tokens.StopNotify(ac.tokenUpdates)

		ac.logger.Debug("auto-config has been stopped")

		ac.Lock()
		ac.cancel = nil
		ac.running = false
		// this should be the final cleanup task as its what notifies
		// the rest of the world that this go routine has exited.
		close(exit)
		ac.Unlock()
	}()

	for {
		select {
		case <-ctx.Done():
			ac.logger.Debug("stopping auto-config")
			return
		case <-ac.tokenUpdates.Ch:
			ac.logger.Debug("handling a token update event")

			if err := ac.handleTokenUpdate(ctx); err != nil {
				ac.logger.Error("error in handling token update event", "error", err)
			}
		case u := <-ac.cacheUpdates:
			ac.logger.Debug("handling a cache update event", "correlation_id", u.CorrelationID)

			if err := ac.handleCacheEvent(u); err != nil {
				ac.logger.Error("error in handling cache update event", "error", err)
			}

			// reset the fallback timer as the certificate may have been updated
			fallbackTimer.Stop()
			fallbackTimer = time.NewTimer(calcFallbackInterval())
		case <-fallbackTimer.C:
			// This is a safety net in case the cert doesn't get renewed
			// in time. The agent would be stuck in that case because the watches
			// never use the AutoEncrypt.Sign endpoint.

			// check auto encrypt client cert expiration
			cert := ac.acConfig.TLSConfigurator.AutoEncryptCert()
			if cert == nil || cert.NotAfter.Before(time.Now()) {
				if err := ac.handleFallback(ctx); err != nil {
					ac.logger.Error("error when handling a certificate expiry event", "error", err)
					fallbackTimer = time.NewTimer(ac.acConfig.FallbackRetry)
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
