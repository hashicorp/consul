package xds

import (
	"sync/atomic"
	"testing"
	"time"

	envoy_api_v2 "github.com/envoyproxy/go-control-plane/envoy/api/v2"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/stretchr/testify/require"
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

func TestServer_StreamAggregatedResources_v2_BasicProtocol_TCP(t *testing.T) {
	aclResolve := func(id string) (acl.Authorizer, error) {
		// Allow all
		return acl.RootAuthorizer("manage"), nil
	}
	scenario := newTestServerScenario(t, aclResolve, "web-sidecar-proxy", "", 0)
	mgr, errCh, envoy := scenario.mgr, scenario.errCh, scenario.envoy

	sid := structs.NewServiceID("web-sidecar-proxy", nil)

	// Register the proxy to create state needed to Watch() on
	mgr.RegisterProxy(t, sid)

	// Send initial cluster discover (empty payload)
	envoy.SendReq(t, ClusterType_v2, 0, 0)

	// Check no response sent yet
	assertChanBlocked(t, envoy.stream.sendCh)

	requireProtocolVersionGauge(t, scenario, "v2", 1)

	// Deliver a new snapshot
	snap := newTestSnapshot(t, nil, "")
	mgr.DeliverConfig(t, sid, snap)

	expectClusterResponse := func(v, n uint64) *envoy_api_v2.DiscoveryResponse {
		return &envoy_api_v2.DiscoveryResponse{
			VersionInfo: hexString(v),
			TypeUrl:     ClusterType_v2,
			Nonce:       hexString(n),
			Resources: makeTestResources_v2(t,
				makeTestCluster_v2(t, snap, "tcp:local_app"),
				makeTestCluster_v2(t, snap, "tcp:db"),
				makeTestCluster_v2(t, snap, "tcp:geo-cache"),
			),
		}
	}
	expectEndpointResponse := func(v, n uint64) *envoy_api_v2.DiscoveryResponse {
		return &envoy_api_v2.DiscoveryResponse{
			VersionInfo: hexString(v),
			TypeUrl:     EndpointType_v2,
			Nonce:       hexString(n),
			Resources: makeTestResources_v2(t,
				makeTestEndpoints_v2(t, snap, "tcp:db"),
				makeTestEndpoints_v2(t, snap, "tcp:geo-cache"),
			),
		}
	}
	expectListenerResponse := func(v, n uint64) *envoy_api_v2.DiscoveryResponse {
		return &envoy_api_v2.DiscoveryResponse{
			VersionInfo: hexString(v),
			TypeUrl:     ListenerType_v2,
			Nonce:       hexString(n),
			Resources: makeTestResources_v2(t,
				makeTestListener_v2(t, snap, "tcp:public_listener"),
				makeTestListener_v2(t, snap, "tcp:db"),
				makeTestListener_v2(t, snap, "tcp:geo-cache"),
			),
		}
	}

	assertResponseSent(t, envoy.stream.sendCh, expectClusterResponse(1, 1))

	// Envoy then tries to discover endpoints for those clusters. Technically it
	// includes the cluster names in the ResourceNames field but we ignore that
	// completely for now so not bothering to simulate that.
	envoy.SendReq(t, EndpointType_v2, 0, 0)

	// It also (in parallel) issues the next cluster request (which acts as an ACK
	// of the version we sent)
	envoy.SendReq(t, ClusterType_v2, 1, 1)

	// We should get a response immediately since the config is already present in
	// the server for endpoints. Note that this should not be racy if the server
	// is behaving well since the Cluster send above should be blocked until we
	// deliver a new config version.
	assertResponseSent(t, envoy.stream.sendCh, expectEndpointResponse(1, 2))

	// And no other response yet
	assertChanBlocked(t, envoy.stream.sendCh)

	// Envoy now sends listener request along with next endpoint one
	envoy.SendReq(t, ListenerType_v2, 0, 0)
	envoy.SendReq(t, EndpointType_v2, 1, 2)

	// And should get a response immediately.
	assertResponseSent(t, envoy.stream.sendCh, expectListenerResponse(1, 3))

	// Now send Route request along with next listener one
	envoy.SendReq(t, RouteType_v2, 0, 0)
	envoy.SendReq(t, ListenerType_v2, 1, 3)

	// We don't serve routes yet so this should block with no response
	assertChanBlocked(t, envoy.stream.sendCh)

	// WOOP! Envoy now has full connect config. Lets verify that if we update it,
	// all the responses get resent with the new version. We don't actually want
	// to change everything because that's tedious - our implementation will
	// actually resend all blocked types on the new "version" anyway since it
	// doesn't know _what_ changed. We could do something trivial but let's
	// simulate a leaf cert expiring and being rotated.
	snap.ConnectProxy.Leaf = proxycfg.TestLeafForCA(t, snap.Roots.Roots[0])
	mgr.DeliverConfig(t, sid, snap)

	// All 3 response that have something to return should return with new version
	// note that the ordering is not deterministic in general. Trying to make this
	// test order-agnostic though is a massive pain because we
	// don't know the order the nonces will be assigned. For now we rely and
	// require our implementation to always deliver updates in a specific order
	// which is reasonable anyway to ensure consistency of the config Envoy sees.
	assertResponseSent(t, envoy.stream.sendCh, expectClusterResponse(2, 4))
	assertResponseSent(t, envoy.stream.sendCh, expectEndpointResponse(2, 5))
	assertResponseSent(t, envoy.stream.sendCh, expectListenerResponse(2, 6))

	// Let's pretend that Envoy doesn't like that new listener config. It will ACK
	// all the others (same version) but NACK the listener. This is the most
	// subtle part of xDS and the server implementation so I'll elaborate. A full
	// description of the protocol can be found at
	// https://www.envoyproxy.io/docs/envoy/latest/api-docs/xds_protocol
	// Envoy delays making a followup request for a type until after it has
	// processed and applied the last response. The next request then will include
	// the nonce in the last response which acknowledges _receiving_ and handling
	// that response. It also includes the currently applied version. If all is
	// good and it successfully applies the config, then the version in the next
	// response will be the same version just sent. This is considered to be an
	// ACK of that version for that type. If envoy fails to apply the config for
	// some reason, it will still acknowledge that it received it (still return
	// the responses nonce), but will show the previous version it's still using.
	// This is considered a NACK. It's important that the server pay attention to
	// the _nonce_ and not the version when deciding what to send otherwise a bad
	// version that can't be applied in Envoy will cause a busy loop.
	//
	// In this case we are simulating that Envoy failed to apply the Listener
	// response but did apply the other types so all get the new nonces, but
	// listener stays on v1.
	envoy.SendReq(t, ClusterType_v2, 2, 4)
	envoy.SendReq(t, EndpointType_v2, 2, 5)
	envoy.SendReq(t, ListenerType_v2, 1, 6)

	// Even though we nacked, we should still NOT get then v2 listeners
	// redelivered since nothing has changed.
	assertChanBlocked(t, envoy.stream.sendCh)

	// Change config again and make sure it's delivered to everyone!
	snap.ConnectProxy.Leaf = proxycfg.TestLeafForCA(t, snap.Roots.Roots[0])
	mgr.DeliverConfig(t, sid, snap)

	assertResponseSent(t, envoy.stream.sendCh, expectClusterResponse(3, 7))
	assertResponseSent(t, envoy.stream.sendCh, expectEndpointResponse(3, 8))
	assertResponseSent(t, envoy.stream.sendCh, expectListenerResponse(3, 9))

	envoy.Close()
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("timed out waiting for handler to finish")
	}
}

