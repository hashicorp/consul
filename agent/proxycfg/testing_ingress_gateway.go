// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package proxycfg

import (
	"time"

	"github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/types"
)

func TestConfigSnapshotIngressGateway(
	t testing.T,
	populateServices bool,
	protocol string,
	variation string,
	nsFn func(ns *structs.NodeService),
	configFn func(entry *structs.IngressGatewayConfigEntry),
	extraUpdates []UpdateEvent,
	additionalEntries ...structs.ConfigEntry,
) *ConfigSnapshot {
	roots, placeholderLeaf := TestCerts(t)

	entry := &structs.IngressGatewayConfigEntry{
		Kind: structs.IngressGateway,
		Name: "ingress-gateway",
	}

	if populateServices {
		entry.Listeners = []structs.IngressListener{
			{
				Port:     8080,
				Protocol: protocol,
				Services: []structs.IngressService{
					{Name: "db"},
				},
			},
		}
	}

	if configFn != nil {
		configFn(entry)
	}

	baseEvents := []UpdateEvent{
		{
			CorrelationID: rootsWatchID,
			Result:        roots,
		},
		{
			CorrelationID: gatewayConfigWatchID,
			Result: &structs.ConfigEntryResponse{
				Entry: entry,
			},
		},
		{
			CorrelationID: leafWatchID,
			Result:        placeholderLeaf, // TODO(rb): should this be generated differently?
		},
		{
			CorrelationID: gatewayServicesWatchID,
			Result: &structs.IndexedGatewayServices{
				Services: nil,
			},
		},
	}

	if populateServices {
		baseEvents = testSpliceEvents(baseEvents, []UpdateEvent{{
			CorrelationID: gatewayServicesWatchID,
			Result: &structs.IndexedGatewayServices{
				Services: []*structs.GatewayService{
					{
						Service:  structs.NewServiceName("db", nil),
						Port:     8080,
						Hosts:    nil,
						Protocol: protocol,
					},
				},
			},
		}})

		upstreams := structs.TestUpstreams(t, false)
		upstreams = structs.Upstreams{upstreams[0]} // just keep 'db'

		baseEvents = testSpliceEvents(baseEvents, setupTestVariationConfigEntriesAndSnapshot(
			t, variation, false, upstreams, additionalEntries...,
		))
	}

	return testConfigSnapshotFixture(t, &structs.NodeService{
		Kind:            structs.ServiceKindIngressGateway,
		Service:         "ingress-gateway",
		Port:            9999,
		Address:         "1.2.3.4",
		Meta:            nil,
		TaggedAddresses: nil,
	}, nsFn, nil, testSpliceEvents(baseEvents, extraUpdates))
}

// TestConfigSnapshotIngressGateway_NilConfigEntry is used to test when
// the update event for the config entry returns nil
// since this always happens on the first watch if it doesn't exist.
func TestConfigSnapshotIngressGateway_NilConfigEntry(
	t testing.T,
) *ConfigSnapshot {
	roots, placeholderLeaf := TestCerts(t)

	baseEvents := []UpdateEvent{
		{
			CorrelationID: rootsWatchID,
			Result:        roots,
		},
		{
			CorrelationID: gatewayConfigWatchID,
			Result: &structs.ConfigEntryResponse{
				Entry: nil, // The first watch on a config entry will return nil if the config entry doesn't exist.
			},
		},
		{
			CorrelationID: leafWatchID,
			Result:        placeholderLeaf,
		},
		{
			CorrelationID: gatewayServicesWatchID,
			Result: &structs.IndexedGatewayServices{
				Services: nil,
			},
		},
	}

	return testConfigSnapshotFixture(t, &structs.NodeService{
		Kind:            structs.ServiceKindIngressGateway,
		Service:         "ingress-gateway",
		Port:            9999,
		Address:         "1.2.3.4",
		Meta:            nil,
		TaggedAddresses: nil,
	}, nil, nil, testSpliceEvents(baseEvents, nil))
}

func TestConfigSnapshotIngressGatewaySDS_GatewayLevel_MixedTLS(t testing.T) *ConfigSnapshot {
	secureUID := UpstreamIDFromString("secure")
	secureChain := discoverychain.TestCompileConfigEntries(
		t,
		"secure",
		"default",
		"default",
		"dc1",
		connect.TestClusterID+".consul",
		nil,
		nil,
	)

	insecureUID := UpstreamIDFromString("insecure")
	insecureChain := discoverychain.TestCompileConfigEntries(
		t,
		"insecure",
		"default",
		"default",
		"dc1",
		connect.TestClusterID+".consul",
		nil,
		nil,
	)

	return TestConfigSnapshotIngressGateway(t, false, "tcp", "default", nil, func(entry *structs.IngressGatewayConfigEntry) {
		// Disable GW-level defaults so we can mix TLS and non-TLS listeners
		entry.TLS = structs.GatewayTLSConfig{
			SDS: nil,
		}
		entry.Listeners = []structs.IngressListener{
			// Setup two TCP listeners, one with and one without SDS config
			{
				Port:     8080,
				Protocol: "tcp",
				TLS: &structs.GatewayTLSConfig{
					SDS: &structs.GatewayTLSSDSConfig{
						ClusterName:  "listener-sds-cluster",
						CertResource: "listener-cert",
					},
				},
				Services: []structs.IngressService{
					{Name: "secure"},
				},
			},
			{
				Port:     9090,
				Protocol: "tcp",
				TLS:      nil,
				Services: []structs.IngressService{
					{Name: "insecure"},
				},
			},
		}
	}, []UpdateEvent{
		{
			CorrelationID: gatewayServicesWatchID,
			Result: &structs.IndexedGatewayServices{
				Services: []*structs.GatewayService{
					{
						Service:  structs.NewServiceName("secure", nil),
						Port:     8080,
						Protocol: "tcp",
					},
					{
						Service:  structs.NewServiceName("insecure", nil),
						Port:     9090,
						Protocol: "tcp",
					},
				},
			},
		},
		{
			CorrelationID: "discovery-chain:" + secureUID.String(),
			Result: &structs.DiscoveryChainResponse{
				Chain: secureChain,
			},
		},
		{
			CorrelationID: "upstream-target:" + secureChain.ID() + ":" + secureUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodes(t, "secure"),
			},
		},
		{
			CorrelationID: "discovery-chain:" + insecureUID.String(),
			Result: &structs.DiscoveryChainResponse{
				Chain: insecureChain,
			},
		},
		{
			CorrelationID: "upstream-target:" + insecureChain.ID() + ":" + insecureUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodes(t, "insecure"),
			},
		},
	})
}

func TestConfigSnapshotIngressGatewaySDS_GatewayLevel(t testing.T) *ConfigSnapshot {
	return TestConfigSnapshotIngressGateway(t, true, "tcp", "default", nil, func(entry *structs.IngressGatewayConfigEntry) {
		entry.TLS = structs.GatewayTLSConfig{
			SDS: &structs.GatewayTLSSDSConfig{
				ClusterName:  "sds-cluster",
				CertResource: "cert-resource",
			},
		}
	}, nil)
}

func TestConfigSnapshotIngressGatewaySDS_GatewayAndListenerLevel(t testing.T) *ConfigSnapshot {
	return TestConfigSnapshotIngressGateway(t, true, "tcp", "default", nil, func(entry *structs.IngressGatewayConfigEntry) {
		entry.TLS = structs.GatewayTLSConfig{
			SDS: &structs.GatewayTLSSDSConfig{
				ClusterName:  "sds-cluster",
				CertResource: "cert-resource",
			},
		}
		entry.Listeners[0].TLS = &structs.GatewayTLSConfig{
			SDS: &structs.GatewayTLSSDSConfig{
				// Override the cert, fall back to the cluster at gw level. We
				// don't test every possible valid combination here since we
				// already did that in TestResolveListenerSDSConfig. This is
				// just an extra check to make sure that data is plumbed through
				// correctly.
				CertResource: "listener-cert",
			},
		}
	}, nil)
}

