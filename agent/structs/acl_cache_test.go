package structs

import (
	"testing"

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

	// TODO (ACL-V2) test the other Get/Put/Remote/Purge functions
}
