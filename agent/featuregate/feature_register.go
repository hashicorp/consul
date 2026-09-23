// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package featuregate

import "github.com/hashicorp/go-version"

// APIGatewayUpstreamRouting gates the API Gateway discovery-chain synthesis
// behavior introduced by hashicorp/consul#23294. Phase 1 supports the
// server-side proxycfg path used by agentless API Gateways only.
//
// TODO(release-owner): Replace MinVersion "2.1.0-dev" with the first
// production release that contains both:
//
//	(a) the feature-gate Raft framework (FeatureGateRequestType + snapshot decode), and
//	(b) the guarded #23294 behavior in gateway_httproute.go.
//
// MinVersion must equal an already-published release tag, not a dev build.
// Using a dev version means the minimum-version floor will never be satisfied
// in production clusters, keeping the feature permanently disabled.
var APIGatewayUpstreamRouting = registerFeature(Definition{
	Name:        "api-gateway-upstream-routing",
	MinVersion:  version.Must(version.NewVersion("2.1.0")),
	Description: "Compose API Gateway HTTPRoutes with upstream routing policy",
	Owner:       "proxycfg",
})

// LocalizedDNS gates the inline virtual DNS listener and the egress recursor
// DNS listener added to connect-proxy sidecars (see agent/xds/listeners_dns.go).
// When disabled, neither listener is added to the xDS LDS resources for the
// proxy, regardless of whether there are virtual IPs or recursors configured.
//
// TODO(release-owner): Replace MinVersion "2.1.0-dev" with the first
// production release that contains both the feature-gate Raft framework and
// the guarded local DNS listener behavior in agent/xds/listeners.go.
//
// MinVersion must equal an already-published release tag, not a dev build.
// Using a dev version means the minimum-version floor will never be satisfied
// in production clusters, keeping the feature permanently disabled.
var LocalizedDNS = registerFeature(Definition{
	Name:        "localized-dns",
	MinVersion:  version.Must(version.NewVersion("2.1.0")),
	Description: "Add inline virtual DNS and egress recursor DNS listeners to connect-proxy sidecars",
	Owner:       "proxycfg",
})
