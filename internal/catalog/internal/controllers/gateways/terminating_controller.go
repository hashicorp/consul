package gateways

import (
	"context"
	"errors"

	"github.com/hashicorp/consul/internal/catalog/internal/types"
	"github.com/hashicorp/consul/internal/controller"
)

func TerminatingGatewayController() controller.Controller {
	r := &terminatingGatewayReconciler{}
	return controller.ForType(types.TerminatingGatewayType).WithReconciler(r)
}

type terminatingGatewayReconciler struct {
}

func (r *terminatingGatewayReconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	return errors.New(`not implemented`)
}
