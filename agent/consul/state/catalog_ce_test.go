//go:build !consulent
// +build !consulent

package state

import (
	"net"
	"strconv"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/types"
)

func testIndexerTableChecks() map[string]indexerTestCase {
	obj := &structs.HealthCheck{
		Node:        "NoDe",
		ServiceID:   "SeRvIcE",
		ServiceName: "ServiceName",
		CheckID:     "CheckID",
		Status:      "PASSING",
	}
	objWPeer := &structs.HealthCheck{
		Node:        "NoDe",
		ServiceID:   "SeRvIcE",
		ServiceName: "ServiceName",
		CheckID:     "CheckID",
		Status:      "PASSING",
		PeerName:    "Peer1",
	}
	return map[string]indexerTestCase{
		indexID: {
			read: indexValue{
				source: NodeCheckQuery{
					Node:    "NoDe",
					CheckID: "CheckId",
				},
				expected: []byte("~\x00node\x00checkid\x00"),
			},
			write: indexValue{
				source:   obj,
				expected: []byte("~\x00node\x00checkid\x00"),
			},
			prefix: []indexValue{
				{
					source:   acl.EnterpriseMeta{},
					expected: nil,
				},
				{
					source:   Query{Value: "nOdE"},
					expected: []byte("~\x00node\x00"),
				},
			},
			extra: []indexerTestCase{
				{
					read: indexValue{
						source: NodeCheckQuery{
							Node:     "NoDe",
							CheckID:  "CheckId",
							PeerName: "Peer1",
						},
						expected: []byte("peer1\x00node\x00checkid\x00"),
					},
					write: indexValue{
						source:   objWPeer,
						expected: []byte("peer1\x00node\x00checkid\x00"),
					},
					prefix: []indexValue{
						{
							source: Query{Value: "nOdE",
								PeerName: "Peer1"},
							expected: []byte("peer1\x00node\x00"),
						},
					},
				},
			},
		},
		indexStatus: {
			read: indexValue{
				source:   Query{Value: "PASSING"},
				expected: []byte("~\x00passing\x00"),
			},
			write: indexValue{
				source:   obj,
				expected: []byte("~\x00passing\x00"),
			},
			extra: []indexerTestCase{
				{
					read: indexValue{
						source:   Query{Value: "PASSING", PeerName: "Peer1"},
						expected: []byte("peer1\x00passing\x00"),
					},
					write: indexValue{
						source:   objWPeer,
						expected: []byte("peer1\x00passing\x00"),
					},
				},
			},
		},
		indexService: {
			read: indexValue{
				source:   Query{Value: "ServiceName"},
				expected: []byte("~\x00servicename\x00"),
			},
			write: indexValue{
				source:   obj,
				expected: []byte("~\x00servicename\x00"),
			},
			extra: []indexerTestCase{
				{
					read: indexValue{
						source:   Query{Value: "ServiceName", PeerName: "Peer1"},
						expected: []byte("peer1\x00servicename\x00"),
					},
					write: indexValue{
						source:   objWPeer,
						expected: []byte("peer1\x00servicename\x00"),
					},
				},
			},
		},
		indexNodeService: {
			read: indexValue{
				source: NodeServiceQuery{
					Node:    "NoDe",
					Service: "SeRvIcE",
				},
				expected: []byte("~\x00node\x00service\x00"),
			},
			write: indexValue{
				source:   obj,
				expected: []byte("~\x00node\x00service\x00"),
			},
			extra: []indexerTestCase{
				{
					read: indexValue{
						source: NodeServiceQuery{
							Node:     "NoDe",
							PeerName: "Peer1",
							Service:  "SeRvIcE",
						},
						expected: []byte("peer1\x00node\x00service\x00"),
					},
					write: indexValue{
						source:   objWPeer,
						expected: []byte("peer1\x00node\x00service\x00"),
					},
				},
			},
		},
		indexNode: {
			read: indexValue{
				source: Query{
					Value: "NoDe",
				},
				expected: []byte("~\x00node\x00"),
			},
			write: indexValue{
				source:   obj,
				expected: []byte("~\x00node\x00"),
			},
			extra: []indexerTestCase{
				{
					read: indexValue{
						source: Query{
							Value:    "NoDe",
							PeerName: "Peer1",
						},
						expected: []byte("peer1\x00node\x00"),
					},
					write: indexValue{
						source:   objWPeer,
						expected: []byte("peer1\x00node\x00"),
					},
				},
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
	encodedPort := string([]byte{0x80, 0, 0, 0, 0, 0, 0xc3, 0xcb})
	// On 32-bit systems the int encoding will be different
	if strconv.IntSize == 32 {
		encodedPort = string([]byte{0x80, 0, 0xc3, 0xcb})
	}
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
	uuidBuf, uuid := generateUUID()

	return map[string]indexerTestCase{
		indexID: {
			read: indexValue{
				source:   Query{Value: "NoDeId"},
				expected: []byte("~\x00nodeid\x00"),
			},
			write: indexValue{
				source:   &structs.Node{Node: "NoDeId"},
				expected: []byte("~\x00nodeid\x00"),
			},
			prefix: []indexValue{
				{
					source:   (*acl.EnterpriseMeta)(nil),
					expected: nil,
				},
				{
					source:   acl.EnterpriseMeta{},
					expected: nil,
				},
				{
					source:   Query{Value: "NoDeId"},
					expected: []byte("~\x00nodeid\x00"),
				},
				{
					source:   Query{},
					expected: []byte("~\x00"),
				},
			},
			extra: []indexerTestCase{
				{
					read: indexValue{
						source:   Query{Value: "NoDeId", PeerName: "Peer1"},
						expected: []byte("peer1\x00nodeid\x00"),
					},
					write: indexValue{
						source:   &structs.Node{Node: "NoDeId", PeerName: "Peer1"},
						expected: []byte("peer1\x00nodeid\x00"),
					},
					prefix: []indexValue{
						{
							source:   Query{PeerName: "Peer1"},
							expected: []byte("peer1\x00"),
						},
						{
							source:   Query{Value: "NoDeId", PeerName: "Peer1"},
							expected: []byte("peer1\x00nodeid\x00"),
						},
					},
				},
			},
		},
		indexUUID: {
			read: indexValue{
				source:   Query{Value: uuid},
				expected: append([]byte("~\x00"), uuidBuf...),
			},
			write: indexValue{
				source: &structs.Node{
					ID:   types.NodeID(uuid),
					Node: "NoDeId",
				},
				expected: append([]byte("~\x00"), uuidBuf...),
			},
			prefix: []indexValue{
				{ // partial length
					source:   Query{Value: uuid[:6]},
					expected: append([]byte("~\x00"), uuidBuf[:3]...),
				},
				{ // full length
					source:   Query{Value: uuid},
					expected: append([]byte("~\x00"), uuidBuf...),
				},
				{
					source:   Query{},
					expected: []byte("~\x00"),
				},
			},
			extra: []indexerTestCase{
				{
					read: indexValue{
						source:   Query{Value: uuid, PeerName: "Peer1"},
						expected: append([]byte("peer1\x00"), uuidBuf...),
					},
					write: indexValue{
						source: &structs.Node{
							ID:       types.NodeID(uuid),
							PeerName: "Peer1",
							Node:     "NoDeId",
						},
						expected: append([]byte("peer1\x00"), uuidBuf...),
					},
					prefix: []indexValue{
						{ // partial length
							source:   Query{Value: uuid[:6], PeerName: "Peer1"},
							expected: append([]byte("peer1\x00"), uuidBuf[:3]...),
						},
						{ // full length
							source:   Query{Value: uuid, PeerName: "Peer1"},
							expected: append([]byte("peer1\x00"), uuidBuf...),
						},
						{
							source:   Query{PeerName: "Peer1"},
							expected: []byte("peer1\x00"),
						},
					},
				},
			},
		},
		indexMeta: {
			read: indexValue{
				source: KeyValueQuery{
					Key:   "KeY",
					Value: "VaLuE",
				},
				expected: []byte("~\x00KeY\x00VaLuE\x00"),
			},
			writeMulti: indexValueMulti{
				source: &structs.Node{
					Node: "NoDeId",
					Meta: map[string]string{
						"MaP-kEy-1": "mAp-VaL-1",
						"mAp-KeY-2": "MaP-vAl-2",
					},
				},
				expected: [][]byte{
					[]byte("~\x00MaP-kEy-1\x00mAp-VaL-1\x00"),
					[]byte("~\x00mAp-KeY-2\x00MaP-vAl-2\x00"),
				},
			},
			extra: []indexerTestCase{
				{
					read: indexValue{
						source: KeyValueQuery{
							Key:      "KeY",
							Value:    "VaLuE",
							PeerName: "Peer1",
						},
						expected: []byte("peer1\x00KeY\x00VaLuE\x00"),
					},
					writeMulti: indexValueMulti{
						source: &structs.Node{
							Node: "NoDeId",
							Meta: map[string]string{
								"MaP-kEy-1": "mAp-VaL-1",
								"mAp-KeY-2": "MaP-vAl-2",
							},
							PeerName: "Peer1",
						},
						expected: [][]byte{
							[]byte("peer1\x00MaP-kEy-1\x00mAp-VaL-1\x00"),
							[]byte("peer1\x00mAp-KeY-2\x00MaP-vAl-2\x00"),
						},
					},
				},
			},
		},

		// TODO(partitions): fix schema tests for tables that reference nodes too
	}
}

func testIndexerTableServices() map[string]indexerTestCase {
	obj := &structs.ServiceNode{
		Node:        "NoDeId",
		ServiceID:   "SeRviCe",
		ServiceName: "ServiceName",
	}
	objWPeer := &structs.ServiceNode{
		Node:        "NoDeId",
		ServiceID:   "SeRviCe",
		ServiceName: "ServiceName",
		PeerName:    "Peer1",
	}

	return map[string]indexerTestCase{
		indexID: {
			read: indexValue{
				source: NodeServiceQuery{
					Node:    "NoDeId",
					Service: "SeRvIcE",
				},
				expected: []byte("~\x00nodeid\x00service\x00"),
			},
			write: indexValue{
				source:   obj,
				expected: []byte("~\x00nodeid\x00service\x00"),
			},
			prefix: []indexValue{
				{
					source:   (*acl.EnterpriseMeta)(nil),
					expected: nil,
				},
				{
					source:   acl.EnterpriseMeta{},
					expected: nil,
				},
				{
					source:   Query{},
					expected: []byte("~\x00"),
				},
				{
					source:   Query{Value: "NoDeId"},
					expected: []byte("~\x00nodeid\x00"),
				},
			},
			extra: []indexerTestCase{
				{
					read: indexValue{
						source: NodeServiceQuery{
							Node:     "NoDeId",
							PeerName: "Peer1",
							Service:  "SeRvIcE",
						},
						expected: []byte("peer1\x00nodeid\x00service\x00"),
					},
					write: indexValue{
						source:   objWPeer,
						expected: []byte("peer1\x00nodeid\x00service\x00"),
					},
					prefix: []indexValue{
						{
							source:   Query{Value: "NoDeId", PeerName: "Peer1"},
							expected: []byte("peer1\x00nodeid\x00"),
						},
						{
							source:   Query{PeerName: "Peer1"},
							expected: []byte("peer1\x00"),
						},
					},
				},
			},
		},
		indexNode: {
			read: indexValue{
				source: Query{
					Value: "NoDeId",
				},
				expected: []byte("~\x00nodeid\x00"),
			},
			write: indexValue{
				source:   obj,
				expected: []byte("~\x00nodeid\x00"),
			},
			extra: []indexerTestCase{
				{
					read: indexValue{
						source: Query{
							Value:    "NoDeId",
							PeerName: "Peer1",
						},
						expected: []byte("peer1\x00nodeid\x00"),
					},
					write: indexValue{
						source:   objWPeer,
						expected: []byte("peer1\x00nodeid\x00"),
					},
				},
			},
		},
		indexService: {
			read: indexValue{
				source:   Query{Value: "ServiceName"},
				expected: []byte("~\x00servicename\x00"),
			},
			write: indexValue{
				source:   obj,
				expected: []byte("~\x00servicename\x00"),
			},
			extra: []indexerTestCase{
				{
					read: indexValue{
						source:   Query{Value: "ServiceName", PeerName: "Peer1"},
						expected: []byte("peer1\x00servicename\x00"),
					},
					write: indexValue{
						source:   objWPeer,
						expected: []byte("peer1\x00servicename\x00"),
					},
				},
			},
		},
		indexConnect: {
			read: indexValue{
				source:   Query{Value: "ConnectName"},
				expected: []byte("~\x00connectname\x00"),
			},
			write: indexValue{
				source: &structs.ServiceNode{
					ServiceName:    "ConnectName",
					ServiceConnect: structs.ServiceConnect{Native: true},
				},
				expected: []byte("~\x00connectname\x00"),
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
						expected: []byte("~\x00connectname\x00"),
					},
				},
				{
					write: indexValue{
						source: &structs.ServiceNode{
							ServiceName: "ServiceName",
							ServiceKind: structs.ServiceKindConnectProxy,
							ServiceProxy: structs.ConnectProxyConfig{
								DestinationServiceName: "ConnectName",
							},
							PeerName: "Peer1",
						},
						expected: []byte("peer1\x00connectname\x00"),
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
				{
					read: indexValue{
						source:   Query{Value: "ConnectName", PeerName: "Peer1"},
						expected: []byte("peer1\x00connectname\x00"),
					},
					write: indexValue{
						source: &structs.ServiceNode{
							ServiceName:    "ConnectName",
							ServiceConnect: structs.ServiceConnect{Native: true},
							PeerName:       "Peer1",
						},
						expected: []byte("peer1\x00connectname\x00"),
					},
				},
			},
		},
		indexKind: {
			read: indexValue{
				source:   Query{Value: "connect-proxy"},
				expected: []byte("~\x00connect-proxy\x00"),
			},
			write: indexValue{
				source: &structs.ServiceNode{
					ServiceKind: structs.ServiceKindConnectProxy,
				},
				expected: []byte("~\x00connect-proxy\x00"),
			},
			extra: []indexerTestCase{
				{
					write: indexValue{
						source: &structs.ServiceNode{
							ServiceName: "ServiceName",
							ServiceKind: structs.ServiceKindTypical,
						},
						expected: []byte("~\x00\x00"),
					},
				},
				{
					write: indexValue{
						source: &structs.ServiceNode{
							ServiceName: "ServiceName",
							ServiceKind: structs.ServiceKindTypical,
							PeerName:    "Peer1",
						},
						expected: []byte("peer1\x00\x00"),
					},
				},
				{
					read: indexValue{
						source:   Query{Value: "connect-proxy", PeerName: "Peer1"},
						expected: []byte("peer1\x00connect-proxy\x00"),
					},
					write: indexValue{
						source: &structs.ServiceNode{
							ServiceKind: structs.ServiceKindConnectProxy,
							PeerName:    "Peer1",
						},
						expected: []byte("peer1\x00connect-proxy\x00"),
					},
				},
			},
		},
	}
}

func testIndexerTableServiceVirtualIPs() map[string]indexerTestCase {
	obj := ServiceVirtualIP{
		Service: structs.PeeredServiceName{
			ServiceName: structs.ServiceName{
				Name: "foo",
			},
		},
		IP: net.ParseIP("127.0.0.1"),
	}
	peeredObj := ServiceVirtualIP{
		Service: structs.PeeredServiceName{
			ServiceName: structs.ServiceName{
				Name: "foo",
			},
			Peer: "Billing",
		},
		IP: net.ParseIP("127.0.0.1"),
	}

	return map[string]indexerTestCase{
		indexID: {
			read: indexValue{
				source: structs.PeeredServiceName{
					ServiceName: structs.ServiceName{
						Name: "foo",
					},
				},
				expected: []byte("~\x00foo\x00"),
			},
			write: indexValue{
				source:   obj,
				expected: []byte("~\x00foo\x00"),
			},
			prefix: []indexValue{
				{
					source: Query{
						Value: "foo",
					},
					expected: []byte("~\x00foo\x00"),
				},
				{
					source: Query{
						Value:    "foo",
						PeerName: "*", // test wildcard PeerName
					},
					expected: []byte("peer:"),
				},
			},
			extra: []indexerTestCase{
				{
					read: indexValue{
						source: structs.PeeredServiceName{
							ServiceName: structs.ServiceName{
								Name: "foo",
							},
							Peer: "Billing",
						},
						expected: []byte("peer:billing\x00foo\x00"),
					},
					write: indexValue{
						source:   peeredObj,
						expected: []byte("peer:billing\x00foo\x00"),
					},
				},
			},
		},
	}
}

func testIndexerTableKindServiceNames() map[string]indexerTestCase {
	obj := &KindServiceName{
		Service: structs.ServiceName{
			Name: "web-sidecar-proxy",
		},
		Kind: structs.ServiceKindConnectProxy,
	}

	return map[string]indexerTestCase{
		indexID: {
			read: indexValue{
				source: &KindServiceName{
					Service: structs.ServiceName{
						Name: "web-sidecar-proxy",
					},
					Kind: structs.ServiceKindConnectProxy,
				},
				expected: []byte("connect-proxy\x00web-sidecar-proxy\x00"),
			},
			write: indexValue{
				source:   obj,
				expected: []byte("connect-proxy\x00web-sidecar-proxy\x00"),
			},
		},
		indexKind: {
			read: indexValue{
				source:   Query{Value: string(structs.ServiceKindConnectProxy)},
				expected: []byte("connect-proxy\x00"),
			},
			write: indexValue{
				source:   obj,
				expected: []byte("connect-proxy\x00"),
			},
		},
	}
}
