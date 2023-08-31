package proxycfg

import (
	"fmt"
	"time"

	"github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/assert"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/types"
)

// TestConfigSnapshot returns a fully populated snapshot
func TestConfigSnapshot(t testing.T, nsFn func(ns *structs.NodeService), extraUpdates []UpdateEvent) *ConfigSnapshot {
	roots, leaf := TestCerts(t)

	// no entries implies we'll get a default chain
	dbChain := discoverychain.TestCompileConfigEntries(t, "db", "default", "default", "dc1", connect.TestClusterID+".consul", nil)
	assert.True(t, dbChain.Default)

	var (
		upstreams   = structs.TestUpstreams(t, false)
		dbUpstream  = upstreams[0]
		geoUpstream = upstreams[1]

		dbUID  = NewUpstreamID(&dbUpstream)
		geoUID = NewUpstreamID(&geoUpstream)

		webSN = structs.ServiceIDString("web", nil)
	)

	baseEvents := []UpdateEvent{
		{
			CorrelationID: rootsWatchID,
			Result:        roots,
		},
		{
			CorrelationID: leafWatchID,
			Result:        leaf,
		},
		{
			CorrelationID: intentionsWatchID,
			Result:        structs.Intentions{}, // no intentions defined
		},
		{
			CorrelationID: svcChecksWatchIDPrefix + webSN,
			Result:        []structs.CheckType{},
		},
		{
			CorrelationID: "upstream:" + geoUID.String(),
			Result: &structs.PreparedQueryExecuteResponse{
				Nodes: TestPreparedQueryNodes(t, "geo-cache"),
			},
		},
		{
			CorrelationID: "discovery-chain:" + dbUID.String(),
			Result: &structs.DiscoveryChainResponse{
				Chain: dbChain,
			},
		},
		{
			CorrelationID: "upstream-target:" + dbChain.ID() + ":" + dbUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: TestUpstreamNodes(t, "db"),
			},
		},
	}

	return testConfigSnapshotFixture(t, &structs.NodeService{
		Kind:    structs.ServiceKindConnectProxy,
		Service: "web-sidecar-proxy",
		Port:    9999,
		Proxy: structs.ConnectProxyConfig{
			DestinationServiceID:   "web",
			DestinationServiceName: "web",
			LocalServiceAddress:    "127.0.0.1",
			LocalServicePort:       8080,
			Config: map[string]interface{}{
				"foo": "bar",
			},
			Upstreams: upstreams,
		},
		Meta:            nil,
		TaggedAddresses: nil,
	}, nsFn, nil, testSpliceEvents(baseEvents, extraUpdates))
}

// TestConfigSnapshotDiscoveryChain returns a fully populated snapshot using a discovery chain
func TestConfigSnapshotDiscoveryChain(
	t testing.T,
	variation string,
	enterprise bool,
	nsFn func(ns *structs.NodeService),
	extraUpdates []UpdateEvent,
	additionalEntries ...structs.ConfigEntry,
) *ConfigSnapshot {
	roots, leaf := TestCerts(t)

	var entMeta acl.EnterpriseMeta
	if enterprise {
		entMeta = acl.NewEnterpriseMetaWithPartition("ap1", "ns1")
	}

	var (
		upstreams   = structs.TestUpstreams(t, enterprise)
		geoUpstream = upstreams[1]

		geoUID = NewUpstreamID(&geoUpstream)

		webSN = structs.ServiceIDString("web", &entMeta)
	)

	baseEvents := testSpliceEvents([]UpdateEvent{
		{
			CorrelationID: rootsWatchID,
			Result:        roots,
		},
		{
			CorrelationID: leafWatchID,
			Result:        leaf,
		},
		{
			CorrelationID: intentionsWatchID,
			Result:        structs.Intentions{}, // no intentions defined
		},
		{
			CorrelationID: meshConfigEntryID,
			Result: &structs.ConfigEntryResponse{
				Entry: nil,
			},
		},
		{
			CorrelationID: svcChecksWatchIDPrefix + webSN,
			Result:        []structs.CheckType{},
		},
		{
			CorrelationID: "upstream:" + geoUID.String(),
			Result: &structs.PreparedQueryExecuteResponse{
				Nodes: TestPreparedQueryNodes(t, "geo-cache"),
			},
		},
	}, setupTestVariationConfigEntriesAndSnapshot(
		t, variation, enterprise, upstreams, additionalEntries...,
	))

	return testConfigSnapshotFixture(t, &structs.NodeService{
		Kind:    structs.ServiceKindConnectProxy,
		Service: "web-sidecar-proxy",
		Port:    9999,
		Proxy: structs.ConnectProxyConfig{
			DestinationServiceID:   "web",
			DestinationServiceName: "web",
			LocalServiceAddress:    "127.0.0.1",
			LocalServicePort:       8080,
			Config: map[string]interface{}{
				"foo": "bar",
			},
			Upstreams: upstreams,
		},
		Meta:            nil,
		TaggedAddresses: nil,
		EnterpriseMeta:  entMeta,
	}, nsFn, nil, testSpliceEvents(baseEvents, extraUpdates))
}

