package xds

import (
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	envoy_discovery_v3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"

	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/require"
	rpcstatus "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
)

// NOTE: For these tests, prefer not using xDS protobuf "factory" methods if
// possible to avoid using them to test themselves.
//
// Stick to very straightforward stuff in xds_protocol_helpers_test.go.

func TestServer_DeltaAggregatedResources_v3_BasicProtocol_TCP(t *testing.T) {
	aclResolve := func(id string) (acl.Authorizer, error) {
		// Allow all
		return acl.RootAuthorizer("manage"), nil
	}
	scenario := newTestServerDeltaScenario(t, aclResolve, "web-sidecar-proxy", "", 0)
	mgr, errCh, envoy := scenario.mgr, scenario.errCh, scenario.envoy

	sid := structs.NewServiceID("web-sidecar-proxy", nil)

	// Register the proxy to create state needed to Watch() on
	mgr.RegisterProxy(t, sid)

	var snap *proxycfg.ConfigSnapshot

	runStep(t, "initial setup", func(t *testing.T) {
		snap = newTestSnapshot(t, nil, "")

		// Send initial cluster discover. We'll assume we are testing a partial
		// reconnect and include some initial resource versions that will be
		// cleaned up.
		envoy.SendDeltaReq(t, ClusterType, &envoy_discovery_v3.DeltaDiscoveryRequest{
			InitialResourceVersions: mustMakeVersionMap(t,
				makeTestCluster(t, snap, "tcp:geo-cache"),
			),
		})

		// Check no response sent yet
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

		requireProtocolVersionGauge(t, scenario, "v3", 1)

		// Deliver a new snapshot (tcp with one tcp upstream)
		mgr.DeliverConfig(t, sid, snap)
	})

	runStep(t, "first sync", func(t *testing.T) {
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: ClusterType,
			Nonce:   hexString(1),
			Resources: makeTestResources(t,
				makeTestCluster(t, snap, "tcp:local_app"),
				makeTestCluster(t, snap, "tcp:db"),
				// SAME_AS_INITIAL_VERSION: makeTestCluster(t, snap, "tcp:geo-cache"),
			),
		})

		// Envoy then tries to discover endpoints for those clusters.
		envoy.SendDeltaReq(t, EndpointType, &envoy_discovery_v3.DeltaDiscoveryRequest{
			// We'll assume we are testing a partial "reconnect"
			InitialResourceVersions: mustMakeVersionMap(t,
				makeTestEndpoints(t, snap, "tcp:geo-cache"),
			),
			ResourceNamesSubscribe: []string{
				"db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul",
				// "geo-cache.default.dc1.query.11111111-2222-3333-4444-555555555555.consul",
				//
				// Include "fake-endpoints" here to test subscribing to an unknown
				// thing and have consul tell us there's no data for it.
				"fake-endpoints",
			},
		})

		// It also (in parallel) issues the cluster ACK
		envoy.SendDeltaReqACK(t, ClusterType, 1)

		// We should get a response immediately since the config is already present in
		// the server for endpoints. Note that this should not be racy if the server
		// is behaving well since the Cluster send above should be blocked until we
		// deliver a new config version.
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: EndpointType,
			Nonce:   hexString(2),
			Resources: makeTestResources(t,
				makeTestEndpoints(t, snap, "tcp:db"),
				// SAME_AS_INITIAL_VERSION: makeTestEndpoints(t, snap, "tcp:geo-cache"),
				// SAME_AS_INITIAL_VERSION: "fake-endpoints",
			),
		})

		// And no other response yet
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

		// Envoy now sends listener request
		envoy.SendDeltaReq(t, ListenerType, nil)

		// It also (in parallel) issues the endpoint ACK
		envoy.SendDeltaReqACK(t, EndpointType, 2)

		// And should get a response immediately.
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: ListenerType,
			Nonce:   hexString(3),
			Resources: makeTestResources(t,
				makeTestListener(t, snap, "tcp:public_listener"),
				makeTestListener(t, snap, "tcp:db"),
				makeTestListener(t, snap, "tcp:geo-cache"),
			),
		})

		// And no other response yet
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

		// ACKs the listener
		envoy.SendDeltaReqACK(t, ListenerType, 3)

		// If we re-subscribe to something even if there are no changes we get a
		// fresh copy.
		envoy.SendDeltaReq(t, EndpointType, &envoy_discovery_v3.DeltaDiscoveryRequest{
			ResourceNamesSubscribe: []string{
				"geo-cache.default.dc1.query.11111111-2222-3333-4444-555555555555.consul",
			},
		})

		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: EndpointType,
			Nonce:   hexString(4),
			Resources: makeTestResources(t,
				makeTestEndpoints(t, snap, "tcp:geo-cache"),
			),
		})

		envoy.SendDeltaReqACK(t, EndpointType, 4)

		// And no other response yet
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)
	})

	deleteAllButOneEndpoint := func(snap *proxycfg.ConfigSnapshot, svc, targetID string) {
		snap.ConnectProxy.ConfigSnapshotUpstreams.WatchedUpstreamEndpoints[svc][targetID] =
			snap.ConnectProxy.ConfigSnapshotUpstreams.WatchedUpstreamEndpoints[svc][targetID][0:1]
	}

	runStep(t, "avoid sending config for unsubscribed resource", func(t *testing.T) {
		envoy.SendDeltaReq(t, EndpointType, &envoy_discovery_v3.DeltaDiscoveryRequest{
			ResourceNamesUnsubscribe: []string{
				"db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul",
			},
		})

		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

		// now reconfigure the snapshot and JUST edit the endpoints to strike one of the two current endpoints for DB
		snap = newTestSnapshot(t, snap, "")
		deleteAllButOneEndpoint(snap, "db", "db.default.default.dc1")
		mgr.DeliverConfig(t, sid, snap)

		// We never send an EDS reply about this change.
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)
	})

	runStep(t, "restore endpoint subscription", func(t *testing.T) {
		// Fix the snapshot
		snap = newTestSnapshot(t, snap, "")
		mgr.DeliverConfig(t, sid, snap)

		// We never send an EDS reply about this change.
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

		// and fix the subscription
		envoy.SendDeltaReq(t, EndpointType, &envoy_discovery_v3.DeltaDiscoveryRequest{
			ResourceNamesSubscribe: []string{
				"db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul",
			},
		})
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: EndpointType,
			Nonce:   hexString(5),
			Resources: makeTestResources(t,
				makeTestEndpoints(t, snap, "tcp:db"),
			),
		})

		envoy.SendDeltaReqACK(t, EndpointType, 5)

		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)
	})

	runStep(t, "simulate envoy NACKing an endpoint update", func(t *testing.T) {
		// Trigger only an EDS update.
		snap = newTestSnapshot(t, snap, "")
		deleteAllButOneEndpoint(snap, "db", "db.default.default.dc1")
		mgr.DeliverConfig(t, sid, snap)

		// Send envoy an EDS update.
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: EndpointType,
			Nonce:   hexString(6),
			Resources: makeTestResources(t,
				makeTestEndpoints(t, snap, "tcp:db[0]"),
			),
		})

		envoy.SendDeltaReqNACK(t, EndpointType, 6, &rpcstatus.Status{})

		// Send it again.
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: EndpointType,
			Nonce:   hexString(7),
			Resources: makeTestResources(t,
				makeTestEndpoints(t, snap, "tcp:db[0]"),
			),
		})

		envoy.SendDeltaReqACK(t, EndpointType, 7)

		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)
	})

	// NOTE: this has to be the last subtest since it kills the stream
	runStep(t, "simulate an envoy error sending an update to envoy", func(t *testing.T) {
		// Force sends to fail
		envoy.SetSendErr(errors.New("test error"))

		// Trigger only an EDS update (flipping BACK to 2 endpoints in the LBassignment)
		snap = newTestSnapshot(t, snap, "")
		mgr.DeliverConfig(t, sid, snap)

		// We never send any replies about this change because we died.
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)
	})

	envoy.Close()
	select {
	case err := <-errCh:
		// Envoy died.
		expect := status.Errorf(codes.Unavailable,
			"failed to send upsert reply for type %q: test error",
			EndpointType)
		require.EqualError(t, err, expect.Error())
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("timed out waiting for handler to finish")
	}
}

