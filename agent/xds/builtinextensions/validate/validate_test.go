package validate

import (
	"testing"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_aggregate_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/clusters/aggregate/v3"
	"github.com/hashicorp/consul/agent/xds/xdscommon"
	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/anypb"
)

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
			extensionName: "bad",
			ok:            false,
		},
		"empty envoy ID": {
			arguments: map[string]interface{}{"envoyID": ""},
			ok:        false,
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
			},
			ok: true,
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {

			extensionName := builtinValidateExtension
			if tc.extensionName != "" {
				extensionName = tc.extensionName
			}

			svc := api.CompoundServiceName{Name: "svc"}
			ext := xdscommon.ExtensionConfiguration{
				ServiceName: svc,
				EnvoyExtension: api.EnvoyExtension{
					Name:      extensionName,
					Arguments: tc.arguments,
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
