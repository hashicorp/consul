package trafficpermissions

import (
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// TODO: Mapper TrafficPermissions -> ComputedTrafficPermissions
func MapWorkload(context.Context, controller.Runtime, resource *pbresource.Resource) ([]controller.Request, error) {
	// resource is TrafficPermissionsType
	// transform to ComputedTrafficPermisisonsType
	return nil, nil
}

// ServiceEndpointsController creates a controller to perform automatic endpoint management for
// services.
func ServiceEndpointsController(workloadMap WorkloadMapper) controller.Controller {
	if workloadMap == nil {
		panic("No WorkloadMapper was provided to the ServiceEndpointsController constructor")
	}

	return controller.ForType(types.ComputedTrafficPermissionsType).
		WithWatch(types.TrafficPermissionsType, controller.ReplaceType(types.ComputedTrafficPermissionsType)).
		WithWatch(types.WorkloadType, workloadMap.MapWorkload).
		WithReconciler(newServiceEndpointsReconciler(workloadMap))
}