// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package meshconfiguration

import (
	"context"
	"errors"

	"github.com/hashicorp/consul/internal/controller"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
)

func Controller() controller.Controller {
	r := &reconciler{}

	return controller.ForType(pbmesh.MeshConfigurationType).WithReconciler(r)
}

type reconciler struct{}

func (r *reconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	return errors.New("not implemented")
}
