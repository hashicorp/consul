// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package structs

// DeepCopy generates a deep copy of *APIGatewayJWTRequirement
func (o *APIGatewayJWTRequirement) DeepCopy() *APIGatewayJWTRequirement {
	return new(APIGatewayJWTRequirement)
}

// DeepCopy generates a deep copy of *JWTFilter
func (o *JWTFilter) DeepCopy() *JWTFilter {
	return new(JWTFilter)
}
