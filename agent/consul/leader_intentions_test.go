// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/structs"
	tokenStore "github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
)

func TestLeader_ReplicateIntentions(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	// This setup is a little hacky, but should work. We spin up BOTH servers with
	// no intentions and force them to think they're not eligible for intentions
	// config entries yet by overriding serf tags.

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
		c.Build = "1.6.0"
		c.OverrideInitialSerfTags = func(tags map[string]string) {
			tags["ft_si"] = "0"
		}
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	waitForLeaderEstablishment(t, s1)

	retry.Run(t, func(r *retry.R) {
		if s1.DatacenterSupportsIntentionsAsConfigEntries() {
			r.Fatal("server 1 shouldn't activate service-intentions")
		}
	})

	s1.tokens.UpdateAgentToken("root", tokenStore.TokenSourceConfig)

	replicationRules := `acl = "read" service_prefix "" { policy = "read" intentions = "read" } operator = "write" `
	// create some tokens
	replToken1, err := upsertTestTokenWithPolicyRules(codec, "root", "dc1", replicationRules)
	require.NoError(t, err)

	replToken2, err := upsertTestTokenWithPolicyRules(codec, "root", "dc1", replicationRules)
	require.NoError(t, err)

	// dc2 as a secondary DC
	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
		c.ACLTokenReplication = false
		c.Build = "1.6.0"
		c.OverrideInitialSerfTags = func(tags map[string]string) {
			tags["ft_si"] = "0"
		}
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	s2.tokens.UpdateAgentToken("root", tokenStore.TokenSourceConfig)

	// start out with one token
	s2.tokens.UpdateReplicationToken(replToken1.SecretID, tokenStore.TokenSourceConfig)

	// Create the WAN link
	joinWAN(t, s2, s1)
	waitForLeaderEstablishment(t, s2)

	retry.Run(t, func(r *retry.R) {
		if s2.DatacenterSupportsIntentionsAsConfigEntries() {
			r.Fatal("server 2 shouldn't activate service-intentions")
		}
	})

	legacyApply := func(s *Server, req *structs.IntentionRequest) error {
		if req.Op != structs.IntentionOpDelete {
			// Do these directly on the inputs so it's corrected for future
			// equality checks.
			req.Intention.CreatedAt = time.Now().UTC()
			req.Intention.UpdatedAt = req.Intention.CreatedAt
			//nolint:staticcheck
			req.Intention.UpdatePrecedence()
			//nolint:staticcheck
			require.NoError(t, req.Intention.Validate())
			//nolint:staticcheck
			req.Intention.SetHash()
		}

		req2 := *req
		req2.Intention = req.Intention.Clone()
		if req.Op != structs.IntentionOpDelete {
			req2.Intention.Hash = req.Intention.Hash // not part of Clone
		}
		_, err := s.raftApply(structs.IntentionRequestType, req2)
		return err
	}

	// Directly insert legacy intentions into raft in dc1.
	id := generateUUID()
	ixn := structs.IntentionRequest{
		Datacenter:   "dc1",
		WriteRequest: structs.WriteRequest{Token: "root"},
		Op:           structs.IntentionOpCreate,
		Intention: &structs.Intention{
			ID:              id,
			SourceNS:        structs.IntentionDefaultNamespace,
			SourceName:      "test",
			DestinationNS:   structs.IntentionDefaultNamespace,
			DestinationName: "test",
			Action:          structs.IntentionActionAllow,
			SourceType:      structs.IntentionSourceConsul,
			Meta:            map[string]string{},
		},
	}
	require.NoError(t, legacyApply(s1, &ixn))

	// Wait for it to get replicated to dc2
	var createdAt time.Time
	retry.Run(t, func(r *retry.R) {
		req := &structs.IntentionQueryRequest{
			Datacenter:   "dc2",
			QueryOptions: structs.QueryOptions{Token: "root"},
			IntentionID:  ixn.Intention.ID,
		}
		var resp structs.IndexedIntentions
		require.NoError(r, s2.RPC(context.Background(), "Intention.Get", req, &resp), "ID=%q", ixn.Intention.ID)
		require.Len(r, resp.Intentions, 1)

		actual := resp.Intentions[0]
		createdAt = actual.CreatedAt
	})

	// Sleep a bit so that the UpdatedAt field will definitely be different
	time.Sleep(1 * time.Millisecond)

	// delete underlying acl token being used for replication
	require.NoError(t, deleteTestToken(codec, "root", "dc1", replToken1.AccessorID))

	// switch to the other token
	s2.tokens.UpdateReplicationToken(replToken2.SecretID, tokenStore.TokenSourceConfig)

	// Update the intention in dc1
	ixn.Op = structs.IntentionOpUpdate
	ixn.Intention.ID = id
	ixn.Intention.SourceName = "*"
	require.NoError(t, legacyApply(s1, &ixn))

	// Wait for dc2 to get the update
	var resp structs.IndexedIntentions
	retry.Run(t, func(r *retry.R) {
		req := &structs.IntentionQueryRequest{
			Datacenter:   "dc2",
			QueryOptions: structs.QueryOptions{Token: "root"},
			IntentionID:  ixn.Intention.ID,
		}

		require.NoError(r, s2.RPC(context.Background(), "Intention.Get", req, &resp), "ID=%q", ixn.Intention.ID)
		require.Len(r, resp.Intentions, 1)
		require.Equal(r, "*", resp.Intentions[0].SourceName)
	})

	actual := resp.Intentions[0]
	require.Equal(t, createdAt, actual.CreatedAt)
	require.WithinDuration(t, time.Now(), actual.UpdatedAt, 5*time.Second)

	actual.CreateIndex, actual.ModifyIndex = 0, 0
	actual.CreatedAt = ixn.Intention.CreatedAt
	actual.UpdatedAt = ixn.Intention.UpdatedAt
	//nolint:staticcheck
	ixn.Intention.UpdatePrecedence()
	require.Equal(t, ixn.Intention, actual)

	// Delete
	require.NoError(t, legacyApply(s1, &structs.IntentionRequest{
		Datacenter:   "dc1",
		WriteRequest: structs.WriteRequest{Token: "root"},
		Op:           structs.IntentionOpDelete,
		Intention: &structs.Intention{
			ID: ixn.Intention.ID,
		},
	}))

	// Wait for the delete to be replicated
	retry.Run(t, func(r *retry.R) {
		req := &structs.IntentionQueryRequest{
			Datacenter:   "dc2",
			QueryOptions: structs.QueryOptions{Token: "root"},
			IntentionID:  ixn.Intention.ID,
		}
		var resp structs.IndexedIntentions
		err := s2.RPC(context.Background(), "Intention.Get", req, &resp)
		require.Error(r, err)
		if !strings.Contains(err.Error(), ErrIntentionNotFound.Error()) {
			r.Fatalf("expected intention not found, got: %v", err)
		}
	})
}

