// +build !consulent

package state

import (
	"github.com/hashicorp/consul/agent/structs"
)

func testIndexerTableChecks() map[string]indexerTestCase {
	obj := &structs.HealthCheck{
		Node:        "NoDe",
		ServiceID:   "SeRvIcE",
		ServiceName: "ServiceName",
		CheckID:     "CheckID",
		Status:      "PASSING",
	}
	return map[string]indexerTestCase{
		indexID: {
			read: indexValue{
				source: NodeCheckQuery{
					Node:    "NoDe",
					CheckID: "CheckId",
				},
				expected: []byte("node\x00checkid\x00"),
			},
			write: indexValue{
				source:   obj,
				expected: []byte("node\x00checkid\x00"),
			},
			prefix: []indexValue{
				{
					source:   structs.EnterpriseMeta{},
					expected: nil,
				},
				{
					source:   Query{Value: "nOdE"},
					expected: []byte("node\x00"),
				},
			},
		},
		indexStatus: {
			read: indexValue{
				source:   Query{Value: "PASSING"},
				expected: []byte("passing\x00"),
			},
			write: indexValue{
				source:   obj,
				expected: []byte("passing\x00"),
			},
		},
		indexService: {
			read: indexValue{
				source:   Query{Value: "ServiceName"},
				expected: []byte("servicename\x00"),
			},
			write: indexValue{
				source:   obj,
				expected: []byte("servicename\x00"),
			},
		},
		indexNodeService: {
			read: indexValue{
				source: NodeServiceQuery{
					Node:    "NoDe",
					Service: "SeRvIcE",
				},
				expected: []byte("node\x00service\x00"),
			},
			write: indexValue{
				source:   obj,
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
				source:   obj,
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
				source:   structs.ServiceName{Name: "UpStReAm"},
				expected: []byte("upstream\x00"),
			},
			write: indexValue{
				source:   obj,
				expected: []byte("upstream\x00"),
			},
		},
		indexDownstream: {
			read: indexValue{
				source:   structs.ServiceName{Name: "DownStream"},
				expected: []byte("downstream\x00"),
			},
			write: indexValue{
				source:   obj,
				expected: []byte("downstream\x00"),
			},
		},
	}
}

func testIndexerTableGatewayServices() map[string]indexerTestCase {
	obj := &structs.GatewayService{
		Gateway: structs.ServiceName{Name: "GateWay"},
		Service: structs.ServiceName{Name: "SerVice"},
		Port:    50123,
	}
	encodedPort := string([]byte{0x96, 0x8f, 0x06, 0, 0, 0, 0, 0, 0, 0})
	return map[string]indexerTestCase{
		indexID: {
			read: indexValue{
				source: []interface{}{
					structs.ServiceName{Name: "GateWay"},
					structs.ServiceName{Name: "SerVice"},
					50123,
				},
				expected: []byte("gateway\x00service\x00" + encodedPort),
			},
			write: indexValue{
				source:   obj,
				expected: []byte("gateway\x00service\x00" + encodedPort),
			},
		},
		indexGateway: {
			read: indexValue{
				source:   structs.ServiceName{Name: "GateWay"},
				expected: []byte("gateway\x00"),
			},
			write: indexValue{
				source:   obj,
				expected: []byte("gateway\x00"),
			},
		},
		indexService: {
			read: indexValue{
				source:   structs.ServiceName{Name: "SerVice"},
				expected: []byte("service\x00"),
			},
			write: indexValue{
				source:   obj,
				expected: []byte("service\x00"),
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
			prefix: []indexValue{
				{
					source:   (*structs.EnterpriseMeta)(nil),
					expected: nil,
				},
				{
					source:   structs.EnterpriseMeta{},
					expected: nil,
				},
				{
					source:   Query{Value: "NoDeId"},
					expected: []byte("nodeid\x00"),
				},
			},
		},
		// TODO: uuid
		// TODO: meta

		// TODO(partitions): fix schema tests for tables that reference nodes too
	}
}

func testIndexerTableServices() map[string]indexerTestCase {
	obj := &structs.ServiceNode{
		Node:        "NoDeId",
		ServiceID:   "SeRviCe",
		ServiceName: "ServiceName",
	}

	return map[string]indexerTestCase{
		indexID: {
			read: indexValue{
				source: NodeServiceQuery{
					Node:    "NoDeId",
					Service: "SeRvIcE",
				},
				expected: []byte("nodeid\x00service\x00"),
			},
			write: indexValue{
				source:   obj,
				expected: []byte("nodeid\x00service\x00"),
			},
			prefix: []indexValue{
				{
					source:   (*structs.EnterpriseMeta)(nil),
					expected: nil,
				},
				{
					source:   structs.EnterpriseMeta{},
					expected: nil,
				},
				{
					source:   Query{Value: "NoDeId"},
					expected: []byte("nodeid\x00"),
				},
			},
		},
		indexNode: {
			read: indexValue{
				source: Query{
					Value: "NoDeId",
				},
				expected: []byte("nodeid\x00"),
			},
			write: indexValue{
				source:   obj,
				expected: []byte("nodeid\x00"),
			},
		},
		indexService: {
			read: indexValue{
				source:   Query{Value: "ServiceName"},
				expected: []byte("servicename\x00"),
			},
			write: indexValue{
				source:   obj,
				expected: []byte("servicename\x00"),
			},
		},
		indexConnect: {
			read: indexValue{
				source:   Query{Value: "ConnectName"},
				expected: []byte("connectname\x00"),
			},
			write: indexValue{
				source: &structs.ServiceNode{
					ServiceName:    "ConnectName",
					ServiceConnect: structs.ServiceConnect{Native: true},
				},
				expected: []byte("connectname\x00"),
			},
			extra: []indexerTestCase{
				{
					write: indexValue{
						source: &structs.ServiceNode{
							ServiceName: "ServiceName",
							ServiceKind: structs.ServiceKindConnectProxy,
							ServiceProxy: structs.ConnectProxyConfig{
								DestinationServiceName: "ConnectName",
							},
						},
						expected: []byte("connectname\x00"),
					},
				},
				{
					write: indexValue{
						source:               &structs.ServiceNode{ServiceName: "ServiceName"},
						expectedIndexMissing: true,
					},
				},
				{
					write: indexValue{
						source: &structs.ServiceNode{
							ServiceName: "ServiceName",
							ServiceKind: structs.ServiceKindTerminatingGateway,
						},
						expectedIndexMissing: true,
					},
				},
			},
		},
		indexKind: {
			read: indexValue{
				source:   Query{Value: "connect-proxy"},
				expected: []byte("connect-proxy\x00"),
			},
			write: indexValue{
				source: &structs.ServiceNode{
					ServiceKind: structs.ServiceKindConnectProxy,
				},
				expected: []byte("connect-proxy\x00"),
			},
			extra: []indexerTestCase{
				{
					write: indexValue{
						source: &structs.ServiceNode{
							ServiceName: "ServiceName",
							ServiceKind: structs.ServiceKindTypical,
						},
						expected: []byte("\x00"),
					},
				},
			},
		},
	}
}
