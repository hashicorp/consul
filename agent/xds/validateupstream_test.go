package xds

import (
	"testing"

	envoy_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	"github.com/hashicorp/consul/agent/structs"
	testinf "github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/xds/proxysupport"
	"github.com/hashicorp/consul/agent/xds/validateupstream"
	"github.com/hashicorp/consul/agent/xds/xdscommon"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestValidateUpstreams(t *testing.T) {
	sni := "db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul"
	listenerName := "db:127.0.0.1:9191"
	httpServiceDefaults := &structs.ServiceConfigEntry{
		Kind:     structs.ServiceDefaults,
		Name:     "db",
		Protocol: "http",
	}

	splitter := &structs.ServiceSplitterConfigEntry{
		Kind: structs.ServiceSplitter,
		Name: "db",
		Splits: []structs.ServiceSplit{
			{
				Weight:        50,
				Service:       "db",
				ServiceSubset: "v1",
			},
			{
				Weight:        50,
				Service:       "db",
				ServiceSubset: "v2",
			},
		},
	}
	resolver := &structs.ServiceResolverConfigEntry{
		Kind: structs.ServiceResolver,
		Name: "db",
		Subsets: map[string]structs.ServiceResolverSubset{
			"v1": {Filter: "Service.Meta.version == v1"},
			"v2": {Filter: "Service.Meta.version == v2"},
		},
	}
	dbUID := proxycfg.NewUpstreamID(&structs.Upstream{
		DestinationName: "db",
		LocalBindPort:   9191,
	})
	nodes := proxycfg.TestUpstreamNodes(t, "db")

	// TODO failover. This is strange because we need to first find
	// the aggregate cluster and use that to find and validate the other clusters.
	// TODO Test tproxy.
	// TODO explicit upstreams and tproxy for the same service.
	tests := []struct {
		name    string
		create  func(t testinf.T) *proxycfg.ConfigSnapshot
		patcher func(*xdscommon.IndexedResources) *xdscommon.IndexedResources
		err     string
		dc string
		peer string
		trustDomain string
		serviceName *api.CompoundServiceName
	}{
		{
			name: "tcp-success",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "default", nil, nil)
			},
		},
		{
			name: "tcp-missing-listener",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "default", nil, nil)
			},
			patcher: func(ir *xdscommon.IndexedResources) *xdscommon.IndexedResources {
				delete(ir.Index[xdscommon.ListenerType], listenerName)
				return ir
			},
			err: "no listener",
		},
		{
			name: "tcp-missing-cluster",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "default", nil, nil)
			},
			patcher: func(ir *xdscommon.IndexedResources) *xdscommon.IndexedResources {
				delete(ir.Index[xdscommon.ClusterType], sni)
				return ir
			},
			err: "unexpected route/listener destination cluster",
		},
		{
			name: "tcp-missing-load-assignment",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "default", nil, nil)
			},
			patcher: func(ir *xdscommon.IndexedResources) *xdscommon.IndexedResources {
				delete(ir.Index[xdscommon.EndpointType], sni)
				return ir
			},
			err: "no cluster load assignment",
		},
		{
			name: "tcp-missing-eds-endpoints",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "default", nil, nil)
			},
			patcher: func(ir *xdscommon.IndexedResources) *xdscommon.IndexedResources {
				msg := ir.Index[xdscommon.EndpointType][sni]
				cla, ok := msg.(*envoy_endpoint_v3.ClusterLoadAssignment)
				require.True(t, ok)
				cla.Endpoints = nil
				ir.Index[xdscommon.EndpointType][sni] = cla
				return ir
			},
			err: "zero endpoints",
		},
		{
			name: "http-success",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "default", nil, nil, httpServiceDefaults)
			},
		},
		{
			name: "http-rds-success",
			// RDS, Envoy's Route Discovery Service, is only used for HTTP services with a customized discovery chain, so we
			// need to use the test snapshot and add L7 config entries.
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "default", nil, []proxycfg.UpdateEvent{
					// The events ensure there are endpoints for the v1 and v2 subsets.
					{
						CorrelationID: "upstream-target:v1.db.default.default.dc1:" + dbUID.String(),
						Result: &structs.IndexedCheckServiceNodes{
							Nodes: nodes,
						},
					},
					{
						CorrelationID: "upstream-target:v2.db.default.default.dc1:" + dbUID.String(),
						Result: &structs.IndexedCheckServiceNodes{
							Nodes: nodes,
						},
					},
				}, httpServiceDefaults, resolver, splitter)
			},
			patcher: func(ir *xdscommon.IndexedResources) *xdscommon.IndexedResources {
				return ir
			},
		},
		{
			name: "http-rds-missing-route",
			// RDS, Envoy's Route Discovery Service, is only used for HTTP services with a customized discovery chain, so we
			// need to use the test snapshot and add L7 config entries.
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "default", nil, []proxycfg.UpdateEvent{
					// The events ensure there are endpoints for the v1 and v2 subsets.
					{
						CorrelationID: "upstream-target:v1.db.default.default.dc1:" + dbUID.String(),
						Result: &structs.IndexedCheckServiceNodes{
							Nodes: nodes,
						},
					},
					{
						CorrelationID: "upstream-target:v2.db.default.default.dc1:" + dbUID.String(),
						Result: &structs.IndexedCheckServiceNodes{
							Nodes: nodes,
						},
					},
				}, httpServiceDefaults, resolver, splitter)
			},
			patcher: func(ir *xdscommon.IndexedResources) *xdscommon.IndexedResources {
				delete(ir.Index[xdscommon.RouteType], "db")
				return ir
			},
			err: "no route",
		},
		{
			name: "redirect",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "redirect-to-cluster-peer", nil, nil)
			},
		},
		// TODO tproxy is actually broken because the list of SNIs matches all service and tproxy uses SNIs on the filter chain.
		// This case is EXTREMELY complicated.
		{
			name: "tproxy",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotTransparentProxyHTTPUpstream(t)
			},
		},
		{
			name: "non-eds",
			create: proxycfg.TestConfigSnapshotPeering,
			serviceName: &api.CompoundServiceName{Name: "payments"},
			peer: "cloud",
			trustDomain: "1c053652-8512-4373-90cf-5a7f6263a994.consul",
		},
		{
			name: "non-eds-missing-endpoints",
			create: proxycfg.TestConfigSnapshotPeering,
			serviceName: &api.CompoundServiceName{Name: "payments"},
			peer: "cloud",
			trustDomain: "1c053652-8512-4373-90cf-5a7f6263a994.consul",
			patcher: func(ir *xdscommon.IndexedResources) *xdscommon.IndexedResources {
				sni := "payments.default.cloud.external.1c053652-8512-4373-90cf-5a7f6263a994.consul"
				msg := ir.Index[xdscommon.ClusterType][sni]
				c, ok := msg.(*envoy_cluster_v3.Cluster)
				require.True(t, ok)
				c.LoadAssignment = nil
				ir.Index[xdscommon.ClusterType][sni] = c
				return ir
			},
			err: "zero endpoints",
		},
	}

	latestEnvoyVersion := proxysupport.EnvoyVersions[0]
	sf, err := determineSupportedProxyFeaturesFromString(latestEnvoyVersion)
	require.NoError(t, err)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Sanity check default with no overrides first
			snap := tt.create(t)

			// We need to replace the TLS certs with deterministic ones to make golden
			// files workable. Note we don't update these otherwise they'd change
			// golden files for every test case and so not be any use!
			setupTLSRootsAndLeaf(t, snap)

			g := newResourceGenerator(testutil.Logger(t), nil, false)
			g.ProxyFeatures = sf

			res, err := g.allResourcesFromSnapshot(snap)
			require.NoError(t, err)

			indexedResources := indexResources(g.Logger, res)
			//fmt.Println(indexedResources)
			if tt.patcher != nil {
				indexedResources = tt.patcher(indexedResources)
			}
			trustDomain := tt.trustDomain
			if trustDomain == "" {
				trustDomain = "dc1"
			}
			serviceName := tt.serviceName
			if serviceName == nil {
				serviceName = &api.CompoundServiceName{
					Name: "db",
				}
			}
			peer := tt.peer

			dc := tt.dc
			if dc == "" && peer == "" {
				dc = "dc1"
			}

			err = validateupstream.Validate(indexedResources, *serviceName, dc, peer, trustDomain)

			if len(tt.err) == 0 {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.err)
			}
		})
	}
}
