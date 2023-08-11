// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/armon/go-metrics"
	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_discovery_v3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/hashicorp/go-hclog"
	goversion "github.com/hashicorp/go-version"
	"github.com/stretchr/testify/require"
	rpcstatus "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/grpc-external/limiter"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/envoyextensions/extensioncommon"
	"github.com/hashicorp/consul/envoyextensions/xdscommon"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/version"
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

	testutil.RunStep(t, "initial setup", func(t *testing.T) {
		snap = newTestSnapshot(t, nil, "",
			func(ns *structs.NodeService) {
				// Add extension for local proxy.
				ns.Proxy.EnvoyExtensions = []structs.EnvoyExtension{
					{
						Name: api.BuiltinLuaExtension,
						Arguments: map[string]interface{}{
							"ProxyType": "connect-proxy",
							"Listener":  "inbound",
							"Script":    "x = 0",
						},
					},
				}
			})

		// Send initial cluster discover. We'll assume we are testing a partial
		// reconnect and include some initial resource versions that will be
		// cleaned up.
		envoy.SendDeltaReq(t, xdscommon.ClusterType, &envoy_discovery_v3.DeltaDiscoveryRequest{
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

	testutil.RunStep(t, "first sync", func(t *testing.T) {
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: xdscommon.ClusterType,
			Nonce:   hexString(1),
			Resources: makeTestResources(t,
				makeTestCluster(t, snap, "tcp:local_app"),
				makeTestCluster(t, snap, "tcp:db"),
				// SAME_AS_INITIAL_VERSION: makeTestCluster(t, snap, "tcp:geo-cache"),
			),
		})

		// Envoy then tries to discover endpoints for those clusters.
		envoy.SendDeltaReq(t, xdscommon.EndpointType, &envoy_discovery_v3.DeltaDiscoveryRequest{
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

		// We should get a response immediately since the config is already present in
		// the server for endpoints. Note that this should not be racy if the server
		// is behaving well since the Cluster send above should be blocked until we
		// deliver a new config version.
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: xdscommon.EndpointType,
			Nonce:   hexString(2),
			Resources: makeTestResources(t,
				makeTestEndpoints(t, snap, "tcp:db"),
				// SAME_AS_INITIAL_VERSION: makeTestEndpoints(t, snap, "tcp:geo-cache"),
				// SAME_AS_INITIAL_VERSION: "fake-endpoints",
			),
		})

		// After receiving the endpoints Envoy sends an ACK for the cluster
		envoy.SendDeltaReqACK(t, xdscommon.ClusterType, 1)

		// We are caught up, so there should be nothing queued to send.
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

		// Envoy now sends listener request
		envoy.SendDeltaReq(t, xdscommon.ListenerType, nil)

		// It also (in parallel) issues the endpoint ACK
		envoy.SendDeltaReqACK(t, xdscommon.EndpointType, 2)

		// And should get a response immediately.
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: xdscommon.ListenerType,
			Nonce:   hexString(3),
			Resources: makeTestResources(t,
				makeTestListener(t, snap, "tcp:public_listener"),
				makeTestListener(t, snap, "tcp:db"),
				makeTestListener(t, snap, "tcp:geo-cache"),
			),
		})

		// We are caught up, so there should be nothing queued to send.
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

		// ACKs the listener
		envoy.SendDeltaReqACK(t, xdscommon.ListenerType, 3)

		// If Envoy re-subscribes to something even if there are no changes we send a
		// fresh copy.
		envoy.SendDeltaReq(t, xdscommon.EndpointType, &envoy_discovery_v3.DeltaDiscoveryRequest{
			ResourceNamesSubscribe: []string{
				"geo-cache.default.dc1.query.11111111-2222-3333-4444-555555555555.consul",
			},
		})

		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: xdscommon.EndpointType,
			Nonce:   hexString(4),
			Resources: makeTestResources(t,
				makeTestEndpoints(t, snap, "tcp:geo-cache"),
			),
		})

		envoy.SendDeltaReqACK(t, xdscommon.EndpointType, 4)

		// We are caught up, so there should be nothing queued to send.
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

		requireExtensionMetrics(t, scenario, api.BuiltinLuaExtension, sid, nil)
	})

	deleteAllButOneEndpoint := func(snap *proxycfg.ConfigSnapshot, uid proxycfg.UpstreamID, targetID string) {
		snap.ConnectProxy.ConfigSnapshotUpstreams.WatchedUpstreamEndpoints[uid][targetID] =
			snap.ConnectProxy.ConfigSnapshotUpstreams.WatchedUpstreamEndpoints[uid][targetID][0:1]
	}

	testutil.RunStep(t, "avoid sending config for unsubscribed resource", func(t *testing.T) {
		envoy.SendDeltaReq(t, xdscommon.EndpointType, &envoy_discovery_v3.DeltaDiscoveryRequest{
			ResourceNamesUnsubscribe: []string{
				"db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul",
			},
		})

		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

		// now reconfigure the snapshot and JUST edit the endpoints to strike one of the two current endpoints for db.
		snap = newTestSnapshot(t, snap, "", nil)
		deleteAllButOneEndpoint(snap, UID("db"), "db.default.default.dc1")
		mgr.DeliverConfig(t, sid, snap)

		// We never send an EDS reply about this change because Envoy is not subscribed to db anymore.
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)
	})

	testutil.RunStep(t, "restore endpoint subscription", func(t *testing.T) {
		// Restore db's deleted endpoints by generating a new snapshot.
		snap = newTestSnapshot(t, snap, "", nil)
		mgr.DeliverConfig(t, sid, snap)

		// We never send an EDS reply about this change because Envoy is still not subscribed to db.
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

		// When Envoy re-subscribes to db we send the endpoints for it.
		envoy.SendDeltaReq(t, xdscommon.EndpointType, &envoy_discovery_v3.DeltaDiscoveryRequest{
			ResourceNamesSubscribe: []string{
				"db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul",
			},
		})
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: xdscommon.EndpointType,
			Nonce:   hexString(5),
			Resources: makeTestResources(t,
				makeTestEndpoints(t, snap, "tcp:db"),
			),
		})

		envoy.SendDeltaReqACK(t, xdscommon.EndpointType, 5)

		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)
	})

	// NOTE: this has to be the last subtest since it kills the stream
	testutil.RunStep(t, "simulate an envoy error sending an update to envoy", func(t *testing.T) {
		// Force sends to fail
		envoy.SetSendErr(errors.New("test error"))

		// Trigger only an EDS update by deleting endpoints again.
		deleteAllButOneEndpoint(snap, UID("db"), "db.default.default.dc1")

		// We never send any replies about this change because we died.
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

func TestServer_DeltaAggregatedResources_v3_NackLoop(t *testing.T) {
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

	testutil.RunStep(t, "initial setup", func(t *testing.T) {
		snap = newTestSnapshot(t, nil, "", nil)

		// Plug in a bad port for the public listener
		snap.Port = 1

		// Send initial cluster discover.
		envoy.SendDeltaReq(t, xdscommon.ClusterType, &envoy_discovery_v3.DeltaDiscoveryRequest{})

		// Check no response sent yet
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

		requireProtocolVersionGauge(t, scenario, "v3", 1)

		// Deliver a new snapshot (tcp with one tcp upstream)
		mgr.DeliverConfig(t, sid, snap)
	})

	testutil.RunStep(t, "simulate Envoy NACKing initial listener", func(t *testing.T) {
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: xdscommon.ClusterType,
			Nonce:   hexString(1),
			Resources: makeTestResources(t,
				makeTestCluster(t, snap, "tcp:local_app"),
				makeTestCluster(t, snap, "tcp:db"),
				makeTestCluster(t, snap, "tcp:geo-cache"),
			),
		})

		// Envoy then tries to discover endpoints for those clusters.
		envoy.SendDeltaReq(t, xdscommon.EndpointType, &envoy_discovery_v3.DeltaDiscoveryRequest{
			ResourceNamesSubscribe: []string{
				"db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul",
				"geo-cache.default.dc1.query.11111111-2222-3333-4444-555555555555.consul",
			},
		})

		// We should get a response immediately since the config is already present in
		// the server for endpoints. Note that this should not be racy if the server
		// is behaving well since the Cluster send above should be blocked until we
		// deliver a new config version.
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: xdscommon.EndpointType,
			Nonce:   hexString(2),
			Resources: makeTestResources(t,
				makeTestEndpoints(t, snap, "tcp:db"),
				makeTestEndpoints(t, snap, "tcp:geo-cache"),
			),
		})

		// After receiving the endpoints Envoy sends an ACK for the clusters
		envoy.SendDeltaReqACK(t, xdscommon.ClusterType, 1)

		// We are caught up, so there should be nothing queued to send.
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

		// Envoy now sends listener request
		envoy.SendDeltaReq(t, xdscommon.ListenerType, nil)

		// It also (in parallel) issues the endpoint ACK
		envoy.SendDeltaReqACK(t, xdscommon.EndpointType, 2)

		// And should get a response immediately.
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: xdscommon.ListenerType,
			Nonce:   hexString(3),
			Resources: makeTestResources(t,
				// Response contains public_listener with port that Envoy can't bind to
				makeTestListener(t, snap, "tcp:bad_public_listener"),
				makeTestListener(t, snap, "tcp:db"),
				makeTestListener(t, snap, "tcp:geo-cache"),
			),
		})

		// We are caught up, so there should be nothing queued to send.
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

		// Envoy NACKs the listener update due to the bad public listener
		envoy.SendDeltaReqNACK(t, xdscommon.ListenerType, 3, &rpcstatus.Status{})

		// Consul should not respond until a new snapshot is delivered
		// because the current snapshot is known to be bad.
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)
	})

	testutil.RunStep(t, "simulate envoy NACKing a listener update", func(t *testing.T) {
		// Correct the port and deliver a new snapshot
		snap.Port = 9999
		mgr.DeliverConfig(t, sid, snap)

		// And should send a response immediately.
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: xdscommon.ListenerType,
			Nonce:   hexString(4),
			Resources: makeTestResources(t,
				// Send a public listener that Envoy will accept
				makeTestListener(t, snap, "tcp:public_listener"),
				makeTestListener(t, snap, "tcp:db"),
				makeTestListener(t, snap, "tcp:geo-cache"),
			),
		})

		// New listener is acked now
		envoy.SendDeltaReqACK(t, xdscommon.EndpointType, 4)

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
	envoy.SendDeltaReq(t, xdscommon.ClusterType, nil)

	// Check no response sent yet
	assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

	// Deliver a new snapshot (tcp with one http upstream)
	snap := newTestSnapshot(t, nil, "http2", nil, &structs.ServiceConfigEntry{
		Kind:     structs.ServiceDefaults,
		Name:     "db",
		Protocol: "http2",
	})
	mgr.DeliverConfig(t, sid, snap)

	testutil.RunStep(t, "no-rds", func(t *testing.T) {
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: xdscommon.ClusterType,
			Nonce:   hexString(1),
			Resources: makeTestResources(t,
				makeTestCluster(t, snap, "tcp:local_app"),
				makeTestCluster(t, snap, "http2:db"),
				makeTestCluster(t, snap, "tcp:geo-cache"),
			),
		})

		// Envoy then tries to discover endpoints for those clusters.
		envoy.SendDeltaReq(t, xdscommon.EndpointType, &envoy_discovery_v3.DeltaDiscoveryRequest{
			ResourceNamesSubscribe: []string{
				"db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul",
				"geo-cache.default.dc1.query.11111111-2222-3333-4444-555555555555.consul",
			},
		})

		// We should get a response immediately since the config is already present in
		// the server for endpoints. Note that this should not be racy if the server
		// is behaving well since the Cluster send above should be blocked until we
		// deliver a new config version.
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: xdscommon.EndpointType,
			Nonce:   hexString(2),
			Resources: makeTestResources(t,
				makeTestEndpoints(t, snap, "http2:db"),
				makeTestEndpoints(t, snap, "tcp:geo-cache"),
			),
		})

		// After receiving the endpoints Envoy sends an ACK for the clusters
		envoy.SendDeltaReqACK(t, xdscommon.ClusterType, 1)

		// We are caught up, so there should be nothing queued to send.
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

		// Envoy now sends listener request
		envoy.SendDeltaReq(t, xdscommon.ListenerType, nil)

		// It also (in parallel) issues the endpoint ACK
		envoy.SendDeltaReqACK(t, xdscommon.EndpointType, 2)

		// And should get a response immediately.
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: xdscommon.ListenerType,
			Nonce:   hexString(3),
			Resources: makeTestResources(t,
				makeTestListener(t, snap, "tcp:public_listener"),
				makeTestListener(t, snap, "http2:db"),
				makeTestListener(t, snap, "tcp:geo-cache"),
			),
		})

		// We are caught up, so there should be nothing queued to send.
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

		// ACKs the listener
		envoy.SendDeltaReqACK(t, xdscommon.ListenerType, 3)

		// We are caught up, so there should be nothing queued to send.
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)
	})

	// -- reconfigure with a no-op discovery chain

	snap = newTestSnapshot(t, snap, "http2", nil, &structs.ServiceConfigEntry{
		Kind:     structs.ServiceDefaults,
		Name:     "db",
		Protocol: "http2",
	}, &structs.ServiceRouterConfigEntry{
		Kind:   structs.ServiceRouter,
		Name:   "db",
		Routes: nil,
	})
	mgr.DeliverConfig(t, sid, snap)

	testutil.RunStep(t, "with-rds", func(t *testing.T) {
		// Just the "db" listener sees a change
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: xdscommon.ListenerType,
			Nonce:   hexString(4),
			Resources: makeTestResources(t,
				makeTestListener(t, snap, "http2:db:rds"),
			),
		})

		// We are caught up, so there should be nothing queued to send.
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

		// Envoy now sends routes request
		envoy.SendDeltaReq(t, xdscommon.RouteType, &envoy_discovery_v3.DeltaDiscoveryRequest{
			ResourceNamesSubscribe: []string{
				"db",
			},
		})

		// And should get a response immediately.
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: xdscommon.RouteType,
			Nonce:   hexString(5),
			Resources: makeTestResources(t,
				makeTestRoute(t, "http2:db"),
			),
		})

		// After receiving the routes, Envoy sends acks back for the listener and routes.
		envoy.SendDeltaReqACK(t, xdscommon.ListenerType, 4)
		envoy.SendDeltaReqACK(t, xdscommon.RouteType, 5)

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
	server.ResourceMapMutateFn = func(resourceMap *xdscommon.IndexedResources) {
		if atomic.LoadUint32(&slowHackDisabled) == 1 {
			return
		}
		if em, ok := resourceMap.Index[xdscommon.EndpointType]; ok {
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
	testutil.RunStep(t, "get into initial state", func(t *testing.T) {
		snap = newTestSnapshot(t, nil, "", nil)

		// Send initial cluster discover.
		envoy.SendDeltaReq(t, xdscommon.ClusterType, &envoy_discovery_v3.DeltaDiscoveryRequest{})

		// Check no response sent yet
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

		requireProtocolVersionGauge(t, scenario, "v3", 1)

		// Deliver a new snapshot (tcp with one tcp upstream)
		mgr.DeliverConfig(t, sid, snap)

		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: xdscommon.ClusterType,
			Nonce:   hexString(1),
			Resources: makeTestResources(t,
				makeTestCluster(t, snap, "tcp:local_app"),
				makeTestCluster(t, snap, "tcp:db"),
				makeTestCluster(t, snap, "tcp:geo-cache"),
			),
		})

		// Envoy then tries to discover endpoints for those clusters.
		envoy.SendDeltaReq(t, xdscommon.EndpointType, &envoy_discovery_v3.DeltaDiscoveryRequest{
			ResourceNamesSubscribe: []string{
				"db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul",
				"geo-cache.default.dc1.query.11111111-2222-3333-4444-555555555555.consul",
			},
		})

		// We should get a response immediately since the config is already present in
		// the server for endpoints. Note that this should not be racy if the server
		// is behaving well since the Cluster send above should be blocked until we
		// deliver a new config version.
		//
		// NOTE: we do NOT return back geo-cache yet
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: xdscommon.EndpointType,
			Nonce:   hexString(2),
			Resources: makeTestResources(t,
				makeTestEndpoints(t, snap, "tcp:db"),
				// makeTestEndpoints(t, snap, "tcp:geo-cache"),
			),
		})

		// After receiving the endpoints Envoy sends an ACK for the clusters.
		// Envoy aims to wait to receive endpoints before ACKing clusters,
		// but because it received an update for at least one of the clusters it cares about
		// then it will ACK despite not having received an update for all clusters.
		// This behavior was observed against Envoy v1.21 and v1.23.
		envoy.SendDeltaReqACK(t, xdscommon.ClusterType, 1)

		// We are caught up, so there should be nothing queued to send.
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

		// Envoy now sends listener request
		envoy.SendDeltaReq(t, xdscommon.ListenerType, nil)

		// It also (in parallel) issues the endpoint ACK
		envoy.SendDeltaReqACK(t, xdscommon.EndpointType, 2)

		// And should get a response immediately.
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: xdscommon.ListenerType,
			Nonce:   hexString(3),
			Resources: makeTestResources(t,
				makeTestListener(t, snap, "tcp:public_listener"),
				makeTestListener(t, snap, "tcp:db"),
				makeTestListener(t, snap, "tcp:geo-cache"),
			),
		})

		// We are caught up, so there should be nothing queued to send.
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

		// ACKs the listener
		envoy.SendDeltaReqACK(t, xdscommon.ListenerType, 3)
	})

	// Disable hack. Need to wait for one more event to wake up the loop.
	atomic.StoreUint32(&slowHackDisabled, 1)

	testutil.RunStep(t, "delayed endpoint update finally comes in", func(t *testing.T) {
		// Trigger the xds.Server select{} to wake up and notice our hack is disabled.
		// The actual contents of this change are irrelevant.
		snap = newTestSnapshot(t, snap, "", nil)
		mgr.DeliverConfig(t, sid, snap)

		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: xdscommon.EndpointType,
			Nonce:   hexString(4),
			Resources: makeTestResources(t,
				makeTestEndpoints(t, snap, "tcp:geo-cache"),
			),
		})

		// We are caught up, so there should be nothing queued to send.
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

		// It also (in parallel) issues the endpoint ACK
		envoy.SendDeltaReqACK(t, xdscommon.EndpointType, 4)

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
	testutil.RunStep(t, "get into initial state", func(t *testing.T) {
		snap = newTestSnapshot(t, nil, "", nil)

		// Send initial cluster discover.
		envoy.SendDeltaReq(t, xdscommon.ClusterType, &envoy_discovery_v3.DeltaDiscoveryRequest{})

		// Check no response sent yet
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

		requireProtocolVersionGauge(t, scenario, "v3", 1)

		// Deliver a new snapshot (tcp with one tcp upstream)
		mgr.DeliverConfig(t, sid, snap)

		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: xdscommon.ClusterType,
			Nonce:   hexString(1),
			Resources: makeTestResources(t,
				makeTestCluster(t, snap, "tcp:local_app"),
				makeTestCluster(t, snap, "tcp:db"),
				makeTestCluster(t, snap, "tcp:geo-cache"),
			),
		})

		// Envoy then tries to discover endpoints for those clusters.
		envoy.SendDeltaReq(t, xdscommon.EndpointType, &envoy_discovery_v3.DeltaDiscoveryRequest{
			ResourceNamesSubscribe: []string{
				"db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul",
				"geo-cache.default.dc1.query.11111111-2222-3333-4444-555555555555.consul",
			},
		})

		// We should get a response immediately since the config is already present in
		// the server for endpoints. Note that this should not be racy if the server
		// is behaving well since the Cluster send above should be blocked until we
		// deliver a new config version.
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: xdscommon.EndpointType,
			Nonce:   hexString(2),
			Resources: makeTestResources(t,
				makeTestEndpoints(t, snap, "tcp:db"),
				makeTestEndpoints(t, snap, "tcp:geo-cache"),
			),
		})

		// After receiving the endpoints Envoy sends an ACK for the clusters
		envoy.SendDeltaReqACK(t, xdscommon.ClusterType, 1)

		// We are caught up, so there should be nothing queued to send.
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

		// Envoy now sends listener request
		envoy.SendDeltaReq(t, xdscommon.ListenerType, nil)

		// It also (in parallel) issues the endpoint ACK
		envoy.SendDeltaReqACK(t, xdscommon.EndpointType, 2)

		// And should get a response immediately.
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: xdscommon.ListenerType,
			Nonce:   hexString(3),
			Resources: makeTestResources(t,
				makeTestListener(t, snap, "tcp:public_listener"),
				makeTestListener(t, snap, "tcp:db"),
				makeTestListener(t, snap, "tcp:geo-cache"),
			),
		})

		// We are caught up, so there should be nothing queued to send.
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

		// ACKs the listener
		envoy.SendDeltaReqACK(t, xdscommon.ListenerType, 3)
	})

	testutil.RunStep(t, "trigger cluster update needing implicit endpoint replacements", func(t *testing.T) {
		// Update the snapshot in a way that causes a single cluster update.
		snap = newTestSnapshot(t, snap, "", nil, &structs.ServiceResolverConfigEntry{
			Kind:           structs.ServiceResolver,
			Name:           "db",
			ConnectTimeout: 1337 * time.Second,
		})
		mgr.DeliverConfig(t, sid, snap)

		// The cluster is updated
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: xdscommon.ClusterType,
			Nonce:   hexString(4),
			Resources: makeTestResources(t,
				// SAME makeTestCluster(t, snap, "tcp:local_app"),
				makeTestCluster(t, snap, "tcp:db:timeout"),
				// SAME makeTestCluster(t, snap, "tcp:geo-cache"),
			),
		})

		// And we re-send the endpoints for the updated cluster after getting the
		// ACK for the cluster.
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: xdscommon.EndpointType,
			Nonce:   hexString(5),
			Resources: makeTestResources(t,
				makeTestEndpoints(t, snap, "tcp:db"),
				// SAME makeTestEndpoints(t, snap, "tcp:geo-cache"),
			),
		})

		// Envoy then ACK's the clusters and the endpoints.
		envoy.SendDeltaReqACK(t, xdscommon.ClusterType, 4)
		envoy.SendDeltaReqACK(t, xdscommon.EndpointType, 5)

		// We are caught up, so there should be nothing queued to send.
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

	testutil.RunStep(t, "get into initial state", func(t *testing.T) {
		// Send initial cluster discover (empty payload)
		envoy.SendDeltaReq(t, xdscommon.ClusterType, nil)

		// Check no response sent yet
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

		// Deliver a new snapshot (tcp with one http upstream with no-op disco chain)
		snap = newTestSnapshot(t, nil, "http2", nil, &structs.ServiceConfigEntry{
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
			TypeUrl: xdscommon.ClusterType,
			Nonce:   hexString(1),
			Resources: makeTestResources(t,
				makeTestCluster(t, snap, "tcp:local_app"),
				makeTestCluster(t, snap, "http2:db"),
				makeTestCluster(t, snap, "tcp:geo-cache"),
			),
		})

		// Envoy then tries to discover endpoints for those clusters.
		envoy.SendDeltaReq(t, xdscommon.EndpointType, &envoy_discovery_v3.DeltaDiscoveryRequest{
			ResourceNamesSubscribe: []string{
				"db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul",
				"geo-cache.default.dc1.query.11111111-2222-3333-4444-555555555555.consul",
			},
		})

		// We should get a response immediately since the config is already present in
		// the server for endpoints. Note that this should not be racy if the server
		// is behaving well since the Cluster send above should be blocked until we
		// deliver a new config version.
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: xdscommon.EndpointType,
			Nonce:   hexString(2),
			Resources: makeTestResources(t,
				makeTestEndpoints(t, snap, "http2:db"),
				makeTestEndpoints(t, snap, "tcp:geo-cache"),
			),
		})

		// After receiving the endpoints Envoy sends an ACK for the clusters
		envoy.SendDeltaReqACK(t, xdscommon.ClusterType, 1)

		// We are caught up, so there should be nothing queued to send.
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

		// Envoy now sends listener request
		envoy.SendDeltaReq(t, xdscommon.ListenerType, nil)

		// It also (in parallel) issues the endpoint ACK
		envoy.SendDeltaReqACK(t, xdscommon.EndpointType, 2)

		// And should get a response immediately.
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: xdscommon.ListenerType,
			Nonce:   hexString(3),
			Resources: makeTestResources(t,
				makeTestListener(t, snap, "tcp:public_listener"),
				makeTestListener(t, snap, "http2:db:rds"),
				makeTestListener(t, snap, "tcp:geo-cache"),
			),
		})

		// We are caught up, so there should be nothing queued to send.
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

		// Envoy now sends routes request
		envoy.SendDeltaReq(t, xdscommon.RouteType, &envoy_discovery_v3.DeltaDiscoveryRequest{
			ResourceNamesSubscribe: []string{
				"db",
			},
		})

		// And should get a response immediately.
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: xdscommon.RouteType,
			Nonce:   hexString(4),
			Resources: makeTestResources(t,
				makeTestRoute(t, "http2:db"),
			),
		})

		// After receiving the routes, Envoy sends acks back for the listener and routes.
		envoy.SendDeltaReqACK(t, xdscommon.ListenerType, 3)
		envoy.SendDeltaReqACK(t, xdscommon.RouteType, 4)

		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)
	})

	testutil.RunStep(t, "trigger listener update needing implicit route replacements", func(t *testing.T) {
		// Update the snapshot in a way that causes a single listener update.
		//
		// Downgrade from http2 to http
		snap = newTestSnapshot(t, snap, "http", nil, &structs.ServiceConfigEntry{
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
			TypeUrl: xdscommon.ClusterType,
			Nonce:   hexString(5),
			Resources: makeTestResources(t,
				makeTestCluster(t, snap, "http:db"),
			),
		})

		envoy.SendDeltaReqACK(t, xdscommon.ClusterType, 5)

		// The behaviors of Cluster updates triggering re-sends of Endpoint updates
		// tested in TestServer_DeltaAggregatedResources_v3_BasicProtocol_TCP_clusterChangesImpactEndpoints
		// triggers here. It is not explicitly under test, but we have to get past
		// this exchange to get to the part we care about.

		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: xdscommon.EndpointType,
			Nonce:   hexString(6),
			Resources: makeTestResources(t,
				makeTestEndpoints(t, snap, "http:db"),
			),
		})

		envoy.SendDeltaReqACK(t, xdscommon.EndpointType, 6)

		// the listener is updated
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: xdscommon.ListenerType,
			Nonce:   hexString(7),
			Resources: makeTestResources(t,
				makeTestListener(t, snap, "http:db:rds"),
			),
		})

		// THE ACTUAL THING WE CARE ABOUT: replaced route config
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: xdscommon.RouteType,
			Nonce:   hexString(8),
			Resources: makeTestResources(t,
				makeTestRoute(t, "http2:db"),
			),
		})

		// After receiving the routes, Envoy sends acks back for the listener and routes.
		envoy.SendDeltaReqACK(t, xdscommon.ListenerType, 7)
		envoy.SendDeltaReqACK(t, xdscommon.RouteType, 8)

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
			cfgSnap:     proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp", "default", nil, nil, nil),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stopped bool
			lock := &sync.RWMutex{}

			defer func() {
				lock.Lock()
				stopped = true
				lock.Unlock()
			}()

			// aclResolve may be called in a goroutine even after a
			// testcase tt returns. Capture the variable as tc so the
			// values don't swap in the next iteration.
			tc := tt
			aclResolve := func(id string) (acl.Authorizer, error) {
				if !tc.defaultDeny {
					// Allow all
					return acl.RootAuthorizer("allow"), nil
				}
				if tc.acl == "" {
					// No token and defaultDeny is denied
					return acl.RootAuthorizer("deny"), nil
				}

				lock.RLock()
				defer lock.RUnlock()

				if stopped {
					return acl.DenyAll().ToAllowAuthorizer(), nil
				}

				// Ensure the correct token was passed
				require.Equal(t, tc.token, id)
				// Parse the ACL and enforce it
				policy, err := acl.NewPolicyFromSource(tc.acl, nil, nil)
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
				snap = newTestSnapshot(t, nil, "", nil)
			}
			mgr.DeliverConfig(t, sid, snap)

			// Send initial listener discover, in real life Envoy always sends cluster
			// first but it doesn't really matter and listener has a response that
			// includes the token in the ext rbac filter so lets us test more stuff.
			envoy.SendDeltaReq(t, xdscommon.ListenerType, nil)

			// If there is no token, check that we increment the gauge
			if tt.token == "" {
				retry.Run(t, func(r *retry.R) {
					data := scenario.sink.Data()
					require.Len(r, data, 1)

					item := data[0]
					val, ok := item.Gauges["consul.xds.test.xds.server.streamsUnauthenticated"]
					require.True(r, ok)
					require.Equal(r, float32(1), val.Value)
				})
			}

			if !tt.wantDenied {
				assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
					TypeUrl: xdscommon.ListenerType,
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
					status, ok := status.FromError(err)
					require.True(t, ok)
					require.Equal(t, codes.PermissionDenied, status.Code())
					require.Contains(t, err.Error(), "Permission denied")
					mgr.AssertWatchCancelled(t, sid)
				} else {
					require.NoError(t, err)
				}
			case <-time.After(50 * time.Millisecond):
				t.Fatalf("timed out waiting for handler to finish")
			}

			// If there is no token, check that we decrement the gauge
			if tt.token == "" {
				retry.Run(t, func(r *retry.R) {
					data := scenario.sink.Data()
					require.Len(r, data, 1)

					item := data[0]
					val, ok := item.Gauges["consul.xds.test.xds.server.streamsUnauthenticated"]
					require.True(r, ok)
					require.Equal(r, float32(0), val.Value)
				})
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

	policy, err := acl.NewPolicyFromSource(aclRules, nil, nil)
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
	envoy.SendDeltaReq(t, xdscommon.ClusterType, nil)
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
	snap := newTestSnapshot(t, nil, "", nil)
	mgr.DeliverConfig(t, sid, snap)

	assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
		TypeUrl: xdscommon.ClusterType,
		Nonce:   hexString(1),
		Resources: makeTestResources(t,
			makeTestCluster(t, snap, "tcp:local_app"),
			makeTestCluster(t, snap, "tcp:db"),
			makeTestCluster(t, snap, "tcp:geo-cache"),
		),
	})

	// It also (in parallel) issues the next cluster request (which acts as an ACK
	// of the version we sent)
	envoy.SendDeltaReq(t, xdscommon.ClusterType, nil)

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

	policy, err := acl.NewPolicyFromSource(aclRules, nil, nil)
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
	envoy.SendDeltaReq(t, xdscommon.ClusterType, nil)
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
	snap := newTestSnapshot(t, nil, "", nil)
	mgr.DeliverConfig(t, sid, snap)

	assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
		TypeUrl: xdscommon.ClusterType,
		Nonce:   hexString(1),
		Resources: makeTestResources(t,
			makeTestCluster(t, snap, "tcp:local_app"),
			makeTestCluster(t, snap, "tcp:db"),
			makeTestCluster(t, snap, "tcp:geo-cache"),
		),
	})

	// It also (in parallel) issues the next cluster request (which acts as an ACK
	// of the version we sent)
	envoy.SendDeltaReq(t, xdscommon.ClusterType, nil)

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
	envoy.SendDeltaReq(t, xdscommon.ClusterType, nil)

	// Check no response sent yet
	assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

	// Deliver a new snapshot with no services
	snap := proxycfg.TestConfigSnapshotIngressGateway(t, false, "tcp", "default", nil, nil, nil)
	mgr.DeliverConfig(t, sid, snap)

	// REQ: clusters
	envoy.SendDeltaReq(t, xdscommon.ClusterType, nil)

	// RESP: cluster
	assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
		TypeUrl: xdscommon.ClusterType,
		Nonce:   hexString(1),
	})

	assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

	// ACK: clusters
	envoy.SendDeltaReqACK(t, xdscommon.ClusterType, 1)

	// REQ: listeners
	envoy.SendDeltaReq(t, xdscommon.ListenerType, nil)

	// RESP: listeners
	assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
		TypeUrl: xdscommon.ListenerType,
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

