// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestMutateTCPRoute(t *testing.T) {
	type testcase struct {
		routeTenancy *pbresource.Tenancy
		route        *pbmesh.TCPRoute
		expect       *pbmesh.TCPRoute
	}

	cases := map[string]testcase{}

	// Add common parent refs test cases.
	for name, parentTC := range getXRouteParentRefMutateTestCases() {
		cases["parent-ref: "+name] = testcase{
			routeTenancy: parentTC.routeTenancy,
			route: &pbmesh.TCPRoute{
				ParentRefs: parentTC.refs,
			},
			expect: &pbmesh.TCPRoute{
				ParentRefs: parentTC.expect,
			},
		}
	}
	// add common backend ref test cases.
	for name, backendTC := range getXRouteBackendRefMutateTestCases() {
		var (
			refs   []*pbmesh.TCPBackendRef
			expect []*pbmesh.TCPBackendRef
		)
		for _, br := range backendTC.refs {
			refs = append(refs, &pbmesh.TCPBackendRef{
				BackendRef: br,
			})
		}
		for _, br := range backendTC.expect {
			expect = append(expect, &pbmesh.TCPBackendRef{
				BackendRef: br,
			})
		}
		cases["backend-ref: "+name] = testcase{
			routeTenancy: backendTC.routeTenancy,
			route: &pbmesh.TCPRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(pbcatalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.TCPRouteRule{
					{BackendRefs: refs},
				},
			},
			expect: &pbmesh.TCPRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(pbcatalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.TCPRouteRule{
					{BackendRefs: expect},
				},
			},
		}
	}

	run := func(t *testing.T, tc testcase) {
		res := resourcetest.Resource(pbmesh.TCPRouteType, "api").
			WithTenancy(tc.routeTenancy).
			WithData(t, tc.route).
			Build()

		err := MutateTCPRoute(res)
		require.NoError(t, err)

		got := resourcetest.MustDecode[*pbmesh.TCPRoute](t, res)

		if tc.expect == nil {
			tc.expect = proto.Clone(tc.route).(*pbmesh.TCPRoute)
		}

		prototest.AssertDeepEqual(t, tc.expect, got.Data)
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestValidateTCPRoute(t *testing.T) {
	type testcase struct {
		route     *pbmesh.TCPRoute
		expectErr string
	}

	run := func(t *testing.T, tc testcase) {
		res := resourcetest.Resource(pbmesh.TCPRouteType, "api").
			WithTenancy(resource.DefaultNamespacedTenancy()).
			WithData(t, tc.route).
			Build()

		// Ensure things are properly mutated and updated in the inputs.
		err := MutateTCPRoute(res)
		require.NoError(t, err)
		{
			mutated := resourcetest.MustDecode[*pbmesh.TCPRoute](t, res)
			tc.route = mutated.Data
		}

		err = ValidateTCPRoute(res)

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
					newParentRef(pbcatalog.ServiceType, "web", ""),
				},
			},
		},
		"more than one rule": {
			route: &pbmesh.TCPRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(pbcatalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.TCPRouteRule{
					{
						BackendRefs: []*pbmesh.TCPBackendRef{{
							BackendRef: newBackendRef(pbcatalog.ServiceType, "api", ""),
						}},
					},
					{
						BackendRefs: []*pbmesh.TCPBackendRef{{
							BackendRef: newBackendRef(pbcatalog.ServiceType, "db", ""),
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
					newParentRef(pbcatalog.ServiceType, "web", ""),
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

func TestTCPRouteACLs(t *testing.T) {
	testXRouteACLs[*pbmesh.TCPRoute](t, func(t *testing.T, parentRefs, backendRefs []*pbresource.Reference) *pbresource.Resource {
		data := &pbmesh.TCPRoute{
			ParentRefs: nil,
		}
		for _, ref := range parentRefs {
			data.ParentRefs = append(data.ParentRefs, &pbmesh.ParentReference{
				Ref: ref,
			})
		}

		var ruleRefs []*pbmesh.TCPBackendRef
		for _, ref := range backendRefs {
			ruleRefs = append(ruleRefs, &pbmesh.TCPBackendRef{
				BackendRef: &pbmesh.BackendReference{
					Ref: ref,
				},
			})
		}
		data.Rules = []*pbmesh.TCPRouteRule{
			{BackendRefs: ruleRefs},
		}

		return resourcetest.Resource(pbmesh.TCPRouteType, "api-tcp-route").
			WithTenancy(resource.DefaultNamespacedTenancy()).
			WithData(t, data).
			Build()
	})
}
