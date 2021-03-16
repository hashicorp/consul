// +build !consulent

package state

import "github.com/hashicorp/consul/agent/structs"

func testIndexerTableACLPolicies() map[string]indexerTestCase {
	obj := &structs.ACLPolicy{
		ID:   "123e4567-e89b-12d3-a456-426614174abc",
		Name: "PoLiCyNaMe",
	}
	encodedID := []byte{0x12, 0x3e, 0x45, 0x67, 0xe8, 0x9b, 0x12, 0xd3, 0xa4, 0x56, 0x42, 0x66, 0x14, 0x17, 0x4a, 0xbc}
	return map[string]indexerTestCase{
		indexID: {
			read: indexValue{
				source:   obj.ID,
				expected: encodedID,
			},
			write: indexValue{
				source:   obj,
				expected: encodedID,
			},
		},
		indexName: {
			read: indexValue{
				source:   Query{Value: "PolicyName"},
				expected: []byte("policyname\x00"),
			},
			write: indexValue{
				source:   obj,
				expected: []byte("policyname\x00"),
			},
		},
	}
}
