// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package multicluster

import (
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/multicluster/internal/controllers"
	"github.com/hashicorp/consul/internal/multicluster/internal/controllers/v1compat"
	"github.com/hashicorp/consul/internal/multicluster/internal/types"
	"github.com/hashicorp/consul/internal/resource"
)

// RegisterTypes adds all resource types within the "multicluster" API group
// to the given type registry
func RegisterTypes(r resource.Registry) {
	types.Register(r)
}

type CompatControllerDependencies = controllers.CompatDependencies

func DefaultCompatControllerDependencies(ac v1compat.AggregatedConfig) CompatControllerDependencies {
	return CompatControllerDependencies{
		ConfigEntryExports: ac,
	}
}

func RegisterCompatControllers(mgr *controller.Manager, deps CompatControllerDependencies) {
	controllers.RegisterCompat(mgr, deps)
}
