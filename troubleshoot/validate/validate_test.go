// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package validate

import (
	"testing"

	envoy_admin_v3 "github.com/envoyproxy/go-control-plane/envoy/admin/v3"
	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_aggregate_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/clusters/aggregate/v3"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/envoyextensions/extensioncommon"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestErrors(t *testing.T) {
	cases := map[string]struct {
		validate          func() *Validate
		endpointValidator EndpointValidator
		err               string
	}{
		"success": {
			validate: func() *Validate {
				return &Validate{
					envoyID: "db",
					snis: map[string]struct{}{
						"db-sni": {},
					},
					listener: true,
					usesRDS:  true,
					route:    true,
					resources: map[string]*resource{
						"db-sni": {
							required: true,
							cluster:  true,
						},
					},
				}
			},
			endpointValidator: func(r *resource, s string, clusters *envoy_admin_v3.Clusters) {
				r.loadAssignment = true
				r.endpoints = 1
			},
		},
		"no clusters for listener or route": {
			validate: func() *Validate {
				return &Validate{
					envoyID: "db",
					snis: map[string]struct{}{
						"db-sni": {},
					},
					listener: true,
					usesRDS:  true,
					route:    true,
					resources: map[string]*resource{
						"db-sni": {
							required: false,
							cluster:  true,
						},
					},
				}
			},
			endpointValidator: func(r *resource, s string, clusters *envoy_admin_v3.Clusters) {
				r.loadAssignment = true
				r.endpoints = 1
			},
			err: "No clusters found on route or listener",
		},
		"no healthy endpoints": {
			validate: func() *Validate {
				return &Validate{
					envoyID: "db",
					snis: map[string]struct{}{
						"db-sni": {},
					},
					listener: true,
					usesRDS:  true,
					route:    true,
					resources: map[string]*resource{
						"db-sni": {
							required: true,
							cluster:  true,
						},
					},
				}
			},
			endpointValidator: func(r *resource, s string, clusters *envoy_admin_v3.Clusters) {
				r.loadAssignment = true
			},
			err: "No healthy endpoints for cluster \"db-sni\" for upstream \"db\"",
		},
		"success: aggregate cluster with one target with endpoints": {
			validate: func() *Validate {
				return &Validate{
					envoyID: "db",
					snis: map[string]struct{}{
						"db-sni":        {},
						"db-fail-1-sni": {},
						"db-fail-2-sni": {},
					},
					listener: true,
					usesRDS:  true,
					route:    true,
					resources: map[string]*resource{
						"db-sni": {
							required:         true,
							cluster:          true,
							aggregateCluster: true,
							aggregateClusterChildren: []string{
								"db-fail-1-sni",
								"db-fail-2-sni",
							},
						},
						"db-fail-1-sni": {
							required:      true,
							cluster:       true,
							parentCluster: "db-sni",
							// This doesn't usually get set here, but this tests that at least one child cluster has
							// healthy endpoints case.
							endpoints: 1,
						},
						"db-fail-2-sni": {
							required:      true,
							cluster:       true,
							parentCluster: "db-sni",
						},
					},
				}
			},
			endpointValidator: func(r *resource, s string, clusters *envoy_admin_v3.Clusters) {
				r.loadAssignment = true
			},
		},
		"aggregate cluster no healthy endpoints": {
			validate: func() *Validate {
				return &Validate{
					envoyID: "db",
					snis: map[string]struct{}{
						"db-sni":        {},
						"db-fail-1-sni": {},
						"db-fail-2-sni": {},
					},
					listener: true,
					usesRDS:  true,
					route:    true,
					resources: map[string]*resource{
						"db-sni": {
							required:         true,
							cluster:          true,
							aggregateCluster: true,
							aggregateClusterChildren: []string{
								"db-fail-1-sni",
								"db-fail-2-sni",
							},
						},
						"db-fail-1-sni": {
							required:      true,
							cluster:       true,
							parentCluster: "db-sni",
						},
						"db-fail-2-sni": {
							required:      true,
							cluster:       true,
							parentCluster: "db-sni",
						},
					},
				}
			},
			endpointValidator: func(r *resource, s string, clusters *envoy_admin_v3.Clusters) {
				r.loadAssignment = true
				r.endpoints = 0
			},
			err: "No healthy endpoints for aggregate cluster \"db-sni\" for upstream \"db\"",
		},
		"success: passthrough cluster doesn't error even though there are zero endpoints": {
			validate: func() *Validate {
				return &Validate{
					envoyID: "db",
					snis: map[string]struct{}{
						"passthrough~db-sni": {},
					},
					listener: true,
					usesRDS:  true,
					route:    true,
					resources: map[string]*resource{
						"passthrough~db-sni": {
							required: true,
							cluster:  true,
						},
					},
				}
			},
			endpointValidator: func(r *resource, s string, clusters *envoy_admin_v3.Clusters) {
				r.loadAssignment = true
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			v := tc.validate()
			messages := v.GetMessages(true, tc.endpointValidator, nil)

			var outputErrors string
			for _, msgError := range messages.Errors() {
				outputErrors += msgError.Message
				for _, action := range msgError.PossibleActions {
					outputErrors += action
				}
			}
			if tc.err == "" {
				require.True(t, messages.Success())
			} else {
				require.Contains(t, outputErrors, tc.err)
			}

		})
	}

}

func TestIsAggregateCluster(t *testing.T) {
	aggregateClusterConfig, err := anypb.New(&envoy_aggregate_cluster_v3.ClusterConfig{
		Clusters: []string{"c1", "c2"},
	})
	require.NoError(t, err)

	cases := map[string]struct {
		input                    *envoy_cluster_v3.Cluster
		expectedAggregateCluster *envoy_aggregate_cluster_v3.ClusterConfig
		expectedOk               bool
	}{
		"non-aggregate cluster": {
			input: &envoy_cluster_v3.Cluster{
				Name:                 "foo",
				ClusterDiscoveryType: &envoy_cluster_v3.Cluster_Type{Type: envoy_cluster_v3.Cluster_LOGICAL_DNS},
			},
			expectedOk: false,
		},
		"valid aggregate cluster": {
			input: &envoy_cluster_v3.Cluster{
				Name: "foo",
				ClusterDiscoveryType: &envoy_cluster_v3.Cluster_ClusterType{
					ClusterType: &envoy_cluster_v3.Cluster_CustomClusterType{
						Name:        "foo",
						TypedConfig: aggregateClusterConfig,
					},
				},
			},
			expectedOk:               true,
			expectedAggregateCluster: &envoy_aggregate_cluster_v3.ClusterConfig{Clusters: []string{"c1", "c2"}},
		},
	}
	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			ac, ok := isAggregateCluster(tc.input)
			require.Equal(t, tc.expectedOk, ok)

			if tc.expectedOk {
				require.Equal(t, tc.expectedAggregateCluster.Clusters, ac.Clusters)
			}

		})
	}
}

