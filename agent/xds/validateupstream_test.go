//go:build !consulent
// +build !consulent

package xds

import (
	"testing"

	envoy_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
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
	listenerName := "db:127.0.0.1:9191"
	sni := "db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul"
	// TODO HTTP service
	// TODO missing routes because db is TCP and isn't using splitters, routers, etc
	// TODO without EDS
	// TODO explicit upstreams and tproxy for the same service.
	tests := []struct {
		name    string
		create  func(t testinf.T) *proxycfg.ConfigSnapshot
		patcher func(*xdscommon.IndexedResources) *xdscommon.IndexedResources
		err     string
	}{
		{
			name: "success",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "default", nil, nil)
			},
		},
		{
			name: "missing-listener",
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
			name: "missing-cluster",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "default", nil, nil)
			},
			patcher: func(ir *xdscommon.IndexedResources) *xdscommon.IndexedResources {
				delete(ir.Index[xdscommon.ClusterType], sni)
				return ir
			},
			err: "no cluster",
		},
		{
			name: "missing-load-assignment",
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
			name: "missing-eds-endpoints",
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
			err: "expected endpoints on load assignment but we didn't get any",
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
			if tt.patcher != nil {
				indexedResources = tt.patcher(indexedResources)
			}
			err = validateupstream.Validate(indexedResources, api.CompoundServiceName{
				Name: "db",
			}, "dc1", "11111111-2222-3333-4444-555555555555.consul")

			if len(tt.err) == 0 {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Equal(t, tt.err, err.Error())
			}
		})
	}
}
