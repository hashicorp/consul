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

// BindRoutesToGateways takes a slice of bound API gateways and a variadic number of routes.
// It iterates over the parent references for each route. These parents are gateways the
// route should be bound to. If the parent matches a bound gateway, the route is bound to the
// gateway. Otherwise, the route is unbound from the gateway if it was previously bound.
//
// The function returns a list of references to the modified BoundAPIGatewayConfigEntry objects,
// a map of resource references to errors that occurred when they were attempted to be
// bound to a gateway.
func BindRoutesToGateways(gateways []*gatewayMeta, routes ...structs.BoundRoute) ([]*structs.BoundAPIGatewayConfigEntry, map[structs.ResourceReference]error) {
	modified := make([]*structs.BoundAPIGatewayConfigEntry, 0, len(gateways))

	// errored stores the errors from events where a resource reference failed to bind to a gateway.
	errored := make(map[structs.ResourceReference]error)

	for _, route := range routes {
		parentRefs, gatewayRefs := getReferences(route)

		// Iterate over all BoundAPIGateway config entries and try to bind them to the route if they are a parent.
		for _, gateway := range gateways {
			references, routeReferencesGateway := gatewayRefs[configentry.NewKindNameForEntry(gateway.BoundGateway)]
			if routeReferencesGateway {
				didUpdate, errors := gateway.updateRouteBinding(references, route)
				if didUpdate {
					modified = append(modified, gateway.BoundGateway)
				}
				for ref, err := range errors {
					errored[ref] = err
				}
				for _, ref := range references {
					delete(parentRefs, ref)
				}
			} else if gateway.unbindRoute(route) {
				modified = append(modified, gateway.BoundGateway)
			}
		}

		// Add all references that aren't bound at this point to the error set.
		for reference := range parentRefs {
			errored[reference] = errors.New("invalid reference to missing parent")
		}
	}

	return modified, errored
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
