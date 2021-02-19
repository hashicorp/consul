// +build !consulent

package state

import "github.com/hashicorp/consul/agent/structs"

func testIndexerTableChecks() map[string]indexerTestCase {
	return map[string]indexerTestCase{
		indexNodeService: {
			read: indexValue{
				source: NodeServiceQuery{
					Node:    "NoDe",
					Service: "SeRvIcE",
				},
				expected: []byte("node\x00service\x00"),
			},
			write: indexValue{
				source: &structs.HealthCheck{
					Node:      "NoDe",
					ServiceID: "SeRvIcE",
				},
				expected: []byte("node\x00service\x00"),
			},
		},
	}
}

func testIndexerTableNodes() map[string]indexerTestCase {
	return map[string]indexerTestCase{
		indexID: {
			read: indexValue{
				source:   Query{Value: "NoDeId"},
				expected: []byte("nodeid\x00"),
			},
			write: indexValue{
				source:   &structs.Node{Node: "NoDeId"},
				expected: []byte("nodeid\x00"),
			},
		},
	}
}
