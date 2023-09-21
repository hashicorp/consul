// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

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
