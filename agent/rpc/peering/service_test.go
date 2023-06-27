// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package peering_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/google/tcpproxy"
	"github.com/stretchr/testify/require"
	gogrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-uuid"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/consul/rate"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	external "github.com/hashicorp/consul/agent/grpc-external"
	"github.com/hashicorp/consul/agent/grpc-external/limiter"
	grpc "github.com/hashicorp/consul/agent/grpc-internal"
	"github.com/hashicorp/consul/agent/grpc-internal/balancer"
	"github.com/hashicorp/consul/agent/grpc-internal/resolver"
	agentmiddleware "github.com/hashicorp/consul/agent/grpc-middleware"
	"github.com/hashicorp/consul/agent/pool"
	"github.com/hashicorp/consul/agent/router"
	"github.com/hashicorp/consul/agent/rpc/middleware"
	"github.com/hashicorp/consul/agent/rpc/peering"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/proto/private/pbpeering"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/freeport"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/tlsutil"
	"github.com/hashicorp/consul/types"
)

const (
	testTokenPeeringReadSecret  = "9a83c138-a0c7-40f1-89fa-6acf9acd78f5"
	testTokenPeeringWriteSecret = "91f90a41-0840-4afe-b615-68745f9e16c1"
	testTokenServiceReadSecret  = "1ef8e3cf-6e95-49aa-9f73-a0d3ad1a77d4"
	testTokenServiceWriteSecret = "4a3dc05d-d86c-4f20-be43-8f4f8f045fea"
)

func generateTooManyMetaKeys() map[string]string {
	// todo -- modularize in structs.go or testing.go
	tooMuchMeta := make(map[string]string)
	for i := 0; i < 64+1; i++ {
		tooMuchMeta[fmt.Sprint(i)] = "value"
	}

	return tooMuchMeta
}

func TestPeeringService_GenerateToken(t *testing.T) {
	dir := testutil.TempDir(t, "consul")

	signer, _, _ := tlsutil.GeneratePrivateKey()
	ca, _, _ := tlsutil.GenerateCA(tlsutil.CAOpts{Signer: signer})
	cafile := path.Join(dir, "cacert.pem")
	require.NoError(t, os.WriteFile(cafile, []byte(ca), 0600))

	// TODO(peering): see note on newTestServer, refactor to not use this
	s := newTestServer(t, func(c *consul.Config) {
		c.SerfLANConfig.MemberlistConfig.AdvertiseAddr = "127.0.0.1"
		c.TLSConfig.GRPC.CAFile = cafile
		c.DataDir = dir
	})
	client := pbpeering.NewPeeringServiceClient(s.ClientConn(t))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	// TODO(peering): for more failure cases, consider using a table test
	// check meta tags
	reqE := pbpeering.GenerateTokenRequest{PeerName: "peer-b", Meta: generateTooManyMetaKeys()}
	_, errE := client.GenerateToken(ctx, &reqE)
	require.EqualError(t, errE, "rpc error: code = Unknown desc = meta tags failed validation: Node metadata cannot contain more than 64 key/value pairs")

	var (
		peerID string
		secret string
	)
	testutil.RunStep(t, "peering token is generated with data", func(t *testing.T) {
		req := pbpeering.GenerateTokenRequest{
			PeerName: "peer-b",
			Meta:     map[string]string{"foo": "bar"},
		}
		resp, err := client.GenerateToken(ctx, &req)
		require.NoError(t, err)

		tokenJSON, err := base64.StdEncoding.DecodeString(resp.PeeringToken)
		require.NoError(t, err)

		token := &structs.PeeringToken{}
		require.NoError(t, json.Unmarshal(tokenJSON, token))
		require.Equal(t, "server.dc1.peering.11111111-2222-3333-4444-555555555555.consul", token.ServerName)
		require.Len(t, token.ServerAddresses, 1)
		require.Equal(t, s.PublicGRPCAddr, token.ServerAddresses[0])

		// The roots utilized should be the ConnectCA roots and not the ones manually configured.
		_, roots, err := s.Server.FSM().State().CARoots(nil)
		require.NoError(t, err)
		require.Equal(t, []string{roots.Active().RootCert}, token.CA)
		require.Equal(t, "dc1", token.Remote.Datacenter)

		require.NotEmpty(t, token.EstablishmentSecret)
		secret = token.EstablishmentSecret

		require.NotEmpty(t, token.PeerID)
		peerID = token.PeerID

		_, err = uuid.ParseUUID(token.PeerID)
		require.NoError(t, err)
	})

	testutil.RunStep(t, "peerings is created by generating a token", func(t *testing.T) {
		_, peers, err := s.Server.FSM().State().PeeringList(nil, *structs.DefaultEnterpriseMetaInDefaultPartition())
		require.NoError(t, err)
		require.Len(t, peers, 1)

		peers[0].ModifyIndex = 0
		peers[0].CreateIndex = 0

		expect := &pbpeering.Peering{
			Name:      "peer-b",
			Partition: acl.DefaultPartitionName,
			ID:        peerID,
			State:     pbpeering.PeeringState_PENDING,
			Meta:      map[string]string{"foo": "bar"},
		}
		require.Equal(t, expect, peers[0])
	})

	testutil.RunStep(t, "generating a token persists establishment secret", func(t *testing.T) {
		s, err := s.Server.FSM().State().PeeringSecretsRead(nil, peerID)
		require.NoError(t, err)
		require.NotNil(t, s)

		require.Equal(t, secret, s.GetEstablishment().GetSecretID())
	})

	testutil.RunStep(t, "re-generating a peering token re-generates the secret", func(t *testing.T) {
		req := pbpeering.GenerateTokenRequest{PeerName: "peer-b", Meta: map[string]string{"foo": "bar"}}
		resp, err := client.GenerateToken(ctx, &req)
		require.NoError(t, err)

		tokenJSON, err := base64.StdEncoding.DecodeString(resp.PeeringToken)
		require.NoError(t, err)

		token := &structs.PeeringToken{}
		require.NoError(t, json.Unmarshal(tokenJSON, token))

		// There should be a new establishment secret, different from the past one
		require.NotEmpty(t, token.EstablishmentSecret)
		require.NotEqual(t, secret, token.EstablishmentSecret)

		s, err := s.Server.FSM().State().PeeringSecretsRead(nil, peerID)
		require.NoError(t, err)
		require.NotNil(t, s)

		// The secret must be persisted on the server that generated it.
		require.Equal(t, token.EstablishmentSecret, s.GetEstablishment().GetSecretID())
	})

}

func TestPeeringService_GenerateTokenExternalAddress(t *testing.T) {
	dir := testutil.TempDir(t, "consul")

	signer, _, _ := tlsutil.GeneratePrivateKey()
	ca, _, _ := tlsutil.GenerateCA(tlsutil.CAOpts{Signer: signer})
	cafile := path.Join(dir, "cacert.pem")
	require.NoError(t, os.WriteFile(cafile, []byte(ca), 0600))

	// TODO(peering): see note on newTestServer, refactor to not use this
	s := newTestServer(t, func(c *consul.Config) {
		c.SerfLANConfig.MemberlistConfig.AdvertiseAddr = "127.0.0.1"
		c.TLSConfig.GRPC.CAFile = cafile
		c.DataDir = dir
	})
	client := pbpeering.NewPeeringServiceClient(s.ClientConn(t))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	externalAddresses := []string{"32.1.2.3:8502"}
	// happy path
	req := pbpeering.GenerateTokenRequest{PeerName: "peer-b", Meta: map[string]string{"foo": "bar"}, ServerExternalAddresses: externalAddresses}
	resp, err := client.GenerateToken(ctx, &req)
	require.NoError(t, err)

	tokenJSON, err := base64.StdEncoding.DecodeString(resp.PeeringToken)
	require.NoError(t, err)

	token := &structs.PeeringToken{}
	require.NoError(t, json.Unmarshal(tokenJSON, token))
	require.Equal(t, "server.dc1.peering.11111111-2222-3333-4444-555555555555.consul", token.ServerName)
	require.Equal(t, externalAddresses, token.ManualServerAddresses)
	require.Equal(t, []string{s.PublicGRPCAddr}, token.ServerAddresses)

	// The roots utilized should be the ConnectCA roots and not the ones manually configured.
	_, roots, err := s.Server.FSM().State().CARoots(nil)
	require.NoError(t, err)
	require.Equal(t, []string{roots.Active().RootCert}, token.CA)
}

