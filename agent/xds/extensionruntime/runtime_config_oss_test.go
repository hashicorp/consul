// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package extensionruntime

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/envoyextensions/extensioncommon"
)

func TestGetRuntimeConfigurations_TerminatingGateway(t *testing.T) {
	snap := proxycfg.TestConfigSnapshotTerminatingGatewayWithLambdaServiceAndServiceResolvers(t)

	webService := api.CompoundServiceName{
		Name:      "web",
		Namespace: "default",
		Partition: "default",
	}
	dbService := api.CompoundServiceName{
		Name:      "db",
		Namespace: "default",
		Partition: "default",
	}
	cacheService := api.CompoundServiceName{
		Name:      "cache",
		Namespace: "default",
		Partition: "default",
	}
	apiService := api.CompoundServiceName{
		Name:      "api",
		Namespace: "default",
		Partition: "default",
	}

	expected := map[api.CompoundServiceName][]extensioncommon.RuntimeConfig{
		apiService:   {},
		cacheService: {},
		dbService:    {},
		webService: {
			{
				EnvoyExtension: api.EnvoyExtension{
					Name: api.BuiltinAWSLambdaExtension,
					Arguments: map[string]interface{}{
						"ARN":                "arn:aws:lambda:us-east-1:111111111111:function:lambda-1234",
						"PayloadPassthrough": true,
					},
				},
				ServiceName:           webService,
				IsSourcedFromUpstream: true,
				Upstreams: map[api.CompoundServiceName]*extensioncommon.UpstreamData{
					apiService: {
						PrimarySNI: "api.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul",
						SNIs: map[string]struct{}{
							"api.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul": {},
						},
						EnvoyID:           "api",
						OutgoingProxyKind: "terminating-gateway",
					},
					cacheService: {
						PrimarySNI: "cache.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul",
						SNIs: map[string]struct{}{
							"cache.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul": {},
						},
						EnvoyID:           "cache",
						OutgoingProxyKind: "terminating-gateway",
					},
					dbService: {
						PrimarySNI: "db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul",
						SNIs: map[string]struct{}{
							"db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul": {},
						},
						EnvoyID:           "db",
						OutgoingProxyKind: "terminating-gateway",
					},
					webService: {
						PrimarySNI: "web.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul",
						SNIs: map[string]struct{}{
							"canary1.web.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul": {},
							"canary2.web.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul": {},
							"web.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul":         {},
						},
						EnvoyID:           "web",
						OutgoingProxyKind: "terminating-gateway",
					},
				},
				Kind: api.ServiceKindTerminatingGateway,
			},
		},
	}

	require.Equal(t, expected, GetRuntimeConfigurations(snap))
}

