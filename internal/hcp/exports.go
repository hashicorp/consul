package hcp

import (
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/hcp/internal/controllers"
	"github.com/hashicorp/consul/internal/hcp/internal/types"
	"github.com/hashicorp/consul/internal/resource"
)

// RegisterTypes adds all resource types within the "catalog" API group
// to the given type registry
func RegisterTypes(r resource.Registry) {
	types.Register(r)
}

type ControllerDependencies = controllers.Dependencies

func DefaultControllerDependencies() ControllerDependencies {
	return ControllerDependencies{}
}

// RegisterControllers registers controllers for the catalog types with
// the given controller Manager.
func RegisterControllers(mgr *controller.Manager, deps ControllerDependencies) {
	controllers.Register(mgr, deps)
}
