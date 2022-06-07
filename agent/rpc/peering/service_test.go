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
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/agent/consul/state"
	grpc "github.com/hashicorp/consul/agent/grpc/private"
	"github.com/hashicorp/consul/agent/grpc/private/resolver"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/proto/pbservice"
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
			expectErr: "peering token server addresses value is empty",
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

func TestPeeringService_TrustBundleListByService(t *testing.T) {
	// test executes the following scenario:
	// 0 - initial setup test server, state store, RPC client, verify empty results
	// 1 - create a service, verify results still empty
	// 2 - create a peering, verify results still empty
	// 3 - create a config entry, verify results still empty
	// 4 - create trust bundles, verify bundles are returned
	// 5 - delete the config entry, verify results empty
	// 6 - restore config entry, verify bundles are returned
	// 7 - add peering, trust bundles, wildcard config entry, verify updated results are present
	// 8 - delete first config entry, verify bundles are returned
	// 9 - delete the service, verify results empty
	// Note: these steps are dependent on each other by design so that we can verify that
	// combinations of services, peerings, trust bundles, and config entries all affect results

	// fixed for the test
	nodeName := "test-node"

	// keep track of index across steps
	var lastIdx uint64

	// Create test server
	// TODO(peering): see note on newTestServer, refactor to not use this
	srv := newTestServer(t, nil)
	store := srv.Server.FSM().State()
	client := pbpeering.NewPeeringServiceClient(srv.ClientConn(t))

	// Create a node up-front so that we can assign services to it if needed
	svcNode := &structs.Node{Node: nodeName, Address: "127.0.0.1"}
	lastIdx++
	require.NoError(t, store.EnsureNode(lastIdx, svcNode))

	type testDeps struct {
		services []string
		peerings []*pbpeering.Peering
		entries  []*structs.ExportedServicesConfigEntry
		bundles  []*pbpeering.PeeringTrustBundle
	}

	setup := func(t *testing.T, idx uint64, deps testDeps) uint64 {
		// Create any services (and node)
		if len(deps.services) >= 0 {
			svcNode := &structs.Node{Node: nodeName, Address: "127.0.0.1"}
			idx++
			require.NoError(t, store.EnsureNode(idx, svcNode))

			// Create the test services
			for _, svc := range deps.services {
				idx++
				require.NoError(t, store.EnsureService(idx, svcNode.Node, &structs.NodeService{
					ID:      svc,
					Service: svc,
					Port:    int(8000 + idx),
				}))
			}
		}

		// Insert any peerings
		for _, peering := range deps.peerings {
			idx++
			require.NoError(t, store.PeeringWrite(idx, peering))

			// make sure it got created
			q := state.Query{Value: peering.Name}
			_, p, err := store.PeeringRead(nil, q)
			require.NoError(t, err)
			require.NotNil(t, p)
		}

		// Insert any trust bundles
		for _, bundle := range deps.bundles {
			idx++
			require.NoError(t, store.PeeringTrustBundleWrite(idx, bundle))

			q := state.Query{
				Value:          bundle.PeerName,
				EnterpriseMeta: *structs.NodeEnterpriseMetaInPartition(bundle.Partition),
			}
			gotIdx, ptb, err := store.PeeringTrustBundleRead(nil, q)
			require.NoError(t, err)
			require.NotNil(t, ptb)
			require.Equal(t, gotIdx, idx)
		}

		// Write any config entries
		for _, entry := range deps.entries {
			idx++
			require.NoError(t, store.EnsureConfigEntry(idx, entry))
		}

		return idx
	}

	type testCase struct {
		req       *pbpeering.TrustBundleListByServiceRequest
		expect    *pbpeering.TrustBundleListByServiceResponse
		expectErr string
	}

	// TODO(peering): see note on newTestServer, once we have a better server mock,
	// we should add functionality here to verify errors from backend
	verify := func(t *testing.T, tc *testCase) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		t.Cleanup(cancel)

		resp, err := client.TrustBundleListByService(ctx, tc.req)
		require.NoError(t, err)
		// ignore raft fields
		if resp.Bundles != nil {
			for _, b := range resp.Bundles {
				b.CreateIndex = 0
				b.ModifyIndex = 0
			}
		}
		prototest.AssertDeepEqual(t, tc.expect, resp)
	}

	// Execute scenario steps
	// ----------------------

	// 0 - initial empty state
	// -----------------------
	verify(t, &testCase{
		req: &pbpeering.TrustBundleListByServiceRequest{
			ServiceName: "foo",
		},
		expect: &pbpeering.TrustBundleListByServiceResponse{
			Bundles: nil,
		},
	})

	// 1 - create a service, verify results still empty
	// ------------------------------------------------
	lastIdx = setup(t, lastIdx, testDeps{services: []string{"foo"}})
	verify(t, &testCase{
		req: &pbpeering.TrustBundleListByServiceRequest{
			ServiceName: "foo",
		},
		expect: &pbpeering.TrustBundleListByServiceResponse{
			Bundles: []*pbpeering.PeeringTrustBundle{},
		},
	})

	// 2 - create a peering, verify results still empty
	// ------------------------------------------------
	lastIdx = setup(t, lastIdx, testDeps{
		peerings: []*pbpeering.Peering{
			{
				Name:                "peer1",
				State:               pbpeering.PeeringState_ACTIVE,
				PeerServerName:      "peer1-name",
				PeerServerAddresses: []string{"peer1-addr"},
			},
		},
	})
	verify(t, &testCase{
		req: &pbpeering.TrustBundleListByServiceRequest{
			ServiceName: "foo",
		},
		expect: &pbpeering.TrustBundleListByServiceResponse{
			Bundles: []*pbpeering.PeeringTrustBundle{},
		},
	})

	// 3 - create a config entry, verify results still empty
	// -----------------------------------------------------
	lastIdx = setup(t, lastIdx, testDeps{
		entries: []*structs.ExportedServicesConfigEntry{
			{
				Name: "export-foo",
				Services: []structs.ExportedService{
					{
						Name: "foo",
						Consumers: []structs.ServiceConsumer{
							{
								PeerName: "peer1",
							},
						},
					},
				},
			},
		},
	})
	verify(t, &testCase{
		req: &pbpeering.TrustBundleListByServiceRequest{
			ServiceName: "foo",
		},
		expect: &pbpeering.TrustBundleListByServiceResponse{
			Bundles: []*pbpeering.PeeringTrustBundle{},
		},
	})

	// 4 - create trust bundles, verify bundles are returned
	// -----------------------------------------------------
	lastIdx = setup(t, lastIdx, testDeps{
		bundles: []*pbpeering.PeeringTrustBundle{
			{
				TrustDomain: "peer1.com",
				PeerName:    "peer1",
				RootPEMs:    []string{"peer1-root-1"},
			},
		},
	})
	verify(t, &testCase{
		req: &pbpeering.TrustBundleListByServiceRequest{
			ServiceName: "foo",
		},
		expect: &pbpeering.TrustBundleListByServiceResponse{
			Bundles: []*pbpeering.PeeringTrustBundle{
				{
					TrustDomain: "peer1.com",
					PeerName:    "peer1",
					RootPEMs:    []string{"peer1-root-1"},
				},
			},
		},
	})

	// 5 - delete the config entry, verify results empty
	// -------------------------------------------------
	lastIdx++
	require.NoError(t, store.DeleteConfigEntry(lastIdx, structs.ExportedServices, "export-foo", nil))
	verify(t, &testCase{
		req: &pbpeering.TrustBundleListByServiceRequest{
			ServiceName: "foo",
		},
		expect: &pbpeering.TrustBundleListByServiceResponse{
			Bundles: []*pbpeering.PeeringTrustBundle{},
		},
	})

	// 6 - restore config entry, verify bundles are returned
	// -----------------------------------------------------
	lastIdx = setup(t, lastIdx, testDeps{
		entries: []*structs.ExportedServicesConfigEntry{
			{
				Name: "export-foo",
				Services: []structs.ExportedService{
					{
						Name: "foo",
						Consumers: []structs.ServiceConsumer{
							{PeerName: "peer1"},
						},
					},
				},
			},
		},
	})
	verify(t, &testCase{
		req: &pbpeering.TrustBundleListByServiceRequest{
			ServiceName: "foo",
		},
		expect: &pbpeering.TrustBundleListByServiceResponse{
			Bundles: []*pbpeering.PeeringTrustBundle{
				{
					TrustDomain: "peer1.com",
					PeerName:    "peer1",
					RootPEMs:    []string{"peer1-root-1"},
				},
			},
		},
	})

	// 7 - add peering, trust bundles, wildcard config entry, verify updated results are present
	// -----------------------------------------------------------------------------------------
	lastIdx = setup(t, lastIdx, testDeps{
		services: []string{"bar"},
		peerings: []*pbpeering.Peering{
			{
				Name:                "peer2",
				State:               pbpeering.PeeringState_ACTIVE,
				PeerServerName:      "peer2-name",
				PeerServerAddresses: []string{"peer2-addr"},
			},
		},
		entries: []*structs.ExportedServicesConfigEntry{
			{
				Name: "export-all",
				Services: []structs.ExportedService{
					{
						Name: structs.WildcardSpecifier,
						Consumers: []structs.ServiceConsumer{
							{PeerName: "peer1"},
							{PeerName: "peer2"},
						},
					},
				},
			},
		},
		bundles: []*pbpeering.PeeringTrustBundle{
			{
				TrustDomain: "peer2.com",
				PeerName:    "peer2",
				RootPEMs:    []string{"peer2-root-1"},
			},
		},
	})
	verify(t, &testCase{
		req: &pbpeering.TrustBundleListByServiceRequest{
			ServiceName: "foo",
		},
		expect: &pbpeering.TrustBundleListByServiceResponse{
			Bundles: []*pbpeering.PeeringTrustBundle{
				{
					TrustDomain: "peer1.com",
					PeerName:    "peer1",
					RootPEMs:    []string{"peer1-root-1"},
				},
				{
					TrustDomain: "peer2.com",
					PeerName:    "peer2",
					RootPEMs:    []string{"peer2-root-1"},
				},
			},
		},
	})

	// 8 - delete first config entry, verify bundles are returned
	lastIdx++
	require.NoError(t, store.DeleteConfigEntry(lastIdx, structs.ExportedServices, "export-foo", nil))
	verify(t, &testCase{
		req: &pbpeering.TrustBundleListByServiceRequest{
			ServiceName: "foo",
		},
		expect: &pbpeering.TrustBundleListByServiceResponse{
			Bundles: []*pbpeering.PeeringTrustBundle{
				{
					TrustDomain: "peer1.com",
					PeerName:    "peer1",
					RootPEMs:    []string{"peer1-root-1"},
				},
				{
					TrustDomain: "peer2.com",
					PeerName:    "peer2",
					RootPEMs:    []string{"peer2-root-1"},
				},
			},
		},
	})

	// 9 - delete the service, verify results empty
	lastIdx++
	require.NoError(t, store.DeleteService(lastIdx, nodeName, "foo", nil, ""))
	verify(t, &testCase{
		req: &pbpeering.TrustBundleListByServiceRequest{
			ServiceName: "foo",
		},
		expect: &pbpeering.TrustBundleListByServiceResponse{
			Bundles: []*pbpeering.PeeringTrustBundle{},
		},
	})
}

