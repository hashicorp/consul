package types

import (
	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/internal/resource"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	TrafficPermissionKind = "TrafficPermission"
)

var (
	TrafficPermissionV1AlphaType = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV1Alpha1,
		Kind:         TrafficPermissionKind,
	}

	TrafficPermissionType = TrafficPermissionV1AlphaType
)

func RegisterTrafficPermission(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     TrafficPermissionV1AlphaType,
		Proto:    &pbauth.TrafficPermission{},
		Validate: ValidateTrafficPermission,
	})
}

func ValidateTrafficPermission(res *pbresource.Resource) error {
	// TODO Marshal and validate properties

	var tp pbauth.TrafficPermission

	if err := res.Data.UnmarshalTo(&tp); err != nil {
		return resource.NewErrDataParse(&tp, err)
	}

	var err error

	if tp.Name == "" {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "name",
			Wrapped: resource.ErrEmpty,
		})
	}

	// TODO: There is probably a better way to indicate a nested field.
	if !(tp.Data.Action == ActionAllow || tp.Data.Action == ActionDeny) {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "data.action",
			Wrapped: errInvalidAction,
		})
	}

	// Validate permissions
	for i, permission := range tp.Data.Permissions {
		if len(permission.DestinationRules) <= 0 {
			err = multierror.Append(err, resource.ErrInvalidListElement{
				Name:    "destination_rules",
				Index:   i,
				Wrapped: errEmptyDestinationRules,
			})
		}

		// TODO: Validate destination rules
		//for j, dr := range permission.DestinationRules {}

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
	if tp.Data.Destination.IdentityName == "" {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "data.destination.identity_name",
			Wrapped: resource.ErrEmpty,
		})
	}

	return err
}
