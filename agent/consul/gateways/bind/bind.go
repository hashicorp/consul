package bind

import (
	"errors"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
)

// ReferenceSet stores an O(1) accessible set of ResourceReference objects.
type ReferenceSet = map[structs.ResourceReference]any

// BindGateways takes a reference to the state store and a route.
// It iterates over the parent references for the given route which are gateways the
// route should be bound to and updates those BoundAPIGatewayConfigEntry objects accordingly.
// The function returns a list of references to the modified BoundAPIGatewayConfigEntry objects,
// a map of resource references to errors that occurred when they were attempted to be
// bound to a gateway, and an error if the overall process was unsucessful.
func BindGateways(store *state.Store, route BoundRouter) ([]*structs.BoundAPIGatewayConfigEntry, map[structs.ResourceReference]error, error) {
	parentRefs := getParentReferences(route)

	modifiedState := make(map[configentry.KindName]*structs.BoundAPIGatewayConfigEntry)

	// errored stores the errors from events where a resource reference failed to bind to a gateway.
	errored := make(map[structs.ResourceReference]error)

	// Iterate over all BoundAPIGateway config entries and try to bind them to the route if they are a parent.
	_, entries, err := store.ConfigEntriesByKind(nil, structs.BoundAPIGateway, acl.WildcardEnterpriseMeta())
	if err != nil {
		return nil, nil, err
	}
	for _, entry := range entries {
		gateway := entry.(*structs.BoundAPIGatewayConfigEntry)
		key := configentry.NewKindNameForEntry(gateway)
		for reference := range parentRefs {
			didBind, err := bind(gateway, reference, route)
			if err != nil {
				errored[reference] = err
				delete(parentRefs, reference)
				continue
			}
			if didBind {
				delete(parentRefs, reference)
				modifiedState[key] = gateway
			}
		}
		if _, ok := modifiedState[key]; ok && unbind(gateway, route) {
			modifiedState[key] = gateway
		}
	}

	// Add all references that aren't bound at this point to the error set.
	for reference := range parentRefs {
		errored[reference] = errors.New("invalid reference to missing parent")
	}

	modified := []*structs.BoundAPIGatewayConfigEntry{}
	for _, gateway := range modifiedState {
		modified = append(modified, gateway)
	}

	return modified, errored, nil
}

func getParentReferences(route BoundRouter) ReferenceSet {
	refs := make(map[structs.ResourceReference]any)

	for _, ref := range route.GetParents() {
		refs[ref] = struct{}{}
	}

	return refs
}

func bind(gateway *structs.BoundAPIGatewayConfigEntry, reference structs.ResourceReference, route BoundRouter) (bool, error) {
	if reference.Kind != "Gateway" || reference.Name != gateway.Name || !reference.EnterpriseMeta.IsSame(&gateway.EnterpriseMeta) {
		return false, nil
	}

	return false, nil
}

func unbind(gateway *structs.BoundAPIGatewayConfigEntry, route BoundRouter) bool {
	return false
}

func routeReferencesGateway(gateway structs.APIGatewayConfigEntry, route BoundRouter) bool {
	return false
}
