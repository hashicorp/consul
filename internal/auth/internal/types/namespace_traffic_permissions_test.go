// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/version/versiontest"
)

func TestValidateNamespaceTrafficPermissions_ParseError(t *testing.T) {
	data := &pbauth.ComputedTrafficPermissions{AllowPermissions: nil}

	res := resourcetest.Resource(pbauth.NamespaceTrafficPermissionsType, "tp").
		WithData(t, data).
		Build()

	err := ValidateTrafficPermissions(res)
	require.Error(t, err)
	require.ErrorAs(t, err, &resource.ErrDataParse{})
}

func TestValidateNamespaceTrafficPermissions(t *testing.T) {
	cases := map[string]struct {
		ntp       *pbauth.NamespaceTrafficPermissions
		expectErr string
	}{
		"ok-minimal": {
			ntp: &pbauth.NamespaceTrafficPermissions{
				Action: pbauth.Action_ACTION_ALLOW,
			},
		},
		"unspecified-action": {
			// Any type other than the TrafficPermissions type would work
			// to cause the error we are expecting
			ntp: &pbauth.NamespaceTrafficPermissions{
				Action:      pbauth.Action_ACTION_UNSPECIFIED,
				Permissions: nil,
			},
			expectErr: `invalid "data.action" field: action must be either allow or deny`,
		},
		"invalid-action": {
			ntp: &pbauth.NamespaceTrafficPermissions{
				Action:      pbauth.Action(50),
				Permissions: nil,
			},
			expectErr: `invalid "data.action" field: action must be either allow or deny`,
		},
		"source-tenancy": {
			ntp: &pbauth.NamespaceTrafficPermissions{
				Action: pbauth.Action_ACTION_ALLOW,
				Permissions: []*pbauth.Permission{
					{
						Sources: []*pbauth.Source{
							{
								Partition:     "ap1",
								Peer:          "cl1",
								SamenessGroup: "sg1",
							},
						},
						DestinationRules: nil,
					},
				},
			},
			expectErr: `invalid element at index 0 of list "permissions": invalid element at index 0 of list "sources": invalid element at index 0 of list "source": permissions sources may not specify partitions, peers, and sameness_groups together`,
		},
		// TODO: remove when L7 traffic permissions are implemented
		"l7-fields-path": {
			ntp: &pbauth.NamespaceTrafficPermissions{
				Action: pbauth.Action_ACTION_ALLOW,
				Permissions: []*pbauth.Permission{
					{
						Sources: []*pbauth.Source{
							{
								Partition: "ap1",
							},
						},
						DestinationRules: []*pbauth.DestinationRule{
							{
								PathExact: "wi2",
							},
						},
					},
				},
			},
			expectErr: `invalid element at index 0 of list "permissions": invalid element at index 0 of list "destination_rules": invalid element at index 0 of list "destination_rule": traffic permissions with L7 rules are not yet supported`,
		},
		"l7-fields-methods": {
			ntp: &pbauth.NamespaceTrafficPermissions{
				Action: pbauth.Action_ACTION_ALLOW,
				Permissions: []*pbauth.Permission{
					{
						Sources: []*pbauth.Source{
							{
								Partition: "ap1",
							},
						},
						DestinationRules: []*pbauth.DestinationRule{
							{
								Methods: []string{"PUT"},
							},
						},
					},
				},
			},
			expectErr: `invalid element at index 0 of list "permissions": invalid element at index 0 of list "destination_rules": invalid element at index 0 of list "destination_rule": traffic permissions with L7 rules are not yet supported`,
		},
		"l7-fields-header": {
			ntp: &pbauth.NamespaceTrafficPermissions{
				Action: pbauth.Action_ACTION_ALLOW,
				Permissions: []*pbauth.Permission{
					{
						Sources: []*pbauth.Source{
							{
								Partition: "ap1",
							},
						},
						DestinationRules: []*pbauth.DestinationRule{
							{
								Header: &pbauth.DestinationRuleHeader{Name: "foo"},
							},
						},
					},
				},
			},
			expectErr: `invalid element at index 0 of list "permissions": invalid element at index 0 of list "destination_rules": invalid element at index 0 of list "destination_rule": traffic permissions with L7 rules are not yet supported`,
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			res := resourcetest.Resource(pbauth.NamespaceTrafficPermissionsType, "ntp").
				WithData(t, tc.ntp).
				Build()

			err := ValidateNamespaceTrafficPermissions(res)
			if tc.expectErr == "" {
				require.NoError(t, err)
			} else {
				testutil.RequireErrorContains(t, err, tc.expectErr)
			}
		})
	}
}

