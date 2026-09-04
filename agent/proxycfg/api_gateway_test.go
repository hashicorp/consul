// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package proxycfg

import (
	"bytes"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/proxycfg/internal/watch"
	"github.com/hashicorp/consul/agent/structs"
)

// mustCompileTestHTTPChain compiles a real, always-succeeding http discovery
// chain for a plain service with no router - the "good" half of the
// recompileDiscoveryChains partial-failure tests below.
func mustCompileTestHTTPChain(t *testing.T, serviceName string) *structs.CompiledDiscoveryChain {
	t.Helper()

	set := configentry.NewDiscoveryChainSet()
	set.AddServices(&structs.ServiceConfigEntry{
		Kind:     structs.ServiceDefaults,
		Name:     serviceName,
		Protocol: "http",
	})
	return discoverychain.TestCompileConfigEntries(t, serviceName, "default", "default", "dc1", "test-trust-domain.consul", nil, set)
}

// newTestHTTPRoute builds a single-service, path-prefix HTTPRoute pointed at
// serviceName - the minimal shape recompileDiscoveryChains needs to reach
// Synthesize. hostname must be distinct per route sharing a listener, or
// consolidateHTTPRoutes will merge multiple routes into one synthetic router
// and a single bad route will take a good one down with it - exactly the
// noisy-neighbour behavior these tests exist to rule out, so getting this
// wrong would silently invalidate the isolation being asserted.
func newTestHTTPRoute(name, hostname, serviceName string) *structs.HTTPRouteConfigEntry {
	return &structs.HTTPRouteConfigEntry{
		Kind:      structs.HTTPRoute,
		Name:      name,
		Hostnames: []string{hostname},
		Rules: []structs.HTTPRouteRule{{
			Matches: []structs.HTTPMatch{{
				Path: structs.HTTPPathMatch{Match: structs.HTTPPathMatchPrefix, Value: "/"},
			}},
			Services: []structs.HTTPService{{Name: serviceName}},
		}},
	}
}

// TestRecompileDiscoveryChains_UnknownRouteKindIsolation asserts that a
// routeRef with an unrecognized Kind (e.g. a future route type this build
// doesn't understand yet, or corrupted/skewed state) is skipped and logged
// like any other bad route, rather than aborting synthesizeChains for the
// rest of the listener and, via recompileDiscoveryChains's immediate
// `return err`, discarding chains already synthesized for other listeners
// in the same recompile pass.
func TestRecompileDiscoveryChains_UnknownRouteKindIsolation(t *testing.T) {
	var logBuf bytes.Buffer
	logger := hclog.New(&hclog.LoggerOptions{Output: &logBuf, Level: hclog.Debug})

	h := &handlerAPIGateway{
		handlerState: handlerState{
			stateConfig: stateConfig{
				logger: logger,
				source: &structs.QuerySource{Datacenter: "dc1"},
			},
		},
	}

	goodRouteRef := structs.ResourceReference{Kind: structs.HTTPRoute, Name: "good-route"}
	unknownRouteRef := structs.ResourceReference{Kind: "grpc-route", Name: "unknown-route"}
	otherGoodRouteRef := structs.ResourceReference{Kind: structs.HTTPRoute, Name: "other-good-route"}

	httpRoutes := watch.NewMap[structs.ResourceReference, *structs.HTTPRouteConfigEntry]()
	for ref, route := range map[structs.ResourceReference]*structs.HTTPRouteConfigEntry{
		goodRouteRef:      newTestHTTPRoute("good-route", "good.example.com", "good-svc"),
		otherGoodRouteRef: newTestHTTPRoute("other-good-route", "other-good.example.com", "other-good-svc"),
	} {
		httpRoutes.InitWatch(ref, nil)
		httpRoutes.Set(ref, route)
	}

	discoveryChain := map[UpstreamID]*structs.CompiledDiscoveryChain{
		NewUpstreamIDFromServiceName(structs.NewServiceName("good-svc", nil)):       mustCompileTestHTTPChain(t, "good-svc"),
		NewUpstreamIDFromServiceName(structs.NewServiceName("other-good-svc", nil)): mustCompileTestHTTPChain(t, "other-good-svc"),
	}
	initialChainCount := len(discoveryChain)

	snap := &ConfigSnapshot{
		APIGateway: configSnapshotAPIGateway{
			ConfigSnapshotUpstreams: ConfigSnapshotUpstreams{
				DiscoveryChain: discoveryChain,
			},
			GatewayConfig: &structs.APIGatewayConfigEntry{
				Kind: structs.APIGateway,
				Name: "gateway",
			},
			HTTPRoutes: httpRoutes,
			TCPRoutes:  watch.NewMap[structs.ResourceReference, *structs.TCPRouteConfigEntry](),
			Listeners: map[string]structs.APIGatewayListener{
				"listener-1": {Name: "listener-1", Protocol: structs.ListenerProtocolHTTP},
				"listener-2": {Name: "listener-2", Protocol: structs.ListenerProtocolHTTP},
			},
			BoundListeners: map[string]structs.BoundAPIGatewayListener{
				"listener-1": {Name: "listener-1", Routes: []structs.ResourceReference{goodRouteRef, unknownRouteRef}},
				"listener-2": {Name: "listener-2", Routes: []structs.ResourceReference{otherGoodRouteRef}},
			},
		},
	}

	err := h.recompileDiscoveryChains(snap)
	require.NoError(t, err, "an unrecognized route kind on one listener must not fail the whole gateway "+
		"recompile, nor discard chains already synthesized for other listeners in this pass")

	require.Len(t, snap.APIGateway.DiscoveryChain, initialChainCount+2,
		"both good-route (listener-1) and other-good-route (listener-2) should still synthesize; "+
			"the unknown-kind route contributes nothing but must not take either down")

	require.Contains(t, logBuf.String(), "unknown route kind",
		"the skipped unknown-kind route error must actually reach the logs, not just be computed and discarded")
	require.Contains(t, logBuf.String(), "listener-1",
		"the warning should identify which listener the skipped route was on")
}

