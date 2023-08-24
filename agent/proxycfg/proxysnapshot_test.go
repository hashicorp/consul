package proxycfg

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestConfigSnapshot_AllowEmptyClusters(t *testing.T) {
	type testCase struct {
		description    string
		cfgSnapshot    *ConfigSnapshot
		expectedResult bool
	}
	testsCases := []testCase{
		{
			description:    "Mesh proxies are not allowed",
			cfgSnapshot:    &ConfigSnapshot{Kind: structs.ServiceKindConnectProxy},
			expectedResult: false,
		},
		{
			description:    "Ingress gateways are allowed",
			cfgSnapshot:    &ConfigSnapshot{Kind: structs.ServiceKindIngressGateway},
			expectedResult: true,
		},
		{
			description:    "Terminating gateways are allowed",
			cfgSnapshot:    &ConfigSnapshot{Kind: structs.ServiceKindTerminatingGateway},
			expectedResult: true,
		},
		{
			description:    "API Gateways are allowed",
			cfgSnapshot:    &ConfigSnapshot{Kind: structs.ServiceKindAPIGateway},
			expectedResult: true,
		},
		{
			description:    "Mesh Gateways are allowed",
			cfgSnapshot:    &ConfigSnapshot{Kind: structs.ServiceKindMeshGateway},
			expectedResult: true,
		},
	}
	for _, tc := range testsCases {
		t.Run(tc.description, func(t *testing.T) {
			require.Equal(t, tc.expectedResult, tc.cfgSnapshot.AllowEmptyClusters())
		})
	}
}

func TestConfigSnapshot_AllowEmptyListeners(t *testing.T) {
	type testCase struct {
		description    string
		cfgSnapshot    *ConfigSnapshot
		expectedResult bool
	}
	testsCases := []testCase{
		{
			description:    "Mesh proxies are not allowed",
			cfgSnapshot:    &ConfigSnapshot{Kind: structs.ServiceKindConnectProxy},
			expectedResult: false,
		},
		{
			description:    "Ingress gateways are allowed",
			cfgSnapshot:    &ConfigSnapshot{Kind: structs.ServiceKindIngressGateway},
			expectedResult: true,
		},
		{
			description:    "Terminating gateways are not allowed",
			cfgSnapshot:    &ConfigSnapshot{Kind: structs.ServiceKindTerminatingGateway},
			expectedResult: false,
		},
		{
			description:    "API Gateways are allowed",
			cfgSnapshot:    &ConfigSnapshot{Kind: structs.ServiceKindAPIGateway},
			expectedResult: true,
		},
		{
			description:    "Mesh Gateways are not allowed",
			cfgSnapshot:    &ConfigSnapshot{Kind: structs.ServiceKindMeshGateway},
			expectedResult: false,
		},
	}
	for _, tc := range testsCases {
		t.Run(tc.description, func(t *testing.T) {
			require.Equal(t, tc.expectedResult, tc.cfgSnapshot.AllowEmptyListeners())
		})
	}
}

func TestConfigSnapshot_AllowEmptyRoutes(t *testing.T) {
	type testCase struct {
		description    string
		cfgSnapshot    *ConfigSnapshot
		expectedResult bool
	}
	testsCases := []testCase{
		{
			description:    "Mesh proxies are not allowed",
			cfgSnapshot:    &ConfigSnapshot{Kind: structs.ServiceKindConnectProxy},
			expectedResult: false,
		},
		{
			description:    "Ingress gateways are allowed",
			cfgSnapshot:    &ConfigSnapshot{Kind: structs.ServiceKindIngressGateway},
			expectedResult: true,
		},
		{
			description:    "Terminating gateways are not allowed",
			cfgSnapshot:    &ConfigSnapshot{Kind: structs.ServiceKindTerminatingGateway},
			expectedResult: false,
		},
		{
			description:    "API Gateways are allowed",
			cfgSnapshot:    &ConfigSnapshot{Kind: structs.ServiceKindAPIGateway},
			expectedResult: true,
		},
		{
			description:    "Mesh Gateways are not allowed",
			cfgSnapshot:    &ConfigSnapshot{Kind: structs.ServiceKindMeshGateway},
			expectedResult: false,
		},
	}
	for _, tc := range testsCases {
		t.Run(tc.description, func(t *testing.T) {
			require.Equal(t, tc.expectedResult, tc.cfgSnapshot.AllowEmptyRoutes())
		})
	}
}

