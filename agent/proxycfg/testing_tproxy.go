package proxycfg

import (
	"time"

	"github.com/hashicorp/consul/api"

	"github.com/mitchellh/go-testing-interface"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/structs"
)

func TestConfigSnapshotTransparentProxy(t testing.T) *ConfigSnapshot {
	// DiscoveryChain without an UpstreamConfig should yield a
	// filter chain when in transparent proxy mode
	var (
		google      = structs.NewServiceName("google", nil)
		googleUID   = NewUpstreamIDFromServiceName(google)
		googleChain = discoverychain.TestCompileConfigEntries(t, "google", "default", "default", "dc1", connect.TestClusterID+".consul", nil)

		noEndpoints      = structs.NewServiceName("no-endpoints", nil)
		noEndpointsUID   = NewUpstreamIDFromServiceName(noEndpoints)
		noEndpointsChain = discoverychain.TestCompileConfigEntries(t, "no-endpoints", "default", "default", "dc1", connect.TestClusterID+".consul", nil)

		db = structs.NewServiceName("db", nil)
	)

	return TestConfigSnapshot(t, func(ns *structs.NodeService) {
		ns.Proxy.Mode = structs.ProxyModeTransparent
	}, []UpdateEvent{
		{
			CorrelationID: meshConfigEntryID,
			Result: &structs.ConfigEntryResponse{
				Entry: nil,
			},
		},
		{
			CorrelationID: intentionUpstreamsID,
			Result: &structs.IndexedServiceList{
				Services: structs.ServiceList{
					google,
					noEndpoints,
					// In transparent proxy mode, watches for
					// upstreams in the local DC are handled by the
					// IntentionUpstreams watch!
					db,
				},
			},
		},
		{
			CorrelationID: "discovery-chain:" + googleUID.String(),
			Result: &structs.DiscoveryChainResponse{
				Chain: googleChain,
			},
		},
		{
			CorrelationID: "discovery-chain:" + noEndpointsUID.String(),
			Result: &structs.DiscoveryChainResponse{
				Chain: noEndpointsChain,
			},
		},
		{
			CorrelationID: "upstream-target:google.default.default.dc1:" + googleUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: []structs.CheckServiceNode{
					{
						Node: &structs.Node{
							Address:    "8.8.8.8",
							Datacenter: "dc1",
						},
						Service: &structs.NodeService{
							Service: "google",
							Address: "9.9.9.9",
							Port:    9090,
							TaggedAddresses: map[string]structs.ServiceAddress{
								"virtual":                      {Address: "10.0.0.1"},
								structs.TaggedAddressVirtualIP: {Address: "240.0.0.1"},
							},
						},
					},
				},
			},
		},
		{
			CorrelationID: "upstream-target:google-v2.default.default.dc1:" + googleUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				// Other targets of the discovery chain should be ignored.
				// We only match on the upstream's virtual IP, not the IPs of other targets.
				Nodes: []structs.CheckServiceNode{
					{
						Node: &structs.Node{
							Address:    "7.7.7.7",
							Datacenter: "dc1",
						},
						Service: &structs.NodeService{
							Service: "google-v2",
							TaggedAddresses: map[string]structs.ServiceAddress{
								"virtual": {Address: "10.10.10.10"},
							},
						},
					},
				},
			},
		},
		{
			CorrelationID: "upstream-target:" + noEndpointsChain.ID() + ":" + noEndpointsUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				// DiscoveryChains without endpoints do not get a
				// filter chain because there are no addresses to
				// match on.
				Nodes: []structs.CheckServiceNode{},
			},
		},
	})
}

