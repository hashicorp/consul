// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

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

func (m *gatewayMeta) checkJWTProviders() (map[structs.ResourceReference]error, error) {
	return nil, nil
}

func (m *gatewayMeta) validateJWTForRoute(_ *structs.HTTPRouteConfigEntry) (bool, map[structs.ResourceReference]error) {
	return true, nil
}
