package xds

import (
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	envoy "github.com/envoyproxy/go-control-plane/envoy/api/v2"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
)

// testManager is a mock of proxycfg.Manager that's simpler to control for
// testing. It also implements ConnectAuthz to allow control over authorization.
type testManager struct {
	sync.Mutex
	chans   map[structs.ServiceID]chan *proxycfg.ConfigSnapshot
	cancels chan structs.ServiceID
	authz   map[string]connectAuthzResult
}

type connectAuthzResult struct {
	authz    bool
	reason   string
	m        *cache.ResultMeta
	err      error
	validate func(req *structs.ConnectAuthorizeRequest) error
}

func newTestManager(t *testing.T) *testManager {
	return &testManager{
		chans:   map[structs.ServiceID]chan *proxycfg.ConfigSnapshot{},
		cancels: make(chan structs.ServiceID, 10),
		authz:   make(map[string]connectAuthzResult),
	}
}

// RegisterProxy simulates a proxy registration
func (m *testManager) RegisterProxy(t *testing.T, proxyID structs.ServiceID) {
	m.Lock()
	defer m.Unlock()
	m.chans[proxyID] = make(chan *proxycfg.ConfigSnapshot, 1)
}

// Deliver simulates a proxy registration
func (m *testManager) DeliverConfig(t *testing.T, proxyID structs.ServiceID, cfg *proxycfg.ConfigSnapshot) {
	t.Helper()
	m.Lock()
	defer m.Unlock()
	select {
	case m.chans[proxyID] <- cfg:
	case <-time.After(10 * time.Millisecond):
		t.Fatalf("took too long to deliver config")
	}
}

// Watch implements ConfigManager
func (m *testManager) Watch(proxyID structs.ServiceID) (<-chan *proxycfg.ConfigSnapshot, proxycfg.CancelFunc) {
	m.Lock()
	defer m.Unlock()
	// ch might be nil but then it will just block forever
	return m.chans[proxyID], func() {
		m.cancels <- proxyID
	}
}

// AssertWatchCancelled checks that the most recent call to a Watch cancel func
// was from the specified proxyID and that one is made in a short time. This
// probably won't work if you are running multiple Watches in parallel on
// multiple proxyIDS due to timing/ordering issues but I don't think we need to
// do that.
func (m *testManager) AssertWatchCancelled(t *testing.T, proxyID structs.ServiceID) {
	t.Helper()
	select {
	case got := <-m.cancels:
		require.Equal(t, proxyID, got)
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("timed out waiting for Watch cancel for %s", proxyID)
	}
}