func TestServer_DeltaAggregatedResources_v3_BasicProtocol_HTTP2(t *testing.T) {
	aclResolve := func(id string) (acl.Authorizer, error) {
		// Allow all
		return acl.RootAuthorizer("manage"), nil
	}
	scenario := newTestServerDeltaScenario(t, aclResolve, "web-sidecar-proxy", "", 0)
	mgr, errCh, envoy := scenario.mgr, scenario.errCh, scenario.envoy

	sid := structs.NewServiceID("web-sidecar-proxy", nil)

	// Register the proxy to create state needed to Watch() on
	mgr.RegisterProxy(t, sid)

	// Send initial cluster discover (empty payload)
	envoy.SendDeltaReq(t, ClusterType, nil)

	// Check no response sent yet
	assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

	// Deliver a new snapshot (tcp with one http upstream)
	snap := newTestSnapshot(t, nil, "http2", &structs.ServiceConfigEntry{
		Kind:     structs.ServiceDefaults,
		Name:     "db",
		Protocol: "http2",
	})
	mgr.DeliverConfig(t, sid, snap)

	runStep(t, "no-rds", func(t *testing.T) {
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: ClusterType,
			Nonce:   hexString(1),
			Resources: makeTestResources(t,
				makeTestCluster(t, snap, "tcp:local_app"),
				makeTestCluster(t, snap, "http2:db"),
				makeTestCluster(t, snap, "tcp:geo-cache"),
			),
		})

		// Envoy then tries to discover endpoints for those clusters.
		envoy.SendDeltaReq(t, EndpointType, &envoy_discovery_v3.DeltaDiscoveryRequest{
			ResourceNamesSubscribe: []string{
				"db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul",
				"geo-cache.default.dc1.query.11111111-2222-3333-4444-555555555555.consul",
			},
		})

		// It also (in parallel) issues the cluster ACK
		envoy.SendDeltaReqACK(t, ClusterType, 1)

		// We should get a response immediately since the config is already present in
		// the server for endpoints. Note that this should not be racy if the server
		// is behaving well since the Cluster send above should be blocked until we
		// deliver a new config version.
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: EndpointType,
			Nonce:   hexString(2),
			Resources: makeTestResources(t,
				makeTestEndpoints(t, snap, "http2:db"),
				makeTestEndpoints(t, snap, "tcp:geo-cache"),
			),
		})

		// And no other response yet
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

		// Envoy now sends listener request
		envoy.SendDeltaReq(t, ListenerType, nil)

		// It also (in parallel) issues the endpoint ACK
		envoy.SendDeltaReqACK(t, EndpointType, 2)

		// And should get a response immediately.
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: ListenerType,
			Nonce:   hexString(3),
			Resources: makeTestResources(t,
				makeTestListener(t, snap, "tcp:public_listener"),
				makeTestListener(t, snap, "http2:db"),
				makeTestListener(t, snap, "tcp:geo-cache"),
			),
		})

		// And no other response yet
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

		// ACKs the listener
		envoy.SendDeltaReqACK(t, ListenerType, 3)

		// And no other response yet
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)
	})

	// -- reconfigure with a no-op discovery chain

	snap = newTestSnapshot(t, snap, "http2", &structs.ServiceConfigEntry{
		Kind:     structs.ServiceDefaults,
		Name:     "db",
		Protocol: "http2",
	}, &structs.ServiceRouterConfigEntry{
		Kind:   structs.ServiceRouter,
		Name:   "db",
		Routes: nil,
	})
	mgr.DeliverConfig(t, sid, snap)

	runStep(t, "with-rds", func(t *testing.T) {
		// Just the "db" listener sees a change
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: ListenerType,
			Nonce:   hexString(4),
			Resources: makeTestResources(t,
				makeTestListener(t, snap, "http2:db:rds"),
			),
		})

		// And no other response yet
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

		// Envoy now sends routes request
		envoy.SendDeltaReq(t, RouteType, &envoy_discovery_v3.DeltaDiscoveryRequest{
			ResourceNamesSubscribe: []string{
				"db",
			},
		})

		// ACKs the listener
		envoy.SendDeltaReqACK(t, ListenerType, 4)

		// And should get a response immediately.
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: RouteType,
			Nonce:   hexString(5),
			Resources: makeTestResources(t,
				makeTestRoute(t, "http2:db"),
			),
		})

		envoy.SendDeltaReqACK(t, RouteType, 5)

		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)
	})

	envoy.Close()
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("timed out waiting for handler to finish")
	}
}

