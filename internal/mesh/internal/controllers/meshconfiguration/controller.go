// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package meshconfiguration

import (
	"context"
	"errors"

	"github.com/hashicorp/consul/internal/controller"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
)

const (
	ControllerName = "consul.io/mesh-configuration"
)

// Controller instantiates a new Controller for managing MeshConfiguration resources.
func Controller() *controller.Controller {
	r := &reconciler{}

	return controller.NewController(ControllerName, pbmesh.MeshConfigurationType).WithReconciler(r)
}

// reconciler implements the Reconciler interface to modify runtime state based
// on requests passed into it.
type reconciler struct{}

// Reconcile takes in the current controller request and updates the runtime state based on
// the request received.
func (r *reconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	return errors.New("not implemented")
}
