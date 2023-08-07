// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package leafcert

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
)

// caChangeJitterWindow is the time over which we spread each round of retries
// when attempting to get a new certificate following a root rotation. It's
// selected to be a trade-off between not making rotation unnecessarily slow on
// a tiny cluster while not hammering the servers on a huge cluster
// unnecessarily hard. Servers rate limit to protect themselves from the
// expensive crypto work, but in practice have 10k+ RPCs all in the same second
// will cause a major disruption even on large servers due to downloading the
// payloads, parsing msgpack etc. Instead we pick a window that for now is fixed
// but later might be either user configurable (not nice since it would become
// another hard-to-tune value) or set dynamically by the server based on it's
// knowledge of how many certs need to be rotated. Currently the server doesn't
// know that so we pick something that is reasonable. We err on the side of
// being slower that we need in trivial cases but gentler for large deployments.
// 30s means that even with a cluster of 10k service instances, the server only
// has to cope with ~333 RPCs a second which shouldn't be too bad if it's rate
// limiting the actual expensive crypto work.
//
// The actual backoff strategy when we are rate limited is to have each cert
// only retry once with each window of this size, at a point in the window
// selected at random. This performs much better than exponential backoff in
// terms of getting things rotated quickly with more predictable load and so
// fewer rate limited requests. See the full simulation this is based on at
// https://github.com/banks/sim-rate-limit-backoff/blob/master/README.md for
// more detail.
const caChangeJitterWindow = 30 * time.Second

// NOTE: this function only has one goroutine in it per key at all times
func (m *Manager) attemptLeafRefresh(
	req *ConnectCALeafRequest,
	existing *structs.IssuedCert,
	state fetchState,
) (*structs.IssuedCert, fetchState, error) {
	if req.MaxQueryTime <= 0 {
		req.MaxQueryTime = DefaultQueryTimeout
	}

	// Handle brand new request first as it's simplest.
	if existing == nil {
		return m.generateNewLeaf(req, state, true)
	}

	// We have a certificate in cache already. Check it's still valid.
	now := time.Now()
	minExpire, maxExpire := calculateSoftExpiry(now, existing)
	expiresAt := minExpire.Add(lib.RandomStagger(maxExpire.Sub(minExpire)))

	// Check if we have been force-expired by a root update that jittered beyond
	// the timeout of the query it was running.
	if !state.forceExpireAfter.IsZero() && state.forceExpireAfter.Before(expiresAt) {
		expiresAt = state.forceExpireAfter
	}

	if expiresAt.Equal(now) || expiresAt.Before(now) {
		// Already expired, just make a new one right away
		return m.generateNewLeaf(req, state, false)
	}

	// If we called Get() with MustRevalidate then this call came from a non-blocking query.
	// Any prior CA rotations should've already expired the cert.
	// All we need to do is check whether the current CA is the one that signed the leaf. If not, generate a new leaf.
	// This is not a perfect solution (as a CA rotation update can be missed) but it should take care of instances like
	// see https://github.com/hashicorp/consul/issues/10871, https://github.com/hashicorp/consul/issues/9862
	// This seems to me like a hack, so maybe we can revisit the caching/ fetching logic in this case
	if req.MustRevalidate {
		roots, err := m.rootsReader.Get()
		if err != nil {
			return nil, state, err
		} else if roots == nil {
			return nil, state, errors.New("no CA roots")
		}
		if activeRootHasKey(roots, state.authorityKeyID) {
			return nil, state, nil
		}

		// if we reach here then the current leaf was not signed by the same CAs, just regen
		return m.generateNewLeaf(req, state, false)
	}

	// We are about to block and wait for a change or timeout.

	// Make a chan we can be notified of changes to CA roots on. It must be
	// buffered so we don't miss broadcasts from rootsWatch. It is an edge trigger
	// so a single buffer element is sufficient regardless of whether we consume
	// the updates fast enough since as soon as we see an element in it, we will
	// reload latest CA from cache.
	rootUpdateCh := make(chan struct{}, 1)

	// The roots may have changed in between blocking calls. We need to verify
	// that the existing cert was signed by the current root. If it was we still
	// want to do the whole jitter thing. We could code that again here but it's
	// identical to the select case below so we just trigger our own update chan
	// and let the logic below handle checking if the CA actually changed in the
	// common case where it didn't it is a no-op anyway.
	rootUpdateCh <- struct{}{}

	// Subscribe our chan to get root update notification.
	m.rootWatcher.Subscribe(rootUpdateCh)
	defer m.rootWatcher.Unsubscribe(rootUpdateCh)

	// Setup the timeout chan outside the loop so we don't keep bumping the timeout
	// later if we loop around.
	timeoutTimer := time.NewTimer(req.MaxQueryTime)
	defer timeoutTimer.Stop()

	// Setup initial expiry chan. We may change this if root update occurs in the
	// loop below.
	expiresTimer := time.NewTimer(expiresAt.Sub(now))
	defer func() {
		// Resolve the timer reference at defer time, so we use the latest one each time.
		expiresTimer.Stop()
	}()

	// Current cert is valid so just wait until it expires or we time out.
	for {
		select {
		case <-timeoutTimer.C:
			// We timed out the request with same cert.
			return nil, state, nil

		case <-expiresTimer.C:
			// Cert expired or was force-expired by a root change.
			return m.generateNewLeaf(req, state, false)

		case <-rootUpdateCh:
			// A root cache change occurred, reload roots from cache.
			roots, err := m.rootsReader.Get()
			if err != nil {
				return nil, state, err
			} else if roots == nil {
				return nil, state, errors.New("no CA roots")
			}

			// Handle _possibly_ changed roots. We still need to verify the new active
			// root is not the same as the one our current cert was signed by since we
			// can be notified spuriously if we are the first request since the
			// rootsWatcher didn't know about the CA we were signed by. We also rely
			// on this on every request to do the initial check that the current roots
			// are the same ones the current cert was signed by.
			if activeRootHasKey(roots, state.authorityKeyID) {
				// Current active CA is the same one that signed our current cert so
				// keep waiting for a change.
				continue
			}
			state.activeRootRotationStart = time.Now()

			// CA root changed. We add some jitter here to avoid a thundering herd.
			// See docs on caChangeJitterWindow const.
			delay := m.getJitteredCAChangeDelay()

			// Force the cert to be expired after the jitter - the delay above might
			// be longer than we have left on our timeout. We set forceExpireAfter in
			// the cache state so the next request will notice we still need to renew
			// and do it at the right time. This is cleared once a new cert is
			// returned by generateNewLeaf.
			state.forceExpireAfter = state.activeRootRotationStart.Add(delay)
			// If the delay time is within the current timeout, we want to renew the
			// as soon as it's up. We change the expire time and chan so that when we
			// loop back around, we'll wait at most delay until generating a new cert.
			if state.forceExpireAfter.Before(expiresAt) {
				expiresAt = state.forceExpireAfter
				// Stop the former one and create a new one.
				expiresTimer.Stop()
				expiresTimer = time.NewTimer(delay)
			}
			continue
		}
	}
}

