// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package structs

// APIGatewayJWTRequirement holds the list of JWT providers to be verified against
type APIGatewayJWTRequirement struct{}

// APIGatewayJWTProvider defines the provider to verify a JWT against
type APIGatewayJWTProvider struct{}
