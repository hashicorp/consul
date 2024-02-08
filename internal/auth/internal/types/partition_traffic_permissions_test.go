// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"testing"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	pbresource "github.com/hashicorp/consul/proto-public/pbresource/v1"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/stretchr/testify/require"
)

func TestValidatePartitionTrafficPermissions_ParseError(t *testing.T) {
	data := &pbauth.ComputedTrafficPermissions{AllowPermissions: nil}

	res := resourcetest.Resource(pbauth.PartitionTrafficPermissionsType, "ptp").
		WithData(t, data).
		Build()

	err := ValidatePartitionTrafficPermissions(res)
	require.Error(t, err)
	require.ErrorAs(t, err, &resource.ErrDataParse{})
}

func TestValidatePartitionTrafficPermissions(t *testing.T) {
	// TODO: refactor test cases as these are similar to namespace traffic permissions
	cases := map[string]struct {
		id        *pbresource.ID
		ptp       *pbauth.PartitionTrafficPermissions
		expectErr string
	}{
		"ok-minimal": {
			ptp: &pbauth.PartitionTrafficPermissions{
				Action: pbauth.Action_ACTION_ALLOW,
			},
		},
		"unspecified-action": {
			ptp: &pbauth.PartitionTrafficPermissions{
				Action: pbauth.Action_ACTION_UNSPECIFIED,
			},
			expectErr: `invalid "data.action" field`,
		},
		"invalid-action": {
			ptp: &pbauth.PartitionTrafficPermissions{
				Action: pbauth.Action(50),
			},
			expectErr: `invalid "data.action" field`,
		},
		"source-tenancy": {
			ptp: &pbauth.PartitionTrafficPermissions{
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
					},
				},
			},
			expectErr: `invalid element at index 0 of list "permissions": invalid element at index 0 of list "sources": invalid element at index 0 of list "source": permissions sources may not specify partitions, peers, and sameness_groups together`,
		},
		"source-has-same-tenancy-as-tp": {
			id: &pbresource.ID{
				Tenancy: &pbresource.Tenancy{
					Partition: resource.DefaultPartitionName,
				},
			},
			ptp: &pbauth.PartitionTrafficPermissions{
				Action: pbauth.Action_ACTION_ALLOW,
				Permissions: []*pbauth.Permission{
					{
						Sources: []*pbauth.Source{
							{
								Partition:     resource.DefaultPartitionName,
								Peer:          resource.DefaultPeerName,
								SamenessGroup: "",
							},
						},
					},
				},
			},
		},
		"source-has-partition-set": {
			id: &pbresource.ID{
				Tenancy: &pbresource.Tenancy{
					Partition: resource.DefaultPartitionName,
				},
			},
			ptp: &pbauth.PartitionTrafficPermissions{
				Action: pbauth.Action_ACTION_ALLOW,
				Permissions: []*pbauth.Permission{
					{
						Sources: []*pbauth.Source{
							{
								Partition:     "pt1",
								Peer:          resource.DefaultPeerName,
								SamenessGroup: "",
							},
						},
					},
				},
			},
		},
		"source-has-peer-set": {
			id: &pbresource.ID{
				Tenancy: &pbresource.Tenancy{
					Partition: resource.DefaultPartitionName,
				},
			},
			ptp: &pbauth.PartitionTrafficPermissions{
				Action: pbauth.Action_ACTION_ALLOW,
				Permissions: []*pbauth.Permission{
					{
						Sources: []*pbauth.Source{
							{
								Partition:     resource.DefaultPartitionName,
								Peer:          "peer",
								SamenessGroup: "",
							},
						},
					},
				},
			},
		},
		"source-has-sameness-group-set": {
			id: &pbresource.ID{
				Tenancy: &pbresource.Tenancy{
					Partition: resource.DefaultPartitionName,
				},
			},
			ptp: &pbauth.PartitionTrafficPermissions{
				Action: pbauth.Action_ACTION_ALLOW,
				Permissions: []*pbauth.Permission{
					{
						Sources: []*pbauth.Source{
							{
								Partition:     resource.DefaultPartitionName,
								Peer:          resource.DefaultPeerName,
								SamenessGroup: "sg1",
							},
						},
					},
				},
			},
		},
		"source-has-peer-and-partition-set": {
			id: &pbresource.ID{
				Tenancy: &pbresource.Tenancy{
					Partition: resource.DefaultPartitionName,
				},
			},
			ptp: &pbauth.PartitionTrafficPermissions{
				Action: pbauth.Action_ACTION_ALLOW,
				Permissions: []*pbauth.Permission{
					{
						Sources: []*pbauth.Source{
							{
								Partition:     "part",
								Peer:          "peer",
								SamenessGroup: "",
							},
						},
					},
				},
			},
			expectErr: `invalid element at index 0 of list "permissions": invalid element at index 0 of list "sources": invalid element at index 0 of list "source": permissions sources may not specify partitions, peers, and sameness_groups together`,
		},
		"source-has-sameness-group-and-partition-set": {
			id: &pbresource.ID{
				Tenancy: &pbresource.Tenancy{
					Partition: resource.DefaultPartitionName,
				},
			},
			ptp: &pbauth.PartitionTrafficPermissions{
				Action: pbauth.Action_ACTION_ALLOW,
				Permissions: []*pbauth.Permission{
					{
						Sources: []*pbauth.Source{
							{
								Partition:     "part",
								Peer:          resource.DefaultPeerName,
								SamenessGroup: "sg1",
							},
						},
					},
				},
			},
			expectErr: `invalid element at index 0 of list "permissions": invalid element at index 0 of list "sources": invalid element at index 0 of list "source": permissions sources may not specify partitions, peers, and sameness_groups together`,
		},
		"source-has-sameness-group-and-partition-peer-set": {
			id: &pbresource.ID{
				Tenancy: &pbresource.Tenancy{
					Partition: resource.DefaultPartitionName,
				},
			},
			ptp: &pbauth.PartitionTrafficPermissions{
				Action: pbauth.Action_ACTION_ALLOW,
				Permissions: []*pbauth.Permission{
					{
						Sources: []*pbauth.Source{
							{
								Partition:     "part",
								Peer:          "peer",
								SamenessGroup: "sg1",
							},
						},
					},
				},
			},
			expectErr: `invalid element at index 0 of list "permissions": invalid element at index 0 of list "sources": invalid element at index 0 of list "source": permissions sources may not specify partitions, peers, and sameness_groups together`,
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			resBuilder := resourcetest.Resource(pbauth.PartitionTrafficPermissionsType, "ptp").
				WithData(t, tc.ptp)
			if tc.id != nil {
				resBuilder = resBuilder.WithTenancy(tc.id.Tenancy)
			}
			res := resBuilder.Build()

			err := ValidatePartitionTrafficPermissions(res)
			if tc.expectErr == "" {
				require.NoError(t, err)
			} else {
				testutil.RequireErrorContains(t, err, tc.expectErr)
			}
		})
	}
}