func TestServer_DeltaAggregatedResources_v3_SlowEndpointPopulation(t *testing.T) {
	// This illustrates a scenario related to https://github.com/hashicorp/consul/issues/10563

	aclResolve := func(id string) (acl.Authorizer, error) {
		// Allow all
		return acl.RootAuthorizer("manage"), nil
	}
	scenario := newTestServerDeltaScenario(t, aclResolve, "web-sidecar-proxy", "", 0)
	server, mgr, errCh, envoy := scenario.server, scenario.mgr, scenario.errCh, scenario.envoy

	// This mutateFn causes any endpoint with a name containing "geo-cache" to be
	// omitted from the response while the hack is active.
	var slowHackDisabled uint32
	server.ResourceMapMutateFn = func(resourceMap *IndexedResources) {
		if atomic.LoadUint32(&slowHackDisabled) == 1 {
			return
		}
		if em, ok := resourceMap.Index[EndpointType]; ok {
			for k := range em {
				if strings.Contains(k, "geo-cache") {
					delete(em, k)
				}
			}
		}
	}

	sid := structs.NewServiceID("web-sidecar-proxy", nil)

	// Register the proxy to create state needed to Watch() on
	mgr.RegisterProxy(t, sid)

	var snap *proxycfg.ConfigSnapshot
	runStep(t, "get into initial state", func(t *testing.T) {
		snap = newTestSnapshot(t, nil, "")

		// Send initial cluster discover.
		envoy.SendDeltaReq(t, ClusterType, &envoy_discovery_v3.DeltaDiscoveryRequest{})

		// Check no response sent yet
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

		requireProtocolVersionGauge(t, scenario, "v3", 1)

		// Deliver a new snapshot (tcp with one tcp upstream)
		mgr.DeliverConfig(t, sid, snap)

		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: ClusterType,
			Nonce:   hexString(1),
			Resources: makeTestResources(t,
				makeTestCluster(t, snap, "tcp:local_app"),
				makeTestCluster(t, snap, "tcp:db"),
				makeTestCluster(t, snap, "tcp:geo-cache"),
			),
		})

		// Envoy then tries to discover endpoints for those clusters.
		envoy.SendDeltaReq(t, EndpointType, &envoy_discovery_v3.DeltaDiscoveryRequest{
			ResourceNamesSubscribe: []string{
				"db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul",
				"geo-cache.default.dc1.query.11111111-2222-3333-4444-555555555555.consul",
			},
		})

		// It also (in parallel) issues the cluster ACK
		envoy.SendDeltaReqACK(t, ClusterType, 1)

		// We should get a response immediately since the config is already present in
		// the server for endpoints. Note that this should not be racy if the server
		// is behaving well since the Cluster send above should be blocked until we
		// deliver a new config version.
		//
		// NOTE: we do NOT return back geo-cache yet
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: EndpointType,
			Nonce:   hexString(2),
			Resources: makeTestResources(t,
				makeTestEndpoints(t, snap, "tcp:db"),
				// makeTestEndpoints(t, snap, "tcp:geo-cache"),
			),
		})

		// And no other response yet
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

		// Envoy now sends listener request
		envoy.SendDeltaReq(t, ListenerType, nil)

		// It also (in parallel) issues the endpoint ACK
		envoy.SendDeltaReqACK(t, EndpointType, 2)

		// And should get a response immediately.
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: ListenerType,
			Nonce:   hexString(3),
			Resources: makeTestResources(t,
				makeTestListener(t, snap, "tcp:public_listener"),
				makeTestListener(t, snap, "tcp:db"),
				makeTestListener(t, snap, "tcp:geo-cache"),
			),
		})

		// And no other response yet
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

		// ACKs the listener
		envoy.SendDeltaReqACK(t, ListenerType, 3)
	})

	// Disable hack. Need to wait for one more event to wake up the loop.
	atomic.StoreUint32(&slowHackDisabled, 1)

	runStep(t, "delayed endpoint update finally comes in", func(t *testing.T) {
		// Trigger the xds.Server select{} to wake up and notice our hack is disabled.
		// The actual contents of this change are irrelevant.
		snap = newTestSnapshot(t, snap, "")
		mgr.DeliverConfig(t, sid, snap)

		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: EndpointType,
			Nonce:   hexString(4),
			Resources: makeTestResources(t,
				makeTestEndpoints(t, snap, "tcp:geo-cache"),
			),
		})

		// And no other response yet
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

		// It also (in parallel) issues the endpoint ACK
		envoy.SendDeltaReqACK(t, EndpointType, 4)

	})

	envoy.Close()
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("timed out waiting for handler to finish")
	}
}

