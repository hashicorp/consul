// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package proxycfg

import (
	"bytes"
	"text/template"

	"github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/private/pbpeering"
)

func TestConfigSnapshotPeering(t testing.T) *ConfigSnapshot {
	return testConfigSnapshot(t, false, false)
}

func TestConfigSnapshotPeeringWithEscapeOverrides(t testing.T) *ConfigSnapshot {
	return testConfigSnapshot(t, true, false)
}

func TestConfigSnapshotPeeringWithHTTP2(t testing.T) *ConfigSnapshot {
	return testConfigSnapshot(t, false, true)
}

func testConfigSnapshot(t testing.T, escapeOverride bool, useHTTP2 bool) *ConfigSnapshot {
	var (
		paymentsUpstream = structs.Upstream{
			DestinationName: "payments",
			DestinationPeer: "cloud",
			LocalBindPort:   9090,
		}
		paymentsUID = NewUpstreamID(&paymentsUpstream)

		refundsUpstream = structs.Upstream{
			DestinationName: "refunds",
			DestinationPeer: "cloud",
			LocalBindPort:   9090,
		}
		refundsUID = NewUpstreamID(&refundsUpstream)
	)

	protocol := "tcp"
	if useHTTP2 {
		protocol = "http2"
	}

	const peerTrustDomain = "1c053652-8512-4373-90cf-5a7f6263a994.consul"

	return TestConfigSnapshot(t, func(ns *structs.NodeService) {
		ns.Proxy.Upstreams = structs.Upstreams{
			paymentsUpstream,
			refundsUpstream,
		}

		if escapeOverride {
			if ns.Proxy.Upstreams[0].Config == nil {
				ns.Proxy.Upstreams[0].Config = map[string]interface{}{}
			}

			uid := NewUpstreamID(&ns.Proxy.Upstreams[0])

			ns.Proxy.Upstreams[0].Config["envoy_listener_json"] =
				customListenerJSON(t, customListenerJSONOptions{
					Name: uid.EnvoyID() + ":custom-upstream",
				})
			ns.Proxy.Upstreams[0].Config["envoy_cluster_json"] =
				customClusterJSON(t, customClusterJSONOptions{
					Name: uid.EnvoyID() + ":custom-upstream",
				})
		}

	}, []UpdateEvent{
		{
			CorrelationID: peerTrustBundleIDPrefix + "cloud",
			Result: &pbpeering.TrustBundleReadResponse{
				Bundle: TestPeerTrustBundles(t).Bundles[0],
			},
		},
		{
			CorrelationID: upstreamPeerWatchIDPrefix + paymentsUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: []structs.CheckServiceNode{
					{
						Node: &structs.Node{
							Address:    "85.252.102.31",
							Datacenter: "cloud-dc",
						},
						Service: &structs.NodeService{
							Service: "payments-sidecar-proxy",
							Kind:    structs.ServiceKindConnectProxy,
							Port:    443,
							TaggedAddresses: map[string]structs.ServiceAddress{
								structs.TaggedAddressLAN: {
									Address: "85.252.102.31",
									Port:    443,
								},
								structs.TaggedAddressWAN: {
									Address: "123.us-east-1.elb.notaws.com",
									Port:    8443,
								},
							},
							Connect: structs.ServiceConnect{
								PeerMeta: &structs.PeeringServiceMeta{
									SNI: []string{
										"payments.default.default.cloud.external." + peerTrustDomain,
									},
									SpiffeID: []string{
										"spiffe://" + peerTrustDomain + "/ns/default/dc/cloud-dc/svc/payments",
									},
									Protocol: protocol,
								},
							},
						},
					},
				},
			},
		},
		{
			CorrelationID: upstreamPeerWatchIDPrefix + refundsUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: []structs.CheckServiceNode{
					{
						Node: &structs.Node{
							Address:    "106.96.90.233",
							Datacenter: "cloud-dc",
						},
						Service: &structs.NodeService{
							Service: "refunds-sidecar-proxy",
							Kind:    structs.ServiceKindConnectProxy,
							Port:    443,
							Connect: structs.ServiceConnect{
								PeerMeta: &structs.PeeringServiceMeta{
									SNI: []string{
										"refunds.default.default.cloud.external." + peerTrustDomain,
									},
									SpiffeID: []string{
										"spiffe://" + peerTrustDomain + "/ns/default/dc/cloud-dc/svc/refunds",
									},
									Protocol: protocol,
								},
							},
						},
					},
				},
			},
		},
	})
}

