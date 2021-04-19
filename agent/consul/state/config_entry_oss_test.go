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
				source:   &structs.ProxyConfigEntry{},
				expected: []byte("proxy-defaults\x00global\x00"),
			},
			extra: []indexerTestCase{
				{
					write: indexValue{
						source:   &structs.ServiceConfigEntry{Name: "NaMe"},
						expected: []byte("service-defaults\x00name\x00"),
					},
				},
			},
		},
		indexKind: {
			read: indexValue{
				source: ConfigEntryKindQuery{
					Kind: "Service-Defaults",
				},
				expected: []byte("service-defaults\x00"),
			},
			write: indexValue{
				source:   &structs.ServiceConfigEntry{},
				expected: []byte("service-defaults\x00"),
			},
		},
	}
}
