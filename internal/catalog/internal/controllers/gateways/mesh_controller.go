package gateways

import (
	"context"
	"errors"

	"github.com/hashicorp/consul/internal/catalog/internal/types"
	"github.com/hashicorp/consul/internal/controller"
)

func MeshGatewayController() controller.Controller {
	r := &meshGatewayReconciler{}
	return controller.ForType(types.MeshGatewayType).WithReconciler(r)
}

type meshGatewayReconciler struct {
}

func (r *meshGatewayReconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	return errors.New(`not implemented`)
}