func TestValidatePartitionTrafficPermissions_Permissions(t *testing.T) {
	for n, tc := range permissionsTestCases() {
		t.Run(n, func(t *testing.T) {
			tp := &pbauth.PartitionTrafficPermissions{
				Action:      pbauth.Action_ACTION_ALLOW,
				Permissions: []*pbauth.Permission{tc.p},
			}

			res := resourcetest.Resource(pbauth.PartitionTrafficPermissionsType, "ptp").
				WithTenancy(resource.DefaultPartitionedTenancy()).
				WithData(t, tp).
				Build()

			err := MutatePartitionTrafficPermissions(res)
			require.NoError(t, err)

			err = ValidatePartitionTrafficPermissions(res)
			if tc.expectErr == "" {
				require.NoError(t, err)
			} else {
				testutil.RequireErrorContains(t, err, tc.expectErr)
			}
		})
	}
}

func TestMutatePartitionTrafficPermissions(t *testing.T) {
	type testcase struct {
		policyTenancy *pbresource.Tenancy
		ptp           *pbauth.PartitionTrafficPermissions
		expect        *pbauth.PartitionTrafficPermissions
	}

	run := func(t *testing.T, tc testcase) {
		tenancy := tc.policyTenancy
		if tenancy == nil {
			tenancy = resource.DefaultPartitionedTenancy()
		}
		res := resourcetest.Resource(pbauth.PartitionTrafficPermissionsType, "ptp").
			WithTenancy(tenancy).
			WithData(t, tc.ptp).
			Build()

		err := MutatePartitionTrafficPermissions(res)

		got := resourcetest.MustDecode[*pbauth.PartitionTrafficPermissions](t, res)
		require.NoError(t, err)
		prototest.AssertDeepEqual(t, tc.expect, got.Data)
	}

	cases := map[string]testcase{
		"empty-1": {
			ptp:    &pbauth.PartitionTrafficPermissions{},
			expect: &pbauth.PartitionTrafficPermissions{},
		},
		"kitchen-sink-default-partition": {
			ptp: &pbauth.PartitionTrafficPermissions{
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
			expect: &pbauth.PartitionTrafficPermissions{
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
			ptp: &pbauth.PartitionTrafficPermissions{
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
			expect: &pbauth.PartitionTrafficPermissions{
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
			},
			ptp: &pbauth.PartitionTrafficPermissions{
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
			expect: &pbauth.PartitionTrafficPermissions{
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
			},
			ptp: &pbauth.PartitionTrafficPermissions{
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
			expect: &pbauth.PartitionTrafficPermissions{
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

func TestPartitionTrafficPermissionsACLs(t *testing.T) {
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

	reg, ok := registry.Resolve(pbauth.PartitionTrafficPermissionsType)
	require.True(t, ok)

	run := func(t *testing.T, tc testcase) {
		tpData := &pbauth.PartitionTrafficPermissions{
			Action: pbauth.Action_ACTION_ALLOW,
		}
		res := resourcetest.Resource(pbauth.PartitionTrafficPermissionsType, "ptp1").
			WithTenancy(resource.DefaultPartitionedTenancy()).
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
		"mesh read": {
			rules:   `mesh = "read"`,
			readOK:  ALLOW,
			writeOK: DENY,
			listOK:  DEFAULT,
		},
		"partition write": {
			rules:   `mesh = "write"`,
			readOK:  ALLOW,
			writeOK: ALLOW,
			listOK:  DEFAULT,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}
