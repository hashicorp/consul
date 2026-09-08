// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package proxycfg

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/proxycfg/internal/watch"
	"github.com/hashicorp/consul/agent/structs"
)

// wireReadyListener attaches the listener, bound-listener and route-config
// state that activeUpstreamIDs walks when deciding which upstreams reach CDS.
// It reproduces what getReadyListeners (agent/xds/listeners_apigateway.go)
// requires: an operator listener config (Port and Protocol) with no Conflicted
// condition, a bound listener that lists the route, and the route config entry
// itself. Upstreams set on a snapshot without this wiring are invisible to the
// gate, which is the point of the tightening — the gate must only wait on
// upstreams cluster generation will actually emit.
func wireReadyListener(c *configSnapshotAPIGateway, key APIGatewayListenerKey, routeRefs ...structs.ResourceReference) {
	const name = "listener-1"
	if c.GatewayConfig == nil {
		c.GatewayConfig = &structs.APIGatewayConfigEntry{Name: "gw"}
	}
	if c.Listeners == nil {
		c.Listeners = map[string]structs.APIGatewayListener{}
	}
	if c.BoundListeners == nil {
		c.BoundListeners = map[string]structs.BoundAPIGatewayListener{}
	}
	// The operator config (Port, Protocol, ...) lives on the APIGatewayListener
	// itself; getReadyListeners derives the listener key from it, not from the
	// bound listener, which only carries the controller's binding result
	// (Routes/Certificates).
	c.Listeners[name] = structs.APIGatewayListener{
		Name:     name,
		Protocol: structs.APIGatewayListenerProtocol(key.Protocol),
		Port:     key.Port,
	}
	c.BoundListeners[name] = structs.BoundAPIGatewayListener{
		Name:   name,
		Routes: routeRefs,
	}

	c.HTTPRoutes = watch.NewMap[structs.ResourceReference, *structs.HTTPRouteConfigEntry]()
	c.TCPRoutes = watch.NewMap[structs.ResourceReference, *structs.TCPRouteConfigEntry]()
	for _, ref := range routeRefs {
		switch ref.Kind {
		case structs.HTTPRoute:
			c.HTTPRoutes.InitWatch(ref, func() {})
			c.HTTPRoutes.Set(ref, &structs.HTTPRouteConfigEntry{Name: ref.Name})
		case structs.TCPRoute:
			c.TCPRoutes.InitWatch(ref, func() {})
			c.TCPRoutes.Set(ref, &structs.TCPRouteConfigEntry{Name: ref.Name})
		}
	}
}

