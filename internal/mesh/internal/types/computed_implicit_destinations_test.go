// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestValidateComputedImplicitDestinations(t *testing.T) {
	type testcase struct {
		data      *pbmesh.ComputedImplicitDestinations
		expectErr string
	}
	run := func(t *testing.T, tc testcase) {
		res := resourcetest.Resource(pbmesh.ComputedImplicitDestinationsType, "api").
			WithTenancy(resource.DefaultNamespacedTenancy()).
			WithData(t, tc.data).
			Build()

		err := ValidateComputedImplicitDestinations(res)

		// Verify that validate didn't actually change the object.
		got := resourcetest.MustDecode[*pbmesh.ComputedImplicitDestinations](t, res)
		prototest.AssertDeepEqual(t, tc.data, got.Data)

		if tc.expectErr == "" {
			require.NoError(t, err)
		} else {
			testutil.RequireErrorContains(t, err, tc.expectErr)
		}
	}

	cases := map[string]testcase{
		// emptiness
		"empty": {
			data: &pbmesh.ComputedImplicitDestinations{},
		},
		"svc/nil ref": {
			data: &pbmesh.ComputedImplicitDestinations{
				Destinations: []*pbmesh.ImplicitDestination{
					{DestinationRef: nil},
				},
			},
			expectErr: `invalid element at index 0 of list "destinations": invalid "destination_ref" field: missing required field`,
		},
		"svc/bad type": {
			data: &pbmesh.ComputedImplicitDestinations{
				Destinations: []*pbmesh.ImplicitDestination{
					{DestinationRef: newRefWithTenancy(pbcatalog.WorkloadType, "default.default", "api")},
				},
			},
			expectErr: `invalid element at index 0 of list "destinations": invalid "destination_ref" field: invalid "type" field: reference must have type catalog.v2beta1.Service`,
		},
		"svc/nil tenancy": {
			data: &pbmesh.ComputedImplicitDestinations{
				Destinations: []*pbmesh.ImplicitDestination{
					{DestinationRef: &pbresource.Reference{Type: pbcatalog.ServiceType, Name: "api"}},
				},
			},
			expectErr: `invalid element at index 0 of list "destinations": invalid "destination_ref" field: invalid "tenancy" field: missing required field`,
		},
		"svc/bad dest tenancy/partition": {
			data: &pbmesh.ComputedImplicitDestinations{
				Destinations: []*pbmesh.ImplicitDestination{
					{DestinationRef: newRefWithTenancy(pbcatalog.ServiceType, ".bar", "api")},
				},
			},
			expectErr: `invalid element at index 0 of list "destinations": invalid "destination_ref" field: invalid "tenancy" field: invalid "partition" field: cannot be empty`,
		},
		"svc/bad dest tenancy/namespace": {
			data: &pbmesh.ComputedImplicitDestinations{
				Destinations: []*pbmesh.ImplicitDestination{
					{DestinationRef: newRefWithTenancy(pbcatalog.ServiceType, "foo", "api")},
				},
			},
			expectErr: `invalid element at index 0 of list "destinations": invalid "destination_ref" field: invalid "tenancy" field: invalid "namespace" field: cannot be empty`,
		},
		"no ports": {
			data: &pbmesh.ComputedImplicitDestinations{
				Destinations: []*pbmesh.ImplicitDestination{
					{
						DestinationRef: newRefWithTenancy(pbcatalog.ServiceType, "foo.bar", "api"),
					},
				},
			},
			expectErr: `invalid element at index 0 of list "destinations": invalid "destination_ports" field: cannot be empty`,
		},
		"bad port/empty": {
			data: &pbmesh.ComputedImplicitDestinations{
				Destinations: []*pbmesh.ImplicitDestination{
					{
						DestinationRef:   newRefWithTenancy(pbcatalog.ServiceType, "foo.bar", "api"),
						DestinationPorts: []string{""},
					},
				},
			},
			expectErr: `invalid element at index 0 of list "destinations": invalid element at index 0 of list "destination_ports": cannot be empty`,
		},
		"bad port/no letters": {
			data: &pbmesh.ComputedImplicitDestinations{
				Destinations: []*pbmesh.ImplicitDestination{
					{
						DestinationRef:   newRefWithTenancy(pbcatalog.ServiceType, "foo.bar", "api"),
						DestinationPorts: []string{"1234"},
					},
				},
			},
			expectErr: `invalid element at index 0 of list "destinations": invalid element at index 0 of list "destination_ports": value must be 1-15 characters`,
		},
		"bad port/too long": {
			data: &pbmesh.ComputedImplicitDestinations{
				Destinations: []*pbmesh.ImplicitDestination{
					{
						DestinationRef:   newRefWithTenancy(pbcatalog.ServiceType, "foo.bar", "api"),
						DestinationPorts: []string{strings.Repeat("a", 16)},
					},
				},
			},
			expectErr: `invalid element at index 0 of list "destinations": invalid element at index 0 of list "destination_ports": value must be 1-15 characters`,
		},
		"normal": {
			data: &pbmesh.ComputedImplicitDestinations{
				Destinations: []*pbmesh.ImplicitDestination{
					{
						DestinationRef:   newRefWithTenancy(pbcatalog.ServiceType, "foo.bar", "api"),
						DestinationPorts: []string{"p1"},
					},
					{
						DestinationRef:   newRefWithTenancy(pbcatalog.ServiceType, "foo.zim", "api"),
						DestinationPorts: []string{"p2"},
					},
					{
						DestinationRef:   newRefWithTenancy(pbcatalog.ServiceType, "gir.zim", "api"),
						DestinationPorts: []string{"p3"},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestComputedImplicitDestinationsACLs(t *testing.T) {
	// Wire up a registry to generically invoke hooks
	registry := resource.NewRegistry()
	Register(registry)

	type testcase struct {
		rules   string
		check   func(t *testing.T, authz acl.Authorizer, res *pbresource.Resource)
		readOK  string
		writeOK string
		listOK  string
	}

	const (
		DENY    = "deny"
		ALLOW   = "allow"
		DEFAULT = "default"
	)

	checkF := func(t *testing.T, expect string, got error) {
		switch expect {
		case ALLOW:
			if acl.IsErrPermissionDenied(got) {
				t.Fatal("should be allowed")
			}
		case DENY:
			if !acl.IsErrPermissionDenied(got) {
				t.Fatal("should be denied")
			}
		case DEFAULT:
			require.Nil(t, got, "expected fallthrough decision")
		default:
			t.Fatalf("unexpected expectation: %q", expect)
		}
	}

	reg, ok := registry.Resolve(pbmesh.ComputedImplicitDestinationsType)
	require.True(t, ok)

	run := func(t *testing.T, tc testcase) {
		cidData := &pbmesh.ComputedImplicitDestinations{}
		res := resourcetest.Resource(pbmesh.ComputedImplicitDestinationsType, "wi1").
			WithTenancy(resource.DefaultNamespacedTenancy()).
			WithData(t, cidData).
			Build()
		resourcetest.ValidateAndNormalize(t, registry, res)

		config := acl.Config{
			WildcardName: structs.WildcardSpecifier,
		}
		authz, err := acl.NewAuthorizerFromRules(tc.rules, &config, nil)
		require.NoError(t, err)
		authz = acl.NewChainedAuthorizer([]acl.Authorizer{authz, acl.DenyAll()})

		t.Run("read", func(t *testing.T) {
			err := reg.ACLs.Read(authz, &acl.AuthorizerContext{}, res.Id, res)
			checkF(t, tc.readOK, err)
		})
		t.Run("write", func(t *testing.T) {
			err := reg.ACLs.Write(authz, &acl.AuthorizerContext{}, res)
			checkF(t, tc.writeOK, err)
		})
		t.Run("list", func(t *testing.T) {
			err := reg.ACLs.List(authz, &acl.AuthorizerContext{})
			checkF(t, tc.listOK, err)
		})
	}

	cases := map[string]testcase{
		"no rules": {
			rules:   ``,
			readOK:  DENY,
			writeOK: DENY,
			listOK:  DEFAULT,
		},
		"operator read": {
			rules:   `operator = "read" `,
			readOK:  DENY,
			writeOK: DENY,
			listOK:  DEFAULT,
		},
		"operator write": {
			rules:   `operator = "write" `,
			readOK:  DENY,
			writeOK: ALLOW,
			listOK:  DEFAULT,
		},
		"workload identity w1 read": {
			rules:   `identity "wi1" { policy = "read" }`,
			readOK:  ALLOW,
			writeOK: DENY,
			listOK:  DEFAULT,
		},
		"workload identity w1 write": {
			rules:   `identity "wi1" { policy = "write" }`,
			readOK:  ALLOW,
			writeOK: DENY,
			listOK:  DEFAULT,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}
