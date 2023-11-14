// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	gogrpc "google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/pool"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/private/pbpeering"
	"github.com/hashicorp/consul/proto/private/pbpeerstream"
	"github.com/hashicorp/consul/sdk/freeport"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/types"
)

func TestPeeringBackend_ForwardToLeader(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	ca := connect.TestCA(t, nil)
	_, server1 := testServerWithConfig(t, func(c *Config) {
		c.GRPCTLSPort = freeport.GetOne(t)
		c.CAConfig = &structs.CAConfiguration{
			ClusterID: connect.TestClusterID,
			Provider:  structs.ConsulCAProvider,
			Config: map[string]interface{}{
				"PrivateKey": ca.SigningKey,
				"RootCert":   ca.RootCert,
			},
		}
	})
	_, server2 := testServerWithConfig(t, func(c *Config) {
		c.Bootstrap = false
	})

	// Join a 2nd server (not the leader)
	testrpc.WaitForLeader(t, server1.RPC, "dc1")
	testrpc.WaitForActiveCARoot(t, server1.RPC, "dc1", nil)

	joinLAN(t, server2, server1)
	testrpc.WaitForLeader(t, server2.RPC, "dc1")

	// Make a write call to server2 and make sure it gets forwarded to server1
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	// Dial server2 directly
	conn, err := gogrpc.DialContext(ctx, server2.config.RPCAddr.String(),
		gogrpc.WithContextDialer(newServerDialer(server2.config.RPCAddr.String())),
		//nolint:staticcheck
		gogrpc.WithInsecure(),
		gogrpc.WithBlock())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	peeringClient := pbpeering.NewPeeringServiceClient(conn)

	testutil.RunStep(t, "forward a write", func(t *testing.T) {
		// Do the grpc Write call to server2
		req := pbpeering.GenerateTokenRequest{
			PeerName: "foo",
		}
		_, err := peeringClient.GenerateToken(ctx, &req)
		require.NoError(t, err)

		// TODO(peering) check that state store is updated on leader, indicating a forwarded request after state store
		// is implemented.
	})
}

func TestPeeringBackend_GetLocalServerAddresses(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	_, cfg := testServerConfig(t)
	cfg.GRPCTLSPort = freeport.GetOne(t)

	srv, err := newServer(t, cfg)
	require.NoError(t, err)
	testrpc.WaitForLeader(t, srv.RPC, "dc1")

	backend := NewPeeringBackend(srv)

	testutil.RunStep(t, "peer to servers", func(t *testing.T) {
		addrs, err := backend.GetLocalServerAddresses()
		require.NoError(t, err)

		expect := fmt.Sprintf("127.0.0.1:%d", srv.config.GRPCTLSPort)
		require.Equal(t, []string{expect}, addrs)
	})

	testutil.RunStep(t, "prefer WAN address for servers", func(t *testing.T) {
		req := structs.RegisterRequest{
			Datacenter:     cfg.Datacenter,
			Node:           cfg.NodeName,
			ID:             cfg.NodeID,
			Address:        "127.0.0.1",
			EnterpriseMeta: *acl.DefaultEnterpriseMeta(),

			// Add a tagged WAN address to the server registration
			TaggedAddresses: map[string]string{
				structs.TaggedAddressWAN: "3.4.5.6",
			},
		}
		require.NoError(t, srv.fsm.State().EnsureRegistration(200, &req))

		addrs, err := backend.GetLocalServerAddresses()
		require.NoError(t, err)

		expect := fmt.Sprintf("3.4.5.6:%d", srv.config.GRPCTLSPort)
		require.Equal(t, []string{expect}, addrs)
	})

	testutil.RunStep(t, "existence of mesh config entry is not enough to peer through gateways", func(t *testing.T) {
		mesh := structs.MeshConfigEntry{
			// Enable unrelated config.
			TransparentProxy: structs.TransparentProxyMeshConfig{
				MeshDestinationsOnly: true,
			},
		}

		require.NoError(t, srv.fsm.State().EnsureConfigEntry(1, &mesh))
		addrs, err := backend.GetLocalServerAddresses()
		require.NoError(t, err)

		// Still expect server address because PeerThroughMeshGateways was not enabled.
		expect := fmt.Sprintf("3.4.5.6:%d", srv.config.GRPCTLSPort)
		require.Equal(t, []string{expect}, addrs)
	})

	testutil.RunStep(t, "cannot peer through gateways without registered gateways", func(t *testing.T) {
		mesh := structs.MeshConfigEntry{
			Peering: &structs.PeeringMeshConfig{PeerThroughMeshGateways: true},
		}
		require.NoError(t, srv.fsm.State().EnsureConfigEntry(1, &mesh))

		addrs, err := backend.GetLocalServerAddresses()
		require.Nil(t, addrs)
		testutil.RequireErrorContains(t, err,
			"servers are configured to PeerThroughMeshGateways, but no mesh gateway instances are registered")
	})

	testutil.RunStep(t, "peer through mesh gateways", func(t *testing.T) {
		reg := structs.RegisterRequest{
			ID:      types.NodeID("b5489ca9-f5e9-4dba-a779-61fec4e8e364"),
			Node:    "gw-node",
			Address: "1.2.3.4",
			TaggedAddresses: map[string]string{
				structs.TaggedAddressWAN: "172.217.22.14",
			},
			Service: &structs.NodeService{
				ID:      "mesh-gateway",
				Service: "mesh-gateway",
				Kind:    structs.ServiceKindMeshGateway,
				Port:    443,
				TaggedAddresses: map[string]structs.ServiceAddress{
					structs.TaggedAddressWAN: {Address: "154.238.12.252", Port: 8443},
				},
			},
		}
		require.NoError(t, srv.fsm.State().EnsureRegistration(2, &reg))

		addrs, err := backend.GetLocalServerAddresses()
		require.NoError(t, err)
		require.Equal(t, []string{"154.238.12.252:8443"}, addrs)
	})
}