func TestConfigSnapshotTransparentProxyHTTPUpstream(t testing.T) *ConfigSnapshot {
	// DiscoveryChain without an UpstreamConfig should yield a
	// filter chain when in transparent proxy mode
	var (
		google      = structs.NewServiceName("google", nil)
		googleUID   = NewUpstreamIDFromServiceName(google)
		googleChain = discoverychain.TestCompileConfigEntries(t, "google", "default", "default", "dc1", connect.TestClusterID+".consul", nil,
			// Set default service protocol to HTTP
			&structs.ProxyConfigEntry{
				Kind: structs.ProxyDefaults,
				Name: structs.ProxyConfigGlobal,
				Config: map[string]interface{}{
					"protocol": "http",
				},
			},
		)

		noEndpoints      = structs.NewServiceName("no-endpoints", nil)
		noEndpointsUID   = NewUpstreamIDFromServiceName(noEndpoints)
		noEndpointsChain = discoverychain.TestCompileConfigEntries(t, "no-endpoints", "default", "default", "dc1", connect.TestClusterID+".consul", nil)

		db = structs.NewServiceName("db", nil)
	)

	return TestConfigSnapshot(t, func(ns *structs.NodeService) {
		ns.Proxy.Mode = structs.ProxyModeTransparent
	}, []UpdateEvent{
		{
			CorrelationID: meshConfigEntryID,
			Result: &structs.ConfigEntryResponse{
				Entry: nil,
			},
		},
		{
			CorrelationID: intentionUpstreamsID,
			Result: &structs.IndexedServiceList{
				Services: structs.ServiceList{
					google,
					noEndpoints,
					// In transparent proxy mode, watches for
					// upstreams in the local DC are handled by the
					// IntentionUpstreams watch!
					db,
				},
			},
		},
		{
			CorrelationID: "discovery-chain:" + googleUID.String(),
			Result: &structs.DiscoveryChainResponse{
				Chain: googleChain,
			},
		},
		{
			CorrelationID: "discovery-chain:" + noEndpointsUID.String(),
			Result: &structs.DiscoveryChainResponse{
				Chain: noEndpointsChain,
			},
		},
		{
			CorrelationID: "upstream-target:google.default.default.dc1:" + googleUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: []structs.CheckServiceNode{
					{
						Node: &structs.Node{
							Address:    "8.8.8.8",
							Datacenter: "dc1",
						},
						Service: &structs.NodeService{
							Service: "google",
							Address: "9.9.9.9",
							Port:    9090,
							TaggedAddresses: map[string]structs.ServiceAddress{
								"virtual":                      {Address: "10.0.0.1"},
								structs.TaggedAddressVirtualIP: {Address: "240.0.0.1"},
							},
						},
					},
				},
			},
		},
		{
			CorrelationID: "upstream-target:google-v2.default.default.dc1:" + googleUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				// Other targets of the discovery chain should be ignored.
				// We only match on the upstream's virtual IP, not the IPs of other targets.
				Nodes: []structs.CheckServiceNode{
					{
						Node: &structs.Node{
							Address:    "7.7.7.7",
							Datacenter: "dc1",
						},
						Service: &structs.NodeService{
							Service: "google-v2",
							TaggedAddresses: map[string]structs.ServiceAddress{
								"virtual": {Address: "10.10.10.10"},
							},
						},
					},
				},
			},
		},
		{
			CorrelationID: "upstream-target:" + noEndpointsChain.ID() + ":" + noEndpointsUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				// DiscoveryChains without endpoints do not get a
				// filter chain because there are no addresses to
				// match on.
				Nodes: []structs.CheckServiceNode{},
			},
		},
	})
}

