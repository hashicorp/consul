package bind

import (
	"errors"
	"fmt"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"golang.org/x/exp/slices"
)

// ReferenceSet stores an O(1) accessible set of ResourceReference objects.
type ReferenceSet = map[structs.ResourceReference]any

// BoundRouter indicates a route that has parent gateways which
// can be accessed by calling the GetParents associated function.
type BoundRouter interface {
	structs.ConfigEntry
	GetParents() []structs.ResourceReference
}

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
		key := configentry.NewKindNameForEntry(gateway) // TODO rename to kindName
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

func refEqual(a, b structs.ResourceReference) bool {
	return a.Kind == b.Kind && a.Name == b.Name && a.EnterpriseMeta.IsSame(&b.EnterpriseMeta)
}

func toResourceReference(router BoundRouter) structs.ResourceReference {
	return structs.ResourceReference{
		Kind: router.GetKind(),
		Name: router.GetName(),
	}
}

func bind(gateway *structs.BoundAPIGatewayConfigEntry, reference structs.ResourceReference, route BoundRouter) (bool, error) {
	if reference.Kind != structs.APIGateway || reference.Name != gateway.Name || !reference.EnterpriseMeta.IsSame(&gateway.EnterpriseMeta) {
		return false, nil
	}

	if len(gateway.Listeners) == 0 {
		return false, fmt.Errorf("route cannot bind because gateway has no listeners")
	}

	didBind := false
	for _, listener := range gateway.Listeners {
		if listener.Name == reference.SectionName || reference.SectionName == "" {
			// Upsert the route to the listener.
			for i, listenerRoute := range listener.Routes {
				routeRef := toResourceReference(route)
				if refEqual(listenerRoute, routeRef) {
					listener.Routes[i] = routeRef
					didBind = true
				}
			}
			listener.Routes = append(listener.Routes, toResourceReference(route))
			didBind = true
		}
	}

	if !didBind {
		return false, fmt.Errorf("invalid section name: %s", reference.SectionName)
	}

	return true, nil
}

func unbind(gateway *structs.BoundAPIGatewayConfigEntry, route BoundRouter) bool {
	for _, listener := range gateway.Listeners {
		for i, listenerRoute := range listener.Routes {
			if refEqual(listenerRoute, toResourceReference(route)) {
				listener.Routes = slices.Delete(listener.Routes, i, i+1)
				return true
			}
		}
	}

	return false
}
