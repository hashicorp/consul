// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package acl

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

const anonymousAccessorID = "00000000-0000-0000-0000-000000000002"

// TestACL_TokenExpiry verifies that an expiring token works until it expires
// and is unusable afterwards. Consul does not allow an expiration closer than
// one minute, so this test waits a little over that.
func TestACL_TokenExpiry(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "", "")
	_, _, err := e.admin.ACL().PolicyCreate(&api.ACLPolicy{Name: "expiry-reader", Rules: `service_prefix "" { policy = "read" }`}, nil)
	require.NoError(t, err)

	expiring, _, err := e.admin.ACL().TokenCreate(&api.ACLToken{
		Description:   "expires",
		ExpirationTTL: time.Minute,
		Policies:      []*api.ACLTokenPolicyLink{{Name: "expiry-reader"}},
	}, nil)
	require.NoError(t, err)
	require.NotNil(t, expiring.ExpirationTime)

	lasting, _, err := e.admin.ACL().TokenCreate(&api.ACLToken{
		Description: "does not expire",
		Policies:    []*api.ACLTokenPolicyLink{{Name: "expiry-reader"}},
	}, nil)
	require.NoError(t, err)
	require.Nil(t, lasting.ExpirationTime)

	expiringClient := e.tokenClient(t, expiring.SecretID)
	lastingClient := e.tokenClient(t, lasting.SecretID)

	t.Run("before it expires the token works", func(t *testing.T) {
		self, _, err := expiringClient.ACL().TokenReadSelf(nil)
		require.NoError(t, err)
		require.Equal(t, expiring.AccessorID, self.AccessorID)
		require.WithinDuration(t, *expiring.ExpirationTime, *self.ExpirationTime, time.Second)

		_, _, err = expiringClient.Catalog().Services(nil)
		require.NoError(t, err)
	})

	t.Run("after it expires the token is rejected while other tokens are unaffected", func(t *testing.T) {
		time.Sleep(time.Until(*expiring.ExpirationTime) + 5*time.Second)

		_, _, err := expiringClient.Catalog().Services(nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "ACL not found")

		_, _, err = expiringClient.ACL().TokenReadSelf(nil)
		require.Error(t, err)

		_, _, err = lastingClient.Catalog().Services(nil)
		require.NoError(t, err)
	})
}

// TestACL_AnonymousToken verifies the permissions of requests that carry no
// token, which are governed by the anonymous token.
func TestACL_AnonymousToken(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "", "")
	anonymous := e.tokenClient(t, "")
	require.NoError(t, registerService(e.adminClient, "public-service"))

	serviceNames := func(r retry.TestingTB, c *api.Client) []string {
		services, _, err := c.Catalog().Services(nil)
		require.NoError(r, err)
		var names []string
		for name := range services {
			names = append(names, name)
		}
		return names
	}

	t.Run("the anonymous token exists and grants nothing by default", func(t *testing.T) {
		token, _, err := e.admin.ACL().TokenRead(anonymousAccessorID, nil)
		require.NoError(t, err)
		require.Equal(t, "anonymous", token.SecretID)
		require.Equal(t, "Anonymous Token", token.Description)
		require.Empty(t, token.Policies)

		require.Empty(t, serviceNames(t, anonymous), "nothing is visible without a token")
		_, _, err = anonymous.KV().Get("any/key", nil)
		requireDenied(t, err, "an anonymous KV read")
	})

	t.Run("granting the anonymous token a policy changes what unauthenticated requests can do", func(t *testing.T) {
		_, _, err := e.admin.ACL().PolicyCreate(&api.ACLPolicy{
			Name:  "anonymous-discovery",
			Rules: `service_prefix "" { policy = "read" } node_prefix "" { policy = "read" }`,
		}, nil)
		require.NoError(t, err)
		_, _, err = e.admin.ACL().TokenUpdate(&api.ACLToken{
			AccessorID: anonymousAccessorID,
			Policies:   []*api.ACLTokenPolicyLink{{Name: "anonymous-discovery"}},
		}, nil)
		require.NoError(t, err)

		eventually(t, func(r *retry.R) {
			require.Contains(r, serviceNames(r, anonymous), "public-service")
		})

		_, _, err = anonymous.KV().Get("any/key", nil)
		requireDenied(t, err, "KV is still denied")
		err = registerService(anonymous, "sneaky")
		requireDenied(t, err, "writes are still denied")
	})

	t.Run("the anonymous token cannot be deleted", func(t *testing.T) {
		_, err := e.admin.ACL().TokenDelete(anonymousAccessorID, nil)

		require.Error(t, err)
		require.Contains(t, err.Error(), "not permitted on the anonymous token")
	})

	t.Run("a token that does not exist is rejected rather than treated as anonymous", func(t *testing.T) {
		stranger := e.tokenClient(t, "00000000-0000-0000-0000-00000000dead")

		_, _, err := stranger.Catalog().Services(nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "ACL not found")
	})
}

// TestACL_AgentDefaultToken verifies the default token of an agent, which is
// used for requests that arrive without a token. It is how legacy clients and
// DNS get a non-anonymous identity.
func TestACL_AgentDefaultToken(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "", "")
	anonymous := e.tokenClient(t, "")

	_, err := e.adminClient.KV().Put(&api.KVPair{Key: "app/config", Value: []byte("v")}, nil)
	require.NoError(t, err)
	reader, readerClient := e.newToken(t, "default-token-reader", `key_prefix "app/" { policy = "read" }`)

	t.Run("without a default token unauthenticated requests are denied", func(t *testing.T) {
		_, _, err := anonymous.KV().Get("app/config", nil)
		requireDenied(t, err, "no default token yet")
	})

	t.Run("setting a default token changes unauthenticated requests on that agent", func(t *testing.T) {
		_, err := e.adminClient.Agent().UpdateDefaultACLToken(reader.SecretID, nil)
		require.NoError(t, err)

		eventually(t, func(r *retry.R) {
			pair, _, err := anonymous.KV().Get("app/config", nil)
			require.NoError(r, err)
			require.Equal(r, "v", string(pair.Value))
		})

		_, err = anonymous.KV().Put(&api.KVPair{Key: "app/config", Value: []byte("nope")}, nil)
		requireDenied(t, err, "the default token is read-only")
	})

	t.Run("only a token with agent:write can change the default token", func(t *testing.T) {
		_, err := readerClient.Agent().UpdateDefaultACLToken("", nil)

		requireDenied(t, err, "a reader changing the default token")
	})

	t.Run("clearing the default token restores the original behavior", func(t *testing.T) {
		_, err := e.adminClient.Agent().UpdateDefaultACLToken("", nil)
		require.NoError(t, err)

		eventually(t, func(r *retry.R) {
			_, _, err := anonymous.KV().Get("app/config", nil)
			requireDenied(r, err, "the default token is gone")
		})
	})
}
