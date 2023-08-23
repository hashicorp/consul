package dataplane

import (
	"context"
	"testing"

	"github.com/hashicorp/go-hclog"
	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	acl "github.com/hashicorp/consul/acl"
	resolver "github.com/hashicorp/consul/acl/resolver"
	external "github.com/hashicorp/consul/agent/grpc-external"
	"github.com/hashicorp/consul/agent/grpc-external/testutils"
	structs "github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto-public/pbdataplane"
	"github.com/hashicorp/consul/types"
)

const (
	testToken        = "acl-token-get-envoy-bootstrap-params"
	proxyServiceID   = "web-proxy"
	nodeName         = "foo"
	nodeID           = "2980b72b-bd9d-9d7b-d4f9-951bf7508d95"
	proxyConfigKey   = "envoy_dogstatsd_url"
	proxyConfigValue = "udp://127.0.0.1:8125"
	serverDC         = "dc1"
)

func testRegisterRequestProxy(t *testing.T) *structs.RegisterRequest {
	return &structs.RegisterRequest{
		Datacenter: serverDC,
		Node:       nodeName,
		ID:         types.NodeID(nodeID),
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			Kind:    structs.ServiceKindConnectProxy,
			Service: proxyServiceID,
			ID:      proxyServiceID,
			Address: "127.0.0.2",
			Port:    2222,
			Proxy: structs.ConnectProxyConfig{
				DestinationServiceName: "web",
				Config: map[string]interface{}{
					proxyConfigKey: proxyConfigValue,
				},
			},
			EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
		},
	}
}

func testRegisterIngressGateway(t *testing.T) *structs.RegisterRequest {
	registerReq := structs.TestRegisterIngressGateway(t)
	registerReq.ID = types.NodeID("2980b72b-bd9d-9d7b-d4f9-951bf7508d95")
	registerReq.Service.ID = registerReq.Service.Service
	registerReq.Service.Proxy.Config = map[string]interface{}{
		proxyConfigKey: proxyConfigValue,
	}
	return registerReq
}