func TestServer_DeltaAggregatedResources_v3_BasicProtocol_TCP_clusterChangesImpactEndpoints(t *testing.T) {
	aclResolve := func(id string) (acl.Authorizer, error) {
		// Allow all
		return acl.RootAuthorizer("manage"), nil
	}
	scenario := newTestServerDeltaScenario(t, aclResolve, "web-sidecar-proxy", "", 0)
	mgr, errCh, envoy := scenario.mgr, scenario.errCh, scenario.envoy

	sid := structs.NewServiceID("web-sidecar-proxy", nil)

	// Register the proxy to create state needed to Watch() on
	mgr.RegisterProxy(t, sid)

	var snap *proxycfg.ConfigSnapshot
	runStep(t, "get into initial state", func(t *testing.T) {
		snap = newTestSnapshot(t, nil, "")

		// Send initial cluster discover.
		envoy.SendDeltaReq(t, ClusterType, &envoy_discovery_v3.DeltaDiscoveryRequest{})

		// Check no response sent yet
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

		requireProtocolVersionGauge(t, scenario, "v3", 1)

		// Deliver a new snapshot (tcp with one tcp upstream)
		mgr.DeliverConfig(t, sid, snap)

		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: ClusterType,
			Nonce:   hexString(1),
			Resources: makeTestResources(t,
				makeTestCluster(t, snap, "tcp:local_app"),
				makeTestCluster(t, snap, "tcp:db"),
				makeTestCluster(t, snap, "tcp:geo-cache"),
			),
		})

		// Envoy then tries to discover endpoints for those clusters.
		envoy.SendDeltaReq(t, EndpointType, &envoy_discovery_v3.DeltaDiscoveryRequest{
			ResourceNamesSubscribe: []string{
				"db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul",
				"geo-cache.default.dc1.query.11111111-2222-3333-4444-555555555555.consul",
			},
		})

		// It also (in parallel) issues the cluster ACK
		envoy.SendDeltaReqACK(t, ClusterType, 1)

		// We should get a response immediately since the config is already present in
		// the server for endpoints. Note that this should not be racy if the server
		// is behaving well since the Cluster send above should be blocked until we
		// deliver a new config version.
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: EndpointType,
			Nonce:   hexString(2),
			Resources: makeTestResources(t,
				makeTestEndpoints(t, snap, "tcp:db"),
				makeTestEndpoints(t, snap, "tcp:geo-cache"),
			),
		})

		// And no other response yet
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

		// Envoy now sends listener request
		envoy.SendDeltaReq(t, ListenerType, nil)

		// It also (in parallel) issues the endpoint ACK
		envoy.SendDeltaReqACK(t, EndpointType, 2)

		// And should get a response immediately.
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: ListenerType,
			Nonce:   hexString(3),
			Resources: makeTestResources(t,
				makeTestListener(t, snap, "tcp:public_listener"),
				makeTestListener(t, snap, "tcp:db"),
				makeTestListener(t, snap, "tcp:geo-cache"),
			),
		})

		// And no other response yet
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

		// ACKs the listener
		envoy.SendDeltaReqACK(t, ListenerType, 3)
	})

	runStep(t, "trigger cluster update needing implicit endpoint replacements", func(t *testing.T) {
		// Update the snapshot in a way that causes a single cluster update.
		snap = newTestSnapshot(t, snap, "", &structs.ServiceResolverConfigEntry{
			Kind:           structs.ServiceResolver,
			Name:           "db",
			ConnectTimeout: 1337 * time.Second,
		})
		mgr.DeliverConfig(t, sid, snap)

		// The cluster is updated
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: ClusterType,
			Nonce:   hexString(4),
			Resources: makeTestResources(t,
				// SAME makeTestCluster(t, snap, "tcp:local_app"),
				makeTestCluster(t, snap, "tcp:db:timeout"),
				// SAME makeTestCluster(t, snap, "tcp:geo-cache"),
			),
		})

		envoy.SendDeltaReqACK(t, ClusterType, 4)

		// And we re-send the endpoints for the updated cluster after getting the
		// ACK for the cluster.
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: EndpointType,
			Nonce:   hexString(5),
			Resources: makeTestResources(t,
				makeTestEndpoints(t, snap, "tcp:db"),
				// SAME makeTestEndpoints(t, snap, "tcp:geo-cache"),
			),
		})
		envoy.SendDeltaReqACK(t, EndpointType, 5)

		// And no other response yet
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)
	})

	envoy.Close()
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("timed out waiting for handler to finish")
	}
}

