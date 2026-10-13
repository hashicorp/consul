// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package proxycfg

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/proxycfg/internal/watch"
	"github.com/hashicorp/consul/agent/structs"
)

// mustCompileTestHTTPChain compiles a real, always-succeeding http discovery
// chain for a plain service with no router - the "good" half of the
// recompileDiscoveryChains partial-failure test below.
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

// newBrokenSubsetTestChain hand-builds a chain whose router composes a route
// to a destination pinned to a service-resolver subset that is never
// defined. This is the same technique used in
// agent/consul/discoverychain/gateway_test.go's newBrokenSubsetChain: it
// makes discovery-chain *synthesis* fail with a real "does not have a subset
// named" error, without touching protocol resolution.
func newBrokenSubsetTestChain(serviceName string) *structs.CompiledDiscoveryChain {
	routerNode := "router:" + serviceName
	return &structs.CompiledDiscoveryChain{
		ServiceName: serviceName,
		Namespace:   "default",
		Partition:   "default",
		Datacenter:  "dc1",
		StartNode:   routerNode,
		Nodes: map[string]*structs.DiscoveryGraphNode{
			routerNode: {
				Type: structs.DiscoveryGraphNodeTypeRouter,
				Name: serviceName + "-router",
				Routes: []*structs.DiscoveryRoute{{
					Definition: &structs.ServiceRoute{
						Destination: &structs.ServiceRouteDestination{
							Service:       "downstream-tcp-svc",
							Namespace:     "default",
							Partition:     "default",
							ServiceSubset: "ghost-subset",
						},
					},
				}},
			},
		},
	}
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

// TestRecompileDiscoveryChains_PartialFailureIsolation is a direct,
// event-harness-free test of handlerAPIGateway.recompileDiscoveryChains -
// nothing in this package tests it today, not even the happy path, despite
// it being the only place the Synthesize -> synthesizeChains skip-list
// actually reaches an operator (as a warning log) and where the
// services[i]/compiled[i] positional-pairing assumption gets exercised
// end to end.
//
// Two listeners are used specifically to prove a broken route on one
// listener doesn't affect a second, unrelated listener on the same gateway.
func TestRecompileDiscoveryChains_PartialFailureIsolation(t *testing.T) {
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

	goodRoute1Ref := structs.ResourceReference{Kind: structs.HTTPRoute, Name: "good-route-1"}
	badRouteRef := structs.ResourceReference{Kind: structs.HTTPRoute, Name: "bad-route"}
	goodRoute2Ref := structs.ResourceReference{Kind: structs.HTTPRoute, Name: "good-route-2"}

	httpRoutes := watch.NewMap[structs.ResourceReference, *structs.HTTPRouteConfigEntry]()
	for ref, route := range map[structs.ResourceReference]*structs.HTTPRouteConfigEntry{
		goodRoute1Ref: newTestHTTPRoute("good-route-1", "good1.example.com", "good-svc-1"),
		badRouteRef:   newTestHTTPRoute("bad-route", "bad.example.com", "bad-svc"),
		goodRoute2Ref: newTestHTTPRoute("good-route-2", "good2.example.com", "good-svc-2"),
	} {
		httpRoutes.InitWatch(ref, nil)
		httpRoutes.Set(ref, route)
	}

	discoveryChain := map[UpstreamID]*structs.CompiledDiscoveryChain{
		NewUpstreamIDFromServiceName(structs.NewServiceName("good-svc-1", nil)): mustCompileTestHTTPChain(t, "good-svc-1"),
		NewUpstreamIDFromServiceName(structs.NewServiceName("bad-svc", nil)):    newBrokenSubsetTestChain("bad-svc"),
		NewUpstreamIDFromServiceName(structs.NewServiceName("good-svc-2", nil)): mustCompileTestHTTPChain(t, "good-svc-2"),
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
				"listener-1": {Name: "listener-1", Routes: []structs.ResourceReference{goodRoute1Ref, badRouteRef}},
				"listener-2": {Name: "listener-2", Routes: []structs.ResourceReference{goodRoute2Ref}},
			},
		},
	}

	err := h.recompileDiscoveryChains(snap)
	require.NoError(t, err, "a broken route on one listener must not fail the whole gateway recompile "+
		"(no crash, no false-positive on the compiled[i].ServiceName != service.Name check)")

	require.Len(t, snap.APIGateway.DiscoveryChain, initialChainCount+2,
		"exactly two new synthesized chains should be added: one for good-route-1 (listener-1) "+
			"and one for good-route-2 (listener-2); bad-route contributes nothing")

	require.Contains(t, logBuf.String(), "skipping misconfigured HTTPRoute",
		"the skipped bad-route error must actually reach the logs, not just be computed and discarded")
	require.Contains(t, logBuf.String(), "listener-1",
		"the warning should identify which listener the skipped route was on")
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