// TestConfigSnapshotAPIGateway_bootstrapGate covers the api-gateway completeness
// predicate that the xDS layer uses to hold a gateway's FIRST push (ITCO-15826),
// and asserts that valid() itself does NOT gate on endpoints in this approach —
// proxycfg keeps delivering snapshots so steady-state churn is never starved;
// the gate lives per-stream in agent/xds/delta.go instead.
//
// The predicate carries the #13259 hardening: it ignores orphan chains (not
// wired to any listener upstream) and also gates on mesh-gateway endpoints for
// remote/local-mode targets, mirroring makeLoadAssignmentEndpointGroup.
func TestConfigSnapshotAPIGateway_bootstrapGate(t *testing.T) {
	const targetID = "web.default.default.dc1"
	uid := NewUpstreamIDFromServiceName(structs.NewServiceName("web", nil))

	// localKey is the gateway's own locality; it only affects mesh-gateway
	// targets whose locality differs from it.
	localKey := GatewayKey{Datacenter: "dc1"}

	routeRef := structs.ResourceReference{Kind: structs.HTTPRoute, Name: "route-1"}
	listenerKey := APIGatewayListenerKey{Protocol: "http", Port: 8080}

	newAPIGW := func() *configSnapshotAPIGateway {
		c := &configSnapshotAPIGateway{
			GatewayConfigLoaded:      true,
			BoundGatewayConfigLoaded: true,
		}
		c.Leaf = &structs.IssuedCert{}
		c.DiscoveryChain = map[UpstreamID]*structs.CompiledDiscoveryChain{
			uid: {
				ServiceName: "web",
				Targets: map[string]*structs.DiscoveryTarget{
					targetID: {ID: targetID, Service: "web"},
				},
			},
		}
		// Wire the chain to a route on a ready listener so it is NOT skipped by
		// discoveryChainsMissingEndpoints, which only gates on chains that reach
		// a CDS cluster — see activeUpstreamIDs.
		c.Upstreams = listenerRouteUpstreams{}
		c.Upstreams.set(routeRef, listenerKey,
			structs.Upstreams{{DestinationName: "web"}})
		wireReadyListener(c, listenerKey, routeRef)
		c.WatchedUpstreamEndpoints = map[UpstreamID]map[string]structs.CheckServiceNodes{}
		c.WatchedGatewayEndpoints = map[UpstreamID]map[string]structs.CheckServiceNodes{}
		return c
	}

	t.Run("chain present, endpoints missing => incomplete but still valid()", func(t *testing.T) {
		c := newAPIGW()
		require.NotEmpty(t, c.discoveryChainsMissingEndpoints(localKey))
		// The completeness gate is enforced in the xDS layer, not here, so proxycfg
		// must still consider the snapshot deliverable (no whole-snapshot withholding).
		require.True(t, c.valid())
	})

	t.Run("endpoints present (even empty) => complete", func(t *testing.T) {
		c := newAPIGW()
		c.WatchedUpstreamEndpoints[uid] = map[string]structs.CheckServiceNodes{targetID: {}}
		require.Empty(t, c.discoveryChainsMissingEndpoints(localKey))
	})

	t.Run("missing targets reported for diagnostics", func(t *testing.T) {
		c := newAPIGW()
		require.ElementsMatch(t, []string{uid.String() + "/" + targetID}, c.discoveryChainsMissingEndpoints(localKey))
	})

	t.Run("external and peered targets never block completeness", func(t *testing.T) {
		c := newAPIGW()
		c.DiscoveryChain[uid].Targets[targetID].External = true
		require.Empty(t, c.discoveryChainsMissingEndpoints(localKey))

		c = newAPIGW()
		c.DiscoveryChain[uid].Targets[targetID].Peer = "peer1"
		require.Empty(t, c.discoveryChainsMissingEndpoints(localKey))
	})

	t.Run("orphan chain (no listener upstream) never blocks completeness", func(t *testing.T) {
		c := newAPIGW()
		// Drop the listener-upstream wiring: the chain is now an orphan (a
		// registered watch target that never resolved to a listener upstream,
		// e.g. a backend-not-found route). It emits no CDS cluster, so it must
		// not block admission even though its endpoints are absent.
		c.Upstreams = listenerRouteUpstreams{}
		require.Empty(t, c.discoveryChainsMissingEndpoints(localKey))

		// Control: wiring the very same chain to a listener makes it block, so
		// the orphan exclusion is what emptied the result above rather than some
		// unrelated property of the fixture.
		c.Upstreams.set(routeRef, listenerKey,
			structs.Upstreams{{DestinationName: "web"}})
		require.NotEmpty(t, c.discoveryChainsMissingEndpoints(localKey))
	})

	t.Run("remote mesh-gateway target gates on gateway endpoints", func(t *testing.T) {
		c := newAPIGW()
		// Target in another DC routed via a remote mesh gateway. Its own
		// endpoints are present, but the mesh-gateway endpoints are not, which is
		// still a CDS-without-EDS state (makeLoadAssignmentEndpointGroup uses the
		// gateway endpoints for these targets).
		tgt := c.DiscoveryChain[uid].Targets[targetID]
		tgt.Datacenter = "dc2"
		tgt.MeshGateway = structs.MeshGatewayConfig{Mode: structs.MeshGatewayModeRemote}
		c.WatchedUpstreamEndpoints[uid] = map[string]structs.CheckServiceNodes{targetID: {}}

		gwKey := GatewayKey{Datacenter: "dc2"}
		require.ElementsMatch(t,
			[]string{uid.String() + "/" + targetID + "@" + gwKey.String()},
			c.discoveryChainsMissingEndpoints(localKey))

		// Once the mesh-gateway endpoints arrive, the target is complete.
		c.WatchedGatewayEndpoints[uid] = map[string]structs.CheckServiceNodes{gwKey.String(): {}}
		require.Empty(t, c.discoveryChainsMissingEndpoints(localKey))
	})

	t.Run("local mesh-gateway target gates on local gateway endpoints", func(t *testing.T) {
		c := newAPIGW()
		// Local mode reaches a remote DC through this gateway's OWN local mesh
		// gateway, so the key is localKey rather than the target's locality.
		tgt := c.DiscoveryChain[uid].Targets[targetID]
		tgt.Datacenter = "dc2"
		tgt.MeshGateway = structs.MeshGatewayConfig{Mode: structs.MeshGatewayModeLocal}
		c.WatchedUpstreamEndpoints[uid] = map[string]structs.CheckServiceNodes{targetID: {}}

		require.ElementsMatch(t,
			[]string{uid.String() + "/" + targetID + "@" + localKey.String()},
			c.discoveryChainsMissingEndpoints(localKey))

		c.WatchedGatewayEndpoints[uid] = map[string]structs.CheckServiceNodes{localKey.String(): {}}
		require.Empty(t, c.discoveryChainsMissingEndpoints(localKey))
	})

	t.Run("mesh-gateway target in our own locality is not gated", func(t *testing.T) {
		// localKey.Matches: the target is local, so EDS serves it from the
		// target endpoints and never consults WatchedGatewayEndpoints.
		for _, mode := range []structs.MeshGatewayMode{
			structs.MeshGatewayModeRemote,
			structs.MeshGatewayModeLocal,
		} {
			c := newAPIGW()
			tgt := c.DiscoveryChain[uid].Targets[targetID]
			tgt.Datacenter = localKey.Datacenter
			tgt.Partition = localKey.Partition
			tgt.MeshGateway = structs.MeshGatewayConfig{Mode: mode}
			c.WatchedUpstreamEndpoints[uid] = map[string]structs.CheckServiceNodes{targetID: {}}

			require.Empty(t, c.discoveryChainsMissingEndpoints(localKey), "mode %q", mode)
		}
	})

	t.Run("empty gateway key is not gated", func(t *testing.T) {
		// Mirrors makeLoadAssignmentEndpointGroup's gatewayKey.IsEmpty() early
		// return: with no mesh-gateway mode set, or a local-mode gateway whose
		// own locality is unset, no gateway is consulted and gating on one would
		// withhold a cluster Envoy can serve.
		c := newAPIGW()
		c.DiscoveryChain[uid].Targets[targetID].Datacenter = "dc2"
		c.WatchedUpstreamEndpoints[uid] = map[string]structs.CheckServiceNodes{targetID: {}}
		require.Empty(t, c.discoveryChainsMissingEndpoints(localKey),
			"no mesh-gateway mode means no gateway lookup")

		c = newAPIGW()
		tgt := c.DiscoveryChain[uid].Targets[targetID]
		tgt.Datacenter = "dc2"
		tgt.MeshGateway = structs.MeshGatewayConfig{Mode: structs.MeshGatewayModeLocal}
		c.WatchedUpstreamEndpoints[uid] = map[string]structs.CheckServiceNodes{targetID: {}}
		require.Empty(t, c.discoveryChainsMissingEndpoints(GatewayKey{}),
			"local mode with an empty locality yields an empty gateway key")
	})

	t.Run("route on a conflicted listener is not gated", func(t *testing.T) {
		// getReadyListeners skips listeners whose status carries Conflicted=True,
		// so cluster generation never emits a cluster for their routes. The gate
		// must skip them too, or it withholds the whole gateway's first push
		// waiting on endpoints for a cluster that is never published.
		c := newAPIGW()
		require.NotEmpty(t, c.discoveryChainsMissingEndpoints(localKey),
			"precondition: a ready listener does gate on this chain")

		c.GatewayConfig.Status = structs.Status{Conditions: []structs.Condition{{
			Type:   "Conflicted",
			Status: "True",
			Resource: &structs.ResourceReference{
				Kind:        structs.APIGateway,
				Name:        c.GatewayConfig.Name,
				SectionName: "listener-1",
			},
		}}}
		require.False(t, c.GatewayConfig.ListenerIsReady("listener-1"),
			"precondition: the listener is now unready")
		require.Empty(t, c.discoveryChainsMissingEndpoints(localKey))
	})

	t.Run("route not bound to the listener is not gated", func(t *testing.T) {
		// Upstreams is built from the route's declared parentRefs
		// (referenceIsForListener), and a parentRef with no sectionName matches
		// every listener on the gateway. BoundListeners is the controller's
		// authoritative binding result, so it is the narrower set getReadyListeners
		// walks. A route that declared an intent but never bound emits no cluster.
		c := newAPIGW()
		// Keep the listener fully wired (Listeners entry with Port + Protocol
		// set by wireReadyListener) so the only thing that changed is the
		// missing route binding.
		c.BoundListeners["listener-1"] = structs.BoundAPIGatewayListener{
			Name: "listener-1",
		}
		require.Empty(t, c.discoveryChainsMissingEndpoints(localKey))
	})

	t.Run("route bound but whose config entry has not arrived is not gated", func(t *testing.T) {
		// getReadyListeners skips route refs that are not present in
		// HTTPRoutes/TCPRoutes, so the same ordering must not gate.
		c := newAPIGW()
		c.HTTPRoutes.CancelWatch(routeRef)
		require.Empty(t, c.discoveryChainsMissingEndpoints(localKey))
	})

	t.Run("exported ConfigSnapshot accessor used by the xDS layer", func(t *testing.T) {
		// api-gateway kind: delegates to the sub-snapshot.
		incomplete := &ConfigSnapshot{Kind: structs.ServiceKindAPIGateway, APIGateway: *newAPIGW()}
		require.ElementsMatch(t, []string{uid.String() + "/" + targetID}, incomplete.APIGatewayDiscoveryChainsMissingEndpoints())

		complete := newAPIGW()
		complete.WatchedUpstreamEndpoints[uid] = map[string]structs.CheckServiceNodes{targetID: {}}
		full := &ConfigSnapshot{Kind: structs.ServiceKindAPIGateway, APIGateway: *complete}
		require.Empty(t, full.APIGatewayDiscoveryChainsMissingEndpoints())

		// non-api-gateway kinds are never gated by the xDS bootstrap check.
		sidecar := &ConfigSnapshot{Kind: structs.ServiceKindConnectProxy}
		require.Nil(t, sidecar.APIGatewayDiscoveryChainsMissingEndpoints())
	})
}

