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
	gogrpc "google.golang.org/grpc"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	grpc "github.com/hashicorp/consul/agent/grpc-internal"
	"github.com/hashicorp/consul/agent/grpc-internal/resolver"
	"github.com/hashicorp/consul/agent/pool"
	"github.com/hashicorp/consul/agent/router"
	"github.com/hashicorp/consul/agent/rpc/middleware"
	"github.com/hashicorp/consul/agent/rpc/peering"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/proto/pbpeering"
	"github.com/hashicorp/consul/proto/prototest"
	"github.com/hashicorp/consul/sdk/freeport"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
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
	require.Equal(t, s.PublicGRPCAddr, token.ServerAddresses[0])
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
		State:     pbpeering.PeeringState_PENDING,
		Meta:      map[string]string{"foo": "bar"},
	}
	require.Equal(t, expect, peers[0])
}

func TestPeeringService_Establish(t *testing.T) {
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
		{
			name: "success",
			req: &pbpeering.EstablishRequest{
				PeerName:     "peer1-usw1",
				PeeringToken: validTokenB64,
				Meta:         map[string]string{"foo": "bar"},
			},
			expectResp: &pbpeering.EstablishResponse{},
			expectPeering: peering.TestPeering(
				"peer1-usw1",
				pbpeering.PeeringState_ESTABLISHING,
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
		ID:                   testUUID(t),
		Name:                 "foo",
		State:                pbpeering.PeeringState_ESTABLISHING,
		PeerCAPems:           nil,
		PeerServerName:       "test",
		PeerServerAddresses:  []string{"addr1"},
		ImportedServiceCount: 0,
		ExportedServiceCount: 0,
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

func TestPeeringService_Delete(t *testing.T) {
	// TODO(peering): see note on newTestServer, refactor to not use this
	s := newTestServer(t, nil)

	p := &pbpeering.Peering{
		ID:                  testUUID(t),
		Name:                "foo",
		State:               pbpeering.PeeringState_ESTABLISHING,
		PeerCAPems:          nil,
		PeerServerName:      "test",
		PeerServerAddresses: []string{"addr1"},
	}
	err := s.Server.FSM().State().PeeringWrite(10, p)
	require.NoError(t, err)
	require.Nil(t, p.DeletedAt)
	require.True(t, p.IsActive())

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
}

func TestPeeringService_List(t *testing.T) {
	// TODO(peering): see note on newTestServer, refactor to not use this
	s := newTestServer(t, nil)

	// Insert peerings directly to state store.
	// Note that the state store holds reference to the underlying
	// variables; do not modify them after writing.
	foo := &pbpeering.Peering{
		ID:                   testUUID(t),
		Name:                 "foo",
		State:                pbpeering.PeeringState_ESTABLISHING,
		PeerCAPems:           nil,
		PeerServerName:       "fooservername",
		PeerServerAddresses:  []string{"addr1"},
		ImportedServiceCount: 0,
		ExportedServiceCount: 0,
	}
	require.NoError(t, s.Server.FSM().State().PeeringWrite(10, foo))
	bar := &pbpeering.Peering{
		ID:                   testUUID(t),
		Name:                 "bar",
		State:                pbpeering.PeeringState_ACTIVE,
		PeerCAPems:           nil,
		PeerServerName:       "barservername",
		PeerServerAddresses:  []string{"addr1"},
		ImportedServiceCount: 0,
		ExportedServiceCount: 0,
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

func TestPeeringService_TrustBundleRead(t *testing.T) {
	srv := newTestServer(t, nil)
	store := srv.Server.FSM().State()
	client := pbpeering.NewPeeringServiceClient(srv.ClientConn(t))

	var lastIdx uint64 = 1
	_ = setupTestPeering(t, store, "my-peering", lastIdx)

	mysql := &structs.CheckServiceNode{
		Node: &structs.Node{
			Node:     "node1",
			Address:  "10.0.0.1",
			PeerName: "my-peering",
		},
		Service: &structs.NodeService{
			ID:       "mysql-1",
			Service:  "mysql",
			Port:     5000,
			PeerName: "my-peering",
		},
	}

	lastIdx++
	require.NoError(t, store.EnsureNode(lastIdx, mysql.Node))
	lastIdx++
	require.NoError(t, store.EnsureService(lastIdx, mysql.Node.Node, mysql.Service))

	bundle := &pbpeering.PeeringTrustBundle{
		TrustDomain: "peer1.com",
		PeerName:    "my-peering",
		RootPEMs:    []string{"peer1-root-1"},
	}
	lastIdx++
	require.NoError(t, store.PeeringTrustBundleWrite(lastIdx, bundle))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resp, err := client.TrustBundleRead(ctx, &pbpeering.TrustBundleReadRequest{
		Name: "my-peering",
	})
	require.NoError(t, err)
	require.Equal(t, lastIdx, resp.Index)
	require.NotNil(t, resp.Bundle)
	prototest.AssertDeepEqual(t, bundle, resp.Bundle)
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
	require.NoError(t, s.Server.FSM().State().PeeringWrite(lastIdx, &pbpeering.Peering{
		ID:                  testUUID(t),
		Name:                "foo",
		State:               pbpeering.PeeringState_ESTABLISHING,
		PeerServerName:      "test",
		PeerServerAddresses: []string{"addr1"},
	}))

	lastIdx++
	require.NoError(t, s.Server.FSM().State().PeeringWrite(lastIdx, &pbpeering.Peering{
		ID:                  testUUID(t),
		Name:                "bar",
		State:               pbpeering.PeeringState_ESTABLISHING,
		PeerServerName:      "test-bar",
		PeerServerAddresses: []string{"addr2"},
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
						PeerName: "foo",
					},
					{
						PeerName: "bar",
					},
				},
			},
			{
				Name: "web",
				Consumers: []structs.ServiceConsumer{
					{
						PeerName: "baz",
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

	req := pbpeering.TrustBundleListByServiceRequest{
		ServiceName: "api",
	}
	resp, err := client.TrustBundleListByService(context.Background(), &req)
	require.NoError(t, err)
	require.Len(t, resp.Bundles, 2)
	require.Equal(t, []string{"bar-root-1"}, resp.Bundles[0].RootPEMs)
	require.Equal(t, []string{"foo-root-1"}, resp.Bundles[1].RootPEMs)
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

	conf.GRPCPort = ports[3]

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

	externalGRPCServer := gogrpc.NewServer()

	deps := newDefaultDeps(t, conf)
	server, err := consul.NewServer(conf, deps, externalGRPCServer)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, server.Shutdown())
	})

	// Normally the gRPC server listener is created at the agent level and
	// passed down into the Server creation.
	grpcAddr := fmt.Sprintf("127.0.0.1:%d", conf.GRPCPort)

	ln, err := net.Listen("tcp", grpcAddr)
	require.NoError(t, err)
	go func() {
		_ = externalGRPCServer.Serve(ln)
	}()
	t.Cleanup(externalGRPCServer.Stop)

	testrpc.WaitForLeader(t, server.RPC, conf.Datacenter)

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
	}
}

func setupTestPeering(t *testing.T, store *state.Store, name string, index uint64) string {
	t.Helper()
	err := store.PeeringWrite(index, &pbpeering.Peering{
		ID:   testUUID(t),
		Name: name,
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

func noopForwardRPC(structs.RPCInfo, func(*gogrpc.ClientConn) error) (bool, error) {
	return false, nil
}