func TestConfigSnapshot_LoggerName(t *testing.T) {
	type testCase struct {
		description    string
		cfgSnapshot    *ConfigSnapshot
		expectedResult string
	}
	testsCases := []testCase{
		{
			description:    "Mesh proxies have a logger named ''",
			cfgSnapshot:    &ConfigSnapshot{Kind: structs.ServiceKindConnectProxy},
			expectedResult: "",
		},
		{
			description:    "Ingress gateways have a logger named 'ingress_gateway'",
			cfgSnapshot:    &ConfigSnapshot{Kind: structs.ServiceKindIngressGateway},
			expectedResult: "ingress_gateway",
		},
		{
			description:    "Terminating gateways have a logger named 'terminating_gateway'",
			cfgSnapshot:    &ConfigSnapshot{Kind: structs.ServiceKindTerminatingGateway},
			expectedResult: "terminating_gateway",
		},
		{
			description:    "API Gateways have a logger named ''",
			cfgSnapshot:    &ConfigSnapshot{Kind: structs.ServiceKindAPIGateway},
			expectedResult: "",
		},
		{
			description:    "Mesh Gateways have a logger named 'mesh_gateway'",
			cfgSnapshot:    &ConfigSnapshot{Kind: structs.ServiceKindMeshGateway},
			expectedResult: "mesh_gateway",
		},
	}
	for _, tc := range testsCases {
		t.Run(tc.description, func(t *testing.T) {
			require.Equal(t, tc.expectedResult, tc.cfgSnapshot.LoggerName())
		})
	}
}