// TestDiscoveryChainsMissingEndpoints_MirrorsEndpointGeneration pins the gate's
// mesh-gateway logic to makeLoadAssignmentEndpointGroup (agent/xds/endpoints.go),
// which decides whether EDS actually emits an assignment for a target.
//
// The gate must block exactly when that function would skip the cluster. Blocking
// less would let the cold-start incoherence through; blocking more withholds
// config for a cluster Envoy could serve, which at best delays the first push to
// the bootstrap-gate timeout. edsWouldSkip below is transcribed from that
// function, so this test fails if either side drifts.
func TestDiscoveryChainsMissingEndpoints_MirrorsEndpointGeneration(t *testing.T) {
	const targetID = "web.default.default.dc2"
	uid := NewUpstreamIDFromServiceName(structs.NewServiceName("web", nil))
	routeRef := structs.ResourceReference{Kind: structs.HTTPRoute, Name: "route-1"}
	listenerKey := APIGatewayListenerKey{Protocol: "http", Port: 8080}

	// Transcribed from makeLoadAssignmentEndpointGroup. api-gateway clusters are
	// always generated with forMeshGateway=false, so that argument is omitted.
	edsWouldSkip := func(c *configSnapshotAPIGateway, localKey GatewayKey) bool {
		target := c.DiscoveryChain[uid].Targets[targetID]
		if _, ok := c.WatchedUpstreamEndpoints[uid][targetID]; !ok {
			return true
		}
		var gatewayKey GatewayKey
		switch target.MeshGateway.Mode {
		case structs.MeshGatewayModeRemote:
			gatewayKey.Datacenter = target.Datacenter
			gatewayKey.Partition = target.Partition
		case structs.MeshGatewayModeLocal:
			gatewayKey = localKey
		}
		if gatewayKey.IsEmpty() || localKey.Matches(target.Datacenter, target.Partition) {
			return false
		}
		_, ok := c.WatchedGatewayEndpoints[uid][gatewayKey.String()]
		return !ok
	}

	build := func(mode structs.MeshGatewayMode, targetDC, targetPartition string, haveTargetEndpoints bool) *configSnapshotAPIGateway {
		c := &configSnapshotAPIGateway{GatewayConfigLoaded: true, BoundGatewayConfigLoaded: true}
		c.Leaf = &structs.IssuedCert{}
		c.DiscoveryChain = map[UpstreamID]*structs.CompiledDiscoveryChain{
			uid: {ServiceName: "web", Targets: map[string]*structs.DiscoveryTarget{
				targetID: {
					ID:          targetID,
					Service:     "web",
					Datacenter:  targetDC,
					Partition:   targetPartition,
					MeshGateway: structs.MeshGatewayConfig{Mode: mode},
				},
			}},
		}
		c.Upstreams = listenerRouteUpstreams{}
		c.Upstreams.set(routeRef, listenerKey,
			structs.Upstreams{{DestinationName: "web"}})
		wireReadyListener(c, listenerKey, routeRef)
		c.WatchedUpstreamEndpoints = map[UpstreamID]map[string]structs.CheckServiceNodes{}
		if haveTargetEndpoints {
			c.WatchedUpstreamEndpoints[uid] = map[string]structs.CheckServiceNodes{targetID: {}}
		}
		c.WatchedGatewayEndpoints = map[UpstreamID]map[string]structs.CheckServiceNodes{}
		return c
	}

	localDC := GatewayKey{Datacenter: "dc1", Partition: "default"}

	cases := []struct {
		name             string
		mode             structs.MeshGatewayMode
		targetDC         string
		targetPartition  string
		localKey         GatewayKey
		targetEndpoints  bool
		gatewayEndpoints bool
	}{
		{name: "no target endpoints", mode: structs.MeshGatewayModeNone, targetDC: "dc1", targetPartition: "default", localKey: localDC},
		{name: "local target, no gateway", mode: structs.MeshGatewayModeNone, targetDC: "dc1", targetPartition: "default", localKey: localDC, targetEndpoints: true},
		{name: "remote dc, remote mode, gateway missing", mode: structs.MeshGatewayModeRemote, targetDC: "dc2", targetPartition: "default", localKey: localDC, targetEndpoints: true},
		{name: "remote dc, remote mode, gateway present", mode: structs.MeshGatewayModeRemote, targetDC: "dc2", targetPartition: "default", localKey: localDC, targetEndpoints: true, gatewayEndpoints: true},
		{name: "remote dc, local mode, gateway missing", mode: structs.MeshGatewayModeLocal, targetDC: "dc2", targetPartition: "default", localKey: localDC, targetEndpoints: true},
		{name: "remote dc, local mode, gateway present", mode: structs.MeshGatewayModeLocal, targetDC: "dc2", targetPartition: "default", localKey: localDC, targetEndpoints: true, gatewayEndpoints: true},
		{name: "same dc, remote mode", mode: structs.MeshGatewayModeRemote, targetDC: "dc1", targetPartition: "default", localKey: localDC, targetEndpoints: true},
		{name: "same dc, local mode", mode: structs.MeshGatewayModeLocal, targetDC: "dc1", targetPartition: "default", localKey: localDC, targetEndpoints: true},
		{name: "remote dc, no mesh-gateway mode", mode: structs.MeshGatewayModeNone, targetDC: "dc2", targetPartition: "default", localKey: localDC, targetEndpoints: true},
		{name: "remote mode, target has no dc", mode: structs.MeshGatewayModeRemote, targetDC: "", targetPartition: "", localKey: localDC, targetEndpoints: true},
		{name: "local mode, empty local key", mode: structs.MeshGatewayModeLocal, targetDC: "dc2", targetPartition: "default", localKey: GatewayKey{}, targetEndpoints: true},
		{name: "remote mode, empty local key", mode: structs.MeshGatewayModeRemote, targetDC: "dc2", targetPartition: "default", localKey: GatewayKey{}, targetEndpoints: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := build(tc.mode, tc.targetDC, tc.targetPartition, tc.targetEndpoints)
			if tc.gatewayEndpoints {
				gwKey := GatewayKey{Datacenter: tc.targetDC, Partition: tc.targetPartition}
				if tc.mode == structs.MeshGatewayModeLocal {
					gwKey = tc.localKey
				}
				c.WatchedGatewayEndpoints[uid] = map[string]structs.CheckServiceNodes{gwKey.String(): {}}
			}

			gateBlocks := len(c.discoveryChainsMissingEndpoints(tc.localKey)) > 0
			require.Equal(t, edsWouldSkip(c, tc.localKey), gateBlocks,
				"the gate must block exactly when EDS would skip the cluster")
		})
	}
}
