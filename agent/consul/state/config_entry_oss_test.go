//go:build !consulent
// +build !consulent

package state

import "github.com/hashicorp/consul/agent/structs"

func testIndexerTableConfigEntries() map[string]indexerTestCase {
	return map[string]indexerTestCase{
		indexID: {
			read: indexValue{
				source: ConfigEntryKindName{
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
					source:   structs.EnterpriseMeta{},
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