func TestConfigSnapshotTransparentProxyCatalogDestinationsOnly(t testing.T) *ConfigSnapshot {
	// DiscoveryChain without an UpstreamConfig should yield a
	// filter chain when in transparent proxy mode
	var (
		google      = structs.NewServiceName("google", nil)
		googleUID   = NewUpstreamIDFromServiceName(google)
		googleChain = discoverychain.TestCompileConfigEntries(t, "google", "default", "default", "dc1", connect.TestClusterID+".consul", nil)

		noEndpoints      = structs.NewServiceName("no-endpoints", nil)
		noEndpointsUID   = NewUpstreamIDFromServiceName(noEndpoints)
		noEndpointsChain = discoverychain.TestCompileConfigEntries(t, "no-endpoints", "default", "default", "dc1", connect.TestClusterID+".consul", nil)

		db = structs.NewServiceName("db", nil)
	)

	return TestConfigSnapshot(t, func(ns *structs.NodeService) {
		ns.Proxy.Mode = structs.ProxyModeTransparent
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
			CorrelationID: intentionUpstreamsID,
			Result: &structs.IndexedServiceList{
				Services: structs.ServiceList{
					google,
					noEndpoints,
					// In transparent proxy mode, watches for
					// upstreams in the local DC are handled by the
					// IntentionUpstreams watch!
					db,
				},
			},
		},
		{
			CorrelationID: "discovery-chain:" + googleUID.String(),
			Result: &structs.DiscoveryChainResponse{
				Chain: googleChain,
			},
		},
		{
			CorrelationID: "discovery-chain:" + noEndpointsUID.String(),
			Result: &structs.DiscoveryChainResponse{
				Chain: noEndpointsChain,
			},
		},
		{
			CorrelationID: "upstream-target:" + googleChain.ID() + ":" + googleUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: []structs.CheckServiceNode{
					{
						Node: &structs.Node{
							Address:    "8.8.8.8",
							Datacenter: "dc1",
						},
						Service: &structs.NodeService{
							Service: "google",
							Address: "9.9.9.9",
							Port:    9090,
							TaggedAddresses: map[string]structs.ServiceAddress{
								"virtual": {Address: "10.0.0.1"},
							},
						},
					},
				},
			},
		},
		{
			CorrelationID: "upstream-target:" + noEndpointsChain.ID() + ":" + noEndpointsUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				// DiscoveryChains without endpoints do not get a
				// filter chain because there are no addresses to
				// match on.
				Nodes: []structs.CheckServiceNode{},
			},
		},
	})
}

func TestConfigSnapshotTransparentProxyDialDirectly(t testing.T) *ConfigSnapshot {
	// DiscoveryChain without an UpstreamConfig should yield a
	// filter chain when in transparent proxy mode
	var (
		kafka      = structs.NewServiceName("kafka", nil)
		kafkaUID   = NewUpstreamIDFromServiceName(kafka)
		kafkaChain = discoverychain.TestCompileConfigEntries(t, "kafka", "default", "default", "dc1", connect.TestClusterID+".consul", nil)

		mongo      = structs.NewServiceName("mongo", nil)
		mongoUID   = NewUpstreamIDFromServiceName(mongo)
		mongoChain = discoverychain.TestCompileConfigEntries(t, "mongo", "default", "default", "dc1", connect.TestClusterID+".consul", nil, &structs.ServiceResolverConfigEntry{
			Kind:           structs.ServiceResolver,
			Name:           "mongo",
			ConnectTimeout: 33 * time.Second,
		})

		db = structs.NewServiceName("db", nil)
	)

	return TestConfigSnapshot(t, func(ns *structs.NodeService) {
		ns.Proxy.Mode = structs.ProxyModeTransparent
	}, []UpdateEvent{
		{
			CorrelationID: meshConfigEntryID,
			Result: &structs.ConfigEntryResponse{
				Entry: nil,
			},
		},
		{
			CorrelationID: intentionUpstreamsID,
			Result: &structs.IndexedServiceList{
				Services: structs.ServiceList{
					kafka,
					mongo,
					// In transparent proxy mode, watches for
					// upstreams in the local DC are handled by the
					// IntentionUpstreams watch!
					db,
				},
			},
		},
		{
			CorrelationID: "discovery-chain:" + kafkaUID.String(),
			Result: &structs.DiscoveryChainResponse{
				Chain: kafkaChain,
			},
		},
		{
			CorrelationID: "discovery-chain:" + mongoUID.String(),
			Result: &structs.DiscoveryChainResponse{
				Chain: mongoChain,
			},
		},
		{
			CorrelationID: "upstream-target:" + mongoChain.ID() + ":" + mongoUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				// There should still be a filter chain for mongo's virtual address
				Nodes: []structs.CheckServiceNode{
					{
						Node: &structs.Node{
							Datacenter: "dc1",
						},
						Service: &structs.NodeService{
							Service: "mongo",
							Address: "10.10.10.10",
							Port:    27017,
							TaggedAddresses: map[string]structs.ServiceAddress{
								"virtual": {Address: "6.6.6.6"},
							},
							Proxy: structs.ConnectProxyConfig{
								TransparentProxy: structs.TransparentProxyConfig{
									DialedDirectly: true,
								},
							},
						},
					},
					{
						Node: &structs.Node{
							Datacenter: "dc1",
						},
						Service: &structs.NodeService{
							Service: "mongo",
							Address: "10.10.10.12",
							Port:    27017,
							TaggedAddresses: map[string]structs.ServiceAddress{
								"virtual": {Address: "6.6.6.6"},
							},
							Proxy: structs.ConnectProxyConfig{
								TransparentProxy: structs.TransparentProxyConfig{
									DialedDirectly: true,
								},
							},
						},
					},
				},
			},
		},
		{
			CorrelationID: "upstream-target:" + kafkaChain.ID() + ":" + kafkaUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: []structs.CheckServiceNode{
					{
						Node: &structs.Node{
							Datacenter: "dc1",
						},
						Service: &structs.NodeService{
							Service: "kafka",
							Address: "9.9.9.9",
							Port:    9092,
							Proxy: structs.ConnectProxyConfig{
								TransparentProxy: structs.TransparentProxyConfig{
									DialedDirectly: true,
								},
							},
						},
					},
				},
			},
		},
	})
}