func TestPeeringBackend_GetDialAddresses(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	_, cfg := testServerConfig(t)
	cfg.GRPCTLSPort = freeport.GetOne(t)

	srv, err := newServer(t, cfg)
	require.NoError(t, err)
	testrpc.WaitForLeader(t, srv.RPC, "dc1")

	backend := NewPeeringBackend(srv)

	dialerPeerID := testUUID()
	acceptorPeerID := testUUID()

	type expectation struct {
		addrs        []string
		gatewayAddrs []string
		err          string
	}

	type testCase struct {
		name   string
		setup  func(store *state.Store)
		peerID string
		expect expectation
	}

	run := func(t *testing.T, tc testCase) {
		if tc.setup != nil {
			tc.setup(srv.fsm.State())
		}

		ring, gatewayRing, err := backend.GetDialAddresses(testutil.Logger(t), nil, tc.peerID)
		if tc.expect.err != "" {
			testutil.RequireErrorContains(t, err, tc.expect.err)
			return
		}
		require.Equal(t, len(tc.expect.gatewayAddrs) > 0, gatewayRing != nil)
		require.NotNil(t, ring)

		if len(tc.expect.gatewayAddrs) > 0 {
			var addrs []string
			gatewayRing.Do(func(value any) {
				addr, ok := value.(string)

				require.True(t, ok)
				addrs = append(addrs, addr)
			})
			require.Equal(t, tc.expect.gatewayAddrs, addrs)
		}

		var addrs []string
		ring.Do(func(value any) {
			addr, ok := value.(string)

			require.True(t, ok)
			addrs = append(addrs, addr)
		})
		require.Equal(t, tc.expect.addrs, addrs)
	}

	// NOTE: The following tests are set up to run serially with RunStep to save on the setup/teardown cost for a test server.
	tt := []testCase{
		{
			name: "unknown peering",
			setup: func(store *state.Store) {
				// Test peering is not written during setup
			},
			peerID: acceptorPeerID,
			expect: expectation{
				err: fmt.Sprintf(`unknown peering %q`, acceptorPeerID),
			},
		},
		{
			name: "no server addresses",
			setup: func(store *state.Store) {
				require.NoError(t, store.PeeringWrite(1, &pbpeering.PeeringWriteRequest{
					Peering: &pbpeering.Peering{
						Name: "acceptor",
						ID:   acceptorPeerID,
						// Acceptor peers do not have PeerServerAddresses populated locally.
					},
				}))
			},
			peerID: acceptorPeerID,
			expect: expectation{
				err: "no known addresses",
			},
		},
		{
			name: "manual server addrs are returned when defined",
			setup: func(store *state.Store) {
				require.NoError(t, store.PeeringWrite(2, &pbpeering.PeeringWriteRequest{
					Peering: &pbpeering.Peering{
						Name:                  "dialer",
						ID:                    dialerPeerID,
						ManualServerAddresses: []string{"5.6.7.8:8502"},
						PeerServerAddresses:   []string{"1.2.3.4:8502", "2.3.4.5:8503"},
					},
				}))
				// Mesh config entry does not exist
			},
			peerID: dialerPeerID,
			expect: expectation{
				addrs: []string{"5.6.7.8:8502"},
			},
		},
		{
			name: "only server addrs are returned when mesh config does not exist",
			setup: func(store *state.Store) {
				require.NoError(t, store.PeeringWrite(2, &pbpeering.PeeringWriteRequest{
					Peering: &pbpeering.Peering{
						Name:                "dialer",
						ID:                  dialerPeerID,
						PeerServerAddresses: []string{"1.2.3.4:8502", "2.3.4.5:8503"},
					},
				}))

				// Mesh config entry does not exist
			},
			peerID: dialerPeerID,
			expect: expectation{
				addrs: []string{"1.2.3.4:8502", "2.3.4.5:8503"},
			},
		},
		{
			name: "only server addrs are returned when not peering through gateways",
			setup: func(store *state.Store) {
				require.NoError(t, srv.fsm.State().EnsureConfigEntry(3, &structs.MeshConfigEntry{
					Peering: &structs.PeeringMeshConfig{
						PeerThroughMeshGateways: false, // Peering through gateways is not enabled
					},
				}))
			},
			peerID: dialerPeerID,
			expect: expectation{
				addrs: []string{"1.2.3.4:8502", "2.3.4.5:8503"},
			},
		},
		{
			name: "only server addrs are returned when peering through gateways without gateways registered",
			setup: func(store *state.Store) {
				require.NoError(t, srv.fsm.State().EnsureConfigEntry(4, &structs.MeshConfigEntry{
					Peering: &structs.PeeringMeshConfig{
						PeerThroughMeshGateways: true,
					},
				}))

				// No gateways are registered
			},
			peerID: dialerPeerID,
			expect: expectation{
				// Fall back to remote server addresses
				addrs: []string{"1.2.3.4:8502", "2.3.4.5:8503"},
			},
		},
		{
			name: "gateway addresses are included after gateways are registered",
			setup: func(store *state.Store) {
				require.NoError(t, srv.fsm.State().EnsureRegistration(5, &structs.RegisterRequest{
					ID:      types.NodeID(testUUID()),
					Node:    "gateway-node-1",
					Address: "5.6.7.8",
					Service: &structs.NodeService{
						Kind:    structs.ServiceKindMeshGateway,
						ID:      "mesh-gateway-1",
						Service: "mesh-gateway",
						Port:    8443,
						TaggedAddresses: map[string]structs.ServiceAddress{
							structs.TaggedAddressWAN: {
								Address: "my-lb-addr.not-aws.com",
								Port:    443,
							},
						},
					},
				}))
				require.NoError(t, srv.fsm.State().EnsureRegistration(6, &structs.RegisterRequest{
					ID:      types.NodeID(testUUID()),
					Node:    "gateway-node-2",
					Address: "6.7.8.9",
					Service: &structs.NodeService{
						Kind:    structs.ServiceKindMeshGateway,
						ID:      "mesh-gateway-2",
						Service: "mesh-gateway",
						Port:    8443,
						TaggedAddresses: map[string]structs.ServiceAddress{
							structs.TaggedAddressWAN: {
								Address: "my-other-lb-addr.not-aws.com",
								Port:    443,
							},
						},
					},
				}))
			},
			peerID: dialerPeerID,
			expect: expectation{
				// Gateways come first, and we use their LAN addresses since this is for outbound communication.
				addrs:        []string{"5.6.7.8:8443", "6.7.8.9:8443", "1.2.3.4:8502", "2.3.4.5:8503"},
				gatewayAddrs: []string{"5.6.7.8:8443", "6.7.8.9:8443"},
			},
		},
		{
			name: "addresses are returned if the peering is marked as terminated",
			setup: func(store *state.Store) {
				require.NoError(t, store.PeeringWrite(5, &pbpeering.PeeringWriteRequest{
					Peering: &pbpeering.Peering{
						Name:                "dialer",
						ID:                  dialerPeerID,
						PeerServerAddresses: []string{"1.2.3.4:8502", "2.3.4.5:8503"},
						State:               pbpeering.PeeringState_TERMINATED,
					},
				}))
			},
			peerID: dialerPeerID,
			expect: expectation{
				// Gateways come first, and we use their LAN addresses since this is for outbound communication.
				addrs:        []string{"5.6.7.8:8443", "6.7.8.9:8443", "1.2.3.4:8502", "2.3.4.5:8503"},
				gatewayAddrs: []string{"5.6.7.8:8443", "6.7.8.9:8443"},
			},
		},
		{
			name: "addresses are not returned if the peering is deleted",
			setup: func(store *state.Store) {
				require.NoError(t, store.PeeringWrite(5, &pbpeering.PeeringWriteRequest{
					Peering: &pbpeering.Peering{
						Name:                "dialer",
						ID:                  dialerPeerID,
						PeerServerAddresses: []string{"1.2.3.4:8502", "2.3.4.5:8503"},

						// Mark as deleted
						State:     pbpeering.PeeringState_DELETING,
						DeletedAt: timestamppb.New(time.Now()),
					},
				}))
			},
			peerID: dialerPeerID,
			expect: expectation{
				err: fmt.Sprintf(`peering %q was deleted`, dialerPeerID),
			},
		},
	}

	for _, tc := range tt {
		testutil.RunStep(t, tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func newServerDialer(serverAddr string) func(context.Context, string) (net.Conn, error) {
	return func(ctx context.Context, addr string) (net.Conn, error) {
		d := net.Dialer{}
		conn, err := d.DialContext(ctx, "tcp", serverAddr)
		if err != nil {
			return nil, err
		}

		_, err = conn.Write([]byte{byte(pool.RPCGRPC)})
		if err != nil {
			conn.Close()
			return nil, err
		}

		return conn, nil
	}
}

func TestPeerStreamService_ForwardToLeader(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	ca := connect.TestCA(t, nil)
	_, server1 := testServerWithConfig(t, func(c *Config) {
		c.GRPCTLSPort = freeport.GetOne(t)
		c.CAConfig = &structs.CAConfiguration{
			ClusterID: connect.TestClusterID,
			Provider:  structs.ConsulCAProvider,
			Config: map[string]interface{}{
				"PrivateKey": ca.SigningKey,
				"RootCert":   ca.RootCert,
			},
		}
	})
	_, server2 := testServerWithConfig(t, func(c *Config) {
		c.Bootstrap = false
	})

	// server1 is leader, server2 follower
	testrpc.WaitForLeader(t, server1.RPC, "dc1")
	testrpc.WaitForActiveCARoot(t, server1.RPC, "dc1", nil)

	joinLAN(t, server2, server1)
	testrpc.WaitForLeader(t, server2.RPC, "dc1")

	peerId := testUUID()

	// Simulate a GenerateToken call on server1, which stores the establishment secret
	{
		require.NoError(t, server1.FSM().State().PeeringWrite(10, &pbpeering.PeeringWriteRequest{
			Peering: &pbpeering.Peering{
				Name: "foo",
				ID:   peerId,
			},
			SecretsRequest: &pbpeering.SecretsWriteRequest{
				PeerID: peerId,
				Request: &pbpeering.SecretsWriteRequest_GenerateToken{
					GenerateToken: &pbpeering.SecretsWriteRequest_GenerateTokenRequest{
						EstablishmentSecret: "389bbcdf-1c31-47d6-ae96-f2a3f4c45f84",
					},
				},
			},
		}))
	}

	testutil.RunStep(t, "server2 forwards write to server1", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		t.Cleanup(cancel)

		// We will dial server2 which should forward to server1
		conn, err := gogrpc.DialContext(ctx, server2.config.RPCAddr.String(),
			gogrpc.WithContextDialer(newServerDialer(server2.config.RPCAddr.String())),
			//nolint:staticcheck
			gogrpc.WithInsecure(),
			gogrpc.WithBlock())
		require.NoError(t, err)
		t.Cleanup(func() { conn.Close() })

		peerStreamClient := pbpeerstream.NewPeerStreamServiceClient(conn)
		req := &pbpeerstream.ExchangeSecretRequest{
			PeerID:              peerId,
			EstablishmentSecret: "389bbcdf-1c31-47d6-ae96-f2a3f4c45f84",
		}
		_, err = peerStreamClient.ExchangeSecret(ctx, req)
		require.NoError(t, err)
	})
}
