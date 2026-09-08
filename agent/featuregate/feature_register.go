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