func TestConfigSnapshotTransparentProxyResolverRedirectUpstream(t testing.T) *ConfigSnapshot {
	// Service-Resolver redirect with explicit upstream should spawn an outbound listener.
	var (
		db      = structs.NewServiceName("db-redir", nil)
		dbUID   = NewUpstreamIDFromServiceName(db)
		dbChain = discoverychain.TestCompileConfigEntries(t, "db-redir", "default", "default", "dc1", connect.TestClusterID+".consul", nil,
			&structs.ServiceResolverConfigEntry{
				Kind: structs.ServiceResolver,
				Name: "db-redir",
				Redirect: &structs.ServiceResolverRedirect{
					Service: "db",
				},
			},
		)

		google      = structs.NewServiceName("google", nil)
		googleUID   = NewUpstreamIDFromServiceName(google)
		googleChain = discoverychain.TestCompileConfigEntries(t, "google", "default", "default", "dc1", connect.TestClusterID+".consul", nil)
	)

	return TestConfigSnapshot(t, func(ns *structs.NodeService) {
		ns.Proxy.Mode = structs.ProxyModeTransparent
		ns.Proxy.Upstreams[0].DestinationName = "db-redir"
	}, []UpdateEvent{
		{
			CorrelationID: "discovery-chain:" + dbUID.String(),
			Result: &structs.DiscoveryChainResponse{
				Chain: dbChain,
			},
		},
		{
			CorrelationID: intentionUpstreamsID,
			Result: &structs.IndexedServiceList{
				Services: structs.ServiceList{
					google,
				},
			},
		},
		{
			CorrelationID: "discovery-chain:" + googleUID.String(),
			Result: &structs.DiscoveryChainResponse{
				Chain: googleChain,
			},
		},
		{
			CorrelationID: "upstream-target:google.default.default.dc1:" + googleUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: []structs.CheckServiceNode{
					{
						Node: &structs.Node{
							Address:    "8.8.8.8",
							Datacenter: "dc1",
						},
						Service: &structs.NodeService{
							Service: "google",
							Address: "9.9.9.9",
							Port:    9090,
							TaggedAddresses: map[string]structs.ServiceAddress{
								"virtual":                      {Address: "10.0.0.1"},
								structs.TaggedAddressVirtualIP: {Address: "240.0.0.1"},
							},
						},
					},
				},
			},
		},
	})
}

