// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package apigateways

import (
	"context"
	"errors"

	"github.com/hashicorp/consul/internal/controller"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
)

const (
	ControllerName = "consul.io/api-gateway"
)

func Controller() *controller.Controller {
	r := &reconciler{}

	return controller.NewController(ControllerName, pbmesh.APIGatewayType).
		WithReconciler(r)
}

type reconciler struct{}

// Reconcile is responsible for creating a Service w/ a MeshGateway owner,
// in addition to other things discussed in the RFC.
func (r *reconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	rt.Logger = rt.Logger.With("resource-id", req.ID)
	rt.Logger.Trace("reconciling api gateway")

	//TODO NET-7378

	return errors.New("not implemented")
}
