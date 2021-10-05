package structs

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
)

func TestStructs_ACLCaches(t *testing.T) {

	t.Run("New", func(t *testing.T) {

		t.Run("Valid Sizes", func(t *testing.T) {
			// 1 isn't valid due to a bug in golang-lru library
			config := ACLCachesConfig{2, 2, 2, 2, 2}

			cache, err := NewACLCaches(&config)
			require.NoError(t, err)
			require.NotNil(t, cache)
			require.NotNil(t, cache.identities)
			require.NotNil(t, cache.policies)
			require.NotNil(t, cache.parsedPolicies)
			require.NotNil(t, cache.authorizers)
		})

		t.Run("Zero Sizes", func(t *testing.T) {
			// 1 isn't valid due to a bug in golang-lru library
			config := ACLCachesConfig{0, 0, 0, 0, 0}

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

	t.Run("Roles", func(t *testing.T) {
		// 1 isn't valid due to a bug in golang-lru library
		config := ACLCachesConfig{Roles: 4}

		cache, err := NewACLCaches(&config)
		require.NoError(t, err)
		require.NotNil(t, cache)

		cache.PutRole("foo", &ACLRole{})

		entry := cache.GetRole("foo")
		require.NotNil(t, entry)
		require.NotNil(t, entry.Role)
	})
}

func TestNewPolicyAuthorizerWithCache(t *testing.T) {
	config := ACLCachesConfig{
		Identities:     0,
		Policies:       0,
		ParsedPolicies: 4,
		Authorizers:    2,
		Roles:          0,
	}
	cache, err := NewACLCaches(&config)
	require.NoError(t, err)

	testPolicies := ACLPolicies{
		&ACLPolicy{
			ID:          "5d5653a1-2c2b-4b36-b083-fc9f1398eb7b",
			Name:        "policy1",
			Description: "policy1",
			Rules:       `node_prefix "" { policy = "read" }`,
			Syntax:      acl.SyntaxCurrent,
			RaftIndex: RaftIndex{
				CreateIndex: 1,
				ModifyIndex: 2,
			},
		},
		&ACLPolicy{
			ID:          "b35541f0-a88a-48da-bc66-43553c60b628",
			Name:        "policy2",
			Description: "policy2",
			Rules:       `agent_prefix "" { policy = "read" }`,
			Syntax:      acl.SyntaxCurrent,
			RaftIndex: RaftIndex{
				CreateIndex: 3,
				ModifyIndex: 4,
			},
		},
		&ACLPolicy{
			ID:          "383abb79-94ca-46c6-89b7-8ecb69046de9",
			Name:        "policy3",
			Description: "policy3",
			Rules:       `key_prefix "" { policy = "read" }`,
			Syntax:      acl.SyntaxCurrent,
			RaftIndex: RaftIndex{
				CreateIndex: 5,
				ModifyIndex: 6,
			},
		},
		&ACLPolicy{
			ID:          "8bf38965-95e5-4e86-9be7-f6070cc0708b",
			Name:        "policy4",
			Description: "policy4",
			Rules:       `service_prefix "" { policy = "read" }`,
			Syntax:      acl.SyntaxCurrent,
			RaftIndex: RaftIndex{
				CreateIndex: 7,
				ModifyIndex: 8,
			},
		},
	}

	t.Run("Cache Miss", func(t *testing.T) {
		authz, err := NewPolicyAuthorizerWithCache(testPolicies, cache, nil)
		require.NoError(t, err)
		require.NotNil(t, authz)

		require.Equal(t, acl.Allow, authz.NodeRead("foo", nil))
		require.Equal(t, acl.Allow, authz.AgentRead("foo", nil))
		require.Equal(t, acl.Allow, authz.KeyRead("foo", nil))
		require.Equal(t, acl.Allow, authz.ServiceRead("foo", nil))
		require.Equal(t, acl.Default, authz.ACLRead(nil))
	})

	t.Run("Check Cache", func(t *testing.T) {
		entry := cache.GetAuthorizer(testPolicies.HashKey())
		require.NotNil(t, entry)
		authz := entry.Authorizer
		require.NotNil(t, authz)

		require.Equal(t, acl.Allow, authz.NodeRead("foo", nil))
		require.Equal(t, acl.Allow, authz.AgentRead("foo", nil))
		require.Equal(t, acl.Allow, authz.KeyRead("foo", nil))
		require.Equal(t, acl.Allow, authz.ServiceRead("foo", nil))
		require.Equal(t, acl.Default, authz.ACLRead(nil))

		// setup the cache for the next test
		cache.PutAuthorizer(testPolicies.HashKey(), acl.DenyAll())
	})

	t.Run("Cache Hit", func(t *testing.T) {
		authz, err := NewPolicyAuthorizerWithCache(testPolicies, cache, nil)
		require.NoError(t, err)
		require.NotNil(t, authz)

		// we reset the Authorizer in the cache so now everything should be defaulted
		require.Equal(t, acl.Deny, authz.NodeRead("foo", nil))
		require.Equal(t, acl.Deny, authz.AgentRead("foo", nil))
		require.Equal(t, acl.Deny, authz.KeyRead("foo", nil))
		require.Equal(t, acl.Deny, authz.ServiceRead("foo", nil))
		require.Equal(t, acl.Deny, authz.ACLRead(nil))
	})
}

func TestResolveWithCache(t *testing.T) {
	config := ACLCachesConfig{
		Identities:     0,
		Policies:       0,
		ParsedPolicies: 4,
		Authorizers:    0,
		Roles:          0,
	}
	cache, err := NewACLCaches(&config)
	require.NoError(t, err)

	testPolicies := ACLPolicies{
		&ACLPolicy{
			ID:          "5d5653a1-2c2b-4b36-b083-fc9f1398eb7b",
			Name:        "policy1",
			Description: "policy1",
			Rules:       `node_prefix "" { policy = "read" }`,
			Syntax:      acl.SyntaxCurrent,
			RaftIndex: RaftIndex{
				CreateIndex: 1,
				ModifyIndex: 2,
			},
		},
		&ACLPolicy{
			ID:          "b35541f0-a88a-48da-bc66-43553c60b628",
			Name:        "policy2",
			Description: "policy2",
			Rules:       `agent_prefix "" { policy = "read" }`,
			Syntax:      acl.SyntaxCurrent,
			RaftIndex: RaftIndex{
				CreateIndex: 3,
				ModifyIndex: 4,
			},
		},
		&ACLPolicy{
			ID:          "383abb79-94ca-46c6-89b7-8ecb69046de9",
			Name:        "policy3",
			Description: "policy3",
			Rules:       `key_prefix "" { policy = "read" }`,
			Syntax:      acl.SyntaxCurrent,
			RaftIndex: RaftIndex{
				CreateIndex: 5,
				ModifyIndex: 6,
			},
		},
		&ACLPolicy{
			ID:          "8bf38965-95e5-4e86-9be7-f6070cc0708b",
			Name:        "policy4",
			Description: "policy4",
			Rules:       `service_prefix "" { policy = "read" }`,
			Syntax:      acl.SyntaxCurrent,
			RaftIndex: RaftIndex{
				CreateIndex: 7,
				ModifyIndex: 8,
			},
		},
	}

	t.Run("Cache Misses", func(t *testing.T) {
		policies, err := resolveWithCache(testPolicies, cache, nil)
		require.NoError(t, err)
		require.Len(t, policies, 4)
		require.Len(t, policies[0].NodePrefixes, 1)
		require.Len(t, policies[1].AgentPrefixes, 1)
		require.Len(t, policies[2].KeyPrefixes, 1)
		require.Len(t, policies[3].ServicePrefixes, 1)
	})

	t.Run("Check Cache", func(t *testing.T) {
		for i := range testPolicies {
			entry := cache.GetParsedPolicy(fmt.Sprintf("%x", testPolicies[i].Hash))
			require.NotNil(t, entry)

			// set this to detect using from the cache next time
			testPolicies[i].Rules = "invalid"
		}
	})

	t.Run("Cache Hits", func(t *testing.T) {
		policies, err := resolveWithCache(testPolicies, cache, nil)
		require.NoError(t, err)
		require.Len(t, policies, 4)
		require.Len(t, policies[0].NodePrefixes, 1)
		require.Len(t, policies[1].AgentPrefixes, 1)
		require.Len(t, policies[2].KeyPrefixes, 1)
		require.Len(t, policies[3].ServicePrefixes, 1)
	})
}