//nolint:staticcheck
func TestLeader_batchLegacyIntentionUpdates(t *testing.T) {
	t.Parallel()

	ixn1 := structs.TestIntention(t)
	ixn1.ID = "ixn1"
	ixn2 := structs.TestIntention(t)
	ixn2.ID = "ixn2"
	ixnLarge := structs.TestIntention(t)
	ixnLarge.ID = "ixnLarge"
	ixnLarge.Description = strings.Repeat("x", maxIntentionTxnSize-1)

	cases := []struct {
		deletes  structs.Intentions
		updates  structs.Intentions
		expected []structs.TxnOps
	}{
		// 1 deletes, 0 updates
		{
			deletes: structs.Intentions{ixn1},
			expected: []structs.TxnOps{
				{
					&structs.TxnOp{
						Intention: &structs.TxnIntentionOp{
							Op:        structs.IntentionOpDelete,
							Intention: ixn1,
						},
					},
				},
			},
		},
		// 0 deletes, 1 updates
		{
			updates: structs.Intentions{ixn1},
			expected: []structs.TxnOps{
				{
					&structs.TxnOp{
						Intention: &structs.TxnIntentionOp{
							Op:        structs.IntentionOpUpdate,
							Intention: ixn1,
						},
					},
				},
			},
		},
		// 1 deletes, 1 updates
		{
			deletes: structs.Intentions{ixn1},
			updates: structs.Intentions{ixn2},
			expected: []structs.TxnOps{
				{
					&structs.TxnOp{
						Intention: &structs.TxnIntentionOp{
							Op:        structs.IntentionOpDelete,
							Intention: ixn1,
						},
					},
					&structs.TxnOp{
						Intention: &structs.TxnIntentionOp{
							Op:        structs.IntentionOpUpdate,
							Intention: ixn2,
						},
					},
				},
			},
		},
		// 1 large intention update
		{
			updates: structs.Intentions{ixnLarge},
			expected: []structs.TxnOps{
				{
					&structs.TxnOp{
						Intention: &structs.TxnIntentionOp{
							Op:        structs.IntentionOpUpdate,
							Intention: ixnLarge,
						},
					},
				},
			},
		},
		// 2 deletes (w/ a large intention), 1 updates
		{
			deletes: structs.Intentions{ixn1, ixnLarge},
			updates: structs.Intentions{ixn2},
			expected: []structs.TxnOps{
				{
					&structs.TxnOp{
						Intention: &structs.TxnIntentionOp{
							Op:        structs.IntentionOpDelete,
							Intention: ixn1,
						},
					},
					&structs.TxnOp{
						Intention: &structs.TxnIntentionOp{
							Op:        structs.IntentionOpDelete,
							Intention: ixnLarge,
						},
					},
				},
				{
					&structs.TxnOp{
						Intention: &structs.TxnIntentionOp{
							Op:        structs.IntentionOpUpdate,
							Intention: ixn2,
						},
					},
				},
			},
		},
		// 1 deletes , 2 updates (w/ a large intention)
		{
			deletes: structs.Intentions{ixn1},
			updates: structs.Intentions{ixnLarge, ixn2},
			expected: []structs.TxnOps{
				{
					&structs.TxnOp{
						Intention: &structs.TxnIntentionOp{
							Op:        structs.IntentionOpDelete,
							Intention: ixn1,
						},
					},
					&structs.TxnOp{
						Intention: &structs.TxnIntentionOp{
							Op:        structs.IntentionOpUpdate,
							Intention: ixnLarge,
						},
					},
				},
				{
					&structs.TxnOp{
						Intention: &structs.TxnIntentionOp{
							Op:        structs.IntentionOpUpdate,
							Intention: ixn2,
						},
					},
				},
			},
		},
	}

	for _, tc := range cases {
		actual := batchLegacyIntentionUpdates(tc.deletes, tc.updates)
		assert.Equal(t, tc.expected, actual)
	}
}