func TestPeeringService_GenerateToken_ACLEnforcement(t *testing.T) {
	// TODO(peering): see note on newTestServer, refactor to not use this
	s := newTestServer(t, func(conf *consul.Config) {
		conf.ACLsEnabled = true
		conf.ACLResolverSettings.ACLDefaultPolicy = acl.PolicyDeny
	})
	upsertTestACLs(t, s.Server.FSM().State())

	client := pbpeering.NewPeeringServiceClient(s.ClientConn(t))

	type testcase struct {
		name      string
		req       *pbpeering.GenerateTokenRequest
		token     string
		expectErr string
	}
	run := func(t *testing.T, tc testcase) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		t.Cleanup(cancel)

		options := structs.QueryOptions{Token: tc.token}
		ctx, err := external.ContextWithQueryOptions(ctx, options)
		require.NoError(t, err)
		_, err = client.GenerateToken(ctx, tc.req)
		if tc.expectErr != "" {
			require.Contains(t, err.Error(), tc.expectErr)
			return
		}
		require.NoError(t, err)
	}
	tcs := []testcase{
		{
			name:      "anonymous token lacks permissions",
			req:       &pbpeering.GenerateTokenRequest{PeerName: "foo"},
			expectErr: "lacks permission 'peering:write'",
		},
		{
			name: "read token lacks permissions",
			req: &pbpeering.GenerateTokenRequest{
				PeerName: "foo",
			},
			token:     testTokenPeeringReadSecret,
			expectErr: "lacks permission 'peering:write'",
		},
		{
			name: "write token grants permission",
			req: &pbpeering.GenerateTokenRequest{
				PeerName: "foo",
			},
			token: testTokenPeeringWriteSecret,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestPeeringService_Establish_Validation(t *testing.T) {
	validToken := peering.TestPeeringToken("83474a06-cca4-4ff4-99a4-4152929c8160")
	validTokenJSON, _ := json.Marshal(&validToken)
	validTokenB64 := base64.StdEncoding.EncodeToString(validTokenJSON)

	// TODO(peering): see note on newTestServer, refactor to not use this
	s := newTestServer(t, nil)
	client := pbpeering.NewPeeringServiceClient(s.ClientConn(t))

	type testcase struct {
		name          string
		req           *pbpeering.EstablishRequest
		expectResp    *pbpeering.EstablishResponse
		expectPeering *pbpeering.Peering
		expectErr     string
	}
	run := func(t *testing.T, tc testcase) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		t.Cleanup(cancel)

		resp, err := client.Establish(ctx, tc.req)
		if tc.expectErr != "" {
			require.Contains(t, err.Error(), tc.expectErr)
			return
		}
		require.NoError(t, err)
		prototest.AssertDeepEqual(t, tc.expectResp, resp)

		// if a peering was expected to be written, try to read it back
		if tc.expectPeering != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			t.Cleanup(cancel)

			resp, err := client.PeeringRead(ctx, &pbpeering.PeeringReadRequest{Name: tc.expectPeering.Name})
			require.NoError(t, err)
			// check individual values we care about since we don't know exactly
			// what the create/modify indexes will be
			require.Equal(t, tc.expectPeering.Name, resp.Peering.Name)
			require.Equal(t, tc.expectPeering.Partition, resp.Peering.Partition)
			require.Equal(t, tc.expectPeering.State, resp.Peering.State)
			require.Equal(t, tc.expectPeering.PeerCAPems, resp.Peering.PeerCAPems)
			require.Equal(t, tc.expectPeering.PeerServerAddresses, resp.Peering.PeerServerAddresses)
			require.Equal(t, tc.expectPeering.PeerServerName, resp.Peering.PeerServerName)
		}
	}
	tcs := []testcase{
		{
			name:      "invalid peer name",
			req:       &pbpeering.EstablishRequest{PeerName: "--AA--"},
			expectErr: "--AA-- is not a valid peer name",
		},
		{
			name: "invalid token (base64)",
			req: &pbpeering.EstablishRequest{
				PeerName:     "peer1-usw1",
				PeeringToken: "+++/+++",
			},
			expectErr: "illegal base64 data",
		},
		{
			name: "invalid token (JSON)",
			req: &pbpeering.EstablishRequest{
				PeerName:     "peer1-usw1",
				PeeringToken: "Cg==", // base64 of "-"
			},
			expectErr: "unexpected end of JSON input",
		},
		{
			name: "invalid token (empty)",
			req: &pbpeering.EstablishRequest{
				PeerName:     "peer1-usw1",
				PeeringToken: "e30K", // base64 of "{}"
			},
			expectErr: "peering token server addresses value is empty",
		},
		{
			name: "too many meta tags",
			req: &pbpeering.EstablishRequest{
				PeerName:     "peer1-usw1",
				PeeringToken: validTokenB64,
				Meta:         generateTooManyMetaKeys(),
			},
			expectErr: "meta tags failed validation:",
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

// When tokens have the same name as the dialing cluster, we
// should be throwing an error to note the server name conflict.
func TestPeeringService_Establish_serverNameConflict(t *testing.T) {
	// TODO(peering): see note on newTestServer, refactor to not use this
	s := newTestServer(t, nil)
	client := pbpeering.NewPeeringServiceClient(s.ClientConn(t))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	// Manufacture token to have the same server name but a PeerID not in the store.
	id, err := uuid.GenerateUUID()
	require.NoError(t, err, "could not generate uuid")

	serverName, _, err := s.Server.GetPeeringBackend().GetTLSMaterials(true)
	require.NoError(t, err)

	peeringToken := structs.PeeringToken{
		ServerAddresses:     []string{"1.2.3.4:8502"},
		ServerName:          serverName,
		EstablishmentSecret: "foo",
		PeerID:              id,
	}

	jsonToken, err := json.Marshal(peeringToken)
	require.NoError(t, err, "could not marshal peering token")
	base64Token := base64.StdEncoding.EncodeToString(jsonToken)

	establishReq := &pbpeering.EstablishRequest{
		PeerName:     "peer-two",
		PeeringToken: base64Token,
	}

	respE, errE := client.Establish(ctx, establishReq)
	require.Error(t, errE)
	require.Contains(t, errE.Error(), "cannot create a peering within the same cluster")
	require.Nil(t, respE)
}

func TestPeeringService_Establish(t *testing.T) {
	// TODO(peering): see note on newTestServer, refactor to not use this
	s1 := newTestServer(t, func(conf *consul.Config) {
		conf.NodeName = "s1"
		conf.Datacenter = "test-dc1"
		conf.PrimaryDatacenter = "test-dc1"
	})
	client1 := pbpeering.NewPeeringServiceClient(s1.ClientConn(t))

	s2 := newTestServer(t, func(conf *consul.Config) {
		conf.NodeName = "s2"
		conf.Datacenter = "dc2"
		conf.PrimaryDatacenter = "dc2"
	})
	client2 := pbpeering.NewPeeringServiceClient(s2.ClientConn(t))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	// Generate a peering token for s2
	tokenResp, err := client1.GenerateToken(ctx, &pbpeering.GenerateTokenRequest{PeerName: "my-peer-s2"})
	require.NoError(t, err)

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	var peerID string
	testutil.RunStep(t, "peering can be established from token", func(t *testing.T) {
		retry.Run(t, func(r *retry.R) {
			_, err = client2.Establish(ctx, &pbpeering.EstablishRequest{PeerName: "my-peer-s1", PeeringToken: tokenResp.PeeringToken})
			require.NoError(r, err)
		})

		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		t.Cleanup(cancel)

		// Read the expected peering at s2 to validate it
		resp, err := client2.PeeringRead(ctx, &pbpeering.PeeringReadRequest{Name: "my-peer-s1"})
		require.NoError(t, err)

		peerID = resp.Peering.ID

		// Check individual values, ignoring the create/modify indexes.
		tokenJSON, err := base64.StdEncoding.DecodeString(tokenResp.PeeringToken)
		require.NoError(t, err)

		var token structs.PeeringToken
		require.NoError(t, json.Unmarshal(tokenJSON, &token))

		require.Equal(t, "my-peer-s1", resp.Peering.Name)
		require.Equal(t, token.CA, resp.Peering.PeerCAPems)
		require.Equal(t, token.ServerAddresses, resp.Peering.PeerServerAddresses)
		require.Equal(t, token.ServerName, resp.Peering.PeerServerName)
		require.Equal(t, "test-dc1", token.Remote.Datacenter)
		require.Equal(t, "test-dc1", resp.Peering.Remote.Datacenter)
		require.Equal(t, token.Remote.Partition, resp.Peering.Remote.Partition)
	})

	testutil.RunStep(t, "stream secret is persisted", func(t *testing.T) {
		secret, err := s2.Server.FSM().State().PeeringSecretsRead(nil, peerID)
		require.NoError(t, err)
		require.NotEmpty(t, secret.GetStream().GetActiveSecretID())
	})

	testutil.RunStep(t, "peering tokens cannot be reused after secret exchange", func(t *testing.T) {
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		t.Cleanup(cancel)

		_, err = client2.Establish(ctx, &pbpeering.EstablishRequest{PeerName: "my-peer-s1", PeeringToken: tokenResp.PeeringToken})
		require.Contains(t, err.Error(), "invalid peering establishment secret")
	})
}

func TestPeeringService_Establish_ThroughMeshGateway(t *testing.T) {
	// This test is timing-sensitive, must not be run in parallel.
	// t.Parallel()

	acceptor := newTestServer(t, func(conf *consul.Config) {
		conf.NodeName = "acceptor"
	})
	acceptorClient := pbpeering.NewPeeringServiceClient(acceptor.ClientConn(t))

	dialer := newTestServer(t, func(conf *consul.Config) {
		conf.NodeName = "dialer"
		conf.Datacenter = "dc2"
		conf.PrimaryDatacenter = "dc2"
	})
	dialerClient := pbpeering.NewPeeringServiceClient(dialer.ClientConn(t))

	var peeringToken string

	testutil.RunStep(t, "retry until timeout on dial errors", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		t.Cleanup(cancel)

		testToken := structs.PeeringToken{
			ServerAddresses: []string{fmt.Sprintf("127.0.0.1:%d", freeport.GetOne(t))},
			PeerID:          testUUID(t),
		}
		testTokenJSON, _ := json.Marshal(&testToken)
		testTokenB64 := base64.StdEncoding.EncodeToString(testTokenJSON)

		start := time.Now()
		_, err := dialerClient.Establish(ctx, &pbpeering.EstablishRequest{
			PeerName:     "my-peer-acceptor",
			PeeringToken: testTokenB64,
		})
		require.Error(t, err)
		testutil.RequireErrorContains(t, err, "connection refused")

		require.Greater(t, time.Since(start), 3*time.Second)
	})

	testutil.RunStep(t, "peering can be established from token", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		t.Cleanup(cancel)

		// Generate a peering token for dialer
		tokenResp, err := acceptorClient.GenerateToken(ctx, &pbpeering.GenerateTokenRequest{PeerName: "my-peer-dialer"})
		require.NoError(t, err)

		// Capture peering token for re-use later
		peeringToken = tokenResp.PeeringToken

		// The context timeout is short, it checks that we do not wait the 1s that we do when peering through mesh gateways
		ctx, cancel = context.WithTimeout(context.Background(), 300*time.Millisecond)
		t.Cleanup(cancel)

		_, err = dialerClient.Establish(ctx, &pbpeering.EstablishRequest{
			PeerName:     "my-peer-acceptor",
			PeeringToken: tokenResp.PeeringToken,
		})
		require.NoError(t, err)
	})

	testutil.RunStep(t, "fail fast on permission denied", func(t *testing.T) {
		// This test case re-uses the previous token since the establishment secret will have been invalidated.
		// The context timeout is short, it checks that we do not retry.
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		t.Cleanup(cancel)

		_, err := dialerClient.Establish(ctx, &pbpeering.EstablishRequest{
			PeerName:     "my-peer-acceptor",
			PeeringToken: peeringToken,
		})
		grpcErr, ok := grpcstatus.FromError(err)
		require.True(t, ok)
		require.Equal(t, codes.PermissionDenied, grpcErr.Code())
		testutil.RequireErrorContains(t, err, "a new peering token must be generated")
	})

	gatewayPort := freeport.GetOne(t)

	testutil.RunStep(t, "fail past bad mesh gateway", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		t.Cleanup(cancel)

		// Generate a new peering token for the dialer.
		tokenResp, err := acceptorClient.GenerateToken(ctx, &pbpeering.GenerateTokenRequest{PeerName: "my-peer-dialer"})
		require.NoError(t, err)

		store := dialer.Server.FSM().State()
		require.NoError(t, store.EnsureConfigEntry(1, &structs.MeshConfigEntry{
			Peering: &structs.PeeringMeshConfig{
				PeerThroughMeshGateways: true,
			},
		}))

		// Register a gateway that isn't actually listening.
		require.NoError(t, store.EnsureRegistration(2, &structs.RegisterRequest{
			ID:      types.NodeID(testUUID(t)),
			Node:    "gateway-node-1",
			Address: "127.0.0.1",
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindMeshGateway,
				ID:      "mesh-gateway-1",
				Service: "mesh-gateway",
				Port:    gatewayPort,
			},
		}))

		ctx, cancel = context.WithTimeout(context.Background(), 6*time.Second)
		t.Cleanup(cancel)

		// Call to establish should succeed when we fall back to remote server address.
		_, err = dialerClient.Establish(ctx, &pbpeering.EstablishRequest{
			PeerName:     "my-peer-acceptor",
			PeeringToken: tokenResp.PeeringToken,
		})
		require.NoError(t, err)
	})

	testutil.RunStep(t, "route through gateway", func(t *testing.T) {
		// Spin up a proxy listening at the gateway port registered above.
		gatewayAddr := fmt.Sprintf("127.0.0.1:%d", gatewayPort)

		// Configure a TCP proxy with an SNI route corresponding to the acceptor cluster.
		var proxy tcpproxy.Proxy
		target := &connWrapper{
			proxy: tcpproxy.DialProxy{
				Addr: acceptor.PublicGRPCAddr,
			},
		}
		proxy.AddSNIRoute(gatewayAddr, "server.dc1.peering.11111111-2222-3333-4444-555555555555.consul", target)
		proxy.AddStopACMESearch(gatewayAddr)

		require.NoError(t, proxy.Start())
		t.Cleanup(func() {
			proxy.Close()
			proxy.Wait()
		})

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		t.Cleanup(cancel)

		// Generate a new peering token for the dialer.
		tokenResp, err := acceptorClient.GenerateToken(ctx, &pbpeering.GenerateTokenRequest{PeerName: "my-peer-dialer"})
		require.NoError(t, err)

		store := dialer.Server.FSM().State()
		require.NoError(t, store.EnsureConfigEntry(1, &structs.MeshConfigEntry{
			Peering: &structs.PeeringMeshConfig{
				PeerThroughMeshGateways: true,
			},
		}))

		// Context is 1s sleep + 3s retry loop. Any longer and we're trying the remote gateway
		ctx, cancel = context.WithTimeout(context.Background(), 4*time.Second)
		t.Cleanup(cancel)

		start := time.Now()

		// Call to establish should succeed through the proxy.
		_, err = dialerClient.Establish(ctx, &pbpeering.EstablishRequest{
			PeerName:     "my-peer-acceptor",
			PeeringToken: tokenResp.PeeringToken,
		})
		require.NoError(t, err)

		// Dialing through a gateway is preceded by a mandatory 1s sleep.
		require.Greater(t, time.Since(start), 1*time.Second)

		// target.called is true when the tcproxy's conn handler was invoked.
		// This lets us know that the "Establish" success flowed through the proxy masquerading as a gateway.
		require.True(t, target.called)
	})
}

