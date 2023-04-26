package controller

import (
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// ForType begins building a Controller for the given resource type.
func ForType(managedType *pbresource.Type) Controller {
	return Controller{managedType: managedType}
}

// Controller runs a reconciliation loop to respond to changes in resources and
// their dependencies. It is heavily inspired by Kubernetes' controller pattern:
// https://kubernetes.io/docs/concepts/architecture/controller/
//
// Use the builder methods in this package (starting with ForType) to construct
// a controller, and then pass it to a Manager to be executed.
type Controller struct {
	managedType *pbresource.Type
}