// TestRecompileDiscoveryChains_WholeListenerFailureIsolation asserts that a
// listener where EVERY route fails to compile (not just one bad route among
// several good ones) is cleanly skipped - contributing no chains and no
// error - while a second, independent listener on the same gateway still
// succeeds. This exercises the `len(upstreams) == 0 { continue }` skip in
// recompileDiscoveryChains (agent/proxycfg/api_gateway.go) for the case where
// that length is zero because nothing on the listener survived synthesis.
func TestRecompileDiscoveryChains_WholeListenerFailureIsolation(t *testing.T) {
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

	badRouteRef := structs.ResourceReference{Kind: structs.HTTPRoute, Name: "bad-route"}
	goodRouteRef := structs.ResourceReference{Kind: structs.HTTPRoute, Name: "good-route"}

	httpRoutes := watch.NewMap[structs.ResourceReference, *structs.HTTPRouteConfigEntry]()
	for ref, route := range map[structs.ResourceReference]*structs.HTTPRouteConfigEntry{
		badRouteRef:  newTestHTTPRoute("bad-route", "bad.example.com", "bad-svc"),
		goodRouteRef: newTestHTTPRoute("good-route", "good.example.com", "good-svc"),
	} {
		httpRoutes.InitWatch(ref, nil)
		httpRoutes.Set(ref, route)
	}

	discoveryChain := map[UpstreamID]*structs.CompiledDiscoveryChain{
		NewUpstreamIDFromServiceName(structs.NewServiceName("bad-svc", nil)):  newBrokenSubsetTestChain("bad-svc"),
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
				"listener-1": {Name: "listener-1", Routes: []structs.ResourceReference{badRouteRef}},
				"listener-2": {Name: "listener-2", Routes: []structs.ResourceReference{goodRouteRef}},
			},
		},
	}

	err := h.recompileDiscoveryChains(snap)
	require.NoError(t, err, "a listener with zero surviving routes must not fail the whole gateway recompile")

	require.Len(t, snap.APIGateway.DiscoveryChain, initialChainCount+1,
		"only good-route (listener-2) should contribute a synthesized chain; listener-1 contributes nothing "+
			"because every route on it failed to compile")

	require.Contains(t, logBuf.String(), "skipping misconfigured HTTPRoute",
		"the skipped bad-route error must still reach the logs even though it was the only route on its listener")
	require.Contains(t, logBuf.String(), "listener-1",
		"the warning should identify which listener the fully-failed route was on")
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

