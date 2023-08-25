package trafficpermissionsmapper

import (
	"context"
	"sync"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/radix"
	"github.com/hashicorp/consul/internal/resource"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type TrafficPermissionsMapper struct {
	lock sync.Mutex

	// workloadIdentityPrefixes is used to find the traffic permissions
	// which apply to a particular WorkloadIdentity. The workloadIdentityPrefix
	// radix tree is used to match on workload selectors which use prefixes.
	workloadIdentityPrefixes *radix.Tree[[]controller.Request]

	// workloadIdentityExact is used to find the traffic permissions
	// which apply to a particular WorkloadIdentity. The workloadIdentityExact
	// radix tree is used to match on fully qualified workload selectors.
	workloadIdentityExact *radix.Tree[[]controller.Request]

	workloadIdentityToCTP map[string]controller.Request
}

func New() *TrafficPermissionsMapper {
	return &TrafficPermissionsMapper{
		lock:                     sync.Mutex{},
		workloadIdentityPrefixes: radix.New[[]controller.Request](),
		workloadIdentityExact:    radix.New[[]controller.Request](),
	}
}

func (t *TrafficPermissionsMapper) MapWorkloadIdentity(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	t.lock.Lock()
	defer t.lock.Unlock()

	/*
	 * When a WorkloadIdentity comes in on the map queue,
	 * we need to translate it to all the relevant CTPs.
	 * Fortunately, this should mean we only need to look
	 * the CTP which represents the WorkloadIdentity as a
	 * destination.
	 */

	ctp, _ := t.workloadIdentityToCTP[res.Id.Name]

	return []controller.Request{ctp}, nil
}

func (t *TrafficPermissionsMapper) MapTrafficPermission(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	t.lock.Lock()
	defer t.lock.Unlock()

	tp, err := resource.GetDecodedResource[pbauth.TrafficPermission, *pbauth.TrafficPermission](ctx, rt.Client, res.Id)
	if err != nil {
		// TODO wrap error
		return nil, err
	}

	dest := tp.Data.Data.Destination.IdentityName
	var workloadIdentities []controller.Request
	if isExplicitDestination(dest) {
		// traverse the explicit tree
		workloadIdentities, _ = t.workloadIdentityExact.Get(dest)
	} else {
		// traverse the wildcard tree
		t.workloadIdentityPrefixes.WalkPath(dest, func(path string, requests []controller.Request) bool {
			workloadIdentities = append(workloadIdentities, requests...)
			return false
		})
	}

	return workloadIdentities, nil
}

func (t *TrafficPermissionsMapper) UntrackComputedTrafficPermission(computedTrafficPermissionID *pbresource.ID) {
	t.lock.Lock()
	defer t.lock.Unlock()

	// TODO

	return
}

func (t *TrafficPermissionsMapper) WorkloadIdentityFromCTP(ctp *pbresource.Resource, ctpData *pbauth.ComputedTrafficPermission) *pbresource.ID {
	// TODO: We can probably just give the CTP a name field that is aligned
	// with its corresponding WorkloadIdentity since that should be a 1:1 mapping
	return nil
}

func isExplicitDestination(destination string) bool {
	// TODO: We are not supporting wildcards unless we have time
	return true
}
