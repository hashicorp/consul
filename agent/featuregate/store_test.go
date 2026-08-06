// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package featuregate

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStore(t *testing.T) {
	var store Store
	require.False(t, store.Enabled(APIGatewayUpstreamRouting))

	require.True(t, store.Publish(Snapshot{
		StatusIndex: 10,
		PolicyIndex: 8,
		Features: map[string]bool{
			APIGatewayUpstreamRouting.String(): true,
		},
	}))
	require.True(t, store.Enabled(APIGatewayUpstreamRouting))

	// Equal and older generations cannot invalidate committed state.
	require.False(t, store.Publish(Snapshot{StatusIndex: 10}))
	require.False(t, store.Publish(Snapshot{StatusIndex: 9}))
	require.True(t, store.Enabled(APIGatewayUpstreamRouting))

	current := store.Current()
	current.Features[APIGatewayUpstreamRouting.String()] = false
	require.True(t, store.Enabled(APIGatewayUpstreamRouting))

	require.True(t, store.Publish(Snapshot{
		StatusIndex: 11,
		PolicyIndex: 8,
		Features: map[string]bool{
			APIGatewayUpstreamRouting.String(): false,
		},
	}))
	require.False(t, store.Enabled(APIGatewayUpstreamRouting))
}

func TestStore_Watch(t *testing.T) {
	store := &Store{}
	watch := store.Watch()

	require.True(t, store.Publish(Snapshot{StatusIndex: 1}))
	require.Eventually(t, func() bool {
		select {
		case <-watch:
			return true
		default:
			return false
		}
	}, time.Second, time.Millisecond)

	next := store.Watch()
	require.False(t, store.Publish(Snapshot{StatusIndex: 1}))
	select {
	case <-next:
		t.Fatal("stale publication notified watchers")
	default:
	}
}
