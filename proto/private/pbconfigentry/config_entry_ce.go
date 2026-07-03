// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package pbconfigentry

import "github.com/hashicorp/consul/agent/structs"

func gwJWTRequirementToStructs(m *APIGatewayJWTRequirement) *structs.APIGatewayJWTRequirement {
	return &structs.APIGatewayJWTRequirement{}
}

func gwJWTRequirementFromStructs(*structs.APIGatewayJWTRequirement) *APIGatewayJWTRequirement {
	return &APIGatewayJWTRequirement{}
}

func routeJWTFilterToStructs(m *JWTFilter) *structs.JWTFilter {
	return &structs.JWTFilter{}
}

func routeJWTFilterFromStructs(*structs.JWTFilter) *JWTFilter {
	return &JWTFilter{}
}

func routeExtAuthzFilterToStructs(m *HTTPRouteExtAuthzFilter) *structs.HTTPRouteExtAuthzFilter {
	if m == nil {
		return nil
	}
	return &structs.HTTPRouteExtAuthzFilter{}
}

func routeExtAuthzFilterFromStructs(m *structs.HTTPRouteExtAuthzFilter) *HTTPRouteExtAuthzFilter {
	if m == nil {
		return nil
	}
	return &HTTPRouteExtAuthzFilter{}
}

func gwExtAuthzToStructs(m *APIGatewayExtAuthz) *structs.APIGatewayExtAuthz {
	if m == nil {
		return nil
	}
	return &structs.APIGatewayExtAuthz{}
}

func gwExtAuthzFromStructs(m *structs.APIGatewayExtAuthz) *APIGatewayExtAuthz {
	if m == nil {
		return nil
	}
	return &APIGatewayExtAuthz{}
}