func TestServer_DeltaAggregatedResources_v3_CapacityReached(t *testing.T) {
	aclResolve := func(id string) (acl.Authorizer, error) { return acl.ManageAll(), nil }

	scenario := newTestServerDeltaScenario(t, aclResolve, "web-sidecar-proxy", "", 0)
	mgr, errCh, envoy := scenario.mgr, scenario.errCh, scenario.envoy

	sid := structs.NewServiceID("web-sidecar-proxy", nil)

	mgr.RegisterProxy(t, sid)
	mgr.DrainStreams(sid)

	snap := newTestSnapshot(t, nil, "", nil)

	envoy.SendDeltaReq(t, xdscommon.ClusterType, &envoy_discovery_v3.DeltaDiscoveryRequest{
		InitialResourceVersions: mustMakeVersionMap(t,
			makeTestCluster(t, snap, "tcp:geo-cache"),
		),
	})

	select {
	case err := <-errCh:
		require.Error(t, err)
		require.Equal(t, codes.ResourceExhausted.String(), status.Code(err).String())
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("timed out waiting for handler to finish")
	}
}

type capacityReachedLimiter struct{}

func (capacityReachedLimiter) BeginSession() (limiter.Session, error) {
	return nil, limiter.ErrCapacityReached
}

func TestServer_DeltaAggregatedResources_v3_StreamDrained(t *testing.T) {
	aclResolve := func(id string) (acl.Authorizer, error) { return acl.ManageAll(), nil }
	scenario := newTestServerDeltaScenario(t, aclResolve, "web-sidecar-proxy", "", 0)
	mgr, errCh, envoy := scenario.mgr, scenario.errCh, scenario.envoy

	sid := structs.NewServiceID("web-sidecar-proxy", nil)

	mgr.RegisterProxy(t, sid)

	testutil.RunStep(t, "successful request/response", func(t *testing.T) {
		snap := newTestSnapshot(t, nil, "", nil)

		envoy.SendDeltaReq(t, xdscommon.ClusterType, &envoy_discovery_v3.DeltaDiscoveryRequest{
			InitialResourceVersions: mustMakeVersionMap(t,
				makeTestCluster(t, snap, "tcp:geo-cache"),
			),
		})

		mgr.DeliverConfig(t, sid, snap)

		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: xdscommon.ClusterType,
			Nonce:   hexString(1),
			Resources: makeTestResources(t,
				makeTestCluster(t, snap, "tcp:local_app"),
				makeTestCluster(t, snap, "tcp:db"),
			),
		})
	})

	testutil.RunStep(t, "terminate limiter session", func(t *testing.T) {
		mgr.DrainStreams(sid)

		select {
		case err := <-errCh:
			require.Error(t, err)
			require.Equal(t, codes.ResourceExhausted.String(), status.Code(err).String())
		case <-time.After(50 * time.Millisecond):
			t.Fatalf("timed out waiting for handler to finish")
		}
	})

	testutil.RunStep(t, "check drain counter incremented", func(t *testing.T) {
		data := scenario.sink.Data()
		require.Len(t, data, 1)

		item := data[0]
		require.Len(t, item.Counters, 1)

		val, ok := item.Counters["consul.xds.test.xds.server.streamDrained"]
		require.True(t, ok)
		require.Equal(t, 1, val.Count)
	})

	testutil.RunStep(t, "check streamStart metric recorded", func(t *testing.T) {
		data := scenario.sink.Data()
		require.Len(t, data, 1)

		item := data[0]
		require.Len(t, item.Samples, 1)

		val, ok := item.Samples["consul.xds.test.xds.server.streamStart"]
		require.True(t, ok)
		require.Equal(t, 1, val.Count)
	})
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
		name := xdscommon.GetResourceName(res)
		m[name] = mustHashResource(t, res)
	}
	return m
}

