// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package structs

import (
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
)

// TestAPIGatewayExtAuthzEnabled_CE verifies that the gateway-wide ext_authz
// posture defaults to enabled in CE, where the gateway-wide toggle is not
// configurable.
func TestAPIGatewayExtAuthzEnabled_CE(t *testing.T) {
	var e *APIGatewayConfigEntry
	require.True(t, (&APIGatewayConfigEntry{}).ExtAuthzEnabled())
	require.True(t, e.ExtAuthzEnabled(), "nil receiver should still report enabled in CE")
}

// TestExtAuthzStructsEmptyInCE verifies that the ext_authz config types are
// empty structs in CE. Their fields are enterprise-only; keeping them empty in
// CE guarantees no enterprise-only configuration surface leaks into CE.
func TestExtAuthzStructsEmptyInCE(t *testing.T) {
	require.Zero(t, unsafe.Sizeof(HTTPRouteExtAuthzFilter{}), "HTTPRouteExtAuthzFilter must be empty in CE")
	require.Zero(t, unsafe.Sizeof(APIGatewayExtAuthz{}), "APIGatewayExtAuthz must be empty in CE")
}
