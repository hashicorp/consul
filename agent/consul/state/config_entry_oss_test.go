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

func TestStore_peersForService(t *testing.T) {
	queryName := "foo"

	type testCase struct {
		name   string
		write  structs.ConfigEntry
		expect []string
	}

	cases := []testCase{
		{
			name:   "empty everything",
			expect: nil,
		},
		{
			name: "service is not exported",
			write: &structs.ExportedServicesConfigEntry{
				Name: "default",
				Services: []structs.ExportedService{
					{
						Name: "not-" + queryName,
						Consumers: []structs.ServiceConsumer{
							{
								PeerName: "zip",
							},
						},
					},
				},
			},
			expect: nil,
		},
		{
			name: "wildcard name matches",
			write: &structs.ExportedServicesConfigEntry{
				Name: "default",
				Services: []structs.ExportedService{
					{
						Name: "not-" + queryName,
						Consumers: []structs.ServiceConsumer{
							{
								PeerName: "zip",
							},
						},
					},
					{
						Name: structs.WildcardSpecifier,
						Consumers: []structs.ServiceConsumer{
							{
								PeerName: "bar",
							},
							{
								PeerName: "baz",
							},
						},
					},
				},
			},
			expect: []string{"bar", "baz"},
		},
		{
			name: "exact name takes precedence over wildcard",
			write: &structs.ExportedServicesConfigEntry{
				Name: "default",
				Services: []structs.ExportedService{
					{
						Name: queryName,
						Consumers: []structs.ServiceConsumer{
							{
								PeerName: "baz",
							},
						},
					},
					{
						Name: structs.WildcardSpecifier,
						Consumers: []structs.ServiceConsumer{
							{
								PeerName: "zip",
							},
						},
					},
				},
			},
			expect: []string{"baz"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := testStateStore(t)
			var lastIdx uint64

			// Write the entry.
			if tc.write != nil {
				require.NoError(t, tc.write.Normalize())
				require.NoError(t, tc.write.Validate())

				lastIdx++
				require.NoError(t, s.EnsureConfigEntry(lastIdx, tc.write))
			}

			// Read the entries back.
			tx := s.db.ReadTxn()
			defer tx.Abort()

			idx, peers, err := peersForServiceTxn(tx, nil, queryName, acl.DefaultEnterpriseMeta())
			require.NoError(t, err)

			// This is a little weird, but when there are no results, the index returned should be the max index for the
			// config entries table so that the caller can watch for changes to it
			if len(peers) == 0 {
				require.Equal(t, maxIndexTxn(tx, tableConfigEntries), idx)
			} else {
				require.Equal(t, lastIdx, idx)
			}

			// Verify the result.
			require.Len(t, peers, len(tc.expect))
			require.Equal(t, tc.expect, peers)
		})
	}
}
