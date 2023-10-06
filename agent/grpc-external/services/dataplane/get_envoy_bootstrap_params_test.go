// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dataplane

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/mesh"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/acl/resolver"
	external "github.com/hashicorp/consul/agent/grpc-external"
	"github.com/hashicorp/consul/agent/grpc-external/testutils"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto-public/pbdataplane"
)

const (
	testIdentity     = "test-identity"
	testToken        = "acl-token-get-envoy-bootstrap-params"
	testServiceName  = "web"
	proxyServiceID   = "web-proxy"
	nodeName         = "foo"
	nodeID           = "2980b72b-bd9d-9d7b-d4f9-951bf7508d95"
	proxyConfigKey   = "envoy_dogstatsd_url"
	proxyConfigValue = "udp://127.0.0.1:8125"
	serverDC         = "dc1"

	protocolKey       = "protocol"
	connectTimeoutKey = "local_connect_timeout_ms"
	requestTimeoutKey = "local_request_timeout_ms"

	proxyDefaultsProtocol         = "http"
	proxyDefaultsRequestTimeout   = 1111
	serviceDefaultsProtocol       = "tcp"
	serviceDefaultsConnectTimeout = 4444

	testAccessLogs = "{\"name\":\"Consul Listener Filter Log\",\"typedConfig\":{\"@type\":\"type.googleapis.com/envoy.extensions.access_loggers.stream.v3.StdoutAccessLog\",\"logFormat\":{\"jsonFormat\":{\"custom_field\":\"%START_TIME%\"}}}}"
)

