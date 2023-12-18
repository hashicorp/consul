// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package meshgateways

import (
	"context"
	"errors"

	"github.com/hashicorp/consul/internal/controller"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
)

const (
	ControllerName = "consul.io/mesh-gateway"
)

func Controller() *controller.Controller {
	r := &reconciler{}

	return controller.NewController(ControllerName, pbmesh.MeshGatewayType).
		WithReconciler(r)
}

type reconciler struct{}

func (r *reconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	// TODO NET-6426, NET-6427, NET-6428, NET-6429, NET-6430, NET-6431, NET-6432
	return errors.New("not implemented")
}