func TestConfigSnapshotTransparentProxyTerminatingGatewayCatalogDestinationsOnly(t testing.T) *ConfigSnapshot {
	// DiscoveryChain without an UpstreamConfig should yield a
	// filter chain when in transparent proxy mode
	var (
		google      = structs.NewServiceName("google", nil)
		googleUID   = NewUpstreamIDFromServiceName(google)
		googleChain = discoverychain.TestCompileConfigEntries(t, "google", "default", "default", "dc1", connect.TestClusterID+".consul", nil)

		kafka      = structs.NewServiceName("kafka", nil)
		kafkaUID   = NewUpstreamIDFromServiceName(kafka)
		kafkaChain = discoverychain.TestCompileConfigEntries(t, "kafka", "default", "default", "dc1", connect.TestClusterID+".consul", nil)

		db = structs.NewServiceName("db", nil)
	)

	// DiscoveryChain without an UpstreamConfig should yield a filter chain when in transparent proxy mode

	tgate := structs.CheckServiceNode{
		Node: &structs.Node{
			Address:    "8.8.8.8",
			Datacenter: "dc1",
		},
		Service: &structs.NodeService{
			Service: "tgate1",
			Kind:    structs.ServiceKind(structs.TerminatingGateway),
			Address: "9.9.9.9",
			Port:    9090,
			TaggedAddresses: map[string]structs.ServiceAddress{
				structs.ServiceGatewayVirtualIPTag(google): {Address: "10.0.0.1"},
				structs.ServiceGatewayVirtualIPTag(kafka):  {Address: "10.0.0.2"},
				"virtual": {Address: "6.6.6.6"},
			},
		},
	}

	return TestConfigSnapshot(t, func(ns *structs.NodeService) {
		ns.Proxy.Mode = structs.ProxyModeTransparent
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
			CorrelationID: intentionUpstreamsID,
			Result: &structs.IndexedServiceList{
				Services: structs.ServiceList{
					google,
					kafka,
					// In transparent proxy mode, watches for
					// upstreams in the local DC are handled by the
					// IntentionUpstreams watch!
					db,
				},
			},
		},
		{
			CorrelationID: "discovery-chain:" + googleUID.String(),
			Result: &structs.DiscoveryChainResponse{
				Chain: googleChain,
			},
		},
		{
			CorrelationID: "discovery-chain:" + kafkaUID.String(),
			Result: &structs.DiscoveryChainResponse{
				Chain: kafkaChain,
			},
		},
		{
			CorrelationID: "upstream-target:" + googleChain.ID() + ":" + googleUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: []structs.CheckServiceNode{tgate},
			},
		},
		{
			CorrelationID: "upstream-target:" + kafkaChain.ID() + ":" + kafkaUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: []structs.CheckServiceNode{tgate},
			},
		},
	})
}

func TestConfigSnapshotTransparentProxyDestination(t testing.T) *ConfigSnapshot {
	// DiscoveryChain without an UpstreamConfig should yield a
	// filter chain when in transparent proxy mode
	var (
		google    = structs.NewServiceName("google", nil)
		googleUID = NewUpstreamIDFromServiceName(google)
		googleCE  = structs.ServiceConfigEntry{
			Name: "google",
			Destination: &structs.DestinationConfig{
				Addresses: []string{
					"www.google.com",
					"api.google.com",
				},
				Port: 443,
			},
		}

		kafka    = structs.NewServiceName("kafka", nil)
		kafkaUID = NewUpstreamIDFromServiceName(kafka)
		kafkaCE  = structs.ServiceConfigEntry{
			Name: "kafka",
			Destination: &structs.DestinationConfig{
				Addresses: []string{
					"192.168.2.1",
					"192.168.2.2",
				},
				Port: 9093,
			},
		}
	)

	serviceNodes := structs.CheckServiceNodes{
		{
			Node: &structs.Node{
				Node:       "node1",
				Address:    "172.168.0.1",
				Datacenter: "dc1",
			},
			Service: &structs.NodeService{
				ID:      "tgtw1",
				Address: "172.168.0.1",
				Port:    8443,
				Kind:    structs.ServiceKindTerminatingGateway,
				TaggedAddresses: map[string]structs.ServiceAddress{
					structs.TaggedAddressLANIPv4:   {Address: "172.168.0.1", Port: 8443},
					structs.TaggedAddressVirtualIP: {Address: "240.0.0.1"},
				},
			},
			Checks: []*structs.HealthCheck{
				{
					Node:        "node1",
					ServiceName: "tgtw",
					Name:        "force",
					Status:      api.HealthPassing,
				},
			},
		},
	}
	return TestConfigSnapshot(t, func(ns *structs.NodeService) {
		ns.Proxy.Mode = structs.ProxyModeTransparent
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
			CorrelationID: intentionUpstreamsDestinationID,
			Result: &structs.IndexedServiceList{
				Services: structs.ServiceList{
					google,
					kafka,
				},
			},
		},
		{
			CorrelationID: DestinationConfigEntryID + googleUID.String(),
			Result: &structs.ConfigEntryResponse{
				Entry: &googleCE,
			},
		},
		{
			CorrelationID: DestinationConfigEntryID + kafkaUID.String(),
			Result: &structs.ConfigEntryResponse{
				Entry: &kafkaCE,
			},
		},
		{
			CorrelationID: DestinationGatewayID + googleUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: serviceNodes,
			},
		},
		{
			CorrelationID: DestinationGatewayID + kafkaUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: serviceNodes,
			},
		},
	})
}

