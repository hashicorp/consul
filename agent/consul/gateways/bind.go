package gateways

import (
	"errors"

	"github.com/hashicorp/consul/agent/structs"
)

// referenceSet stores an O(1) accessible set of ResourceReference objects.
type referenceSet = map[structs.ResourceReference]any

// BindRouteToGateways takes a slice of bound API gateways and a route.
// It iterates over the parent references for the given route. These parents are gateways the
// route should be bound to. If the parent matches a bound gateway in the state store,
// the route is bound to the gateway. Otherwise, the route is unbound from the gateway if it
// was bound.
//
// The function returns a list of references to the modified BoundAPIGatewayConfigEntry objects,
// a map of resource references to errors that occurred when they were attempted to be
// bound to a gateway, and an error if the overall process was unsucessful.
func BindRouteToGateways(gateways []*structs.BoundAPIGatewayConfigEntry, route structs.BoundRoute) ([]*structs.BoundAPIGatewayConfigEntry, map[structs.ResourceReference]error, error) {
	modified := make([]*structs.BoundAPIGatewayConfigEntry, 0, len(gateways))

	// errored stores the errors from events where a resource reference failed to bind to a gateway.
	errored := make(map[structs.ResourceReference]error)

	parentRefs := getParentReferences(route)

	// Iterate over all BoundAPIGateway config entries and try to bind them to the route if they are a parent.
	for _, gateway := range gateways {
		if shouldBind(gateway, parentRefs) {
			for reference := range parentRefs {
				didBind, err := gateway.BindRoute(reference, route)
				if err != nil {
					delete(parentRefs, reference)
					errored[reference] = err
					continue
				}
				if didBind {
					delete(parentRefs, reference)
					modified = append(modified, gateway)
				}
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

func getParentReferences(route structs.BoundRoute) referenceSet {
	refs := make(referenceSet)
	for _, ref := range route.GetParents() {
		refs[ref] = struct{}{}
	}

	return refs
}

func shouldBind(gateway *structs.BoundAPIGatewayConfigEntry, parentRefs referenceSet) bool {
	for reference := range parentRefs {
		if reference.Kind == structs.APIGateway && gateway.Name == reference.Name && gateway.IsSame(&reference.EnterpriseMeta) {
			return true
		}
	}
	return false
}