func TestMakeValidate(t *testing.T) {

	cases := map[string]struct {
		extensionName string
		arguments     map[string]interface{}
		expected      *Validate
		snis          map[string]struct{}
		ok            bool
	}{
		"with no arguments": {
			arguments: nil,
			ok:        false,
		},
		"with an invalid name": {
			arguments: map[string]interface{}{
				"envoyID": "id",
			},
			snis: map[string]struct{}{
				"sni1": {},
				"sni2": {},
			},
			extensionName: "bad",
			ok:            false,
		},
		"empty envoy ID": {
			arguments: map[string]interface{}{"envoyID": ""},
			snis: map[string]struct{}{
				"sni1": {},
				"sni2": {},
			},
			ok: false,
		},
		"missing snis": {
			arguments: map[string]interface{}{
				"envoyID": "id",
			},
			expected: &Validate{
				envoyID:   "id",
				resources: map[string]*resource{},
				snis: map[string]struct{}{
					"sni1": {},
					"sni2": {},
				},
			},
			ok: false,
		},
		"valid everything": {
			arguments: map[string]interface{}{
				"envoyID": "id",
			},
			snis: map[string]struct{}{
				"sni1": {},
				"sni2": {},
			},
			expected: &Validate{
				envoyID:   "id",
				resources: map[string]*resource{},
				snis: map[string]struct{}{
					"sni1": {},
					"sni2": {},
				},
			},
			ok: true,
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {

			extensionName := api.BuiltinValidateExtension
			if tc.extensionName != "" {
				extensionName = tc.extensionName
			}

			svc := api.CompoundServiceName{Name: "svc"}
			ext := extensioncommon.RuntimeConfig{
				ServiceName: svc,
				EnvoyExtension: api.EnvoyExtension{
					Name:      extensionName,
					Arguments: tc.arguments,
				},
				Upstreams: map[api.CompoundServiceName]*extensioncommon.UpstreamData{
					svc: {
						SNI: tc.snis,
					},
				},
			}

			patcher, err := MakeValidate(ext)

			if tc.ok {
				require.NoError(t, err)
				require.Equal(t, tc.expected, patcher)
			} else {
				require.Error(t, err)
			}
		})
	}
}
