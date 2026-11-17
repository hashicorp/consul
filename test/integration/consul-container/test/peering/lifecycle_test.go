// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package peering

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
)

// TestPeering_Lifecycle walks a peering through its life: establishment with a
// single-use token, inspection from both sides, deletion, and
// re-establishment.
func TestPeering_Lifecycle(t *testing.T) {
	t.Parallel()

	p := newPeeringPair(t, false)
	ctx := context.Background()
	var firstToken string

	t.Run("a token from the acceptor establishes an active peering on both sides", func(t *testing.T) {
		firstToken = p.connect(t)
	})

	t.Run("both sides describe the peering", func(t *testing.T) {
		eventually(t, func(r *retry.R) {
			acceptorView := peeringState(r, p.acc, acceptorPeerName)
			require.NotNil(r, acceptorView)
			require.Equal(r, api.PeeringStateActive, acceptorView.State)
			require.Equal(r, "dc2", acceptorView.Remote.Datacenter)
			require.NotNil(r, acceptorView.StreamStatus.LastHeartbeat, "the stream must be heartbeating")

			dialerView := peeringState(r, p.dial, dialerPeerName)
			require.NotNil(r, dialerView)
			require.Equal(r, api.PeeringStateActive, dialerView.State)
			require.Equal(r, "dc1", dialerView.Remote.Datacenter)
			require.NotEmpty(r, dialerView.PeerID)
			require.NotEmpty(r, dialerView.PeerServerAddresses, "the dialer must know how to reach the acceptor")
			require.NotEmpty(r, dialerView.PeerCAPems, "the dialer must trust the acceptor's peering CA")

			require.Equal(r, []string{acceptorPeerName}, peeringNames(r, p.acc))
			require.Equal(r, []string{dialerPeerName}, peeringNames(r, p.dial))
		})
	})

	t.Run("generating another token for an active peering does not disturb it", func(t *testing.T) {
		_, err := generateToken(p.acc, acceptorPeerName)
		require.NoError(t, err)

		require.Equal(t, api.PeeringStateActive, peeringState(t, p.acc, acceptorPeerName).State)
		require.Equal(t, api.PeeringStateActive, peeringState(t, p.dial, dialerPeerName).State)
	})

	t.Run("a token can be used only once", func(t *testing.T) {
		err := establish(p.dial, "second-attempt", firstToken)
		require.Error(t, err)
		require.Contains(t, err.Error(), "a new peering token must be generated")

		// The failed attempt leaves a pending peering behind; clean it up.
		_, err = p.dial.Peerings().Delete(ctx, "second-attempt", nil)
		require.NoError(t, err)
		eventually(t, func(r *retry.R) {
			require.Nil(r, peeringState(r, p.dial, "second-attempt"))
		})
	})

	t.Run("a malformed token is rejected", func(t *testing.T) {
		err := establish(p.dial, "from-garbage", "not-a-token")
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to decode token")
		require.Nil(t, peeringState(t, p.dial, "from-garbage"), "no peering is created for an unusable token")
	})

	t.Run("deleting the dialer's peering terminates the acceptor's", func(t *testing.T) {
		_, err := p.dial.Peerings().Delete(ctx, dialerPeerName, nil)
		require.NoError(t, err)

		eventually(t, func(r *retry.R) {
			require.Nil(r, peeringState(r, p.dial, dialerPeerName), "the dialer's peering is removed")

			acceptorView := peeringState(r, p.acc, acceptorPeerName)
			require.NotNil(r, acceptorView, "the acceptor keeps its record")
			require.Equal(r, api.PeeringStateTerminated, acceptorView.State)
		})
	})

	t.Run("a terminated peering must be cleaned up on both sides before it is re-established", func(t *testing.T) {
		// A new token for the name of the terminated peering cannot revive it.
		token, err := generateToken(p.acc, acceptorPeerName)
		require.NoError(t, err)
		err = establish(p.dial, dialerPeerName, token)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid peering establishment secret")

		for _, step := range []struct {
			c    *api.Client
			name string
		}{{p.dial, dialerPeerName}, {p.acc, acceptorPeerName}} {
			_, err := step.c.Peerings().Delete(ctx, step.name, nil)
			require.NoError(t, err)
		}
		eventually(t, func(r *retry.R) {
			require.Empty(r, peeringNames(r, p.acc))
			require.Empty(r, peeringNames(r, p.dial))
		})

		p.connect(t)
	})
}

func eventually(t *testing.T, check func(r *retry.R)) {
	t.Helper()
	retry.RunWith(libcluster.LongFailer(), t, check)
}