func requireExtensionMetrics(
	t *testing.T,
	scenario *testServerScenario,
	extName string,
	sid structs.ServiceID,
	err error,
) {
	data := scenario.sink.Data()
	require.Len(t, data, 1)
	item := data[0]

	expectLabels := []metrics.Label{
		{Name: "extension", Value: extName},
		{Name: "version", Value: "builtin/" + version.Version},
		{Name: "service", Value: sid.ID},
		{Name: "partition", Value: sid.PartitionOrDefault()},
		{Name: "namespace", Value: sid.NamespaceOrDefault()},
		{Name: "error", Value: strconv.FormatBool(err != nil)},
	}

	for _, s := range []string{
		"consul.xds.test.envoy_extension.validate_arguments;",
		"consul.xds.test.envoy_extension.validate;",
		"consul.xds.test.envoy_extension.extend;",
	} {
		foundLabel := false
		for k, v := range item.Samples {
			if strings.HasPrefix(k, s) {
				foundLabel = true
				require.ElementsMatch(t, expectLabels, v.Labels)
			}
		}
		require.True(t, foundLabel)
	}
}

func Test_validateAndApplyEnvoyExtension_Validations(t *testing.T) {
	type testCase struct {
		name          string
		runtimeConfig extensioncommon.RuntimeConfig
		err           bool
		errString     string
	}

	envoyVersion, _ := goversion.NewVersion("1.25.0")
	consulVersion, _ := goversion.NewVersion("1.16.0")

	svc := api.CompoundServiceName{
		Name:      "s1",
		Partition: "ap1",
		Namespace: "ns1",
	}

	makeRuntimeConfig := func(required bool, consulVersion string, envoyVersion string, args map[string]interface{}) extensioncommon.RuntimeConfig {
		if args == nil {
			args = map[string]interface{}{
				"ARN": "arn:aws:lambda:us-east-1:111111111111:function:lambda-1234",
			}

		}
		return extensioncommon.RuntimeConfig{
			EnvoyExtension: api.EnvoyExtension{
				Name:          api.BuiltinAWSLambdaExtension,
				Required:      required,
				ConsulVersion: consulVersion,
				EnvoyVersion:  envoyVersion,
				Arguments:     args,
			},
			ServiceName: svc,
		}
	}

	cases := []testCase{
		{
			name:          "invalid consul version constraint - required",
			runtimeConfig: makeRuntimeConfig(true, "bad", ">= 1.0", nil),
			err:           true,
			errString:     "failed to parse Consul version constraint for extension",
		},
		{
			name:          "invalid consul version constraint - not required",
			runtimeConfig: makeRuntimeConfig(false, "bad", ">= 1.0", nil),
			err:           false,
		},
		{
			name:          "invalid envoy version constraint - required",
			runtimeConfig: makeRuntimeConfig(true, ">= 1.0", "bad", nil),
			err:           true,
			errString:     "failed to parse Envoy version constraint for extension",
		},
		{
			name:          "invalid envoy version constraint - not required",
			runtimeConfig: makeRuntimeConfig(false, ">= 1.0", "bad", nil),
			err:           false,
		},
		{
			name:          "no envoy version constraint match",
			runtimeConfig: makeRuntimeConfig(false, "", ">= 2.0.0", nil),
			err:           false,
		},
		{
			name:          "no consul version constraint match",
			runtimeConfig: makeRuntimeConfig(false, ">= 2.0.0", "", nil),
			err:           false,
		},
		{
			name:          "invalid extension arguments - required",
			runtimeConfig: makeRuntimeConfig(true, ">= 1.15.0", ">= 1.25.0", map[string]interface{}{"bad": "args"}),
			err:           true,
			errString:     "failed to construct extension",
		},
		{
			name:          "invalid extension arguments - not required",
			runtimeConfig: makeRuntimeConfig(false, ">= 1.15.0", ">= 1.25.0", map[string]interface{}{"bad": "args"}),
			err:           false,
		},
		{
			name:          "valid everything - no resources and required",
			runtimeConfig: makeRuntimeConfig(true, ">= 1.15.0", ">= 1.25.0", nil),
			err:           true,
			errString:     "failed to patch xDS resources in",
		},
		{
			name:          "valid everything",
			runtimeConfig: makeRuntimeConfig(false, ">= 1.15.0", ">= 1.25.0", nil),
			err:           false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snap := proxycfg.ConfigSnapshot{
				ProxyID: proxycfg.ProxyID{
					ServiceID: structs.NewServiceID("s1", nil),
				},
			}
			resources, err := validateAndApplyEnvoyExtension(hclog.NewNullLogger(), &snap, nil, tc.runtimeConfig, envoyVersion, consulVersion)
			if tc.err {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errString)
			} else {
				require.NoError(t, err)
				require.Nil(t, resources)
			}
		})
	}
}

