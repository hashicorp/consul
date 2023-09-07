// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func IsRouteType(typ *pbresource.Type) bool {
	switch {
	case resource.EqualType(typ, HTTPRouteType),
		resource.EqualType(typ, GRPCRouteType),
		resource.EqualType(typ, TCPRouteType):
		return true
	}
	return false
}

func IsFailoverPolicyType(typ *pbresource.Type) bool {
	switch {
	case resource.EqualType(typ, catalog.FailoverPolicyType):
		return true
	}
	return false
}

func IsDestinationPolicyType(typ *pbresource.Type) bool {
	switch {
	case resource.EqualType(typ, DestinationPolicyType):
		return true
	}
	return false
}

func IsServiceType(typ *pbresource.Type) bool {
	switch {
	case resource.EqualType(typ, catalog.ServiceType):
		return true
	}
	return false
}

func IsComputedRoutesType(typ *pbresource.Type) bool {
	switch {
	case resource.EqualType(typ, ComputedRoutesType):
		return true
	}
	return false
}