func TestConfigSnapshotIngressGatewaySDS_GatewayAndListenerLevel_HTTP(t testing.T) *ConfigSnapshot {
	set := configentry.NewDiscoveryChainSet()
	set.AddEntries(&structs.ServiceConfigEntry{
		Kind:     structs.ServiceDefaults,
		Name:     "http",
		Protocol: "http",
	})
	var (
		http      = structs.NewServiceName("http", nil)
		httpUID   = NewUpstreamIDFromServiceName(http)
		httpChain = discoverychain.TestCompileConfigEntries(t, "http", "default", "default", "dc1", connect.TestClusterID+".consul", nil, set)
	)

	return TestConfigSnapshotIngressGateway(t, false, "http", "default", nil, func(entry *structs.IngressGatewayConfigEntry) {
		entry.TLS = structs.GatewayTLSConfig{
			SDS: &structs.GatewayTLSSDSConfig{
				ClusterName:  "sds-cluster",
				CertResource: "cert-resource",
			},
		}
		entry.Listeners = []structs.IngressListener{
			{
				Port:     8080,
				Protocol: "http",
				Services: []structs.IngressService{
					{Name: "http"},
				},
				TLS: &structs.GatewayTLSConfig{
					SDS: &structs.GatewayTLSSDSConfig{
						// Override the cert, fall back to the cluster at gw level. We
						// don't test every possible valid combination here since we
						// already did that in TestResolveListenerSDSConfig. This is
						// just an extra check to make sure that data is plumbed through
						// correctly.
						CertResource: "listener-cert",
					},
				},
			},
		}
	}, []UpdateEvent{
		{
			CorrelationID: gatewayServicesWatchID,
			Result: &structs.IndexedGatewayServices{
				Services: []*structs.GatewayService{
					{
						Service:  http,
						Port:     8080,
						Protocol: "http",
					},
				},
			},
		},
		{
			CorrelationID: "discovery-chain:" + httpUID.String(),
			Result: &structs.DiscoveryChainResponse{
				Chain: httpChain,
			},
		},
		{
			CorrelationID: "upstream-target:" + httpChain.ID() + ":" + httpUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodes(t, "http"),
			},
		},
	})
}

func TestConfigSnapshotIngressGatewaySDS_ServiceLevel(t testing.T) *ConfigSnapshot {
	var (
		s1      = structs.NewServiceName("s1", nil)
		s1UID   = NewUpstreamIDFromServiceName(s1)
		s1Chain = discoverychain.TestCompileConfigEntries(t, "s1", "default", "default", "dc1", connect.TestClusterID+".consul", nil, nil)

		s2      = structs.NewServiceName("s2", nil)
		s2UID   = NewUpstreamIDFromServiceName(s2)
		s2Chain = discoverychain.TestCompileConfigEntries(t, "s2", "default", "default", "dc1", connect.TestClusterID+".consul", nil, nil)
	)

	return TestConfigSnapshotIngressGateway(t, false, "tcp", "default", nil, func(entry *structs.IngressGatewayConfigEntry) {
		// Disable GW-level defaults so we can test only service-level
		entry.TLS = structs.GatewayTLSConfig{
			SDS: nil,
		}
		entry.Listeners = []structs.IngressListener{
			// Setup http listeners, one multiple services with SDS
			{
				Port:     8080,
				Protocol: "http",
				TLS:      nil, // no listener-level SDS config
				Services: []structs.IngressService{
					{
						Name:  "s1",
						Hosts: []string{"s1.example.com"},
						TLS: &structs.GatewayServiceTLSConfig{
							SDS: &structs.GatewayTLSSDSConfig{
								ClusterName:  "sds-cluster-1",
								CertResource: "s1.example.com-cert",
							},
						},
					},
					{
						Name:  "s2",
						Hosts: []string{"s2.example.com"},
						TLS: &structs.GatewayServiceTLSConfig{
							SDS: &structs.GatewayTLSSDSConfig{
								ClusterName:  "sds-cluster-2",
								CertResource: "s2.example.com-cert",
							},
						},
					},
				},
			},
		}
	}, []UpdateEvent{
		{
			CorrelationID: gatewayServicesWatchID,
			Result: &structs.IndexedGatewayServices{
				Services: []*structs.GatewayService{
					{
						Service:  s1,
						Port:     8080,
						Protocol: "http",
					},
					{
						Service:  s2,
						Port:     8080,
						Protocol: "http",
					},
				},
			},
		},
		{
			CorrelationID: "discovery-chain:" + s1UID.String(),
			Result: &structs.DiscoveryChainResponse{
				Chain: s1Chain,
			},
		},
		{
			CorrelationID: "upstream-target:" + s1Chain.ID() + ":" + s1UID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodes(t, "s1"),
			},
		},
		{
			CorrelationID: "discovery-chain:" + s2UID.String(),
			Result: &structs.DiscoveryChainResponse{
				Chain: s2Chain,
			},
		},
		{
			CorrelationID: "upstream-target:" + s2Chain.ID() + ":" + s2UID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodes(t, "s2"),
			},
		},
	})
}

func TestConfigSnapshotIngressGatewaySDS_ListenerAndServiceLevel(t testing.T) *ConfigSnapshot {
	var (
		s1      = structs.NewServiceName("s1", nil)
		s1UID   = NewUpstreamIDFromServiceName(s1)
		s1Chain = discoverychain.TestCompileConfigEntries(t, "s1", "default", "default", "dc1", connect.TestClusterID+".consul", nil, nil)

		s2      = structs.NewServiceName("s2", nil)
		s2UID   = NewUpstreamIDFromServiceName(s2)
		s2Chain = discoverychain.TestCompileConfigEntries(t, "s2", "default", "default", "dc1", connect.TestClusterID+".consul", nil, nil)
	)

	return TestConfigSnapshotIngressGateway(t, false, "tcp", "default", nil, func(entry *structs.IngressGatewayConfigEntry) {
		// Disable GW-level defaults so we can test only service-level
		entry.TLS = structs.GatewayTLSConfig{
			SDS: nil,
		}
		entry.Listeners = []structs.IngressListener{
			// Setup http listeners, one multiple services with SDS
			{
				Port:     8080,
				Protocol: "http",
				TLS: &structs.GatewayTLSConfig{
					SDS: &structs.GatewayTLSSDSConfig{
						ClusterName:  "sds-cluster-2",
						CertResource: "*.example.com-cert",
					},
				},
				Services: []structs.IngressService{
					{
						Name:  "s1",
						Hosts: []string{"s1.example.com"},
						TLS: &structs.GatewayServiceTLSConfig{
							SDS: &structs.GatewayTLSSDSConfig{
								ClusterName:  "sds-cluster-1",
								CertResource: "s1.example.com-cert",
							},
						},
					},
					{
						Name: "s2",
						// s2 uses the default listener cert
					},
				},
			},
		}
	}, []UpdateEvent{
		{
			CorrelationID: gatewayServicesWatchID,
			Result: &structs.IndexedGatewayServices{
				Services: []*structs.GatewayService{
					{
						Service:  s1,
						Port:     8080,
						Protocol: "http",
					},
					{
						Service:  s2,
						Port:     8080,
						Protocol: "http",
					},
				},
			},
		},
		{
			CorrelationID: "discovery-chain:" + s1UID.String(),
			Result: &structs.DiscoveryChainResponse{
				Chain: s1Chain,
			},
		},
		{
			CorrelationID: "upstream-target:" + s1Chain.ID() + ":" + s1UID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodes(t, "s1"),
			},
		},
		{
			CorrelationID: "discovery-chain:" + s2UID.String(),
			Result: &structs.DiscoveryChainResponse{
				Chain: s2Chain,
			},
		},
		{
			CorrelationID: "upstream-target:" + s2Chain.ID() + ":" + s2UID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodes(t, "s2"),
			},
		},
	})
}

