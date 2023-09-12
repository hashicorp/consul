package types

import (
	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/internal/resource"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	TrafficPermissionsKind = "TrafficPermissions"
)

var (
	TrafficPermissionsV1Alpha1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV1Alpha1,
		Kind:         TrafficPermissionsKind,
	}

	TrafficPermissionsType = TrafficPermissionsV1Alpha1Type
)

func RegisterTrafficPermissions(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     TrafficPermissionsV1Alpha1Type,
		Proto:    &pbauth.TrafficPermissions{},
		Scope:    resource.ScopeNamespace,
		Validate: ValidateTrafficPermissions,
	})
}

func ValidateTrafficPermissions(res *pbresource.Resource) error {
	var tp pbauth.TrafficPermissions

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
	if tp.Destination == nil || (len(tp.Destination.IdentityName) == 0 && len(tp.Destination.IdentityPrefix) == 0) {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "data.destination",
			Wrapped: resource.ErrEmpty,
		})
	}
	if len(tp.Destination.IdentityName) > 0 && len(tp.Destination.IdentityPrefix) > 0 {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "data.destination",
			Wrapped: errInvalidPrefixValues,
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
			if len(src.Partition) > 0 && len(src.Peer) > 0 && len(src.SamenessGroup) > 0 {
				err = multierror.Append(err, resource.ErrInvalidListElement{
					Name:    "sources",
					Index:   i,
					Wrapped: errSourcesTenancy,
				})
			}

			if len(permission.DestinationRules) > 0 {
				for i, d := range permission.DestinationRules {
					if (len(d.PathExact) > 0 && len(d.PathPrefix) > 0) ||
						(len(d.PathRegex) > 0 && len(d.PathExact) > 0) ||
						(len(d.PathRegex) > 0 && len(d.PathPrefix) > 0) {
						err = multierror.Append(err, resource.ErrInvalidListElement{
							Name:    "data.destination",
							Index:   i,
							Wrapped: errInvalidPrefixValues,
						})
					}

				}
			}
		}

		// TODO: Validate sources
	}

	return err
}
