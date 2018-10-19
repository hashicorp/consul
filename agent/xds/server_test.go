package xds

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"testing"
	"text/template"
	"time"

	envoy "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/gogo/protobuf/jsonpb"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
)

// testManager is a mock of proxycfg.Manager that's simpler to control for
// testing. It also implements ConnectAuthz to allow control over authorization.
type testManager struct {
	sync.Mutex
	chans   map[string]chan *proxycfg.ConfigSnapshot
	cancels chan string
	authz   map[string]connectAuthzResult
}

type connectAuthzResult struct {
	authz  bool
	reason string
	m      *cache.ResultMeta
	err    error
}

func newTestManager(t *testing.T) *testManager {
	return &testManager{
		chans:   map[string]chan *proxycfg.ConfigSnapshot{},
		cancels: make(chan string, 10),
		authz:   make(map[string]connectAuthzResult),
	}
}

// RegisterProxy simulates a proxy registration
func (m *testManager) RegisterProxy(t *testing.T, proxyID string) {
	m.Lock()
	defer m.Unlock()
	m.chans[proxyID] = make(chan *proxycfg.ConfigSnapshot, 1)
}

// Deliver simulates a proxy registration
func (m *testManager) DeliverConfig(t *testing.T, proxyID string, cfg *proxycfg.ConfigSnapshot) {
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
func (m *testManager) Watch(proxyID string) (<-chan *proxycfg.ConfigSnapshot, proxycfg.CancelFunc) {
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
// multiple proxyIDS due to timing/ordering issues but I dont think we need to
// do that.
func (m *testManager) AssertWatchCancelled(t *testing.T, proxyID string) {
	t.Helper()
	select {
	case got := <-m.cancels:
		require.Equal(t, proxyID, got)
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("timed out waiting for Watch cancel for %s", proxyID)
	}
}

// ConnectAuthorize implements ConnectAuthz
func (m *testManager) ConnectAuthorize(token string, req *structs.ConnectAuthorizeRequest) (authz bool, reason string, meta *cache.ResultMeta, err error) {
	m.Lock()
	defer m.Unlock()
	if res, ok := m.authz[token]; ok {
		return res.authz, res.reason, res.m, res.err
	}
	// Default allow but with reason that won't match by accident in a test case
	return true, "OK: allowed by default test implementation", nil, nil
}

func TestServer_StreamAggregatedResources_BasicProtocol(t *testing.T) {
	logger := log.New(os.Stderr, "", log.LstdFlags)
	mgr := newTestManager(t)
	aclResolve := func(id string) (acl.Authorizer, error) {
		// Allow all
		return acl.RootAuthorizer("manage"), nil
	}
	envoy := NewTestEnvoy(t, "web-sidecar-proxy", "")
	defer envoy.Close()

	s := Server{logger, mgr, mgr, aclResolve}

	go func() {
		err := s.StreamAggregatedResources(envoy.stream)
		require.NoError(t, err)
	}()

	// Register the proxy to create state needed to Watch() on
	mgr.RegisterProxy(t, "web-sidecar-proxy")

	// Send initial cluster discover
	envoy.SendReq(t, ClusterType, 0, 0)

	// Check no response sent yet
	assertChanBlocked(t, envoy.stream.sendCh)

	// Deliver a new snapshot
	snap := proxycfg.TestConfigSnapshot(t)
	mgr.DeliverConfig(t, "web-sidecar-proxy", snap)

	assertResponseSent(t, envoy.stream.sendCh, expectClustersJSON(t, snap, "", 1, 1))

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
	assertResponseSent(t, envoy.stream.sendCh, expectEndpointsJSON(t, snap, "", 1, 2))

	// And no other response yet
	assertChanBlocked(t, envoy.stream.sendCh)

	// Envoy now sends listener request along with next endpoint one
	envoy.SendReq(t, ListenerType, 0, 0)
	envoy.SendReq(t, EndpointType, 1, 2)

	// And should get a response immediately.
	assertResponseSent(t, envoy.stream.sendCh, expectListenerJSON(t, snap, "", 1, 3))

	// Now send Route request along with next listner one
	envoy.SendReq(t, RouteType, 0, 0)
	envoy.SendReq(t, ListenerType, 1, 3)

	// We don't serve routes yet so this shoould block with no response
	assertChanBlocked(t, envoy.stream.sendCh)

	// WOOP! Envoy now has full connect config. Lets verify that if we update it,
	// all the responses get resent with the new version. We don't actually want
	// to change everything because that's tedious - our implementation will
	// actually resend all blocked types on the new "version" anyway since it
	// doesn't know _what_ changed. We could do something trivial but let's
	// simulate a leaf cert expiring and being rotated.
	snap.Leaf = proxycfg.TestLeafForCA(t, snap.Roots.Roots[0])
	mgr.DeliverConfig(t, "web-sidecar-proxy", snap)

	// All 3 response that have something to return should return with new version
	// note that the ordering is not determinisic in general. Trying to make this
	// test order-agnostic though is a massive pain since we are comparing
	// non-identical JSON strings (so can simply sort by anything) and because we
	// don't know the order the nonces will be assigned. For now we rely and
	// require our implementation to always deliver updates in a specific order
	// which is reasonable anyway to ensure consistency of the config Envoy sees.
	assertResponseSent(t, envoy.stream.sendCh, expectClustersJSON(t, snap, "", 2, 4))
	assertResponseSent(t, envoy.stream.sendCh, expectEndpointsJSON(t, snap, "", 2, 5))
	assertResponseSent(t, envoy.stream.sendCh, expectListenerJSON(t, snap, "", 2, 6))

	// Let's pretent that Envoy doesn't like that new listener config. It will ACK
	// all the others (same version) but NACK the listener. This is the most
	// subtle part of xDS and the server implementation so I'll elaborate. A full
	// description of the protocol can be found at
	// https://github.com/envoyproxy/data-plane-api/blob/master/XDS_PROTOCOL.md.
	// Envoy delays making a followup reqeest for a type until after it has
	// processed and applied the last response. The next request then will include
	// the nonce in the last response which acknowledges _recieving_ and handling
	// that response. It also includes the currently applied version. If all is
	// good and it successfully applies the config, then the version in the next
	// response will be the same version just sent. This is considered to be an
	// ACK of that version for that type. If envoy fails to apply the config for
	// some reason, it will still acknowledge that it received it (still return
	// the responses nonce), but will show the previous version it's still using.
	// This is considered a NACK. It's impotant that the server pay attention to
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
	snap.Leaf = proxycfg.TestLeafForCA(t, snap.Roots.Roots[0])
	mgr.DeliverConfig(t, "web-sidecar-proxy", snap)

	assertResponseSent(t, envoy.stream.sendCh, expectClustersJSON(t, snap, "", 3, 7))
	assertResponseSent(t, envoy.stream.sendCh, expectEndpointsJSON(t, snap, "", 3, 8))
	assertResponseSent(t, envoy.stream.sendCh, expectListenerJSON(t, snap, "", 3, 9))
}

func expectListenerJSONResources(t *testing.T, snap *proxycfg.ConfigSnapshot, token string, v, n uint64) map[string]string {
	tokenVal := ""
	if token != "" {
		tokenVal = fmt.Sprintf(",\n"+`"value": "%s"`, token)
	}
	return map[string]string{
		"public_listener": `{
													"@type": "type.googleapis.com/envoy.api.v2.Listener",
													"name": "public_listener:0.0.0.0:9999",
													"address": {
														"socketAddress": {
															"address": "0.0.0.0",
															"portValue": 9999
														}
													},
													"filterChains": [
														{
															"tlsContext": ` + expectedPublicTLSContextJSON(t, snap) + `,
															"filters": [
																{
																	"name": "envoy.ext_authz",
																	"config": {
																			"grpc_service": {
																					"envoy_grpc": {
																						"cluster_name": "local_agent"
																					},
																					"initial_metadata": [
																						{
																							"key": "x-consul-token"
																							` + tokenVal + `
																						}
																					]
																				},
																			"stat_prefix": "connect_authz"
																		}
																},
																{
																	"name": "envoy.tcp_proxy",
																	"config": {
																			"cluster": "local_app",
																			"stat_prefix": "public_listener"
																		}
																}
															]
														}
													]
												}`,
		"service:db": `{
										"@type": "type.googleapis.com/envoy.api.v2.Listener",
										"name": "service:db:127.0.0.1:9191",
										"address": {
											"socketAddress": {
												"address": "127.0.0.1",
												"portValue": 9191
											}
										},
										"filterChains": [
											{
												"filters": [
													{
														"name": "envoy.tcp_proxy",
														"config": {
																"cluster": "service:db",
																"stat_prefix": "service:db"
															}
													}
												]
											}
										]
									}`,
		"prepared_query:geo-cache": `{
																	"@type": "type.googleapis.com/envoy.api.v2.Listener",
																	"name": "prepared_query:geo-cache:127.10.10.10:8181",
																	"address": {
																		"socketAddress": {
																			"address": "127.10.10.10",
																			"portValue": 8181
																		}
																	},
																	"filterChains": [
																		{
																			"filters": [
																				{
																					"name": "envoy.tcp_proxy",
																					"config": {
																							"cluster": "prepared_query:geo-cache",
																							"stat_prefix": "prepared_query:geo-cache"
																						}
																				}
																			]
																		}
																	]
																}`,
	}
}

func expectListenerJSONFromResources(t *testing.T, snap *proxycfg.ConfigSnapshot, token string, v, n uint64, resourcesJSON map[string]string) string {
	resJSON := ""
	// Sort resources into specific order because that matters in JSONEq
	// comparison later.
	keyOrder := []string{"public_listener"}
	for _, u := range snap.Proxy.Upstreams {
		keyOrder = append(keyOrder, u.Identifier())
	}
	for _, k := range keyOrder {
		j, ok := resourcesJSON[k]
		if !ok {
			continue
		}
		if resJSON != "" {
			resJSON += ",\n"
		}
		resJSON += j
	}
	return `{
		"versionInfo": "` + hexString(v) + `",
		"resources": [` + resJSON + `],
		"typeUrl": "type.googleapis.com/envoy.api.v2.Listener",
		"nonce": "` + hexString(n) + `"
		}`
}

func expectListenerJSON(t *testing.T, snap *proxycfg.ConfigSnapshot, token string, v, n uint64) string {
	return expectListenerJSONFromResources(t, snap, token, v, n,
		expectListenerJSONResources(t, snap, token, v, n))
}

func expectEndpointsJSON(t *testing.T, snap *proxycfg.ConfigSnapshot, token string, v, n uint64) string {
	return `{
		"versionInfo": "` + hexString(v) + `",
		"resources": [
			{
				"@type": "type.googleapis.com/envoy.api.v2.ClusterLoadAssignment",
				"clusterName": "service:db",
				"endpoints": [
					{
						"lbEndpoints": [
							{
								"endpoint": {
									"address": {
										"socketAddress": {
											"address": "10.10.1.1",
											"portValue": 0
										}
									}
								}
							},
							{
								"endpoint": {
									"address": {
										"socketAddress": {
											"address": "10.10.1.2",
											"portValue": 0
										}
									}
								}
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

func expectClustersJSON(t *testing.T, snap *proxycfg.ConfigSnapshot, token string, v, n uint64) string {
	return `{
		"versionInfo": "` + hexString(v) + `",
		"resources": [
			{
				"@type": "type.googleapis.com/envoy.api.v2.Cluster",
				"name": "local_app",
				"connectTimeout": "5s",
				"hosts": [
					{
						"socketAddress": {
							"address": "127.0.0.1",
							"portValue": 8080
						}
					}
				]
			},
			{
				"@type": "type.googleapis.com/envoy.api.v2.Cluster",
				"name": "service:db",
				"type": "EDS",
				"edsClusterConfig": {
					"edsConfig": {
						"ads": {

						}
					}
				},
				"connectTimeout": "5s",
				"tlsContext": ` + expectedUpstreamTLSContextJSON(t, snap) + `
			},
			{
				"@type": "type.googleapis.com/envoy.api.v2.Cluster",
				"name": "prepared_query:geo-cache",
				"type": "EDS",
				"edsClusterConfig": {
					"edsConfig": {
						"ads": {

						}
					}
				},
				"connectTimeout": "5s",
				"tlsContext": ` + expectedUpstreamTLSContextJSON(t, snap) + `
			}
		],
		"typeUrl": "type.googleapis.com/envoy.api.v2.Cluster",
		"nonce": "` + hexString(n) + `"
	}
	`
}

func expectedUpstreamTLSContextJSON(t *testing.T, snap *proxycfg.ConfigSnapshot) string {
	return expectedTLSContextJSON(t, snap, false)
}

func expectedPublicTLSContextJSON(t *testing.T, snap *proxycfg.ConfigSnapshot) string {
	return expectedTLSContextJSON(t, snap, true)
}

func expectedTLSContextJSON(t *testing.T, snap *proxycfg.ConfigSnapshot, requireClientCert bool) string {
	// Assume just one root for now, can get fancier later if needed.
	caPEM := snap.Roots.Roots[0].RootCert
	reqClient := ""
	if requireClientCert {
		reqClient = `,
		"requireClientCertificate": true`
	}
	return `{
		"commonTlsContext": {
			"tlsParams": {},
			"tlsCertificates": [
				{
					"certificateChain": {
						"inlineString": "` + strings.Replace(snap.Leaf.CertPEM, "\n", "\\n", -1) + `"
					},
					"privateKey": {
						"inlineString": "` + strings.Replace(snap.Leaf.PrivateKeyPEM, "\n", "\\n", -1) + `"
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
	}`
}

func assertChanBlocked(t *testing.T, ch chan *envoy.DiscoveryResponse) {
	t.Helper()
	select {
	case r := <-ch:
		t.Fatalf("chan should block but recieved: %v", r)
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
		t.Fatalf("no response recieved after 50ms")
	}
}

// assertResponse is a helper to test a envoy.DiscoveryResponse matches the
// JSON representaion we expect. We use JSON because the responses use protobuf
// Any type which includes binary protobuf encoding and would make creating
// expected structs require the same code that is under test!
func assertResponse(t *testing.T, r *envoy.DiscoveryResponse, wantJSON string) {
	t.Helper()
	m := jsonpb.Marshaler{
		Indent: "  ",
	}
	gotJSON, err := m.MarshalToString(r)
	require.NoError(t, err)
	require.JSONEqf(t, wantJSON, gotJSON, "got:\n%s", gotJSON)
}

func TestServer_StreamAggregatedResources_ACLEnforcment(t *testing.T) {

	tests := []struct {
		name        string
		defaultDeny bool
		acl         string
		token       string
		wantDenied  bool
	}{
		// Note that although we've stubbed actual ACL checks in the testManager
		// ConnectAuthorize mock, by asserting against specifc reason strings here
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
			name:        "default deny, service:write token",
			defaultDeny: true,
			acl:         `service "web" { policy = "write" }`,
			token:       "service-write-on-web",
			wantDenied:  false,
		},
		{
			name:        "default deny, service:read token",
			defaultDeny: true,
			acl:         `service "web" { policy = "read" }`,
			token:       "service-write-on-web",
			wantDenied:  true,
		},
		{
			name:        "default deny, service:write token on different service",
			defaultDeny: true,
			acl:         `service "not-web" { policy = "write" }`,
			token:       "service-write-on-not-web",
			wantDenied:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := log.New(os.Stderr, "", log.LstdFlags)
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
				policy, err := acl.NewPolicyFromSource("", 0, tt.acl, acl.SyntaxLegacy, nil)
				require.NoError(t, err)
				return acl.NewPolicyAuthorizer(acl.RootAuthorizer("deny"), []*acl.Policy{policy}, nil)
			}
			envoy := NewTestEnvoy(t, "web-sidecar-proxy", tt.token)
			defer envoy.Close()

			s := Server{logger, mgr, mgr, aclResolve}

			errCh := make(chan error, 1)
			go func() {
				errCh <- s.StreamAggregatedResources(envoy.stream)
			}()

			// Register the proxy to create state needed to Watch() on
			mgr.RegisterProxy(t, "web-sidecar-proxy")

			// Deliver a new snapshot
			snap := proxycfg.TestConfigSnapshot(t)
			mgr.DeliverConfig(t, "web-sidecar-proxy", snap)

			// Send initial listener discover, in real life Envoy always sends cluster
			// first but it doesn't really matter and listener has a response that
			// includes the token in the ext authz filter so lets us test more stuff.
			envoy.SendReq(t, ListenerType, 0, 0)

			if !tt.wantDenied {
				assertResponseSent(t, envoy.stream.sendCh, expectListenerJSON(t, snap, tt.token, 1, 1))
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
					mgr.AssertWatchCancelled(t, "web-sidecar-proxy")
				} else {
					require.NoError(t, err)
				}
			case <-time.After(50 * time.Millisecond):
				t.Fatalf("timed out waiting for handler to finish")
			}
		})
	}
}

// This tests the ext_authz service method that implements connect authz.
func TestServer_Check(t *testing.T) {

	tests := []struct {
		name            string
		source          string
		dest            string
		sourcePrincipal string
		destPrincipal   string
		authzResult     connectAuthzResult
		wantErr         bool
		wantErrCode     codes.Code
		wantDenied      bool
		wantReason      string
	}{
		{
			name:        "auth allowed",
			source:      "web",
			dest:        "db",
			authzResult: connectAuthzResult{true, "default allow", nil, nil},
			wantDenied:  false,
			wantReason:  "default allow",
		},
		{
			name:        "auth denied",
			source:      "web",
			dest:        "db",
			authzResult: connectAuthzResult{false, "default deny", nil, nil},
			wantDenied:  true,
			wantReason:  "default deny",
		},
		{
			name:            "no source",
			sourcePrincipal: "",
			dest:            "db",
			// Should never make it to authz call.
			wantErr:     true,
			wantErrCode: codes.InvalidArgument,
		},
		{
			name:   "no dest",
			source: "web",
			dest:   "",
			// Should never make it to authz call.
			wantErr:     true,
			wantErrCode: codes.InvalidArgument,
		},
		{
			name:          "dest invalid format",
			source:        "web",
			destPrincipal: "not-a-spiffe-id",
			// Should never make it to authz call.
			wantDenied: true,
			wantReason: "Destination Principal is not a valid Connect identitiy",
		},
		{
			name:          "dest not a service URI",
			source:        "web",
			destPrincipal: "spiffe://trust-domain.consul",
			// Should never make it to authz call.
			wantDenied: true,
			wantReason: "Destination Principal is not a valid Service identitiy",
		},
		{
			name:        "ACL not got permission for authz call",
			source:      "web",
			dest:        "db",
			authzResult: connectAuthzResult{false, "", nil, acl.ErrPermissionDenied},
			wantErr:     true,
			wantErrCode: codes.PermissionDenied,
		},
		{
			name:        "Random error running authz",
			source:      "web",
			dest:        "db",
			authzResult: connectAuthzResult{false, "", nil, errors.New("gremlin attack")},
			wantErr:     true,
			wantErrCode: codes.Internal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := "my-real-acl-token"
			logger := log.New(os.Stderr, "", log.LstdFlags)
			mgr := newTestManager(t)

			// Setup expected auth result against that token no lock as no other
			// goroutine is touching this yet.
			mgr.authz[token] = tt.authzResult

			aclResolve := func(id string) (acl.Authorizer, error) {
				return nil, nil
			}
			envoy := NewTestEnvoy(t, "web-sidecar-proxy", token)
			defer envoy.Close()

			s := Server{logger, mgr, mgr, aclResolve}

			// Create a context with the correct token
			ctx := metadata.NewIncomingContext(context.Background(),
				metadata.Pairs("x-consul-token", token))

			r := TestCheckRequest(t, tt.source, tt.dest)
			// If sourcePrincipal is set override, or if source is also not set
			// explicitly override to empty.
			if tt.sourcePrincipal != "" || tt.source == "" {
				r.Attributes.Source.Principal = tt.sourcePrincipal
			}
			if tt.destPrincipal != "" || tt.dest == "" {
				r.Attributes.Destination.Principal = tt.destPrincipal
			}
			resp, err := s.Check(ctx, r)
			// Denied is not an error
			if tt.wantErr {
				require.Error(t, err)
				grpcStatus := status.Convert(err)
				require.Equal(t, tt.wantErrCode, grpcStatus.Code())
				require.Nil(t, resp)
				return
			}
			require.NoError(t, err)
			if tt.wantDenied {
				require.Equal(t, int32(codes.PermissionDenied), resp.Status.Code)
			} else {
				require.Equal(t, int32(codes.OK), resp.Status.Code)
			}
			require.Contains(t, resp.Status.Message, tt.wantReason)
		})
	}
}

func TestServer_ConfigOverrides(t *testing.T) {

	tests := []struct {
		name  string
		setup func(snap *proxycfg.ConfigSnapshot) string
	}{
		{
			name: "sanity check no custom",
			setup: func(snap *proxycfg.ConfigSnapshot) string {
				// Default snap and expectation
				return expectListenerJSON(t, snap, "my-token", 1, 1)
			},
		},
		{
			name: "custom public_listener no type",
			setup: func(snap *proxycfg.ConfigSnapshot) string {
				snap.Proxy.Config["envoy_public_listener_json"] =
					customListenerJSON(t, customListenerJSONOptions{
						Name:        "custom-public-listen",
						IncludeType: false,
					})
				resources := expectListenerJSONResources(t, snap, "my-token", 1, 1)

				// Replace the public listener with the custom one WITH type since
				// that's how it comes out the other end, and with TLS and authz
				// overridden.
				resources["public_listener"] = customListenerJSON(t, customListenerJSONOptions{
					Name: "custom-public-listen",
					// We should add type, TLS and authz
					IncludeType:   true,
					OverrideAuthz: true,
					TLSContext:    expectedPublicTLSContextJSON(t, snap),
				})
				return expectListenerJSONFromResources(t, snap, "my-token", 1, 1, resources)
			},
		},
		{
			name: "custom public_listener with type",
			setup: func(snap *proxycfg.ConfigSnapshot) string {
				snap.Proxy.Config["envoy_public_listener_json"] =
					customListenerJSON(t, customListenerJSONOptions{
						Name:        "custom-public-listen",
						IncludeType: true,
					})
				resources := expectListenerJSONResources(t, snap, "my-token", 1, 1)

				// Replace the public listener with the custom one WITH type since
				// that's how it comes out the other end, and with TLS and authz
				// overridden.
				resources["public_listener"] = customListenerJSON(t, customListenerJSONOptions{
					Name: "custom-public-listen",
					// We should add type, TLS and authz
					IncludeType:   true,
					OverrideAuthz: true,
					TLSContext:    expectedPublicTLSContextJSON(t, snap),
				})
				return expectListenerJSONFromResources(t, snap, "my-token", 1, 1, resources)
			},
		},
		{
			name: "custom public_listener with TLS should be overridden",
			setup: func(snap *proxycfg.ConfigSnapshot) string {
				snap.Proxy.Config["envoy_public_listener_json"] =
					customListenerJSON(t, customListenerJSONOptions{
						Name:        "custom-public-listen",
						IncludeType: true,
						TLSContext:  `{"requireClientCertificate": false}`,
					})
				resources := expectListenerJSONResources(t, snap, "my-token", 1, 1)

				// Replace the public listener with the custom one WITH type since
				// that's how it comes out the other end, and with TLS and authz
				// overridden.
				resources["public_listener"] = customListenerJSON(t, customListenerJSONOptions{
					Name: "custom-public-listen",
					// We should add type, TLS and authz
					IncludeType:   true,
					OverrideAuthz: true,
					TLSContext:    expectedPublicTLSContextJSON(t, snap),
				})
				return expectListenerJSONFromResources(t, snap, "my-token", 1, 1, resources)
			},
		},
		{
			name: "custom upstream no type",
			setup: func(snap *proxycfg.ConfigSnapshot) string {
				snap.Proxy.Upstreams[0].Config["envoy_listener_json"] =
					customListenerJSON(t, customListenerJSONOptions{
						Name:        "custom-upstream",
						IncludeType: false,
					})
				resources := expectListenerJSONResources(t, snap, "my-token", 1, 1)

				// Replace an upstream listener with the custom one WITH type since
				// that's how it comes out the other end. Note we do override TLS
				resources["service:db"] =
					customListenerJSON(t, customListenerJSONOptions{
						Name: "custom-upstream",
						// We should add type
						IncludeType: true,
					})
				return expectListenerJSONFromResources(t, snap, "my-token", 1, 1, resources)
			},
		},
		{
			name: "custom upstream with type",
			setup: func(snap *proxycfg.ConfigSnapshot) string {
				snap.Proxy.Upstreams[0].Config["envoy_listener_json"] =
					customListenerJSON(t, customListenerJSONOptions{
						Name:        "custom-upstream",
						IncludeType: true,
					})
				resources := expectListenerJSONResources(t, snap, "my-token", 1, 1)

				// Replace an upstream listener with the custom one WITH type since
				// that's how it comes out the other end.
				resources["service:db"] =
					customListenerJSON(t, customListenerJSONOptions{
						Name: "custom-upstream",
						// We should add type
						IncludeType: true,
					})
				return expectListenerJSONFromResources(t, snap, "my-token", 1, 1, resources)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			// Sanity check default with no overrides first
			snap := proxycfg.TestConfigSnapshot(t)
			expect := tt.setup(snap)

			listeners, err := listenersFromSnapshot(snap, "my-token")
			require.NoError(err)
			r, err := createResponse(ListenerType, "00000001", "00000001", listeners)
			require.NoError(err)

			assertResponse(t, r, expect)
		})
	}
}

type customListenerJSONOptions struct {
	Name          string
	IncludeType   bool
	OverrideAuthz bool
	TLSContext    string
}

const customListenerJSONTpl = `{
	{{ if .IncludeType -}}
	"@type": "type.googleapis.com/envoy.api.v2.Listener",
	{{- end }}
	"name": "{{ .Name }}",
	"address": {
		"socketAddress": {
			"address": "11.11.11.11",
			"portValue": 11111
		}
	},
	"filterChains": [
		{
			{{ if .TLSContext -}}
			"tlsContext": {{ .TLSContext }},
			{{- end }}
			"filters": [
				{{ if .OverrideAuthz -}}
				{
					"name": "envoy.ext_authz",
					"config": {
							"grpc_service": {
										"envoy_grpc": {
													"cluster_name": "local_agent"
												},
										"initial_metadata": [
													{
																"key": "x-consul-token",
																"value": "my-token"
															}
												]
									},
							"stat_prefix": "connect_authz"
						}
				},
				{{- end }}
				{
					"name": "envoy.tcp_proxy",
					"config": {
							"cluster": "random-cluster",
							"stat_prefix": "foo-stats"
						}
				}
			]
		}
	]
}`

var customListenerJSONTemplate = template.Must(template.New("").Parse(customListenerJSONTpl))

func customListenerJSON(t *testing.T, opts customListenerJSONOptions) string {
	t.Helper()
	var buf bytes.Buffer
	err := customListenerJSONTemplate.Execute(&buf, opts)
	require.NoError(t, err)
	return buf.String()
}