func TestConfigSnapshotIngressGatewaySDS_MixedNoTLS(t testing.T) *ConfigSnapshot {
	var (
		s1      = structs.NewServiceName("s1", nil)
		s1UID   = NewUpstreamIDFromServiceName(s1)
		s1Chain = discoverychain.TestCompileConfigEntries(t, "s1", "default", "default", "dc1", connect.TestClusterID+".consul", nil, nil)

		s2      = structs.NewServiceName("s2", nil)
		s2UID   = NewUpstreamIDFromServiceName(s2)
		s2Chain = discoverychain.TestCompileConfigEntries(t, "s2", "default", "default", "dc1", connect.TestClusterID+".consul", nil, nil)
	)

	return TestConfigSnapshotIngressGateway(t, false, "tcp", "default", nil, func(entry *structs.IngressGatewayConfigEntry) {
		// Disable GW-level defaults so we can test only service-level
		entry.TLS = structs.GatewayTLSConfig{
			SDS: nil,
		}
		entry.Listeners = []structs.IngressListener{
			// Setup http listeners, one multiple services with SDS
			{
				Port:     8080,
				Protocol: "http",
				TLS:      nil, // No listener level TLS setup either
				Services: []structs.IngressService{
					{
						Name:  "s1",
						Hosts: []string{"s1.example.com"},
						TLS: &structs.GatewayServiceTLSConfig{
							SDS: &structs.GatewayTLSSDSConfig{
								ClusterName:  "sds-cluster-1",
								CertResource: "s1.example.com-cert",
							},
						},
					},
					{
						Name: "s2",
						// s2 has no SDS config so should be non-TLS
					},
				},
			},
		}
	}, []UpdateEvent{
		{
			CorrelationID: gatewayServicesWatchID,
			Result: &structs.IndexedGatewayServices{
				Services: []*structs.GatewayService{
					{
						Service:  s1,
						Port:     8080,
						Protocol: "http",
					},
					{
						Service:  s2,
						Port:     8080,
						Protocol: "http",
					},
				},
			},
		},
		{
			CorrelationID: "discovery-chain:" + s1UID.String(),
			Result: &structs.DiscoveryChainResponse{
				Chain: s1Chain,
			},
		},
		{
			CorrelationID: "upstream-target:" + s1Chain.ID() + ":" + s1UID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodes(t, "s1"),
			},
		},
		{
			CorrelationID: "discovery-chain:" + s2UID.String(),
			Result: &structs.DiscoveryChainResponse{
				Chain: s2Chain,
			},
		},
		{
			CorrelationID: "upstream-target:" + s2Chain.ID() + ":" + s2UID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodes(t, "s2"),
			},
		},
	})
}

func TestConfigSnapshotIngressGateway_MixedListeners(t testing.T) *ConfigSnapshot {
	var (
		s1      = structs.NewServiceName("s1", nil)
		s1UID   = NewUpstreamIDFromServiceName(s1)
		s1Chain = discoverychain.TestCompileConfigEntries(t, "s1", "default", "default", "dc1", connect.TestClusterID+".consul", nil, nil)

		s2      = structs.NewServiceName("s2", nil)
		s2UID   = NewUpstreamIDFromServiceName(s2)
		s2Chain = discoverychain.TestCompileConfigEntries(t, "s2", "default", "default", "dc1", connect.TestClusterID+".consul", nil, nil)
	)

	return TestConfigSnapshotIngressGateway(t, false, "tcp", "default", nil, func(entry *structs.IngressGatewayConfigEntry) {
		entry.TLS = structs.GatewayTLSConfig{
			Enabled: false, // No Gateway-level built-in TLS
			SDS:     nil,   // Undo gateway-level SDS
		}
		entry.Listeners = []structs.IngressListener{
			// One listener has built-in TLS, one doesn't
			{
				Port:     8080,
				Protocol: "http",
				TLS: &structs.GatewayTLSConfig{
					Enabled: true, // built-in TLS enabled
				},
				Services: []structs.IngressService{
					{Name: "s1"},
				},
			},
			{
				Port:     9090,
				Protocol: "http",
				TLS:      nil, // No TLS enabled
				Services: []structs.IngressService{
					{Name: "s2"},
				},
			},
		}
	}, []UpdateEvent{
		{
			CorrelationID: gatewayServicesWatchID,
			Result: &structs.IndexedGatewayServices{
				Services: []*structs.GatewayService{
					{
						Service:  s1,
						Port:     8080,
						Protocol: "http",
					},
					{
						Service:  s2,
						Port:     9090,
						Protocol: "http",
					},
				},
			},
		},
		{
			CorrelationID: "discovery-chain:" + s1UID.String(),
			Result: &structs.DiscoveryChainResponse{
				Chain: s1Chain,
			},
		},
		{
			CorrelationID: "upstream-target:" + s1Chain.ID() + ":" + s1UID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodes(t, "s1"),
			},
		},
		{
			CorrelationID: "discovery-chain:" + s2UID.String(),
			Result: &structs.DiscoveryChainResponse{
				Chain: s2Chain,
			},
		},
		{
			CorrelationID: "upstream-target:" + s2Chain.ID() + ":" + s2UID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodes(t, "s2"),
			},
		},
	})
}

func TestConfigSnapshotIngress_HTTPMultipleServices(t testing.T) *ConfigSnapshot {
	// We do not add baz/qux here so that we test the chain.IsDefault() case
	entries := []structs.ConfigEntry{
		&structs.ProxyConfigEntry{
			Kind: structs.ProxyDefaults,
			Name: structs.ProxyConfigGlobal,
			Config: map[string]interface{}{
				"protocol": "http",
			},
		},
		&structs.ServiceResolverConfigEntry{
			Kind:           structs.ServiceResolver,
			Name:           "foo",
			ConnectTimeout: 22 * time.Second,
			RequestTimeout: 22 * time.Second,
		},
		&structs.ServiceResolverConfigEntry{
			Kind:           structs.ServiceResolver,
			Name:           "bar",
			ConnectTimeout: 22 * time.Second,
			RequestTimeout: 22 * time.Second,
		},
	}

	set := configentry.NewDiscoveryChainSet()
	set.AddEntries(entries...)

	var (
		foo      = structs.NewServiceName("foo", nil)
		fooUID   = NewUpstreamIDFromServiceName(foo)
		fooChain = discoverychain.TestCompileConfigEntries(t, "foo", "default", "default", "dc1", connect.TestClusterID+".consul", nil, set)

		bar      = structs.NewServiceName("bar", nil)
		barUID   = NewUpstreamIDFromServiceName(bar)
		barChain = discoverychain.TestCompileConfigEntries(t, "bar", "default", "default", "dc1", connect.TestClusterID+".consul", nil, set)

		baz      = structs.NewServiceName("baz", nil)
		bazUID   = NewUpstreamIDFromServiceName(baz)
		bazChain = discoverychain.TestCompileConfigEntries(t, "baz", "default", "default", "dc1", connect.TestClusterID+".consul", nil, set)

		qux      = structs.NewServiceName("qux", nil)
		quxUID   = NewUpstreamIDFromServiceName(qux)
		quxChain = discoverychain.TestCompileConfigEntries(t, "qux", "default", "default", "dc1", connect.TestClusterID+".consul", nil, set)
	)

	require.False(t, fooChain.Default)
	require.False(t, barChain.Default)
	require.True(t, bazChain.Default)
	require.True(t, quxChain.Default)

	return TestConfigSnapshotIngressGateway(t, false, "http", "default", nil, func(entry *structs.IngressGatewayConfigEntry) {
		entry.Listeners = []structs.IngressListener{
			{
				Port:     8080,
				Protocol: "http",
				Services: []structs.IngressService{
					{
						Name: "foo",
						Hosts: []string{
							"test1.example.com",
							"test2.example.com",
							"test2.example.com:8080",
						},
					},
					{Name: "bar"},
				},
			},
			{
				Port:     443,
				Protocol: "http",
				Services: []structs.IngressService{
					{Name: "baz"},
					{Name: "qux"},
				},
			},
		}
	}, []UpdateEvent{
		{
			CorrelationID: gatewayServicesWatchID,
			Result: &structs.IndexedGatewayServices{
				Services: []*structs.GatewayService{
					{
						Service:  foo,
						Port:     8080,
						Protocol: "http",
						Hosts: []string{
							"test1.example.com",
							"test2.example.com",
							"test2.example.com:8080",
						},
					},
					{
						Service:  bar,
						Port:     8080,
						Protocol: "http",
					},
					{
						Service:  baz,
						Port:     443,
						Protocol: "http",
					},
					{
						Service:  qux,
						Port:     443,
						Protocol: "http",
					},
				},
			},
		},
		{
			CorrelationID: "discovery-chain:" + fooUID.String(),
			Result: &structs.DiscoveryChainResponse{
				Chain: fooChain,
			},
		},
		{
			CorrelationID: "upstream-target:" + fooChain.ID() + ":" + fooUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodes(t, "foo"),
			},
		},
		{
			CorrelationID: "discovery-chain:" + barUID.String(),
			Result: &structs.DiscoveryChainResponse{
				Chain: barChain,
			},
		},
		{
			CorrelationID: "upstream-target:" + barChain.ID() + ":" + barUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodes(t, "bar"),
			},
		},
		{
			CorrelationID: "discovery-chain:" + bazUID.String(),
			Result: &structs.DiscoveryChainResponse{
				Chain: bazChain,
			},
		},
		{
			CorrelationID: "upstream-target:" + bazChain.ID() + ":" + bazUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodes(t, "baz"),
			},
		},
		{
			CorrelationID: "discovery-chain:" + quxUID.String(),
			Result: &structs.DiscoveryChainResponse{
				Chain: quxChain,
			},
		},
		{
			CorrelationID: "upstream-target:" + quxChain.ID() + ":" + quxUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodes(t, "qux"),
			},
		},
	})
}

