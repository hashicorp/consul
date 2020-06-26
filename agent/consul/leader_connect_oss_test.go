// +build !consulent

package consul

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/structs"
	tokenStore "github.com/hashicorp/consul/agent/token"
	"github.com/stretchr/testify/require"
)

func TestLeader_OSS_IntentionUpgradeCleanup(t *testing.T) {
	t.Parallel()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.ACLDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLMasterToken = "root"
		c.ACLDefaultPolicy = "deny"
		// set the build to ensure all the version checks pass and enable all the connect features that operate cross-dc
		c.Build = "1.6.0"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	waitForLeaderEstablishment(t, s1)

	s1.tokens.UpdateAgentToken("root", tokenStore.TokenSourceConfig)

	lastIndex := uint64(0)
	nextIndex := func() uint64 {
		lastIndex++
		return lastIndex
	}

	wildEntMeta := structs.WildcardEnterpriseMeta()

	resetIntentions := func(t *testing.T) {
		t.Helper()
		_, ixns, err := s1.fsm.State().Intentions(nil, wildEntMeta)
		require.NoError(t, err)

		for _, ixn := range ixns {
			require.NoError(t, s1.fsm.State().IntentionDelete(nextIndex(), ixn.ID))
		}
	}

	compare := func(t *testing.T, expect [][]string) {
		t.Helper()

		_, ixns, err := s1.fsm.State().Intentions(nil, wildEntMeta)
		require.NoError(t, err)

		var actual [][]string
		for _, ixn := range ixns {
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
			resetIntentions(t)

			// Do something super evil and directly reach into the FSM to seed it with "bad" data.
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
				require.NoError(t, s1.fsm.State().IntentionSet(nextIndex(), ixn))
			}

			// Sleep a bit so that the UpdatedAt field will definitely be different
			time.Sleep(1 * time.Millisecond)

			// TODO: figure out how to test this properly during leader startup

			require.NoError(t, s1.runIntentionUpgradeCleanup(context.Background()))

			compare(t, tc.expect)
		})
	}
}