// TestRecompileDiscoveryChains_ListenerAllRoutesUnknownKindOnly asserts that
// a listener whose only bound route has an unrecognized Kind - so nothing on
// it ever resolves to a real HTTPRoute/TCPRoute chain - is skipped and logged
// like any other fully-failed listener. This exercises a code path distinct
// from TestRecompileDiscoveryChains_UnknownRouteKindIsolation: there, the
// unknown-kind route shares a listener with a good HTTPRoute, so `chains` in
// synthesizeChains (agent/proxycfg/snapshot.go) ends up non-empty and
// synthesizer.Synthesize is invoked normally. Here, chains stays empty for
// listener-1, so synthesizeChains takes its `len(chains) == 0` early return
// (returning `preSkipped` directly) and Synthesize is never called at all -
// proving preSkipped is still surfaced to the operator log via that branch.
func TestRecompileDiscoveryChains_ListenerAllRoutesUnknownKindOnly(t *testing.T) {
	var logBuf bytes.Buffer
	logger := hclog.New(&hclog.LoggerOptions{Output: &logBuf, Level: hclog.Debug})

	h := &handlerAPIGateway{
		handlerState: handlerState{
			stateConfig: stateConfig{
				logger: logger,
				source: &structs.QuerySource{Datacenter: "dc1"},
			},
		},
	}

	unknownRouteRef := structs.ResourceReference{Kind: "grpc-route", Name: "unknown-route"}
	goodRouteRef := structs.ResourceReference{Kind: structs.HTTPRoute, Name: "good-route"}

	httpRoutes := watch.NewMap[structs.ResourceReference, *structs.HTTPRouteConfigEntry]()
	httpRoutes.InitWatch(goodRouteRef, nil)
	httpRoutes.Set(goodRouteRef, newTestHTTPRoute("good-route", "good.example.com", "good-svc"))

	discoveryChain := map[UpstreamID]*structs.CompiledDiscoveryChain{
		NewUpstreamIDFromServiceName(structs.NewServiceName("good-svc", nil)): mustCompileTestHTTPChain(t, "good-svc"),
	}
	initialChainCount := len(discoveryChain)

	snap := &ConfigSnapshot{
		APIGateway: configSnapshotAPIGateway{
			ConfigSnapshotUpstreams: ConfigSnapshotUpstreams{
				DiscoveryChain: discoveryChain,
			},
			GatewayConfig: &structs.APIGatewayConfigEntry{
				Kind: structs.APIGateway,
				Name: "gateway",
			},
			HTTPRoutes: httpRoutes,
			TCPRoutes:  watch.NewMap[structs.ResourceReference, *structs.TCPRouteConfigEntry](),
			Listeners: map[string]structs.APIGatewayListener{
				"listener-1": {Name: "listener-1", Protocol: structs.ListenerProtocolHTTP},
				"listener-2": {Name: "listener-2", Protocol: structs.ListenerProtocolHTTP},
			},
			BoundListeners: map[string]structs.BoundAPIGatewayListener{
				"listener-1": {Name: "listener-1", Routes: []structs.ResourceReference{unknownRouteRef}},
				"listener-2": {Name: "listener-2", Routes: []structs.ResourceReference{goodRouteRef}},
			},
		},
	}

	err := h.recompileDiscoveryChains(snap)
	require.NoError(t, err, "a listener whose only route has an unrecognized kind must not fail the whole "+
		"gateway recompile, even though synthesizeChains never calls Synthesize for it")

	require.Len(t, snap.APIGateway.DiscoveryChain, initialChainCount+1,
		"only good-route (listener-2) should contribute a synthesized chain; listener-1 contributes nothing "+
			"since its only route never resolved to a compiled chain")

	require.Contains(t, logBuf.String(), "unknown route kind",
		"the unknown-kind error must reach the logs even via synthesizeChains' len(chains)==0 early return")
	require.Contains(t, logBuf.String(), "listener-1",
		"the warning should identify which listener the unknown-kind route was on")
}
