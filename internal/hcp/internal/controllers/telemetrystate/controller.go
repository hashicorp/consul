package telemetrystate

import (
	"context"

	"github.com/hashicorp/consul/internal/controller"
)

type stateReconciler struct{}

func (r *stateReconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	return nil
}
