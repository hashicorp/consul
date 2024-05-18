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

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/version/versiontest"
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
			expectErr: `invalid element at index 1 of list "parent_refs": invalid "port" field: parent ref "catalog.v2beta1.Service/default.default/api" for port "http" exists twice`,
		},
		"duplicate wild parents": {
			routeTenancy: resource.DefaultNamespacedTenancy(),
			refs: []*pbmesh.ParentReference{
				newParentRef(pbcatalog.ServiceType, "", "api", ""),
				newParentRef(pbcatalog.ServiceType, "", "api", ""),
			},
			expectErr: `invalid element at index 1 of list "parent_refs": invalid "port" field: parent ref "catalog.v2beta1.Service/default.default/api" for wildcard port exists twice`,
		},
		"duplicate parents via exact+wild overlap": {
			routeTenancy: resource.DefaultNamespacedTenancy(),
			refs: []*pbmesh.ParentReference{
				newParentRef(pbcatalog.ServiceType, "", "api", "http"),
				newParentRef(pbcatalog.ServiceType, "", "api", ""),
			},
			expectErr: `invalid element at index 1 of list "parent_refs": invalid "port" field: parent ref "catalog.v2beta1.Service/default.default/api" for ports [http] covered by wildcard port already`,
		},
		"duplicate parents via exact+wild overlap (reversed)": {
			routeTenancy: resource.DefaultNamespacedTenancy(),
			refs: []*pbmesh.ParentReference{
				newParentRef(pbcatalog.ServiceType, "", "api", ""),
				newParentRef(pbcatalog.ServiceType, "", "api", "http"),
			},
			expectErr: `invalid element at index 1 of list "parent_refs": invalid "port" field: parent ref "catalog.v2beta1.Service/default.default/api" for port "http" covered by wildcard port already`,
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
		require.NotEmpty(t, parentRefs)
		require.NotEmpty(t, backendRefs)
		res := userNewRoute(t, parentRefs, backendRefs)
		res.Id.Tenancy = parentRefs[0].Tenancy
		resourcetest.ValidateAndNormalize(t, registry, res)
		return res
	}

	const (
		DENY    = resourcetest.DENY
		ALLOW   = resourcetest.ALLOW
		DEFAULT = resourcetest.DEFAULT
	)

	serviceRef := func(tenancy, name string) *pbresource.Reference {
		return newRefWithTenancy(pbcatalog.ServiceType, tenancy, name)
	}

	resOneParentOneBackend := func(parentTenancy, backendTenancy string) *pbresource.Resource {
		return newRoute(t,
			[]*pbresource.Reference{
				serviceRef(parentTenancy, "api1"),
			},
			[]*pbresource.Reference{
				serviceRef(backendTenancy, "backend1"),
			},
		)
	}
	resTwoParentsOneBackend := func(parentTenancy, backendTenancy string) *pbresource.Resource {
		return newRoute(t,
			[]*pbresource.Reference{
				serviceRef(parentTenancy, "api1"),
				serviceRef(parentTenancy, "api2"),
			},
			[]*pbresource.Reference{
				serviceRef(backendTenancy, "backend1"),
			},
		)
	}
	resOneParentTwoBackends := func(parentTenancy, backendTenancy string) *pbresource.Resource {
		return newRoute(t,
			[]*pbresource.Reference{
				serviceRef(parentTenancy, "api1"),
			},
			[]*pbresource.Reference{
				serviceRef(backendTenancy, "backend1"),
				serviceRef(backendTenancy, "backend2"),
			},
		)
	}
	resTwoParentsTwoBackends := func(parentTenancy, backendTenancy string) *pbresource.Resource {
		return newRoute(t,
			[]*pbresource.Reference{
				serviceRef(parentTenancy, "api1"),
				serviceRef(parentTenancy, "api2"),
			},
			[]*pbresource.Reference{
				serviceRef(backendTenancy, "backend1"),
				serviceRef(backendTenancy, "backend2"),
			},
		)
	}

	run := func(t *testing.T, name string, tc resourcetest.ACLTestCase) {
		t.Run(name, func(t *testing.T) {
			resourcetest.RunACLTestCase(t, tc, registry)
		})
	}

	isEnterprise := versiontest.IsEnterprise()

	serviceRead := func(partition, namespace, name string) string {
		if isEnterprise {
			return fmt.Sprintf(` partition %q { namespace %q { service %q { policy = "read" } } }`, partition, namespace, name)
		}
		return fmt.Sprintf(` service %q { policy = "read" } `, name)
	}
	serviceWrite := func(partition, namespace, name string) string {
		if isEnterprise {
			return fmt.Sprintf(` partition %q { namespace %q { service %q { policy = "write" } } }`, partition, namespace, name)
		}
		return fmt.Sprintf(` service %q { policy = "write" } `, name)
	}

	assert := func(t *testing.T, name string, rules string, res *pbresource.Resource, readOK, writeOK string) {
		tc := resourcetest.ACLTestCase{
			Rules:                    rules,
			Res:                      res,
			ReadOK:                   readOK,
			WriteOK:                  writeOK,
			ListOK:                   DEFAULT,
			ReadHookRequiresResource: true,
		}
		run(t, name, tc)
	}

	tenancies := []string{"default.default"}
	if isEnterprise {
		tenancies = append(tenancies, "default.foo", "alpha.default", "alpha.foo")
	}

	for _, parentTenancyStr := range tenancies {
		t.Run("route tenancy: "+parentTenancyStr, func(t *testing.T) {
			for _, backendTenancyStr := range tenancies {
				t.Run("backend tenancy: "+backendTenancyStr, func(t *testing.T) {
					for _, aclTenancyStr := range tenancies {
						t.Run("acl tenancy: "+aclTenancyStr, func(t *testing.T) {
							aclTenancy := resourcetest.Tenancy(aclTenancyStr)

							maybe := func(match string, parentOnly bool) string {
								if parentTenancyStr != aclTenancyStr {
									return DENY
								}
								if !parentOnly && backendTenancyStr != aclTenancyStr {
									return DENY
								}
								return match
							}

							t.Run("no rules", func(t *testing.T) {
								rules := ``
								assert(t, "1parent 1backend", rules, resOneParentOneBackend(parentTenancyStr, backendTenancyStr), DENY, DENY)
								assert(t, "1parent 2backends", rules, resOneParentTwoBackends(parentTenancyStr, backendTenancyStr), DENY, DENY)
								assert(t, "2parents 1backend", rules, resTwoParentsOneBackend(parentTenancyStr, backendTenancyStr), DENY, DENY)
								assert(t, "2parents 2backends", rules, resTwoParentsTwoBackends(parentTenancyStr, backendTenancyStr), DENY, DENY)
							})
							t.Run("api1:read", func(t *testing.T) {
								rules := serviceRead(aclTenancy.Partition, aclTenancy.Namespace, "api1")
								assert(t, "1parent 1backend", rules, resOneParentOneBackend(parentTenancyStr, backendTenancyStr), maybe(ALLOW, true), DENY)
								assert(t, "1parent 2backends", rules, resOneParentTwoBackends(parentTenancyStr, backendTenancyStr), maybe(ALLOW, true), DENY)
								assert(t, "2parents 1backend", rules, resTwoParentsOneBackend(parentTenancyStr, backendTenancyStr), DENY, DENY)
								assert(t, "2parents 2backends", rules, resTwoParentsTwoBackends(parentTenancyStr, backendTenancyStr), DENY, DENY)
							})
							t.Run("api1:write", func(t *testing.T) {
								rules := serviceWrite(aclTenancy.Partition, aclTenancy.Namespace, "api1")
								assert(t, "1parent 1backend", rules, resOneParentOneBackend(parentTenancyStr, backendTenancyStr), maybe(ALLOW, true), DENY)
								assert(t, "1parent 2backends", rules, resOneParentTwoBackends(parentTenancyStr, backendTenancyStr), maybe(ALLOW, true), DENY)
								assert(t, "2parents 1backend", rules, resTwoParentsOneBackend(parentTenancyStr, backendTenancyStr), DENY, DENY)
								assert(t, "2parents 2backends", rules, resTwoParentsTwoBackends(parentTenancyStr, backendTenancyStr), DENY, DENY)
							})
							t.Run("api1:write backend1:read", func(t *testing.T) {
								rules := serviceWrite(aclTenancy.Partition, aclTenancy.Namespace, "api1") +
									serviceRead(aclTenancy.Partition, aclTenancy.Namespace, "backend1")
								assert(t, "1parent 1backend", rules, resOneParentOneBackend(parentTenancyStr, backendTenancyStr), maybe(ALLOW, true), maybe(ALLOW, false))
								assert(t, "1parent 2backends", rules, resOneParentTwoBackends(parentTenancyStr, backendTenancyStr), maybe(ALLOW, true), DENY)
								assert(t, "2parents 1backend", rules, resTwoParentsOneBackend(parentTenancyStr, backendTenancyStr), DENY, DENY)
								assert(t, "2parents 2backends", rules, resTwoParentsTwoBackends(parentTenancyStr, backendTenancyStr), DENY, DENY)
							})
						})
					}
				})
			}
		})
	}
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