func TestConfigSnapshotPeeringTProxy(t testing.T) *ConfigSnapshot {
	// Test two explicitly defined upstreams api-a and noEndpoints
	// as well as one implicitly inferred upstream db.

	var (
		noEndpointsUpstream = structs.Upstream{
			DestinationName: "no-endpoints",
			DestinationPeer: "peer-a",
			LocalBindPort:   1234,
		}
		noEndpoints = structs.PeeredServiceName{
			ServiceName: structs.NewServiceName("no-endpoints", nil),
			Peer:        "peer-a",
		}

		apiAUpstream = structs.Upstream{
			DestinationName: "api-a",
			DestinationPeer: "peer-a",
			LocalBindPort:   9090,
		}
		apiA = structs.PeeredServiceName{
			ServiceName: structs.NewServiceName("api-a", nil),
			Peer:        "peer-a",
		}

		db = structs.PeeredServiceName{
			ServiceName: structs.NewServiceName("db", nil),
			Peer:        "peer-a",
		}
	)

	const peerTrustDomain = "1c053652-8512-4373-90cf-5a7f6263a994.consul"

	return TestConfigSnapshot(t, func(ns *structs.NodeService) {
		ns.Proxy.Mode = structs.ProxyModeTransparent
		ns.Proxy.Upstreams = []structs.Upstream{
			noEndpointsUpstream,
			apiAUpstream,
		}
	}, []UpdateEvent{
		{
			CorrelationID: meshConfigEntryID,
			Result: &structs.ConfigEntryResponse{
				Entry: nil,
			},
		},
		{
			CorrelationID: peeredUpstreamsID,
			Result: &structs.IndexedPeeredServiceList{
				Services: []structs.PeeredServiceName{
					apiA,
					noEndpoints,
					db, // implicitly added here
				},
			},
		},
		{
			CorrelationID: peerTrustBundleIDPrefix + "peer-a",
			Result: &pbpeering.TrustBundleReadResponse{
				Bundle: TestPeerTrustBundles(t).Bundles[0],
			},
		},
		{
			CorrelationID: upstreamPeerWatchIDPrefix + NewUpstreamID(&noEndpointsUpstream).String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: []structs.CheckServiceNode{},
			},
		},
		{
			CorrelationID: upstreamPeerWatchIDPrefix + NewUpstreamID(&apiAUpstream).String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: structs.CheckServiceNodes{
					{
						Node: &structs.Node{
							Node:     "node1",
							Address:  "127.0.0.1",
							PeerName: "peer-a",
						},
						Service: &structs.NodeService{
							ID:       "api-a-1",
							Service:  "api-a",
							PeerName: "peer-a",
							Address:  "1.2.3.4",
							TaggedAddresses: map[string]structs.ServiceAddress{
								"virtual":                      {Address: "10.0.0.1"},
								structs.TaggedAddressVirtualIP: {Address: "240.0.0.1"},
							},
							Connect: structs.ServiceConnect{
								PeerMeta: &structs.PeeringServiceMeta{
									SNI: []string{
										"api-a.default.default.cloud.external." + peerTrustDomain,
									},
									SpiffeID: []string{
										"spiffe://" + peerTrustDomain + "/ns/default/dc/cloud-dc/svc/api-a",
									},
									Protocol: "tcp",
								},
							},
						},
					},
				},
			},
		},
		{
			CorrelationID: upstreamPeerWatchIDPrefix + NewUpstreamIDFromPeeredServiceName(db).String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: structs.CheckServiceNodes{
					{
						Node: &structs.Node{
							Node:     "node1",
							Address:  "127.0.0.1",
							PeerName: "peer-a",
						},
						Service: &structs.NodeService{
							ID:       "db-1",
							Service:  "db",
							PeerName: "peer-a",
							Address:  "2.3.4.5", // Expect no endpoint or listener for this address
							TaggedAddresses: map[string]structs.ServiceAddress{
								"virtual":                      {Address: "10.0.0.2"},
								structs.TaggedAddressVirtualIP: {Address: "240.0.0.2"},
							},
							Connect: structs.ServiceConnect{
								PeerMeta: &structs.PeeringServiceMeta{
									SNI: []string{
										"db.default.default.cloud.external." + peerTrustDomain,
									},
									SpiffeID: []string{
										"spiffe://" + peerTrustDomain + "/ns/default/dc/cloud-dc/svc/db",
									},
									Protocol: "tcp",
								},
							},
						},
					},
				},
			},
		},
	})
}

