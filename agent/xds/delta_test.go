package xds

import (
	"sync/atomic"
	"testing"
	"time"

	envoy_discovery_v3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"

	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
)

/* TODO: Test scenarios

- initial resource versions
- removing resources
- nack
- unsubscribe
- error during handling causing retry

*/

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

	snap := newTestSnapshot(t, nil, "")

	// Send initial cluster discover (empty payload)
	envoy.SendDeltaReq(t, ClusterType, &envoy_discovery_v3.DeltaDiscoveryRequest{
		// We'll assume we are testing a partial "reconnect"
		InitialResourceVersions: mustMakeVersionMap(t,
			makeTestCluster(t, snap, "tcp:geo-cache"),
		),
	})

	// Check no response sent yet
	assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

	// Deliver a new snapshot (tcp with one tcp upstream)
	mgr.DeliverConfig(t, sid, snap)

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
			// see what happens if you try to subscribe to an unknown thing
			"fake-endpoints",
		},
	})

	// It also (in parallel) issues the cluster ACK
	envoy.SendDeltaReqACK(t, ClusterType, 1, true, nil)

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
		),
	})

	// And no other response yet
	assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

	// Envoy now sends listener request
	// { "typeUrl": "type.googleapis.com/envoy.config.listener.v3.Listener" }
	envoy.SendDeltaReq(t, ListenerType, nil)

	// It also (in parallel) issues the endpoint ACK
	envoy.SendDeltaReqACK(t, EndpointType, 2, true, nil)

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

	// cleanup unused resources now that we've created/updated relevant things
	assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
		TypeUrl: EndpointType,
		Nonce:   hexString(4),
		RemovedResources: []string{
			"fake-endpoints", // correcting the errant subscription
		},
	})

	// And no other response yet
	assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

	// ACKs the listener
	envoy.SendDeltaReqACK(t, ListenerType, 3, true, nil)

	// ACK the endpoint removal
	envoy.SendDeltaReqACK(t, EndpointType, 4, true, nil)

	// If we re-subscribe to something even if there are no changes we get a
	// fresh copy.
	envoy.SendDeltaReq(t, EndpointType, &envoy_discovery_v3.DeltaDiscoveryRequest{
		ResourceNamesSubscribe: []string{
			"geo-cache.default.dc1.query.11111111-2222-3333-4444-555555555555.consul",
		},
	})

	assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
		TypeUrl: EndpointType,
		Nonce:   hexString(5),
		Resources: makeTestResources(t,
			makeTestEndpoints(t, snap, "tcp:geo-cache"),
		),
	})

	envoy.SendDeltaReqACK(t, EndpointType, 5, true, nil)

	// And no other response yet
	assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

	// TODO: test NACK

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
	// { "typeUrl": "type.googleapis.com/envoy.config.cluster.v3.Cluster" }
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

	require.True(t, t.Run("no-rds", func(t *testing.T) {
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
		// {
		//   "typeUrl": "type.googleapis.com/envoy.config.endpoint.v3.ClusterLoadAssignment",
		//   "resourceNamesSubscribe": [
		//     "api.default.dc1.internal.2902259a-31e7-62e0-fccc-0d482f347e98.consul"
		//   ]
		// }
		envoy.SendDeltaReq(t, EndpointType, &envoy_discovery_v3.DeltaDiscoveryRequest{
			ResourceNamesSubscribe: []string{
				"db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul",
				"geo-cache.default.dc1.query.11111111-2222-3333-4444-555555555555.consul",
			},
		})

		// It also (in parallel) issues the cluster ACK
		// { "typeUrl": "type.googleapis.com/envoy.config.cluster.v3.Cluster", "responseNonce": "00000001" }"
		envoy.SendDeltaReqACK(t, ClusterType, 1, true, nil)

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
		// { "typeUrl": "type.googleapis.com/envoy.config.listener.v3.Listener" }
		envoy.SendDeltaReq(t, ListenerType, nil)

		// It also (in parallel) issues the endpoint ACK
		// { "typeUrl": "type.googleapis.com/envoy.config.endpoint.v3.ClusterLoadAssignment", "responseNonce": "00000002" }
		envoy.SendDeltaReqACK(t, EndpointType, 2, true, nil)

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
		// { "typeUrl": "type.googleapis.com/envoy.config.endpoint.v3.Listener", "responseNonce": "00000003" }
		envoy.SendDeltaReqACK(t, ListenerType, 3, true, nil)

		// And no other response yet
		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)
	}))

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

	require.True(t, t.Run("with-rds", func(t *testing.T) {
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
		// {
		//   "typeUrl": "type.googleapis.com/envoy.config.route.v3.RouteConfiguration",
		//   "resourceNamesSubscribe": [
		//     "db"
		//   ]
		// }
		envoy.SendDeltaReq(t, RouteType, &envoy_discovery_v3.DeltaDiscoveryRequest{
			ResourceNamesSubscribe: []string{
				"db",
			},
		})

		// ACKs the listener
		// { "typeUrl": "type.googleapis.com/envoy.config.endpoint.v3.Listener", "responseNonce": "00000003" }
		envoy.SendDeltaReqACK(t, ListenerType, 4, true, nil)

		// And should get a response immediately.
		assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
			TypeUrl: RouteType,
			Nonce:   hexString(5),
			Resources: makeTestResources(t,
				makeTestRoute(t, "http2:db"),
			),
		})

		envoy.SendDeltaReqACK(t, RouteType, 5, true, nil)

		assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)
	}))

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
				policy, err := acl.NewPolicyFromSource("", 0, tt.acl, acl.SyntaxLegacy, nil, nil)
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

	policy, err := acl.NewPolicyFromSource("", 0, aclRules, acl.SyntaxLegacy, nil, nil)
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

	policy, err := acl.NewPolicyFromSource("", 0, aclRules, acl.SyntaxLegacy, nil, nil)
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

	// RESP: clustesr
	assertDeltaResponseSent(t, envoy.deltaStream.sendCh, &envoy_discovery_v3.DeltaDiscoveryResponse{
		TypeUrl: ClusterType,
		Nonce:   hexString(1),
	})

	assertDeltaChanBlocked(t, envoy.deltaStream.sendCh)

	// ACK: clusters
	envoy.SendDeltaReqACK(t, ClusterType, 1, true, nil)

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
// JSON representation we expect. We use JSON because the responses use protobuf
// Any type which includes binary protobuf encoding and would make creating
// expected structs require the same code that is under test!
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
