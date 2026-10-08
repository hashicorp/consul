// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package middleware

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/consul/rate"
)

// An unmapped RPC defaults to a rate-limited read, so every client agent's
// gate refresh would count against the read limit and gate writes would use it.
func TestRPCRateLimitSpecs_FeatureGateRPCsAreExempt(t *testing.T) {
	for _, name := range []string{"Operator.FeatureGateGet", "Operator.FeatureGateSet"} {
		spec, ok := rpcRateLimitSpecs[name]
		require.True(t, ok, "%s must be mapped", name)
		require.Equal(t, rate.OperationTypeExempt, spec.Type, name)
	}
}
