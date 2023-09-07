// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package gateways

import (
	"context"

	"github.com/hashicorp/consul/agent/consul/controller"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
)

func (r *apiGatewayReconciler) enqueueJWTProviderReferencedGatewaysAndHTTPRoutes(_ *state.Store, _ context.Context, _ controller.Request) error {
	return nil
}

func (m *gatewayMeta) checkJWTProviders(_ *state.Store) (map[structs.ResourceReference]error, error) {
	return nil, nil
}

func validateJWTForRoute(_ *state.Store, _ *structs.StatusUpdater, _ *structs.HTTPRouteConfigEntry) error {
	return nil
}
