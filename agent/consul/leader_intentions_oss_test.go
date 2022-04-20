//go:build !consulent
// +build !consulent

package consul

import (
	"sort"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/require"
)

func testLeader_LegacyIntentionMigrationHookEnterprise(_ *testing.T, _ *Server, _ bool) {
}

func appendLegacyIntentionsForMigrationTestEnterprise(_ *testing.T, _ *Server, ixns []*structs.Intention) []*structs.Intention {
	return ixns
}

func TestMigrateIntentionsToConfigEntries(t *testing.T) {
	compare := func(t *testing.T, got structs.Intentions, expect [][]string) {
		t.Helper()

		var actual [][]string
		for _, ixn := range got {
			actual = append(actual, []string{
				ixn.SourceNS,
				ixn.SourceName,
				ixn.DestinationNS,
				ixn.DestinationName,
			})
		}
		require.ElementsMatch(t, expect, actual)
	}

	type testCase struct {
		insert [][]string
		expect [][]string
	}

	cases := map[string]testCase{
		"no change": {
			insert: [][]string{
				{"default", "foo", "default", "bar"},
				{"default", "*", "default", "*"},
			},
			expect: [][]string{
				{"default", "foo", "default", "bar"},
				{"default", "*", "default", "*"},
			},
		},
		"non-wildcard deletions": {
			insert: [][]string{
				{"default", "foo", "default", "bar"},
				{"alpha", "*", "default", "bar"},
				{"default", "foo", "beta", "*"},
				{"alpha", "zoo", "beta", "bar"},
			},
			expect: [][]string{
				{"default", "foo", "default", "bar"},
			},
		},
		"updates with no deletions and no collisions": {
			insert: [][]string{
				{"default", "foo", "default", "bar"},
				{"default", "foo", "*", "*"},
				{"*", "*", "default", "bar"},
				{"*", "*", "*", "*"},
			},
			expect: [][]string{
				{"default", "foo", "default", "bar"},
				{"default", "foo", "default", "*"},
				{"default", "*", "default", "bar"},
				{"default", "*", "default", "*"},
			},
		},
		"updates with only collision deletions": {
			insert: [][]string{
				{"default", "foo", "default", "bar"},
				{"default", "foo", "default", "*"},
				{"default", "foo", "*", "*"},
				{"default", "*", "default", "bar"},
				{"*", "*", "default", "bar"},
				{"default", "*", "default", "*"},
				{"*", "*", "*", "*"},
			},
			expect: [][]string{
				{"default", "foo", "default", "bar"},
				{"default", "foo", "default", "*"},
				{"default", "*", "default", "bar"},
				{"default", "*", "default", "*"},
			},
		},
		"a bit of everything": {
			insert: [][]string{
				{"default", "foo", "default", "bar"}, // retained
				{"default", "foo", "*", "*"},         // upgrade
				{"default", "*", "default", "bar"},   // retained in collision
				{"*", "*", "default", "bar"},         // deleted in collision
				{"default", "*", "default", "*"},     // retained in collision
				{"*", "*", "*", "*"},                 // deleted in collision
				{"alpha", "*", "default", "bar"},     // deleted
				{"default", "foo", "beta", "*"},      // deleted
				{"alpha", "zoo", "beta", "bar"},      // deleted
			},
			expect: [][]string{
				{"default", "foo", "default", "bar"},
				{"default", "foo", "default", "*"},
				{"default", "*", "default", "bar"},
				{"default", "*", "default", "*"},
			},
		},
	}
	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {

			// Do something super evil and directly reach into the FSM to seed it with "bad" data.
			var ixns structs.Intentions
			for _, elem := range tc.insert {
				require.Len(t, elem, 4)
				ixn := structs.TestIntention(t)
				ixn.ID = generateUUID()
				ixn.SourceNS = elem[0]
				ixn.SourceName = elem[1]
				ixn.DestinationNS = elem[2]
				ixn.DestinationName = elem[3]
				ixn.CreatedAt = time.Now().UTC()
				ixn.UpdatedAt = ixn.CreatedAt

				ixns = append(ixns, ixn)
			}

			// Sleep a bit so that the UpdatedAt field will definitely be different
			time.Sleep(1 * time.Millisecond)

			got := migrateIntentionsToConfigEntries(ixns)

			// Convert them back to the line-item version.
			var gotIxns structs.Intentions
			for _, entry := range got {
				gotIxns = append(gotIxns, entry.ToIntentions()...)
			}
			sort.Sort(structs.IntentionPrecedenceSorter(gotIxns))

			compare(t, gotIxns, tc.expect)
		})
	}
}