// TestDiscoveryChainsMissingEndpoints_OrphanChainSkipped is the direct regression
// test for the "noisy-neighbour" bug introduced by PR #13169 where a single
// HTTPRoute backend that failed BackendRef resolution (e.g. namespace
// "gapi-blue" does not exist) left an orphan chain in DiscoveryChain that was
// never added to any listener's Upstreams map. The pre-fix predicate iterated all
// chains — including orphan ones — and reported the orphan's unsatisfied EDS
// targets as missing, which permanently blocked snapshot admission and caused
// "no healthy upstream" 503 for every route on the gateway.
//
// Three sub-cases confirm the predicate behaves correctly:
//
//  1. All chains in Upstreams, all endpoints present  → nothing missing
//  2. Orphan chain (not in Upstreams) whose target has no endpoints:
//     pre-fix: reported missing (bug — blocks admission)
//     post-fix: not reported (orphan is skipped)
//  3. Active chain (in Upstreams) whose target has no endpoints → reported missing
//     (the gate still fires for genuinely-missing EDS data)
func TestDiscoveryChainsMissingEndpoints_OrphanChainSkipped(t *testing.T) {
	t.Parallel()

	// Build two real chains: one for the active service (in a listener upstream)
	// and one for the orphan (BackendNotFound — never put in Upstreams).
	activeUID := NewUpstreamIDFromServiceName(structs.NewServiceName("active-svc", nil))
	orphanUID := NewUpstreamIDFromServiceName(structs.NewServiceName("orphan-svc", nil))

	activeChain := mustCompileTestHTTPChain(t, "active-svc")
	orphanChain := mustCompileTestHTTPChain(t, "orphan-svc")

	// Identify the single target ID in each chain (mustCompileTestHTTPChain
	// produces a one-target chain).
	var activeTargetID string
	for id := range activeChain.Targets {
		activeTargetID = id
		break
	}
	var orphanTargetID string
	for id := range orphanChain.Targets {
		orphanTargetID = id
		break
	}

	// Build the listenerRouteUpstreams that represents the resolved listener
	// state: only active-svc reaches a listener; orphan-svc (BackendNotFound)
	// does not.
	routeRef := structs.ResourceReference{Kind: structs.HTTPRoute, Name: "good-route"}
	listenerKey := APIGatewayListenerKey{Protocol: "http", Port: 8080}
	// Do NOT set Datacenter here: NewUpstreamID normalises empty DC to ""
	// (no "?dc=" suffix), matching NewUpstreamIDFromServiceName's output.
	activeUpstream := structs.Upstream{DestinationName: "active-svc", DestinationNamespace: "default", DestinationPartition: "default"}
	upstreams := listenerRouteUpstreams{}
	upstreams.set(routeRef, listenerKey, structs.Upstreams{activeUpstream})

	// localKey matches the DC/partition of all test chains so no mesh-gateway
	// path is taken; this keeps the subtests focused on the orphan-chain gate.
	localKey := GatewayKey{Datacenter: "dc1", Partition: "default"}

	t.Run("all_active_endpoints_present_returns_true", func(t *testing.T) {
		snap := &configSnapshotAPIGateway{
			ConfigSnapshotUpstreams: ConfigSnapshotUpstreams{
				DiscoveryChain: map[UpstreamID]*structs.CompiledDiscoveryChain{
					activeUID: activeChain,
					orphanUID: orphanChain,
				},
				WatchedUpstreamEndpoints: map[UpstreamID]map[string]structs.CheckServiceNodes{
					// active chain has its EDS data populated
					activeUID: {activeTargetID: structs.CheckServiceNodes{}},
					// orphan chain also populated (unrelated — demonstrates gating
					// only on Upstreams membership regardless of EDS state)
					orphanUID: {orphanTargetID: structs.CheckServiceNodes{}},
				},
			},
			Upstreams: upstreams,
		}
		wireReadyListener(snap, listenerKey, routeRef)
		require.Empty(t, snap.discoveryChainsMissingEndpoints(localKey),
			"when all active chains have endpoints, the gate must report nothing missing")
	})

	t.Run("orphan_chain_no_endpoints_returns_true", func(t *testing.T) {
		// This is the exact regression scenario: orphan chain's EDS target is
		// absent from WatchedUpstreamEndpoints, but because orphan-svc is NOT
		// in Upstreams, the gate must skip it and return true.
		snap := &configSnapshotAPIGateway{
			ConfigSnapshotUpstreams: ConfigSnapshotUpstreams{
				DiscoveryChain: map[UpstreamID]*structs.CompiledDiscoveryChain{
					activeUID: activeChain,
					orphanUID: orphanChain,
				},
				WatchedUpstreamEndpoints: map[UpstreamID]map[string]structs.CheckServiceNodes{
					// active chain has its EDS data
					activeUID: {activeTargetID: structs.CheckServiceNodes{}},
					// orphan chain intentionally has NO entry — simulates the
					// BackendNotFound case where watch was never registered
				},
			},
			Upstreams: upstreams,
		}
		wireReadyListener(snap, listenerKey, routeRef)
		require.Empty(t, snap.discoveryChainsMissingEndpoints(localKey),
			"orphan chain (not in Upstreams) with missing endpoints must NOT block admission — "+
				"this is the PR #13169 regression guard")
	})

	t.Run("active_chain_missing_endpoints_returns_false", func(t *testing.T) {
		// Confirm the gate still fires correctly when a *real* active upstream
		// is missing its EDS data (cold-start window, not an orphan).
		snap := &configSnapshotAPIGateway{
			ConfigSnapshotUpstreams: ConfigSnapshotUpstreams{
				DiscoveryChain: map[UpstreamID]*structs.CompiledDiscoveryChain{
					activeUID: activeChain,
				},
				WatchedUpstreamEndpoints: map[UpstreamID]map[string]structs.CheckServiceNodes{
					// active chain's EDS target deliberately missing
				},
			},
			Upstreams: upstreams,
		}
		wireReadyListener(snap, listenerKey, routeRef)
		require.NotEmpty(t, snap.discoveryChainsMissingEndpoints(localKey),
			"active chain (in Upstreams) with missing EDS target must still be reported missing — "+
				"the cold-start gate must remain intact for genuine EDS lag")
	})

	t.Run("mesh_gateway_endpoints_missing_returns_false", func(t *testing.T) {
		// Regression guard for the rolling-restart segfault: a service whose
		// discovery-chain target lives in a remote datacenter (dc2) routes
		// through a remote mesh gateway. makeLoadAssignmentEndpointGroup checks
		// WatchedGatewayEndpoints for those targets. If that watch hasn't fired
		// yet, it returns valid=false, EDS is skipped, but CDS already emitted
		// the cluster → segfault. This subtest ensures the gate also blocks on
		// WatchedGatewayEndpoints.
		//
		// The single target has MeshGatewayModeRemote and lives in a remote
		// datacenter (dc2 != localKey's dc1) so that localKey.Matches() returns
		// false and the gateway-endpoint check is triggered. Using a remote
		// datacenter (not a remote partition) keeps this identical under CE and ent.
		remoteUID := NewUpstreamIDFromServiceName(structs.NewServiceName("remote-svc", nil))
		remoteTargetID := "remote-svc.default.default.dc2"

		remoteChain := &structs.CompiledDiscoveryChain{
			ServiceName: "remote-svc",
			Namespace:   "default",
			Partition:   "default",
			Datacenter:  "dc2",
			StartNode:   "resolver:remote-svc.default.default.dc2",
			Nodes: map[string]*structs.DiscoveryGraphNode{
				"resolver:remote-svc.default.default.dc2": {
					Type: structs.DiscoveryGraphNodeTypeResolver,
					Name: "remote-svc",
					Resolver: &structs.DiscoveryResolver{
						ConnectTimeout: 5000000000,
						Target:         remoteTargetID,
					},
				},
			},
			Targets: map[string]*structs.DiscoveryTarget{
				remoteTargetID: {
					ID:          remoteTargetID,
					Service:     "remote-svc",
					Namespace:   "default",
					Partition:   "default",
					Datacenter:  "dc2",
					MeshGateway: structs.MeshGatewayConfig{Mode: structs.MeshGatewayModeRemote},
				},
			},
		}

		remoteRouteRef := structs.ResourceReference{Kind: structs.HTTPRoute, Name: "remote-route"}
		remoteUpstream := structs.Upstream{DestinationName: "remote-svc", DestinationNamespace: "default", DestinationPartition: "default"}
		remoteUpstreams := listenerRouteUpstreams{}
		remoteUpstreams.set(remoteRouteRef, listenerKey, structs.Upstreams{remoteUpstream})

		snap := &configSnapshotAPIGateway{
			ConfigSnapshotUpstreams: ConfigSnapshotUpstreams{
				DiscoveryChain: map[UpstreamID]*structs.CompiledDiscoveryChain{
					remoteUID: remoteChain,
				},
				WatchedUpstreamEndpoints: map[UpstreamID]map[string]structs.CheckServiceNodes{
					// upstream endpoint watch fired (real service nodes populated)
					remoteUID: {remoteTargetID: structs.CheckServiceNodes{}},
				},
				// WatchedGatewayEndpoints intentionally absent — gateway watch hasn't fired yet
				WatchedGatewayEndpoints: map[UpstreamID]map[string]structs.CheckServiceNodes{},
			},
			Upstreams: remoteUpstreams,
		}
		wireReadyListener(snap, listenerKey, remoteRouteRef)
		require.NotEmpty(t, snap.discoveryChainsMissingEndpoints(localKey),
			"active chain with a remote mesh-gateway target whose WatchedGatewayEndpoints "+
				"has not yet been populated must block snapshot admission (rolling-restart segfault guard)")
	})

	t.Run("mesh_gateway_endpoints_present_returns_true", func(t *testing.T) {
		// Same setup as above but with WatchedGatewayEndpoints populated — gate must pass.
		remoteUID := NewUpstreamIDFromServiceName(structs.NewServiceName("remote-svc", nil))
		remoteTargetID := "remote-svc.default.default.dc2"

		remoteChain := &structs.CompiledDiscoveryChain{
			ServiceName: "remote-svc",
			Namespace:   "default",
			Partition:   "default",
			Datacenter:  "dc2",
			StartNode:   "resolver:remote-svc.default.default.dc2",
			Nodes: map[string]*structs.DiscoveryGraphNode{
				"resolver:remote-svc.default.default.dc2": {
					Type: structs.DiscoveryGraphNodeTypeResolver,
					Name: "remote-svc",
					Resolver: &structs.DiscoveryResolver{
						ConnectTimeout: 5000000000,
						Target:         remoteTargetID,
					},
				},
			},
			Targets: map[string]*structs.DiscoveryTarget{
				remoteTargetID: {
					ID:          remoteTargetID,
					Service:     "remote-svc",
					Namespace:   "default",
					Partition:   "default",
					Datacenter:  "dc2",
					MeshGateway: structs.MeshGatewayConfig{Mode: structs.MeshGatewayModeRemote},
				},
			},
		}

		remoteRouteRef := structs.ResourceReference{Kind: structs.HTTPRoute, Name: "remote-route"}
		remoteUpstream := structs.Upstream{DestinationName: "remote-svc", DestinationNamespace: "default", DestinationPartition: "default"}
		remoteUpstreams := listenerRouteUpstreams{}
		remoteUpstreams.set(remoteRouteRef, listenerKey, structs.Upstreams{remoteUpstream})

		gwKey := GatewayKey{Datacenter: "dc2", Partition: "default"}
		snap := &configSnapshotAPIGateway{
			ConfigSnapshotUpstreams: ConfigSnapshotUpstreams{
				DiscoveryChain: map[UpstreamID]*structs.CompiledDiscoveryChain{
					remoteUID: remoteChain,
				},
				WatchedUpstreamEndpoints: map[UpstreamID]map[string]structs.CheckServiceNodes{
					remoteUID: {remoteTargetID: structs.CheckServiceNodes{}},
				},
				WatchedGatewayEndpoints: map[UpstreamID]map[string]structs.CheckServiceNodes{
					// gateway watch has fired
					remoteUID: {gwKey.String(): structs.CheckServiceNodes{}},
				},
			},
			Upstreams: remoteUpstreams,
		}
		wireReadyListener(snap, listenerKey, remoteRouteRef)
		require.Empty(t, snap.discoveryChainsMissingEndpoints(localKey),
			"active chain with a remote mesh-gateway target whose WatchedGatewayEndpoints "+
				"is populated must allow snapshot admission")
	})
}