func TestServer_DeltaAggregatedResources_v3_BasicProtocol_HTTP2_RDS_listenerChangesImpactRoutes(t *testing.T) {
	aclResolve := func(id string) (acl.Authorizer, error) {
		// Allow all
		return acl.RootAuthorizer("manage"), nil
	}
	scenario := newTestServerDeltaScenario(t, aclResolve, "web-sidecar-proxy", "", 0)
	mgr, errCh, envoy := scenario.mgr, scenario.errCh, scenario.envoy

	sid := structs.NewServiceID("web-sidecar-proxy", nil)

	// Register the proxy to create state needed to Watch() on
	mgr.RegisterProxy(t, sid)

	var snap *proxycfg.ConfigSnapshot

	runStep(t, "get into initial state", func(t *testing.T) {
		// Send initial cluster discover (empty payload)
		envoy.SendDeltaReq(t, ClusterType, nil)

		// Check no response sent yet
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

		// Deliver a new snapshot (tcp with one http upstream with no-op disco chain)
		snap = newTestSnapshot(t, nil, "http2", &structs.ServiceConfigEntry{
			Kind:     structs.ServiceDefaults,
			Name:     "db",
			Protocol: "http2",
		}, &structs.ServiceRouterConfigEntry{
			Kind:   structs.ServiceRouter,
			Name:   "db",
			Routes: nil,
		})
		mgr.DeliverConfig(t, sid, snap)

		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: ClusterType,
			Nonce:   hexString(1),
			Resources: makeTestResources(t,
				makeTestCluster(t, snap, "tcp:local_app"),
				makeTestCluster(t, snap, "http2:db"),
				makeTestCluster(t, snap, "tcp:geo-cache"),
			),
		})

		// Envoy then tries to discover endpoints for those clusters.
		envoy.SendDeltaReq(t, EndpointType, &envoy_discovery_v3.DeltaDiscoveryRequest{
			ResourceNamesSubscribe: []string{
				"db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul",
				"geo-cache.default.dc1.query.11111111-2222-3333-4444-555555555555.consul",
			},
		})

		// It also (in parallel) issues the cluster ACK
		envoy.SendDeltaReqACK(t, ClusterType, 1)

		// We should get a response immediately since the config is already present in
		// the server for endpoints. Note that this should not be racy if the server
		// is behaving well since the Cluster send above should be blocked until we
		// deliver a new config version.
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: EndpointType,
			Nonce:   hexString(2),
			Resources: makeTestResources(t,
				makeTestEndpoints(t, snap, "http2:db"),
				makeTestEndpoints(t, snap, "tcp:geo-cache"),
			),
		})

		// And no other response yet
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

		// Envoy now sends listener request
		envoy.SendDeltaReq(t, ListenerType, nil)

		// It also (in parallel) issues the endpoint ACK
		envoy.SendDeltaReqACK(t, EndpointType, 2)

		// And should get a response immediately.
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: ListenerType,
			Nonce:   hexString(3),
			Resources: makeTestResources(t,
				makeTestListener(t, snap, "tcp:public_listener"),
				makeTestListener(t, snap, "http2:db:rds"),
				makeTestListener(t, snap, "tcp:geo-cache"),
			),
		})

		// And no other response yet
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

		// Envoy now sends routes request
		envoy.SendDeltaReq(t, RouteType, &envoy_discovery_v3.DeltaDiscoveryRequest{
			ResourceNamesSubscribe: []string{
				"db",
			},
		})

		// ACKs the listener
		envoy.SendDeltaReqACK(t, ListenerType, 3)

		// And should get a response immediately.
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: RouteType,
			Nonce:   hexString(4),
			Resources: makeTestResources(t,
				makeTestRoute(t, "http2:db"),
			),
		})

		envoy.SendDeltaReqACK(t, RouteType, 4)

		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)
	})

	runStep(t, "trigger listener update needing implicit route replacements", func(t *testing.T) {
		// Update the snapshot in a way that causes a single listener update.
		//
		// Downgrade from http2 to http
		snap = newTestSnapshot(t, snap, "http", &structs.ServiceConfigEntry{
			Kind:     structs.ServiceDefaults,
			Name:     "db",
			Protocol: "http",
		}, &structs.ServiceRouterConfigEntry{
			Kind:   structs.ServiceRouter,
			Name:   "db",
			Routes: nil,
		})
		mgr.DeliverConfig(t, sid, snap)

		// db cluster is refreshed (unrelated to the test scenario other than it's required)
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: ClusterType,
			Nonce:   hexString(5),
			Resources: makeTestResources(t,
				makeTestCluster(t, snap, "http:db"),
			),
		})

		// the listener is updated
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: ListenerType,
			Nonce:   hexString(6),
			Resources: makeTestResources(t,
				makeTestListener(t, snap, "http:db:rds"),
			),
		})

		envoy.SendDeltaReqACK(t, ClusterType, 5)

		// ACKs the listener
		envoy.SendDeltaReqACK(t, ListenerType, 6)

		// The behaviors of Cluster updates triggering re-sends of Endpoint updates
		// tested in TestServer_DeltaAggregatedResources_v3_BasicProtocol_TCP_clusterChangesImpactEndpoints
		// triggers here. It is not explicitly under test, but we have to get past
		// this exchange to get to the part we care about.

		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: EndpointType,
			Nonce:   hexString(7),
			Resources: makeTestResources(t,
				makeTestEndpoints(t, snap, "http:db"),
			),
		})

		envoy.SendDeltaReqACK(t, EndpointType, 7)

		// THE ACTUAL THING WE CARE ABOUT: replaced route config
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: RouteType,
			Nonce:   hexString(8),
			Resources: makeTestResources(t,
				makeTestRoute(t, "http2:db"),
			),
		})

		envoy.SendDeltaReqACK(t, RouteType, 8)

		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)
	})

	envoy.Close()
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("timed out waiting for handler to finish")
	}
}

