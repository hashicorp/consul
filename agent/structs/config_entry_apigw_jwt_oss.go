// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package structs

// APIGatewayJWTRequirement holds the list of JWT providers to be verified against
type APIGatewayJWTRequirement struct{}

// JWTFilter holds the JWT Filter configuration for an HTTPRoute
type JWTFilter struct{}
