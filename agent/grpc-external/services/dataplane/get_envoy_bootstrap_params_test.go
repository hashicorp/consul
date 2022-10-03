package dataplane

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/imdario/mergo"
	"github.com/mitchellh/copystructure"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/acl/resolver"
	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/consul/state"
	external "github.com/hashicorp/consul/agent/grpc-external"
	"github.com/hashicorp/consul/agent/grpc-external/testutils"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto-public/pbdataplane"
	"github.com/hashicorp/consul/types"
)

const (
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
	registerReq.ID = types.NodeID("2980b72b-bd9d-9d7b-d4f9-951bf7508d95")
	registerReq.Service.ID = registerReq.Service.Service
	registerReq.Service.Proxy.Config = map[string]interface{}{
		proxyConfigKey: proxyConfigValue,
	}
	return registerReq
}

func testProxyDefaults(t *testing.T) structs.ConfigEntry {
	return &structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: structs.ProxyConfigGlobal,
		Config: map[string]interface{}{
			protocolKey:       proxyDefaultsProtocol,
			requestTimeoutKey: proxyDefaultsRequestTimeout,
		},
	}
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
			Return(testutils.TestAuthorizerServiceRead(t, tc.registerReq.Service.ID), nil)

		options := structs.QueryOptions{Token: testToken}
		ctx, err := external.ContextWithQueryOptions(context.Background(), options)
		require.NoError(t, err)

		server := NewServer(Config{
			GetStore:                          func() StateStore { return store },
			Logger:                            hclog.NewNullLogger(),
			ACLResolver:                       aclResolver,
			Datacenter:                        serverDC,
			MergeNodeServiceWithCentralConfig: testMergeNodeServiceWithCentralConfig,
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
		require.Equal(t, convertToResponseServiceKind(tc.registerReq.Service.Kind), resp.ServiceKind)
		require.Equal(t, tc.registerReq.Node, resp.NodeName)
		require.Equal(t, string(tc.registerReq.ID), resp.NodeId)

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
			proxyDefaults: testProxyDefaults(t),
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
		Return(testutils.TestAuthorizerDenyAll(t), nil)

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

// Copied implementation from agent/consul/merge_service_config.go to avoid import cycle.
func testMergeNodeServiceWithCentralConfig(
	ws memdb.WatchSet,
	state *state.Store,
	args *structs.ServiceSpecificRequest,
	ns *structs.NodeService,
	logger hclog.Logger) (uint64, *structs.NodeService, error) {

	serviceName := ns.Service
	var upstreams []structs.ServiceID
	if ns.IsSidecarProxy() {
		// This is a sidecar proxy, ignore the proxy service's config since we are
		// managed by the target service config.
		serviceName = ns.Proxy.DestinationServiceName

		// Also if we have any upstreams defined, add them to the defaults lookup request
		// so we can learn about their configs.
		for _, us := range ns.Proxy.Upstreams {
			if us.DestinationType == "" || us.DestinationType == structs.UpstreamDestTypeService {
				sid := us.DestinationID()
				sid.EnterpriseMeta.Merge(&ns.EnterpriseMeta)
				upstreams = append(upstreams, sid)
			}
		}
	}

	configReq := &structs.ServiceConfigRequest{
		Name:           serviceName,
		Datacenter:     args.Datacenter,
		QueryOptions:   args.QueryOptions,
		MeshGateway:    ns.Proxy.MeshGateway,
		Mode:           ns.Proxy.Mode,
		UpstreamIDs:    upstreams,
		EnterpriseMeta: ns.EnterpriseMeta,
	}

	// prefer using this vs directly calling the ConfigEntry.ResolveServiceConfig RPC
	// so as to pass down the same watch set to also watch on changes to
	// proxy-defaults/global and service-defaults.
	cfgIndex, configEntries, err := state.ReadResolvedServiceConfigEntries(
		ws,
		configReq.Name,
		&configReq.EnterpriseMeta,
		upstreams,
		configReq.Mode,
	)
	if err != nil {
		return 0, nil, fmt.Errorf("Failure looking up service config entries for %s: %v",
			ns.ID, err)
	}

	defaults, err := configentry.ComputeResolvedServiceConfig(
		configReq,
		upstreams,
		false,
		configEntries,
		logger,
	)
	if err != nil {
		return 0, nil, fmt.Errorf("Failure computing service defaults for %s: %v",
			ns.ID, err)
	}

	mergedns, err := testMergeServiceConfig(defaults, ns)
	if err != nil {
		return 0, nil, fmt.Errorf("Failure merging service definition with config entry defaults for %s: %v",
			ns.ID, err)
	}

	return cfgIndex, mergedns, nil
}

// Copied implementation from agent/consul/merge_service_config.go to avoid import cycle.
func testMergeServiceConfig(defaults *structs.ServiceConfigResponse, service *structs.NodeService) (*structs.NodeService, error) {
	if defaults == nil {
		return service, nil
	}

	// We don't want to change s.registration in place since it is our source of
	// truth about what was actually registered before defaults applied. So copy
	// it first.
	nsRaw, err := copystructure.Copy(service)
	if err != nil {
		return nil, err
	}

	// Merge proxy defaults
	ns := nsRaw.(*structs.NodeService)

	if err := mergo.Merge(&ns.Proxy.Config, defaults.ProxyConfig); err != nil {
		return nil, err
	}
	if err := mergo.Merge(&ns.Proxy.Expose, defaults.Expose); err != nil {
		return nil, err
	}

	if ns.Proxy.MeshGateway.Mode == structs.MeshGatewayModeDefault {
		ns.Proxy.MeshGateway.Mode = defaults.MeshGateway.Mode
	}
	if ns.Proxy.Mode == structs.ProxyModeDefault {
		ns.Proxy.Mode = defaults.Mode
	}
	if ns.Proxy.TransparentProxy.OutboundListenerPort == 0 {
		ns.Proxy.TransparentProxy.OutboundListenerPort = defaults.TransparentProxy.OutboundListenerPort
	}
	if !ns.Proxy.TransparentProxy.DialedDirectly {
		ns.Proxy.TransparentProxy.DialedDirectly = defaults.TransparentProxy.DialedDirectly
	}

	// remoteUpstreams contains synthetic Upstreams generated from central config (service-defaults.UpstreamConfigs).
	remoteUpstreams := make(map[structs.ServiceID]structs.Upstream)

	for _, us := range defaults.UpstreamIDConfigs {
		parsed, err := structs.ParseUpstreamConfigNoDefaults(us.Config)
		if err != nil {
			return nil, fmt.Errorf("failed to parse upstream config map for %s: %v", us.Upstream.String(), err)
		}

		remoteUpstreams[us.Upstream] = structs.Upstream{
			DestinationNamespace: us.Upstream.NamespaceOrDefault(),
			DestinationPartition: us.Upstream.PartitionOrDefault(),
			DestinationName:      us.Upstream.ID,
			Config:               us.Config,
			MeshGateway:          parsed.MeshGateway,
			CentrallyConfigured:  true,
		}
	}

	// localUpstreams stores the upstreams seen from the local registration so that we can merge in the synthetic entries.
	// In transparent proxy mode ns.Proxy.Upstreams will likely be empty because users do not need to define upstreams explicitly.
	// So to store upstream-specific flags from central config, we add entries to ns.Proxy.Upstream with those values.
	localUpstreams := make(map[structs.ServiceID]struct{})

	// Merge upstream defaults into the local registration
	for i := range ns.Proxy.Upstreams {
		// Get a pointer not a value copy of the upstream struct
		us := &ns.Proxy.Upstreams[i]
		if us.DestinationType != "" && us.DestinationType != structs.UpstreamDestTypeService {
			continue
		}
		localUpstreams[us.DestinationID()] = struct{}{}

		remoteCfg, ok := remoteUpstreams[us.DestinationID()]
		if !ok {
			// No config defaults to merge
			continue
		}

		// The local upstream config mode has the highest precedence, so only overwrite when it's set to the default
		if us.MeshGateway.Mode == structs.MeshGatewayModeDefault {
			us.MeshGateway.Mode = remoteCfg.MeshGateway.Mode
		}

		// Merge in everything else that is read from the map
		if err := mergo.Merge(&us.Config, remoteCfg.Config); err != nil {
			return nil, err
		}

		// Delete the mesh gateway key from opaque config since this is the value that was resolved from
		// the servers and NOT the final merged value for this upstream.
		// Note that we use the "mesh_gateway" key and not other variants like "MeshGateway" because
		// UpstreamConfig.MergeInto and ResolveServiceConfig only use "mesh_gateway".
		delete(us.Config, "mesh_gateway")
	}

	// Ensure upstreams present in central config are represented in the local configuration.
	// This does not apply outside of transparent mode because in that situation every possible upstream already exists
	// inside of ns.Proxy.Upstreams.
	if ns.Proxy.Mode == structs.ProxyModeTransparent {
		for id, remote := range remoteUpstreams {
			if _, ok := localUpstreams[id]; ok {
				// Remote upstream is already present locally
				continue
			}

			ns.Proxy.Upstreams = append(ns.Proxy.Upstreams, remote)
		}
	}

	return ns, err
}