func Test_applyEnvoyExtension_CanApply(t *testing.T) {
	type testCase struct {
		name     string
		canApply bool
	}

	cases := []testCase{
		{
			name:     "cannot apply: is not applied",
			canApply: false,
		},
		{
			name:     "can apply: is applied",
			canApply: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			extender := extensioncommon.BasicEnvoyExtender{
				Extension: &maybeCanApplyExtension{
					canApply: tc.canApply,
				},
			}
			config := &extensioncommon.RuntimeConfig{
				Kind:                  api.ServiceKindConnectProxy,
				ServiceName:           api.CompoundServiceName{Name: "api"},
				Upstreams:             map[api.CompoundServiceName]*extensioncommon.UpstreamData{},
				IsSourcedFromUpstream: false,
				EnvoyExtension: api.EnvoyExtension{
					Name:     "maybeCanApplyExtension",
					Required: false,
				},
			}
			listener := &envoy_listener_v3.Listener{
				Name:                  xdscommon.OutboundListenerName,
				IgnoreGlobalConnLimit: false,
			}
			indexedResources := xdscommon.IndexResources(testutil.Logger(t), map[string][]proto.Message{
				xdscommon.ListenerType: {
					listener,
				},
			})

			result, err := applyEnvoyExtension(&extender, indexedResources, config)
			require.NoError(t, err)
			resultListener := result.Index[xdscommon.ListenerType][xdscommon.OutboundListenerName].(*envoy_listener_v3.Listener)
			require.Equal(t, tc.canApply, resultListener.IgnoreGlobalConnLimit)
		})
	}
}

