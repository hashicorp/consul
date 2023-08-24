//go:build !consulent
// +build !consulent

package xds

import (
	envoy_http_jwt_authn_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/jwt_authn/v3"
	envoy_http_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"

	"github.com/hashicorp/consul/agent/structs"
)

type GatewayAuthFilterBuilder struct {
	listener       structs.APIGatewayListener
	route          *structs.HTTPRouteConfigEntry
	providers      map[string]*structs.JWTProviderConfigEntry
	envoyProviders map[string]*envoy_http_jwt_authn_v3.JwtProvider
}

func (g *GatewayAuthFilterBuilder) makeGatewayAuthFilters() ([]*envoy_http_v3.HttpFilter, error) {
	return nil, nil
}
