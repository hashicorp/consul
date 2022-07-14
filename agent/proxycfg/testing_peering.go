package proxycfg

import (
	"github.com/mitchellh/go-testing-interface"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbpeering"
)

func TestConfigSnapshotPeering(t testing.T) *ConfigSnapshot {
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
	})
}

func TestConfigSnapshotPeeringTProxy(t testing.T) *ConfigSnapshot {
	var (
		noEndpointsUpstream = structs.Upstream{
			DestinationName: "no-endpoints",
			DestinationPeer: "peer-a",
		}
		noEndpoints = structs.PeeredServiceName{
			ServiceName: structs.NewServiceName("noEndpoints", nil),
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
				Entry: &structs.MeshConfigEntry{
					TransparentProxy: structs.TransparentProxyMeshConfig{
						MeshDestinationsOnly: true,
					},
				},
			},
		},
		{
			CorrelationID: peeredUpstreamsID,
			Result: &structs.IndexedPeeredServiceList{
				Services: []structs.PeeredServiceName{
					apiA,
					noEndpoints,
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
							Connect: structs.ServiceConnect{
								PeerMeta: &structs.PeeringServiceMeta{
									SNI: []string{
										"apia-1.default.default.cloud.external." + peerTrustDomain,
									},
									SpiffeID: []string{
										"spiffe://" + peerTrustDomain + "/ns/default/dc/cloud-dc/svc/peer-a",
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