func TestGetRuntimeConfigurations_ConnectProxy(t *testing.T) {
	dbService := api.CompoundServiceName{
		Name:      "db",
		Partition: "default",
		Namespace: "default",
	}
	webService := api.CompoundServiceName{
		Name:      "web",
		Partition: "",
		Namespace: "default",
	}

	// Setup multiple extensions to ensure only the expected one (AWS) is in the ExtensionConfiguration map
	// sourced from upstreams, and all local extensions are included.
	envoyExtensions := []structs.EnvoyExtension{
		{
			Name: api.BuiltinAWSLambdaExtension,
			Arguments: map[string]interface{}{
				"ARN":                "arn:aws:lambda:us-east-1:111111111111:function:lambda-1234",
				"PayloadPassthrough": true,
			},
		},
		{
			Name: "ext2",
			Arguments: map[string]interface{}{
				"arg1": 1,
				"arg2": "val2",
			},
		},
	}

	serviceDefaults := &structs.ServiceConfigEntry{
		Kind:            structs.ServiceDefaults,
		Name:            "db",
		Protocol:        "http",
		EnvoyExtensions: envoyExtensions,
	}

	serviceDefaultsV2 := &structs.ServiceConfigEntry{
		Kind:     structs.ServiceDefaults,
		Name:     "db-v2",
		Protocol: "http",
	}

	serviceSplitter := &structs.ServiceSplitterConfigEntry{
		Kind: structs.ServiceSplitter,
		Name: "db",
		Splits: []structs.ServiceSplit{
			{Weight: 50},
			{Weight: 50, Service: "db-v2"},
		},
	}

	// Setup a snapshot where the db upstream is on a connect proxy.
	snapConnect := proxycfg.TestConfigSnapshotDiscoveryChain(t, "default", false, nil, nil, serviceDefaults, serviceDefaultsV2, serviceSplitter)
	// Setup a snapshot where the db upstream is on a terminating gateway.
	snapTermGw := proxycfg.TestConfigSnapshotDiscoveryChain(t, "register-to-terminating-gateway", false, nil, nil, serviceDefaults, serviceDefaultsV2, serviceSplitter)
	// Setup a snapshot with the local service web has extensions.
	snapWebConnect := proxycfg.TestConfigSnapshotDiscoveryChain(t, "default", false, func(ns *structs.NodeService) {
		ns.Proxy.EnvoyExtensions = envoyExtensions
	}, nil)

	type testCase struct {
		snapshot *proxycfg.ConfigSnapshot
		expected map[api.CompoundServiceName][]extensioncommon.RuntimeConfig
	}
	cases := map[string]testCase{
		"connect proxy upstream": {
			snapshot: snapConnect,
			expected: map[api.CompoundServiceName][]extensioncommon.RuntimeConfig{
				dbService: {
					{
						EnvoyExtension: api.EnvoyExtension{
							Name: api.BuiltinAWSLambdaExtension,
							Arguments: map[string]interface{}{
								"ARN":                "arn:aws:lambda:us-east-1:111111111111:function:lambda-1234",
								"PayloadPassthrough": true,
							},
						},
						ServiceName:           dbService,
						IsSourcedFromUpstream: true,
						Upstreams: map[api.CompoundServiceName]*extensioncommon.UpstreamData{
							dbService: {
								PrimarySNI: "db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul",
								SNIs: map[string]struct{}{
									"db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul":    {},
									"db-v2.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul": {},
								},
								EnvoyID:           "db",
								OutgoingProxyKind: "connect-proxy",
							},
						},
						Kind: api.ServiceKindConnectProxy,
					},
				},
				webService: {},
			},
		},
		"terminating gateway upstream": {
			snapshot: snapTermGw,
			expected: map[api.CompoundServiceName][]extensioncommon.RuntimeConfig{
				dbService: {
					{
						EnvoyExtension: api.EnvoyExtension{
							Name: api.BuiltinAWSLambdaExtension,
							Arguments: map[string]interface{}{
								"ARN":                "arn:aws:lambda:us-east-1:111111111111:function:lambda-1234",
								"PayloadPassthrough": true,
							},
						},
						ServiceName:           dbService,
						IsSourcedFromUpstream: true,
						Upstreams: map[api.CompoundServiceName]*extensioncommon.UpstreamData{
							dbService: {
								PrimarySNI: "db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul",
								SNIs: map[string]struct{}{
									"db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul":    {},
									"db-v2.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul": {},
								},
								EnvoyID:           "db",
								OutgoingProxyKind: "terminating-gateway",
							},
						},
						Kind: api.ServiceKindConnectProxy,
					},
				},
				webService: {},
			},
		},
		"local service extensions": {
			snapshot: snapWebConnect,
			expected: map[api.CompoundServiceName][]extensioncommon.RuntimeConfig{
				dbService: {},
				webService: {
					{
						EnvoyExtension: api.EnvoyExtension{
							Name: api.BuiltinAWSLambdaExtension,
							Arguments: map[string]interface{}{
								"ARN":                "arn:aws:lambda:us-east-1:111111111111:function:lambda-1234",
								"PayloadPassthrough": true,
							},
						},
						ServiceName:           webService,
						Kind:                  api.ServiceKindConnectProxy,
						IsSourcedFromUpstream: false,
						Upstreams: map[api.CompoundServiceName]*extensioncommon.UpstreamData{
							dbService: {
								PrimarySNI: "db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul",
								SNIs: map[string]struct{}{
									"db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul": {},
								},
								EnvoyID:           "db",
								OutgoingProxyKind: "connect-proxy",
							},
						},
					},
					{
						EnvoyExtension: api.EnvoyExtension{
							Name: "ext2",
							Arguments: map[string]interface{}{
								"arg1": 1,
								"arg2": "val2",
							},
						},
						ServiceName:           webService,
						Kind:                  api.ServiceKindConnectProxy,
						IsSourcedFromUpstream: false,
						Upstreams: map[api.CompoundServiceName]*extensioncommon.UpstreamData{
							dbService: {
								PrimarySNI: "db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul",
								SNIs: map[string]struct{}{
									"db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul": {},
								},
								EnvoyID:           "db",
								OutgoingProxyKind: "connect-proxy",
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.expected, GetRuntimeConfigurations(tc.snapshot))
		})
	}
}
