// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package proxycfgglue

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/proxycfg"
)

func indexGenerator() func() uint64 {
	var idx uint64
	return func() uint64 {
		idx++
		return idx
	}
}

func getEventResult[ResultType any](t *testing.T, eventCh <-chan proxycfg.UpdateEvent) ResultType {
	t.Helper()

	select {
	case event := <-eventCh:
		require.NoError(t, event.Err, "event should not have an error")
		result, ok := event.Result.(ResultType)
		require.Truef(t, ok, "unexpected result type: %T", event.Result)
		return result
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event")
	}

	panic("this should never be reached")
}

func expectNoEvent(t *testing.T, eventCh <-chan proxycfg.UpdateEvent) {
	select {
	case <-eventCh:
		t.Fatal("expected no event")
	case <-time.After(100 * time.Millisecond):
	}
}

func getEventError(t *testing.T, eventCh <-chan proxycfg.UpdateEvent) error {
	t.Helper()

	select {
	case event := <-eventCh:
		require.Error(t, event.Err)
		return event.Err
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event")
	}

	panic("this should never be reached")
}