func TestServer_StreamAggregatedResources_v2_BasicProtocol_HTTP(t *testing.T) {
	aclResolve := func(id string) (acl.Authorizer, error) {
		// Allow all
		return acl.RootAuthorizer("manage"), nil
	}
	scenario := newTestServerScenario(t, aclResolve, "web-sidecar-proxy", "", 0)
	mgr, errCh, envoy := scenario.mgr, scenario.errCh, scenario.envoy

	sid := structs.NewServiceID("web-sidecar-proxy", nil)

	// Register the proxy to create state needed to Watch() on
	mgr.RegisterProxy(t, sid)

	// Send initial cluster discover (empty payload)
	envoy.SendReq(t, ClusterType_v2, 0, 0)

	// Check no response sent yet
	assertChanBlocked(t, envoy.stream.sendCh)

	// Deliver a new snapshot
	// Deliver a new snapshot (tcp with one http upstream)
	snap := newTestSnapshot(t, nil, "http2", &structs.ServiceConfigEntry{
		Kind:     structs.ServiceDefaults,
		Name:     "db",
		Protocol: "http2",
	})
	mgr.DeliverConfig(t, sid, snap)

	expectClusterResponse := func(v, n uint64) *envoy_api_v2.DiscoveryResponse {
		return &envoy_api_v2.DiscoveryResponse{
			VersionInfo: hexString(v),
			TypeUrl:     ClusterType_v2,
			Nonce:       hexString(n),
			Resources: makeTestResources_v2(t,
				makeTestCluster_v2(t, snap, "tcp:local_app"),
				makeTestCluster_v2(t, snap, "http2:db"),
				makeTestCluster_v2(t, snap, "tcp:geo-cache"),
			),
		}
	}
	expectEndpointResponse := func(v, n uint64) *envoy_api_v2.DiscoveryResponse {
		return &envoy_api_v2.DiscoveryResponse{
			VersionInfo: hexString(v),
			TypeUrl:     EndpointType_v2,
			Nonce:       hexString(n),
			Resources: makeTestResources_v2(t,
				makeTestEndpoints_v2(t, snap, "http2:db"),
				makeTestEndpoints_v2(t, snap, "tcp:geo-cache"),
			),
		}
	}
	expectListenerResponse := func(v, n uint64) *envoy_api_v2.DiscoveryResponse {
		return &envoy_api_v2.DiscoveryResponse{
			VersionInfo: hexString(v),
			TypeUrl:     ListenerType_v2,
			Nonce:       hexString(n),
			Resources: makeTestResources_v2(t,
				makeTestListener_v2(t, snap, "tcp:public_listener"),
				makeTestListener_v2(t, snap, "http2:db"),
				makeTestListener_v2(t, snap, "tcp:geo-cache"),
			),
		}
	}

	runStep(t, "no-rds", func(t *testing.T) {

		// REQ: clusters
		envoy.SendReq(t, ClusterType_v2, 0, 0)

		// RESP: clusters
		assertResponseSent(t, envoy.stream.sendCh, expectClusterResponse(1, 1))

		assertChanBlocked(t, envoy.stream.sendCh)

		// REQ: endpoints
		envoy.SendReq(t, EndpointType_v2, 0, 0)

		// ACK: clusters
		envoy.SendReq(t, ClusterType_v2, 1, 1)

		// RESP: endpoints
		assertResponseSent(t, envoy.stream.sendCh, expectEndpointResponse(1, 2))

		assertChanBlocked(t, envoy.stream.sendCh)

		// REQ: listeners
		envoy.SendReq(t, ListenerType_v2, 0, 0)

		// ACK: endpoints
		envoy.SendReq(t, EndpointType_v2, 1, 2)

		// RESP: listeners
		assertResponseSent(t, envoy.stream.sendCh, expectListenerResponse(1, 3))

		assertChanBlocked(t, envoy.stream.sendCh)

		// ACK: listeners
		envoy.SendReq(t, ListenerType_v2, 1, 3)

		assertChanBlocked(t, envoy.stream.sendCh)
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

	// update this test helper to reflect the RDS-linked listener
	expectListenerResponse = func(v, n uint64) *envoy_api_v2.DiscoveryResponse {
		return &envoy_api_v2.DiscoveryResponse{
			VersionInfo: hexString(v),
			TypeUrl:     ListenerType_v2,
			Nonce:       hexString(n),
			Resources: makeTestResources_v2(t,
				makeTestListener_v2(t, snap, "tcp:public_listener"),
				makeTestListener_v2(t, snap, "http2:db:rds"),
				makeTestListener_v2(t, snap, "tcp:geo-cache"),
			),
		}
	}

	runStep(t, "with-rds", func(t *testing.T) {
		// RESP: listeners (but also a stray update of the other registered types)
		assertResponseSent(t, envoy.stream.sendCh, expectClusterResponse(2, 4))
		assertResponseSent(t, envoy.stream.sendCh, expectEndpointResponse(2, 5))
		assertResponseSent(t, envoy.stream.sendCh, expectListenerResponse(2, 6))

		assertChanBlocked(t, envoy.stream.sendCh)

		// ACK: listeners (but also stray ACKs of the other registered types)
		envoy.SendReq(t, ClusterType_v2, 2, 4)
		envoy.SendReq(t, EndpointType_v2, 2, 5)
		envoy.SendReq(t, ListenerType_v2, 2, 6)

		// REQ: routes
		envoy.SendReq(t, RouteType_v2, 0, 0)

		// RESP: routes
		assertResponseSent(t, envoy.stream.sendCh, &envoy_api_v2.DiscoveryResponse{
			VersionInfo: hexString(2),
			TypeUrl:     RouteType_v2,
			Nonce:       hexString(7),
			Resources: makeTestResources_v2(t,
				makeTestRoute_v2(t, "http2:db"),
			),
		})

		assertChanBlocked(t, envoy.stream.sendCh)

		// ACK: routes
		envoy.SendReq(t, RouteType_v2, 2, 7)
	})

	envoy.Close()
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("timed out waiting for handler to finish")
	}
}

func TestServer_StreamAggregatedResources_v2_ACLEnforcement(t *testing.T) {
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

			scenario := newTestServerScenario(t, aclResolve, "web-sidecar-proxy", tt.token, 0)
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
			envoy.SendReq(t, ListenerType_v2, 0, 0)

			if !tt.wantDenied {
				assertResponseSent(t, envoy.stream.sendCh, &envoy_api_v2.DiscoveryResponse{
					VersionInfo: hexString(1),
					TypeUrl:     ListenerType_v2,
					Nonce:       hexString(1),
					Resources: makeTestResources_v2(t,
						makeTestListener_v2(t, snap, "tcp:public_listener"),
						makeTestListener_v2(t, snap, "tcp:db"),
						makeTestListener_v2(t, snap, "tcp:geo-cache"),
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

func TestServer_StreamAggregatedResources_v2_ACLTokenDeleted_StreamTerminatedDuringDiscoveryRequest(t *testing.T) {
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
	scenario := newTestServerScenario(t, aclResolve, "web-sidecar-proxy", token,
		1*time.Hour, // make sure this doesn't kick in
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
	envoy.SendReq(t, ClusterType_v2, 0, 0)
	{
		err, ok := getError()
		require.NoError(t, err)
		require.False(t, ok)
	}

	// Check no response sent yet
	assertChanBlocked(t, envoy.stream.sendCh)
	{
		err, ok := getError()
		require.NoError(t, err)
		require.False(t, ok)
	}

	// Deliver a new snapshot
	snap := newTestSnapshot(t, nil, "")
	mgr.DeliverConfig(t, sid, snap)

	assertResponseSent(t, envoy.stream.sendCh, &envoy_api_v2.DiscoveryResponse{
		VersionInfo: hexString(1),
		TypeUrl:     ClusterType_v2,
		Nonce:       hexString(1),
		Resources: makeTestResources_v2(t,
			makeTestCluster_v2(t, snap, "tcp:local_app"),
			makeTestCluster_v2(t, snap, "tcp:db"),
			makeTestCluster_v2(t, snap, "tcp:geo-cache"),
		),
	})

	// Now nuke the ACL token.
	validToken.Store("")

	// It also (in parallel) issues the next cluster request (which acts as an ACK
	// of the version we sent)
	envoy.SendReq(t, ClusterType_v2, 1, 1)

	select {
	case err := <-errCh:
		require.Error(t, err)
		gerr, ok := status.FromError(err)
		require.Truef(t, ok, "not a grpc status error: type='%T' value=%v", err, err)
		require.Equal(t, codes.Unauthenticated, gerr.Code())
		require.Equal(t, "unauthenticated: ACL not found", gerr.Message())

		mgr.AssertWatchCancelled(t, sid)
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("timed out waiting for handler to finish")
	}
}

func TestServer_StreamAggregatedResources_v2_ACLTokenDeleted_StreamTerminatedInBackground(t *testing.T) {
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
	scenario := newTestServerScenario(t, aclResolve, "web-sidecar-proxy", token,
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
	envoy.SendReq(t, ClusterType_v2, 0, 0)
	{
		err, ok := getError()
		require.NoError(t, err)
		require.False(t, ok)
	}

	// Check no response sent yet
	assertChanBlocked(t, envoy.stream.sendCh)
	{
		err, ok := getError()
		require.NoError(t, err)
		require.False(t, ok)
	}

	// Deliver a new snapshot
	snap := newTestSnapshot(t, nil, "")
	mgr.DeliverConfig(t, sid, snap)

	assertResponseSent(t, envoy.stream.sendCh, &envoy_api_v2.DiscoveryResponse{
		VersionInfo: hexString(1),
		TypeUrl:     ClusterType_v2,
		Nonce:       hexString(1),
		Resources: makeTestResources_v2(t,
			makeTestCluster_v2(t, snap, "tcp:local_app"),
			makeTestCluster_v2(t, snap, "tcp:db"),
			makeTestCluster_v2(t, snap, "tcp:geo-cache"),
		),
	})

	// It also (in parallel) issues the next cluster request (which acts as an ACK
	// of the version we sent)
	envoy.SendReq(t, ClusterType_v2, 1, 1)

	// Check no response sent yet
	assertChanBlocked(t, envoy.stream.sendCh)
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

func TestServer_StreamAggregatedResources_v2_IngressEmptyResponse(t *testing.T) {
	aclResolve := func(id string) (acl.Authorizer, error) {
		// Allow all
		return acl.RootAuthorizer("manage"), nil
	}
	scenario := newTestServerScenario(t, aclResolve, "ingress-gateway", "", 0)
	mgr, errCh, envoy := scenario.mgr, scenario.errCh, scenario.envoy

	sid := structs.NewServiceID("ingress-gateway", nil)

	// Register the proxy to create state needed to Watch() on
	mgr.RegisterProxy(t, sid)

	// Send initial cluster discover
	envoy.SendReq(t, ClusterType_v2, 0, 0)

	// Check no response sent yet
	assertChanBlocked(t, envoy.stream.sendCh)

	// Deliver a new snapshot with no services
	snap := proxycfg.TestConfigSnapshotIngressGatewayNoServices(t)
	mgr.DeliverConfig(t, sid, snap)

	emptyClusterResp := &envoy_api_v2.DiscoveryResponse{
		VersionInfo: hexString(1),
		TypeUrl:     ClusterType_v2,
		Nonce:       hexString(1),
	}
	emptyListenerResp := &envoy_api_v2.DiscoveryResponse{
		VersionInfo: hexString(1),
		TypeUrl:     ListenerType_v2,
		Nonce:       hexString(2),
	}
	emptyRouteResp := &envoy_api_v2.DiscoveryResponse{
		VersionInfo: hexString(1),
		TypeUrl:     RouteType_v2,
		Nonce:       hexString(3),
	}

	assertResponseSent(t, envoy.stream.sendCh, emptyClusterResp)

	// Send initial listener discover
	envoy.SendReq(t, ListenerType_v2, 0, 0)
	assertResponseSent(t, envoy.stream.sendCh, emptyListenerResp)

	envoy.SendReq(t, RouteType_v2, 0, 0)
	assertResponseSent(t, envoy.stream.sendCh, emptyRouteResp)

	envoy.Close()
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("timed out waiting for handler to finish")
	}
}

func assertChanBlocked(t *testing.T, ch chan *envoy_api_v2.DiscoveryResponse) {
	t.Helper()
	select {
	case r := <-ch:
		t.Fatalf("chan should block but received: %v", r)
	case <-time.After(10 * time.Millisecond):
		return
	}
}

func assertResponseSent(t *testing.T, ch chan *envoy_api_v2.DiscoveryResponse, want *envoy_api_v2.DiscoveryResponse) {
	t.Helper()
	select {
	case got := <-ch:
		assertResponse(t, got, want)
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("no response received after 50ms")
	}
}

// assertResponse is a helper to test a envoy.DiscoveryResponse matches the
// expected value. We use JSON during comparison here because the responses use protobuf
// Any type which includes binary protobuf encoding.
func assertResponse(t *testing.T, got, want *envoy_api_v2.DiscoveryResponse) {
	t.Helper()

	gotJSON := protoToJSON(t, got)
	wantJSON := protoToJSON(t, want)
	require.JSONEqf(t, wantJSON, gotJSON, "got:\n%s", gotJSON)
}

func makeTestResources_v2(t *testing.T, resources ...proto.Message) []*any.Any {
	var ret []*any.Any
	for _, res := range resources {
		any, err := ptypes.MarshalAny(res)
		require.NoError(t, err)
		ret = append(ret, any)
	}
	return ret
}

func makeTestListener_v2(t *testing.T, snap *proxycfg.ConfigSnapshot, fixtureName string) *envoy_api_v2.Listener {
	v3 := makeTestListener(t, snap, fixtureName)
	v2, err := convertListenerToV2(v3)
	require.NoError(t, err)
	return v2
}

func makeTestCluster_v2(t *testing.T, snap *proxycfg.ConfigSnapshot, fixtureName string) *envoy_api_v2.Cluster {
	v3 := makeTestCluster(t, snap, fixtureName)
	v2, err := convertClusterToV2(v3)
	require.NoError(t, err)
	return v2
}

func makeTestEndpoints_v2(t *testing.T, snap *proxycfg.ConfigSnapshot, fixtureName string) *envoy_api_v2.ClusterLoadAssignment {
	v3 := makeTestEndpoints(t, snap, fixtureName)
	v2, err := convertClusterLoadAssignmentToV2(v3)
	require.NoError(t, err)
	return v2
}

func makeTestRoute_v2(t *testing.T, fixtureName string) *envoy_api_v2.RouteConfiguration {
	v3 := makeTestRoute(t, fixtureName)
	v2, err := convertRouteConfigurationToV2(v3)
	require.NoError(t, err)
	return v2
}
