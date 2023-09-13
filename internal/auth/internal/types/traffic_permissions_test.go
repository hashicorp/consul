// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/internal/resource"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func createTrafficPermissionsResource(t *testing.T, data protoreflect.ProtoMessage) *pbresource.Resource {
	res := &pbresource.Resource{
		Id: &pbresource.ID{
			Type: TrafficPermissionsType,
			Tenancy: &pbresource.Tenancy{
				Partition: "default",
				Namespace: "default",
				PeerName:  "local",
			},
			Name: "test-traffic-permissions",
		},
	}

	var err error
	res.Data, err = anypb.New(data)
	require.NoError(t, err)
	return res
}

func TestTrafficPermissions_OkMinimal(t *testing.T) {
	data := &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{IdentityName: "wi-1"},
		Action:      pbauth.Action_ACTION_ALLOW,
	}

	res := createTrafficPermissionsResource(t, data)

	err := ValidateTrafficPermissions(res)
	require.NoError(t, err)
}

func TestTrafficPermissions_OkFull(t *testing.T) {
	data := &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{
			IdentityName: "w1",
		},
		Action: pbauth.Action_ACTION_ALLOW,
		Permissions: []*pbauth.Permission{
			{
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
			{
				Sources: []*pbauth.Source{
					{
						IdentityName: "wi-3",
						Peer:         "p1",
					},
				},
			},
		},
	}

	res := createTrafficPermissionsResource(t, data)

	err := ValidateTrafficPermissions(res)
	require.NoError(t, err)
}

func TestValidateTrafficPermissions_ParseError(t *testing.T) {
	// Any type other than the TrafficPermissions type would work
	// to cause the error we are expecting
	data := &pbauth.ComputedTrafficPermissions{AllowPermissions: nil}

	res := createTrafficPermissionsResource(t, data)

	err := ValidateTrafficPermissions(res)
	require.Error(t, err)
	require.ErrorAs(t, err, &resource.ErrDataParse{})
}

func TestValidateTrafficPermissions_UnsupportedAction(t *testing.T) {
	data := &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{
			IdentityName: "wi1",
		},
		Action:      pbauth.Action_ACTION_UNSPECIFIED,
		Permissions: nil,
	}

	res := createTrafficPermissionsResource(t, data)

	err := ValidateTrafficPermissions(res)
	require.Error(t, err)
	expected := resource.ErrInvalidField{
		Name:    "data.action",
		Wrapped: errInvalidAction,
	}
	var actual resource.ErrInvalidField
	require.ErrorAs(t, err, &actual)
	require.Equal(t, expected, actual)
}

func TestValidateTrafficPermissions_DestinationRulePathPrefixRegex(t *testing.T) {
	data := &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{
			IdentityName: "w1",
		},
		Action: pbauth.Action_ACTION_ALLOW,
		Permissions: []*pbauth.Permission{
			{
				Sources: nil,
				DestinationRules: []*pbauth.DestinationRule{
					{
						PathExact:  "wi2",
						PathPrefix: "wi",
						PathRegex:  "wi.*",
					},
				},
			},
		},
	}

	res := createTrafficPermissionsResource(t, data)

	err := ValidateTrafficPermissions(res)
	require.Error(t, err)
	expected := resource.ErrInvalidListElement{
		Name:    "destination_rule",
		Wrapped: errInvalidPrefixValues,
	}
	var actual resource.ErrInvalidListElement
	require.ErrorAs(t, err, &actual)
	require.Equal(t, "permissions", actual.Name)
	err = actual.Unwrap()
	require.ErrorAs(t, err, &actual)
	require.ErrorIs(t, expected, actual.Unwrap())
}

func TestValidateTrafficPermissions_NoDestination(t *testing.T) {
	data := &pbauth.TrafficPermissions{
		Action: pbauth.Action_ACTION_ALLOW,
		Permissions: []*pbauth.Permission{
			{
				Sources: nil,
				DestinationRules: []*pbauth.DestinationRule{
					{
						PathExact: "wi2",
					},
				},
			},
		},
	}

	res := createTrafficPermissionsResource(t, data)

	err := ValidateTrafficPermissions(res)
	require.Error(t, err)
	expected := resource.ErrInvalidField{
		Name:    "data.destination",
		Wrapped: resource.ErrEmpty,
	}
	var actual resource.ErrInvalidField
	require.ErrorAs(t, err, &actual)
	require.Equal(t, "data.destination", actual.Name)
	require.Equal(t, expected, actual)
}

func TestValidateTrafficPermissions_SourceTenancy(t *testing.T) {
	data := &pbauth.TrafficPermissions{
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
	}

	res := createTrafficPermissionsResource(t, data)

	err := ValidateTrafficPermissions(res)
	require.Error(t, err)
	expected := resource.ErrInvalidListElement{
		Name:    "source",
		Wrapped: errSourcesTenancy,
	}
	var actual resource.ErrInvalidListElement
	require.ErrorAs(t, err, &actual)
	require.Equal(t, "permissions", actual.Name)
	err = actual.Unwrap()
	require.ErrorAs(t, err, &actual)
	require.ErrorIs(t, expected, actual.Unwrap())
}

func TestValidateTrafficPermissions_ExcludeSourceTenancy(t *testing.T) {
	data := &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{
			IdentityName: "w1",
		},
		Action: pbauth.Action_ACTION_ALLOW,
		Permissions: []*pbauth.Permission{
			{
				Sources: []*pbauth.Source{
					{
						Namespace: "ns1",
						Exclude: []*pbauth.ExcludeSource{
							{
								Partition:     "ap1",
								Peer:          "cl1",
								SamenessGroup: "sg1",
							},
						},
					},
				},
			},
		},
	}

	res := createTrafficPermissionsResource(t, data)

	err := ValidateTrafficPermissions(res)
	require.Error(t, err)
	expected := resource.ErrInvalidListElement{
		Name:    "exclude_source",
		Wrapped: errSourcesTenancy,
	}
	var actual resource.ErrInvalidListElement
	require.ErrorAs(t, err, &actual)
	require.Equal(t, "permissions", actual.Name)
	err = actual.Unwrap()
	require.ErrorAs(t, err, &actual)
	require.ErrorIs(t, expected, actual.Unwrap())
}
