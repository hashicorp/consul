package controllers

import (
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/multicluster/internal/controllers/exportedservices"
)

func Register(mgr *controller.Manager) {
	mgr.Register(exportedservices.Controller())
}