func TestConfigSnapshotIngress_GRPCMultipleServices(t testing.T) *ConfigSnapshot {
	// We do not add baz/qux here so that we test the chain.IsDefault() case
	entries := []structs.ConfigEntry{
		&structs.ProxyConfigEntry{
			Kind: structs.ProxyDefaults,
			Name: structs.ProxyConfigGlobal,
			Config: map[string]interface{}{
				"protocol": "http",
			},
		},
		&structs.ServiceResolverConfigEntry{
			Kind:           structs.ServiceResolver,
			Name:           "foo",
			ConnectTimeout: 22 * time.Second,
			RequestTimeout: 22 * time.Second,
		},
		&structs.ServiceResolverConfigEntry{
			Kind:           structs.ServiceResolver,
			Name:           "bar",
			ConnectTimeout: 22 * time.Second,
			RequestTimeout: 22 * time.Second,
		},
	}

	set := configentry.NewDiscoveryChainSet()
	set.AddEntries(entries...)

	var (
		foo      = structs.NewServiceName("foo", nil)
		fooUID   = NewUpstreamIDFromServiceName(foo)
		fooChain = discoverychain.TestCompileConfigEntries(t, "foo", "default", "default", "dc1", connect.TestClusterID+".consul", nil, set)

		bar      = structs.NewServiceName("bar", nil)
		barUID   = NewUpstreamIDFromServiceName(bar)
		barChain = discoverychain.TestCompileConfigEntries(t, "bar", "default", "default", "dc1", connect.TestClusterID+".consul", nil, set)
	)

	require.False(t, fooChain.Default)
	require.False(t, barChain.Default)

	return TestConfigSnapshotIngressGateway(t, false, "http", "default", nil, func(entry *structs.IngressGatewayConfigEntry) {
		entry.Listeners = []structs.IngressListener{
			{
				Port:     8080,
				Protocol: "grpc",
				Services: []structs.IngressService{
					{
						Name: "foo",
						Hosts: []string{
							"test1.example.com",
							"test2.example.com",
							"test2.example.com:8080",
						},
					},
					{Name: "bar"},
				},
			},
		}
	}, []UpdateEvent{
		{
			CorrelationID: gatewayServicesWatchID,
			Result: &structs.IndexedGatewayServices{
				Services: []*structs.GatewayService{
					{
						Service:  foo,
						Port:     8080,
						Protocol: "grpc",
						Hosts: []string{
							"test1.example.com",
							"test2.example.com",
							"test2.example.com:8080",
						},
					},
					{
						Service:  bar,
						Port:     8080,
						Protocol: "grpc",
					},
				},
			},
		},
		{
			CorrelationID: "discovery-chain:" + fooUID.String(),
			Result: &structs.DiscoveryChainResponse{
				Chain: fooChain,
			},
		},
		{
			CorrelationID: "upstream-target:" + fooChain.ID() + ":" + fooUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodes(t, "foo"),
			},
		},
		{
			CorrelationID: "discovery-chain:" + barUID.String(),
			Result: &structs.DiscoveryChainResponse{
				Chain: barChain,
			},
		},
		{
			CorrelationID: "upstream-target:" + barChain.ID() + ":" + barUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodes(t, "bar"),
			},
		},
	})
}

func TestConfigSnapshotIngress_MultipleListenersDuplicateService(t testing.T) *ConfigSnapshot {
	var (
		foo      = structs.NewServiceName("foo", nil)
		fooUID   = NewUpstreamIDFromServiceName(foo)
		fooChain = discoverychain.TestCompileConfigEntries(t, "foo", "default", "default", "dc1", connect.TestClusterID+".consul", nil, nil)

		bar      = structs.NewServiceName("bar", nil)
		barUID   = NewUpstreamIDFromServiceName(bar)
		barChain = discoverychain.TestCompileConfigEntries(t, "bar", "default", "default", "dc1", connect.TestClusterID+".consul", nil, nil)
	)

	return TestConfigSnapshotIngressGateway(t, false, "http", "default", nil, func(entry *structs.IngressGatewayConfigEntry) {
		entry.Listeners = []structs.IngressListener{
			{
				Port:     8080,
				Protocol: "http",
				Services: []structs.IngressService{
					{Name: "foo"},
					{Name: "bar"},
				},
			},
			{
				Port:     443,
				Protocol: "http",
				Services: []structs.IngressService{
					{Name: "foo"},
				},
			},
		}
	}, []UpdateEvent{
		{
			CorrelationID: gatewayServicesWatchID,
			Result: &structs.IndexedGatewayServices{
				Services: []*structs.GatewayService{
					{
						Service:  foo,
						Port:     8080,
						Protocol: "http",
					},
					{
						Service:  bar,
						Port:     8080,
						Protocol: "http",
					},
					{
						Service:  foo,
						Port:     443,
						Protocol: "http",
					},
				},
			},
		},
		{
			CorrelationID: "discovery-chain:" + fooUID.String(),
			Result: &structs.DiscoveryChainResponse{
				Chain: fooChain,
			},
		},
		{
			CorrelationID: "upstream-target:" + fooChain.ID() + ":" + fooUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodes(t, "foo"),
			},
		},
		{
			CorrelationID: "discovery-chain:" + barUID.String(),
			Result: &structs.DiscoveryChainResponse{
				Chain: barChain,
			},
		},
		{
			CorrelationID: "upstream-target:" + barChain.ID() + ":" + barUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodesAlternate(t),
			},
		},
	})
}

