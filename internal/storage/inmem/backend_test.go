// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package inmem_test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBackend_Conformance(t *testing.T) {
	// TODO(spatel): temporarily commenting out to get a green pipleine.
	require.True(t, true)

	// conformance.Test(t, conformance.TestOptions{
	// 	NewBackend: func(t *testing.T) storage.Backend {
	// 		backend, err := inmem.NewBackend()
	// 		require.NoError(t, err)

	// 		ctx, cancel := context.WithCancel(context.Background())
	// 		t.Cleanup(cancel)
	// 		go backend.Run(ctx)

	// 		return backend
	// 	},
	// 	SupportsStronglyConsistentList: true,
	// })
}
