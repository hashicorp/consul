// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package peering

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
)

// newToken creates a token holding one policy with the given rules and returns
// a client to the pair's acceptor or dialer that authenticates with it.
func (p *peeringPair) newToken(t *testing.T, acceptor bool, name, rules string) *api.Client {
	t.Helper()

	admin, cluster := p.dial, p.dialer
	if acceptor {
		admin, cluster = p.acc, p.acceptor
	}

	_, _, err := admin.ACL().PolicyCreate(&api.ACLPolicy{Name: name, Rules: rules}, nil)
	require.NoError(t, err)
	token, _, err := admin.ACL().TokenCreate(&api.ACLToken{
		Description: name,
		Policies:    []*api.ACLTokenPolicyLink{{Name: name}},
	}, nil)
	require.NoError(t, err)
	return tokenClient(t, cluster, token.SecretID)
}

// TestPeering_ACLs verifies the permissions that guard peering with default-deny
// ACLs: peering:write creates, establishes and deletes peerings, peering:read
// inspects them, and exporting services is a mesh:write operation.
func TestPeering_ACLs(t *testing.T) {
	t.Parallel()

	p := newPeeringPair(t, true)
	ctx := context.Background()

	type clients struct{ peeringWriter, peeringReader, meshWriter, unrelated *api.Client }
	newClients := func(acceptor bool) clients {
		return clients{
			peeringWriter: p.newToken(t, acceptor, "peering-writer", `peering = "write"`),
			peeringReader: p.newToken(t, acceptor, "peering-reader", `peering = "read"`),
			meshWriter:    p.newToken(t, acceptor, "mesh-writer", `mesh = "write"`),
			unrelated:     p.newToken(t, acceptor, "unrelated", `service "something-else" { policy = "read" }`),
		}
	}
	acceptor, dialer := newClients(true), newClients(false)

	var token string

	t.Run("only peering:write can generate a token", func(t *testing.T) {
		for name, c := range map[string]*api.Client{
			"peering:read": acceptor.peeringReader,
			"mesh:write":   acceptor.meshWriter,
			"no access":    acceptor.unrelated,
		} {
			_, err := generateToken(c, acceptorPeerName)
			require.Error(t, err, name)
			require.Contains(t, err.Error(), "Permission denied", name)
		}

		var err error
		token, err = generateToken(acceptor.peeringWriter, acceptorPeerName)
		require.NoError(t, err)
	})

	t.Run("only peering:write can establish a peering", func(t *testing.T) {
		for name, c := range map[string]*api.Client{
			"peering:read": dialer.peeringReader,
			"no access":    dialer.unrelated,
		} {
			err := establish(c, dialerPeerName, token)
			require.Error(t, err, name)
			require.Contains(t, err.Error(), "Permission denied", name)
		}

		require.NoError(t, establish(dialer.peeringWriter, dialerPeerName, token))
		libassert.PeeringStatus(t, p.acc, acceptorPeerName, api.PeeringStateActive)
		libassert.PeeringStatus(t, p.dial, dialerPeerName, api.PeeringStateActive)
	})

	t.Run("reading peerings requires peering:read or peering:write", func(t *testing.T) {
		for name, c := range map[string]*api.Client{
			"peering:read":  acceptor.peeringReader,
			"peering:write": acceptor.peeringWriter,
		} {
			peering, _, err := c.Peerings().Read(ctx, acceptorPeerName, nil)
			require.NoError(t, err, name)
			require.Equal(t, acceptorPeerName, peering.Name, name)

			list, _, err := c.Peerings().List(ctx, nil)
			require.NoError(t, err, name)
			require.Len(t, list, 1, name)
		}

		// Denied reads are slow because the servers retry the RPC before giving up,
		// so each denial path is checked once.
		_, _, err := acceptor.meshWriter.Peerings().Read(ctx, acceptorPeerName, nil)
		require.Error(t, err, "mesh:write does not grant peering:read")

		_, _, err = acceptor.unrelated.Peerings().List(ctx, nil)
		require.Error(t, err, "a token with no peering permissions cannot list peerings")
	})

	t.Run("exporting services requires mesh:write", func(t *testing.T) {
		_, _, err := acceptor.peeringWriter.ConfigEntries().Set(exportTo(acceptorPeerName, "web"), nil)
		require.Error(t, err, "peering:write is not enough to choose what is exported")
		require.Contains(t, err.Error(), "Permission denied")

		ok, _, err := acceptor.meshWriter.ConfigEntries().Set(exportTo(acceptorPeerName, "web"), nil)
		require.NoError(t, err)
		require.True(t, ok)
	})

	t.Run("only peering:write can delete a peering", func(t *testing.T) {
		for name, c := range map[string]*api.Client{
			"peering:read": acceptor.peeringReader,
			"no access":    acceptor.unrelated,
		} {
			_, err := c.Peerings().Delete(ctx, acceptorPeerName, nil)
			require.Error(t, err, name)
			require.Contains(t, err.Error(), "Permission denied", name)
		}
		require.NotNil(t, peeringState(t, p.acc, acceptorPeerName), "a denied delete must not remove the peering")

		_, err := acceptor.peeringWriter.Peerings().Delete(ctx, acceptorPeerName, nil)
		require.NoError(t, err)
		eventually(t, func(r *retry.R) {
			require.Nil(r, peeringState(r, p.acc, acceptorPeerName))
		})
	})
}
