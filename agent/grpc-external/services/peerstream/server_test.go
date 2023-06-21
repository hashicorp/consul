// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package peerstream

import (
	"context"
	"testing"

	"github.com/hashicorp/consul/proto/private/pbpeering"
	"github.com/hashicorp/consul/proto/private/pbpeerstream"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/stretchr/testify/require"
)

func TestServer_ExchangeSecret(t *testing.T) {
	srv, store := newTestServer(t, nil)
	_ = writePeeringToBeDialed(t, store, 1, "my-peer")

	testutil.RunStep(t, "unknown establishment secret is rejected", func(t *testing.T) {
		resp, err := srv.ExchangeSecret(context.Background(), &pbpeerstream.ExchangeSecretRequest{
			PeerID:              testPeerID,
			EstablishmentSecret: "bad",
		})
		testutil.RequireErrorContains(t, err, `rpc error: code = PermissionDenied desc = invalid peering establishment secret`)
		require.Nil(t, resp)
	})

	var secret string
	testutil.RunStep(t, "known establishment secret is accepted", func(t *testing.T) {
		// First write the establishment secret so that it can be exchanged
		require.NoError(t, store.PeeringSecretsWrite(1, &pbpeering.SecretsWriteRequest{
			PeerID: testPeerID,
			Request: &pbpeering.SecretsWriteRequest_GenerateToken{
				GenerateToken: &pbpeering.SecretsWriteRequest_GenerateTokenRequest{
					EstablishmentSecret: testEstablishmentSecretID,
				},
			},
		}))

		// Exchange the now-valid establishment secret for a stream secret
		resp, err := srv.ExchangeSecret(context.Background(), &pbpeerstream.ExchangeSecretRequest{
			PeerID:              testPeerID,
			EstablishmentSecret: testEstablishmentSecretID,
		})
		require.NoError(t, err)
		require.NotEmpty(t, resp.StreamSecret)

		secret = resp.StreamSecret
	})

	testutil.RunStep(t, "pending secret is persisted to server", func(t *testing.T) {
		s, err := store.PeeringSecretsRead(nil, testPeerID)
		require.NoError(t, err)

		require.Equal(t, secret, s.GetStream().GetPendingSecretID())
	})
}