func TestConfigSnapshotIngressGatewayWithChain(
	t testing.T,
	variant string,
	webEntMeta, fooEntMeta *acl.EnterpriseMeta,
) *ConfigSnapshot {
	if webEntMeta == nil {
		webEntMeta = &acl.EnterpriseMeta{}
	}
	if fooEntMeta == nil {
		fooEntMeta = &acl.EnterpriseMeta{}
	}

	var (
		updates  []UpdateEvent
		configFn func(entry *structs.IngressGatewayConfigEntry)

		populateServices                      bool
		useSDS                                bool
		listenerSDS, webSDS, fooSDS, wildcard bool
	)
	switch variant {
	case "router-header-manip":
		configFn = func(entry *structs.IngressGatewayConfigEntry) {
			entry.Listeners = []structs.IngressListener{
				{
					Port:     8080,
					Protocol: "http",
					Services: []structs.IngressService{
						{
							Name: "db",
							RequestHeaders: &structs.HTTPHeaderModifiers{
								Add: map[string]string{
									"foo": "bar",
								},
								Set: map[string]string{
									"bar": "baz",
								},
								Remove: []string{"qux"},
							},
							ResponseHeaders: &structs.HTTPHeaderModifiers{
								Add: map[string]string{
									"foo": "bar",
								},
								Set: map[string]string{
									"bar": "baz",
								},
								Remove: []string{"qux"},
							},
						},
					},
				},
			}
		}
		populateServices = true
	case "sds-listener-level":
		// Listener-level SDS means all services share the default route.
		useSDS = true
		listenerSDS = true
	case "sds-listener-level-wildcard":
		// Listener-level SDS means all services share the default route.
		useSDS = true
		listenerSDS = true
		wildcard = true
	case "sds-service-level":
		// Services should get separate routes and no default since they all
		// have custom certs.
		useSDS = true
		webSDS = true
		fooSDS = true
	case "sds-service-level-mixed-tls":
		// Web needs a separate route as it has custom filter chain but foo
		// should use default route for listener.
		useSDS = true
		webSDS = true
	default:
		t.Fatalf("unknown variant %q", variant)
		return nil
	}

	if useSDS {
		webUpstream := structs.Upstream{
			DestinationName: "web",
			// We use empty not default here because of the way upstream identifiers
			// vary between OSS and Enterprise currently causing test conflicts. In
			// real life `proxycfg` always sets ingress upstream namespaces to
			// `NamespaceOrDefault` which shouldn't matter because we should be
			// consistent within a single binary it's just inconvenient if OSS and
			// enterprise tests generate different output.
			DestinationNamespace: webEntMeta.NamespaceOrEmpty(),
			DestinationPartition: webEntMeta.PartitionOrEmpty(),
			LocalBindPort:        9191,
			IngressHosts: []string{
				"www.example.com",
			},
		}
		fooUpstream := structs.Upstream{
			DestinationName:      "foo",
			DestinationNamespace: fooEntMeta.NamespaceOrEmpty(),
			DestinationPartition: fooEntMeta.PartitionOrEmpty(),
			LocalBindPort:        9191,
			IngressHosts: []string{
				"foo.example.com",
			},
		}

		var (
			web    = structs.NewServiceName("web", webEntMeta)
			webUID = NewUpstreamID(&webUpstream)

			foo    = structs.NewServiceName("foo", fooEntMeta)
			fooUID = NewUpstreamID(&fooUpstream)
		)

		configFn = func(entry *structs.IngressGatewayConfigEntry) {
			entry.TLS.SDS = nil
			il := structs.IngressListener{
				Port:     9191,
				Protocol: "http",
				Services: []structs.IngressService{
					{
						Name:           "web",
						Hosts:          []string{"www.example.com"},
						EnterpriseMeta: *webEntMeta,
					},
					{
						Name:           "foo",
						Hosts:          []string{"foo.example.com"},
						EnterpriseMeta: *fooEntMeta,
					},
				},
			}

			// Now set the appropriate SDS configs
			if listenerSDS {
				il.TLS = &structs.GatewayTLSConfig{
					SDS: &structs.GatewayTLSSDSConfig{
						ClusterName:  "listener-cluster",
						CertResource: "listener-cert",
					},
				}
			}
			if webSDS {
				il.Services[0].TLS = &structs.GatewayServiceTLSConfig{
					SDS: &structs.GatewayTLSSDSConfig{
						ClusterName:  "web-cluster",
						CertResource: "www-cert",
					},
				}
			}
			if fooSDS {
				il.Services[1].TLS = &structs.GatewayServiceTLSConfig{
					SDS: &structs.GatewayTLSSDSConfig{
						ClusterName:  "foo-cluster",
						CertResource: "foo-cert",
					},
				}
			}
			if wildcard {
				// undo all that and set just a single wildcard config with no TLS to test
				// the lookup path where we have to compare an actual resolved upstream to
				// a wildcard config.
				il.Services = []structs.IngressService{
					{
						Name: "*",
					},
				}
			}
			entry.Listeners = []structs.IngressListener{il}
		}

		if wildcard {
			// We also don't support user-specified hosts with wildcard so remove
			// those from the upstreams.
			webUpstream.IngressHosts = nil
			fooUpstream.IngressHosts = nil
		}

		entries := []structs.ConfigEntry{
			&structs.ProxyConfigEntry{
				Kind: structs.ProxyDefaults,
				Name: structs.ProxyConfigGlobal,
				Config: map[string]interface{}{
					"protocol": "http",
				},
			},
			&structs.ServiceResolverConfigEntry{
				Kind:           structs.ServiceResolver,
				Name:           "web",
				EnterpriseMeta: *webEntMeta,
				ConnectTimeout: 22 * time.Second,
				RequestTimeout: 22 * time.Second,
			},
			&structs.ServiceResolverConfigEntry{
				Kind:           structs.ServiceResolver,
				Name:           "foo",
				EnterpriseMeta: *fooEntMeta,
				ConnectTimeout: 22 * time.Second,
				RequestTimeout: 22 * time.Second,
			},
		}

		set := configentry.NewDiscoveryChainSet()
		set.AddEntries(entries...)

		webChain := discoverychain.TestCompileConfigEntries(t, "web",
			webEntMeta.NamespaceOrDefault(),
			webEntMeta.PartitionOrDefault(), "dc1",
			connect.TestClusterID+".consul", nil, set)
		fooChain := discoverychain.TestCompileConfigEntries(t, "foo",
			fooEntMeta.NamespaceOrDefault(),
			fooEntMeta.PartitionOrDefault(), "dc1",
			connect.TestClusterID+".consul", nil, set)

		updates = []UpdateEvent{
			{
				CorrelationID: gatewayServicesWatchID,
				Result: &structs.IndexedGatewayServices{
					Services: []*structs.GatewayService{
						{
							Service:  web,
							Port:     9191,
							Protocol: "http",
							Hosts:    webUpstream.IngressHosts,
						},
						{
							Service:  foo,
							Port:     9191,
							Protocol: "http",
							Hosts:    fooUpstream.IngressHosts,
						},
					},
				},
			},
			{
				CorrelationID: "discovery-chain:" + webUID.String(),
				Result: &structs.DiscoveryChainResponse{
					Chain: webChain,
				},
			},
			{
				CorrelationID: "discovery-chain:" + fooUID.String(),
				Result: &structs.DiscoveryChainResponse{
					Chain: fooChain,
				},
			},
			{
				CorrelationID: "upstream-target:" + webChain.ID() + ":" + webUID.String(),
				Result: &structs.IndexedCheckServiceNodes{
					Nodes: TestUpstreamNodes(t, "web"),
				},
			},
			{
				CorrelationID: "upstream-target:" + fooChain.ID() + ":" + fooUID.String(),
				Result: &structs.IndexedCheckServiceNodes{
					Nodes: TestUpstreamNodes(t, "foo"),
				},
			},
		}
	}

	return TestConfigSnapshotIngressGateway(t, populateServices, "http", "chain-and-router", nil, configFn, updates)
}

func TestConfigSnapshotIngressGateway_TLSMinVersionListenersGatewayDefaults(t testing.T) *ConfigSnapshot {
	var (
		s1      = structs.NewServiceName("s1", nil)
		s1UID   = NewUpstreamIDFromServiceName(s1)
		s1Chain = discoverychain.TestCompileConfigEntries(t, "s1", "default", "default", "dc1", connect.TestClusterID+".consul", nil, nil)

		s2      = structs.NewServiceName("s2", nil)
		s2UID   = NewUpstreamIDFromServiceName(s2)
		s2Chain = discoverychain.TestCompileConfigEntries(t, "s2", "default", "default", "dc1", connect.TestClusterID+".consul", nil, nil)

		s3      = structs.NewServiceName("s3", nil)
		s3UID   = NewUpstreamIDFromServiceName(s3)
		s3Chain = discoverychain.TestCompileConfigEntries(t, "s3", "default", "default", "dc1", connect.TestClusterID+".consul", nil, nil)

		s4      = structs.NewServiceName("s4", nil)
		s4UID   = NewUpstreamIDFromServiceName(s4)
		s4Chain = discoverychain.TestCompileConfigEntries(t, "s4", "default", "default", "dc1", connect.TestClusterID+".consul", nil, nil)
	)

	return TestConfigSnapshotIngressGateway(t, true, "tcp", "default", nil,
		func(entry *structs.IngressGatewayConfigEntry) {
			entry.TLS.Enabled = true
			entry.TLS.TLSMinVersion = types.TLSv1_2

			// One listener disables TLS, one inherits TLS minimum version from the gateway
			// config, two others set different versions
			entry.Listeners = []structs.IngressListener{
				// Omits listener TLS config, should default to gateway TLS config
				{
					Port:     8080,
					Protocol: "http",
					Services: []structs.IngressService{
						{Name: "s1"},
					},
				},
				// Explicitly sets listener TLS config to nil, should default to gateway TLS config
				{
					Port:     8081,
					Protocol: "http",
					Services: []structs.IngressService{
						{Name: "s2"},
					},
					TLS: nil,
				},
				// Explicitly enables TLS config, but with no listener default TLS params,
				// should default to gateway TLS config
				{
					Port:     8082,
					Protocol: "http",
					Services: []structs.IngressService{
						{Name: "s3"},
					},
					TLS: &structs.GatewayTLSConfig{
						Enabled: true,
					},
				},
				// Explicitly unset gateway default TLS min version in favor of proxy default
				{
					Port:     8083,
					Protocol: "http",
					Services: []structs.IngressService{
						{Name: "s3"},
					},
					TLS: &structs.GatewayTLSConfig{
						Enabled:       true,
						TLSMinVersion: types.TLSVersionAuto,
					},
				},
				// Disables listener TLS
				{
					Port:     8084,
					Protocol: "http",
					Services: []structs.IngressService{
						{Name: "s4"},
					},
					TLS: &structs.GatewayTLSConfig{
						Enabled: false,
					},
				},
			}
		}, []UpdateEvent{
			{
				CorrelationID: gatewayServicesWatchID,
				Result: &structs.IndexedGatewayServices{
					// One listener disables TLS, one inherits TLS minimum version from the gateway
					// config, two others set different versions
					Services: []*structs.GatewayService{
						{
							Service:  s1,
							Port:     8080,
							Protocol: "http",
						},
						{
							Service:  s2,
							Port:     8081,
							Protocol: "http",
						},
						{
							Service:  s3,
							Port:     8082,
							Protocol: "http",
						},
						{
							Service:  s4,
							Port:     8083,
							Protocol: "http",
						},
						{
							Service:  s4,
							Port:     8084,
							Protocol: "http",
						},
					},
				},
			},
			{
				CorrelationID: "discovery-chain:" + s1UID.String(),
				Result: &structs.DiscoveryChainResponse{
					Chain: s1Chain,
				},
			},
			{
				CorrelationID: "discovery-chain:" + s2UID.String(),
				Result: &structs.DiscoveryChainResponse{
					Chain: s2Chain,
				},
			},
			{
				CorrelationID: "discovery-chain:" + s3UID.String(),
				Result: &structs.DiscoveryChainResponse{
					Chain: s3Chain,
				},
			},
			{
				CorrelationID: "discovery-chain:" + s4UID.String(),
				Result: &structs.DiscoveryChainResponse{
					Chain: s4Chain,
				},
			},
			{
				CorrelationID: "upstream-target:" + s1Chain.ID() + ":" + s1UID.String(),
				Result: &structs.IndexedCheckServiceNodes{
					Nodes: TestUpstreamNodes(t, "s1"),
				},
			},
			{
				CorrelationID: "upstream-target:" + s2Chain.ID() + ":" + s2UID.String(),
				Result: &structs.IndexedCheckServiceNodes{
					Nodes: TestUpstreamNodes(t, "s2"),
				},
			},
			{
				CorrelationID: "upstream-target:" + s3Chain.ID() + ":" + s3UID.String(),
				Result: &structs.IndexedCheckServiceNodes{
					Nodes: TestUpstreamNodes(t, "s3"),
				},
			},
			{
				CorrelationID: "upstream-target:" + s4Chain.ID() + ":" + s4UID.String(),
				Result: &structs.IndexedCheckServiceNodes{
					Nodes: TestUpstreamNodes(t, "s4"),
				},
			},
		})
}

