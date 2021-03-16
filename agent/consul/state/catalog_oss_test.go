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
		indexNode: {
			read: indexValue{
				source: Query{
					Value: "NoDe",
				},
				expected: []byte("node\x00"),
			},
			write: indexValue{
				source: &structs.HealthCheck{
					Node:      "NoDe",
					ServiceID: "SeRvIcE",
				},
				expected: []byte("node\x00"),
			},
		},
	}
}

func testIndexerTableMeshTopology() map[string]indexerTestCase {
	obj := upstreamDownstream{
		Upstream:   structs.ServiceName{Name: "UpStReAm"},
		Downstream: structs.ServiceName{Name: "DownStream"},
	}

	return map[string]indexerTestCase{
		indexID: {
			read: indexValue{
				source: []interface{}{
					structs.ServiceName{Name: "UpStReAm"},
					structs.ServiceName{Name: "DownStream"},
				},
				expected: []byte("upstream\x00downstream\x00"),
			},
			write: indexValue{
				source:   obj,
				expected: []byte("upstream\x00downstream\x00"),
			},
		},
		indexUpstream: {
			read: indexValue{
				source: structs.ServiceName{Name: "UpStReAm"},

				expected: []byte("upstream\x00"),
			},
			write: indexValue{
				source:   obj,
				expected: []byte("upstream\x00"),
			},
		},
		indexDownstream: {
			read: indexValue{
				source: structs.ServiceName{Name: "DownStream"},

				expected: []byte("downstream\x00"),
			},
			write: indexValue{
				source:   obj,
				expected: []byte("downstream\x00"),
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

func testIndexerTableServices() map[string]indexerTestCase {
	return map[string]indexerTestCase{
		indexNode: {
			read: indexValue{
				source: Query{
					Value: "NoDe",
				},
				expected: []byte("node\x00"),
			},
			write: indexValue{
				source: &structs.ServiceNode{
					Node:      "NoDe",
					ServiceID: "SeRvIcE",
				},
				expected: []byte("node\x00"),
			},
		},
	}
}
