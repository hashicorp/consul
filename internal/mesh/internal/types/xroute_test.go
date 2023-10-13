// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type xRouteParentRefMutateTestcase struct {
	routeTenancy *pbresource.Tenancy
	refs         []*pbmesh.ParentReference
	expect       []*pbmesh.ParentReference
}

func getXRouteParentRefMutateTestCases() map[string]xRouteParentRefMutateTestcase {
	newRef := func(typ *pbresource.Type, tenancyStr, name string) *pbresource.Reference {
		return resourcetest.Resource(typ, name).
			WithTenancy(resourcetest.Tenancy(tenancyStr)).
			Reference("")
	}

	newParentRef := func(typ *pbresource.Type, tenancyStr, name, port string) *pbmesh.ParentReference {
		return &pbmesh.ParentReference{
			Ref:  newRef(typ, tenancyStr, name),
			Port: port,
		}
	}

	return map[string]xRouteParentRefMutateTestcase{
		"parent ref tenancies defaulted": {
			routeTenancy: resourcetest.Tenancy("foo.bar"),
			refs: []*pbmesh.ParentReference{
				newParentRef(pbcatalog.ServiceType, "", "api", ""),
				newParentRef(pbcatalog.ServiceType, ".zim", "api", ""),
				newParentRef(pbcatalog.ServiceType, "gir.zim", "api", ""),
			},
			expect: []*pbmesh.ParentReference{
				newParentRef(pbcatalog.ServiceType, "foo.bar", "api", ""),
				newParentRef(pbcatalog.ServiceType, "foo.zim", "api", ""),
				newParentRef(pbcatalog.ServiceType, "gir.zim", "api", ""),
			},
		},
	}
}

type xRouteBackendRefMutateTestcase struct {
	routeTenancy *pbresource.Tenancy
	refs         []*pbmesh.BackendReference
	expect       []*pbmesh.BackendReference
}

func getXRouteBackendRefMutateTestCases() map[string]xRouteBackendRefMutateTestcase {
	newRef := func(typ *pbresource.Type, tenancyStr, name string) *pbresource.Reference {
		return resourcetest.Resource(typ, name).
			WithTenancy(resourcetest.Tenancy(tenancyStr)).
			Reference("")
	}

	newBackendRef := func(typ *pbresource.Type, tenancyStr, name, port string) *pbmesh.BackendReference {
		return &pbmesh.BackendReference{
			Ref:  newRef(typ, tenancyStr, name),
			Port: port,
		}
	}

	return map[string]xRouteBackendRefMutateTestcase{
		"backend ref tenancies defaulted": {
			routeTenancy: resourcetest.Tenancy("foo.bar"),
			refs: []*pbmesh.BackendReference{
				newBackendRef(pbcatalog.ServiceType, "", "api", ""),
				newBackendRef(pbcatalog.ServiceType, ".zim", "api", ""),
				newBackendRef(pbcatalog.ServiceType, "gir.zim", "api", ""),
			},
			expect: []*pbmesh.BackendReference{
				newBackendRef(pbcatalog.ServiceType, "foo.bar", "api", ""),
				newBackendRef(pbcatalog.ServiceType, "foo.zim", "api", ""),
				newBackendRef(pbcatalog.ServiceType, "gir.zim", "api", ""),
			},
		},
	}
}

type xRouteParentRefTestcase struct {
	routeTenancy *pbresource.Tenancy
	refs         []*pbmesh.ParentReference
	expectErr    string
}