func TestServer_DeltaAggregatedResources_v3_ACLEnforcement(t *testing.T) {
	tests := []struct {
		name        string
		defaultDeny bool
		acl         string
		token       string
		wantDenied  bool
		cfgSnap     *proxycfg.ConfigSnapshot
	}{
		// Note that although we've stubbed actual ACL checks in the testManager
		// ConnectAuthorize mock, by asserting against specific reason strings here
		// even in the happy case which can't match the default one returned by the
		// mock we are implicitly validating that the implementation used the
		// correct token from the context.
		{
			name:        "no ACLs configured",
			defaultDeny: false,
			wantDenied:  false,
		},
		{
			name:        "default deny, no token",
			defaultDeny: true,
			wantDenied:  true,
		},
		{
			name:        "default deny, write token",
			defaultDeny: true,
			acl:         `service "web" { policy = "write" }`,
			token:       "service-write-on-web",
			wantDenied:  false,
		},
		{
			name:        "default deny, read token",
			defaultDeny: true,
			acl:         `service "web" { policy = "read" }`,
			token:       "service-write-on-web",
			wantDenied:  true,
		},
		{
			name:        "default deny, write token on different service",
			defaultDeny: true,
			acl:         `service "not-web" { policy = "write" }`,
			token:       "service-write-on-not-web",
			wantDenied:  true,
		},
		{
			name:        "ingress default deny, write token on different service",
			defaultDeny: true,
			acl:         `service "not-ingress" { policy = "write" }`,
			token:       "service-write-on-not-ingress",
			wantDenied:  true,
			cfgSnap:     proxycfg.TestConfigSnapshotIngressGateway(t),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aclResolve := func(id string) (acl.Authorizer, error) {
				if !tt.defaultDeny {
					// Allow all
					return acl.RootAuthorizer("allow"), nil
				}
				if tt.acl == "" {
					// No token and defaultDeny is denied
					return acl.RootAuthorizer("deny"), nil
				}
				// Ensure the correct token was passed
				require.Equal(t, tt.token, id)
				// Parse the ACL and enforce it
				policy, err := acl.NewPolicyFromSource(tt.acl, acl.SyntaxLegacy, nil, nil)
				require.NoError(t, err)
				return acl.NewPolicyAuthorizerWithDefaults(acl.RootAuthorizer("deny"), []*acl.Policy{policy}, nil)
			}

			scenario := newTestServerDeltaScenario(t, aclResolve, "web-sidecar-proxy", tt.token, 0)
			mgr, errCh, envoy := scenario.mgr, scenario.errCh, scenario.envoy

			sid := structs.NewServiceID("web-sidecar-proxy", nil)
			// Register the proxy to create state needed to Watch() on
			mgr.RegisterProxy(t, sid)

			// Deliver a new snapshot
			snap := tt.cfgSnap
			if snap == nil {
				snap = newTestSnapshot(t, nil, "")
			}
			mgr.DeliverConfig(t, sid, snap)

			// Send initial listener discover, in real life Envoy always sends cluster
			// first but it doesn't really matter and listener has a response that
			// includes the token in the ext rbac filter so lets us test more stuff.
			envoy.SendDeltaReq(t, ListenerType, nil)

			if !tt.wantDenied {
				assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
					TypeUrl: ListenerType,
					Nonce:   hexString(1),
					Resources: makeTestResources(t,
						makeTestListener(t, snap, "tcp:public_listener"),
						makeTestListener(t, snap, "tcp:db"),
						makeTestListener(t, snap, "tcp:geo-cache"),
					),
				})
				// Close the client stream since all is well. We _don't_ do this in the
				// expected error case because we want to verify the error closes the
				// stream from server side.
				envoy.Close()
			}

			select {
			case err := <-errCh:
				if tt.wantDenied {
					require.Error(t, err)
					require.Contains(t, err.Error(), "permission denied")
					mgr.AssertWatchCancelled(t, sid)
				} else {
					require.NoError(t, err)
				}
			case <-time.After(50 * time.Millisecond):
				t.Fatalf("timed out waiting for handler to finish")
			}
		})
	}
}