func TestServer_StreamAggregatedResources_BasicProtocol(t *testing.T) {
	mgr := newTestManager(t)
	aclResolve := func(id string) (acl.Authorizer, error) {
		// Allow all
		return acl.RootAuthorizer("manage"), nil
	}
	envoy := NewTestEnvoy(t, "web-sidecar-proxy", "")
	defer envoy.Close()

	s := Server{
		Logger:       testutil.Logger(t),
		CfgMgr:       mgr,
		ResolveToken: aclResolve,
	}

	sid := structs.NewServiceID("web-sidecar-proxy", nil)

	go func() {
		err := s.StreamAggregatedResources(envoy.stream)
		require.NoError(t, err)
	}()

	// Register the proxy to create state needed to Watch() on
	mgr.RegisterProxy(t, sid)

	// Send initial cluster discover
	envoy.SendReq(t, ClusterType, 0, 0)

	// Check no response sent yet
	assertChanBlocked(t, envoy.stream.sendCh)

	// Deliver a new snapshot
	snap := proxycfg.TestConfigSnapshot(t)
	mgr.DeliverConfig(t, sid, snap)

	assertResponseSent(t, envoy.stream.sendCh, expectClustersJSON(snap, 1, 1))

	// Envoy then tries to discover endpoints for those clusters. Technically it
	// includes the cluster names in the ResourceNames field but we ignore that
	// completely for now so not bothering to simulate that.
	envoy.SendReq(t, EndpointType, 0, 0)

	// It also (in parallel) issues the next cluster request (which acts as an ACK
	// of the version we sent)
	envoy.SendReq(t, ClusterType, 1, 1)

	// We should get a response immediately since the config is already present in
	// the server for endpoints. Note that this should not be racy if the server
	// is behaving well since the Cluster send above should be blocked until we
	// deliver a new config version.
	assertResponseSent(t, envoy.stream.sendCh, expectEndpointsJSON(1, 2))

	// And no other response yet
	assertChanBlocked(t, envoy.stream.sendCh)

	// Envoy now sends listener request along with next endpoint one
	envoy.SendReq(t, ListenerType, 0, 0)
	envoy.SendReq(t, EndpointType, 1, 2)

	// And should get a response immediately.
	assertResponseSent(t, envoy.stream.sendCh, expectListenerJSON(snap, 1, 3))

	// Now send Route request along with next listener one
	envoy.SendReq(t, RouteType, 0, 0)
	envoy.SendReq(t, ListenerType, 1, 3)

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
	// test order-agnostic though is a massive pain since we are comparing
	// non-identical JSON strings (so can simply sort by anything) and because we
	// don't know the order the nonces will be assigned. For now we rely and
	// require our implementation to always deliver updates in a specific order
	// which is reasonable anyway to ensure consistency of the config Envoy sees.
	assertResponseSent(t, envoy.stream.sendCh, expectClustersJSON(snap, 2, 4))
	assertResponseSent(t, envoy.stream.sendCh, expectEndpointsJSON(2, 5))
	assertResponseSent(t, envoy.stream.sendCh, expectListenerJSON(snap, 2, 6))

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
	envoy.SendReq(t, ClusterType, 2, 4)
	envoy.SendReq(t, EndpointType, 2, 5)
	envoy.SendReq(t, ListenerType, 1, 6) // v1 is a NACK

	// Even though we nacked, we should still NOT get then v2 listeners
	// redelivered since nothing has changed.
	assertChanBlocked(t, envoy.stream.sendCh)

	// Change config again and make sure it's delivered to everyone!
	snap.ConnectProxy.Leaf = proxycfg.TestLeafForCA(t, snap.Roots.Roots[0])
	mgr.DeliverConfig(t, sid, snap)

	assertResponseSent(t, envoy.stream.sendCh, expectClustersJSON(snap, 3, 7))
	assertResponseSent(t, envoy.stream.sendCh, expectEndpointsJSON(3, 8))
	assertResponseSent(t, envoy.stream.sendCh, expectListenerJSON(snap, 3, 9))
}

func expectEndpointsJSON(v, n uint64) string {
	return `{
		"versionInfo": "` + hexString(v) + `",
		"resources": [
			{
				"@type": "type.googleapis.com/envoy.api.v2.ClusterLoadAssignment",
				"clusterName": "db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul",
				"endpoints": [
					{
						"lbEndpoints": [
							{
								"endpoint": {
									"address": {
										"socketAddress": {
											"address": "10.10.1.1",
											"portValue": 8080
										}
									}
								},
								"healthStatus": "HEALTHY",
								"loadBalancingWeight": 1
							},
							{
								"endpoint": {
									"address": {
										"socketAddress": {
											"address": "10.10.1.2",
											"portValue": 8080
										}
									}
								},
								"healthStatus": "HEALTHY",
								"loadBalancingWeight": 1
							}
						]
					}
				]
			},
			{
				"@type": "type.googleapis.com/envoy.api.v2.ClusterLoadAssignment",
				"clusterName": "geo-cache.default.dc1.query.11111111-2222-3333-4444-555555555555.consul",
				"endpoints": [
					{
						"lbEndpoints": [
							{
								"endpoint": {
									"address": {
										"socketAddress": {
											"address": "10.10.1.1",
											"portValue": 8080
										}
									}
								},
								"healthStatus": "HEALTHY",
								"loadBalancingWeight": 1
							},
							{
								"endpoint": {
									"address": {
										"socketAddress": {
											"address": "10.10.1.2",
											"portValue": 8080
										}
									}
								},
								"healthStatus": "HEALTHY",
								"loadBalancingWeight": 1
							}
						]
					}
				]
			}
		],
		"typeUrl": "type.googleapis.com/envoy.api.v2.ClusterLoadAssignment",
		"nonce": "` + hexString(n) + `"
	}`
}

func expectedUpstreamTransportSocketJSON(snap *proxycfg.ConfigSnapshot, sni string) string {
	return expectedTransportSocketJSON(snap, "type.googleapis.com/envoy.api.v2.auth.UpstreamTlsContext", false, sni)
}

func expectedPublicTransportSocketJSON(snap *proxycfg.ConfigSnapshot) string {
	return expectedTransportSocketJSON(snap, "type.googleapis.com/envoy.api.v2.auth.DownstreamTlsContext", true, "")
}

