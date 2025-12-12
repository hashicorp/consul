// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package structs

// APIGatewayJWTRequirement holds the list of JWT providers to be verified against
type APIGatewayJWTRequirement struct{}

// JWTFilter holds the JWT Filter configuration for an HTTPRoute
type JWTFilter struct{}
