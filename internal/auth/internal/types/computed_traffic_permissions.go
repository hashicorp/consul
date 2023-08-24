package types

import (
	"github.com/hashicorp/consul/internal/resource"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	ComputedTrafficPermissionKind = "ComputedTrafficPermission"
)

var (
	ComputedTrafficPermissionV1AlphaType = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV1Alpha1,
		Kind:         ComputedTrafficPermissionKind,
	}

	ComputedTrafficPermissionType = ComputedTrafficPermissionV1AlphaType
)

func RegisterComputedTrafficPermission(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     ComputedTrafficPermissionV1AlphaType,
		Proto:    &pbauth.ComputedTrafficPermission{},
		Validate: ValidateComputedTrafficPermission,
	})
}

func ValidateComputedTrafficPermission(res *pbresource.Resource) error {
	var ctp pbauth.ComputedTrafficPermission

	if err := res.Data.UnmarshalTo(&ctp); err != nil {
		return resource.NewErrDataParse(&ctp, err)
	}

	// There isn't really anything else to validate other than that it is unmarshallable -- which it should always be.
	// It is valid for the ComputedTrafficPermissions to be totally empty as this would mean that the default behavior
	// applies for all sources.

	return nil
}
