package types

import (
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

func ValidateTrafficPermission(_ *pbresource.Resource) error {
	// TODO Marshal and validate properties

	return nil
}