func Test_applyEnvoyExtension_PartialApplicationDisallowed(t *testing.T) {
	type testCase struct {
		name            string
		fail            bool
		returnOnFailure bool
	}

	cases := []testCase{
		{
			name:            "failure: returns nothing",
			fail:            true,
			returnOnFailure: false,
		},
		// Not expected, but cover to be sure.
		{
			name:            "failure: returns values",
			fail:            true,
			returnOnFailure: true,
		},
		// Ensure that under normal circumstances, the extension would succeed in
		// modifying resources.
		{
			name: "success: resources modified",
			fail: false,
		},
	}

	for _, tc := range cases {
		for _, indexType := range []string{
			xdscommon.ListenerType,
			xdscommon.ClusterType,
		} {
			typeShortName := indexType[strings.LastIndex(indexType, ".")+1:]
			t.Run(fmt.Sprintf("%s: %s", tc.name, typeShortName), func(t *testing.T) {
				extender := extensioncommon.BasicEnvoyExtender{
					Extension: &partialFailureExtension{
						returnOnFailure: tc.returnOnFailure,
						// Alternate which resource fails so that we can test for
						// partial modification independent of patch order.
						failListener: tc.fail && indexType == xdscommon.ListenerType,
						failCluster:  tc.fail && indexType == xdscommon.ClusterType,
					},
				}
				config := &extensioncommon.RuntimeConfig{
					Kind:                  api.ServiceKindConnectProxy,
					ServiceName:           api.CompoundServiceName{Name: "api"},
					Upstreams:             map[api.CompoundServiceName]*extensioncommon.UpstreamData{},
					IsSourcedFromUpstream: false,
					EnvoyExtension: api.EnvoyExtension{
						Name:     "partialFailureExtension",
						Required: false,
					},
				}
				cluster := &envoy_cluster_v3.Cluster{
					Name:          xdscommon.LocalAppClusterName,
					RespectDnsTtl: false,
				}
				listener := &envoy_listener_v3.Listener{
					Name:                  xdscommon.OutboundListenerName,
					IgnoreGlobalConnLimit: false,
				}
				indexedResources := xdscommon.IndexResources(testutil.Logger(t), map[string][]proto.Message{
					xdscommon.ClusterType: {
						cluster,
					},
					xdscommon.ListenerType: {
						listener,
					},
				})

				result, err := applyEnvoyExtension(&extender, indexedResources, config)
				if tc.fail {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}

				resultListener := result.Index[xdscommon.ListenerType][xdscommon.OutboundListenerName].(*envoy_listener_v3.Listener)
				resultCluster := result.Index[xdscommon.ClusterType][xdscommon.LocalAppClusterName].(*envoy_cluster_v3.Cluster)
				require.Equal(t, !tc.fail, resultListener.IgnoreGlobalConnLimit)
				require.Equal(t, !tc.fail, resultCluster.RespectDnsTtl)

				// Regardless of success, original values should not be modified.
				originalListener := indexedResources.Index[xdscommon.ListenerType][xdscommon.OutboundListenerName].(*envoy_listener_v3.Listener)
				originalCluster := indexedResources.Index[xdscommon.ClusterType][xdscommon.LocalAppClusterName].(*envoy_cluster_v3.Cluster)
				require.False(t, originalListener.IgnoreGlobalConnLimit)
				require.False(t, originalCluster.RespectDnsTtl)
			})
		}
	}
}

