package peering_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"path"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-uuid"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	gogrpc "google.golang.org/grpc"

	grpc "github.com/hashicorp/consul/agent/grpc/private"
	"github.com/hashicorp/consul/agent/grpc/private/resolver"
	"github.com/hashicorp/consul/proto/prototest"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/pool"
	"github.com/hashicorp/consul/agent/router"
	"github.com/hashicorp/consul/agent/rpc/middleware"
	"github.com/hashicorp/consul/agent/rpc/peering"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/proto/pbpeering"
	"github.com/hashicorp/consul/sdk/freeport"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/tlsutil"
	"github.com/hashicorp/consul/types"
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
	require.NoError(t, ioutil.WriteFile(cafile, []byte(ca), 0600))

	// TODO(peering): see note on newTestServer, refactor to not use this
	s := newTestServer(t, func(c *consul.Config) {
		c.SerfLANConfig.MemberlistConfig.AdvertiseAddr = "127.0.0.1"
		c.TLSConfig.InternalRPC.CAFile = cafile
		c.DataDir = dir
	})
	client := pbpeering.NewPeeringServiceClient(s.ClientConn(t))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	expectedAddr := s.Server.Listener.Addr().String()

	// TODO(peering): for more failure cases, consider using a table test
	// check meta tags
	reqE := pbpeering.GenerateTokenRequest{PeerName: "peerB", Datacenter: "dc1", Meta: generateTooManyMetaKeys()}
	_, errE := client.GenerateToken(ctx, &reqE)
	require.EqualError(t, errE, "rpc error: code = Unknown desc = meta tags failed validation: Node metadata cannot contain more than 64 key/value pairs")

	// happy path
	req := pbpeering.GenerateTokenRequest{PeerName: "peerB", Datacenter: "dc1", Meta: map[string]string{"foo": "bar"}}
	resp, err := client.GenerateToken(ctx, &req)
	require.NoError(t, err)

	tokenJSON, err := base64.StdEncoding.DecodeString(resp.PeeringToken)
	require.NoError(t, err)

	token := &structs.PeeringToken{}
	require.NoError(t, json.Unmarshal(tokenJSON, token))
	require.Equal(t, "server.dc1.consul", token.ServerName)
	require.Len(t, token.ServerAddresses, 1)
	require.Equal(t, expectedAddr, token.ServerAddresses[0])
	require.Equal(t, []string{ca}, token.CA)

	require.NotEmpty(t, token.PeerID)
	_, err = uuid.ParseUUID(token.PeerID)
	require.NoError(t, err)

	_, peers, err := s.Server.FSM().State().PeeringList(nil, *structs.DefaultEnterpriseMetaInDefaultPartition())
	require.NoError(t, err)
	require.Len(t, peers, 1)

	peers[0].ModifyIndex = 0
	peers[0].CreateIndex = 0

	expect := &pbpeering.Peering{
		Name:      "peerB",
		Partition: acl.DefaultPartitionName,
		ID:        token.PeerID,
		State:     pbpeering.PeeringState_INITIAL,
		Meta:      map[string]string{"foo": "bar"},
	}
	require.Equal(t, expect, peers[0])
}

func TestPeeringService_Initiate(t *testing.T) {
	validToken := peering.TestPeeringToken("83474a06-cca4-4ff4-99a4-4152929c8160")
	validTokenJSON, _ := json.Marshal(&validToken)
	validTokenB64 := base64.StdEncoding.EncodeToString(validTokenJSON)

	// TODO(peering): see note on newTestServer, refactor to not use this
	s := newTestServer(t, nil)
	client := pbpeering.NewPeeringServiceClient(s.ClientConn(t))

	type testcase struct {
		name          string
		req           *pbpeering.InitiateRequest
		expectResp    *pbpeering.InitiateResponse
		expectPeering *pbpeering.Peering
		expectErr     string
	}
	run := func(t *testing.T, tc testcase) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		t.Cleanup(cancel)

		resp, err := client.Initiate(ctx, tc.req)
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
			req:       &pbpeering.InitiateRequest{PeerName: "--AA--"},
			expectErr: "--AA-- is not a valid peer name",
		},
		{
			name: "invalid token (base64)",
			req: &pbpeering.InitiateRequest{
				PeerName:     "peer1-usw1",
				PeeringToken: "+++/+++",
			},
			expectErr: "illegal base64 data",
		},
		{
			name: "invalid token (JSON)",
			req: &pbpeering.InitiateRequest{
				PeerName:     "peer1-usw1",
				PeeringToken: "Cg==", // base64 of "-"
			},
			expectErr: "unexpected end of JSON input",
		},
		{
			name: "invalid token (empty)",
			req: &pbpeering.InitiateRequest{
				PeerName:     "peer1-usw1",
				PeeringToken: "e30K", // base64 of "{}"
			},
			expectErr: "peering token CA value is empty",
		},
		{
			name: "too many meta tags",
			req: &pbpeering.InitiateRequest{
				PeerName:     "peer1-usw1",
				PeeringToken: validTokenB64,
				Meta:         generateTooManyMetaKeys(),
			},
			expectErr: "meta tags failed validation:",
		},
		{
			name: "success",
			req: &pbpeering.InitiateRequest{
				PeerName:     "peer1-usw1",
				PeeringToken: validTokenB64,
				Meta:         map[string]string{"foo": "bar"},
			},
			expectResp: &pbpeering.InitiateResponse{},
			expectPeering: peering.TestPeering(
				"peer1-usw1",
				pbpeering.PeeringState_INITIAL,
				map[string]string{"foo": "bar"},
			),
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
		Name:                "foo",
		State:               pbpeering.PeeringState_INITIAL,
		PeerCAPems:          nil,
		PeerServerName:      "test",
		PeerServerAddresses: []string{"addr1"},
	}
	err := s.Server.FSM().State().PeeringWrite(10, p)
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

