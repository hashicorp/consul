// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/version/versiontest"
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
	const (
		TrafficPermissions = 1 << iota
		NamespaceTrafficPermissions
		PartitionTrafficPermissions
	)
	all := TrafficPermissions | NamespaceTrafficPermissions | PartitionTrafficPermissions

	cases := map[string]struct {
		// bitmask of what xTrafficPermissions to test
		xTPs int

		// following fields will be used to construct all the xTrafficPermissions
		destination *pbauth.Destination // used only by TrafficPermissions
		action      pbauth.Action
		permissions []*pbauth.Permission

		id         *pbresource.ID
		expectErr  string
		enterprise bool
	}{
		"ok-minimal": {
			xTPs:        all,
			destination: &pbauth.Destination{IdentityName: "wi-1"},
			action:      pbauth.Action_ACTION_ALLOW,
		},
		"ok-permissions": {
			xTPs:        all,
			destination: &pbauth.Destination{IdentityName: "wi-1"},
			action:      pbauth.Action_ACTION_ALLOW,
			permissions: []*pbauth.Permission{
				{
					Sources: []*pbauth.Source{
						{
							IdentityName: "wi-2",
							Namespace:    "default",
							Partition:    "default",
						},
						{
							IdentityName: "wi-1",
							Namespace:    "default",
							Partition:    "ap1",
						},
					},
					DestinationRules: []*pbauth.DestinationRule{
						{
							PathPrefix: "/",
							Methods:    []string{"GET"},
							Headers:    []*pbauth.DestinationRuleHeader{{Name: "X-Consul-Token", Present: false, Invert: true}},
							PortNames:  []string{"https"},
							Exclude:    []*pbauth.ExcludePermissionRule{{PathExact: "/admin"}},
						},
					},
				},
			},
		},
		"unspecified-action": {
			xTPs:        all,
			destination: &pbauth.Destination{IdentityName: "wi-1"},
			action:      pbauth.Action_ACTION_UNSPECIFIED,
			expectErr:   `invalid "data.action" field`,
		},
		"invalid-action": {
			xTPs:        all,
			destination: &pbauth.Destination{IdentityName: "wi-1"},
			action:      pbauth.Action(50),
			expectErr:   `invalid "data.action" field`,
		},
		"deny-action": {
			xTPs:        all,
			destination: &pbauth.Destination{IdentityName: "wi-1"},
			action:      pbauth.Action_ACTION_DENY,
			expectErr:   `invalid "data.action" field`,
			enterprise:  true,
		},
		"no-destination": {
			xTPs:        TrafficPermissions,
			destination: nil,
			action:      pbauth.Action_ACTION_ALLOW,
			expectErr:   `invalid "data.destination" field: cannot be empty`,
		},
		"source-tenancy": {
			xTPs:        all,
			destination: &pbauth.Destination{IdentityName: "wi-1"},
			action:      pbauth.Action_ACTION_ALLOW,
			permissions: []*pbauth.Permission{
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
			expectErr: `invalid element at index 0 of list "permissions": invalid element at index 0 of list "sources": permissions sources may not specify partitions, peers, and sameness_groups together`,
		},
		"source-has-same-tenancy-as-tp": {
			xTPs: all,
			id: &pbresource.ID{
				Tenancy: &pbresource.Tenancy{
					Partition: resource.DefaultPartitionName,
				},
			},
			destination: &pbauth.Destination{IdentityName: "wi-1"},
			action:      pbauth.Action_ACTION_ALLOW,
			permissions: []*pbauth.Permission{
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
		"source-has-partition-set": {
			xTPs: all,
			id: &pbresource.ID{
				Tenancy: &pbresource.Tenancy{
					Partition: resource.DefaultPartitionName,
				},
			},
			destination: &pbauth.Destination{IdentityName: "wi-1"},
			action:      pbauth.Action_ACTION_ALLOW,
			permissions: []*pbauth.Permission{
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
		"source-has-peer-set": {
			xTPs: all,
			id: &pbresource.ID{
				Tenancy: &pbresource.Tenancy{
					Partition: resource.DefaultPartitionName,
				},
			},
			destination: &pbauth.Destination{IdentityName: "wi-1"},
			action:      pbauth.Action_ACTION_ALLOW,
			permissions: []*pbauth.Permission{
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
		"source-has-sameness-group-set": {
			xTPs: all,
			id: &pbresource.ID{
				Tenancy: &pbresource.Tenancy{
					Partition: resource.DefaultPartitionName,
				},
			},
			destination: &pbauth.Destination{IdentityName: "wi-1"},
			action:      pbauth.Action_ACTION_ALLOW,
			permissions: []*pbauth.Permission{
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
		"source-has-peer-and-partition-set": {
			xTPs: all,
			id: &pbresource.ID{
				Tenancy: &pbresource.Tenancy{
					Partition: resource.DefaultPartitionName,
				},
			},
			destination: &pbauth.Destination{IdentityName: "wi-1"},
			action:      pbauth.Action_ACTION_ALLOW,
			permissions: []*pbauth.Permission{
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
			expectErr: `invalid element at index 0 of list "permissions": invalid element at index 0 of list "sources": permissions sources may not specify partitions, peers, and sameness_groups together`,
		},
		"source-has-sameness-group-and-partition-set": {
			xTPs: all,
			id: &pbresource.ID{
				Tenancy: &pbresource.Tenancy{
					Partition: resource.DefaultPartitionName,
				},
			},
			destination: &pbauth.Destination{IdentityName: "wi-1"},
			action:      pbauth.Action_ACTION_ALLOW,
			permissions: []*pbauth.Permission{
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
			expectErr: `invalid element at index 0 of list "permissions": invalid element at index 0 of list "sources": permissions sources may not specify partitions, peers, and sameness_groups together`,
		},
		"source-has-sameness-group-and-partition-peer-set": {
			xTPs: all,
			id: &pbresource.ID{
				Tenancy: &pbresource.Tenancy{
					Partition: resource.DefaultPartitionName,
				},
			},
			destination: &pbauth.Destination{IdentityName: "wi-1"},
			action:      pbauth.Action_ACTION_ALLOW,
			permissions: []*pbauth.Permission{
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
			expectErr: `invalid element at index 0 of list "permissions": invalid element at index 0 of list "sources": permissions sources may not specify partitions, peers, and sameness_groups together`,
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			check := func(t *testing.T, typ *pbresource.Type, data protoreflect.ProtoMessage, validateFunc resource.ValidationHook) {
				resBuilder := resourcetest.Resource(typ, "tp").
					WithData(t, data)
				if tc.id != nil {
					resBuilder = resBuilder.WithTenancy(tc.id.Tenancy)
				}
				res := resBuilder.Build()
				err := validateFunc(res)
				if tc.expectErr == "" {
					require.NoError(t, err)
				} else if tc.enterprise && versiontest.IsEnterprise() {
					require.NoError(t, err)
				} else {
					// Expect error in CE, not ENT
					testutil.RequireErrorContains(t, err, tc.expectErr)
				}
			}
			if tc.xTPs&TrafficPermissions != 0 {
				t.Run("TrafficPermissions", func(t *testing.T) {
					tp := &pbauth.TrafficPermissions{
						Destination: tc.destination,
						Action:      tc.action,
						Permissions: tc.permissions,
					}
					check(t, pbauth.TrafficPermissionsType, tp, ValidateTrafficPermissions)
				})
			}
			if tc.xTPs&NamespaceTrafficPermissions != 0 {
				t.Run("NamespaceTrafficPermissions", func(t *testing.T) {
					ntp := &pbauth.NamespaceTrafficPermissions{
						Action:      tc.action,
						Permissions: tc.permissions,
					}
					check(t, pbauth.NamespaceTrafficPermissionsType, ntp, ValidateNamespaceTrafficPermissions)
				})
			}
			if tc.xTPs&PartitionTrafficPermissions != 0 {
				t.Run("PartitionTrafficPermissions", func(t *testing.T) {
					ptp := &pbauth.PartitionTrafficPermissions{
						Action:      tc.action,
						Permissions: tc.permissions,
					}
					check(t, pbauth.PartitionTrafficPermissionsType, ptp, ValidatePartitionTrafficPermissions)
				})
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
		"destination-rule-empty": {
			p: &pbauth.Permission{
				Sources: []*pbauth.Source{{IdentityName: "i1"}},
				DestinationRules: []*pbauth.DestinationRule{
					{},
				},
			},
			expectErr: `invalid element at index 0 of list "destination_rules": invalid element at index 0 of list "destination_rule": rules must contain path, method, header, or port fields`,
		},
		"destination-rule-only-empty-exclude": {
			p: &pbauth.Permission{
				Sources: []*pbauth.Source{{IdentityName: "i1"}},
				DestinationRules: []*pbauth.DestinationRule{
					{Exclude: []*pbauth.ExcludePermissionRule{{}}},
				},
			},
			expectErr: `invalid element at index 0 of list "destination_rules": invalid element at index 0 of list "exclude_permission_rules": invalid element at index 0 of list "exclude_permission_rule": rules must contain path, method, header, or port fields`,
		},
		"destination-rule-empty-exclude": {
			p: &pbauth.Permission{
				Sources: []*pbauth.Source{{IdentityName: "i1"}},
				DestinationRules: []*pbauth.DestinationRule{
					{
						PathExact: "/",
						Exclude:   []*pbauth.ExcludePermissionRule{{}},
					},
				},
			},
			expectErr: `invalid element at index 0 of list "destination_rules": invalid element at index 0 of list "exclude_permission_rules": invalid element at index 0 of list "exclude_permission_rule": rules must contain path, method, header, or port fields`,
		},
		"destination-rule-mismatched-ports-exclude": {
			p: &pbauth.Permission{
				Sources: []*pbauth.Source{{IdentityName: "i1"}},
				DestinationRules: []*pbauth.DestinationRule{
					{
						PortNames: []string{"foo"},
						Exclude:   []*pbauth.ExcludePermissionRule{{PortNames: []string{"bar"}}},
					},
				},
			},
			expectErr: `invalid element at index 0 of list "destination_rules": invalid element at index 0 of list "exclude_permission_rules": invalid element at index 0 of list "exclude_permission_header_rule": exclude permission rules must select a subset of ports and methods defined in the destination rule`,
		},
		"destination-rule-ports-exclude": {
			p: &pbauth.Permission{
				Sources: []*pbauth.Source{{IdentityName: "i1"}},
				DestinationRules: []*pbauth.DestinationRule{
					{
						Exclude: []*pbauth.ExcludePermissionRule{{PortNames: []string{"bar"}}},
					},
				},
			},
		},
		"destination-rule-invalid-headers-exclude": {
			p: &pbauth.Permission{
				Sources: []*pbauth.Source{{IdentityName: "i1"}},
				DestinationRules: []*pbauth.DestinationRule{
					{
						Headers: []*pbauth.DestinationRuleHeader{{Name: "auth"}},
						Exclude: []*pbauth.ExcludePermissionRule{{Headers: []*pbauth.DestinationRuleHeader{{}}}},
					},
				},
			},
			expectErr: `invalid element at index 0 of list "destination_rules": invalid element at index 0 of list "exclude_permission_header_rules": invalid element at index 0 of list "exclude_permission_header_rule": header rule must contain header name`,
		},
		"destination-rule-mismatched-methods-exclude": {
			p: &pbauth.Permission{
				Sources: []*pbauth.Source{{IdentityName: "i1"}},
				DestinationRules: []*pbauth.DestinationRule{
					{
						Methods: []string{"post"},
						Exclude: []*pbauth.ExcludePermissionRule{{Methods: []string{"patch"}}},
					},
				},
			},
			expectErr: `invalid element at index 0 of list "destination_rules": invalid element at index 0 of list "exclude_permission_rules": invalid element at index 0 of list "exclude_permission_header_rule": exclude permission rules must select a subset of ports and methods defined in the destination rule`,
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

type mutationTestCase struct {
	tenancy     *pbresource.Tenancy
	permissions []*pbauth.Permission
	expect      []*pbauth.Permission
	expectErr   string
}

func mutationTestCases() map[string]mutationTestCase {
	return map[string]mutationTestCase{
		"empty": {
			permissions: nil,
			expect:      nil,
		},
		"kitchen-sink-default-partition": {
			permissions: []*pbauth.Permission{
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
			expect: []*pbauth.Permission{
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
							// TODO(peering/v2) revisit peer defaulting
							// Peer:         "local",
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
		"kitchen-sink-excludes-default-partition": {
			permissions: []*pbauth.Permission{
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
			expect: []*pbauth.Permission{
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
									// TODO(peering/v2) revisit peer defaulting
									// Peer:         "local",
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
		"kitchen-sink-non-default-partition": {
			tenancy: &pbresource.Tenancy{
				Partition: "ap1",
				Namespace: "ns3",
			},
			permissions: []*pbauth.Permission{
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
			expect: []*pbauth.Permission{
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
							// TODO(peering/v2) revisit to figure out defaulting
							// Peer:         "local",
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
		"kitchen-sink-excludes-non-default-partition": {
			tenancy: &pbresource.Tenancy{
				Partition: "ap1",
				Namespace: "ns3",
			},
			permissions: []*pbauth.Permission{
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
			expect: []*pbauth.Permission{
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
									// TODO(peering/v2) revisit peer defaulting
									// Peer:         "local",
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
	}
}

func TestMutateTrafficPermissions(t *testing.T) {
	run := func(t *testing.T, tc mutationTestCase) {
		tenancy := tc.tenancy
		if tenancy == nil {
			tenancy = resource.DefaultNamespacedTenancy()
		}
		res := resourcetest.Resource(pbauth.TrafficPermissionsType, "api").
			WithTenancy(tenancy).
			WithData(t, &pbauth.TrafficPermissions{
				Destination: &pbauth.Destination{IdentityName: "wi1"},
				Action:      pbauth.Action_ACTION_ALLOW,
				Permissions: tc.permissions,
			}).
			Build()

		err := MutateTrafficPermissions(res)

		got := resourcetest.MustDecode[*pbauth.TrafficPermissions](t, res)

		if tc.expectErr == "" {
			require.NoError(t, err)
			prototest.AssertDeepEqual(t, tc.expect, got.Data.Permissions)
		} else {
			testutil.RequireErrorContains(t, err, tc.expectErr)
		}
	}

	for name, tc := range mutationTestCases() {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestTrafficPermissionsACLs(t *testing.T) {
	registry := resource.NewRegistry()
	Register(registry)

	tpData := &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{IdentityName: "wi1"},
		Action:      pbauth.Action_ACTION_ALLOW,
	}
	cases := map[string]resourcetest.ACLTestCase{
		"no rules": {
			Rules:   ``,
			Data:    tpData,
			Typ:     pbauth.TrafficPermissionsType,
			ReadOK:  resourcetest.DENY,
			WriteOK: resourcetest.DENY,
			ListOK:  resourcetest.DEFAULT,
		},
		"workload identity w1 read, no intentions": {
			Rules:   `identity "wi1" { policy = "read" }`,
			Data:    tpData,
			Typ:     pbauth.TrafficPermissionsType,
			ReadOK:  resourcetest.ALLOW,
			WriteOK: resourcetest.DENY,
			ListOK:  resourcetest.DEFAULT,
		},
		"workload identity w1 read, deny intentions": {
			Rules:   `identity "wi1" { policy = "read", intentions = "deny" }`,
			Data:    tpData,
			Typ:     pbauth.TrafficPermissionsType,
			ReadOK:  resourcetest.DENY,
			WriteOK: resourcetest.DENY,
			ListOK:  resourcetest.DEFAULT,
		},
		"workload identity w1 read, intentions read": {
			Rules:   `identity "wi1" { policy = "read", intentions = "read" }`,
			Data:    tpData,
			Typ:     pbauth.TrafficPermissionsType,
			ReadOK:  resourcetest.ALLOW,
			WriteOK: resourcetest.DENY,
			ListOK:  resourcetest.DEFAULT,
		},
		"workload identity w1 read, intentions write": {
			Rules:   `identity "wi1" { policy = "read", intentions = "write" }`,
			Data:    tpData,
			Typ:     pbauth.TrafficPermissionsType,
			ReadOK:  resourcetest.ALLOW,
			WriteOK: resourcetest.ALLOW,
			ListOK:  resourcetest.DEFAULT,
		},
		"workload identity w1 write, deny intentions": {
			Rules:   `identity "wi1" { policy = "write", intentions = "deny" }`,
			Data:    tpData,
			Typ:     pbauth.TrafficPermissionsType,
			ReadOK:  resourcetest.DENY,
			WriteOK: resourcetest.DENY,
			ListOK:  resourcetest.DEFAULT,
		},
		"workload identity w1 write, intentions read": {
			Rules:   `identity "wi1" { policy = "write", intentions = "read" }`,
			Data:    tpData,
			Typ:     pbauth.TrafficPermissionsType,
			ReadOK:  resourcetest.ALLOW,
			WriteOK: resourcetest.DENY,
			ListOK:  resourcetest.DEFAULT,
		},
		"workload identity w1 write, intentions write": {
			Rules:   `identity "wi1" { policy = "write", intentions = "write" }`,
			Data:    tpData,
			Typ:     pbauth.TrafficPermissionsType,
			ReadOK:  resourcetest.ALLOW,
			WriteOK: resourcetest.ALLOW,
			ListOK:  resourcetest.DEFAULT,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			resourcetest.RunACLTestCase(t, tc, registry)
		})
	}
}