func TestLeader_LegacyIntentionMigration(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	// This setup is a little hacky, but should work. We spin up a server with
	// no intentions and force it to think it's not eligible for intentions
	// config entries yet by overriding serf tags.
	//
	// Then we directly write legacy intentions into raft. This is mimicking
	// what a service-intentions aware server might do if an older copy of
	// consul was still leader.
	//
	// This lets us generate a snapshot+raft state containing legacy intentions
	// without having to spin up an old version of consul for the test.
	//
	// Then we shut it down and bring up a new copy on that datadir which
	// should then trigger migration code.
	dir1pre, s1pre := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.Build = "1.6.0"
		c.OverrideInitialSerfTags = func(tags map[string]string) {
			tags["ft_si"] = "0"
		}
	})
	defer os.RemoveAll(dir1pre)
	defer s1pre.Shutdown()

	testrpc.WaitForLeader(t, s1pre.RPC, "dc1")

	retry.Run(t, func(r *retry.R) {
		if s1pre.DatacenterSupportsIntentionsAsConfigEntries() {
			r.Fatal("server 1 shouldn't activate service-intentions")
		}
	})

	// Insert a bunch of legacy intentions.
	makeIxn := func(src, dest string, allow bool) *structs.Intention {
		ixn := &structs.Intention{
			ID:              generateUUID(),
			SourceNS:        structs.IntentionDefaultNamespace,
			SourceName:      src,
			DestinationNS:   structs.IntentionDefaultNamespace,
			DestinationName: dest,
			SourceType:      structs.IntentionSourceConsul,
			Meta:            map[string]string{},
		}

		if allow {
			ixn.Action = structs.IntentionActionAllow
		} else {
			ixn.Action = structs.IntentionActionDeny
		}

		//nolint:staticcheck
		ixn.UpdatePrecedence()
		//nolint:staticcheck
		ixn.SetHash()
		return ixn
	}
	ixns := []*structs.Intention{
		makeIxn("api", "db", true),
		makeIxn("web", "db", false),
		makeIxn("*", "web", true),
		makeIxn("*", "api", false),
		makeIxn("intern", "*", false),
		makeIxn("contractor", "*", false),
		makeIxn("*", "*", true),
	}
	ixns = appendLegacyIntentionsForMigrationTestEnterprise(t, s1pre, ixns)

	testLeader_LegacyIntentionMigrationHookEnterprise(t, s1pre, true)

	var retained []*structs.Intention
	for _, ixn := range ixns {
		ixn2 := *ixn
		_, err := s1pre.raftApply(structs.IntentionRequestType, &structs.IntentionRequest{
			Op:        structs.IntentionOpCreate,
			Intention: &ixn2,
		})
		require.NoError(t, err)

		if _, present := ixn.Meta["unit-test-discarded"]; !present {
			retained = append(retained, ixn)
		}
	}

	mapify := func(ixns []*structs.Intention) map[string]*structs.Intention {
		m := make(map[string]*structs.Intention)
		for _, ixn := range ixns {
			m[ixn.ID] = ixn
		}
		return m
	}

	checkIntentions := func(t *testing.T, srv *Server, legacyOnly bool, expect map[string]*structs.Intention) {
		t.Helper()
		wildMeta := structs.WildcardEnterpriseMetaInDefaultPartition()
		retry.Run(t, func(r *retry.R) {
			var (
				got structs.Intentions
				err error
			)
			if legacyOnly {
				_, got, err = srv.fsm.State().LegacyIntentions(nil, wildMeta)
			} else {
				_, got, _, err = srv.fsm.State().Intentions(nil, wildMeta)
			}
			require.NoError(r, err)
			gotM := mapify(got)

			assert.Len(r, gotM, len(expect))
			for k, expectV := range expect {
				gotV, ok := gotM[k]
				if !ok {
					r.Errorf("results are missing key %q: %v", k, expectV)
					continue
				}

				assert.Equal(r, expectV.ID, gotV.ID)
				assert.Equal(r, expectV.SourceNS, gotV.SourceNS)
				assert.Equal(r, expectV.SourceName, gotV.SourceName)
				assert.Equal(r, expectV.DestinationNS, gotV.DestinationNS)
				assert.Equal(r, expectV.DestinationName, gotV.DestinationName)
				assert.Equal(r, expectV.Action, gotV.Action)
				assert.Equal(r, expectV.Meta, gotV.Meta)
				assert.Equal(r, expectV.Precedence, gotV.Precedence)
				assert.Equal(r, expectV.SourceType, gotV.SourceType)
			}
		})
	}

	expectM := mapify(ixns)
	expectRetainedM := mapify(retained)

	require.True(t, t.Run("check initial intentions", func(t *testing.T) {
		checkIntentions(t, s1pre, false, expectM)
	}))
	require.True(t, t.Run("check initial legacy intentions", func(t *testing.T) {
		checkIntentions(t, s1pre, true, expectM)
	}))

	// Shutdown s1pre and restart it to trigger migration.
	s1pre.Shutdown()

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.DataDir = s1pre.config.DataDir
		c.Datacenter = "dc1"
		c.NodeName = s1pre.config.NodeName
		c.NodeID = s1pre.config.NodeID
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	testLeader_LegacyIntentionMigrationHookEnterprise(t, s1, false)

	// Wait until the migration routine is complete.
	retry.Run(t, func(r *retry.R) {
		intentionFormat, err := s1.GetSystemMetadata(structs.SystemMetadataIntentionFormatKey)
		require.NoError(r, err)
		if intentionFormat != structs.SystemMetadataIntentionFormatConfigValue {
			r.Fatal("intention migration is not yet complete")
		}
	})

	// check that all 7 intentions are present the general way after migration
	require.True(t, t.Run("check migrated intentions", func(t *testing.T) {
		checkIntentions(t, s1, false, expectRetainedM)
	}))
	require.True(t, t.Run("check migrated legacy intentions", func(t *testing.T) {
		// check that no intentions exist in the legacy table
		checkIntentions(t, s1, true, map[string]*structs.Intention{})
	}))

	mapifyConfigs := func(entries interface{}) map[configentry.KindName]*structs.ServiceIntentionsConfigEntry {
		m := make(map[configentry.KindName]*structs.ServiceIntentionsConfigEntry)
		switch v := entries.(type) {
		case []*structs.ServiceIntentionsConfigEntry:
			for _, entry := range v {
				kn := configentry.NewKindName(entry.Kind, entry.Name, &entry.EnterpriseMeta)
				m[kn] = entry
			}
		case []structs.ConfigEntry:
			for _, entry := range v {
				kn := configentry.NewKindName(entry.GetKind(), entry.GetName(), entry.GetEnterpriseMeta())
				m[kn] = entry.(*structs.ServiceIntentionsConfigEntry)
			}
		default:
			t.Fatalf("bad type: %T", entries)
		}
		return m
	}

	// also check config entries
	_, gotConfigs, err := s1.fsm.State().ConfigEntriesByKind(nil, structs.ServiceIntentions, structs.WildcardEnterpriseMetaInDefaultPartition())
	require.NoError(t, err)
	gotConfigsM := mapifyConfigs(gotConfigs)

	expectConfigs := structs.MigrateIntentions(retained)
	for _, entry := range expectConfigs {
		require.NoError(t, entry.LegacyNormalize()) // tidy them up the same way the write would
	}
	expectConfigsM := mapifyConfigs(expectConfigs)

	assert.Len(t, gotConfigsM, len(expectConfigsM))
	for kn, expectV := range expectConfigsM {
		gotV, ok := gotConfigsM[kn]
		if !ok {
			t.Errorf("results are missing key %q", kn)
			continue
		}

		// Migrated intentions won't have toplevel Meta.
		assert.Nil(t, gotV.Meta)

		require.Len(t, gotV.Sources, len(expectV.Sources))

		expSrcMap := make(map[string]*structs.SourceIntention)
		for i, src := range expectV.Sources {
			require.NotEmpty(t, src.LegacyID, "index[%d] missing LegacyID", i)

			// Do a shallow copy and strip the times from the copy
			src2 := *src
			src2.LegacyCreateTime = nil
			src2.LegacyUpdateTime = nil
			expSrcMap[src2.LegacyID] = &src2
		}

		for i, got := range gotV.Sources {
			require.NotEmpty(t, got.LegacyID, "index[%d] missing LegacyID", i)

			// Do a shallow copy and strip the times from the copy
			got2 := *got
			got2.LegacyCreateTime = nil
			got2.LegacyUpdateTime = nil

			cmp, ok := expSrcMap[got2.LegacyID]
			require.True(t, ok, "missing %q", got2.LegacyID)

			assert.Equal(t, cmp, &got2, "index[%d]", i)
		}
	}
}
