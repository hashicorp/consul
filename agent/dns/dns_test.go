package dns

import (
	"reflect"
	"testing"

	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/stretchr/testify/require"
)

func TestDNS_Recursor_StrategyRandom(t *testing.T) {
	configuredRecursors := []string{"1.1.1.1", "8.8.4.4", "8.8.8.8"}
	recursorStrategy := RecursorStrategy("random")

	retry.RunWith(&retry.Counter{Count: 5}, t, func(r *retry.R) {
		recursorsToQuery := make([]string, 0)
		for _, idx := range recursorStrategy.Indexes(len(configuredRecursors)) {
			recursorsToQuery = append(recursorsToQuery, configuredRecursors[idx])
		}

		// Ensure the slices contain the same elements
		require.ElementsMatch(t, configuredRecursors, recursorsToQuery)

		if reflect.DeepEqual(configuredRecursors, recursorsToQuery) {
			// Error if the elements are in the same order, and retry generating
			// random recursor list
			r.Fatal("dns recursor order is not randomized.")
		}
	})
}

func TestDNS_Recursor_StrategySequential(t *testing.T) {
	expectedRecursors := []string{"1.1.1.1", "8.8.4.4", "8.8.8.8"}
	recursorStrategy := RecursorStrategy("sequential")

	recursorsToQuery := make([]string, 0)
	for _, idx := range recursorStrategy.Indexes(len(expectedRecursors)) {
		recursorsToQuery = append(recursorsToQuery, expectedRecursors[idx])
	}

	// The list of recursors should match the order in which they were defined
	// in the configuration
	require.Equal(t, recursorsToQuery, expectedRecursors)
}