func Test_StreamHandler_UpsertServices(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	type testCase struct {
		name   string
		msg    *pbpeering.ReplicationMessage_Response
		input  structs.CheckServiceNodes
		expect structs.CheckServiceNodes
	}

	s := newTestServer(t, nil)
	testrpc.WaitForLeader(t, s.Server.RPC, "dc1")
	testrpc.WaitForActiveCARoot(t, s.Server.RPC, "dc1", nil)

	srv := peering.NewService(
		testutil.Logger(t),
		peering.Config{
			Datacenter:     "dc1",
			ConnectEnabled: true,
		},
		consul.NewPeeringBackend(s.Server, nil),
	)

	require.NoError(t, s.Server.FSM().State().PeeringWrite(0, &pbpeering.Peering{
		Name: "my-peer",
	}))

	_, p, err := s.Server.FSM().State().PeeringRead(nil, state.Query{Value: "my-peer"})
	require.NoError(t, err)

	client := peering.NewMockClient(context.Background())

	errCh := make(chan error, 1)
	client.ErrCh = errCh

	go func() {
		// Pass errors from server handler into ErrCh so that they can be seen by the client on Recv().
		// This matches gRPC's behavior when an error is returned by a server.
		err := srv.StreamResources(client.ReplicationStream)
		if err != nil {
			errCh <- err
		}
	}()

	sub := &pbpeering.ReplicationMessage{
		Payload: &pbpeering.ReplicationMessage_Request_{
			Request: &pbpeering.ReplicationMessage_Request{
				PeerID:      p.ID,
				ResourceURL: pbpeering.TypeURLService,
			},
		},
	}
	require.NoError(t, client.Send(sub))

	// Receive subscription request from peer for our services
	_, err = client.Recv()
	require.NoError(t, err)

	// Receive first roots replication message
	receiveRoots, err := client.Recv()
	require.NoError(t, err)
	require.NotNil(t, receiveRoots.GetResponse())
	require.Equal(t, pbpeering.TypeURLRoots, receiveRoots.GetResponse().ResourceURL)

	remoteEntMeta := structs.DefaultEnterpriseMetaInPartition("remote-partition")
	localEntMeta := acl.DefaultEnterpriseMeta()
	localPeerName := "my-peer"

	// Scrub data we don't need for the assertions below.
	scrubCheckServiceNodes := func(instances structs.CheckServiceNodes) {
		for _, csn := range instances {
			csn.Node.RaftIndex = structs.RaftIndex{}

			csn.Service.TaggedAddresses = nil
			csn.Service.Weights = nil
			csn.Service.RaftIndex = structs.RaftIndex{}
			csn.Service.Proxy = structs.ConnectProxyConfig{}

			for _, c := range csn.Checks {
				c.RaftIndex = structs.RaftIndex{}
				c.Definition = structs.HealthCheckDefinition{}
			}
		}
	}

	run := func(t *testing.T, tc testCase) {
		pbCSN := &pbservice.IndexedCheckServiceNodes{}
		for _, csn := range tc.input {
			pbCSN.Nodes = append(pbCSN.Nodes, pbservice.NewCheckServiceNodeFromStructs(&csn))
		}

		any, err := anypb.New(pbCSN)
		require.NoError(t, err)
		tc.msg.Resource = any

		resp := &pbpeering.ReplicationMessage{
			Payload: &pbpeering.ReplicationMessage_Response_{
				Response: tc.msg,
			},
		}
		require.NoError(t, client.Send(resp))

		msg, err := client.RecvWithTimeout(1 * time.Second)
		require.NoError(t, err)

		req := msg.GetRequest()
		require.NotNil(t, req)
		require.Equal(t, tc.msg.Nonce, req.Nonce)
		require.Nil(t, req.Error)

		_, got, err := s.Server.FSM().State().CombinedCheckServiceNodes(nil, structs.NewServiceName("api", nil), localPeerName)
		require.NoError(t, err)

		scrubCheckServiceNodes(got)
		require.Equal(t, tc.expect, got)
	}

	// NOTE: These test cases do not run against independent state stores, they show sequential updates for a given service.
	//       Every new upsert must replace the data from the previous case.
	tt := []testCase{
		{
			name: "upsert an instance on a node",
			msg: &pbpeering.ReplicationMessage_Response{
				ResourceURL: pbpeering.TypeURLService,
				ResourceID:  "api",
				Nonce:       "1",
				Operation:   pbpeering.ReplicationMessage_Response_UPSERT,
			},
			input: structs.CheckServiceNodes{
				{
					Node: &structs.Node{
						ID:         "112e2243-ab62-4e8a-9317-63306972183c",
						Node:       "node-1",
						Address:    "10.0.0.1",
						Datacenter: "dc1",
						Partition:  remoteEntMeta.PartitionOrEmpty(),
					},
					Service: &structs.NodeService{
						Kind:           "",
						ID:             "api-1",
						Service:        "api",
						Port:           8080,
						EnterpriseMeta: *remoteEntMeta,
					},
					Checks: []*structs.HealthCheck{
						{
							CheckID:        "node-1-check",
							Node:           "node-1",
							Status:         api.HealthPassing,
							EnterpriseMeta: *remoteEntMeta,
						},
						{
							CheckID:        "api-1-check",
							ServiceID:      "api-1",
							ServiceName:    "api",
							Node:           "node-1",
							Status:         api.HealthCritical,
							EnterpriseMeta: *remoteEntMeta,
						},
					},
				},
			},
			expect: structs.CheckServiceNodes{
				{
					Node: &structs.Node{
						ID:         "112e2243-ab62-4e8a-9317-63306972183c",
						Node:       "node-1",
						Address:    "10.0.0.1",
						Datacenter: "dc1",
						Partition:  localEntMeta.PartitionOrEmpty(),
						PeerName:   localPeerName,
					},
					Service: &structs.NodeService{
						Kind:           "",
						ID:             "api-1",
						Service:        "api",
						Port:           8080,
						EnterpriseMeta: *localEntMeta,
						PeerName:       localPeerName,
					},
					Checks: []*structs.HealthCheck{
						{
							CheckID:        "node-1-check",
							Node:           "node-1",
							Status:         api.HealthPassing,
							EnterpriseMeta: *localEntMeta,
							PeerName:       localPeerName,
						},
						{
							CheckID:        "api-1-check",
							ServiceID:      "api-1",
							ServiceName:    "api",
							Node:           "node-1",
							Status:         api.HealthCritical,
							EnterpriseMeta: *localEntMeta,
							PeerName:       localPeerName,
						},
					},
				},
			},
		},
		{
			name: "upsert two instances on the same node",
			msg: &pbpeering.ReplicationMessage_Response{
				ResourceURL: pbpeering.TypeURLService,
				ResourceID:  "api",
				Nonce:       "2",
				Operation:   pbpeering.ReplicationMessage_Response_UPSERT,
			},
			input: structs.CheckServiceNodes{
				{
					Node: &structs.Node{
						ID:         "112e2243-ab62-4e8a-9317-63306972183c",
						Node:       "node-1",
						Address:    "10.0.0.1",
						Datacenter: "dc1",
						Partition:  remoteEntMeta.PartitionOrEmpty(),
					},
					Service: &structs.NodeService{
						Kind:           "",
						ID:             "api-1",
						Service:        "api",
						Port:           8080,
						EnterpriseMeta: *remoteEntMeta,
					},
					Checks: []*structs.HealthCheck{
						{
							CheckID:        "node-1-check",
							Node:           "node-1",
							Status:         api.HealthPassing,
							EnterpriseMeta: *remoteEntMeta,
						},
						{
							CheckID:        "api-1-check",
							ServiceID:      "api-1",
							ServiceName:    "api",
							Node:           "node-1",
							Status:         api.HealthCritical,
							EnterpriseMeta: *remoteEntMeta,
						},
					},
				},
				{
					Node: &structs.Node{
						ID:         "112e2243-ab62-4e8a-9317-63306972183c",
						Node:       "node-1",
						Address:    "10.0.0.1",
						Datacenter: "dc1",
						Partition:  remoteEntMeta.PartitionOrEmpty(),
					},
					Service: &structs.NodeService{
						Kind:           "",
						ID:             "api-2",
						Service:        "api",
						Port:           9090,
						EnterpriseMeta: *remoteEntMeta,
					},
					Checks: []*structs.HealthCheck{
						{
							CheckID:        "node-1-check",
							Node:           "node-1",
							Status:         api.HealthPassing,
							EnterpriseMeta: *remoteEntMeta,
						},
						{
							CheckID:        "api-2-check",
							ServiceID:      "api-2",
							ServiceName:    "api",
							Node:           "node-1",
							Status:         api.HealthWarning,
							EnterpriseMeta: *remoteEntMeta,
						},
					},
				},
			},
			expect: structs.CheckServiceNodes{
				{
					Node: &structs.Node{
						ID:         "112e2243-ab62-4e8a-9317-63306972183c",
						Node:       "node-1",
						Address:    "10.0.0.1",
						Datacenter: "dc1",
						Partition:  localEntMeta.PartitionOrEmpty(),
						PeerName:   localPeerName,
					},
					Service: &structs.NodeService{
						Kind:           "",
						ID:             "api-1",
						Service:        "api",
						Port:           8080,
						EnterpriseMeta: *localEntMeta,
						PeerName:       localPeerName,
					},
					Checks: []*structs.HealthCheck{
						{
							CheckID:        "node-1-check",
							Node:           "node-1",
							Status:         api.HealthPassing,
							EnterpriseMeta: *localEntMeta,
							PeerName:       localPeerName,
						},
						{
							CheckID:        "api-1-check",
							ServiceID:      "api-1",
							ServiceName:    "api",
							Node:           "node-1",
							Status:         api.HealthCritical,
							EnterpriseMeta: *localEntMeta,
							PeerName:       localPeerName,
						},
					},
				},
				{
					Node: &structs.Node{
						ID:         "112e2243-ab62-4e8a-9317-63306972183c",
						Node:       "node-1",
						Address:    "10.0.0.1",
						Datacenter: "dc1",
						Partition:  localEntMeta.PartitionOrEmpty(),
						PeerName:   localPeerName,
					},
					Service: &structs.NodeService{
						Kind:           "",
						ID:             "api-2",
						Service:        "api",
						Port:           9090,
						EnterpriseMeta: *localEntMeta,
						PeerName:       localPeerName,
					},
					Checks: []*structs.HealthCheck{
						{
							CheckID:        "node-1-check",
							Node:           "node-1",
							Status:         api.HealthPassing,
							EnterpriseMeta: *localEntMeta,
							PeerName:       localPeerName,
						},
						{
							CheckID:        "api-2-check",
							ServiceID:      "api-2",
							ServiceName:    "api",
							Node:           "node-1",
							Status:         api.HealthWarning,
							EnterpriseMeta: *localEntMeta,
							PeerName:       localPeerName,
						},
					},
				},
			},
		},
	}
	for _, tc := range tt {
		testutil.RunStep(t, tc.name, func(t *testing.T) {
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

	conf.PrimaryDatacenter = "dc1"
	conf.ConnectEnabled = true

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

func setupTestPeering(t *testing.T, store *state.Store, name string, index uint64) string {
	err := store.PeeringWrite(index, &pbpeering.Peering{
		Name: name,
	})
	require.NoError(t, err)

	_, p, err := store.PeeringRead(nil, state.Query{Value: name})
	require.NoError(t, err)
	require.NotNil(t, p)

	return p.ID
}
