package gateways

import (
	"context"
	"fmt"

	"github.com/hashicorp/consul/agent/structs"
)

// Route is generic over routes that can be registered for the API Gateway.
type Route interface {
	structs.HTTPRouteConfigEntry | structs.TCPRouteConfigEntry
	GetParents() []structs.ResourceReference
}

// BindRoute takes an gateway and a route and binds/unbinds the route to/from
// the gateway depending on whether the route references the gateway or not.
// It returns true if the gateway is modified and false otherwise.
func BindRoute[T Route](ctx context.Context, gateway *structs.APIGatewayConfigEntry, route T) bool {
	if routeReferencesGateway(*gateway, route) {
		return bind(ctx, gateway, route)
	}

	return unbind(ctx, gateway, route)
}

func bind[T Route](ctx context.Context, gateway *structs.APIGatewayConfigEntry, route T) bool {
	for _, parent := range route.GetParents() {
		fmt.Println(parent)
	}

	return false
}

func unbind[T Route](ctx context.Context, gateway *structs.APIGatewayConfigEntry, route T) bool {
	return false
}

// TODO
func canBind() bool {
	return true
}

func routeReferencesGateway[T Route](gateway structs.APIGatewayConfigEntry, route T) bool {
	return false
}
