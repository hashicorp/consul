// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package inmem_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/internal/storage/conformance"
	"github.com/hashicorp/consul/internal/storage/inmem"
)

func TestBackend_Conformance(t *testing.T) {
	conformance.Test(t, conformance.TestOptions{
		NewBackend: func(t *testing.T) storage.Backend {
			backend, err := inmem.NewBackend()
			require.NoError(t, err)

			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			go backend.Run(ctx)

			return backend
		},
		SupportsStronglyConsistentList: true,
	})
}
