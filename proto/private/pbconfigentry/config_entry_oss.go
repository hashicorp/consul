// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package pbconfigentry

import "github.com/hashicorp/consul/agent/structs"

func gwJWTRequirementToStructs(m *APIGatewayJWTRequirement) *structs.APIGatewayJWTRequirement {
	return &structs.APIGatewayJWTRequirement{}
}

func gwJWTRequirementFromStructs(*structs.APIGatewayJWTRequirement) *APIGatewayJWTRequirement {
	return &APIGatewayJWTRequirement{}
}

func gwJWTProviderToStructs(m []*APIGatewayJWTProvider) []*structs.APIGatewayJWTProvider {
	return []*structs.APIGatewayJWTProvider{}
}

func gwJWTProviderFromStructs([]*structs.APIGatewayJWTProvider) []*APIGatewayJWTProvider {
	return []*APIGatewayJWTProvider{}
}
