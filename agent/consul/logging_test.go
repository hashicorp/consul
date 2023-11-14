// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/sdk/testutil"
)

func TestLoggerStore_Named(t *testing.T) {
	t.Parallel()

	logger := testutil.Logger(t)
	store := newLoggerStore(logger)
	require.NotNil(t, store)

	l1 := store.Named("test1")
	l2 := store.Named("test2")
	require.Truef(t, l1 != l2,
		"expected %p and %p to have a different memory address",
		l1,
		l2,
	)
}

func TestLoggerStore_NamedCache(t *testing.T) {
	t.Parallel()

	logger := testutil.Logger(t)
	store := newLoggerStore(logger)
	require.NotNil(t, store)

	l1 := store.Named("test")
	l2 := store.Named("test")
	require.Truef(t, l1 == l2,
		"expected %p and %p to have the same memory address",
		l1,
		l2,
	)
}
