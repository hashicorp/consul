// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package leafcert

import (
	"time"

	"github.com/hashicorp/consul/agent/structs"
)

// calculateSoftExpiry encapsulates our logic for when to renew a cert based on
// it's age. It returns a pair of times min, max which makes it easier to test
// the logic without non-deterministic jitter to account for. The caller should
// choose a time randomly in between these.
//
// We want to balance a few factors here:
//   - renew too early and it increases the aggregate CSR rate in the cluster
//   - renew too late and it risks disruption to the service if a transient
//     error prevents the renewal
//   - we want a broad amount of jitter so if there is an outage, we don't end
//     up with all services in sync and causing a thundering herd every
//     renewal period. Broader is better for smoothing requests but pushes
//     both earlier and later tradeoffs above.
//
// Somewhat arbitrarily the current strategy looks like this:
//
//	         0                              60%             90%
//	  Issued [------------------------------|===============|!!!!!] Expires
//	72h TTL: 0                             ~43h            ~65h
//	 1h TTL: 0                              36m             54m
//
// Where |===| is the soft renewal period where we jitter for the first attempt
// and |!!!| is the danger zone where we just try immediately.
//
// In the happy path (no outages) the average renewal occurs half way through
// the soft renewal region or at 75% of the cert lifetime which is ~54 hours for
// a 72 hour cert, or 45 mins for a 1 hour cert.
//
// If we are already in the softRenewal period, we randomly pick a time between
// now and the start of the danger zone.
//
// We pass in now to make testing easier.
func calculateSoftExpiry(now time.Time, cert *structs.IssuedCert) (min time.Time, max time.Time) {
	certLifetime := cert.ValidBefore.Sub(cert.ValidAfter)
	if certLifetime < 10*time.Minute {
		// Shouldn't happen as we limit to 1 hour shortest elsewhere but just be
		// defensive against strange times or bugs.
		return now, now
	}

	// Find the 60% mark in diagram above
	softRenewTime := cert.ValidAfter.Add(time.Duration(float64(certLifetime) * 0.6))
	hardRenewTime := cert.ValidAfter.Add(time.Duration(float64(certLifetime) * 0.9))

	if now.After(hardRenewTime) {
		// In the hard renew period, or already expired. Renew now!
		return now, now
	}

	if now.After(softRenewTime) {
		// Already in the soft renew period, make now the lower bound for jitter
		softRenewTime = now
	}
	return softRenewTime, hardRenewTime
}