func TestConfigSnapshotIngressGateway_SingleTLSListener(t testing.T) *ConfigSnapshot {
	var (
		s1      = structs.NewServiceName("s1", nil)
		s1UID   = NewUpstreamIDFromServiceName(s1)
		s1Chain = discoverychain.TestCompileConfigEntries(t, "s1", "default", "default", "dc1", connect.TestClusterID+".consul", nil, nil)

		s2      = structs.NewServiceName("s2", nil)
		s2UID   = NewUpstreamIDFromServiceName(s2)
		s2Chain = discoverychain.TestCompileConfigEntries(t, "s2", "default", "default", "dc1", connect.TestClusterID+".consul", nil, nil)
	)
	return TestConfigSnapshotIngressGateway(t, true, "tcp", "simple", nil,
		func(entry *structs.IngressGatewayConfigEntry) {
			entry.Listeners = []structs.IngressListener{
				{
					Port:     8080,
					Protocol: "http",
					Services: []structs.IngressService{
						{Name: "s1"},
					},
				},
				{
					Port:     8081,
					Protocol: "http",
					Services: []structs.IngressService{
						{Name: "s2"},
					},
					TLS: &structs.GatewayTLSConfig{
						Enabled:       true,
						TLSMinVersion: types.TLSv1_2,
					},
				},
			}
		}, []UpdateEvent{
			{
				CorrelationID: gatewayServicesWatchID,
				Result: &structs.IndexedGatewayServices{
					// One listener should inherit non-TLS gateway config, another
					// listener configures TLS with an explicit minimum version
					Services: []*structs.GatewayService{
						{
							Service:  s1,
							Port:     8080,
							Protocol: "http",
						},
						{
							Service:  s2,
							Port:     8081,
							Protocol: "http",
						},
					},
				},
			},
			{
				CorrelationID: "discovery-chain:" + s1UID.String(),
				Result: &structs.DiscoveryChainResponse{
					Chain: s1Chain,
				},
			},
			{
				CorrelationID: "discovery-chain:" + s2UID.String(),
				Result: &structs.DiscoveryChainResponse{
					Chain: s2Chain,
				},
			},
			{
				CorrelationID: "upstream-target:" + s1Chain.ID() + ":" + s1UID.String(),
				Result: &structs.IndexedCheckServiceNodes{
					Nodes: TestUpstreamNodes(t, "s1"),
				},
			},
			{
				CorrelationID: "upstream-target:" + s2Chain.ID() + ":" + s2UID.String(),
				Result: &structs.IndexedCheckServiceNodes{
					Nodes: TestUpstreamNodes(t, "s2"),
				},
			},
		})
}

func TestConfigSnapshotIngressGateway_SingleTLSListener_GRPC(t testing.T) *ConfigSnapshot {
	var (
		s1      = structs.NewServiceName("s1", nil)
		s1UID   = NewUpstreamIDFromServiceName(s1)
		s1Chain = discoverychain.TestCompileConfigEntries(t, "s1", "default", "default", "dc1", connect.TestClusterID+".consul", nil, nil)

		s2      = structs.NewServiceName("s2", nil)
		s2UID   = NewUpstreamIDFromServiceName(s2)
		s2Chain = discoverychain.TestCompileConfigEntries(t, "s2", "default", "default", "dc1", connect.TestClusterID+".consul", nil, nil)
	)
	return TestConfigSnapshotIngressGateway(t, true, "grpc", "simple", nil,
		func(entry *structs.IngressGatewayConfigEntry) {
			entry.Listeners = []structs.IngressListener{
				{
					Port:     8080,
					Protocol: "grpc",
					Services: []structs.IngressService{
						{Name: "s1"},
					},
				},
				{
					Port:     8081,
					Protocol: "grpc",
					Services: []structs.IngressService{
						{Name: "s2"},
					},
					TLS: &structs.GatewayTLSConfig{
						Enabled:       true,
						TLSMinVersion: types.TLSv1_2,
					},
				},
			}
		}, []UpdateEvent{
			{
				CorrelationID: gatewayServicesWatchID,
				Result: &structs.IndexedGatewayServices{
					// One listener should inherit non-TLS gateway config, another
					// listener configures TLS with an explicit minimum version
					Services: []*structs.GatewayService{
						{
							Service:  s1,
							Port:     8080,
							Protocol: "grpc",
						},
						{
							Service:  s2,
							Port:     8081,
							Protocol: "grpc",
						},
					},
				},
			},
			{
				CorrelationID: "discovery-chain:" + s1UID.String(),
				Result: &structs.DiscoveryChainResponse{
					Chain: s1Chain,
				},
			},
			{
				CorrelationID: "discovery-chain:" + s2UID.String(),
				Result: &structs.DiscoveryChainResponse{
					Chain: s2Chain,
				},
			},
			{
				CorrelationID: "upstream-target:" + s1Chain.ID() + ":" + s1UID.String(),
				Result: &structs.IndexedCheckServiceNodes{
					Nodes: TestUpstreamNodes(t, "s1"),
				},
			},
			{
				CorrelationID: "upstream-target:" + s2Chain.ID() + ":" + s2UID.String(),
				Result: &structs.IndexedCheckServiceNodes{
					Nodes: TestUpstreamNodes(t, "s2"),
				},
			},
		})
}

func TestConfigSnapshotIngressGateway_SingleTLSListener_HTTP2(t testing.T) *ConfigSnapshot {
	var (
		s1      = structs.NewServiceName("s1", nil)
		s1UID   = NewUpstreamIDFromServiceName(s1)
		s1Chain = discoverychain.TestCompileConfigEntries(t, "s1", "default", "default", "dc1", connect.TestClusterID+".consul", nil, nil)

		s2      = structs.NewServiceName("s2", nil)
		s2UID   = NewUpstreamIDFromServiceName(s2)
		s2Chain = discoverychain.TestCompileConfigEntries(t, "s2", "default", "default", "dc1", connect.TestClusterID+".consul", nil, nil)
	)
	return TestConfigSnapshotIngressGateway(t, true, "http2", "simple", nil,
		func(entry *structs.IngressGatewayConfigEntry) {
			entry.Listeners = []structs.IngressListener{
				{
					Port:     8080,
					Protocol: "http2",
					Services: []structs.IngressService{
						{Name: "s1"},
					},
				},
				{
					Port:     8081,
					Protocol: "http2",
					Services: []structs.IngressService{
						{Name: "s2"},
					},
					TLS: &structs.GatewayTLSConfig{
						Enabled:       true,
						TLSMinVersion: types.TLSv1_2,
					},
				},
			}
		}, []UpdateEvent{
			{
				CorrelationID: gatewayServicesWatchID,
				Result: &structs.IndexedGatewayServices{
					// One listener should inherit non-TLS gateway config, another
					// listener configures TLS with an explicit minimum version
					Services: []*structs.GatewayService{
						{
							Service:  s1,
							Port:     8080,
							Protocol: "http2",
						},
						{
							Service:  s2,
							Port:     8081,
							Protocol: "http2",
						},
					},
				},
			},
			{
				CorrelationID: "discovery-chain:" + s1UID.String(),
				Result: &structs.DiscoveryChainResponse{
					Chain: s1Chain,
				},
			},
			{
				CorrelationID: "discovery-chain:" + s2UID.String(),
				Result: &structs.DiscoveryChainResponse{
					Chain: s2Chain,
				},
			},
			{
				CorrelationID: "upstream-target:" + s1Chain.ID() + ":" + s1UID.String(),
				Result: &structs.IndexedCheckServiceNodes{
					Nodes: TestUpstreamNodes(t, "s1"),
				},
			},
			{
				CorrelationID: "upstream-target:" + s2Chain.ID() + ":" + s2UID.String(),
				Result: &structs.IndexedCheckServiceNodes{
					Nodes: TestUpstreamNodes(t, "s2"),
				},
			},
		})
}

