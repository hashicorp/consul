// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-metrics"
)

// gatedSnapshot is the slice of *proxycfg.ConfigSnapshot that bootstrapGate
// needs: the discovery-chain targets that would be rendered into CDS but whose
// EDS assignments are not assembled yet. It is empty for every non-api-gateway
// proxy kind, so those streams are never gated.
type gatedSnapshot interface {
	APIGatewayDiscoveryChainsMissingEndpoints() []string
}

// bootstrapGate holds an xDS stream's FIRST push until the api-gateway snapshot
// behind it is internally coherent — every synthesized discovery chain that will
// be rendered into CDS also has its EDS endpoints assembled — so a cold-starting
// Envoy never initializes over clusters whose endpoint assignments Consul has not
// produced yet (ITCO-15826).
//
// Three properties keep the gate from becoming a liveness hazard, which matters
// because Consul programs lds_config/cds_config with initial_fetch_timeout: 0s,
// meaning a withheld push leaves Envoy waiting indefinitely rather than falling
// back:
//
//  1. It is first-push only. Once a push is allowed the gate stays open for the
//     life of the stream, so steady-state updates are never withheld and a
//     later, transiently-incomplete snapshot cannot re-close it.
//  2. It is skipped for resumed streams. An Envoy that advertises
//     initial_resource_versions already finished initialization and started
//     workers, so there is nothing left to protect and holding its push would
//     only delay reconvergence — including after a routine server rebalance.
//  3. It is bounded. If the predicate is somehow never satisfied the gate opens
//     anyway, degrading to a late push (and 503s on the affected routes) rather
//     than a stream that delivers nothing at all.
//
// The zero value is not usable; construct with newBootstrapGate.
type bootstrapGate struct {
	timeout time.Duration

	open    bool
	skip    bool
	expired bool

	timer   *time.Timer
	started time.Time
}

// newBootstrapGate returns a gate that will hold a first push for at most
// timeout. A zero timeout means DefaultBootstrapGateTimeout; a negative timeout
// disables the gate, so it never withholds anything.
func newBootstrapGate(timeout time.Duration) *bootstrapGate {
	switch {
	case timeout == 0:
		timeout = DefaultBootstrapGateTimeout
	case timeout < 0:
		return &bootstrapGate{open: true}
	}
	return &bootstrapGate{timeout: timeout}
}

// expiryCh returns the channel that fires when the gate has held a push for
// longer than its timeout. It is nil — and therefore blocks forever, which is
// what a select wants — until the gate actually starts holding something.
func (g *bootstrapGate) expiryCh() <-chan time.Time {
	if g.timer == nil {
		return nil
	}
	return g.timer.C
}

// markExpired records that expiryCh fired. The next allow call will open the
// gate regardless of snapshot completeness.
func (g *bootstrapGate) markExpired() {
	g.expired = true
}

// markResumedStream records that Envoy advertised resources it already holds,
// which means it is not cold-starting and the gate must not delay its config.
func (g *bootstrapGate) markResumedStream() {
	g.skip = true
}

// stop releases the gate's timer. Safe to call on an unused or already-open gate.
func (g *bootstrapGate) stop() {
	if g.timer != nil {
		g.timer.Stop()
		g.timer = nil
	}
}

// allow reports whether the stream may push this snapshot. It returns false only
// for a cold-starting api-gateway stream whose first push would render CDS
// clusters that have no EDS assignment yet, and only until the timeout elapses.
func (g *bootstrapGate) allow(logger hclog.Logger, snapshot gatedSnapshot) bool {
	if g.open {
		return true
	}

	missing := snapshot.APIGatewayDiscoveryChainsMissingEndpoints()

	switch {
	case len(missing) == 0 || g.skip:
		if g.timer != nil {
			metrics.MeasureSince([]string{"xds", "server", "bootstrapGateHeld"}, g.started)
		}

	case g.expired:
		logger.Warn("api-gateway: releasing initial xDS push before discovery-chain endpoints were assembled; "+
			"the gateway may answer 503 for these targets until their endpoints arrive",
			"held", time.Since(g.started), "missing_targets", missing)
		metrics.IncrCounter([]string{"xds", "server", "bootstrapGateTimeout"}, 1)

	default:
		if g.timer == nil {
			g.started = time.Now()
			g.timer = time.NewTimer(g.timeout)
		}
		logger.Debug("api-gateway: holding initial xDS push until discovery-chain endpoints are assembled",
			"missing_targets", missing)
		return false
	}

	g.stop()
	g.open = true
	return true
}
