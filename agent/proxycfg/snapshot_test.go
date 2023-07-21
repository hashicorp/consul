package proxycfg

import (
	"context"
	"regexp"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	fuzz "github.com/google/gofuzz"
	"github.com/hashicorp/consul/agent/proxycfg/internal/watch"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbpeering"
	"github.com/stretchr/testify/require"
)

func TestConfigSnapshot_Clone(t *testing.T) {
	// ConfigSnapshot is a complex struct that (directly or indirectly) has a copy
	// of most of the structs in the agent/structs package. It'd be easy to break
	// the Clone method accidentally by changing one of those distant structs, so
	// we test it by using a fuzzer to traverse the graph and fill every field and
	// then compare the original to the copy.
	f := fuzz.New()
	f.NilChance(0)
	f.NumElements(1, 3)
	f.SkipFieldsWithPattern(regexp.MustCompile("^ServerSNIFn$"))

	f.Funcs(
		// Populate map[string]interface{} since gofuzz panics on these. We force them
		// to be any rather than concrete types otherwise they won't compare equal when
		// coming back out the other side.
		func(m map[string]any, c fuzz.Continue) {
			m[c.RandString()] = any(float64(c.RandUint64()))
			m[c.RandString()] = any(c.RandString())
			m[c.RandString()] = any([]any{c.RandString(), c.RandString()})
			m[c.RandString()] = any(map[string]any{c.RandString(): c.RandString()})
		},
		func(*context.CancelFunc, fuzz.Continue) {},
	)

	snapshot := new(ConfigSnapshot)
	f.Fuzz(snapshot)

	clone := snapshot.Clone()

	diff := cmp.Diff(snapshot, clone,
		cmpopts.IgnoreUnexported(indexedTarget{}),
		cmpopts.IgnoreUnexported(pbpeering.PeeringTrustBundle{}),
		cmpopts.IgnoreTypes(context.CancelFunc(nil)),
	)
	if diff != "" {
		t.Logf("Copied snaspshot is different to the original. You may need to re-run `make deep-copy`.\nDiff:\n%s", diff)
		t.FailNow()
	}
}

func TestAPIGatewaySnapshotToIngressGatewaySnapshot(t *testing.T) {
	cases := map[string]struct {
		apiGatewaySnapshot *configSnapshotAPIGateway
		expected           configSnapshotIngressGateway
	}{
		"default": {
			apiGatewaySnapshot: &configSnapshotAPIGateway{
				Listeners: map[string]structs.APIGatewayListener{},
			},
			expected: configSnapshotIngressGateway{
				GatewayConfigLoaded: true,
				ConfigSnapshotUpstreams: ConfigSnapshotUpstreams{
					PeerUpstreamEndpoints:    watch.NewMap[UpstreamID, structs.CheckServiceNodes](),
					WatchedLocalGWEndpoints:  watch.NewMap[string, structs.CheckServiceNodes](),
					WatchedGatewayEndpoints:  map[UpstreamID]map[string]structs.CheckServiceNodes{},
					WatchedUpstreamEndpoints: map[UpstreamID]map[string]structs.CheckServiceNodes{},
					UpstreamPeerTrustBundles: watch.NewMap[string, *pbpeering.PeeringTrustBundle](),
					DiscoveryChain:           map[UpstreamID]*structs.CompiledDiscoveryChain{},
				},
				Listeners: map[IngressListenerKey]structs.IngressListener{},
				Defaults:  structs.IngressServiceConfig{},
				Upstreams: map[IngressListenerKey]structs.Upstreams{},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			actual, err := tc.apiGatewaySnapshot.ToIngress("dc1")
			require.NoError(t, err)

			require.Equal(t, tc.expected, actual)
		})
	}
}