func (m *Manager) getJitteredCAChangeDelay() time.Duration {
	if m.config.TestOverrideCAChangeInitialDelay > 0 {
		return m.config.TestOverrideCAChangeInitialDelay
	}
	// CA root changed. We add some jitter here to avoid a thundering herd.
	// See docs on caChangeJitterWindow const.
	return lib.RandomStagger(caChangeJitterWindow)
}

func activeRootHasKey(roots *structs.IndexedCARoots, currentSigningKeyID string) bool {
	for _, ca := range roots.Roots {
		if ca.Active {
			return ca.SigningKeyID == currentSigningKeyID
		}
	}
	// Shouldn't be possible since at least one root should be active.
	return false
}

// generateNewLeaf does the actual work of creating a new private key,
// generating a CSR and getting it signed by the servers.
//
// NOTE: do not hold the lock while doing the RPC/blocking stuff
func (m *Manager) generateNewLeaf(
	req *ConnectCALeafRequest,
	newState fetchState,
	firstTime bool,
) (*structs.IssuedCert, fetchState, error) {
	// Need to lookup RootCAs response to discover trust domain. This should be a
	// cache hit.
	roots, err := m.rootsReader.Get()
	if err != nil {
		return nil, newState, err
	} else if roots == nil {
		return nil, newState, errors.New("no CA roots")
	}
	if roots.TrustDomain == "" {
		return nil, newState, errors.New("cluster has no CA bootstrapped yet")
	}

	// Build the cert uri
	var id connect.CertURI
	var dnsNames []string
	var ipAddresses []net.IP

	switch {
	case req.Service != "":
		id = &connect.SpiffeIDService{
			Host:       roots.TrustDomain,
			Datacenter: req.Datacenter,
			Partition:  req.TargetPartition(),
			Namespace:  req.TargetNamespace(),
			Service:    req.Service,
		}
		dnsNames = append(dnsNames, req.DNSSAN...)

	case req.Agent != "":
		id = &connect.SpiffeIDAgent{
			Host:       roots.TrustDomain,
			Datacenter: req.Datacenter,
			Partition:  req.TargetPartition(),
			Agent:      req.Agent,
		}
		dnsNames = append([]string{"localhost"}, req.DNSSAN...)
		ipAddresses = append([]net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}, req.IPSAN...)

	case req.Kind == structs.ServiceKindMeshGateway:
		id = &connect.SpiffeIDMeshGateway{
			Host:       roots.TrustDomain,
			Datacenter: req.Datacenter,
			Partition:  req.TargetPartition(),
		}
		dnsNames = append(dnsNames, req.DNSSAN...)

	case req.Kind != "":
		return nil, newState, fmt.Errorf("unsupported kind: %s", req.Kind)

	case req.Server:
		if req.Datacenter == "" {
			return nil, newState, errors.New("datacenter name must be specified")
		}
		id = &connect.SpiffeIDServer{
			Host:       roots.TrustDomain,
			Datacenter: req.Datacenter,
		}
		dnsNames = append(dnsNames, connect.PeeringServerSAN(req.Datacenter, roots.TrustDomain))

	default:
		return nil, newState, errors.New("URI must be either service, agent, server, or kind")
	}

	// Create a new private key

	// TODO: for now we always generate EC keys on clients regardless of the key
	// type being used by the active CA. This is fine and allowed in TLS1.2 and
	// signing EC CSRs with an RSA key is supported by all current CA providers so
	// it's OK. IFF we ever need to support a CA provider that refuses to sign a
	// CSR with a different signature algorithm, or if we have compatibility
	// issues with external PKI systems that require EC certs be signed with ECDSA
	// from the CA (this was required in TLS1.1 but not in 1.2) then we can
	// instead intelligently pick the key type we generate here based on the key
	// type of the active signing CA. We already have that loaded since we need
	// the trust domain.
	pk, pkPEM, err := connect.GeneratePrivateKey()
	if err != nil {
		return nil, newState, err
	}

	// Create a CSR.
	csr, err := connect.CreateCSR(id, pk, dnsNames, ipAddresses)
	if err != nil {
		return nil, newState, err
	}

	// Request signing
	args := structs.CASignRequest{
		WriteRequest: structs.WriteRequest{Token: req.Token},
		Datacenter:   req.Datacenter,
		CSR:          csr,
	}

	reply, err := m.certSigner.SignCert(context.Background(), &args)
	if err != nil {
		if err.Error() == consul.ErrRateLimited.Error() {
			if firstTime {
				// This was a first fetch - we have no good value in cache. In this case
				// we just return the error to the caller rather than rely on surprising
				// semi-blocking until the rate limit is appeased or we timeout
				// behavior. It's likely the caller isn't expecting this to block since
				// it's an initial fetch. This also massively simplifies this edge case.
				return nil, newState, err
			}

			if newState.activeRootRotationStart.IsZero() {
				// We hit a rate limit error by chance - for example a cert expired
				// before the root rotation was observed (not triggered by rotation) but
				// while server is working through high load from a recent rotation.
				// Just pretend there is a rotation and the retry logic here will start
				// jittering and retrying in the same way from now.
				newState.activeRootRotationStart = time.Now()
			}

			// Increment the errors in the state
			newState.consecutiveRateLimitErrs++

			delay := m.getJitteredCAChangeDelay()

			// Find the start of the next window we can retry in. See comment on
			// caChangeJitterWindow for details of why we use this strategy.
			windowStart := newState.activeRootRotationStart.Add(
				time.Duration(newState.consecutiveRateLimitErrs) * delay)

			// Pick a random time in that window
			newState.forceExpireAfter = windowStart.Add(delay)

			// Return a result with the existing cert but the new state - the cache
			// will see this as no change. Note that we always have an existing result
			// here due to the nil value check above.
			return nil, newState, nil
		}
		return nil, newState, err
	}
	reply.PrivateKeyPEM = pkPEM

	// Reset rotation state
	newState.forceExpireAfter = time.Time{}
	newState.consecutiveRateLimitErrs = 0
	newState.activeRootRotationStart = time.Time{}

	cert, err := connect.ParseCert(reply.CertPEM)
	if err != nil {
		return nil, newState, err
	}
	// Set the CA key ID so we can easily tell when a active root has changed.
	newState.authorityKeyID = connect.EncodeSigningKeyID(cert.AuthorityKeyId)

	return reply, newState, nil
}
