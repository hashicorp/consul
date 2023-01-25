package gateways

import (
	"fmt"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
)

// gatewayMeta embeds both a BoundAPIGateway and its corresponding APIGateway.
type gatewayMeta struct {
	// Bound is the bound-api-gateway config entry for a given gateway.
	Bound *structs.BoundAPIGatewayConfigEntry
	// Gateway is the api-gateway config entry for the gateway.
	Gateway *structs.APIGatewayConfigEntry
}

// getGatewayMeta queries the state store for an API Gateway and a Bound API
// Gateway matching the given name and enterprise meta and returns a GatewayMeta
// struct containing both.
func getGatewayMeta(store *state.Store, name string, entMeta *acl.EnterpriseMeta) (*gatewayMeta, error) {
	_, bound, err := store.ConfigEntry(nil, structs.BoundAPIGateway, name, entMeta)
	if err != nil {
		return nil, err
	}

	_, gateway, err := store.ConfigEntry(nil, structs.APIGateway, name, entMeta)
	if err != nil {
		return nil, err
	}

	return &gatewayMeta{
		Bound:   bound.(*structs.BoundAPIGatewayConfigEntry),
		Gateway: gateway.(*structs.APIGatewayConfigEntry),
	}, nil
}

// updateRouteBinding takes a parent resource reference and a BoundRoute and
// modifies the listeners on the BoundAPIGateway config entry in GatewayMeta
// to reflect the binding of the route to the gateway.
//
// If the reference is not valid or the route's protocol does not match the
// targeted listener's protocol, a mapping of parent references to associated
// errors is returned.
func (g *gatewayMeta) updateRouteBinding(refs []structs.ResourceReference, route structs.BoundRoute) (bool, map[structs.ResourceReference]error) {
	if g.Bound == nil || g.Gateway == nil {
		return false, nil
	}

	didUpdate := false
	errors := make(map[structs.ResourceReference]error)

	if len(g.Bound.Listeners) == 0 {
		for _, ref := range refs {
			errors[ref] = fmt.Errorf("route cannot bind because gateway has no listeners")
		}
		return false, errors
	}

	for i, listener := range g.Bound.Listeners {
		// Unbind to handle any stale route references.
		didUnbind := listener.UnbindRoute(route)
		if didUnbind {
			didUpdate = true
		}
		g.Bound.Listeners[i] = listener

		for _, ref := range refs {
			didBind, err := g.bindRoute(ref, route)
			if err != nil {
				errors[ref] = err
			}
			if didBind {
				didUpdate = true
			}
		}
	}

	return didUpdate, errors
}

func (g *gatewayMeta) bindRoute(ref structs.ResourceReference, route structs.BoundRoute) (bool, error) {
	if g.Bound == nil || g.Gateway == nil {
		return false, fmt.Errorf("gateway cannot be found")
	}

	if ref.Kind != structs.APIGateway || g.Gateway.Name != ref.Name || !g.Gateway.EnterpriseMeta.IsSame(&ref.EnterpriseMeta) {
		return false, nil
	}

	if len(g.Bound.Listeners) == 0 {
		return false, fmt.Errorf("route cannot bind because gateway has no listeners")
	}

	didBind := false
	for _, listener := range g.Gateway.Listeners {
		// A route with a section name of "" is bound to all listeners on the gateway.
		if listener.Name == ref.SectionName || ref.SectionName == "" {
			if listener.Protocol == route.GetProtocol() {
				i, boundListener := g.boundListenerByName(listener.Name)
				if boundListener != nil && boundListener.BindRoute(route) {
					didBind = true
					g.Bound.Listeners[i] = *boundListener
				}
			} else if ref.SectionName != "" {
				// Failure to bind to a specific listener is an error
				return false, fmt.Errorf("failed to bind route %s to gateway %s: listener %s is not a %s listener", route.GetName(), g.Gateway.Name, listener.Name, route.GetProtocol())
			}
		}
	}

	if !didBind {
		return false, fmt.Errorf("failed to bind route %s to gateway %s: no valid listener has name '%s' and uses %s protocol", route.GetName(), g.Gateway.Name, ref.SectionName, route.GetProtocol())
	}

	return true, nil
}

func (g *gatewayMeta) unbindRoute(route structs.BoundRoute) bool {
	if g.Bound == nil {
		return false
	}

	didUnbind := false
	for i, listener := range g.Bound.Listeners {
		if listener.UnbindRoute(route) {
			didUnbind = true
			g.Bound.Listeners[i] = listener
		}
	}

	return didUnbind
}

func (g *gatewayMeta) boundListenerByName(name string) (int, *structs.BoundAPIGatewayListener) {
	for i, listener := range g.Bound.Listeners {
		if listener.Name == name {
			return i, &listener
		}
	}
	return -1, nil
}