func TestServer_DeltaAggregatedResources_v3_ACLTokenDeleted_StreamTerminatedDuringDiscoveryRequest(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	aclRules := `service "web" { policy = "write" }`
	token := "service-write-on-web"

	policy, err := acl.NewPolicyFromSource(aclRules, acl.SyntaxLegacy, nil, nil)
	require.NoError(t, err)

	var validToken atomic.Value
	validToken.Store(token)

	aclResolve := func(id string) (acl.Authorizer, error) {
		if token := validToken.Load(); token == nil || id != token.(string) {
			return nil, acl.ErrNotFound
		}

		return acl.NewPolicyAuthorizerWithDefaults(acl.RootAuthorizer("deny"), []*acl.Policy{policy}, nil)
	}
	scenario := newTestServerDeltaScenario(t, aclResolve, "web-sidecar-proxy", token,
		100*time.Millisecond, // Make this short.
	)
	mgr, errCh, envoy := scenario.mgr, scenario.errCh, scenario.envoy

	getError := func() (gotErr error, ok bool) {
		select {
		case err := <-errCh:
			return err, true
		default:
			return nil, false
		}
	}

	sid := structs.NewServiceID("web-sidecar-proxy", nil)
	// Register the proxy to create state needed to Watch() on
	mgr.RegisterProxy(t, sid)

	// Send initial cluster discover (OK)
	envoy.SendDeltaReq(t, ClusterType, nil)
	{
		err, ok := getError()
		require.NoError(t, err)
		require.False(t, ok)
	}

	// Check no response sent yet
	assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)
	{
		err, ok := getError()
		require.NoError(t, err)
		require.False(t, ok)
	}

	// Deliver a new snapshot
	snap := newTestSnapshot(t, nil, "")
	mgr.DeliverConfig(t, sid, snap)

	assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
		TypeUrl: ClusterType,
		Nonce:   hexString(1),
		Resources: makeTestResources(t,
			makeTestCluster(t, snap, "tcp:local_app"),
			makeTestCluster(t, snap, "tcp:db"),
			makeTestCluster(t, snap, "tcp:geo-cache"),
		),
	})

	// It also (in parallel) issues the next cluster request (which acts as an ACK
	// of the version we sent)
	envoy.SendDeltaReq(t, ClusterType, nil)

	// Check no response sent yet
	assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)
	{
		err, ok := getError()
		require.NoError(t, err)
		require.False(t, ok)
	}

	// Now nuke the ACL token while there's no activity.
	validToken.Store("")

	select {
	case err := <-errCh:
		require.Error(t, err)
		gerr, ok := status.FromError(err)
		require.Truef(t, ok, "not a grpc status error: type='%T' value=%v", err, err)
		require.Equal(t, codes.Unauthenticated, gerr.Code())
		require.Equal(t, "unauthenticated: ACL not found", gerr.Message())

		mgr.AssertWatchCancelled(t, sid)
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("timed out waiting for handler to finish")
	}
}

