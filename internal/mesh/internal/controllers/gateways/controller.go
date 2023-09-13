package gateways

import (
	"context"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
)

func Controller() controller.Controller {
	r := &reconciler{}
	return controller.ForType(types.ComputedGatewayType).
		WithWatch(types.APIGatewayType, nil).         // TODO(nathancoleman) define mapper
		WithWatch(types.MeshGatewayType, nil).        // TODO(nathancoleman) define mapper
		WithWatch(types.TerminatingGatewayType, nil). // TODO(nathancoleman) define mapper
		WithReconciler(r)
}

type reconciler struct {
}

func (r *reconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	//TODO implement me
	panic("implement me")
}
