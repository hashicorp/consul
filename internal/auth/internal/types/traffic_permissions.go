package types

import (
	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/internal/resource"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	NamespaceTrafficPermissionKind = "NamespaceTrafficPermission"
	PartitionTrafficPermissionKind = "PartitionTrafficPermission"
)

var (
	NamespaceTrafficPermissionV1AlphaType = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV1Alpha1,
		Kind:         NamespaceTrafficPermissionKind,
	}

	PartitionTrafficPermissionV1AlphaType = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV1Alpha1,
		Kind:         PartitionTrafficPermissionKind,
	}

	NamespaceTrafficPermissionType = NamespaceTrafficPermissionV1AlphaType
	PartitionTrafficPermissionType = PartitionTrafficPermissionV1AlphaType
)

func RegisterTrafficPermissions(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     NamespaceTrafficPermissionV1AlphaType,
		Proto:    &pbauth.TrafficPermission{},
		Scope:    resource.ScopeNamespace,
		Validate: ValidateNamespaceTrafficPermission,
	})
	r.Register(resource.Registration{
		Type:     PartitionTrafficPermissionV1AlphaType,
		Proto:    &pbauth.PartitionTrafficPermission{},
		Scope:    resource.ScopePartition,
		Validate: ValidatePartitionTrafficPermission,
	})
}

func ValidatePartitionTrafficPermission(res *pbresource.Resource) error {
	return errNotSupported
}

func ValidateNamespaceTrafficPermission(res *pbresource.Resource) error {
	var tp pbauth.TrafficPermission

	if err := res.Data.UnmarshalTo(&tp); err != nil {
		return resource.NewErrDataParse(&tp, err)
	}

	var err error

	// TODO: Refactor errors for nested fields after further validation logic is added
	if tp.Action == pbauth.Action_ACTION_UNSPECIFIED {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "data.action",
			Wrapped: errInvalidAction,
		})
	}
	if tp.Destination != nil && len(tp.Destination.IdentityName) == 0 {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "data.destination.identity_name",
			Wrapped: resource.ErrEmpty,
		})
	}

	// Validate permissions
	for i, permission := range tp.Permissions {
		// TODO: Validate destination rules

		if len(permission.Sources) <= 0 {
			err = multierror.Append(err, resource.ErrInvalidListElement{
				Name:    "sources",
				Index:   i,
				Wrapped: errEmptySources,
			})
		}

		for _, src := range permission.Sources {
			if len(src.IdentityName) == 0 {
				err = multierror.Append(err, resource.ErrInvalidListElement{
					Name:    "sources",
					Index:   i,
					Wrapped: errEmptySources,
				})
			}
		}

		// TODO: Validate sources
	}

	return err
}
