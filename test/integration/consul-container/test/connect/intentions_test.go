// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package connect

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
)

func setEntry(t *testing.T, c *api.Client, entry api.ConfigEntry) {
	t.Helper()
	ok, _, err := c.ConfigEntries().Set(entry, nil)
	require.NoError(t, err)
	require.True(t, ok)
}

func checkIntention(t *testing.T, c *api.Client, source, destination string) bool {
	t.Helper()
	allowed, _, err := c.Connect().IntentionCheck(&api.IntentionCheck{Source: source, Destination: destination}, nil)
	require.NoError(t, err)
	return allowed
}

// TestIntentions_CRUDAndPrecedence covers the intention lifecycle through the
// service-intentions config entry and the match/check APIs, including the
// precedence rules (exact beats wildcard) that decide the effective action.
func TestIntentions_CRUDAndPrecedence(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	c := e.server

	t.Run("no intentions allows traffic under the default policy", func(t *testing.T) {
		require.True(t, checkIntention(t, c, "web", "db"))
	})

	t.Run("create read list delete", func(t *testing.T) {
		setEntry(t, c, &api.ServiceIntentionsConfigEntry{
			Kind: api.ServiceIntentions,
			Name: "db",
			Sources: []*api.SourceIntention{
				{Name: "web", Action: api.IntentionActionAllow, Description: "web reads db"},
				{Name: "batch", Action: api.IntentionActionDeny},
			},
		})

		web, _, err := c.Connect().IntentionGetExact("web", "db", nil)
		require.NoError(t, err)
		require.NotNil(t, web)
		require.Equal(t, api.IntentionActionAllow, web.Action)
		require.Equal(t, "web reads db", web.Description)

		all, _, err := c.Connect().Intentions(nil)
		require.NoError(t, err)
		var pairs []string
		for _, ixn := range all {
			pairs = append(pairs, ixn.SourceName+"->"+ixn.DestinationName)
		}
		require.Contains(t, pairs, "web->db")
		require.Contains(t, pairs, "batch->db")

		_, err = c.Connect().IntentionDeleteExact("batch", "db", nil)
		require.NoError(t, err)
		batch, _, err := c.Connect().IntentionGetExact("batch", "db", nil)
		require.NoError(t, err)
		require.Nil(t, batch)

		// Deleting one source keeps the others.
		web, _, err = c.Connect().IntentionGetExact("web", "db", nil)
		require.NoError(t, err)
		require.NotNil(t, web)

		_, err = c.ConfigEntries().Delete(api.ServiceIntentions, "db", nil)
		require.NoError(t, err)
		web, _, err = c.Connect().IntentionGetExact("web", "db", nil)
		require.NoError(t, err)
		require.Nil(t, web)
	})

	t.Run("exact intentions beat wildcards", func(t *testing.T) {
		setEntry(t, c, &api.ServiceIntentionsConfigEntry{
			Kind:    api.ServiceIntentions,
			Name:    "*",
			Sources: []*api.SourceIntention{{Name: "*", Action: api.IntentionActionDeny}},
		})
		setEntry(t, c, &api.ServiceIntentionsConfigEntry{
			Kind:    api.ServiceIntentions,
			Name:    "payments",
			Sources: []*api.SourceIntention{{Name: "checkout", Action: api.IntentionActionAllow}},
		})
		setEntry(t, c, &api.ServiceIntentionsConfigEntry{
			Kind:    api.ServiceIntentions,
			Name:    "ledger",
			Sources: []*api.SourceIntention{{Name: "*", Action: api.IntentionActionAllow}},
		})

		require.False(t, checkIntention(t, c, "anything", "unrelated"), "*->* deny applies when nothing is more specific")
		require.True(t, checkIntention(t, c, "checkout", "payments"), "exact allow beats *->* deny")
		require.False(t, checkIntention(t, c, "other", "payments"), "only checkout is allowed to payments")
		require.True(t, checkIntention(t, c, "anyone", "ledger"), "*->ledger allow beats *->* deny")

		matched, _, err := c.Connect().IntentionMatch(&api.IntentionMatch{
			By:    api.IntentionMatchDestination,
			Names: []string{"payments"},
		}, nil)
		require.NoError(t, err)
		require.Len(t, matched["payments"], 2)
		require.Equal(t, "checkout", matched["payments"][0].SourceName, "most specific intention is listed first")
		require.Equal(t, "*", matched["payments"][1].SourceName)
		require.Greater(t, matched["payments"][0].Precedence, matched["payments"][1].Precedence)

		matched, _, err = c.Connect().IntentionMatch(&api.IntentionMatch{
			By:    api.IntentionMatchSource,
			Names: []string{"checkout"},
		}, nil)
		require.NoError(t, err)
		require.NotEmpty(t, matched["checkout"])
		require.Equal(t, "payments", matched["checkout"][0].DestinationName)

		_, err = c.ConfigEntries().Delete(api.ServiceIntentions, "*", nil)
		require.NoError(t, err)
		retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
			require.True(r, checkIntention(t, c, "anything", "unrelated"))
		})
	})

	t.Run("updating a source changes the effective decision", func(t *testing.T) {
		setEntry(t, c, &api.ServiceIntentionsConfigEntry{
			Kind:    api.ServiceIntentions,
			Name:    "orders",
			Sources: []*api.SourceIntention{{Name: "cart", Action: api.IntentionActionDeny}},
		})
		require.False(t, checkIntention(t, c, "cart", "orders"))

		setEntry(t, c, &api.ServiceIntentionsConfigEntry{
			Kind:    api.ServiceIntentions,
			Name:    "orders",
			Sources: []*api.SourceIntention{{Name: "cart", Action: api.IntentionActionAllow}},
		})
		require.True(t, checkIntention(t, c, "cart", "orders"))
	})
}

