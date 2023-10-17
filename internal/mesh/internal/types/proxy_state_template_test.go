// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbmesh/v2beta1/pbproxystate"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestValidateProxyStateTemplate(t *testing.T) {
	type testcase struct {
		pst       *pbmesh.ProxyStateTemplate
		expectErr string
	}

	run := func(t *testing.T, tc testcase) {
		res := resourcetest.Resource(pbmesh.ProxyStateTemplateType, "api").
			WithData(t, tc.pst).
			Build()

		err := ValidateProxyStateTemplate(res)

		// Verify that validate didn't actually change the object.
		got := resourcetest.MustDecode[*pbmesh.ProxyStateTemplate](t, res)
		prototest.AssertDeepEqual(t, tc.pst, got.Data)

		if tc.expectErr == "" {
			require.NoError(t, err)
		} else {
			testutil.RequireErrorContains(t, err, tc.expectErr)
		}
	}

	pstForCluster := func(name string, cluster *pbproxystate.Cluster) *pbmesh.ProxyStateTemplate {
		return &pbmesh.ProxyStateTemplate{
			ProxyState: &pbmesh.ProxyState{
				Clusters: map[string]*pbproxystate.Cluster{
					name: cluster,
				},
			},
		}
	}

	clusterForGroups := func(name string, groups ...*pbproxystate.EndpointGroup) *pbproxystate.Cluster {
		cluster := &pbproxystate.Cluster{
			Name: name,
		}

		require.NotEmpty(t, groups)

		if len(groups) == 1 {
			cluster.Group = &pbproxystate.Cluster_EndpointGroup{
				EndpointGroup: groups[0],
			}
		} else {
			cluster.Group = &pbproxystate.Cluster_FailoverGroup{
				FailoverGroup: &pbproxystate.FailoverGroup{
					EndpointGroups: groups,
				},
			}
		}
		return cluster
	}

	endpointGroup := func(name string) *pbproxystate.EndpointGroup {
		return &pbproxystate.EndpointGroup{
			Name: name,
			Group: &pbproxystate.EndpointGroup_Dynamic{
				Dynamic: &pbproxystate.DynamicEndpointGroup{},
			},
		}
	}

	// also cover clusters with names that don't match the map key
	// also empty map keys
	cases := map[string]testcase{
		// ============== COMMON ==============
		"cluster with missing cluster group": {
			pst: pstForCluster("api-cluster", &pbproxystate.Cluster{
				Name: "api-cluster",
			}),
			expectErr: `invalid "proxy_state" field: invalid value of key "api-cluster" within clusters: invalid "group" field: missing required field`,
		},
		// ============== STANDARD ==============
		"standard cluster with empty map key": {
			pst: pstForCluster("", clusterForGroups("api-cluster",
				endpointGroup(""),
			)),
			expectErr: `invalid "proxy_state" field: map clusters contains an invalid key - "": cannot be empty`,
		},
		"standard cluster with missing cluster name": {
			pst: pstForCluster("api-cluster", clusterForGroups("",
				endpointGroup(""),
			)),
			expectErr: `invalid "proxy_state" field: invalid value of key "api-cluster" within clusters: invalid "name" field: cluster name "" does not match map key "api-cluster"`,
		},
		"standard cluster with empty endpoint group name": {
			pst: pstForCluster("api-cluster", clusterForGroups("api-cluster",
				endpointGroup(""),
			)),
		},
		"standard cluster with same endpoint group name": {
			pst: pstForCluster("api-cluster", clusterForGroups("api-cluster",
				endpointGroup("api-cluster"),
			)),
		},
		"standard cluster with different endpoint group name": {
			pst: pstForCluster("api-cluster", clusterForGroups("api-cluster",
				endpointGroup("garbage"),
			)),
			expectErr: `invalid "proxy_state" field: invalid value of key "api-cluster" within clusters: invalid "group" field: invalid "endpoint_group" field: invalid "name" field: optional but "garbage" does not match enclosing cluster name "api-cluster"`,
		},
		// ============== FAILOVER ==============
		"failover cluster with empty map key": {
			pst: pstForCluster("", clusterForGroups("api-cluster",
				endpointGroup("api-cluster~0"),
				endpointGroup("api-cluster~1"),
			)),
			expectErr: `invalid "proxy_state" field: map clusters contains an invalid key - "": cannot be empty`,
		},
		"failover cluster with missing cluster name": {
			pst: pstForCluster("api-cluster", clusterForGroups("",
				endpointGroup("api-cluster~0"),
				endpointGroup("api-cluster~1"),
			)),
			expectErr: `invalid "proxy_state" field: invalid value of key "api-cluster" within clusters: invalid "name" field: cluster name "" does not match map key "api-cluster"`,
		},
		"failover cluster with empty endpoint group name": {
			pst: pstForCluster("api-cluster", clusterForGroups("api-cluster",
				endpointGroup("api-cluster~0"),
				endpointGroup(""),
			)),
			expectErr: `invalid "proxy_state" field: invalid value of key "api-cluster" within clusters: invalid "group" field: invalid "failover_group" field: invalid element at index 1 of list "endpoint_groups": invalid "name" field: cannot be empty`,
		},
		"failover cluster with same endpoint group name": {
			pst: pstForCluster("api-cluster", clusterForGroups("api-cluster",
				endpointGroup("api-cluster~0"),
				endpointGroup("api-cluster"),
			)),
			expectErr: `invalid "proxy_state" field: invalid value of key "api-cluster" within clusters: invalid "group" field: invalid "failover_group" field: invalid element at index 1 of list "endpoint_groups": invalid "name" field: name cannot be the same as the enclosing cluster "api-cluster"`,
		},
		"failover cluster with no groups": {
			pst: pstForCluster("api-cluster", &pbproxystate.Cluster{
				Name: "api-cluster",
				Group: &pbproxystate.Cluster_FailoverGroup{
					FailoverGroup: &pbproxystate.FailoverGroup{
						EndpointGroups: nil,
					},
				},
			}),
			expectErr: `invalid "proxy_state" field: invalid value of key "api-cluster" within clusters: invalid "group" field: invalid "failover_group" field: invalid "endpoint_groups" field: cannot be empty`,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Logf("%+v", tc.pst)
			run(t, tc)
		})
	}
}
