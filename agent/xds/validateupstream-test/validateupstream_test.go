// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package validateupstream_test

import (
	"testing"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/xds"
	"github.com/hashicorp/consul/agent/xds/testcommon"
	"github.com/hashicorp/consul/envoyextensions/xdscommon"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/troubleshoot/proxy"
	testinf "github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/require"
)

// TestValidateUpstreams only tests validation for listeners, routes, and clusters. Endpoints validation is done in a
// top level test that can parse the output of the /clusters endpoint.
func TestValidateUpstreams(t *testing.T) {
	sni := "db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul"
	listenerName := "db:127.0.0.1:9191"
	httpServiceDefaults := &structs.ServiceConfigEntry{
		Kind:     structs.ServiceDefaults,
		Name:     "db",
		Protocol: "http",
	}

	dbUID := proxycfg.NewUpstreamID(&structs.Upstream{
		DestinationName: "db",
		LocalBindPort:   9191,
	})
	nodes := proxycfg.TestUpstreamNodes(t, "db")

	tests := []struct {
		name    string
		create  func(t testinf.T) *proxycfg.ConfigSnapshot
		patcher func(*xdscommon.IndexedResources) *xdscommon.IndexedResources
		err     string
		peer    string
		vip     string
		envoyID string
	}{
		{
			name: "tcp-success",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "default", false, nil, nil)
			},
		},
		{
			name: "tcp-missing-listener",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "default", false, nil, nil)
			},
			patcher: func(ir *xdscommon.IndexedResources) *xdscommon.IndexedResources {
				delete(ir.Index[xdscommon.ListenerType], listenerName)
				return ir
			},
			err: "No listener for upstream \"db\"",
		},
		{
			name: "tcp-missing-cluster",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "default", false, nil, nil)
			},
			patcher: func(ir *xdscommon.IndexedResources) *xdscommon.IndexedResources {
				delete(ir.Index[xdscommon.ClusterType], sni)
				return ir
			},
			err: "No cluster \"db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul\" for upstream \"db\"",
		},
		{
			name: "http-success",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "default", false, nil, nil, httpServiceDefaults)
			},
		},
		{
			name: "http-rds-success",
			// RDS, Envoy's Route Discovery Service, is only used for HTTP services with a customized discovery chain, so we
			// need to use the test snapshot and add L7 config entries.
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "default", false, nil, []proxycfg.UpdateEvent{
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
				}, configEntriesForDBSplits()...)
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
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "default", false, nil, []proxycfg.UpdateEvent{
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
				}, configEntriesForDBSplits()...)
			},
			patcher: func(ir *xdscommon.IndexedResources) *xdscommon.IndexedResources {
				delete(ir.Index[xdscommon.RouteType], "db")
				return ir
			},
			err: "No route for upstream \"db\"",
		},
		{
			name: "redirect",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "redirect-to-cluster-peer", false, nil, nil)
			},
		},
		{
			name: "failover",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "failover", false, nil, nil)
			},
		},
		{
			name: "failover-to-cluster-peer",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "failover-to-cluster-peer", false, nil, nil)
			},
		},
		{
			name:    "non-eds",
			create:  proxycfg.TestConfigSnapshotPeering,
			envoyID: "payments?peer=cloud",
		},
		{
			name: "tproxy-success",
			vip:  "240.0.0.1",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotTransparentProxyHTTPUpstream(t)
			},
			patcher: func(ir *xdscommon.IndexedResources) *xdscommon.IndexedResources {
				return ir
			},
		},
		{
			name: "tproxy-http-missing-cluster",
			vip:  "240.0.0.1",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotTransparentProxyHTTPUpstream(t)
			},
			patcher: func(ir *xdscommon.IndexedResources) *xdscommon.IndexedResources {
				sni := "google.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul"
				delete(ir.Index[xdscommon.ClusterType], sni)
				return ir
			},
			err: "No cluster \"google.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul\" for upstream \"240.0.0.1\"",
		},
		{
			name: "tproxy-http-redirect-success",
			vip:  "240.0.0.1",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotTransparentProxyHTTPUpstream(t, configEntriesForGoogleRedirect()...)
			},
			patcher: func(ir *xdscommon.IndexedResources) *xdscommon.IndexedResources {
				return ir
			},
		},
		{
			name: "tproxy-http-split-success",
			vip:  "240.0.0.1",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotTransparentProxyHTTPUpstream(t, configEntriesForGoogleSplits()...)
			},
			patcher: func(ir *xdscommon.IndexedResources) *xdscommon.IndexedResources {
				return ir
			},
		},
	}

	latestEnvoyVersion := xdscommon.EnvoyVersions[0]
	sf, err := xdscommon.DetermineSupportedProxyFeaturesFromString(latestEnvoyVersion)
	require.NoError(t, err)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Sanity check default with no overrides first
			snap := tt.create(t)

			// We need to replace the TLS certs with deterministic ones to make golden
			// files workable. Note we don't update these otherwise they'd change
			// golden files for every test case and so not be any use!
			testcommon.SetupTLSRootsAndLeaf(t, snap)

			g := xds.NewResourceGenerator(testutil.Logger(t), nil, false)
			g.ProxyFeatures = sf

			res, err := g.AllResourcesFromSnapshot(snap)
			require.NoError(t, err)

			indexedResources := xdscommon.IndexResources(g.Logger, res)
			if tt.patcher != nil {
				indexedResources = tt.patcher(indexedResources)
			}
			envoyID := tt.envoyID
			vip := tt.vip
			if envoyID == "" && vip == "" {
				envoyID = "db"
			}

			// This only tests validation for listeners, routes, and clusters. Endpoints validation is done in a top
			// level test that can parse the output of the /clusters endpoint. So for this test, we set clusters to nil.
			messages := troubleshoot.Validate(indexedResources, envoyID, vip, false, nil)

			var outputErrors string
			for _, msgError := range messages.Errors() {
				outputErrors += msgError.Message
				for _, action := range msgError.PossibleActions {
					outputErrors += action
				}
			}
			if len(tt.err) == 0 {
				require.True(t, messages.Success())
			} else {
				require.Contains(t, outputErrors, tt.err)
			}
		})
	}
}

func configEntriesForDBSplits() []structs.ConfigEntry {
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
	return []structs.ConfigEntry{httpServiceDefaults, splitter, resolver}
}

func configEntriesForGoogleSplits() []structs.ConfigEntry {
	splitter := &structs.ServiceSplitterConfigEntry{
		Kind: structs.ServiceSplitter,
		Name: "google",
		Splits: []structs.ServiceSplit{
			{
				Weight:        50,
				Service:       "google",
				ServiceSubset: "v1",
			},
			{
				Weight:        50,
				Service:       "google",
				ServiceSubset: "v2",
			},
		},
	}
	resolver := &structs.ServiceResolverConfigEntry{
		Kind: structs.ServiceResolver,
		Name: "google",
		Subsets: map[string]structs.ServiceResolverSubset{
			"v1": {Filter: "Service.Meta.version == v1"},
			"v2": {Filter: "Service.Meta.version == v2"},
		},
	}
	return []structs.ConfigEntry{splitter, resolver}
}

func configEntriesForGoogleRedirect() []structs.ConfigEntry {
	redirectGoogle := &structs.ServiceResolverConfigEntry{
		Kind: structs.ServiceResolver,
		Name: "google",
		Redirect: &structs.ServiceResolverRedirect{
			Service: "google-v2",
		},
	}
	return []structs.ConfigEntry{redirectGoogle}
}
