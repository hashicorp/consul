// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package xds

import (
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/agent/structs"
)

type perRouteFilterBuilder struct {
	providerMap map[string]*structs.JWTProviderConfigEntry
	listener    *structs.APIGatewayListener
	route       *structs.HTTPRouteConfigEntry
}

func (p perRouteFilterBuilder) buildFilter(match *envoy_route_v3.RouteMatch) (map[string]*anypb.Any, error) {
	return nil, nil
}