func TestGetEnvoyBootstrapParams_Success(t *testing.T) {
	type testCase struct {
		name        string
		registerReq *structs.RegisterRequest
		nodeID      bool
	}

	run := func(t *testing.T, tc testCase) {
		store := testutils.TestStateStore(t, nil)
		err := store.EnsureRegistration(1, tc.registerReq)
		require.NoError(t, err)

		aclResolver := &MockACLResolver{}
		aclResolver.On("ResolveTokenAndDefaultMeta", testToken, mock.Anything, mock.Anything).
			Return(testutils.TestAuthorizerServiceRead(t, tc.registerReq.Service.ID), nil)
		ctx := external.ContextWithToken(context.Background(), testToken)

		server := NewServer(Config{
			GetStore:    func() StateStore { return store },
			Logger:      hclog.NewNullLogger(),
			ACLResolver: aclResolver,
			Datacenter:  serverDC,
		})
		client := testClient(t, server)

		req := &pbdataplane.GetEnvoyBootstrapParamsRequest{
			ServiceId: tc.registerReq.Service.ID,
			NodeSpec:  &pbdataplane.GetEnvoyBootstrapParamsRequest_NodeName{NodeName: tc.registerReq.Node}}
		if tc.nodeID {
			req.NodeSpec = &pbdataplane.GetEnvoyBootstrapParamsRequest_NodeId{NodeId: string(tc.registerReq.ID)}
		}
		resp, err := client.GetEnvoyBootstrapParams(ctx, req)
		require.NoError(t, err)

		require.Equal(t, tc.registerReq.Service.Proxy.DestinationServiceName, resp.Service)
		require.Equal(t, serverDC, resp.Datacenter)
		require.Equal(t, tc.registerReq.EnterpriseMeta.PartitionOrDefault(), resp.Partition)
		require.Equal(t, tc.registerReq.EnterpriseMeta.NamespaceOrDefault(), resp.Namespace)
		require.Contains(t, resp.Config.Fields, proxyConfigKey)
		require.Equal(t, structpb.NewStringValue(proxyConfigValue), resp.Config.Fields[proxyConfigKey])
		require.Equal(t, convertToResponseServiceKind(tc.registerReq.Service.Kind), resp.ServiceKind)

	}

	testCases := []testCase{
		{
			name:        "lookup service side car proxy by node name",
			registerReq: testRegisterRequestProxy(t),
		},
		{
			name:        "lookup service side car proxy by node ID",
			registerReq: testRegisterRequestProxy(t),
			nodeID:      true,
		},
		{
			name:        "lookup ingress gw service by node name",
			registerReq: testRegisterIngressGateway(t),
		},
		{
			name:        "lookup ingress gw service by node ID",
			registerReq: testRegisterIngressGateway(t),
			nodeID:      true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestGetEnvoyBootstrapParams_Error(t *testing.T) {
	type testCase struct {
		name            string
		req             *pbdataplane.GetEnvoyBootstrapParamsRequest
		expectedErrCode codes.Code
		expecteErrMsg   string
	}

	run := func(t *testing.T, tc testCase) {
		aclResolver := &MockACLResolver{}

		aclResolver.On("ResolveTokenAndDefaultMeta", testToken, mock.Anything, mock.Anything).
			Return(testutils.TestAuthorizerServiceRead(t, proxyServiceID), nil)
		ctx := external.ContextWithToken(context.Background(), testToken)

		store := testutils.TestStateStore(t, nil)
		registerReq := testRegisterRequestProxy(t)
		err := store.EnsureRegistration(1, registerReq)
		require.NoError(t, err)

		server := NewServer(Config{
			GetStore:    func() StateStore { return store },
			Logger:      hclog.NewNullLogger(),
			ACLResolver: aclResolver,
		})
		client := testClient(t, server)

		resp, err := client.GetEnvoyBootstrapParams(ctx, tc.req)
		require.Nil(t, resp)
		require.Error(t, err)
		errStatus, ok := status.FromError(err)
		require.True(t, ok)
		require.Equal(t, tc.expectedErrCode.String(), errStatus.Code().String())
		require.Equal(t, tc.expecteErrMsg, errStatus.Message())
	}

	testCases := []testCase{
		{
			name: "lookup-service-by-unregistered-node-name",
			req: &pbdataplane.GetEnvoyBootstrapParamsRequest{
				ServiceId: proxyServiceID,
				NodeSpec:  &pbdataplane.GetEnvoyBootstrapParamsRequest_NodeName{NodeName: "blah"}},
			expectedErrCode: codes.NotFound,
			expecteErrMsg:   "node not found",
		},
		{
			name: "lookup-service-by-unregistered-node-id",
			req: &pbdataplane.GetEnvoyBootstrapParamsRequest{
				ServiceId: proxyServiceID,
				NodeSpec:  &pbdataplane.GetEnvoyBootstrapParamsRequest_NodeId{NodeId: "5980b72b-bd9d-9d7b-d4f9-951bf7508d98"}},
			expectedErrCode: codes.NotFound,
			expecteErrMsg:   "node not found",
		},
		{
			name: "lookup-service-by-unregistered-service",
			req: &pbdataplane.GetEnvoyBootstrapParamsRequest{
				ServiceId: "blah-service",
				NodeSpec:  &pbdataplane.GetEnvoyBootstrapParamsRequest_NodeName{NodeName: nodeName}},
			expectedErrCode: codes.NotFound,
			expecteErrMsg:   "Service not found",
		},
		{
			name: "lookup-service-without-node-details",
			req: &pbdataplane.GetEnvoyBootstrapParamsRequest{
				ServiceId: proxyServiceID},
			expectedErrCode: codes.InvalidArgument,
			expecteErrMsg:   "Node ID or name required to lookup the service",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}

}

func TestGetEnvoyBootstrapParams_Unauthenticated(t *testing.T) {
	// Mock the ACL resolver to return ErrNotFound.
	aclResolver := &MockACLResolver{}
	aclResolver.On("ResolveTokenAndDefaultMeta", mock.Anything, mock.Anything, mock.Anything).
		Return(resolver.Result{}, acl.ErrNotFound)
	ctx := external.ContextWithToken(context.Background(), testToken)
	store := testutils.TestStateStore(t, nil)
	server := NewServer(Config{
		GetStore:    func() StateStore { return store },
		Logger:      hclog.NewNullLogger(),
		ACLResolver: aclResolver,
	})
	client := testClient(t, server)
	resp, err := client.GetEnvoyBootstrapParams(ctx, &pbdataplane.GetEnvoyBootstrapParamsRequest{})
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated.String(), status.Code(err).String())
	require.Nil(t, resp)
}

func TestGetEnvoyBootstrapParams_PermissionDenied(t *testing.T) {
	// Mock the ACL resolver to return a deny all authorizer
	aclResolver := &MockACLResolver{}
	aclResolver.On("ResolveTokenAndDefaultMeta", testToken, mock.Anything, mock.Anything).
		Return(testutils.TestAuthorizerDenyAll(t), nil)
	ctx := external.ContextWithToken(context.Background(), testToken)
	store := testutils.TestStateStore(t, nil)
	registerReq := structs.TestRegisterRequestProxy(t)
	proxyServiceID := "web-sidecar-proxy"
	registerReq.Service.ID = proxyServiceID
	err := store.EnsureRegistration(1, registerReq)
	require.NoError(t, err)

	server := NewServer(Config{
		GetStore:    func() StateStore { return store },
		Logger:      hclog.NewNullLogger(),
		ACLResolver: aclResolver,
	})
	client := testClient(t, server)
	req := &pbdataplane.GetEnvoyBootstrapParamsRequest{
		ServiceId: proxyServiceID,
		NodeSpec:  &pbdataplane.GetEnvoyBootstrapParamsRequest_NodeName{NodeName: registerReq.Node}}

	resp, err := client.GetEnvoyBootstrapParams(ctx, req)
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied.String(), status.Code(err).String())
	require.Nil(t, resp)
}