// TestIntentions_L7Permissions verifies that L7 (HTTP) permissions are only
// accepted for HTTP services and that invalid shapes are rejected.
func TestIntentions_L7Permissions(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	c := e.server

	l7 := func(destination string) *api.ServiceIntentionsConfigEntry {
		return &api.ServiceIntentionsConfigEntry{
			Kind: api.ServiceIntentions,
			Name: destination,
			Sources: []*api.SourceIntention{{
				Name: "web",
				Permissions: []*api.IntentionPermission{
					{Action: api.IntentionActionAllow, HTTP: &api.IntentionHTTPPermission{PathPrefix: "/public", Methods: []string{"GET"}}},
					{Action: api.IntentionActionDeny, HTTP: &api.IntentionHTTPPermission{PathPrefix: "/"}},
				},
			}},
		}
	}

	t.Run("rejected for a tcp service", func(t *testing.T) {
		_, _, err := c.ConfigEntries().Set(l7("tcp-svc"), nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "protocol")
	})

	t.Run("accepted for an http service and preserved on read", func(t *testing.T) {
		setEntry(t, c, &api.ServiceConfigEntry{Kind: api.ServiceDefaults, Name: "http-svc", Protocol: "http"})
		setEntry(t, c, l7("http-svc"))

		ixn, _, err := c.Connect().IntentionGetExact("web", "http-svc", nil)
		require.NoError(t, err)
		require.NotNil(t, ixn)
		require.Empty(t, ixn.Action, "L7 intentions carry permissions instead of a top-level action")
		require.Len(t, ixn.Permissions, 2)
		require.Equal(t, "/public", ixn.Permissions[0].HTTP.PathPrefix)
		require.Equal(t, []string{"GET"}, ixn.Permissions[0].HTTP.Methods)
		require.Equal(t, api.IntentionActionDeny, ixn.Permissions[1].Action)
	})

	t.Run("action and permissions are mutually exclusive", func(t *testing.T) {
		setEntry(t, c, &api.ServiceConfigEntry{Kind: api.ServiceDefaults, Name: "http-svc-2", Protocol: "http"})
		entry := l7("http-svc-2")
		entry.Sources[0].Action = api.IntentionActionAllow

		_, _, err := c.ConfigEntries().Set(entry, nil)
		require.Error(t, err)
	})

	t.Run("a permission without any match criteria is rejected", func(t *testing.T) {
		setEntry(t, c, &api.ServiceConfigEntry{Kind: api.ServiceDefaults, Name: "http-svc-3", Protocol: "http"})
		entry := l7("http-svc-3")
		entry.Sources[0].Permissions = []*api.IntentionPermission{{Action: api.IntentionActionAllow, HTTP: &api.IntentionHTTPPermission{}}}

		_, _, err := c.ConfigEntries().Set(entry, nil)
		require.Error(t, err)
	})
}