func TestValidateNamespaceTrafficPermissions_Permissions(t *testing.T) {
	for n, tc := range permissionsTestCases() {
		t.Run(n, func(t *testing.T) {
			tp := &pbauth.NamespaceTrafficPermissions{
				Action:      pbauth.Action_ACTION_ALLOW,
				Permissions: []*pbauth.Permission{tc.p},
			}

			res := resourcetest.Resource(pbauth.NamespaceTrafficPermissionsType, "tp").
				WithTenancy(resource.DefaultNamespacedTenancy()).
				WithData(t, tp).
				Build()

			err := MutateNamespaceTrafficPermissions(res)
			require.NoError(t, err)

			err = ValidateNamespaceTrafficPermissions(res)
			if tc.expectErr == "" {
				require.NoError(t, err)
			} else {
				testutil.RequireErrorContains(t, err, tc.expectErr)
			}
		})
	}
}

func TestMutateNamespaceTrafficPermissions(t *testing.T) {
	type testcase struct {
		policyTenancy *pbresource.Tenancy
		tp            *pbauth.NamespaceTrafficPermissions
		expect        *pbauth.NamespaceTrafficPermissions
		expectErr     string
	}

	run := func(t *testing.T, tc testcase) {
		tenancy := tc.policyTenancy
		if tenancy == nil {
			tenancy = resource.DefaultNamespacedTenancy()
		}
		res := resourcetest.Resource(pbauth.NamespaceTrafficPermissionsType, "ntp").
			WithTenancy(tenancy).
			WithData(t, tc.tp).
			Build()

		err := MutateNamespaceTrafficPermissions(res)

		got := resourcetest.MustDecode[*pbauth.NamespaceTrafficPermissions](t, res)

		if tc.expectErr == "" {
			require.NoError(t, err)
			prototest.AssertDeepEqual(t, tc.expect, got.Data)
		} else {
			testutil.RequireErrorContains(t, err, tc.expectErr)
		}
	}

	cases := map[string]testcase{
		"empty-1": {
			tp:     &pbauth.NamespaceTrafficPermissions{},
			expect: &pbauth.NamespaceTrafficPermissions{},
		},
		"kitchen-sink-default-partition": {
			tp: &pbauth.NamespaceTrafficPermissions{
				Permissions: []*pbauth.Permission{
					{
						Sources: []*pbauth.Source{
							{},
							{
								Peer: "not-default",
							},
							{
								Namespace: "ns1",
							},
							{
								IdentityName: "i1",
								Namespace:    "ns1",
								Partition:    "ap1",
							},
							{
								IdentityName: "i1",
								Namespace:    "ns1",
								Peer:         "local",
							},
						},
					},
				},
			},
			expect: &pbauth.NamespaceTrafficPermissions{
				Permissions: []*pbauth.Permission{
					{
						Sources: []*pbauth.Source{
							{
								Partition: "default",
								Peer:      "local",
							},
							{
								Peer: "not-default",
							},
							{
								Namespace: "ns1",
								Partition: "default",
								Peer:      "local",
							},
							{
								IdentityName: "i1",
								Namespace:    "ns1",
								Partition:    "ap1",
								Peer:         "local",
							},
							{
								IdentityName: "i1",
								Namespace:    "ns1",
								Partition:    "default",
								Peer:         "local",
							},
						},
					},
				},
			},
		},
		"kitchen-sink-excludes-default-partition": {
			tp: &pbauth.NamespaceTrafficPermissions{
				Permissions: []*pbauth.Permission{
					{
						Sources: []*pbauth.Source{
							{
								Exclude: []*pbauth.ExcludeSource{
									{},
									{
										Peer: "not-default",
									},
									{
										Namespace: "ns1",
									},
									{
										IdentityName: "i1",
										Namespace:    "ns1",
										Partition:    "ap1",
									},
									{
										IdentityName: "i1",
										Namespace:    "ns1",
										Peer:         "local",
									},
								},
							},
						},
					},
				},
			},
			expect: &pbauth.NamespaceTrafficPermissions{
				Permissions: []*pbauth.Permission{
					{
						Sources: []*pbauth.Source{
							{
								Partition: "default",
								Peer:      "local",
								Exclude: []*pbauth.ExcludeSource{
									{
										Partition: "default",
										Peer:      "local",
									},
									{
										Peer: "not-default",
									},
									{
										Namespace: "ns1",
										Partition: "default",
										Peer:      "local",
									},
									{
										IdentityName: "i1",
										Namespace:    "ns1",
										Partition:    "ap1",
										Peer:         "local",
									},
									{
										IdentityName: "i1",
										Namespace:    "ns1",
										Partition:    "default",
										Peer:         "local",
									},
								},
							},
						},
					},
				},
			},
		},
		"kitchen-sink-non-default-partition": {
			policyTenancy: &pbresource.Tenancy{
				Partition: "ap1",
				Namespace: "ns3",
				PeerName:  "local",
			},
			tp: &pbauth.NamespaceTrafficPermissions{
				Permissions: []*pbauth.Permission{
					{
						Sources: []*pbauth.Source{
							{},
							{
								Peer: "not-default",
							},
							{
								Namespace: "ns1",
							},
							{
								IdentityName: "i1",
								Namespace:    "ns1",
								Partition:    "ap5",
							},
							{
								IdentityName: "i1",
								Namespace:    "ns1",
								Peer:         "local",
							},
							{
								IdentityName: "i2",
							},
							{
								IdentityName: "i2",
								Partition:    "non-default",
							},
						},
					},
				},
			},
			expect: &pbauth.NamespaceTrafficPermissions{
				Permissions: []*pbauth.Permission{
					{
						Sources: []*pbauth.Source{
							{
								Partition: "ap1",
								Namespace: "",
								Peer:      "local",
							},
							{
								Peer: "not-default",
							},
							{
								Namespace: "ns1",
								Partition: "ap1",
								Peer:      "local",
							},
							{
								IdentityName: "i1",
								Namespace:    "ns1",
								Partition:    "ap5",
								Peer:         "local",
							},
							{
								IdentityName: "i1",
								Namespace:    "ns1",
								Partition:    "ap1",
								Peer:         "local",
							},
							{
								IdentityName: "i2",
								Namespace:    "ns3",
								Partition:    "ap1",
								Peer:         "local",
							},
							{
								IdentityName: "i2",
								Namespace:    "default",
								Partition:    "non-default",
								Peer:         "local",
							},
						},
					},
				},
			},
		},
		"kitchen-sink-excludes-non-default-partition": {
			policyTenancy: &pbresource.Tenancy{
				Partition: "ap1",
				Namespace: "ns3",
				PeerName:  "local",
			},
			tp: &pbauth.NamespaceTrafficPermissions{
				Permissions: []*pbauth.Permission{
					{
						Sources: []*pbauth.Source{
							{
								Exclude: []*pbauth.ExcludeSource{
									{},
									{
										Peer: "not-default",
									},
									{
										Namespace: "ns1",
									},
									{
										IdentityName: "i1",
										Namespace:    "ns1",
										Partition:    "ap5",
									},
									{
										IdentityName: "i1",
										Namespace:    "ns1",
										Peer:         "local",
									},
									{
										IdentityName: "i2",
									},
								},
							},
						},
					},
				},
			},
			expect: &pbauth.NamespaceTrafficPermissions{
				Permissions: []*pbauth.Permission{
					{
						Sources: []*pbauth.Source{
							{
								Partition: "ap1",
								Peer:      "local",
								Exclude: []*pbauth.ExcludeSource{
									{
										Partition: "ap1",
										Namespace: "",
										Peer:      "local",
									},
									{
										Peer: "not-default",
									},
									{
										Namespace: "ns1",
										Partition: "ap1",
										Peer:      "local",
									},
									{
										IdentityName: "i1",
										Namespace:    "ns1",
										Partition:    "ap5",
										Peer:         "local",
									},
									{
										IdentityName: "i1",
										Namespace:    "ns1",
										Partition:    "ap1",
										Peer:         "local",
									},
									{
										IdentityName: "i2",
										Namespace:    "ns3",
										Partition:    "ap1",
										Peer:         "local",
									},
								},
							},
						},
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

func TestNamespaceTrafficPermissionsACLs(t *testing.T) {
	// Wire up a registry to generically invoke hooks
	registry := resource.NewRegistry()
	Register(registry)

	type testcase struct {
		rules   string
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

	reg, ok := registry.Resolve(pbauth.NamespaceTrafficPermissionsType)
	require.True(t, ok)

	run := func(t *testing.T, tc testcase) {
		tpData := &pbauth.NamespaceTrafficPermissions{
			Action: pbauth.Action_ACTION_ALLOW,
		}
		res := resourcetest.Resource(pbauth.NamespaceTrafficPermissionsType, "ntp1").
			WithTenancy(resource.DefaultNamespacedTenancy()).
			WithData(t, tpData).
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

	// TODO: remove once namespaces are available in CE
	enterpriseAllow := func() string {
		if versiontest.IsEnterprise() {
			return ALLOW
		}
		return DENY
	}

	cases := map[string]testcase{
		"no rules": {
			rules:   ``,
			readOK:  DENY,
			writeOK: DENY,
			listOK:  DEFAULT,
		},
		"operator read": {
			rules:   `operator = "read"`,
			readOK:  ALLOW,
			writeOK: DENY,
			listOK:  DEFAULT,
		},
		"operator write": {
			rules:   `operator = "write"`,
			readOK:  ALLOW,
			writeOK: ALLOW,
			listOK:  DEFAULT,
		},
		"namespace read": {
			rules:   `namespace "default" { policy = "read" }`,
			readOK:  enterpriseAllow(),
			writeOK: DENY,
			listOK:  DEFAULT,
		},
		"namespace write": {
			rules:   `namespace "default" { policy = "write" }`,
			readOK:  enterpriseAllow(),
			writeOK: enterpriseAllow(),
			listOK:  DEFAULT,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}
