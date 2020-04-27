package xds

import (
	"path"
	"sort"
	"testing"

	envoy "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/hashicorp/consul/agent/proxycfg"
	testinf "github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/require"
)

func TestRoutesFromSnapshot(t *testing.T) {

	tests := []struct {
		name   string
		create func(t testinf.T) *proxycfg.ConfigSnapshot
		// Setup is called before the test starts. It is passed the snapshot from
		// create func and is allowed to modify it in any way to setup the
		// test input.
		setup              func(snap *proxycfg.ConfigSnapshot)
		overrideGoldenName string
	}{
		{
			name:   "defaults-no-chain",
			create: proxycfg.TestConfigSnapshot,
			setup:  nil, // Default snapshot
		},
		{
			name:   "connect-proxy-with-chain",
			create: proxycfg.TestConfigSnapshotDiscoveryChain,
			setup:  nil,
		},
		{
			name:   "connect-proxy-with-chain-external-sni",
			create: proxycfg.TestConfigSnapshotDiscoveryChainExternalSNI,
			setup:  nil,
		},
		{
			name:   "connect-proxy-with-chain-and-overrides",
			create: proxycfg.TestConfigSnapshotDiscoveryChainWithOverrides,
			setup:  nil,
		},
		{
			name:   "splitter-with-resolver-redirect",
			create: proxycfg.TestConfigSnapshotDiscoveryChain_SplitterWithResolverRedirectMultiDC,
			setup:  nil,
		},
		{
			name:   "connect-proxy-with-chain-and-splitter",
			create: proxycfg.TestConfigSnapshotDiscoveryChainWithSplitter,
			setup:  nil,
		},
		{
			name:   "connect-proxy-with-grpc-router",
			create: proxycfg.TestConfigSnapshotDiscoveryChainWithGRPCRouter,
			setup:  nil,
		},
		{
			name:   "connect-proxy-with-chain-and-router",
			create: proxycfg.TestConfigSnapshotDiscoveryChainWithRouter,
			setup:  nil,
		},
		// TODO(rb): test match stanza skipped for grpc
		// Start ingress gateway test cases
		{
			name:   "ingress-defaults-no-chain",
			create: proxycfg.TestConfigSnapshotIngressGateway,
			setup:  nil, // Default snapshot
		},
		{
			name:   "ingress-with-chain",
			create: proxycfg.TestConfigSnapshotIngress,
			setup:  nil,
		},
		{
			name:   "ingress-with-chain-external-sni",
			create: proxycfg.TestConfigSnapshotIngressExternalSNI,
			setup:  nil,
		},
		{
			name:   "ingress-with-chain-and-overrides",
			create: proxycfg.TestConfigSnapshotIngressWithOverrides,
			setup:  nil,
		},
		{
			name:   "ingress-splitter-with-resolver-redirect",
			create: proxycfg.TestConfigSnapshotIngress_SplitterWithResolverRedirectMultiDC,
			setup:  nil,
		},
		{
			name:   "ingress-with-chain-and-splitter",
			create: proxycfg.TestConfigSnapshotIngressWithSplitter,
			setup:  nil,
		},
		{
			name:   "ingress-with-grpc-router",
			create: proxycfg.TestConfigSnapshotIngressWithGRPCRouter,
			setup:  nil,
		},
		{
			name:   "ingress-with-chain-and-router",
			create: proxycfg.TestConfigSnapshotIngressWithRouter,
			setup:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			// Sanity check default with no overrides first
			snap := tt.create(t)

			// We need to replace the TLS certs with deterministic ones to make golden
			// files workable. Note we don't update these otherwise they'd change
			// golden files for every test case and so not be any use!
			setupTLSRootsAndLeaf(t, snap)

			if tt.setup != nil {
				tt.setup(snap)
			}

			routes, err := routesFromSnapshot(snap, "my-token")
			require.NoError(err)
			sort.Slice(routes, func(i, j int) bool {
				return routes[i].(*envoy.RouteConfiguration).Name < routes[j].(*envoy.RouteConfiguration).Name
			})
			r, err := createResponse(RouteType, "00000001", "00000001", routes)
			require.NoError(err)

			gotJSON := responseToJSON(t, r)

			gName := tt.name
			if tt.overrideGoldenName != "" {
				gName = tt.overrideGoldenName
			}

			require.JSONEq(golden(t, path.Join("routes", gName), gotJSON), gotJSON)
		})
	}
}
