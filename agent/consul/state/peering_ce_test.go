//go:build !consulent
// +build !consulent

package state

import (
	"time"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbpeering"
)

func testIndexerTablePeering() map[string]indexerTestCase {
	id := "432feb2f-5476-4ae2-b33c-e43640ca0e86"
	encodedID := []byte{0x43, 0x2f, 0xeb, 0x2f, 0x54, 0x76, 0x4a, 0xe2, 0xb3, 0x3c, 0xe4, 0x36, 0x40, 0xca, 0xe, 0x86}

	obj := &pbpeering.Peering{
		Name:      "TheName",
		ID:        id,
		DeletedAt: structs.TimeToProto(time.Now()),
	}

	return map[string]indexerTestCase{
		indexID: {
			read: indexValue{
				source:   "432feb2f-5476-4ae2-b33c-e43640ca0e86",
				expected: encodedID,
			},
			write: indexValue{
				source:   obj,
				expected: encodedID,
			},
		},
		indexName: {
			read: indexValue{
				source: Query{
					Value:          "TheNAME",
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInPartition("pArTition"),
				},
				expected: []byte("thename\x00"),
			},
			write: indexValue{
				source:   obj,
				expected: []byte("thename\x00"),
			},
			prefix: []indexValue{
				{
					source:   *structs.DefaultEnterpriseMetaInPartition("pArTition"),
					expected: nil,
				},
			},
		},
		indexDeleted: {
			read: indexValue{
				source: BoolQuery{
					Value:          true,
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInPartition("partITION"),
				},
				expected: []byte("\x01"),
			},
			write: indexValue{
				source:   obj,
				expected: []byte("\x01"),
			},
			extra: []indexerTestCase{
				{
					read: indexValue{
						source: BoolQuery{
							Value:          false,
							EnterpriseMeta: *structs.DefaultEnterpriseMetaInPartition("partITION"),
						},
						expected: []byte("\x00"),
					},
					write: indexValue{
						source: &pbpeering.Peering{
							Name:      "TheName",
							Partition: "PartItioN",
						},
						expected: []byte("\x00"),
					},
				},
			},
		},
	}
}