func TestConfigSnapshotTransparentProxyDestinationHTTP(t testing.T) *ConfigSnapshot {
	// DiscoveryChain without an UpstreamConfig should yield a
	// filter chain when in transparent proxy mode
	var (
		google    = structs.NewServiceName("google", nil)
		googleUID = NewUpstreamIDFromServiceName(google)
		googleCE  = structs.ServiceConfigEntry{Name: "google", Destination: &structs.DestinationConfig{Addresses: []string{"www.google.com"}, Port: 443}, Protocol: "http"}

		kafka    = structs.NewServiceName("kafka", nil)
		kafkaUID = NewUpstreamIDFromServiceName(kafka)
		kafkaCE  = structs.ServiceConfigEntry{Name: "kafka", Destination: &structs.DestinationConfig{Addresses: []string{"192.168.2.1"}, Port: 9093}, Protocol: "http"}

		kafka2    = structs.NewServiceName("kafka2", nil)
		kafka2UID = NewUpstreamIDFromServiceName(kafka2)
		kafka2CE  = structs.ServiceConfigEntry{Name: "kafka2", Destination: &structs.DestinationConfig{Addresses: []string{"192.168.2.2", "192.168.2.3"}, Port: 9093}, Protocol: "http"}
	)

	serviceNodes := structs.CheckServiceNodes{
		{
			Node: &structs.Node{
				Node:       "node1",
				Address:    "172.168.0.1",
				Datacenter: "dc1",
			},
			Service: &structs.NodeService{
				ID:      "tgtw1",
				Address: "172.168.0.1",
				Port:    8443,
				Kind:    structs.ServiceKindTerminatingGateway,
				TaggedAddresses: map[string]structs.ServiceAddress{
					structs.TaggedAddressLANIPv4:   {Address: "172.168.0.1", Port: 8443},
					structs.TaggedAddressVirtualIP: {Address: "240.0.0.1"},
				},
			},
			Checks: []*structs.HealthCheck{
				{
					Node:        "node1",
					ServiceName: "tgtw",
					Name:        "force",
					Status:      api.HealthPassing,
				},
			},
		},
	}
	return TestConfigSnapshot(t, func(ns *structs.NodeService) {
		ns.Proxy.Mode = structs.ProxyModeTransparent
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
			CorrelationID: intentionUpstreamsDestinationID,
			Result: &structs.IndexedServiceList{
				Services: structs.ServiceList{
					google,
					kafka,
					kafka2,
				},
			},
		},
		{
			CorrelationID: DestinationConfigEntryID + googleUID.String(),
			Result: &structs.ConfigEntryResponse{
				Entry: &googleCE,
			},
		},
		{
			CorrelationID: DestinationConfigEntryID + kafkaUID.String(),
			Result: &structs.ConfigEntryResponse{
				Entry: &kafkaCE,
			},
		},
		{
			CorrelationID: DestinationConfigEntryID + kafka2UID.String(),
			Result: &structs.ConfigEntryResponse{
				Entry: &kafka2CE,
			},
		},
		{
			CorrelationID: DestinationGatewayID + googleUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: serviceNodes,
			},
		},
		{
			CorrelationID: DestinationGatewayID + kafkaUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: serviceNodes,
			},
		},
		{
			CorrelationID: DestinationGatewayID + kafka2UID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: serviceNodes,
			},
		},
	})
}
