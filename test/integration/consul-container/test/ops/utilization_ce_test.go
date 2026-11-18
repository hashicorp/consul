// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package ops

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestOperatorUtilization_NotInCE verifies that utilization reporting, which is
// an Enterprise feature, says so on Consul CE instead of failing obscurely.
func TestOperatorUtilization_NotInCE(t *testing.T) {
	e := newEnv(t, "", "")

	out, code := e.server.CLI(t, "operator", "utilization")
	require.NotEqual(t, 0, code)
	require.Contains(t, out, "requires Consul Enterprise")
}