func Test_applyEnvoyExtension_HandlesPanics(t *testing.T) {
	type testCase struct {
		name            string
		panicOnCanApply bool
		panicOnPatch    bool
	}

	cases := []testCase{
		{
			name:            "panic: CanApply",
			panicOnCanApply: true,
		},
		{
			name:         "panic: Extend",
			panicOnPatch: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			extension := &maybePanicExtension{
				panicOnCanApply: tc.panicOnCanApply,
				panicOnPatch:    tc.panicOnPatch,
			}
			extender := extensioncommon.BasicEnvoyExtender{
				Extension: extension,
			}
			config := &extensioncommon.RuntimeConfig{
				Kind:                  api.ServiceKindConnectProxy,
				ServiceName:           api.CompoundServiceName{Name: "api"},
				Upstreams:             map[api.CompoundServiceName]*extensioncommon.UpstreamData{},
				IsSourcedFromUpstream: false,
				EnvoyExtension: api.EnvoyExtension{
					Name:     "maybePanicExtension",
					Required: false,
				},
			}
			listener := &envoy_listener_v3.Listener{
				Name:                  xdscommon.OutboundListenerName,
				IgnoreGlobalConnLimit: false,
			}
			indexedResources := xdscommon.IndexResources(testutil.Logger(t), map[string][]proto.Message{
				xdscommon.ListenerType: {
					listener,
				},
			})

			_, err := applyEnvoyExtension(&extender, indexedResources, config)

			// We did not panic, good.
			// First assert our test is valid by forcing a panic, then check the error message that was returned.
			if tc.panicOnCanApply {
				require.PanicsWithError(t, "this is an expected failure in CanApply", func() {
					extension.CanApply(config)
				})
				require.ErrorContains(t, err, "attempt to apply Envoy extension \"maybePanicExtension\" caused an unexpected panic: this is an expected failure in CanApply")
			}
			if tc.panicOnPatch {
				require.PanicsWithError(t, "this is an expected failure in PatchListener", func() {
					_, _, _ = extension.PatchListener(config.GetListenerPayload(listener))
				})
				require.ErrorContains(t, err, "attempt to apply Envoy extension \"maybePanicExtension\" caused an unexpected panic: this is an expected failure in PatchListener")
			}
		})
	}
}

