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

func ValidateComputedTrafficPermission(_ *pbresource.Resource) error {
	// TODO Marshal and validate

	return nil
}
