package gateways

import (
	"errors"

	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/structs"
)

// referenceSet stores an O(1) accessible set of ResourceReference objects.
type referenceSet = map[structs.ResourceReference]any

// gatewayRefs maps a gateway kind/name to a set of resource references.
type gatewayRefs = map[configentry.KindName][]structs.ResourceReference

// BindRoutesToGateways takes a slice of bound API gateways and a slice of routes.
// It iterates over the parent references for each route. These parents are gateways the
// route should be bound to. If the parent matches a bound gateway, the route is bound to
// the gateway. Otherwise, the route is unbound from the gateway if it was bound.
//
// The function returns a list of references to the modified BoundAPIGatewayConfigEntry objects,
// a map of resource references to errors that occurred when they were attempted to be
// bound to a gateway, and an error if the overall process was unsucessful.
func BindRoutesToGateways(gateways []*structs.BoundAPIGatewayConfigEntry, routes []structs.BoundRoute) ([]*structs.BoundAPIGatewayConfigEntry, map[structs.ResourceReference]error, error) {
	// errored stores the errors from events where a resource reference failed to bind to a gateway.
	errored := make(map[structs.ResourceReference]error)

	modified := make([]*structs.BoundAPIGatewayConfigEntry, 0, len(gateways))
	for _, route := range routes {
		routeModified, routeErrored, err := BindRouteToGateways(gateways, route)
		if err != nil {
			return nil, nil, err
		}
		modified = append(modified, routeModified...)
		for ref, err := range routeErrored {
			errored[ref] = err
		}
	}

	return modified, errored, nil
}

// BindRouteToGateways takes a slice of bound API gateways and a route.
// It iterates over the parent references for the given route. These parents are gateways the
// route should be bound to. If the parent matches a bound gateway, the route is bound to the
// gateway. Otherwise, the route is unbound from the gateway if it was previously bound.
//
// The function returns a list of references to the modified BoundAPIGatewayConfigEntry objects,
// a map of resource references to errors that occurred when they were attempted to be
// bound to a gateway, and an error if the overall process was unsucessful.
func BindRouteToGateways(gateways []*structs.BoundAPIGatewayConfigEntry, route structs.BoundRoute) ([]*structs.BoundAPIGatewayConfigEntry, map[structs.ResourceReference]error, error) {
	// errored stores the errors from events where a resource reference failed to bind to a gateway.
	errored := make(map[structs.ResourceReference]error)

	parentRefs, gatewayRefs := getReferences(route)

	// Iterate over all BoundAPIGateway config entries and try to bind them to the route if they are a parent.
	modified := make([]*structs.BoundAPIGatewayConfigEntry, 0, len(gateways))
	for _, gateway := range gateways {
		references, routeReferencesGateway := gatewayRefs[configentry.NewKindNameForEntry(gateway)]
		if routeReferencesGateway {
			didUpdate, errors := gateway.UpdateRouteBinding(references, route)
			if didUpdate {
				modified = append(modified, gateway)
			}
			for ref, err := range errors {
				errored[ref] = err
			}
			for _, ref := range references {
				delete(parentRefs, ref)
			}
		} else {
			if gateway.UnbindRoute(route) {
				modified = append(modified, gateway)
			}
		}
	}

	// Add all references that aren't bound at this point to the error set.
	for reference := range parentRefs {
		errored[reference] = errors.New("invalid reference to missing parent")
	}

	return modified, errored, nil
}

// getReferences returns a set of all the resource references for a given route as well as
// a map of gateway kind/name to a list of resource references for that gateway.
func getReferences(route structs.BoundRoute) (referenceSet, gatewayRefs) {
	parentRefs := make(referenceSet)
	gatewayRefs := make(gatewayRefs)
	for _, ref := range route.GetParents() {
		parentRefs[ref] = struct{}{}
		kindName := configentry.NewKindName(structs.BoundAPIGateway, ref.Name, &ref.EnterpriseMeta)
		gatewayRefs[kindName] = append(gatewayRefs[kindName], ref)
	}

	return parentRefs, gatewayRefs
}
