package demo

import "github.com/hashicorp/consul/internal/controller"

// RegisterControllers registers controllers for the demo types. Should only be
// called in dev mode.
func RegisterControllers(mgr *controller.Manager) {
	mgr.Register(artistController())
}

func artistController() controller.Controller {
	return controller.ForType(TypeV2Artist)
}
