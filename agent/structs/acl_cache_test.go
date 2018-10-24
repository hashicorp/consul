package structs

import (
	"testing"

	"github.com/hashicorp/consul/acl"
	"github.com/stretchr/testify/require"
)

func TestStructs_ACLCaches(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("Valid Sizes", func(t *testing.T) {
			t.Parallel()
			// 1 isn't valid due to a bug in golang-lru library
			config := ACLCachesConfig{2, 2, 2, 2}

			cache, err := NewACLCaches(&config)
			require.NoError(t, err)
			require.NotNil(t, cache)
			require.NotNil(t, cache.identities)
			require.NotNil(t, cache.policies)
			require.NotNil(t, cache.parsedPolicies)
			require.NotNil(t, cache.authorizers)
		})

		t.Run("Zero Sizes", func(t *testing.T) {
			t.Parallel()
			// 1 isn't valid due to a bug in golang-lru library
			config := ACLCachesConfig{0, 0, 0, 0}

			cache, err := NewACLCaches(&config)
			require.NoError(t, err)
			require.NotNil(t, cache)
			require.Nil(t, cache.identities)
			require.Nil(t, cache.policies)
			require.Nil(t, cache.parsedPolicies)
			require.Nil(t, cache.authorizers)
		})
	})

	t.Run("Identities", func(t *testing.T) {
		t.Parallel()
		// 1 isn't valid due to a bug in golang-lru library
		config := ACLCachesConfig{Identities: 4}

		cache, err := NewACLCaches(&config)
		require.NoError(t, err)
		require.NotNil(t, cache)

		cache.PutIdentity("foo", &ACLToken{})
		entry := cache.GetIdentity("foo")
		require.NotNil(t, entry)
		require.NotNil(t, entry.Identity)
	})

	t.Run("Policies", func(t *testing.T) {
		t.Parallel()
		// 1 isn't valid due to a bug in golang-lru library
		config := ACLCachesConfig{Policies: 4}

		cache, err := NewACLCaches(&config)
		require.NoError(t, err)
		require.NotNil(t, cache)

		cache.PutPolicy("foo", &ACLPolicy{})
		entry := cache.GetPolicy("foo")
		require.NotNil(t, entry)
		require.NotNil(t, entry.Policy)
	})

	t.Run("ParsedPolicies", func(t *testing.T) {
		t.Parallel()
		// 1 isn't valid due to a bug in golang-lru library
		config := ACLCachesConfig{ParsedPolicies: 4}

		cache, err := NewACLCaches(&config)
		require.NoError(t, err)
		require.NotNil(t, cache)

		cache.PutParsedPolicy("foo", &acl.Policy{})
		entry := cache.GetParsedPolicy("foo")
		require.NotNil(t, entry)
		require.NotNil(t, entry.Policy)
	})

	t.Run("Authorizers", func(t *testing.T) {
		t.Parallel()
		// 1 isn't valid due to a bug in golang-lru library
		config := ACLCachesConfig{Authorizers: 4}

		cache, err := NewACLCaches(&config)
		require.NoError(t, err)
		require.NotNil(t, cache)

		cache.PutAuthorizer("foo", acl.DenyAll())
		entry := cache.GetAuthorizer("foo")
		require.NotNil(t, entry)
		require.NotNil(t, entry.Authorizer)
		require.True(t, entry.Authorizer == acl.DenyAll())
	})
}