// TestAPIGatewayCompleteness_CoalesceRace documents how this approach (B)
// handles the coalesce-window race described in ITCO-15826 (cold-start segfault).
//
// The scenario:
//  1. Chains for some services have arrived with their endpoint data.
//  2. During the coalesce delay, another chain (svc2) arrives via handleUpdate
//     but its endpoint watch has NOT yet fired (svc2 not in
//     WatchedUpstreamEndpoints yet).
//  3. If a snapshot were pushed in this window it would carry a CDS cluster for
//     svc2 with no matching EDS — the cold-start segfault path.
//
// In approach B, valid() intentionally does NOT gate on endpoints — proxycfg
// keeps delivering snapshots so steady-state churn is never starved. The
// CDS/EDS coherence gate lives per-stream in the xDS layer, which consults
// discoveryChainsMissingEndpoints to hold only a stream's FIRST push. This test
// verifies that split: valid() stays true during the race, while the
// completeness predicate correctly reports svc2's target as missing until its
// endpoints arrive.
func TestAPIGatewayCompleteness_CoalesceRace(t *testing.T) {
	t.Parallel()

	routeRef := structs.ResourceReference{Kind: structs.HTTPRoute, Name: "routes"}
	listenerKey := APIGatewayListenerKey{Protocol: "http", Port: 8080}
	localKey := GatewayKey{Datacenter: "dc1", Partition: "default"}

	// Build two active chains (both registered in Upstreams).
	svc1UID := NewUpstreamIDFromServiceName(structs.NewServiceName("svc1", nil))
	svc2UID := NewUpstreamIDFromServiceName(structs.NewServiceName("svc2", nil))
	svc1Chain := mustCompileTestHTTPChain(t, "svc1")
	svc2Chain := mustCompileTestHTTPChain(t, "svc2")

	var svc1TargetID, svc2TargetID string
	for id := range svc1Chain.Targets {
		svc1TargetID = id
	}
	for id := range svc2Chain.Targets {
		svc2TargetID = id
	}

	svc1Up := structs.Upstream{DestinationName: "svc1", DestinationNamespace: "default", DestinationPartition: "default"}
	svc2Up := structs.Upstream{DestinationName: "svc2", DestinationNamespace: "default", DestinationPartition: "default"}
	ups := listenerRouteUpstreams{}
	ups.set(routeRef, listenerKey, structs.Upstreams{svc1Up, svc2Up})

	// Simulate the coalesce-window race: svc2 chain arrived but its endpoint
	// watch has not fired yet (no entry in WatchedUpstreamEndpoints for svc2).
	snapPartial := &configSnapshotAPIGateway{
		GatewayConfigLoaded:      true,
		BoundGatewayConfigLoaded: true,
		ConfigSnapshotUpstreams: ConfigSnapshotUpstreams{
			Leaf: &structs.IssuedCert{},
			DiscoveryChain: map[UpstreamID]*structs.CompiledDiscoveryChain{
				svc1UID: svc1Chain,
				svc2UID: svc2Chain, // svc2 chain present but endpoints not yet
			},
			WatchedUpstreamEndpoints: map[UpstreamID]map[string]structs.CheckServiceNodes{
				svc1UID: {svc1TargetID: structs.CheckServiceNodes{}}, // svc1 ready
				// svc2 endpoint watch has NOT fired yet — this is the race window
			},
			WatchedGatewayEndpoints: map[UpstreamID]map[string]structs.CheckServiceNodes{},
		},
		Upstreams: ups,
	}
	wireReadyListener(snapPartial, listenerKey, routeRef)

	// Approach B: valid() must NOT gate on endpoints. proxycfg keeps delivering;
	// the cold-start coherence gate is applied per-stream in the xDS layer.
	require.True(t, snapPartial.valid(),
		"approach B: valid() must stay true during the race — the stream-level gate "+
			"handles cold-start CDS/EDS coherence, so proxycfg is never starved")

	// The completeness predicate (consumed by the xDS stream gate) must report
	// svc2's target as missing during the race window.
	require.ElementsMatch(t,
		[]string{svc2UID.String() + "/" + svc2TargetID},
		snapPartial.discoveryChainsMissingEndpoints(localKey),
		"the predicate must report svc2's target as missing so the stream gate "+
			"holds the first push (cold-start segfault fix, ITCO-15826)")

	// Simulate the endpoint watch for svc2 firing — now the snapshot is complete.
	snapPartial.WatchedUpstreamEndpoints[svc2UID] = map[string]structs.CheckServiceNodes{
		svc2TargetID: {},
	}
	require.Empty(t, snapPartial.discoveryChainsMissingEndpoints(localKey),
		"once svc2 endpoints arrive the predicate reports complete and the stream "+
			"gate opens")
}