func TestConfigSnapshotExposeConfig(t testing.T, nsFn func(ns *structs.NodeService)) *ConfigSnapshot {
	roots, leaf := TestCerts(t)

	var (
		webSN = structs.ServiceIDString("web", nil)
	)

	baseEvents := []UpdateEvent{
		{
			CorrelationID: rootsWatchID,
			Result:        roots,
		},
		{
			CorrelationID: leafWatchID, Result: leaf,
		},
		{
			CorrelationID: intentionsWatchID,
			Result:        structs.Intentions{}, // no intentions defined
		},
		{
			CorrelationID: svcChecksWatchIDPrefix + webSN,
			Result:        []structs.CheckType{},
		},
	}

	return testConfigSnapshotFixture(t, &structs.NodeService{
		Kind:    structs.ServiceKindConnectProxy,
		Service: "web-sidecar-proxy",
		Address: "1.2.3.4",
		Port:    8080,
		Proxy: structs.ConnectProxyConfig{
			DestinationServiceID:   "web",
			DestinationServiceName: "web",
			LocalServicePort:       8080,
			Expose: structs.ExposeConfig{
				Checks: false,
				Paths: []structs.ExposePath{
					{
						LocalPathPort: 8080,
						Path:          "/health1",
						ListenerPort:  21500,
					},
					{
						LocalPathPort: 8080,
						Path:          "/health2",
						ListenerPort:  21501,
					},
				},
			},
		},
		Meta:            nil,
		TaggedAddresses: nil,
	}, nsFn, nil, baseEvents)
}

func TestConfigSnapshotExposeChecks(t testing.T) *ConfigSnapshot {
	return TestConfigSnapshot(t,
		func(ns *structs.NodeService) {
			ns.Address = "1.2.3.4"
			ns.Port = 8080
			ns.Proxy.Upstreams = nil
			ns.Proxy.Expose = structs.ExposeConfig{
				Checks: true,
			}
		},
		[]UpdateEvent{
			{
				CorrelationID: svcChecksWatchIDPrefix + structs.ServiceIDString("web", nil),
				Result: []structs.CheckType{{
					CheckID:   types.CheckID("http"),
					Name:      "http",
					HTTP:      "http://127.0.0.1:8181/debug",
					ProxyHTTP: "http://:21500/debug",
					Method:    "GET",
					Interval:  10 * time.Second,
					Timeout:   1 * time.Second,
				}},
			},
		},
	)
}

func TestConfigSnapshotGRPCExposeHTTP1(t testing.T) *ConfigSnapshot {
	roots, leaf := TestCerts(t)

	return testConfigSnapshotFixture(t, &structs.NodeService{
		Kind:    structs.ServiceKindConnectProxy,
		Service: "grpc-proxy",
		Address: "1.2.3.4",
		Port:    8080,
		Proxy: structs.ConnectProxyConfig{
			DestinationServiceName: "grpc",
			DestinationServiceID:   "grpc",
			LocalServicePort:       8080,
			Config: map[string]interface{}{
				"protocol": "grpc",
			},
			Expose: structs.ExposeConfig{
				Checks: false,
				Paths: []structs.ExposePath{
					{
						LocalPathPort: 8090,
						Path:          "/healthz",
						ListenerPort:  21500,
						Protocol:      "http",
					},
				},
			},
		},
		Meta:            nil,
		TaggedAddresses: nil,
	}, nil, nil, []UpdateEvent{
		{
			CorrelationID: rootsWatchID,
			Result:        roots,
		},
		{
			CorrelationID: leafWatchID,
			Result:        leaf,
		},
		{
			CorrelationID: intentionsWatchID,
			Result:        structs.Intentions{}, // no intentions defined
		},
		{
			CorrelationID: svcChecksWatchIDPrefix + structs.ServiceIDString("grpc", nil),
			Result:        []structs.CheckType{},
		},
	})
}

// TestConfigSnapshotTelemetryCollector returns a fully populated snapshot using a discovery chain
func TestConfigSnapshotTelemetryCollector(t testing.T) *ConfigSnapshot {
	// DiscoveryChain without an UpstreamConfig should yield a
	// filter chain when in transparent proxy mode
	var (
		collector      = structs.NewServiceName(api.TelemetryCollectorName, nil)
		collectorUID   = NewUpstreamIDFromServiceName(collector)
		collectorChain = discoverychain.TestCompileConfigEntries(t, api.TelemetryCollectorName, "default", "default", "dc1", connect.TestClusterID+".consul", nil)
	)

	return TestConfigSnapshot(t, func(ns *structs.NodeService) {
		ns.Proxy.Config = map[string]interface{}{
			"envoy_telemetry_collector_bind_socket_dir": "/tmp/consul/telemetry-collector",
		}
	}, []UpdateEvent{
		{
			CorrelationID: meshConfigEntryID,
			Result: &structs.ConfigEntryResponse{
				Entry: nil,
			},
		},
		{
			CorrelationID: "discovery-chain:" + collectorUID.String(),
			Result: &structs.DiscoveryChainResponse{
				Chain: collectorChain,
			},
		},
		{
			CorrelationID: fmt.Sprintf("upstream-target:%s.default.default.dc1:", api.TelemetryCollectorName) + collectorUID.String(),
			Result: &structs.IndexedCheckServiceNodes{
				Nodes: []structs.CheckServiceNode{
					{
						Node: &structs.Node{
							Address:    "8.8.8.8",
							Datacenter: "dc1",
						},
						Service: &structs.NodeService{
							Service: api.TelemetryCollectorName,
							Address: "9.9.9.9",
							Port:    9090,
						},
					},
				},
			},
		},
	})
}