func TestConfigSnapshotPeeringLocalMeshGateway(t testing.T) *ConfigSnapshot {
	var (
		paymentsUpstream = structs.Upstream{
			DestinationName: "payments",
			DestinationPeer: "cloud",
			LocalBindPort:   9090,
			MeshGateway:     structs.MeshGatewayConfig{Mode: structs.MeshGatewayModeRemote},
		}
		paymentsUID = NewUpstreamID(&paymentsUpstream)

		refundsUpstream = structs.Upstream{
			DestinationName: "refunds",
			DestinationPeer: "cloud",
			LocalBindPort:   9090,
			MeshGateway:     structs.MeshGatewayConfig{Mode: structs.MeshGatewayModeLocal},
		}
		refundsUID = NewUpstreamID(&refundsUpstream)
	)

	const peerTrustDomain = "1c053652-8512-4373-90cf-5a7f6263a994.consul"

	return TestConfigSnapshot(t, func(ns *structs.NodeService) {
		ns.Proxy.Upstreams = structs.Upstreams{
			paymentsUpstream,
			refundsUpstream,
		}
	}, []UpdateEvent{
		{
			CorrelationID: peerTrustBundleIDPrefix + "cloud",
			Result: &pbpeering.TrustBundleReadResponse{
				Bundle: TestPeerTrustBundles(t).Bundles[0],
			},
		},
		{
			CorrelationID: upstreamPeerWatchIDPrefix + paymentsUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: []structs.CheckServiceNode{
					{
						Node: &structs.Node{
							Address:    "85.252.102.31",
							Datacenter: "cloud-dc",
						},
						Service: &structs.NodeService{
							Service: "payments-sidecar-proxy",
							Kind:    structs.ServiceKindConnectProxy,
							Port:    443,
							TaggedAddresses: map[string]structs.ServiceAddress{
								structs.TaggedAddressLAN: {
									Address: "85.252.102.31",
									Port:    443,
								},
								structs.TaggedAddressWAN: {
									Address: "123.us-east-1.elb.notaws.com",
									Port:    8443,
								},
							},
							Connect: structs.ServiceConnect{
								PeerMeta: &structs.PeeringServiceMeta{
									SNI: []string{
										"payments.default.default.cloud.external." + peerTrustDomain,
									},
									SpiffeID: []string{
										"spiffe://" + peerTrustDomain + "/ns/default/dc/cloud-dc/svc/payments",
									},
									Protocol: "tcp",
								},
							},
						},
					},
				},
			},
		},
		{
			CorrelationID: upstreamPeerWatchIDPrefix + refundsUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: []structs.CheckServiceNode{
					{
						Node: &structs.Node{
							Address:    "106.96.90.233",
							Datacenter: "cloud-dc",
						},
						Service: &structs.NodeService{
							Service: "refunds-sidecar-proxy",
							Kind:    structs.ServiceKindConnectProxy,
							Port:    443,
							Connect: structs.ServiceConnect{
								PeerMeta: &structs.PeeringServiceMeta{
									SNI: []string{
										"refunds.default.default.cloud.external." + peerTrustDomain,
									},
									SpiffeID: []string{
										"spiffe://" + peerTrustDomain + "/ns/default/dc/cloud-dc/svc/refunds",
									},
									Protocol: "tcp",
								},
							},
						},
					},
				},
			},
		},
		{
			CorrelationID: "mesh-gateway:dc1",
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: structs.CheckServiceNodes{
					structs.CheckServiceNode{
						Node: &structs.Node{
							ID:         "mesh-gateway",
							Node:       "mesh-gateway",
							Address:    "10.0.0.1",
							Datacenter: "dc1",
						},
						Service: &structs.NodeService{
							Kind:    structs.ServiceKindMeshGateway,
							Service: "mesh-gateway",
							Port:    1234,
							TaggedAddresses: map[string]structs.ServiceAddress{
								structs.TaggedAddressWAN: {Address: "172.100.0.14", Port: 8080},
							},
							EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
						},
					},
				},
			},
		},
	})
}

var (
	customListenerJSONTemplate = template.Must(template.New("").Parse(customListenerJSONTpl))
)

func customListenerJSON(t testing.T, opts customListenerJSONOptions) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, customListenerJSONTemplate.Execute(&buf, opts))
	return buf.String()
}

type customListenerJSONOptions struct {
	Name       string
	TLSContext string
}

const customListenerJSONTpl = `{
	"@type": "type.googleapis.com/envoy.config.listener.v3.Listener",
	"name": "{{ .Name }}",
	"address": {
		"socketAddress": {
			"address": "11.11.11.11",
			"portValue": 11111
		}
	},
	"filterChains": [
		{
			{{ if .TLSContext -}}
			"transport_socket": {
				"name": "tls",
				"typed_config": {
					"@type": "type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.UpstreamTlsContext",
					{{ .TLSContext }}
				}
			},
			{{- end }}
			"filters": [
				{
					"name": "envoy.filters.network.tcp_proxy",
					"typedConfig": {
						"@type": "type.googleapis.com/envoy.extensions.filters.network.tcp_proxy.v3.TcpProxy",
							"cluster": "random-cluster",
							"statPrefix": "foo-stats"
						}
				}
			]
		}
	]
}`

type customClusterJSONOptions struct {
	Name       string
	TLSContext string
}

var customClusterJSONTpl = `{
	"@type": "type.googleapis.com/envoy.config.cluster.v3.Cluster",
	"name": "{{ .Name }}",
	"connectTimeout": "15s",
	"loadAssignment": {
		"clusterName": "{{ .Name }}",
		"endpoints": [
			{
				"lbEndpoints": [
					{
						"endpoint": {
							"address": {
								"socketAddress": {
									"address": "1.2.3.4",
									"portValue": 8443
								}
							}
						}
					}
				]
			}
		]
	}
}`

var customClusterJSONTemplate = template.Must(template.New("").Parse(customClusterJSONTpl))

func customClusterJSON(t testing.T, opts customClusterJSONOptions) string {
	t.Helper()
	var buf bytes.Buffer
	err := customClusterJSONTemplate.Execute(&buf, opts)
	require.NoError(t, err)
	return buf.String()
}