// connWrapper is a wrapper around tcpproxy.DialProxy to enable tracking whether the proxy handled a connection.
type connWrapper struct {
	proxy  tcpproxy.DialProxy
	called bool
}

func (w *connWrapper) HandleConn(src net.Conn) {
	w.called = true
	w.proxy.HandleConn(src)
}

func TestPeeringService_Establish_ACLEnforcement(t *testing.T) {
	validToken := peering.TestPeeringToken("83474a06-cca4-4ff4-99a4-4152929c8160")
	validTokenJSON, _ := json.Marshal(&validToken)
	validTokenB64 := base64.StdEncoding.EncodeToString(validTokenJSON)

	// TODO(peering): see note on newTestServer, refactor to not use this
	s := newTestServer(t, func(conf *consul.Config) {
		conf.ACLsEnabled = true
		conf.ACLResolverSettings.ACLDefaultPolicy = acl.PolicyDeny
	})
	upsertTestACLs(t, s.Server.FSM().State())

	client := pbpeering.NewPeeringServiceClient(s.ClientConn(t))

	type testcase struct {
		name      string
		req       *pbpeering.EstablishRequest
		token     string
		expectErr string
	}
	run := func(t *testing.T, tc testcase) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		t.Cleanup(cancel)

		options := structs.QueryOptions{Token: tc.token}
		ctx, err := external.ContextWithQueryOptions(ctx, options)
		require.NoError(t, err)
		_, err = client.Establish(ctx, tc.req)
		if tc.expectErr != "" {
			require.Contains(t, err.Error(), tc.expectErr)
			return
		}
		require.NotContains(t, err.Error(), "lacks permission")
	}
	tcs := []testcase{
		{
			name: "anonymous token lacks permissions",
			req: &pbpeering.EstablishRequest{
				PeerName:     "foo",
				PeeringToken: validTokenB64,
			},
			expectErr: "lacks permission 'peering:write'",
		},
		{
			name: "read token lacks permissions",
			req: &pbpeering.EstablishRequest{
				PeerName:     "foo",
				PeeringToken: validTokenB64,
			},
			token:     testTokenPeeringReadSecret,
			expectErr: "lacks permission 'peering:write'",
		},
		{
			name: "write token grants permission",
			req: &pbpeering.EstablishRequest{
				PeerName:     "foo",
				PeeringToken: validTokenB64,
			},
			token: testTokenPeeringWriteSecret,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestPeeringService_Read(t *testing.T) {
	// TODO(peering): see note on newTestServer, refactor to not use this
	s := newTestServer(t, nil)

	// insert peering directly to state store
	p := &pbpeering.Peering{
		ID:                  testUUID(t),
		Name:                "foo",
		State:               pbpeering.PeeringState_ESTABLISHING,
		PeerCAPems:          nil,
		PeerServerName:      "test",
		PeerServerAddresses: []string{"addr1"},
	}
	err := s.Server.FSM().State().PeeringWrite(10, &pbpeering.PeeringWriteRequest{Peering: p})
	require.NoError(t, err)

	client := pbpeering.NewPeeringServiceClient(s.ClientConn(t))

	type testcase struct {
		name      string
		req       *pbpeering.PeeringReadRequest
		expect    *pbpeering.PeeringReadResponse
		expectErr string
	}
	run := func(t *testing.T, tc testcase) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		t.Cleanup(cancel)

		resp, err := client.PeeringRead(ctx, tc.req)
		if tc.expectErr != "" {
			require.Contains(t, err.Error(), tc.expectErr)
			return
		}
		require.NoError(t, err)
		prototest.AssertDeepEqual(t, tc.expect, resp)
	}
	tcs := []testcase{
		{
			name:      "returns foo",
			req:       &pbpeering.PeeringReadRequest{Name: "foo"},
			expect:    &pbpeering.PeeringReadResponse{Peering: p},
			expectErr: "",
		},
		{
			name:      "bar not found",
			req:       &pbpeering.PeeringReadRequest{Name: "bar"},
			expect:    &pbpeering.PeeringReadResponse{},
			expectErr: "",
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestPeeringService_Read_ACLEnforcement(t *testing.T) {
	// TODO(peering): see note on newTestServer, refactor to not use this
	s := newTestServer(t, func(conf *consul.Config) {
		conf.ACLsEnabled = true
		conf.ACLResolverSettings.ACLDefaultPolicy = acl.PolicyDeny
	})
	upsertTestACLs(t, s.Server.FSM().State())

	// insert peering directly to state store
	p := &pbpeering.Peering{
		ID:                  testUUID(t),
		Name:                "foo",
		State:               pbpeering.PeeringState_ESTABLISHING,
		PeerCAPems:          nil,
		PeerServerName:      "test",
		PeerServerAddresses: []string{"addr1"},
	}
	err := s.Server.FSM().State().PeeringWrite(10, &pbpeering.PeeringWriteRequest{Peering: p})
	require.NoError(t, err)

	client := pbpeering.NewPeeringServiceClient(s.ClientConn(t))

	type testcase struct {
		name      string
		req       *pbpeering.PeeringReadRequest
		expect    *pbpeering.PeeringReadResponse
		token     string
		expectErr string
	}
	run := func(t *testing.T, tc testcase) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		t.Cleanup(cancel)

		options := structs.QueryOptions{Token: tc.token}
		ctx, err := external.ContextWithQueryOptions(ctx, options)
		require.NoError(t, err)
		resp, err := client.PeeringRead(ctx, tc.req)
		if tc.expectErr != "" {
			require.Contains(t, err.Error(), tc.expectErr)
			return
		}
		require.NoError(t, err)
		prototest.AssertDeepEqual(t, tc.expect, resp)
	}
	tcs := []testcase{
		{
			name:      "anonymous token lacks permissions",
			req:       &pbpeering.PeeringReadRequest{Name: "foo"},
			expect:    &pbpeering.PeeringReadResponse{Peering: p},
			expectErr: "lacks permission 'peering:read'",
		},
		{
			name: "read token grants permission",
			req: &pbpeering.PeeringReadRequest{
				Name: "foo",
			},
			expect: &pbpeering.PeeringReadResponse{Peering: p},
			token:  testTokenPeeringReadSecret,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestPeeringService_Read_Blocking(t *testing.T) {
	// TODO(peering): see note on newTestServer, refactor to not use this
	s := newTestServer(t, nil)

	// insert peering directly to state store
	lastIdx := uint64(10)
	p := &pbpeering.Peering{
		ID:                  testUUID(t),
		Name:                "foo",
		State:               pbpeering.PeeringState_ESTABLISHING,
		PeerCAPems:          nil,
		PeerServerName:      "test",
		PeerServerAddresses: []string{"addr1"},
	}
	err := s.Server.FSM().State().PeeringWrite(lastIdx, &pbpeering.PeeringWriteRequest{Peering: p})
	require.NoError(t, err)

	client := pbpeering.NewPeeringServiceClient(s.ClientConn(t))

	// Setup blocking query
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	options := structs.QueryOptions{
		MinQueryIndex: lastIdx,
		MaxQueryTime:  1 * time.Second,
	}
	ctx, err = external.ContextWithQueryOptions(ctx, options)
	require.NoError(t, err)

	// Mutate the original peering
	p = proto.Clone(p).(*pbpeering.Peering)
	p.PeerServerAddresses = append(p.PeerServerAddresses, "addr2")

	// Async change to trigger update
	marker := time.Now()
	go func() {
		time.Sleep(100 * time.Millisecond)
		lastIdx++
		require.NoError(t, s.Server.FSM().State().PeeringWrite(lastIdx, &pbpeering.PeeringWriteRequest{Peering: p}))
	}()

	var header metadata.MD
	resp, err := client.PeeringRead(ctx, &pbpeering.PeeringReadRequest{Name: "foo"}, gogrpc.Header(&header))
	require.NoError(t, err)

	// The query should return after the async change, but before the timeout
	require.True(t, time.Since(marker) >= 100*time.Millisecond)
	require.True(t, time.Since(marker) < 1*time.Second)

	// Verify query results
	meta, err := external.QueryMetaFromGRPCMeta(header)
	require.NoError(t, err)
	require.Equal(t, lastIdx, meta.Index)

	prototest.AssertDeepEqual(t, p, resp.Peering)
}

func TestPeeringService_Delete(t *testing.T) {
	tt := map[string]pbpeering.PeeringState{
		"active peering":     pbpeering.PeeringState_ACTIVE,
		"terminated peering": pbpeering.PeeringState_TERMINATED,
	}

	for name, overrideState := range tt {
		t.Run(name, func(t *testing.T) {
			// TODO(peering): see note on newTestServer, refactor to not use this
			s := newTestServer(t, nil)

			id := testUUID(t)

			// Write an initial peering
			require.NoError(t, s.Server.FSM().State().PeeringWrite(10, &pbpeering.PeeringWriteRequest{Peering: &pbpeering.Peering{
				ID:   id,
				Name: "foo",
			}}))

			_, p, err := s.Server.FSM().State().PeeringRead(nil, state.Query{Value: "foo"})
			require.NoError(t, err)
			require.Nil(t, p.DeletedAt)
			require.True(t, p.IsActive())

			require.NoError(t, s.Server.FSM().State().PeeringWrite(10, &pbpeering.PeeringWriteRequest{Peering: &pbpeering.Peering{
				ID:   id,
				Name: "foo",

				// Update the peering state to simulate deleting from a non-initial state.
				State: overrideState,
			}}))

			client := pbpeering.NewPeeringServiceClient(s.ClientConn(t))

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			t.Cleanup(cancel)

			_, err = client.PeeringDelete(ctx, &pbpeering.PeeringDeleteRequest{Name: "foo"})
			require.NoError(t, err)

			retry.Run(t, func(r *retry.R) {
				_, resp, err := s.Server.FSM().State().PeeringRead(nil, state.Query{Value: "foo"})
				require.NoError(r, err)

				// Initially the peering will be marked for deletion but eventually the leader
				// routine will clean it up.
				require.Nil(r, resp)
			})
		})
	}
}

func TestPeeringService_Delete_ACLEnforcement(t *testing.T) {
	// TODO(peering): see note on newTestServer, refactor to not use this
	s := newTestServer(t, func(conf *consul.Config) {
		conf.ACLsEnabled = true
		conf.ACLResolverSettings.ACLDefaultPolicy = acl.PolicyDeny
	})
	upsertTestACLs(t, s.Server.FSM().State())

	p := &pbpeering.Peering{
		ID:                  testUUID(t),
		Name:                "foo",
		State:               pbpeering.PeeringState_ESTABLISHING,
		PeerCAPems:          nil,
		PeerServerName:      "test",
		PeerServerAddresses: []string{"addr1"},
	}
	err := s.Server.FSM().State().PeeringWrite(10, &pbpeering.PeeringWriteRequest{Peering: p})
	require.NoError(t, err)
	require.Nil(t, p.DeletedAt)
	require.True(t, p.IsActive())

	client := pbpeering.NewPeeringServiceClient(s.ClientConn(t))

	type testcase struct {
		name      string
		req       *pbpeering.PeeringDeleteRequest
		token     string
		expectErr string
	}
	run := func(t *testing.T, tc testcase) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		t.Cleanup(cancel)

		options := structs.QueryOptions{Token: tc.token}
		ctx, err := external.ContextWithQueryOptions(ctx, options)
		require.NoError(t, err)
		_, err = client.PeeringDelete(ctx, tc.req)
		if tc.expectErr != "" {
			require.Contains(t, err.Error(), tc.expectErr)
			return
		}
		require.NoError(t, err)
	}
	tcs := []testcase{
		{
			name:      "anonymous token lacks permissions",
			req:       &pbpeering.PeeringDeleteRequest{Name: "foo"},
			expectErr: "lacks permission 'peering:write'",
		},
		{
			name: "read token lacks permissions",
			req: &pbpeering.PeeringDeleteRequest{
				Name: "foo",
			},
			token:     testTokenPeeringReadSecret,
			expectErr: "lacks permission 'peering:write'",
		},
		{
			name: "write token grants permission",
			req: &pbpeering.PeeringDeleteRequest{
				Name: "foo",
			},
			token: testTokenPeeringWriteSecret,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}

}

func TestPeeringService_List(t *testing.T) {
	// TODO(peering): see note on newTestServer, refactor to not use this
	s := newTestServer(t, nil)

	// Insert peerings directly to state store.
	// Note that the state store holds reference to the underlying
	// variables; do not modify them after writing.
	lastIdx := uint64(10)
	foo := &pbpeering.Peering{
		ID:                  testUUID(t),
		Name:                "foo",
		State:               pbpeering.PeeringState_ESTABLISHING,
		PeerCAPems:          nil,
		PeerServerName:      "fooservername",
		PeerServerAddresses: []string{"addr1"},
	}
	require.NoError(t, s.Server.FSM().State().PeeringWrite(lastIdx, &pbpeering.PeeringWriteRequest{Peering: foo}))

	lastIdx++
	bar := &pbpeering.Peering{
		ID:                  testUUID(t),
		Name:                "bar",
		State:               pbpeering.PeeringState_ACTIVE,
		PeerCAPems:          nil,
		PeerServerName:      "barservername",
		PeerServerAddresses: []string{"addr1"},
	}
	require.NoError(t, s.Server.FSM().State().PeeringWrite(lastIdx, &pbpeering.PeeringWriteRequest{Peering: bar}))

	client := pbpeering.NewPeeringServiceClient(s.ClientConn(t))

	t.Run("non-blocking query", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		t.Cleanup(cancel)

		var header metadata.MD
		resp, err := client.PeeringList(ctx, &pbpeering.PeeringListRequest{}, gogrpc.Header(&header))
		require.NoError(t, err)

		meta, err := external.QueryMetaFromGRPCMeta(header)
		require.NoError(t, err)
		require.Equal(t, lastIdx, meta.Index)

		expect := &pbpeering.PeeringListResponse{
			Peerings:       []*pbpeering.Peering{bar, foo},
			OBSOLETE_Index: lastIdx,
		}
		prototest.AssertDeepEqual(t, expect, resp)
	})

	t.Run("blocking query", func(t *testing.T) {
		// Setup blocking query
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		t.Cleanup(cancel)

		marker := time.Now()
		options := structs.QueryOptions{
			MinQueryIndex: lastIdx,
			MaxQueryTime:  1 * time.Second,
		}
		ctx, err := external.ContextWithQueryOptions(ctx, options)
		require.NoError(t, err)

		// Async change to trigger update
		baz := &pbpeering.Peering{
			ID:                  testUUID(t),
			Name:                "baz",
			State:               pbpeering.PeeringState_ACTIVE,
			PeerCAPems:          nil,
			PeerServerName:      "bazservername",
			PeerServerAddresses: []string{"addr1"},
		}
		go func() {
			time.Sleep(100 * time.Millisecond)

			lastIdx++
			require.NoError(t, s.Server.FSM().State().PeeringWrite(lastIdx, &pbpeering.PeeringWriteRequest{Peering: baz}))
		}()

		// Make the blocking query
		var header metadata.MD
		resp, err := client.PeeringList(ctx, &pbpeering.PeeringListRequest{}, gogrpc.Header(&header))
		require.NoError(t, err)

		// The query should return after the async change, but before the timeout
		require.True(t, time.Since(marker) >= 100*time.Millisecond)
		require.True(t, time.Since(marker) < 1*time.Second)

		// Verify query results
		meta, err := external.QueryMetaFromGRPCMeta(header)
		require.NoError(t, err)
		require.Equal(t, lastIdx, meta.Index)

		expect := &pbpeering.PeeringListResponse{
			Peerings:       []*pbpeering.Peering{bar, baz, foo},
			OBSOLETE_Index: lastIdx,
		}
		prototest.AssertDeepEqual(t, expect, resp)
	})
}

func TestPeeringService_List_ACLEnforcement(t *testing.T) {
	// TODO(peering): see note on newTestServer, refactor to not use this
	s := newTestServer(t, func(conf *consul.Config) {
		conf.ACLsEnabled = true
		conf.ACLResolverSettings.ACLDefaultPolicy = acl.PolicyDeny
	})
	upsertTestACLs(t, s.Server.FSM().State())

	// insert peering directly to state store
	foo := &pbpeering.Peering{
		ID:                  testUUID(t),
		Name:                "foo",
		State:               pbpeering.PeeringState_ESTABLISHING,
		PeerCAPems:          nil,
		PeerServerName:      "fooservername",
		PeerServerAddresses: []string{"addr1"},
	}
	require.NoError(t, s.Server.FSM().State().PeeringWrite(10, &pbpeering.PeeringWriteRequest{Peering: foo}))
	bar := &pbpeering.Peering{
		ID:                  testUUID(t),
		Name:                "bar",
		State:               pbpeering.PeeringState_ACTIVE,
		PeerCAPems:          nil,
		PeerServerName:      "barservername",
		PeerServerAddresses: []string{"addr1"},
	}
	require.NoError(t, s.Server.FSM().State().PeeringWrite(15, &pbpeering.PeeringWriteRequest{Peering: bar}))

	client := pbpeering.NewPeeringServiceClient(s.ClientConn(t))

	type testcase struct {
		name      string
		token     string
		expect    *pbpeering.PeeringListResponse
		expectErr string
	}
	run := func(t *testing.T, tc testcase) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		t.Cleanup(cancel)

		options := structs.QueryOptions{Token: tc.token}
		ctx, err := external.ContextWithQueryOptions(ctx, options)
		require.NoError(t, err)
		resp, err := client.PeeringList(ctx, &pbpeering.PeeringListRequest{})
		if tc.expectErr != "" {
			require.Contains(t, err.Error(), tc.expectErr)
			return
		}
		require.NoError(t, err)
		prototest.AssertDeepEqual(t, tc.expect, resp)
	}
	tcs := []testcase{
		{
			name:      "anonymous token lacks permissions",
			expectErr: "lacks permission 'peering:read'",
		},
		{
			name:  "read token grants permission",
			token: testTokenPeeringReadSecret,
			expect: &pbpeering.PeeringListResponse{
				Peerings:       []*pbpeering.Peering{bar, foo},
				OBSOLETE_Index: 15,
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestPeeringService_TrustBundleRead(t *testing.T) {
	srv := newTestServer(t, nil)
	store := srv.Server.FSM().State()
	client := pbpeering.NewPeeringServiceClient(srv.ClientConn(t))

	var lastIdx uint64 = 1
	_ = setupTestPeering(t, store, "my-peering", lastIdx)

	bundle := &pbpeering.PeeringTrustBundle{
		TrustDomain: "peer1.com",
		PeerName:    "my-peering",
		RootPEMs:    []string{"peer1-root-1"},
	}
	lastIdx++
	require.NoError(t, store.PeeringTrustBundleWrite(lastIdx, bundle))

	t.Run("non-blocking query", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		t.Cleanup(cancel)

		resp, err := client.TrustBundleRead(ctx, &pbpeering.TrustBundleReadRequest{
			Name: "my-peering",
		})
		require.NoError(t, err)
		require.Equal(t, lastIdx, resp.OBSOLETE_Index)
		require.NotNil(t, resp.Bundle)
		prototest.AssertDeepEqual(t, bundle, resp.Bundle)
	})

	t.Run("blocking query", func(t *testing.T) {
		// Set up the blocking query
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		t.Cleanup(cancel)

		marker := time.Now()
		options := structs.QueryOptions{
			MinQueryIndex: lastIdx,
			MaxQueryTime:  1 * time.Second,
		}
		ctx, err := external.ContextWithQueryOptions(ctx, options)
		require.NoError(t, err)

		updatedBundle := &pbpeering.PeeringTrustBundle{
			TrustDomain: "peer1.com",
			PeerName:    "my-peering",
			RootPEMs:    []string{"peer1-root-1", "peer1-root-2"}, // Adding a CA here
		}

		// Async change to trigger update
		go func() {
			time.Sleep(100 * time.Millisecond)
			lastIdx++
			require.NoError(t, store.PeeringTrustBundleWrite(lastIdx, updatedBundle))
		}()

		// Make the blocking query
		var header metadata.MD
		resp, err := client.TrustBundleRead(ctx, &pbpeering.TrustBundleReadRequest{
			Name: "my-peering",
		}, gogrpc.Header(&header))
		require.NoError(t, err)

		// The query should return after the async change, but before the timeout
		require.True(t, time.Since(marker) >= 100*time.Millisecond)
		require.True(t, time.Since(marker) < 1*time.Second)

		// Verify query results
		meta, err := external.QueryMetaFromGRPCMeta(header)
		require.NoError(t, err)
		require.Equal(t, lastIdx, meta.Index)

		require.Equal(t, lastIdx, resp.OBSOLETE_Index)
		require.NotNil(t, resp.Bundle)
		prototest.AssertDeepEqual(t, updatedBundle, resp.Bundle)
	})
}

func TestPeeringService_TrustBundleRead_ACLEnforcement(t *testing.T) {
	// TODO(peering): see note on newTestServer, refactor to not use this
	s := newTestServer(t, func(conf *consul.Config) {
		conf.ACLsEnabled = true
		conf.ACLResolverSettings.ACLDefaultPolicy = acl.PolicyDeny
	})
	store := s.Server.FSM().State()
	upsertTestACLs(t, s.Server.FSM().State())

	// Insert peering and trust bundle directly to state store.
	_ = setupTestPeering(t, store, "my-peering", 10)

	bundle := &pbpeering.PeeringTrustBundle{
		TrustDomain: "peer1.com",
		PeerName:    "my-peering",
		RootPEMs:    []string{"peer1-root-1"},
	}
	require.NoError(t, store.PeeringTrustBundleWrite(11, bundle))

	client := pbpeering.NewPeeringServiceClient(s.ClientConn(t))

	type testcase struct {
		name      string
		req       *pbpeering.TrustBundleReadRequest
		token     string
		expect    *pbpeering.PeeringTrustBundle
		expectErr string
	}
	run := func(t *testing.T, tc testcase) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		t.Cleanup(cancel)

		options := structs.QueryOptions{Token: tc.token}
		ctx, err := external.ContextWithQueryOptions(ctx, options)
		require.NoError(t, err)
		resp, err := client.TrustBundleRead(ctx, tc.req)
		if tc.expectErr != "" {
			require.Contains(t, err.Error(), tc.expectErr)
			return
		}
		require.NoError(t, err)
		prototest.AssertDeepEqual(t, tc.expect, resp.Bundle)
	}
	tcs := []testcase{
		{
			name:      "anonymous token lacks permissions",
			req:       &pbpeering.TrustBundleReadRequest{Name: "foo"},
			expectErr: "lacks permission 'service:write'",
		},
		{
			name: "service read token lacks permissions",
			req: &pbpeering.TrustBundleReadRequest{
				Name: "my-peering",
			},
			token:     testTokenServiceReadSecret,
			expectErr: "lacks permission 'service:write'",
		},
		{
			name: "with service write token",
			req: &pbpeering.TrustBundleReadRequest{
				Name: "my-peering",
			},
			token:  testTokenServiceWriteSecret,
			expect: bundle,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

// Setup:
// - Peerings "foo" and "bar" with trust bundles saved
// - "api" service exported to both "foo" and "bar"
// - "web" service exported to "baz"
func TestPeeringService_TrustBundleListByService(t *testing.T) {
	s := newTestServer(t, nil)
	store := s.Server.FSM().State()

	var lastIdx uint64 = 10

	lastIdx++
	require.NoError(t, s.Server.FSM().State().PeeringWrite(lastIdx, &pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			ID:                  testUUID(t),
			Name:                "foo",
			State:               pbpeering.PeeringState_ESTABLISHING,
			PeerServerName:      "test",
			PeerServerAddresses: []string{"addr1"},
		},
	}))

	lastIdx++
	require.NoError(t, s.Server.FSM().State().PeeringWrite(lastIdx, &pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			ID:                  testUUID(t),
			Name:                "bar",
			State:               pbpeering.PeeringState_ESTABLISHING,
			PeerServerName:      "test-bar",
			PeerServerAddresses: []string{"addr2"},
		},
	}))

	lastIdx++
	require.NoError(t, store.PeeringTrustBundleWrite(lastIdx, &pbpeering.PeeringTrustBundle{
		TrustDomain: "foo.com",
		PeerName:    "foo",
		RootPEMs:    []string{"foo-root-1"},
	}))

	lastIdx++
	require.NoError(t, store.PeeringTrustBundleWrite(lastIdx, &pbpeering.PeeringTrustBundle{
		TrustDomain: "bar.com",
		PeerName:    "bar",
		RootPEMs:    []string{"bar-root-1"},
	}))

	lastIdx++
	require.NoError(t, store.EnsureNode(lastIdx, &structs.Node{
		Node: "my-node", Address: "127.0.0.1",
	}))

	lastIdx++
	require.NoError(t, store.EnsureService(lastIdx, "my-node", &structs.NodeService{
		ID:      "api",
		Service: "api",
		Port:    8000,
	}))

	entry := structs.ExportedServicesConfigEntry{
		Name: "default",
		Services: []structs.ExportedService{
			{
				Name: "api",
				Consumers: []structs.ServiceConsumer{
					{
						Peer: "foo",
					},
					{
						Peer: "bar",
					},
				},
			},
			{
				Name: "web",
				Consumers: []structs.ServiceConsumer{
					{
						Peer: "baz",
					},
				},
			},
		},
	}
	require.NoError(t, entry.Normalize())
	require.NoError(t, entry.Validate())

	lastIdx++
	require.NoError(t, store.EnsureConfigEntry(lastIdx, &entry))

	client := pbpeering.NewPeeringServiceClient(s.ClientConn(t))

	t.Run("non-blocking query", func(t *testing.T) {
		req := pbpeering.TrustBundleListByServiceRequest{
			ServiceName: "api",
		}
		resp, err := client.TrustBundleListByService(context.Background(), &req)
		require.NoError(t, err)
		require.Len(t, resp.Bundles, 2)
		require.Equal(t, []string{"bar-root-1"}, resp.Bundles[0].RootPEMs)
		require.Equal(t, []string{"foo-root-1"}, resp.Bundles[1].RootPEMs)
		require.Equal(t, uint64(17), resp.OBSOLETE_Index)
	})

	t.Run("blocking query", func(t *testing.T) {
		// Setup blocking query
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		t.Cleanup(cancel)

		options := structs.QueryOptions{
			MinQueryIndex: lastIdx,
			MaxQueryTime:  1 * time.Second,
		}
		ctx, err := external.ContextWithQueryOptions(ctx, options)
		require.NoError(t, err)

		// Async change to trigger update
		marker := time.Now()
		go func() {
			time.Sleep(100 * time.Millisecond)
			lastIdx++
			require.NoError(t, store.PeeringTrustBundleWrite(lastIdx, &pbpeering.PeeringTrustBundle{
				TrustDomain: "bar.com",
				PeerName:    "bar",
				RootPEMs:    []string{"bar-root-1", "bar-root-2"}, // Appending new cert
			}))
		}()

		// Make the blocking query
		req := pbpeering.TrustBundleListByServiceRequest{
			ServiceName: "api",
		}
		var header metadata.MD
		resp, err := client.TrustBundleListByService(ctx, &req, gogrpc.Header(&header))
		require.NoError(t, err)

		// The query should return after the async change, but before the timeout
		require.True(t, time.Since(marker) >= 100*time.Millisecond)
		require.True(t, time.Since(marker) < 1*time.Second)

		// Verify query results
		meta, err := external.QueryMetaFromGRPCMeta(header)
		require.NoError(t, err)
		require.Equal(t, uint64(18), meta.Index)

		require.Len(t, resp.Bundles, 2)
		require.Equal(t, []string{"bar-root-1", "bar-root-2"}, resp.Bundles[0].RootPEMs)
		require.Equal(t, []string{"foo-root-1"}, resp.Bundles[1].RootPEMs)
		require.Equal(t, uint64(18), resp.OBSOLETE_Index)
	})
}

func TestPeeringService_validatePeer(t *testing.T) {
	s1 := newTestServer(t, nil)
	client1 := pbpeering.NewPeeringServiceClient(s1.ClientConn(t))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	testutil.RunStep(t, "generate a token", func(t *testing.T) {
		req := pbpeering.GenerateTokenRequest{PeerName: "peer-b"}
		resp, err := client1.GenerateToken(ctx, &req)
		require.NoError(t, err)
		require.NotEmpty(t, resp)
	})

	s2 := newTestServer(t, func(conf *consul.Config) {
		conf.Datacenter = "dc2"
		conf.PrimaryDatacenter = "dc2"
	})
	client2 := pbpeering.NewPeeringServiceClient(s2.ClientConn(t))

	req := pbpeering.GenerateTokenRequest{PeerName: "my-peer-s1"}
	resp, err := client2.GenerateToken(ctx, &req)
	require.NoError(t, err)
	require.NotEmpty(t, resp)

	s2Token := resp.PeeringToken

	testutil.RunStep(t, "send an establish request for a different peer name", func(t *testing.T) {
		resp, err := client1.Establish(ctx, &pbpeering.EstablishRequest{
			PeerName:     "peer-c",
			PeeringToken: s2Token,
		})
		require.NoError(t, err)
		require.NotEmpty(t, resp)
	})

	testutil.RunStep(t, "attempt to generate token with the same name used as dialer", func(t *testing.T) {
		req := pbpeering.GenerateTokenRequest{PeerName: "peer-c"}
		resp, err := client1.GenerateToken(ctx, &req)

		require.Error(t, err)
		require.Contains(t, err.Error(),
			"cannot create peering with name: \"peer-c\"; there is already an established peering")
		require.Nil(t, resp)
	})

	testutil.RunStep(t, "attempt to establish the with the same name used as acceptor", func(t *testing.T) {
		resp, err := client1.Establish(ctx, &pbpeering.EstablishRequest{
			PeerName:     "peer-b",
			PeeringToken: s2Token,
		})

		require.Error(t, err)
		require.Contains(t, err.Error(),
			"cannot create peering with name: \"peer-b\"; there is an existing peering expecting to be dialed")
		require.Nil(t, resp)
	})
}

// Test RPC endpoint responses when peering is disabled. They should all return an error.
func TestPeeringService_PeeringDisabled(t *testing.T) {
	// TODO(peering): see note on newTestServer, refactor to not use this
	s := newTestServer(t, func(c *consul.Config) { c.PeeringEnabled = false })
	client := pbpeering.NewPeeringServiceClient(s.ClientConn(t))

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	// assertFailedResponse is a helper function that checks the error from a gRPC
	// response is what we expect when peering is disabled.
	assertFailedResponse := func(t *testing.T, err error) {
		actErr, ok := grpcstatus.FromError(err)
		require.True(t, ok)
		require.Equal(t, codes.FailedPrecondition, actErr.Code())
		require.Equal(t, "peering must be enabled to use this endpoint", actErr.Message())
	}

	// Test all the endpoints.

	t.Run("PeeringWrite", func(t *testing.T) {
		_, err := client.PeeringWrite(ctx, &pbpeering.PeeringWriteRequest{})
		assertFailedResponse(t, err)
	})

	t.Run("PeeringRead", func(t *testing.T) {
		_, err := client.PeeringRead(ctx, &pbpeering.PeeringReadRequest{})
		assertFailedResponse(t, err)
	})

	t.Run("PeeringDelete", func(t *testing.T) {
		_, err := client.PeeringDelete(ctx, &pbpeering.PeeringDeleteRequest{})
		assertFailedResponse(t, err)
	})

	t.Run("PeeringList", func(t *testing.T) {
		_, err := client.PeeringList(ctx, &pbpeering.PeeringListRequest{})
		assertFailedResponse(t, err)
	})

	t.Run("Establish", func(t *testing.T) {
		_, err := client.Establish(ctx, &pbpeering.EstablishRequest{})
		assertFailedResponse(t, err)
	})

	t.Run("GenerateToken", func(t *testing.T) {
		_, err := client.GenerateToken(ctx, &pbpeering.GenerateTokenRequest{})
		assertFailedResponse(t, err)
	})

	t.Run("TrustBundleRead", func(t *testing.T) {
		_, err := client.TrustBundleRead(ctx, &pbpeering.TrustBundleReadRequest{})
		assertFailedResponse(t, err)
	})

	t.Run("TrustBundleListByService", func(t *testing.T) {
		_, err := client.TrustBundleListByService(ctx, &pbpeering.TrustBundleListByServiceRequest{})
		assertFailedResponse(t, err)
	})
}

func TestPeeringService_TrustBundleListByService_ACLEnforcement(t *testing.T) {
	// TODO(peering): see note on newTestServer, refactor to not use this
	s := newTestServer(t, func(conf *consul.Config) {
		conf.ACLsEnabled = true
		conf.ACLResolverSettings.ACLDefaultPolicy = acl.PolicyDeny
	})
	store := s.Server.FSM().State()
	upsertTestACLs(t, s.Server.FSM().State())

	var lastIdx uint64 = 10

	lastIdx++
	require.NoError(t, s.Server.FSM().State().PeeringWrite(lastIdx, &pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			ID:                  testUUID(t),
			Name:                "foo",
			State:               pbpeering.PeeringState_ESTABLISHING,
			PeerServerName:      "test",
			PeerServerAddresses: []string{"addr1"},
		},
	}))

	lastIdx++
	require.NoError(t, store.PeeringTrustBundleWrite(lastIdx, &pbpeering.PeeringTrustBundle{
		TrustDomain: "foo.com",
		PeerName:    "foo",
		RootPEMs:    []string{"foo-root-1"},
	}))

	lastIdx++
	require.NoError(t, store.EnsureNode(lastIdx, &structs.Node{
		Node: "my-node", Address: "127.0.0.1",
	}))

	lastIdx++
	require.NoError(t, store.EnsureService(lastIdx, "my-node", &structs.NodeService{
		ID:      "api",
		Service: "api",
		Port:    8000,
	}))

	entry := structs.ExportedServicesConfigEntry{
		Name: "default",
		Services: []structs.ExportedService{
			{
				Name: "api",
				Consumers: []structs.ServiceConsumer{
					{
						Peer: "foo",
					},
				},
			},
		},
	}
	require.NoError(t, entry.Normalize())
	require.NoError(t, entry.Validate())

	lastIdx++
	require.NoError(t, store.EnsureConfigEntry(lastIdx, &entry))

	client := pbpeering.NewPeeringServiceClient(s.ClientConn(t))

	type testcase struct {
		name      string
		req       *pbpeering.TrustBundleListByServiceRequest
		token     string
		expect    []string
		expectErr string
	}
	run := func(t *testing.T, tc testcase) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		t.Cleanup(cancel)

		options := structs.QueryOptions{Token: tc.token}
		ctx, err := external.ContextWithQueryOptions(ctx, options)
		require.NoError(t, err)
		resp, err := client.TrustBundleListByService(ctx, tc.req)
		if tc.expectErr != "" {
			require.Contains(t, err.Error(), tc.expectErr)
			return
		}
		require.NoError(t, err)
		require.Len(t, resp.Bundles, 1)
		require.Equal(t, tc.expect, resp.Bundles[0].RootPEMs)
	}
	tcs := []testcase{
		{
			name:      "anonymous token lacks permissions",
			req:       &pbpeering.TrustBundleListByServiceRequest{ServiceName: "api"},
			expectErr: "lacks permission 'service:write'",
		},
		{
			name: "service read token lacks permission",
			req: &pbpeering.TrustBundleListByServiceRequest{
				ServiceName: "api",
			},
			token:     testTokenServiceReadSecret,
			expectErr: "lacks permission 'service:write'",
		},
		{
			name: "with service write token",
			req: &pbpeering.TrustBundleListByServiceRequest{
				ServiceName: "api",
			},
			token:  testTokenServiceWriteSecret,
			expect: []string{"foo-root-1"},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

// newTestServer is copied from partition/service_test.go, with the addition of certs/cas.
// TODO(peering): these are endpoint tests and should live in the agent/consul
// package. Instead, these can be written around a mock client (see testing.go)
// and a mock backend (future)
func newTestServer(t *testing.T, cb func(conf *consul.Config)) testingServer {
	t.Helper()
	conf := consul.DefaultConfig()
	dir := testutil.TempDir(t, "consul")

	ports := freeport.GetN(t, 4) // {rpc, serf_lan, serf_wan, grpc}

	conf.PeeringEnabled = true
	conf.Bootstrap = true
	conf.Datacenter = "dc1"
	conf.DataDir = dir
	conf.RPCAddr = &net.TCPAddr{IP: []byte{127, 0, 0, 1}, Port: ports[0]}
	conf.RaftConfig.ElectionTimeout = 200 * time.Millisecond
	conf.RaftConfig.LeaderLeaseTimeout = 100 * time.Millisecond
	conf.RaftConfig.HeartbeatTimeout = 200 * time.Millisecond
	conf.TLSConfig.Domain = "consul"

	conf.SerfLANConfig.MemberlistConfig.BindAddr = "127.0.0.1"
	conf.SerfLANConfig.MemberlistConfig.BindPort = ports[1]
	conf.SerfLANConfig.MemberlistConfig.AdvertisePort = ports[1]
	conf.SerfWANConfig.MemberlistConfig.BindAddr = "127.0.0.1"
	conf.SerfWANConfig.MemberlistConfig.BindPort = ports[2]
	conf.SerfWANConfig.MemberlistConfig.AdvertisePort = ports[2]

	conf.PrimaryDatacenter = "dc1"
	conf.ConnectEnabled = true

	ca := connect.TestCA(t, nil)
	conf.CAConfig = &structs.CAConfiguration{
		ClusterID: connect.TestClusterID,
		Provider:  structs.ConsulCAProvider,
		Config: map[string]interface{}{
			"PrivateKey":          ca.SigningKey,
			"RootCert":            ca.RootCert,
			"LeafCertTTL":         "72h",
			"IntermediateCertTTL": "288h",
		},
	}
	conf.GRPCTLSPort = ports[3]

	nodeID, err := uuid.GenerateUUID()
	if err != nil {
		t.Fatal(err)
	}
	conf.NodeID = types.NodeID(nodeID)

	if cb != nil {
		cb(conf)
	}

	// Apply config to copied fields because many tests only set the old
	// values.
	conf.ACLResolverSettings.ACLsEnabled = conf.ACLsEnabled
	conf.ACLResolverSettings.NodeName = conf.NodeName
	conf.ACLResolverSettings.Datacenter = conf.Datacenter
	conf.ACLResolverSettings.EnterpriseMeta = *conf.AgentEnterpriseMeta()

	deps := newDefaultDeps(t, conf)
	externalGRPCServer := external.NewServer(deps.Logger, nil, deps.TLSConfigurator, rate.NullRequestLimitsHandler())

	server, err := consul.NewServer(conf, deps, externalGRPCServer, nil, deps.Logger)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, server.Shutdown())
	})

	require.NoError(t, deps.TLSConfigurator.UpdateAutoTLSCert(connect.TestServerLeaf(t, conf.Datacenter, ca)))
	deps.TLSConfigurator.UpdateAutoTLSPeeringServerName(connect.PeeringServerSAN(conf.Datacenter, connect.TestTrustDomain))

	// Normally the gRPC server listener is created at the agent level and
	// passed down into the Server creation.
	grpcAddr := fmt.Sprintf("127.0.0.1:%d", conf.GRPCTLSPort)

	ln, err := net.Listen("tcp", grpcAddr)
	require.NoError(t, err)
	ln = agentmiddleware.LabelledListener{Listener: ln, Protocol: agentmiddleware.ProtocolTLS}

	go func() {
		_ = externalGRPCServer.Serve(ln)
	}()
	t.Cleanup(externalGRPCServer.Stop)

	testrpc.WaitForLeader(t, server.RPC, conf.Datacenter)
	testrpc.WaitForActiveCARoot(t, server.RPC, conf.Datacenter, nil)

	return testingServer{
		Server:         server,
		PublicGRPCAddr: grpcAddr,
	}
}

func (s testingServer) ClientConn(t *testing.T) *gogrpc.ClientConn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	rpcAddr := s.Server.Listener.Addr().String()

	conn, err := gogrpc.DialContext(ctx, rpcAddr,
		gogrpc.WithContextDialer(newServerDialer(rpcAddr)),
		//nolint:staticcheck
		gogrpc.WithInsecure(),
		gogrpc.WithBlock())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	return conn
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

type testingServer struct {
	Server         *consul.Server
	PublicGRPCAddr string
}

func newConfig(t *testing.T, dc, agentType string) resolver.Config {
	n := t.Name()
	s := strings.Replace(n, "/", "", -1)
	s = strings.Replace(s, "_", "", -1)
	return resolver.Config{
		Datacenter: dc,
		AgentType:  agentType,
		Authority:  strings.ToLower(s),
	}
}

// TODO(peering): remove duplication between this and agent/consul tests
func newDefaultDeps(t *testing.T, c *consul.Config) consul.Deps {
	t.Helper()

	logger := hclog.NewInterceptLogger(&hclog.LoggerOptions{
		Name:   c.NodeName,
		Level:  hclog.Debug,
		Output: testutil.NewLogBuffer(t),
	})

	tls, err := tlsutil.NewConfigurator(c.TLSConfig, logger)
	require.NoError(t, err, "failed to create tls configuration")

	r := router.NewRouter(logger, c.Datacenter, fmt.Sprintf("%s.%s", c.NodeName, c.Datacenter), nil)
	builder := resolver.NewServerResolverBuilder(newConfig(t, c.Datacenter, "client"))
	resolver.Register(builder)

	connPool := &pool.ConnPool{
		Server:          false,
		SrcAddr:         c.RPCSrcAddr,
		Logger:          logger.StandardLogger(&hclog.StandardLoggerOptions{InferLevels: true}),
		MaxTime:         2 * time.Minute,
		MaxStreams:      4,
		TLSConfigurator: tls,
		Datacenter:      c.Datacenter,
	}

	balancerBuilder := balancer.NewBuilder(builder.Authority(), testutil.Logger(t))
	balancerBuilder.Register()
	t.Cleanup(balancerBuilder.Deregister)

	return consul.Deps{
		EventPublisher:  stream.NewEventPublisher(10 * time.Second),
		Logger:          logger,
		TLSConfigurator: tls,
		Tokens:          new(token.Store),
		Router:          r,
		ConnPool:        connPool,
		GRPCConnPool: grpc.NewClientConnPool(grpc.ClientConnPoolConfig{
			Servers:               builder,
			TLSWrapper:            grpc.TLSWrapper(tls.OutgoingRPCWrapper()),
			UseTLSForDC:           tls.UseTLS,
			DialingFromServer:     true,
			DialingFromDatacenter: c.Datacenter,
		}),
		LeaderForwarder:          builder,
		EnterpriseDeps:           newDefaultDepsEnterprise(t, logger, c),
		NewRequestRecorderFunc:   middleware.NewRequestRecorder,
		GetNetRPCInterceptorFunc: middleware.GetNetRPCInterceptor,
		XDSStreamLimiter:         limiter.NewSessionLimiter(),
	}
}

func upsertTestACLs(t *testing.T, store *state.Store) {
	var (
		testPolicyPeeringReadID  = "43fed171-ad1d-4d3b-9df3-c99c1c835c37"
		testPolicyPeeringWriteID = "cddb0821-e720-4411-bbdd-cc62ce417eac"

		testPolicyServiceReadID  = "0e054136-f5d3-4627-a7e6-198f1df923d3"
		testPolicyServiceWriteID = "b55e03f4-c9dd-4210-8d24-f7ea8e2a1918"
	)
	policies := structs.ACLPolicies{
		{
			ID:    testPolicyPeeringReadID,
			Name:  "peering-read",
			Rules: `peering = "read"`,
		},
		{
			ID:    testPolicyPeeringWriteID,
			Name:  "peering-write",
			Rules: `peering = "write"`,
		},
		{
			ID:    testPolicyServiceReadID,
			Name:  "service-read",
			Rules: `service "api" { policy = "read" }`,
		},
		{
			ID:    testPolicyServiceWriteID,
			Name:  "service-write",
			Rules: `service "api" { policy = "write" }`,
		},
	}
	require.NoError(t, store.ACLPolicyBatchSet(100, policies))

	tokens := structs.ACLTokens{
		&structs.ACLToken{
			AccessorID:  "22500c91-723c-4335-be8a-6697417dc35b",
			SecretID:    testTokenPeeringReadSecret,
			Description: "peering read",
			Policies: []structs.ACLTokenPolicyLink{
				{
					ID: testPolicyPeeringReadID,
				},
			},
		},
		&structs.ACLToken{
			AccessorID:  "de924f93-cfec-404c-9a7e-c1c9b96b8cae",
			SecretID:    testTokenPeeringWriteSecret,
			Description: "peering write",
			Policies: []structs.ACLTokenPolicyLink{
				{
					ID: testPolicyPeeringWriteID,
				},
			},
		},
		&structs.ACLToken{
			AccessorID:  "53c54f79-ffed-47d4-904e-e2e0e40c0a01",
			SecretID:    testTokenServiceReadSecret,
			Description: "service read",
			Policies: []structs.ACLTokenPolicyLink{
				{
					ID: testPolicyServiceReadID,
				},
			},
		},
		&structs.ACLToken{
			AccessorID:  "a100fa5f-db72-49f0-8f61-aa1f9f92f657",
			SecretID:    testTokenServiceWriteSecret,
			Description: "service write",
			Policies: []structs.ACLTokenPolicyLink{
				{
					ID: testPolicyServiceWriteID,
				},
			},
		},
	}
	require.NoError(t, store.ACLTokenBatchSet(101, tokens, state.ACLTokenSetOptions{}))
}

//nolint:unparam
func setupTestPeering(t *testing.T, store *state.Store, name string, index uint64) string {
	t.Helper()
	err := store.PeeringWrite(index, &pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			ID:   testUUID(t),
			Name: name,
		},
	})
	require.NoError(t, err)

	_, p, err := store.PeeringRead(nil, state.Query{Value: name})
	require.NoError(t, err)
	require.NotNil(t, p)

	return p.ID
}

func testUUID(t *testing.T) string {
	v, err := lib.GenerateUUID(nil)
	require.NoError(t, err)
	return v
}