func TestServer_DeltaAggregatedResources_v3_ACLTokenDeleted_StreamTerminatedInBackground(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	aclRules := `service "web" { policy = "write" }`
	token := "service-write-on-web"

	policy, err := acl.NewPolicyFromSource(aclRules, acl.SyntaxLegacy, nil, nil)
	require.NoError(t, err)

	var validToken atomic.Value
	validToken.Store(token)

	aclResolve := func(id string) (acl.Authorizer, error) {
		if token := validToken.Load(); token == nil || id != token.(string) {
			return nil, acl.ErrNotFound
		}

		return acl.NewPolicyAuthorizerWithDefaults(acl.RootAuthorizer("deny"), []*acl.Policy{policy}, nil)
	}
	scenario := newTestServerDeltaScenario(t, aclResolve, "web-sidecar-proxy", token,
		100*time.Millisecond, // Make this short.
	)
	mgr, errCh, envoy := scenario.mgr, scenario.errCh, scenario.envoy

	getError := func() (gotErr error, ok bool) {
		select {
		case err := <-errCh:
			return err, true
		default:
			return nil, false
		}
	}

	sid := structs.NewServiceID("web-sidecar-proxy", nil)
	// Register the proxy to create state needed to Watch() on
	mgr.RegisterProxy(t, sid)

	// Send initial cluster discover (OK)
	envoy.SendDeltaReq(t, ClusterType, nil)
	{
		err, ok := getError()
		require.NoError(t, err)
		require.False(t, ok)
	}

	// Check no response sent yet
	assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)
	{
		err, ok := getError()
		require.NoError(t, err)
		require.False(t, ok)
	}

	// Deliver a new snapshot
	snap := newTestSnapshot(t, nil, "")
	mgr.DeliverConfig(t, sid, snap)

	assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
		TypeUrl: ClusterType,
		Nonce:   hexString(1),
		Resources: makeTestResources(t,
			makeTestCluster(t, snap, "tcp:local_app"),
			makeTestCluster(t, snap, "tcp:db"),
			makeTestCluster(t, snap, "tcp:geo-cache"),
		),
	})

	// It also (in parallel) issues the next cluster request (which acts as an ACK
	// of the version we sent)
	envoy.SendDeltaReq(t, ClusterType, nil)

	// Check no response sent yet
	assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)
	{
		err, ok := getError()
		require.NoError(t, err)
		require.False(t, ok)
	}

	// Now nuke the ACL token while there's no activity.
	validToken.Store("")

	select {
	case err := <-errCh:
		require.Error(t, err)
		gerr, ok := status.FromError(err)
		require.Truef(t, ok, "not a grpc status error: type='%T' value=%v", err, err)
		require.Equal(t, codes.Unauthenticated, gerr.Code())
		require.Equal(t, "unauthenticated: ACL not found", gerr.Message())

		mgr.AssertWatchCancelled(t, sid)
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("timed out waiting for handler to finish")
	}
}

func TestServer_DeltaAggregatedResources_v3_IngressEmptyResponse(t *testing.T) {
	aclResolve := func(id string) (acl.Authorizer, error) {
		// Allow all
		return acl.RootAuthorizer("manage"), nil
	}
	scenario := newTestServerDeltaScenario(t, aclResolve, "ingress-gateway", "", 0)
	mgr, errCh, envoy := scenario.mgr, scenario.errCh, scenario.envoy

	sid := structs.NewServiceID("ingress-gateway", nil)

	// Register the proxy to create state needed to Watch() on
	mgr.RegisterProxy(t, sid)

	// Send initial cluster discover
	envoy.SendDeltaReq(t, ClusterType, nil)

	// Check no response sent yet
	assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

	// Deliver a new snapshot with no services
	snap := proxycfg.TestConfigSnapshotIngressGatewayNoServices(t)
	mgr.DeliverConfig(t, sid, snap)

	// REQ: clusters
	envoy.SendDeltaReq(t, ClusterType, nil)

	// RESP: cluster
	assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
		TypeUrl: ClusterType,
		Nonce:   hexString(1),
	})

	assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

	// ACK: clusters
	envoy.SendDeltaReqACK(t, ClusterType, 1)

	// REQ: listeners
	envoy.SendDeltaReq(t, ListenerType, nil)

	// RESP: listeners
	assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
		TypeUrl: ListenerType,
		Nonce:   hexString(2),
	})

	assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

	envoy.Close()
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("timed out waiting for handler to finish")
	}
}

func assertDeltaChanBlocked(t *testing.T, ch chan *envoy_discovery_v3.DeltaDiscoveryResponse) {
	t.Helper()
	select {
	case r := <-ch:
		t.Fatalf("chan should block but received: %v", r)
	case <-time.After(10 * time.Millisecond):
		return
	}
}

func assertDeltaResponseSent(t *testing.T, ch chan *envoy_discovery_v3.DeltaDiscoveryResponse, want *envoy_discovery_v3.DeltaDiscoveryResponse) {
	t.Helper()
	select {
	case got := <-ch:
		assertDeltaResponse(t, got, want)
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("no response received after 50ms")
	}
}

// assertDeltaResponse is a helper to test a envoy.DeltaDiscoveryResponse matches the
// expected value. We use JSON during comparison here because the responses use protobuf
// Any type which includes binary protobuf encoding.
func assertDeltaResponse(t *testing.T, got, want *envoy_discovery_v3.DeltaDiscoveryResponse) {
	t.Helper()

	gotJSON := protoToSortedJSON(t, got)
	wantJSON := protoToSortedJSON(t, want)
	require.JSONEqf(t, wantJSON, gotJSON, "got:\n%s", gotJSON)
}

func mustMakeVersionMap(t *testing.T, resources ...proto.Message) map[string]string {
	m := make(map[string]string)
	for _, res := range resources {
		name := getResourceName(res)
		m[name] = mustHashResource(t, res)
	}
	return m
}
