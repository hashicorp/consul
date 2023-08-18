package mappers

import (
	"sync"

	"github.com/hashicorp/consul/proto-public/pbresource"
)

type TrafficPermissionsMapper struct {
	lock                       sync.Mutex
	trafficPermToComputedPerms map[string][]string
	computedPermToTrafficPerms map[string][]string

	trafficPermToWorkloads map[string][]string
	workloadToTrafficPerm  map[string][]string
}

func (t *TrafficPermissionsMapper) ComputedPermissionFromWorkload()

func (t *TrafficPermissionsMapper) TrackWorkload(workloadID *pbresource.ID, trafficPermissionID *pbresource.ID) {

}

func (t *TrafficPermissionsMapper) UntrackWorkload(workloadID *pbresource.ID) {

}
