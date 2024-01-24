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
)

func TestValidateTrafficPermissions_ParseError(t *testing.T) {
	data := &pbauth.ComputedTrafficPermissions{AllowPermissions: nil}

	res := resourcetest.Resource(pbauth.TrafficPermissionsType, "tp").
		WithData(t, data).
		Build()

	err := ValidateTrafficPermissions(res)
	require.Error(t, err)
	require.ErrorAs(t, err, &resource.ErrDataParse{})
}

func TestValidateTrafficPermissions(t *testing.T) {
	cases := map[string]struct {
		tp        *pbauth.TrafficPermissions
		id        *pbresource.ID
		expectErr string
	}{
		"ok-minimal": {
			tp: &pbauth.TrafficPermissions{
				Destination: &pbauth.Destination{IdentityName: "wi-1"},
				Action:      pbauth.Action_ACTION_ALLOW,
			},
		},
		"unspecified-action": {
			// Any type other than the TrafficPermissions type would work
			// to cause the error we are expecting
			tp: &pbauth.TrafficPermissions{
				Destination: &pbauth.Destination{
					IdentityName: "wi1",
				},
				Action:      pbauth.Action_ACTION_UNSPECIFIED,
				Permissions: nil,
			},
			expectErr: `invalid "data.action" field: action must be either allow or deny`,
		},
		"invalid-action": {
			tp: &pbauth.TrafficPermissions{
				Destination: &pbauth.Destination{
					IdentityName: "wi1",
				},
				Action:      pbauth.Action(50),
				Permissions: nil,
			},
			expectErr: `invalid "data.action" field: action must be either allow or deny`,
		},
		"no-destination": {
			tp: &pbauth.TrafficPermissions{
				Action:      pbauth.Action_ACTION_ALLOW,
				Permissions: nil,
			},
			expectErr: `invalid "data.destination" field: cannot be empty`,
		},
		"source-tenancy": {
			tp: &pbauth.TrafficPermissions{
				Destination: &pbauth.Destination{
					IdentityName: "w1",
				},
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
			tp: &pbauth.TrafficPermissions{
				Destination: &pbauth.Destination{
					IdentityName: "w1",
				},
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
			tp: &pbauth.TrafficPermissions{
				Destination: &pbauth.Destination{
					IdentityName: "w1",
				},
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
			tp: &pbauth.TrafficPermissions{
				Destination: &pbauth.Destination{
					IdentityName: "w1",
				},
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
		"source-has-same-tenancy-as-tp": {
			id: &pbresource.ID{
				Tenancy: &pbresource.Tenancy{
					Partition: resource.DefaultPartitionName,
				},
			},
			tp: &pbauth.TrafficPermissions{
				Destination: &pbauth.Destination{
					IdentityName: "w1",
				},
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
			tp: &pbauth.TrafficPermissions{
				Destination: &pbauth.Destination{
					IdentityName: "w1",
				},
				Action: pbauth.Action_ACTION_ALLOW,
				Permissions: []*pbauth.Permission{
					{
						Sources: []*pbauth.Source{
							{
								Partition:     "part",
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
			tp: &pbauth.TrafficPermissions{
				Destination: &pbauth.Destination{
					IdentityName: "w1",
				},
				Action: pbauth.Action_ACTION_ALLOW,
				Permissions: []*pbauth.Permission{
					{
						Sources: []*pbauth.Source{
							{
								Partition:     resource.DefaultNamespaceName,
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
			tp: &pbauth.TrafficPermissions{
				Destination: &pbauth.Destination{
					IdentityName: "w1",
				},
				Action: pbauth.Action_ACTION_ALLOW,
				Permissions: []*pbauth.Permission{
					{
						Sources: []*pbauth.Source{
							{
								Partition:     resource.DefaultNamespaceName,
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
			tp: &pbauth.TrafficPermissions{
				Destination: &pbauth.Destination{
					IdentityName: "w1",
				},
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
			tp: &pbauth.TrafficPermissions{
				Destination: &pbauth.Destination{
					IdentityName: "w1",
				},
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
			tp: &pbauth.TrafficPermissions{
				Destination: &pbauth.Destination{
					IdentityName: "w1",
				},
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
			resBuilder := resourcetest.Resource(pbauth.TrafficPermissionsType, "tp").
				WithData(t, tc.tp)
			if tc.id != nil {
				resBuilder = resBuilder.WithTenancy(tc.id.Tenancy)
			}
			res := resBuilder.Build()

			err := ValidateTrafficPermissions(res)
			if tc.expectErr == "" {
				require.NoError(t, err)
			} else {
				testutil.RequireErrorContains(t, err, tc.expectErr)
			}
		})
	}
}

type permissionTestCase struct {
	p         *pbauth.Permission
	expectErr string
}

func permissionsTestCases() map[string]permissionTestCase {
	return map[string]permissionTestCase{
		"empty": {
			p:         &pbauth.Permission{},
			expectErr: `invalid "sources" field: cannot be empty`,
		},
		"empty-sources": {
			p: &pbauth.Permission{
				Sources: nil,
				DestinationRules: []*pbauth.DestinationRule{
					{
						PathPrefix: "foo",
						Exclude: []*pbauth.ExcludePermissionRule{
							{
								PathExact: "baz",
							},
						},
					},
					{
						PathPrefix: "bar",
					},
				},
			},
			expectErr: `invalid "sources" field: cannot be empty`,
		},
		"empty-destination-rules": {
			p: &pbauth.Permission{
				Sources: []*pbauth.Source{
					{
						// wildcard identity name
						Namespace: "ns1",
					},
					{
						Namespace: "ns1",
						Exclude: []*pbauth.ExcludeSource{
							// wildcard identity name
							{Namespace: "ns1"},
						},
					},
					{
						IdentityName: "wi-3",
						Namespace:    "ns1",
					},
				},
			},
		},
		"explicit source with excludes": {
			p: &pbauth.Permission{
				Sources: []*pbauth.Source{
					{
						IdentityName: "i1",
						Exclude: []*pbauth.ExcludeSource{
							{
								IdentityName: "i1",
							},
						},
					},
				},
			},
			expectErr: `invalid "exclude_sources" field: must be defined on wildcard sources`,
		},
		"source-partition-and-peer": {
			p: &pbauth.Permission{
				Sources: []*pbauth.Source{
					{
						Partition: "ap1",
						Peer:      "cluster-01",
					},
				},
			},
			expectErr: `permissions sources may not specify partitions, peers, and sameness_groups together`,
		},
		"source-partition-and-sameness-group": {
			p: &pbauth.Permission{
				Sources: []*pbauth.Source{
					{
						Partition:     "ap1",
						SamenessGroup: "sg-1",
					},
				},
			},
			expectErr: `permissions sources may not specify partitions, peers, and sameness_groups together`,
		},
		"source-peer-and-sameness-group": {
			p: &pbauth.Permission{
				Sources: []*pbauth.Source{
					{
						Partition:     "ap1",
						SamenessGroup: "sg-1",
					},
				},
			},
			expectErr: `permissions sources may not specify partitions, peers, and sameness_groups together`,
		},
		"exclude-source-partition-and-peer": {
			p: &pbauth.Permission{
				Sources: []*pbauth.Source{
					{
						Exclude: []*pbauth.ExcludeSource{
							{
								Partition: "ap1",
								Peer:      "cluster-01",
							},
						},
					},
				},
			},
			expectErr: `permissions sources may not specify partitions, peers, and sameness_groups together`,
		},
		"exclude-source-partition-and-sameness-group": {
			p: &pbauth.Permission{
				Sources: []*pbauth.Source{
					{
						Exclude: []*pbauth.ExcludeSource{
							{
								Partition:     "ap1",
								SamenessGroup: "sg-1",
							},
						},
					},
				},
			},
			expectErr: `permissions sources may not specify partitions, peers, and sameness_groups together`,
		},
		"exclude-source-peer-and-sameness-group": {
			p: &pbauth.Permission{
				Sources: []*pbauth.Source{
					{
						Exclude: []*pbauth.ExcludeSource{
							{
								Peer:          "ap1",
								SamenessGroup: "sg-1",
							},
						},
					},
				},
			},
			expectErr: `permissions sources may not specify partitions, peers, and sameness_groups together`,
		},
		"destination-rule-path-prefix-regex": {
			p: &pbauth.Permission{
				Sources: nil,
				DestinationRules: []*pbauth.DestinationRule{
					{
						PathExact:  "wi2",
						PathPrefix: "wi",
						PathRegex:  "wi.*",
					},
				},
			},
			expectErr: `invalid element at index 0 of list "destination_rule": prefix values, regex values, and explicit names must not combined`,
		},
	}
}

func TestValidateTrafficPermissions_Permissions(t *testing.T) {
	for n, tc := range permissionsTestCases() {
		t.Run(n, func(t *testing.T) {
			tp := &pbauth.TrafficPermissions{
				Action: pbauth.Action_ACTION_ALLOW,
				Destination: &pbauth.Destination{
					IdentityName: "w1",
				},
				Permissions: []*pbauth.Permission{tc.p},
			}

			res := resourcetest.Resource(pbauth.TrafficPermissionsType, "tp").
				WithTenancy(resource.DefaultNamespacedTenancy()).
				WithData(t, tp).
				Build()

			err := MutateTrafficPermissions(res)
			require.NoError(t, err)

			err = ValidateTrafficPermissions(res)
			if tc.expectErr == "" {
				require.NoError(t, err)
			} else {
				testutil.RequireErrorContains(t, err, tc.expectErr)
			}
		})
	}
}

func TestMutateTrafficPermissions(t *testing.T) {
	type testcase struct {
		policyTenancy *pbresource.Tenancy
		tp            *pbauth.TrafficPermissions
		expect        *pbauth.TrafficPermissions
		expectErr     string
	}

	run := func(t *testing.T, tc testcase) {
		tenancy := tc.policyTenancy
		if tenancy == nil {
			tenancy = resource.DefaultNamespacedTenancy()
		}
		res := resourcetest.Resource(pbauth.TrafficPermissionsType, "api").
			WithTenancy(tenancy).
			WithData(t, tc.tp).
			Build()

		err := MutateTrafficPermissions(res)

		got := resourcetest.MustDecode[*pbauth.TrafficPermissions](t, res)

		if tc.expectErr == "" {
			require.NoError(t, err)
			prototest.AssertDeepEqual(t, tc.expect, got.Data)
		} else {
			testutil.RequireErrorContains(t, err, tc.expectErr)
		}
	}

	cases := map[string]testcase{
		"empty-1": {
			tp:     &pbauth.TrafficPermissions{},
			expect: &pbauth.TrafficPermissions{},
		},
		"kitchen-sink-default-partition": {
			tp: &pbauth.TrafficPermissions{
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
			expect: &pbauth.TrafficPermissions{
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
			tp: &pbauth.TrafficPermissions{
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
			expect: &pbauth.TrafficPermissions{
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
			tp: &pbauth.TrafficPermissions{
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
			expect: &pbauth.TrafficPermissions{
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
			tp: &pbauth.TrafficPermissions{
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
			expect: &pbauth.TrafficPermissions{
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

func TestTrafficPermissionsACLs(t *testing.T) {
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

	reg, ok := registry.Resolve(pbauth.TrafficPermissionsType)
	require.True(t, ok)

	run := func(t *testing.T, tc testcase) {
		tpData := &pbauth.TrafficPermissions{
			Destination: &pbauth.Destination{IdentityName: "wi1"},
			Action:      pbauth.Action_ACTION_ALLOW,
		}
		res := resourcetest.Resource(pbauth.TrafficPermissionsType, "tp1").
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

	cases := map[string]testcase{
		"no rules": {
			rules:   ``,
			readOK:  DENY,
			writeOK: DENY,
			listOK:  DEFAULT,
		},
		"workload identity w1 read, no intentions": {
			rules:   `identity "wi1" { policy = "read" }`,
			readOK:  ALLOW,
			writeOK: DENY,
			listOK:  DEFAULT,
		},
		"workload identity w1 read, deny intentions": {
			rules:   `identity "wi1" { policy = "read", intentions = "deny" }`,
			readOK:  DENY,
			writeOK: DENY,
			listOK:  DEFAULT,
		},
		"workload identity w1 read, intentions read": {
			rules:   `identity "wi1" { policy = "read", intentions = "read" }`,
			readOK:  ALLOW,
			writeOK: DENY,
			listOK:  DEFAULT,
		},
		"workload identity w1 write, write intentions": {
			rules:   `identity "wi1" { policy = "read", intentions = "write" }`,
			readOK:  ALLOW,
			writeOK: ALLOW,
			listOK:  DEFAULT,
		},
		"workload identity w1 write, deny intentions": {
			rules:   `identity "wi1" { policy = "write", intentions = "deny" }`,
			readOK:  DENY,
			writeOK: DENY,
			listOK:  DEFAULT,
		},
		"workload identity w1 write, intentions read": {
			rules:   `identity "wi1" { policy = "write", intentions = "read" }`,
			readOK:  ALLOW,
			writeOK: DENY,
			listOK:  DEFAULT,
		},
		"workload identity w1 write, intentions write": {
			rules:   `identity "wi1" { policy = "write", intentions = "write" }`,
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
