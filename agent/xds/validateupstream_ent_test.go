//go:build consulent
// +build consulent

package xds

import (
	"testing"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/xds/proxysupport"
	"github.com/hashicorp/consul/agent/xds/validateupstream"
	"github.com/hashicorp/consul/agent/xds/xdscommon"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil"
	testinf "github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/require"
)

func TestValidateUpstreams_Enterprise(t *testing.T) {
	sni := "db.bar.zip.dc1.internal-v1.11111111-2222-3333-4444-555555555555.consul"

	tests := []struct {
		name        string
		create      func(t testinf.T) *proxycfg.ConfigSnapshot
		patcher     func(*xdscommon.IndexedResources) *xdscommon.IndexedResources
		err         string
		peer        string
		serviceName *api.CompoundServiceName
		vip         string
	}{
		{
			name: "partition-namespace-success",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot_Partitions(t, false, nil, nil)
			},
		},
		{
			name: "partition-namespace-missing-endpoints",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot_Partitions(t, false, nil, nil)
			},
			patcher: func(ir *xdscommon.IndexedResources) *xdscommon.IndexedResources {
				delete(ir.Index[xdscommon.EndpointType], sni)
				return ir
			},
			err: "no cluster load assignment",
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
			serviceName := tt.serviceName
			if serviceName == nil {
				serviceName = &api.CompoundServiceName{
					Name:      "db",
					Partition: "zip",
					Namespace: "bar",
				}
			}
			peer := tt.peer

			err = validateupstream.Validate(indexedResources, *serviceName, peer, tt.vip)

			if len(tt.err) == 0 {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.err)
			}
		})
	}
}