func getXRouteParentRefTestCases() map[string]xRouteParentRefTestcase {
	newRef := func(typ *pbresource.Type, tenancyStr, name string) *pbresource.Reference {
		return resourcetest.Resource(typ, name).
			WithTenancy(resourcetest.Tenancy(tenancyStr)).
			Reference("")
	}

	newParentRef := func(typ *pbresource.Type, tenancyStr, name, port string) *pbmesh.ParentReference {
		return &pbmesh.ParentReference{
			Ref:  newRef(typ, tenancyStr, name),
			Port: port,
		}
	}
	return map[string]xRouteParentRefTestcase{
		"no parent refs": {
			routeTenancy: resource.DefaultNamespacedTenancy(),
			expectErr:    `invalid "parent_refs" field: cannot be empty`,
		},
		"parent ref with nil ref": {
			routeTenancy: resource.DefaultNamespacedTenancy(),
			refs: []*pbmesh.ParentReference{
				newParentRef(pbcatalog.ServiceType, "", "api", ""),
				{
					Ref:  nil,
					Port: "http",
				},
			},
			expectErr: `invalid element at index 1 of list "parent_refs": invalid "ref" field: missing required field`,
		},
		"parent ref with bad type ref": {
			routeTenancy: resource.DefaultNamespacedTenancy(),
			refs: []*pbmesh.ParentReference{
				newParentRef(pbcatalog.ServiceType, "", "api", ""),
				newParentRef(pbcatalog.WorkloadType, "", "api", ""),
			},
			expectErr: `invalid element at index 1 of list "parent_refs": invalid "ref" field: invalid "type" field: reference must have type catalog.v2beta1.Service`,
		},
		"parent ref with section": {
			routeTenancy: resource.DefaultNamespacedTenancy(),
			refs: []*pbmesh.ParentReference{
				newParentRef(pbcatalog.ServiceType, "", "api", ""),
				{
					Ref:  resourcetest.Resource(pbcatalog.ServiceType, "web").Reference("section2"),
					Port: "http",
				},
			},
			expectErr: `invalid element at index 1 of list "parent_refs": invalid "ref" field: invalid "section" field: section cannot be set here`,
		},
		"cross namespace parent": {
			routeTenancy: resource.DefaultNamespacedTenancy(),
			refs: []*pbmesh.ParentReference{
				newParentRef(pbcatalog.ServiceType, "default.foo", "api", ""),
			},
			expectErr: `invalid element at index 0 of list "parent_refs": invalid "ref" field: invalid "tenancy" field: resource tenancy and reference tenancy differ`,
		},
		"cross partition parent": {
			routeTenancy: resource.DefaultNamespacedTenancy(),
			refs: []*pbmesh.ParentReference{
				newParentRef(pbcatalog.ServiceType, "alpha.default", "api", ""),
			},
			expectErr: `invalid element at index 0 of list "parent_refs": invalid "ref" field: invalid "tenancy" field: resource tenancy and reference tenancy differ`,
		},
		"cross tenancy parent": {
			routeTenancy: resource.DefaultNamespacedTenancy(),
			refs: []*pbmesh.ParentReference{
				newParentRef(pbcatalog.ServiceType, "alpha.foo", "api", ""),
			},
			expectErr: `invalid element at index 0 of list "parent_refs": invalid "ref" field: invalid "tenancy" field: resource tenancy and reference tenancy differ`,
		},
		"duplicate exact parents": {
			routeTenancy: resource.DefaultNamespacedTenancy(),
			refs: []*pbmesh.ParentReference{
				newParentRef(pbcatalog.ServiceType, "", "api", "http"),
				newParentRef(pbcatalog.ServiceType, "", "api", "http"),
			},
			expectErr: `invalid element at index 1 of list "parent_refs": invalid "port" field: parent ref "catalog.v2beta1.Service/default.local.default/api" for port "http" exists twice`,
		},
		"duplicate wild parents": {
			routeTenancy: resource.DefaultNamespacedTenancy(),
			refs: []*pbmesh.ParentReference{
				newParentRef(pbcatalog.ServiceType, "", "api", ""),
				newParentRef(pbcatalog.ServiceType, "", "api", ""),
			},
			expectErr: `invalid element at index 1 of list "parent_refs": invalid "port" field: parent ref "catalog.v2beta1.Service/default.local.default/api" for wildcard port exists twice`,
		},
		"duplicate parents via exact+wild overlap": {
			routeTenancy: resource.DefaultNamespacedTenancy(),
			refs: []*pbmesh.ParentReference{
				newParentRef(pbcatalog.ServiceType, "", "api", "http"),
				newParentRef(pbcatalog.ServiceType, "", "api", ""),
			},
			expectErr: `invalid element at index 1 of list "parent_refs": invalid "port" field: parent ref "catalog.v2beta1.Service/default.local.default/api" for ports [http] covered by wildcard port already`,
		},
		"duplicate parents via exact+wild overlap (reversed)": {
			routeTenancy: resource.DefaultNamespacedTenancy(),
			refs: []*pbmesh.ParentReference{
				newParentRef(pbcatalog.ServiceType, "", "api", ""),
				newParentRef(pbcatalog.ServiceType, "", "api", "http"),
			},
			expectErr: `invalid element at index 1 of list "parent_refs": invalid "port" field: parent ref "catalog.v2beta1.Service/default.local.default/api" for port "http" covered by wildcard port already`,
		},
		"good single parent ref": {
			routeTenancy: resource.DefaultNamespacedTenancy(),
			refs: []*pbmesh.ParentReference{
				newParentRef(pbcatalog.ServiceType, "", "api", "http"),
			},
		},
		"good muliple parent refs": {
			routeTenancy: resource.DefaultNamespacedTenancy(),
			refs: []*pbmesh.ParentReference{
				newParentRef(pbcatalog.ServiceType, "", "api", "http"),
				newParentRef(pbcatalog.ServiceType, "", "web", ""),
			},
		},
	}
}

