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
		Proto:    &pbauth.NamespaceTrafficPermission{},
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
	return errWildcardNotSupported
}

func ValidateNamespaceTrafficPermission(res *pbresource.Resource) error {
	var tp pbauth.NamespaceTrafficPermission

	if err := res.Data.UnmarshalTo(&tp); err != nil {
		return resource.NewErrDataParse(&tp, err)
	}

	var err error

	// TODO: There is probably a better way to indicate a nested field.
	if !(tp.Action == ActionAllow || tp.Action == ActionDeny) {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "data.action",
			Wrapped: errInvalidAction,
		})
	}

	// Validate permissions
	for i, permission := range tp.Permissions {
		if len(permission.DestinationRules) <= 0 {
			err = multierror.Append(err, resource.ErrInvalidListElement{
				Name:    "destination_rules",
				Index:   i,
				Wrapped: errEmptyDestinationRules,
			})
		}

		// TODO: Validate destination rules

		if len(permission.Sources) <= 0 {
			err = multierror.Append(err, resource.ErrInvalidListElement{
				Name:    "sources",
				Index:   i,
				Wrapped: errEmptySources,
			})
		}

		// TODO: Validate sources
	}

	// TODO: There is probably a better way to indicate a nested field.
	if tp.Destination.IdentityName == "" {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "data.destination.identity_name",
			Wrapped: resource.ErrEmpty,
		})
	}

	return err
}
