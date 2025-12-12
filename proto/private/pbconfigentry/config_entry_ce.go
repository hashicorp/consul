// Copyright IBM Corp. 2014, 2025
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