func expectedTransportSocketJSON(
	snap *proxycfg.ConfigSnapshot,
	extType string,
	requireClientCert bool,
	sni string,
) string {
	// Assume just one root for now, can get fancier later if needed.
	caPEM := snap.Roots.Roots[0].RootCert
	reqClient := ""
	if requireClientCert {
		reqClient = `,
		"requireClientCertificate": true`
	}

	upstreamSNI := ""
	if sni != "" {
		upstreamSNI = `,
		"sni": "` + sni + `"`
	}

	return `{
	"name": "tls",
	"typedConfig": {
		"@type": "` + extType + `",
		"commonTlsContext": {
			"tlsParams": {},
			"tlsCertificates": [
				{
					"certificateChain": {
						"inlineString": "` + strings.Replace(snap.Leaf().CertPEM, "\n", "\\n", -1) + `"
					},
					"privateKey": {
						"inlineString": "` + strings.Replace(snap.Leaf().PrivateKeyPEM, "\n", "\\n", -1) + `"
					}
				}
			],
			"validationContext": {
				"trustedCa": {
					"inlineString": "` + strings.Replace(caPEM, "\n", "\\n", -1) + `"
				}
			}
		}
		` + reqClient + `
		` + upstreamSNI + `
 		}
	}`
}

func assertChanBlocked(t *testing.T, ch chan *envoy.DiscoveryResponse) {
	t.Helper()
	select {
	case r := <-ch:
		t.Fatalf("chan should block but received: %v", r)
	case <-time.After(10 * time.Millisecond):
		return
	}
}

func assertResponseSent(t *testing.T, ch chan *envoy.DiscoveryResponse, wantJSON string) {
	t.Helper()
	select {
	case r := <-ch:
		assertResponse(t, r, wantJSON)
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("no response received after 50ms")
	}
}

// assertResponse is a helper to test a envoy.DiscoveryResponse matches the
// JSON representation we expect. We use JSON because the responses use protobuf
// Any type which includes binary protobuf encoding and would make creating
// expected structs require the same code that is under test!
func assertResponse(t *testing.T, r *envoy.DiscoveryResponse, wantJSON string) {
	t.Helper()
	gotJSON := responseToJSON(t, r)
	require.JSONEqf(t, wantJSON, gotJSON, "got:\n%s", gotJSON)
}