func TestConfigSnapshotIngressGateway_MultiTLSListener_MixedHTTP2gRPC(t testing.T) *ConfigSnapshot {
	var (
		s1      = structs.NewServiceName("s1", nil)
		s1UID   = NewUpstreamIDFromServiceName(s1)
		s1Chain = discoverychain.TestCompileConfigEntries(t, "s1", "default", "default", "dc1", connect.TestClusterID+".consul", nil, nil)

		s2      = structs.NewServiceName("s2", nil)
		s2UID   = NewUpstreamIDFromServiceName(s2)
		s2Chain = discoverychain.TestCompileConfigEntries(t, "s2", "default", "default", "dc1", connect.TestClusterID+".consul", nil, nil)
	)
	return TestConfigSnapshotIngressGateway(t, true, "tcp", "simple", nil,
		func(entry *structs.IngressGatewayConfigEntry) {
			entry.Listeners = []structs.IngressListener{
				{
					Port:     8080,
					Protocol: "grpc",
					Services: []structs.IngressService{
						{Name: "s1"},
					},
					TLS: &structs.GatewayTLSConfig{
						Enabled:       true,
						TLSMinVersion: types.TLSv1_2,
					},
				},
				{
					Port:     8081,
					Protocol: "http2",
					Services: []structs.IngressService{
						{Name: "s2"},
					},
					TLS: &structs.GatewayTLSConfig{
						Enabled:       true,
						TLSMinVersion: types.TLSv1_2,
					},
				},
			}
		}, []UpdateEvent{
			{
				CorrelationID: gatewayServicesWatchID,
				Result: &structs.IndexedGatewayServices{
					// One listener should inherit non-TLS gateway config, another
					// listener configures TLS with an explicit minimum version
					Services: []*structs.GatewayService{
						{
							Service:  s1,
							Port:     8080,
							Protocol: "grpc",
						},
						{
							Service:  s2,
							Port:     8081,
							Protocol: "http2",
						},
					},
				},
			},
			{
				CorrelationID: "discovery-chain:" + s1UID.String(),
				Result: &structs.DiscoveryChainResponse{
					Chain: s1Chain,
				},
			},
			{
				CorrelationID: "discovery-chain:" + s2UID.String(),
				Result: &structs.DiscoveryChainResponse{
					Chain: s2Chain,
				},
			},
			{
				CorrelationID: "upstream-target:" + s1Chain.ID() + ":" + s1UID.String(),
				Result: &structs.IndexedCheckServiceNodes{
					Nodes: TestUpstreamNodes(t, "s1"),
				},
			},
			{
				CorrelationID: "upstream-target:" + s2Chain.ID() + ":" + s2UID.String(),
				Result: &structs.IndexedCheckServiceNodes{
					Nodes: TestUpstreamNodes(t, "s2"),
				},
			},
		})
}

func TestConfigSnapshotIngressGateway_GWTLSListener_MixedHTTP2gRPC(t testing.T) *ConfigSnapshot {
	var (
		s1      = structs.NewServiceName("s1", nil)
		s1UID   = NewUpstreamIDFromServiceName(s1)
		s1Chain = discoverychain.TestCompileConfigEntries(t, "s1", "default", "default", "dc1", connect.TestClusterID+".consul", nil, nil)

		s2      = structs.NewServiceName("s2", nil)
		s2UID   = NewUpstreamIDFromServiceName(s2)
		s2Chain = discoverychain.TestCompileConfigEntries(t, "s2", "default", "default", "dc1", connect.TestClusterID+".consul", nil, nil)
	)
	return TestConfigSnapshotIngressGateway(t, true, "tcp", "simple", nil,
		func(entry *structs.IngressGatewayConfigEntry) {
			entry.TLS = structs.GatewayTLSConfig{
				Enabled:       true,
				TLSMinVersion: types.TLSv1_2,
			}
			entry.Listeners = []structs.IngressListener{
				{
					Port:     8080,
					Protocol: "grpc",
					Services: []structs.IngressService{
						{Name: "s1"},
					},
				},
				{
					Port:     8081,
					Protocol: "http2",
					Services: []structs.IngressService{
						{Name: "s2"},
					},
				},
			}
		}, []UpdateEvent{
			{
				CorrelationID: gatewayServicesWatchID,
				Result: &structs.IndexedGatewayServices{
					// One listener should inherit non-TLS gateway config, another
					// listener configures TLS with an explicit minimum version
					Services: []*structs.GatewayService{
						{
							Service:  s1,
							Port:     8080,
							Protocol: "grpc",
						},
						{
							Service:  s2,
							Port:     8081,
							Protocol: "http2",
						},
					},
				},
			},
			{
				CorrelationID: "discovery-chain:" + s1UID.String(),
				Result: &structs.DiscoveryChainResponse{
					Chain: s1Chain,
				},
			},
			{
				CorrelationID: "discovery-chain:" + s2UID.String(),
				Result: &structs.DiscoveryChainResponse{
					Chain: s2Chain,
				},
			},
			{
				CorrelationID: "upstream-target:" + s1Chain.ID() + ":" + s1UID.String(),
				Result: &structs.IndexedCheckServiceNodes{
					Nodes: TestUpstreamNodes(t, "s1"),
				},
			},
			{
				CorrelationID: "upstream-target:" + s2Chain.ID() + ":" + s2UID.String(),
				Result: &structs.IndexedCheckServiceNodes{
					Nodes: TestUpstreamNodes(t, "s2"),
				},
			},
		})
}

func TestConfigSnapshotIngressGateway_TLSMixedMinVersionListeners(t testing.T) *ConfigSnapshot {
	var (
		s1      = structs.NewServiceName("s1", nil)
		s1UID   = NewUpstreamIDFromServiceName(s1)
		s1Chain = discoverychain.TestCompileConfigEntries(t, "s1", "default", "default", "dc1", connect.TestClusterID+".consul", nil, nil)

		s2      = structs.NewServiceName("s2", nil)
		s2UID   = NewUpstreamIDFromServiceName(s2)
		s2Chain = discoverychain.TestCompileConfigEntries(t, "s2", "default", "default", "dc1", connect.TestClusterID+".consul", nil, nil)

		s3      = structs.NewServiceName("s3", nil)
		s3UID   = NewUpstreamIDFromServiceName(s3)
		s3Chain = discoverychain.TestCompileConfigEntries(t, "s3", "default", "default", "dc1", connect.TestClusterID+".consul", nil, nil)
	)

	return TestConfigSnapshotIngressGateway(t, true, "tcp", "default", nil,
		func(entry *structs.IngressGatewayConfigEntry) {
			entry.TLS.Enabled = true
			entry.TLS.TLSMinVersion = types.TLSv1_2

			// One listener should inherit TLS minimum version from the gateway config,
			// two others each set explicit TLS minimum versions
			entry.Listeners = []structs.IngressListener{
				{
					Port:     8080,
					Protocol: "http",
					Services: []structs.IngressService{
						{Name: "s1"},
					},
				},
				{
					Port:     8081,
					Protocol: "http",
					Services: []structs.IngressService{
						{Name: "s2"},
					},
					TLS: &structs.GatewayTLSConfig{
						Enabled:       true,
						TLSMinVersion: types.TLSv1_0,
					},
				},
				{
					Port:     8082,
					Protocol: "http",
					Services: []structs.IngressService{
						{Name: "s3"},
					},
					TLS: &structs.GatewayTLSConfig{
						Enabled:       true,
						TLSMinVersion: types.TLSv1_3,
					},
				},
			}
		}, []UpdateEvent{
			{
				CorrelationID: gatewayServicesWatchID,
				Result: &structs.IndexedGatewayServices{
					Services: []*structs.GatewayService{
						{
							Service:  s1,
							Port:     8080,
							Protocol: "http",
						},
						{
							Service:  s2,
							Port:     8081,
							Protocol: "http",
						},
						{
							Service:  s3,
							Port:     8082,
							Protocol: "http",
						},
					},
				},
			},
			{
				CorrelationID: "discovery-chain:" + s1UID.String(),
				Result: &structs.DiscoveryChainResponse{
					Chain: s1Chain,
				},
			},
			{
				CorrelationID: "discovery-chain:" + s2UID.String(),
				Result: &structs.DiscoveryChainResponse{
					Chain: s2Chain,
				},
			},
			{
				CorrelationID: "discovery-chain:" + s3UID.String(),
				Result: &structs.DiscoveryChainResponse{
					Chain: s3Chain,
				},
			},
			{
				CorrelationID: "upstream-target:" + s1Chain.ID() + ":" + s1UID.String(),
				Result: &structs.IndexedCheckServiceNodes{
					Nodes: TestUpstreamNodes(t, "s1"),
				},
			},
			{
				CorrelationID: "upstream-target:" + s2Chain.ID() + ":" + s2UID.String(),
				Result: &structs.IndexedCheckServiceNodes{
					Nodes: TestUpstreamNodes(t, "s2"),
				},
			},
			{
				CorrelationID: "upstream-target:" + s3Chain.ID() + ":" + s3UID.String(),
				Result: &structs.IndexedCheckServiceNodes{
					Nodes: TestUpstreamNodes(t, "s3"),
				},
			},
		})
}

