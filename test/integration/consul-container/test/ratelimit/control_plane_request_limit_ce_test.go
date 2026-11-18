// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package ratelimit

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestControlPlaneRequestLimit_NotInCE verifies that the control-plane request
// limit is an Enterprise feature that Consul CE refuses to configure. Server
// request limits are configured in the agent configuration instead, see
// TestServerRequestRateLimit.
func TestControlPlaneRequestLimit_NotInCE(t *testing.T) {
	_, client := newServer(t, "")

	_, err := client.Raw().Write("/v1/config", map[string]interface{}{
		"Kind": "control-plane-request-limit",
		"Name": "global",
		"Mode": "enforcing",
	}, nil, nil)

	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid config entry kind: control-plane-request-limit")
}
