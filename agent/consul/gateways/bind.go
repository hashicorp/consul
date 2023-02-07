package gateways

import (
	"errors"
	"time"

	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/consul/controller"
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
func BindRoutesToGateways(gateways []*gatewayMeta, routes ...structs.BoundRoute) ([]*structs.BoundAPIGatewayConfigEntry, []structs.ResourceReference, map[structs.ResourceReference]error) {
	boundRefs := []structs.ResourceReference{}
	modified := make([]*structs.BoundAPIGatewayConfigEntry, 0, len(gateways))

	// errored stores the errors from events where a resource reference failed to bind to a gateway.
	errored := make(map[structs.ResourceReference]error)

	for _, route := range routes {
		parentRefs, gatewayRefs := getReferences(route)
		routeRef := structs.ResourceReference{
			Kind:           route.GetKind(),
			Name:           route.GetName(),
			EnterpriseMeta: *route.GetEnterpriseMeta(),
		}

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

					// this ref successfully bound, add it to the set that we'll update the
					// status for
					if _, found := errored[ref]; !found {
						boundRefs = append(boundRefs, references...)
					}
				}

				continue
			}

			if gateway.unbindRoute(routeRef) {
				modified = append(modified, gateway.BoundGateway)
			}
		}

		// Add all references that aren't bound at this point to the error set.
		for reference := range parentRefs {
			errored[reference] = errors.New("invalid reference to missing parent")
		}
	}

	return modified, boundRefs, errored
}

// getReferences returns a set of all the resource references for a given route as well as
// a map of gateway kind/name to a list of resource references for that gateway.
func getReferences(route structs.BoundRoute) (referenceSet, gatewayRefs) {
	parentRefs := make(referenceSet)
	gatewayRefs := make(gatewayRefs)

	for _, ref := range route.GetParents() {
		parentRefs[ref] = struct{}{}
		kindName := configentry.NewKindName(structs.BoundAPIGateway, ref.Name, pointerTo(ref.EnterpriseMeta))
		gatewayRefs[kindName] = append(gatewayRefs[kindName], ref)
	}

	return parentRefs, gatewayRefs
}

func requestToResourceRef(req controller.Request) structs.ResourceReference {
	ref := structs.ResourceReference{
		Kind: req.Kind,
		Name: req.Name,
	}

	if req.Meta != nil {
		ref.EnterpriseMeta = *req.Meta
	}

	return ref
}

// RemoveGateway sets the route's status appropriately when the gateway that it's
// attempting to bind to does not exist
func RemoveGateway(gateway structs.ResourceReference, entries ...structs.BoundRoute) []structs.ControlledConfigEntry {
	now := pointerTo(time.Now().UTC())
	modified := []structs.ControlledConfigEntry{}

	for _, route := range entries {
		updater := structs.NewStatusUpdater(route)

		for _, parent := range route.GetParents() {
			if parent.Kind == gateway.Kind && parent.Name == gateway.Name && parent.EnterpriseMeta.IsSame(&gateway.EnterpriseMeta) {
				updater.SetCondition(structs.Condition{
					Type:               "Bound",
					Status:             "False",
					Reason:             "GatewayNotFound",
					Message:            "gateway was not found",
					Resource:           pointerTo(parent),
					LastTransitionTime: now,
				})
			}
		}

		if toUpdate, shouldUpdate := updater.UpdateEntry(); shouldUpdate {
			modified = append(modified, toUpdate)
		}
	}

	return modified
}

// RemoveRoute unbinds the route from the given gateways, returning the list of gateways that were modified.
func RemoveRoute(route structs.ResourceReference, entries ...*gatewayMeta) []*gatewayMeta {
	modified := []*gatewayMeta{}

	for _, entry := range entries {
		if entry.unbindRoute(route) {
			modified = append(modified, entry)
		}
	}

	return modified
}