func testRegisterRequestProxy(t *testing.T) *structs.RegisterRequest {
	return &structs.RegisterRequest{
		Datacenter: serverDC,
		Node:       nodeName,
		ID:         nodeID,
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			Kind:    structs.ServiceKindConnectProxy,
			Service: proxyServiceID,
			ID:      proxyServiceID,
			Address: "127.0.0.2",
			Port:    2222,
			Proxy: structs.ConnectProxyConfig{
				DestinationServiceName: testServiceName,
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
	registerReq.ID = "2980b72b-bd9d-9d7b-d4f9-951bf7508d95"
	registerReq.Service.ID = registerReq.Service.Service
	registerReq.Service.Proxy.Config = map[string]interface{}{
		proxyConfigKey: proxyConfigValue,
	}
	return registerReq
}

func testProxyDefaults(t *testing.T, accesslogs bool) structs.ConfigEntry {
	pd := &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: structs.ProxyConfigGlobal,
		Config: map[string]interface{}{
			protocolKey:       proxyDefaultsProtocol,
			requestTimeoutKey: proxyDefaultsRequestTimeout,
		},
	}
	if accesslogs {
		pd.AccessLogs.Enabled = true
		pd.AccessLogs.JSONFormat = "{ \"custom_field\": \"%START_TIME%\" }"
	}
	return pd
}

func testServiceDefaults(t *testing.T) structs.ConfigEntry {
	return &structs.ServiceConfigEntry{
		Kind:                  structs.ServiceDefaults,
		Name:                  testServiceName,
		Protocol:              serviceDefaultsProtocol,
		LocalConnectTimeoutMs: serviceDefaultsConnectTimeout,
	}
}

func requireConfigField(t *testing.T, resp *pbdataplane.GetEnvoyBootstrapParamsResponse, key string, value interface{}) {
	require.Contains(t, resp.Config.Fields, key)
	require.Equal(t, value, resp.Config.Fields[key])
}

func TestGetEnvoyBootstrapParams_Success(t *testing.T) {
	type testCase struct {
		name            string
		registerReq     *structs.RegisterRequest
		nodeID          bool
		proxyDefaults   structs.ConfigEntry
		serviceDefaults structs.ConfigEntry
	}

	run := func(t *testing.T, tc testCase) {
		store := testutils.TestStateStore(t, nil)
		idx := uint64(1)
		err := store.EnsureRegistration(idx, tc.registerReq)
		require.NoError(t, err)

		if tc.proxyDefaults != nil {
			idx++
			err := store.EnsureConfigEntry(idx, tc.proxyDefaults)
			require.NoError(t, err)
		}
		if tc.serviceDefaults != nil {
			idx++
			err := store.EnsureConfigEntry(idx, tc.serviceDefaults)
			require.NoError(t, err)
		}

		aclResolver := &MockACLResolver{}
		aclResolver.On("ResolveTokenAndDefaultMeta", testToken, mock.Anything, mock.Anything).
			Return(testutils.ACLServiceRead(t, tc.registerReq.Service.ID), nil)

		options := structs.QueryOptions{Token: testToken}
		ctx, err := external.ContextWithQueryOptions(context.Background(), options)
		require.NoError(t, err)

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

		if tc.registerReq.Service.IsGateway() {
			require.Equal(t, tc.registerReq.Service.Service, resp.Service)
		} else {
			require.Equal(t, tc.registerReq.Service.Proxy.DestinationServiceName, resp.Service)
		}

		require.Equal(t, serverDC, resp.Datacenter)
		require.Equal(t, tc.registerReq.EnterpriseMeta.PartitionOrDefault(), resp.Partition)
		require.Equal(t, tc.registerReq.EnterpriseMeta.NamespaceOrDefault(), resp.Namespace)
		requireConfigField(t, resp, proxyConfigKey, structpb.NewStringValue(proxyConfigValue))
		require.Equal(t, tc.registerReq.Node, resp.NodeName)

		if tc.serviceDefaults != nil && tc.proxyDefaults != nil {
			// service-defaults take precedence over proxy-defaults
			requireConfigField(t, resp, protocolKey, structpb.NewStringValue(serviceDefaultsProtocol))
			requireConfigField(t, resp, connectTimeoutKey, structpb.NewNumberValue(serviceDefaultsConnectTimeout))
			requireConfigField(t, resp, requestTimeoutKey, structpb.NewNumberValue(proxyDefaultsRequestTimeout))
		} else if tc.serviceDefaults != nil {
			requireConfigField(t, resp, protocolKey, structpb.NewStringValue(serviceDefaultsProtocol))
			requireConfigField(t, resp, connectTimeoutKey, structpb.NewNumberValue(serviceDefaultsConnectTimeout))
		} else if tc.proxyDefaults != nil {
			requireConfigField(t, resp, protocolKey, structpb.NewStringValue(proxyDefaultsProtocol))
			requireConfigField(t, resp, requestTimeoutKey, structpb.NewNumberValue(proxyDefaultsRequestTimeout))
		}

		if tc.proxyDefaults != nil {
			pd, ok := tc.proxyDefaults.(*structs.ProxyConfigEntry)
			require.True(t, ok, "Invalid Proxy Defaults")
			if pd.AccessLogs.Enabled {
				require.JSONEq(t, "{\"name\":\"Consul Listener Filter Log\",\"typedConfig\":{\"@type\":\"type.googleapis.com/envoy.extensions.access_loggers.stream.v3.StdoutAccessLog\",\"logFormat\":{\"jsonFormat\":{\"custom_field\":\"%START_TIME%\"}}}}", resp.AccessLogs[0])
			}
		}

	}

	testCases := []testCase{
		{
			name:        "lookup service sidecar proxy by node name",
			registerReq: testRegisterRequestProxy(t),
		},
		{
			name:        "lookup service sidecar proxy by node ID",
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
		{
			name:          "merge proxy defaults for sidecar proxy",
			registerReq:   testRegisterRequestProxy(t),
			proxyDefaults: testProxyDefaults(t, false),
		},
		{
			name:          "proxy defaults access logs",
			registerReq:   testRegisterRequestProxy(t),
			proxyDefaults: testProxyDefaults(t, true),
		},
		{
			name:            "merge service defaults for sidecar proxy",
			registerReq:     testRegisterRequestProxy(t),
			serviceDefaults: testServiceDefaults(t),
		},
		{
			name:            "merge proxy defaults and service defaults for sidecar proxy",
			registerReq:     testRegisterRequestProxy(t),
			serviceDefaults: testServiceDefaults(t),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestGetEnvoyBootstrapParams_Success_EnableV2(t *testing.T) {
	type testCase struct {
		name            string
		workloadData    *pbcatalog.Workload
		proxyCfgs       []*pbmesh.ProxyConfiguration
		expBootstrapCfg *pbmesh.BootstrapConfig
		expAccessLogs   string
	}

	run := func(t *testing.T, tc testCase) {
		resourceClient := svctest.RunResourceService(t, catalog.RegisterTypes, mesh.RegisterTypes)

		options := structs.QueryOptions{Token: testToken}
		ctx, err := external.ContextWithQueryOptions(context.Background(), options)
		require.NoError(t, err)

		aclResolver := &MockACLResolver{}

		server := NewServer(Config{
			Logger:            hclog.NewNullLogger(),
			ACLResolver:       aclResolver,
			Datacenter:        serverDC,
			EnableV2:          true,
			ResourceAPIClient: resourceClient,
		})
		client := testClient(t, server)

		// Add required fields to workload data.
		tc.workloadData.Addresses = []*pbcatalog.WorkloadAddress{
			{
				Host: "127.0.0.1",
			},
		}
		tc.workloadData.Ports = map[string]*pbcatalog.WorkloadPort{
			"tcp": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
		}
		workloadResource := resourcetest.Resource(pbcatalog.WorkloadType, "test-workload").
			WithData(t, tc.workloadData).
			WithTenancy(resource.DefaultNamespacedTenancy()).
			Write(t, resourceClient)

		// Create any proxy cfg resources.
		for i, cfg := range tc.proxyCfgs {
			resourcetest.Resource(pbmesh.ProxyConfigurationType, fmt.Sprintf("proxy-cfg-%d", i)).
				WithData(t, cfg).
				WithTenancy(resource.DefaultNamespacedTenancy()).
				Write(t, resourceClient)
		}

		req := &pbdataplane.GetEnvoyBootstrapParamsRequest{
			ProxyId:   workloadResource.Id.Name,
			Namespace: workloadResource.Id.Tenancy.Namespace,
			Partition: workloadResource.Id.Tenancy.Partition,
		}

		aclResolver.On("ResolveTokenAndDefaultMeta", testToken, mock.Anything, mock.Anything).
			Return(testutils.ACLUseProvidedPolicy(t,
				&acl.Policy{
					PolicyRules: acl.PolicyRules{
						Services: []*acl.ServiceRule{
							{
								Name:   workloadResource.Id.Name,
								Policy: acl.PolicyRead,
							},
						},
						Identities: []*acl.IdentityRule{
							{
								Name:   testIdentity,
								Policy: acl.PolicyWrite,
							},
						},
					},
				}), nil)

		resp, err := client.GetEnvoyBootstrapParams(ctx, req)
		require.NoError(t, err)

		require.Equal(t, tc.workloadData.Identity, resp.Identity)
		require.Equal(t, serverDC, resp.Datacenter)
		require.Equal(t, workloadResource.Id.Tenancy.Partition, resp.Partition)
		require.Equal(t, workloadResource.Id.Tenancy.Namespace, resp.Namespace)
		require.Equal(t, resp.NodeName, tc.workloadData.NodeName)
		prototest.AssertDeepEqual(t, tc.expBootstrapCfg, resp.BootstrapConfig)
		if tc.expAccessLogs != "" {
			require.JSONEq(t, tc.expAccessLogs, resp.AccessLogs[0])
		}
	}

	testCases := []testCase{
		{
			name: "workload without node",
			workloadData: &pbcatalog.Workload{
				Identity: testIdentity,
			},
			expBootstrapCfg: &pbmesh.BootstrapConfig{},
		},
		{
			name: "workload with node",
			workloadData: &pbcatalog.Workload{
				Identity: testIdentity,
				NodeName: "test-node",
			},
			expBootstrapCfg: &pbmesh.BootstrapConfig{},
		},
		{
			name: "single proxy configuration",
			workloadData: &pbcatalog.Workload{
				Identity: testIdentity,
			},
			proxyCfgs: []*pbmesh.ProxyConfiguration{
				{
					Workloads: &pbcatalog.WorkloadSelector{Names: []string{"test-workload"}},
					BootstrapConfig: &pbmesh.BootstrapConfig{
						DogstatsdUrl: "dogstats-url",
					},
				},
			},
			expBootstrapCfg: &pbmesh.BootstrapConfig{
				DogstatsdUrl: "dogstats-url",
			},
		},
		{
			name: "multiple proxy configurations",
			workloadData: &pbcatalog.Workload{
				Identity: testIdentity,
			},
			proxyCfgs: []*pbmesh.ProxyConfiguration{
				{
					Workloads: &pbcatalog.WorkloadSelector{Names: []string{"test-workload"}},
					BootstrapConfig: &pbmesh.BootstrapConfig{
						DogstatsdUrl: "dogstats-url",
					},
				},
				{
					Workloads: &pbcatalog.WorkloadSelector{Prefixes: []string{"test-"}},
					BootstrapConfig: &pbmesh.BootstrapConfig{
						StatsdUrl: "stats-url",
					},
					DynamicConfig: &pbmesh.DynamicConfig{
						AccessLogs: &pbmesh.AccessLogsConfig{
							Enabled:    true,
							JsonFormat: "{ \"custom_field\": \"%START_TIME%\" }",
						},
					},
				},

				{
					Workloads: &pbcatalog.WorkloadSelector{Names: []string{"not-test-workload"}},
					BootstrapConfig: &pbmesh.BootstrapConfig{
						PrometheusBindAddr: "prom-addr",
					},
				},
			},
			expBootstrapCfg: &pbmesh.BootstrapConfig{
				DogstatsdUrl: "dogstats-url",
				StatsdUrl:    "stats-url",
			},
			expAccessLogs: testAccessLogs,
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
			Return(testutils.ACLServiceRead(t, proxyServiceID), nil)

		options := structs.QueryOptions{Token: testToken}
		ctx, err := external.ContextWithQueryOptions(context.Background(), options)
		require.NoError(t, err)

		store := testutils.TestStateStore(t, nil)
		registerReq := testRegisterRequestProxy(t)
		err = store.EnsureRegistration(1, registerReq)
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

func TestGetEnvoyBootstrapParams_Error_EnableV2(t *testing.T) {
	type testCase struct {
		name            string
		expectedErrCode codes.Code
		expecteErrMsg   string
		workload        *pbresource.Resource
	}

	run := func(t *testing.T, tc testCase) {
		resourceClient := svctest.RunResourceService(t, catalog.RegisterTypes, mesh.RegisterTypes)

		options := structs.QueryOptions{Token: testToken}
		ctx, err := external.ContextWithQueryOptions(context.Background(), options)
		require.NoError(t, err)

		aclResolver := &MockACLResolver{}
		aclResolver.On("ResolveTokenAndDefaultMeta", testToken, mock.Anything, mock.Anything).
			Return(testutils.ACLServiceRead(t, "doesn't matter"), nil)

		server := NewServer(Config{
			Logger:            hclog.NewNullLogger(),
			ACLResolver:       aclResolver,
			Datacenter:        serverDC,
			EnableV2:          true,
			ResourceAPIClient: resourceClient,
		})
		client := testClient(t, server)

		var req pbdataplane.GetEnvoyBootstrapParamsRequest
		// Write the workload resource.
		if tc.workload != nil {
			_, err = resourceClient.Write(context.Background(), &pbresource.WriteRequest{
				Resource: tc.workload,
			})
			require.NoError(t, err)

			req = pbdataplane.GetEnvoyBootstrapParamsRequest{
				ProxyId:   tc.workload.Id.Name,
				Namespace: tc.workload.Id.Tenancy.Namespace,
				Partition: tc.workload.Id.Tenancy.Partition,
			}
		} else {
			req = pbdataplane.GetEnvoyBootstrapParamsRequest{
				ProxyId:   "not-found",
				Namespace: "default",
				Partition: "default",
			}
		}

		resp, err := client.GetEnvoyBootstrapParams(ctx, &req)
		require.Nil(t, resp)
		require.Error(t, err)
		errStatus, ok := status.FromError(err)
		require.True(t, ok)
		require.Equal(t, tc.expectedErrCode.String(), errStatus.Code().String())
		require.Equal(t, tc.expecteErrMsg, errStatus.Message())
	}

	workload := resourcetest.Resource(pbcatalog.WorkloadType, "test-workload").
		WithData(t, &pbcatalog.Workload{
			Addresses: []*pbcatalog.WorkloadAddress{
				{Host: "127.0.0.1"},
			},
			Ports: map[string]*pbcatalog.WorkloadPort{
				"tcp": {Port: 8080},
			},
		}).
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Build()

	testCases := []testCase{
		{
			name:            "workload doesn't exist",
			expectedErrCode: codes.NotFound,
			expecteErrMsg:   "resource not found",
		},
		{
			name:            "workload without identity",
			expectedErrCode: codes.InvalidArgument,
			expecteErrMsg:   "workload \"test-workload\" doesn't have identity associated with it",
			workload:        workload,
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

	options := structs.QueryOptions{Token: testToken}
	ctx, err := external.ContextWithQueryOptions(context.Background(), options)
	require.NoError(t, err)
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
		Return(testutils.ACLNoPermissions(t), nil)

	options := structs.QueryOptions{Token: testToken}
	ctx, err := external.ContextWithQueryOptions(context.Background(), options)
	require.NoError(t, err)

	store := testutils.TestStateStore(t, nil)
	registerReq := structs.TestRegisterRequestProxy(t)
	proxyServiceID := "web-sidecar-proxy"
	registerReq.Service.ID = proxyServiceID
	err = store.EnsureRegistration(1, registerReq)
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
