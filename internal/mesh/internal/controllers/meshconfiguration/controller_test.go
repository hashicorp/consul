// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package meshconfiguration

import (
	"context"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/stretchr/testify/require"
	"testing"
)

// TestReconciliation ensures that the Reconcile method for the Controller
// correctly updates the runtime state based on the given request.
func TestReconcile(t *testing.T) {
	// This test should be continually updated as we build out the MeshConfiguration controller.
	// At time of writing, it simply returns a not-implemented error.

	ctx := context.Background()
	rt := controller.Runtime{
		Client: nil,
		Logger: nil,
	}
	req := controller.Request{}

	rec := reconciler{}

	err := rec.Reconcile(ctx, rt, req)
	require.Error(t, err)
}
