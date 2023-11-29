package hcclink

import (
	"context"

	"github.com/hashicorp/consul/internal/controller"
	pbhcp "github.com/hashicorp/consul/proto-public/pbhcp/v1"
)

const (
	StatusKey = "consul.io/hcc-link"
)

func HCCLinkController() controller.Controller {
	return controller.ForType(pbhcp.HCCLinkType).
		WithReconciler(&hccLinkReconciler{})
}

type hccLinkReconciler struct {
}

func (r *hccLinkReconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	// TODO: return an error if resources-api experiment is enabled
	return nil
}
