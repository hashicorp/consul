//go:build !consulent
// +build !consulent

package state

import (
	"testing"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/require"
)

func testIndexerTableConfigEntries() map[string]indexerTestCase {
	return map[string]indexerTestCase{
		indexID: {
			read: indexValue{
				source: configentry.KindName{
					Kind: "Proxy-Defaults",
					Name: "NaMe",
				},
				expected: []byte("proxy-defaults\x00name\x00"),
			},
			write: indexValue{
				source:   &structs.ProxyConfigEntry{Name: "NaMe"},
				expected: []byte("proxy-defaults\x00name\x00"),
			},
			prefix: []indexValue{
				{
					source:   acl.EnterpriseMeta{},
					expected: nil,
				},
				{
					source:   ConfigEntryKindQuery{Kind: "Proxy-Defaults"},
					expected: []byte("proxy-defaults\x00"),
				},
			},
		},
	}
}

func TestStore_ExportedServices(t *testing.T) {
	type testCase struct {
		name   string
		write  []structs.ConfigEntry
		query  string
		expect []*structs.ExportedServicesConfigEntry
	}

	cases := []testCase{
		{
			name:   "empty everything",
			write:  []structs.ConfigEntry{},
			query:  "foo",
			expect: []*structs.ExportedServicesConfigEntry{},
		},
		{
			name: "no matching exported services",
			write: []structs.ConfigEntry{
				&structs.ProxyConfigEntry{Name: "foo"},
				&structs.ProxyConfigEntry{Name: "bar"},
				&structs.ExportedServicesConfigEntry{
					Name: "baz",
					Services: []structs.ExportedService{
						{Name: "baz"},
					},
				},
			},
			query:  "foo",
			expect: []*structs.ExportedServicesConfigEntry{},
		},
		{
			name: "exact match service name",
			write: []structs.ConfigEntry{
				&structs.ExportedServicesConfigEntry{
					Name: "foo",
					Services: []structs.ExportedService{
						{Name: "foo"},
					},
				},
				&structs.ExportedServicesConfigEntry{
					Name: "bar",
					Services: []structs.ExportedService{
						{Name: "bar"},
					},
				},
			},
			query: "bar",
			expect: []*structs.ExportedServicesConfigEntry{
				{
					Name: "bar",
					Services: []structs.ExportedService{
						{Name: "bar"},
					},
				},
			},
		},
		{
			name: "wildcard match on service name",
			write: []structs.ConfigEntry{
				&structs.ExportedServicesConfigEntry{
					Name: "foo",
					Services: []structs.ExportedService{
						{Name: "foo"},
					},
				},
				&structs.ExportedServicesConfigEntry{
					Name: "wildcard",
					Services: []structs.ExportedService{
						{Name: structs.WildcardSpecifier},
					},
				},
			},
			query: "foo",
			expect: []*structs.ExportedServicesConfigEntry{
				{
					Name: "foo",
					Services: []structs.ExportedService{
						{Name: "foo"},
					},
				},
				{
					Name: "wildcard",
					Services: []structs.ExportedService{
						{Name: structs.WildcardSpecifier},
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := testStateStore(t)

			// Write the entries.
			for idx, entry := range tc.write {
				require.NoError(t, s.EnsureConfigEntry(uint64(idx+1), entry))
			}

			// Read the entries back.
			tx := s.db.ReadTxn()
			defer tx.Abort()
			idx, entries, err := getExportedServiceConfigEntriesTxn(tx, nil, tc.query, acl.DefaultEnterpriseMeta())
			require.NoError(t, err)
			require.Equal(t, uint64(len(tc.write)), idx)

			// Verify the result.
			require.Len(t, entries, len(tc.expect))
			for idx, got := range entries {
				// ignore raft fields
				got.ModifyIndex = 0
				got.CreateIndex = 0
				require.Equal(t, tc.expect[idx], got)
			}
		})
	}
}