type xRouteBackendRefTestcase struct {
	routeTenancy *pbresource.Tenancy
	refs         []*pbmesh.BackendReference
	expectErr    string
}

func getXRouteBackendRefTestCases() map[string]xRouteBackendRefTestcase {
	newRef := func(typ *pbresource.Type, tenancyStr, name string) *pbresource.Reference {
		return resourcetest.Resource(typ, name).
			WithTenancy(resourcetest.Tenancy(tenancyStr)).
			Reference("")
	}

	newBackendRef := func(typ *pbresource.Type, tenancyStr, name, port string) *pbmesh.BackendReference {
		return &pbmesh.BackendReference{
			Ref:  newRef(typ, tenancyStr, name),
			Port: port,
		}
	}
	return map[string]xRouteBackendRefTestcase{
		"no backend refs": {
			routeTenancy: resource.DefaultNamespacedTenancy(),
			expectErr:    `invalid "backend_refs" field: cannot be empty`,
		},
		"backend ref with nil ref": {
			routeTenancy: resource.DefaultNamespacedTenancy(),
			refs: []*pbmesh.BackendReference{
				newBackendRef(pbcatalog.ServiceType, "", "api", ""),
				{
					Ref:  nil,
					Port: "http",
				},
			},
			expectErr: `invalid element at index 0 of list "rules": invalid element at index 1 of list "backend_refs": invalid "backend_ref" field: invalid "ref" field: missing required field`,
		},
		"backend ref with bad type ref": {
			routeTenancy: resource.DefaultNamespacedTenancy(),
			refs: []*pbmesh.BackendReference{
				newBackendRef(pbcatalog.ServiceType, "", "api", ""),
				newBackendRef(pbcatalog.WorkloadType, "", "api", ""),
			},
			expectErr: `invalid element at index 0 of list "rules": invalid element at index 1 of list "backend_refs": invalid "backend_ref" field: invalid "ref" field: invalid "type" field: reference must have type catalog.v2beta1.Service`,
		},
		"backend ref with section": {
			routeTenancy: resource.DefaultNamespacedTenancy(),
			refs: []*pbmesh.BackendReference{
				newBackendRef(pbcatalog.ServiceType, "", "api", ""),
				{
					Ref:  resourcetest.Resource(pbcatalog.ServiceType, "web").Reference("section2"),
					Port: "http",
				},
			},
			expectErr: `invalid element at index 0 of list "rules": invalid element at index 1 of list "backend_refs": invalid "backend_ref" field: invalid "ref" field: invalid "section" field: section cannot be set here`,
		},
		"backend ref with datacenter": {
			routeTenancy: resource.DefaultNamespacedTenancy(),
			refs: []*pbmesh.BackendReference{
				newBackendRef(pbcatalog.ServiceType, "", "api", ""),
				{
					Ref:        newRef(pbcatalog.ServiceType, "", "db"),
					Port:       "http",
					Datacenter: "dc2",
				},
			},
			expectErr: `invalid element at index 0 of list "rules": invalid element at index 1 of list "backend_refs": invalid "backend_ref" field: invalid "datacenter" field: datacenter is not yet supported on backend refs`,
		},
		"cross namespace backend": {
			routeTenancy: resource.DefaultNamespacedTenancy(),
			refs: []*pbmesh.BackendReference{
				newBackendRef(pbcatalog.ServiceType, "default.foo", "api", ""),
			},
		},
		"cross partition backend": {
			routeTenancy: resource.DefaultNamespacedTenancy(),
			refs: []*pbmesh.BackendReference{
				newBackendRef(pbcatalog.ServiceType, "alpha.default", "api", ""),
			},
		},
		"cross tenancy backend": {
			routeTenancy: resource.DefaultNamespacedTenancy(),
			refs: []*pbmesh.BackendReference{
				newBackendRef(pbcatalog.ServiceType, "alpha.foo", "api", ""),
			},
		},
		"good backend ref": {
			routeTenancy: resource.DefaultNamespacedTenancy(),
			refs: []*pbmesh.BackendReference{
				newBackendRef(pbcatalog.ServiceType, "", "api", ""),
				{
					Ref:  newRef(pbcatalog.ServiceType, "", "db"),
					Port: "http",
				},
			},
		},
	}
}

type xRouteTimeoutsTestcase struct {
	timeouts  *pbmesh.HTTPRouteTimeouts
	expectErr string
}

