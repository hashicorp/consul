// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dns

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/sdk/testutil/retry"
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
		require.ElementsMatch(r, configuredRecursors, recursorsToQuery)

		// Ensure the elements are not in the same order
		require.NotEqual(r, configuredRecursors, recursorsToQuery)
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
