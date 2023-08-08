// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package types

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
)

// TODO(rb): add mutation tests

func TestValidateHTTPRoute(t *testing.T) {
	type testcase struct {
		route     *pbmesh.HTTPRoute
		expectErr string
	}

	run := func(t *testing.T, tc testcase) {
		res := resourcetest.Resource(HTTPRouteType, "api").
			WithData(t, tc.route).
			Build()

		err := ValidateHTTPRoute(res)

		// Verify that validate didn't actually change the object.
		got := resourcetest.MustDecode[pbmesh.HTTPRoute, *pbmesh.HTTPRoute](t, res)
		prototest.AssertDeepEqual(t, tc.route, got.Data)

		if tc.expectErr == "" {
			require.NoError(t, err)
		} else {
			testutil.RequireErrorContains(t, err, tc.expectErr)
		}
	}

	cases := map[string]testcase{
		"hostnames not supported for services": {
			route: &pbmesh.HTTPRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(catalog.ServiceType, "web", ""),
				},
				Hostnames: []string{"foo.local"},
			},
			expectErr: `invalid "hostnames" field: should not populate hostnames`,
		},
	}

	// TODO(rb): add rest of tests for the matching logic
	// TODO(rb): add rest of tests for the retry and timeout logic

	// Add common parent refs test cases.
	for name, parentTC := range getXRouteParentRefTestCases() {
		cases["parent-ref: "+name] = testcase{
			route: &pbmesh.HTTPRoute{
				ParentRefs: parentTC.refs,
			},
			expectErr: parentTC.expectErr,
		}
	}
	// add common backend ref test cases.
	for name, backendTC := range getXRouteBackendRefTestCases() {
		var refs []*pbmesh.HTTPBackendRef
		for _, br := range backendTC.refs {
			refs = append(refs, &pbmesh.HTTPBackendRef{
				BackendRef: br,
			})
		}
		cases["backend-ref: "+name] = testcase{
			route: &pbmesh.HTTPRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(catalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.HTTPRouteRule{
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

type xRouteParentRefTestcase struct {
	refs      []*pbmesh.ParentReference
	expectErr string
}

func getXRouteParentRefTestCases() map[string]xRouteParentRefTestcase {
	return map[string]xRouteParentRefTestcase{
		"no parent refs": {
			expectErr: `invalid "parent_refs" field: cannot be empty`,
		},
		"parent ref with nil ref": {
			refs: []*pbmesh.ParentReference{
				newParentRef(catalog.ServiceType, "api", ""),
				{
					Ref:  nil,
					Port: "http",
				},
			},
			expectErr: `invalid element at index 1 of list "parent_refs": invalid "ref" field: missing required field`,
		},
		"parent ref with bad type ref": {
			refs: []*pbmesh.ParentReference{
				newParentRef(catalog.ServiceType, "api", ""),
				newParentRef(catalog.WorkloadType, "api", ""),
			},
			expectErr: `invalid element at index 1 of list "parent_refs": invalid "ref" field: reference must have type catalog.v1alpha1.Service`,
		},
		"parent ref with section": {
			refs: []*pbmesh.ParentReference{
				newParentRef(catalog.ServiceType, "api", ""),
				{
					Ref:  resourcetest.Resource(catalog.ServiceType, "web").Reference("section2"),
					Port: "http",
				},
			},
			expectErr: `invalid element at index 1 of list "parent_refs": invalid "ref" field: invalid "section" field: section not supported for service parent refs`,
		},
		"duplicate exact parents": {
			refs: []*pbmesh.ParentReference{
				newParentRef(catalog.ServiceType, "api", "http"),
				newParentRef(catalog.ServiceType, "api", "http"),
			},
			expectErr: `invalid element at index 1 of list "parent_refs": invalid "ref" field: parent ref "catalog.v1alpha1.Service/default.local.default/api" for port "http" exists twice`,
		},
		"duplicate wild parents": {
			refs: []*pbmesh.ParentReference{
				newParentRef(catalog.ServiceType, "api", ""),
				newParentRef(catalog.ServiceType, "api", ""),
			},
			expectErr: `invalid element at index 1 of list "parent_refs": invalid "ref" field: parent ref "catalog.v1alpha1.Service/default.local.default/api" for wildcard port exists twice`,
		},
		"duplicate parents via exact+wild overlap": {
			refs: []*pbmesh.ParentReference{
				newParentRef(catalog.ServiceType, "api", "http"),
				newParentRef(catalog.ServiceType, "api", ""),
			},
			expectErr: `invalid element at index 1 of list "parent_refs": invalid "ref" field: parent ref "catalog.v1alpha1.Service/default.local.default/api" for ports [http] covered by wildcard port already`,
		},
		"good single parent ref": {
			refs: []*pbmesh.ParentReference{
				newParentRef(catalog.ServiceType, "api", "http"),
			},
		},
		"good muliple parent refs": {
			refs: []*pbmesh.ParentReference{
				newParentRef(catalog.ServiceType, "api", "http"),
				newParentRef(catalog.ServiceType, "web", ""),
			},
		},
	}
}

type xRouteBackendRefTestcase struct {
	refs      []*pbmesh.BackendReference
	expectErr string
}

func getXRouteBackendRefTestCases() map[string]xRouteBackendRefTestcase {
	return map[string]xRouteBackendRefTestcase{
		"no backend refs": {
			expectErr: `invalid "backend_refs" field: cannot be empty`,
		},
		"backend ref with nil ref": {
			refs: []*pbmesh.BackendReference{
				newBackendRef(catalog.ServiceType, "api", ""),
				{
					Ref:  nil,
					Port: "http",
				},
			},
			expectErr: `invalid element at index 0 of list "rules": invalid element at index 1 of list "backend_refs": invalid "backend_ref" field: invalid "ref" field: missing required field`,
		},
		"backend ref with bad type ref": {
			refs: []*pbmesh.BackendReference{
				newBackendRef(catalog.ServiceType, "api", ""),
				newBackendRef(catalog.WorkloadType, "api", ""),
			},
			expectErr: `invalid element at index 0 of list "rules": invalid element at index 1 of list "backend_refs": invalid "backend_ref" field: invalid "ref" field: reference must have type catalog.v1alpha1.Service`,
		},
		"backend ref with section": {
			refs: []*pbmesh.BackendReference{
				newBackendRef(catalog.ServiceType, "api", ""),
				{
					Ref:  resourcetest.Resource(catalog.ServiceType, "web").Reference("section2"),
					Port: "http",
				},
			},
			expectErr: `invalid element at index 0 of list "rules": invalid element at index 1 of list "backend_refs": invalid "backend_ref" field: invalid "ref" field: invalid "section" field: section not supported for service backend refs`,
		},
		"backend ref with datacenter": {
			refs: []*pbmesh.BackendReference{
				newBackendRef(catalog.ServiceType, "api", ""),
				{
					Ref:        newRef(catalog.ServiceType, "db"),
					Port:       "http",
					Datacenter: "dc2",
				},
			},
			expectErr: `invalid element at index 0 of list "rules": invalid element at index 1 of list "backend_refs": invalid "backend_ref" field: invalid "datacenter" field: datacenter is not yet supported on backend refs`,
		},
	}
}

func newRef(typ *pbresource.Type, name string) *pbresource.Reference {
	return resourcetest.Resource(typ, name).Reference("")
}

func newBackendRef(typ *pbresource.Type, name, port string) *pbmesh.BackendReference {
	return &pbmesh.BackendReference{
		Ref:  newRef(typ, name),
		Port: port,
	}
}

func newParentRef(typ *pbresource.Type, name, port string) *pbmesh.ParentReference {
	return &pbmesh.ParentReference{
		Ref:  newRef(typ, name),
		Port: port,
	}
}
