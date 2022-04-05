//go:build !consulent
// +build !consulent

package state

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/structs"
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