func TestServer_StreamAggregatedResources_ACLEnforcement(t *testing.T) {

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
			mgr := newTestManager(t)
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
			envoy := NewTestEnvoy(t, "web-sidecar-proxy", tt.token)
			defer envoy.Close()

			s := Server{
				Logger:       testutil.Logger(t),
				CfgMgr:       mgr,
				ResolveToken: aclResolve,
			}

			errCh := make(chan error, 1)
			go func() {
				errCh <- s.StreamAggregatedResources(envoy.stream)
			}()

			sid := structs.NewServiceID("web-sidecar-proxy", nil)
			// Register the proxy to create state needed to Watch() on
			mgr.RegisterProxy(t, sid)

			// Deliver a new snapshot
			snap := tt.cfgSnap
			if snap == nil {
				snap = proxycfg.TestConfigSnapshot(t)
			}
			mgr.DeliverConfig(t, sid, snap)

			// Send initial listener discover, in real life Envoy always sends cluster
			// first but it doesn't really matter and listener has a response that
			// includes the token in the ext authz filter so lets us test more stuff.
			envoy.SendReq(t, ListenerType, 0, 0)

			if !tt.wantDenied {
				assertResponseSent(t, envoy.stream.sendCh, expectListenerJSON(snap, 1, 1))
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

func TestServer_StreamAggregatedResources_ACLTokenDeleted_StreamTerminatedDuringDiscoveryRequest(t *testing.T) {
	aclRules := `service "web" { policy = "write" }`
	token := "service-write-on-web"

	policy, err := acl.NewPolicyFromSource("", 0, aclRules, acl.SyntaxLegacy, nil, nil)
	require.NoError(t, err)

	var validToken atomic.Value
	validToken.Store(token)

	mgr := newTestManager(t)
	aclResolve := func(id string) (acl.Authorizer, error) {
		if token := validToken.Load(); token == nil || id != token.(string) {
			return nil, acl.ErrNotFound
		}

		return acl.NewPolicyAuthorizerWithDefaults(acl.RootAuthorizer("deny"), []*acl.Policy{policy}, nil)
	}
	envoy := NewTestEnvoy(t, "web-sidecar-proxy", token)
	defer envoy.Close()

	s := Server{
		Logger:             testutil.Logger(t),
		CfgMgr:             mgr,
		ResolveToken:       aclResolve,
		AuthCheckFrequency: 1 * time.Hour, // make sure this doesn't kick in
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.StreamAggregatedResources(envoy.stream)
	}()

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
	envoy.SendReq(t, ClusterType, 0, 0)
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
	snap := proxycfg.TestConfigSnapshot(t)
	mgr.DeliverConfig(t, sid, snap)

	assertResponseSent(t, envoy.stream.sendCh, expectClustersJSON(snap, 1, 1))

	// Now nuke the ACL token.
	validToken.Store("")

	// It also (in parallel) issues the next cluster request (which acts as an ACK
	// of the version we sent)
	envoy.SendReq(t, ClusterType, 1, 1)

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

func TestServer_StreamAggregatedResources_ACLTokenDeleted_StreamTerminatedInBackground(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	aclRules := `service "web" { policy = "write" }`
	token := "service-write-on-web"

	policy, err := acl.NewPolicyFromSource("", 0, aclRules, acl.SyntaxLegacy, nil, nil)
	require.NoError(t, err)

	var validToken atomic.Value
	validToken.Store(token)

	mgr := newTestManager(t)
	aclResolve := func(id string) (acl.Authorizer, error) {
		if token := validToken.Load(); token == nil || id != token.(string) {
			return nil, acl.ErrNotFound
		}

		return acl.NewPolicyAuthorizerWithDefaults(acl.RootAuthorizer("deny"), []*acl.Policy{policy}, nil)
	}
	envoy := NewTestEnvoy(t, "web-sidecar-proxy", token)
	defer envoy.Close()

	s := Server{
		Logger:             testutil.Logger(t),
		CfgMgr:             mgr,
		ResolveToken:       aclResolve,
		AuthCheckFrequency: 100 * time.Millisecond, // Make this short.
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.StreamAggregatedResources(envoy.stream)
	}()

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
	envoy.SendReq(t, ClusterType, 0, 0)
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
	snap := proxycfg.TestConfigSnapshot(t)
	mgr.DeliverConfig(t, sid, snap)

	assertResponseSent(t, envoy.stream.sendCh, expectClustersJSON(snap, 1, 1))

	// It also (in parallel) issues the next cluster request (which acts as an ACK
	// of the version we sent)
	envoy.SendReq(t, ClusterType, 1, 1)

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

func TestServer_StreamAggregatedResources_IngressEmptyResponse(t *testing.T) {
	mgr := newTestManager(t)
	aclResolve := func(id string) (acl.Authorizer, error) {
		// Allow all
		return acl.RootAuthorizer("manage"), nil
	}
	envoy := NewTestEnvoy(t, "ingress-gateway", "")
	defer envoy.Close()

	s := Server{
		Logger:       testutil.Logger(t),
		CfgMgr:       mgr,
		ResolveToken: aclResolve,
	}

	sid := structs.NewServiceID("ingress-gateway", nil)

	go func() {
		err := s.StreamAggregatedResources(envoy.stream)
		require.NoError(t, err)
	}()

	// Register the proxy to create state needed to Watch() on
	mgr.RegisterProxy(t, sid)

	// Send initial cluster discover
	envoy.SendReq(t, ClusterType, 0, 0)

	// Check no response sent yet
	assertChanBlocked(t, envoy.stream.sendCh)

	// Deliver a new snapshot with no services
	snap := proxycfg.TestConfigSnapshotIngressGatewayNoServices(t)
	mgr.DeliverConfig(t, sid, snap)

	emptyClusterJSON := `{
		"versionInfo": "` + hexString(1) + `",
		"typeUrl": "type.googleapis.com/envoy.api.v2.Cluster",
		"resources": [],
		"nonce": "` + hexString(1) + `"
		}`
	emptyListenerJSON := `{
		"versionInfo": "` + hexString(1) + `",
		"typeUrl": "type.googleapis.com/envoy.api.v2.Listener",
		"resources": [],
		"nonce": "` + hexString(2) + `"
		}`
	emptyRouteJSON := `{
		"versionInfo": "` + hexString(1) + `",
		"typeUrl": "type.googleapis.com/envoy.api.v2.RouteConfiguration",
		"resources": [],
		"nonce": "` + hexString(3) + `"
		}`

	assertResponseSent(t, envoy.stream.sendCh, emptyClusterJSON)

	// Send initial listener discover
	envoy.SendReq(t, ListenerType, 0, 0)
	assertResponseSent(t, envoy.stream.sendCh, emptyListenerJSON)

	envoy.SendReq(t, RouteType, 0, 0)
	assertResponseSent(t, envoy.stream.sendCh, emptyRouteJSON)
}