func getXRouteTimeoutsTestCases() map[string]xRouteTimeoutsTestcase {
	return map[string]xRouteTimeoutsTestcase{
		"bad request": {
			timeouts: &pbmesh.HTTPRouteTimeouts{
				Request: durationpb.New(-1 * time.Second),
			},
			expectErr: `invalid element at index 0 of list "rules": invalid "timeouts" field: invalid "request" field: timeout cannot be negative: -1s`,
		},
		"bad idle": {
			timeouts: &pbmesh.HTTPRouteTimeouts{
				Idle: durationpb.New(-1 * time.Second),
			},
			expectErr: `invalid element at index 0 of list "rules": invalid "timeouts" field: invalid "idle" field: timeout cannot be negative: -1s`,
		},
		"good all": {
			timeouts: &pbmesh.HTTPRouteTimeouts{
				Request: durationpb.New(1 * time.Second),
				Idle:    durationpb.New(3 * time.Second),
			},
		},
	}
}

type xRouteRetriesTestcase struct {
	retries   *pbmesh.HTTPRouteRetries
	expectErr string
}

func getXRouteRetriesTestCases() map[string]xRouteRetriesTestcase {
	return map[string]xRouteRetriesTestcase{
		"bad conditions": {
			retries: &pbmesh.HTTPRouteRetries{
				OnConditions: []string{"garbage"},
			},
			expectErr: `invalid element at index 0 of list "rules": invalid "retries" field: invalid element at index 0 of list "on_conditions": not a valid retry condition: "garbage"`,
		},
		"good all": {
			retries: &pbmesh.HTTPRouteRetries{
				Number:       wrapperspb.UInt32(5),
				OnConditions: []string{"internal"},
			},
		},
	}
}

