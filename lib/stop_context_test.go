// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package lib

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStopChannelContext(t *testing.T) {
	ch := make(chan struct{})

	ctx := StopChannelContext{StopCh: ch}

	select {
	case <-ctx.Done():
		require.FailNow(t, "StopChannelContext should not be done yet")
	default:
		// do nothing things are good
	}

	close(ch)

	select {
	case <-ctx.Done():
		// things are good, as we are done
	default:
		require.FailNow(t, "StopChannelContext should be done")
	}

	// issue it twice to ensure that we indefinitely return the
	// same value - this is what the context interface says is
	// the correct behavior.
	require.Equal(t, context.Canceled, ctx.Err())
	require.Equal(t, context.Canceled, ctx.Err())
}
