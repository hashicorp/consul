// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestValidateTCPRoute(t *testing.T) {
	type testcase struct {
		route     *pbmesh.TCPRoute
		expectErr string
	}

	run := func(t *testing.T, tc testcase) {
		res := resourcetest.Resource(TCPRouteType, "api").
			WithTenancy(resource.DefaultNamespacedTenancy()).
			WithData(t, tc.route).
			Build()

		err := ValidateTCPRoute(res)

		// Verify that validate didn't actually change the object.
		got := resourcetest.MustDecode[*pbmesh.TCPRoute](t, res)
		prototest.AssertDeepEqual(t, tc.route, got.Data)

		if tc.expectErr == "" {
			require.NoError(t, err)
		} else {
			testutil.RequireErrorContains(t, err, tc.expectErr)
		}
	}

	cases := map[string]testcase{
		"no rules": {
			route: &pbmesh.TCPRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(catalog.ServiceType, "web", ""),
				},
			},
		},
		"more than one rule": {
			route: &pbmesh.TCPRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(catalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.TCPRouteRule{
					{
						BackendRefs: []*pbmesh.TCPBackendRef{{
							BackendRef: newBackendRef(catalog.ServiceType, "api", ""),
						}},
					},
					{
						BackendRefs: []*pbmesh.TCPBackendRef{{
							BackendRef: newBackendRef(catalog.ServiceType, "db", ""),
						}},
					},
				},
			},
			expectErr: `invalid "rules" field: must only specify a single rule for now`,
		},
	}

	// Add common parent refs test cases.
	for name, parentTC := range getXRouteParentRefTestCases() {
		cases["parent-ref: "+name] = testcase{
			route: &pbmesh.TCPRoute{
				ParentRefs: parentTC.refs,
			},
			expectErr: parentTC.expectErr,
		}
	}
	// add common backend ref test cases.
	for name, backendTC := range getXRouteBackendRefTestCases() {
		var refs []*pbmesh.TCPBackendRef
		for _, br := range backendTC.refs {
			refs = append(refs, &pbmesh.TCPBackendRef{
				BackendRef: br,
			})
		}
		cases["backend-ref: "+name] = testcase{
			route: &pbmesh.TCPRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(catalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.TCPRouteRule{
					{BackendRefs: refs},
				},
			},
			expectErr: backendTC.expectErr,
		}
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}
