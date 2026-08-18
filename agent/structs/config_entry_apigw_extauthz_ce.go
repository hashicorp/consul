// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package structs

// HTTPRouteExtAuthzFilter controls ext_authz filter behaviour at the individual
// http-route rule level. It is an enterprise-only feature, so it is an empty
// struct in CE; the enterprise build defines its fields.
type HTTPRouteExtAuthzFilter struct{}

// APIGatewayExtAuthz is the gateway-wide external authorization toggle on an
// APIGatewayConfigEntry. It is an enterprise-only feature, so it is an empty
// struct in CE; the enterprise build defines its fields.
type APIGatewayExtAuthz struct{}

// ExtAuthzEnabled reports the gateway-wide default ext_authz posture. In CE the
// gateway-wide toggle is not configurable, so ext_authz is always reported as
// enabled by default; the enterprise build reads the configured value.
func (e *APIGatewayConfigEntry) ExtAuthzEnabled() bool {
	return true
}
