// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package gateways

import (
	"testing"

	"github.com/hashicorp/consul/api"
)

// TestAPIGateway_ListenerJWTRequiresEnterprise documents that JWT validation on
// API gateway listeners is an enterprise feature, and that Community Edition
// servers refuse the configuration up front instead of silently ignoring it.
func TestAPIGateway_ListenerJWTRequiresEnterprise(t *testing.T) {
	t.Parallel()

	e := newAPIEnv(t, false)

	jwt := &api.APIGatewayPolicy{
		JWT: &api.APIGatewayJWTRequirement{Providers: []*api.APIGatewayJWTProvider{{Name: "okta"}}},
	}

	for name, listener := range map[string]api.APIGatewayListener{
		"override": {Name: "l", Port: 7001, Protocol: "http", Override: jwt},
		"default":  {Name: "l", Port: 7001, Protocol: "http", Default: jwt},
	} {
		listener := listener
		t.Run(name, func(t *testing.T) {
			requireSetFails(t, e.server, apiGateway("jwt-gw-"+name, listener), "jwt validation is a consul enterprise feature")
		})
	}
}