func TestConfigSnapshot_Authorize(t *testing.T) {
	type testCase struct {
		description          string
		cfgSnapshot          *ConfigSnapshot
		configureAuthorizer  func(authorizer *acl.MockAuthorizer)
		expectedErrorMessage string
	}
	testsCases := []testCase{
		{
			description: "ConnectProxy - if service write is allowed for the DestinationService then allow.",
			cfgSnapshot: &ConfigSnapshot{
				Kind: structs.ServiceKindConnectProxy,
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "DestinationServiceName",
				},
			},
			expectedErrorMessage: "",
			configureAuthorizer: func(authz *acl.MockAuthorizer) {
				authz.On("ServiceWrite", "DestinationServiceName", mock.Anything).Return(acl.Allow)
			},
		},
		{
			description: "ConnectProxy - if service write is not allowed for the DestinationService then deny.",
			cfgSnapshot: &ConfigSnapshot{
				Kind: structs.ServiceKindConnectProxy,
				Proxy: structs.ConnectProxyConfig{
					DestinationServiceName: "DestinationServiceName",
				},
			},
			expectedErrorMessage: "rpc error: code = PermissionDenied desc = Permission denied: token with AccessorID '' lacks permission 'service:write' on \"DestinationServiceName\"",
			configureAuthorizer: func(authz *acl.MockAuthorizer) {
				authz.On("ServiceWrite", "DestinationServiceName", mock.Anything).Return(acl.Deny)
			},
		},
		{
			description: "Mesh Gateway - if service write is allowed for the Service then allow.",
			cfgSnapshot: &ConfigSnapshot{
				Kind:    structs.ServiceKindMeshGateway,
				Service: "Service",
			},
			expectedErrorMessage: "",
			configureAuthorizer: func(authz *acl.MockAuthorizer) {
				authz.On("ServiceWrite", "Service", mock.Anything).Return(acl.Allow)
			},
		},
		{
			description: "Mesh Gateway - if service write is not allowed for the Service then deny.",
			cfgSnapshot: &ConfigSnapshot{
				Kind:    structs.ServiceKindMeshGateway,
				Service: "Service",
			},
			expectedErrorMessage: "rpc error: code = PermissionDenied desc = Permission denied: token with AccessorID '' lacks permission 'service:write' on \"Service\"",
			configureAuthorizer: func(authz *acl.MockAuthorizer) {
				authz.On("ServiceWrite", "Service", mock.Anything).Return(acl.Deny)
			},
		},
		{
			description: "Terminating Gateway - if service write is allowed for the Service then allow.",
			cfgSnapshot: &ConfigSnapshot{
				Kind:    structs.ServiceKindTerminatingGateway,
				Service: "Service",
			},
			expectedErrorMessage: "rpc error: code = PermissionDenied desc = Permission denied: token with AccessorID '' lacks permission 'service:write' on \"Service\"",
			configureAuthorizer: func(authz *acl.MockAuthorizer) {
				authz.On("ServiceWrite", "Service", mock.Anything).Return(acl.Deny)
			},
		},
		{
			description: "Terminating Gateway - if service write is not allowed for the Service then deny.",
			cfgSnapshot: &ConfigSnapshot{
				Kind:    structs.ServiceKindTerminatingGateway,
				Service: "Service",
			},
			expectedErrorMessage: "rpc error: code = PermissionDenied desc = Permission denied: token with AccessorID '' lacks permission 'service:write' on \"Service\"",
			configureAuthorizer: func(authz *acl.MockAuthorizer) {
				authz.On("ServiceWrite", "Service", mock.Anything).Return(acl.Deny)
			},
		},
		{
			description: "Ingress Gateway - if service write is allowed for the Service then allow.",
			cfgSnapshot: &ConfigSnapshot{
				Kind:    structs.ServiceKindIngressGateway,
				Service: "Service",
			},
			expectedErrorMessage: "rpc error: code = PermissionDenied desc = Permission denied: token with AccessorID '' lacks permission 'service:write' on \"Service\"",
			configureAuthorizer: func(authz *acl.MockAuthorizer) {
				authz.On("ServiceWrite", "Service", mock.Anything).Return(acl.Deny)
			},
		},
		{
			description: "Ingress Gateway - if service write is not allowed for the Service then deny.",
			cfgSnapshot: &ConfigSnapshot{
				Kind:    structs.ServiceKindIngressGateway,
				Service: "Service",
			},
			expectedErrorMessage: "rpc error: code = PermissionDenied desc = Permission denied: token with AccessorID '' lacks permission 'service:write' on \"Service\"",
			configureAuthorizer: func(authz *acl.MockAuthorizer) {
				authz.On("ServiceWrite", "Service", mock.Anything).Return(acl.Deny)
			},
		},
		{
			description: "API Gateway - if service write is allowed for the Service then allow.",
			cfgSnapshot: &ConfigSnapshot{
				Kind:    structs.ServiceKindAPIGateway,
				Service: "Service",
			},
			expectedErrorMessage: "rpc error: code = PermissionDenied desc = Permission denied: token with AccessorID '' lacks permission 'service:write' on \"Service\"",
			configureAuthorizer: func(authz *acl.MockAuthorizer) {
				authz.On("ServiceWrite", "Service", mock.Anything).Return(acl.Deny)
			},
		},
		{
			description: "API Gateway - if service write is not allowed for the Service then deny.",
			cfgSnapshot: &ConfigSnapshot{
				Kind:    structs.ServiceKindAPIGateway,
				Service: "Service",
			},
			expectedErrorMessage: "rpc error: code = PermissionDenied desc = Permission denied: token with AccessorID '' lacks permission 'service:write' on \"Service\"",
			configureAuthorizer: func(authz *acl.MockAuthorizer) {
				authz.On("ServiceWrite", "Service", mock.Anything).Return(acl.Deny)
			},
		},
	}
	for _, tc := range testsCases {
		t.Run(tc.description, func(t *testing.T) {
			authz := &acl.MockAuthorizer{}
			authz.On("ToAllow").Return(acl.AllowAuthorizer{Authorizer: authz})
			tc.configureAuthorizer(authz)
			err := tc.cfgSnapshot.Authorize(authz)
			errMsg := ""
			if err != nil {
				errMsg = err.Error()
			}
			// using contains because Enterprise tests append the parition and namespace
			// information to the message.
			require.True(t, strings.Contains(errMsg, tc.expectedErrorMessage))
		})
	}
}