type maybeCanApplyExtension struct {
	extensioncommon.BasicExtensionAdapter
	canApply bool
}

var _ extensioncommon.BasicExtension = (*maybeCanApplyExtension)(nil)

func (m *maybeCanApplyExtension) CanApply(_ *extensioncommon.RuntimeConfig) bool {
	return m.canApply
}

func (m *maybeCanApplyExtension) PatchListener(payload extensioncommon.ListenerPayload) (*envoy_listener_v3.Listener, bool, error) {
	payload.Message.IgnoreGlobalConnLimit = true
	return payload.Message, true, nil
}

type partialFailureExtension struct {
	extensioncommon.BasicExtensionAdapter
	returnOnFailure bool
	failCluster     bool
	failListener    bool
}

var _ extensioncommon.BasicExtension = (*partialFailureExtension)(nil)

func (p *partialFailureExtension) CanApply(_ *extensioncommon.RuntimeConfig) bool {
	return true
}

func (p *partialFailureExtension) PatchListener(payload extensioncommon.ListenerPayload) (*envoy_listener_v3.Listener, bool, error) {
	// Modify original input message
	payload.Message.IgnoreGlobalConnLimit = true

	err := fmt.Errorf("oops - listener patch failed")
	if !p.failListener {
		err = nil
	}

	returnMsg := payload.Message
	if err != nil && !p.returnOnFailure {
		returnMsg = nil
	}

	patched := err == nil || p.returnOnFailure

	return returnMsg, patched, err
}

