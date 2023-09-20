package gateways

import (
	"context"
	"errors"

	"github.com/hashicorp/consul/internal/catalog/internal/types"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh"
)

func APIGatewayController() controller.Controller {
	r := &apiGatewayReconciler{}
	return controller.ForType(types.APIGatewayType).
		WithWatch(mesh.ComputedRoutesType, nil). // TODO(nathancoleman) define mapper
		WithReconciler(r)
}

type apiGatewayReconciler struct {
}

func (r *apiGatewayReconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	return errors.New(`not implemented`)
}