func testXRouteACLs[R XRouteData](t *testing.T, newRoute func(t *testing.T, parentRefs, backendRefs []*pbresource.Reference) *pbresource.Resource) {
	// Wire up a registry to generically invoke hooks
	registry := resource.NewRegistry()
	Register(registry)

	userNewRoute := newRoute
	newRoute = func(t *testing.T, parentRefs, backendRefs []*pbresource.Reference) *pbresource.Resource {
		res := userNewRoute(t, parentRefs, backendRefs)
		resourcetest.ValidateAndNormalize(t, registry, res)
		return res
	}

	type testcase struct {
		res     *pbresource.Resource
		rules   string
		check   func(t *testing.T, authz acl.Authorizer, res *pbresource.Resource)
		readOK  string
		writeOK string
	}

	const (
		DENY    = "deny"
		ALLOW   = "allow"
		DEFAULT = "default"
	)

	checkF := func(t *testing.T, name string, expect string, got error) {
		switch expect {
		case ALLOW:
			if acl.IsErrPermissionDenied(got) {
				t.Fatal(name + " should be allowed")
			}
		case DENY:
			if !acl.IsErrPermissionDenied(got) {
				t.Fatal(name + " should be denied")
			}
		case DEFAULT:
			require.Nil(t, got, name+" expected fallthrough decision")
		default:
			t.Fatalf(name+" unexpected expectation: %q", expect)
		}
	}

	resOneParentOneBackend := newRoute(t,
		[]*pbresource.Reference{
			newRef(pbcatalog.ServiceType, "api1"),
		},
		[]*pbresource.Reference{
			newRef(pbcatalog.ServiceType, "backend1"),
		},
	)
	resTwoParentsOneBackend := newRoute(t,
		[]*pbresource.Reference{
			newRef(pbcatalog.ServiceType, "api1"),
			newRef(pbcatalog.ServiceType, "api2"),
		},
		[]*pbresource.Reference{
			newRef(pbcatalog.ServiceType, "backend1"),
		},
	)
	resOneParentTwoBackends := newRoute(t,
		[]*pbresource.Reference{
			newRef(pbcatalog.ServiceType, "api1"),
		},
		[]*pbresource.Reference{
			newRef(pbcatalog.ServiceType, "backend1"),
			newRef(pbcatalog.ServiceType, "backend2"),
		},
	)
	resTwoParentsTwoBackends := newRoute(t,
		[]*pbresource.Reference{
			newRef(pbcatalog.ServiceType, "api1"),
			newRef(pbcatalog.ServiceType, "api2"),
		},
		[]*pbresource.Reference{
			newRef(pbcatalog.ServiceType, "backend1"),
			newRef(pbcatalog.ServiceType, "backend2"),
		},
	)

	run := func(t *testing.T, name string, tc testcase) {
		t.Run(name, func(t *testing.T) {
			config := acl.Config{
				WildcardName: structs.WildcardSpecifier,
			}
			authz, err := acl.NewAuthorizerFromRules(tc.rules, &config, nil)
			require.NoError(t, err)
			authz = acl.NewChainedAuthorizer([]acl.Authorizer{authz, acl.DenyAll()})

			reg, ok := registry.Resolve(tc.res.Id.GetType())
			require.True(t, ok)

			err = reg.ACLs.Read(authz, &acl.AuthorizerContext{}, tc.res.Id, nil)
			require.ErrorIs(t, err, resource.ErrNeedData, "read hook should require the data payload")

			checkF(t, "read", tc.readOK, reg.ACLs.Read(authz, &acl.AuthorizerContext{}, tc.res.Id, tc.res))
			checkF(t, "write", tc.writeOK, reg.ACLs.Write(authz, &acl.AuthorizerContext{}, tc.res))
			checkF(t, "list", DEFAULT, reg.ACLs.List(authz, &acl.AuthorizerContext{}))
		})
	}

	serviceRead := func(name string) string {
		return fmt.Sprintf(` service %q { policy = "read" } `, name)
	}
	serviceWrite := func(name string) string {
		return fmt.Sprintf(` service %q { policy = "write" } `, name)
	}

	assert := func(t *testing.T, name string, rules string, res *pbresource.Resource, readOK, writeOK string) {
		tc := testcase{
			res:     res,
			rules:   rules,
			readOK:  readOK,
			writeOK: writeOK,
		}
		run(t, name, tc)
	}

	t.Run("no rules", func(t *testing.T) {
		rules := ``
		assert(t, "1parent 1backend", rules, resOneParentOneBackend, DENY, DENY)
		assert(t, "1parent 2backends", rules, resOneParentTwoBackends, DENY, DENY)
		assert(t, "2parents 1backend", rules, resTwoParentsOneBackend, DENY, DENY)
		assert(t, "2parents 2backends", rules, resTwoParentsTwoBackends, DENY, DENY)
	})
	t.Run("api1:read", func(t *testing.T) {
		rules := serviceRead("api1")
		assert(t, "1parent 1backend", rules, resOneParentOneBackend, ALLOW, DENY)
		assert(t, "1parent 2backends", rules, resOneParentTwoBackends, ALLOW, DENY)
		assert(t, "2parents 1backend", rules, resTwoParentsOneBackend, DENY, DENY)
		assert(t, "2parents 2backends", rules, resTwoParentsTwoBackends, DENY, DENY)
	})
	t.Run("api1:write", func(t *testing.T) {
		rules := serviceWrite("api1")
		assert(t, "1parent 1backend", rules, resOneParentOneBackend, ALLOW, DENY)
		assert(t, "1parent 2backends", rules, resOneParentTwoBackends, ALLOW, DENY)
		assert(t, "2parents 1backend", rules, resTwoParentsOneBackend, DENY, DENY)
		assert(t, "2parents 2backends", rules, resTwoParentsTwoBackends, DENY, DENY)
	})
	t.Run("api1:write backend1:read", func(t *testing.T) {
		rules := serviceWrite("api1") + serviceRead("backend1")
		assert(t, "1parent 1backend", rules, resOneParentOneBackend, ALLOW, ALLOW)
		assert(t, "1parent 2backends", rules, resOneParentTwoBackends, ALLOW, DENY)
		assert(t, "2parents 1backend", rules, resTwoParentsOneBackend, DENY, DENY)
		assert(t, "2parents 2backends", rules, resTwoParentsTwoBackends, DENY, DENY)
	})
}

func newRef(typ *pbresource.Type, name string) *pbresource.Reference {
	return resourcetest.Resource(typ, name).
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Reference("")
}

func newRefWithTenancy(typ *pbresource.Type, tenancyStr, name string) *pbresource.Reference {
	return resourcetest.Resource(typ, name).
		WithTenancy(resourcetest.Tenancy(tenancyStr)).
		Reference("")
}

func newBackendRef(typ *pbresource.Type, name, port string) *pbmesh.BackendReference {
	return &pbmesh.BackendReference{
		Ref:  newRef(typ, name),
		Port: port,
	}
}

func newParentRef(typ *pbresource.Type, name, port string) *pbmesh.ParentReference {
	return newParentRefWithTenancy(typ, "default.default", name, port)
}

func newParentRefWithTenancy(typ *pbresource.Type, tenancyStr string, name, port string) *pbmesh.ParentReference {
	return &pbmesh.ParentReference{
		Ref:  newRefWithTenancy(typ, tenancyStr, name),
		Port: port,
	}
}
