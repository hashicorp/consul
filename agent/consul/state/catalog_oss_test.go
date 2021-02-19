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
