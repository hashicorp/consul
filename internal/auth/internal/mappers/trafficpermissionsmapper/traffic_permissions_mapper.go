package trafficpermissionsmapper

import (
	"context"
	"sync"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type TrafficPermissionsMapper struct {
	lock                       sync.Mutex
	trafficPermToComputedPerms map[string][]string
	computedPermToTrafficPerms map[string][]string

	trafficPermToWorkloads map[string][]string
	workloadToTrafficPerm  map[string][]string
}

func New() *TrafficPermissionsMapper {
	return &TrafficPermissionsMapper{
		lock:                       sync.Mutex{},
		trafficPermToComputedPerms: make(map[string][]string),
		computedPermToTrafficPerms: make(map[string][]string),
		trafficPermToWorkloads:     make(map[string][]string),
		workloadToTrafficPerm:      make(map[string][]string),
	}
}

func (t *TrafficPermissionsMapper) MapWorkloadIdentity(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	return nil, nil
}

func (t *TrafficPermissionsMapper) MapTrafficPermission(context.Context, controller.Runtime, *pbresource.Resource) ([]controller.Request, error) {
	return nil, nil
}

func (t *TrafficPermissionsMapper) UntrackComputedTrafficPermission(computedTrafficPermissionID *pbresource.ID) {
	return
}
