// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package proxycfg

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestWatchExtAuthzMeshTargets_NoopInCE verifies that watching builtin/ext-authz
// mesh (Service) targets declared on an API Gateway is a no-op in CE. This is an
// enterprise-only feature; the enterprise build provides the real
// implementation.
func TestWatchExtAuthzMeshTargets_NoopInCE(t *testing.T) {
	h := &handlerAPIGateway{}
	require.NoError(t, h.watchExtAuthzMeshTargets(context.Background(), nil))
}