func TestConfigSnapshotIngressGateway_TLSMixedMaxVersionListeners(t testing.T) *ConfigSnapshot {
	var (
		s1      = structs.NewServiceName("s1", nil)
		s1UID   = NewUpstreamIDFromServiceName(s1)
		s1Chain = discoverychain.TestCompileConfigEntries(t, "s1", "default", "default", "dc1", connect.TestClusterID+".consul", nil, nil)

		s2      = structs.NewServiceName("s2", nil)
		s2UID   = NewUpstreamIDFromServiceName(s2)
		s2Chain = discoverychain.TestCompileConfigEntries(t, "s2", "default", "default", "dc1", connect.TestClusterID+".consul", nil, nil)

		s3      = structs.NewServiceName("s3", nil)
		s3UID   = NewUpstreamIDFromServiceName(s3)
		s3Chain = discoverychain.TestCompileConfigEntries(t, "s3", "default", "default", "dc1", connect.TestClusterID+".consul", nil, nil)
	)

	return TestConfigSnapshotIngressGateway(t, true, "tcp", "default", nil,
		func(entry *structs.IngressGatewayConfigEntry) {
			entry.TLS.Enabled = true
			entry.TLS.TLSMaxVersion = types.TLSv1_2

			// One listener should inherit TLS maximum version from the gateway config,
			// two others each set explicit TLS maximum versions
			entry.Listeners = []structs.IngressListener{
				{
					Port:     8080,
					Protocol: "http",
					Services: []structs.IngressService{
						{Name: "s1"},
					},
				},
				{
					Port:     8081,
					Protocol: "http",
					Services: []structs.IngressService{
						{Name: "s2"},
					},
					TLS: &structs.GatewayTLSConfig{
						Enabled:       true,
						TLSMaxVersion: types.TLSv1_0,
					},
				},
				{
					Port:     8082,
					Protocol: "http",
					Services: []structs.IngressService{
						{Name: "s3"},
					},
					TLS: &structs.GatewayTLSConfig{
						Enabled:       true,
						TLSMaxVersion: types.TLSv1_3,
					},
				},
			}
		}, []UpdateEvent{
			{
				CorrelationID: gatewayServicesWatchID,
				Result: &structs.IndexedGatewayServices{
					Services: []*structs.GatewayService{
						{
							Service:  s1,
							Port:     8080,
							Protocol: "http",
						},
						{
							Service:  s2,
							Port:     8081,
							Protocol: "http",
						},
						{
							Service:  s3,
							Port:     8082,
							Protocol: "http",
						},
					},
				},
			},
			{
				CorrelationID: "discovery-chain:" + s1UID.String(),
				Result: &structs.DiscoveryChainResponse{
					Chain: s1Chain,
				},
			},
			{
				CorrelationID: "discovery-chain:" + s2UID.String(),
				Result: &structs.DiscoveryChainResponse{
					Chain: s2Chain,
				},
			},
			{
				CorrelationID: "discovery-chain:" + s3UID.String(),
				Result: &structs.DiscoveryChainResponse{
					Chain: s3Chain,
				},
			},
			{
				CorrelationID: "upstream-target:" + s1Chain.ID() + ":" + s1UID.String(),
				Result: &structs.IndexedCheckServiceNodes{
					Nodes: TestUpstreamNodes(t, "s1"),
				},
			},
			{
				CorrelationID: "upstream-target:" + s2Chain.ID() + ":" + s2UID.String(),
				Result: &structs.IndexedCheckServiceNodes{
					Nodes: TestUpstreamNodes(t, "s2"),
				},
			},
			{
				CorrelationID: "upstream-target:" + s3Chain.ID() + ":" + s3UID.String(),
				Result: &structs.IndexedCheckServiceNodes{
					Nodes: TestUpstreamNodes(t, "s3"),
				},
			},
		})
}

func TestConfigSnapshotIngressGateway_TLSMixedCipherVersionListeners(t testing.T) *ConfigSnapshot {
	var (
		s1      = structs.NewServiceName("s1", nil)
		s1UID   = NewUpstreamIDFromServiceName(s1)
		s1Chain = discoverychain.TestCompileConfigEntries(t, "s1", "default", "default", "dc1", connect.TestClusterID+".consul", nil, nil)

		s2      = structs.NewServiceName("s2", nil)
		s2UID   = NewUpstreamIDFromServiceName(s2)
		s2Chain = discoverychain.TestCompileConfigEntries(t, "s2", "default", "default", "dc1", connect.TestClusterID+".consul", nil, nil)
	)

	return TestConfigSnapshotIngressGateway(t, true, "tcp", "default", nil,
		func(entry *structs.IngressGatewayConfigEntry) {
			entry.TLS.Enabled = true
			entry.TLS.CipherSuites = []types.TLSCipherSuite{
				types.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			}

			// One listener should inherit TLS Ciphers from the gateway config,
			// the other should be set explicitly from the listener config
			entry.Listeners = []structs.IngressListener{
				{
					Port:     8080,
					Protocol: "http",
					Services: []structs.IngressService{
						{Name: "s1"},
					},
				},
				{
					Port:     8081,
					Protocol: "http",
					Services: []structs.IngressService{
						{Name: "s2"},
					},
					TLS: &structs.GatewayTLSConfig{
						Enabled: true,
						CipherSuites: []types.TLSCipherSuite{
							types.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
							types.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
						},
					},
				},
			}
		}, []UpdateEvent{
			{
				CorrelationID: gatewayServicesWatchID,
				Result: &structs.IndexedGatewayServices{
					// One listener should inherit TLS minimum version from the gateway config,
					// two others each set explicit TLS minimum versions
					Services: []*structs.GatewayService{
						{
							Service:  s1,
							Port:     8080,
							Protocol: "http",
						},
						{
							Service:  s2,
							Port:     8081,
							Protocol: "http",
						},
					},
				},
			},
			{
				CorrelationID: "discovery-chain:" + s1UID.String(),
				Result: &structs.DiscoveryChainResponse{
					Chain: s1Chain,
				},
			},
			{
				CorrelationID: "discovery-chain:" + s2UID.String(),
				Result: &structs.DiscoveryChainResponse{
					Chain: s2Chain,
				},
			},
			{
				CorrelationID: "upstream-target:" + s1Chain.ID() + ":" + s1UID.String(),
				Result: &structs.IndexedCheckServiceNodes{
					Nodes: TestUpstreamNodes(t, "s1"),
				},
			},
			{
				CorrelationID: "upstream-target:" + s2Chain.ID() + ":" + s2UID.String(),
				Result: &structs.IndexedCheckServiceNodes{
					Nodes: TestUpstreamNodes(t, "s2"),
				},
			},
		})
}
