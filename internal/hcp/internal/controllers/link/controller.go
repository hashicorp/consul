package link

import (
	"context"

	"github.com/hashicorp/consul/internal/controller"
	pbhcp "github.com/hashicorp/consul/proto-public/pbhcp/v1"
)

func HCPLinkController() controller.Controller {
	return controller.ForType(pbhcp.LinkConfigurationType).
		WithReconciler(&hcpLinkReconciler{})
}

type hcpLinkReconciler struct{}

func (_ *hcpLinkReconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	// Apply the updated configuration to the system

	return nil
}