func (p *partialFailureExtension) PatchCluster(payload extensioncommon.ClusterPayload) (*envoy_cluster_v3.Cluster, bool, error) {
	// Modify original input message
	payload.Message.RespectDnsTtl = true

	err := fmt.Errorf("oops - cluster patch failed")
	if !p.failCluster {
		err = nil
	}

	returnMsg := payload.Message
	if err != nil && !p.returnOnFailure {
		returnMsg = nil
	}

	patched := err == nil || p.returnOnFailure

	return returnMsg, patched, err
}

type maybePanicExtension struct {
	extensioncommon.BasicExtensionAdapter
	panicOnCanApply bool
	panicOnPatch    bool
}

var _ extensioncommon.BasicExtension = (*maybePanicExtension)(nil)

func (m *maybePanicExtension) CanApply(_ *extensioncommon.RuntimeConfig) bool {
	if m.panicOnCanApply {
		panic(fmt.Errorf("this is an expected failure in CanApply"))
	}
	return true
}

func (m *maybePanicExtension) PatchListener(payload extensioncommon.ListenerPayload) (*envoy_listener_v3.Listener, bool, error) {
	if m.panicOnPatch {
		panic(fmt.Errorf("this is an expected failure in PatchListener"))
	}
	payload.Message.IgnoreGlobalConnLimit = true
	return payload.Message, true, nil
}