func TestPeeringService_List(t *testing.T) {
	// TODO(peering): see note on newTestServer, refactor to not use this
	s := newTestServer(t, nil)

	// Insert peerings directly to state store.
	// Note that the state store holds reference to the underlying
	// variables; do not modify them after writing.
	foo := &pbpeering.Peering{
		Name:                "foo",
		State:               pbpeering.PeeringState_INITIAL,
		PeerCAPems:          nil,
		PeerServerName:      "fooservername",
		PeerServerAddresses: []string{"addr1"},
	}
	require.NoError(t, s.Server.FSM().State().PeeringWrite(10, foo))
	bar := &pbpeering.Peering{
		Name:                "bar",
		State:               pbpeering.PeeringState_ACTIVE,
		PeerCAPems:          nil,
		PeerServerName:      "barservername",
		PeerServerAddresses: []string{"addr1"},
	}
	require.NoError(t, s.Server.FSM().State().PeeringWrite(15, bar))

	client := pbpeering.NewPeeringServiceClient(s.ClientConn(t))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	resp, err := client.PeeringList(ctx, &pbpeering.PeeringListRequest{})
	require.NoError(t, err)

	expect := &pbpeering.PeeringListResponse{
		Peerings: []*pbpeering.Peering{bar, foo},
	}
	prototest.AssertDeepEqual(t, expect, resp)
}

// newTestServer is copied from partition/service_test.go, with the addition of certs/cas.
// TODO(peering): these are endpoint tests and should live in the agent/consul
// package. Instead, these can be written around a mock client (see testing.go)
// and a mock backend (future)
func newTestServer(t *testing.T, cb func(conf *consul.Config)) testingServer {
	t.Helper()
	conf := consul.DefaultConfig()
	dir := testutil.TempDir(t, "consul")

	ports := freeport.GetN(t, 3) // {rpc, serf_lan, serf_wan}

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
	server, err := consul.NewServer(conf, deps, gogrpc.NewServer())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, server.Shutdown())
	})

	testrpc.WaitForLeader(t, server.RPC, conf.Datacenter)

	backend := consul.NewPeeringBackend(server, deps.GRPCConnPool)
	handler := &peering.Service{Backend: backend}

	grpcServer := gogrpc.NewServer()
	pbpeering.RegisterPeeringServiceServer(grpcServer, handler)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { lis.Close() })

	g := new(errgroup.Group)
	g.Go(func() error {
		return grpcServer.Serve(lis)
	})
	t.Cleanup(func() {
		if grpcServer.Stop(); err != nil {
			t.Logf("grpc server shutdown: %v", err)
		}
		if err := g.Wait(); err != nil {
			t.Logf("grpc server error: %v", err)
		}
	})

	return testingServer{
		Server:  server,
		Backend: backend,
		Addr:    lis.Addr(),
	}
}

func (s testingServer) ClientConn(t *testing.T) *gogrpc.ClientConn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	conn, err := gogrpc.DialContext(ctx, s.Addr.String(), gogrpc.WithInsecure())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	return conn
}

type testingServer struct {
	Server  *consul.Server
	Addr    net.Addr
	Backend peering.Backend
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
	builder := resolver.NewServerResolverBuilder(resolver.Config{})
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

	return consul.Deps{
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
	}
}
