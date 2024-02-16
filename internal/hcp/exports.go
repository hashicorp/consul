// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hcp

import (
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/hcp/internal/controllers"
	"github.com/hashicorp/consul/internal/hcp/internal/controllers/link"
	"github.com/hashicorp/consul/internal/hcp/internal/types"
	"github.com/hashicorp/consul/internal/resource"
)

// RegisterTypes adds all resource types within the "hcp" API group
// to the given type registry
func RegisterTypes(r resource.Registry) {
	types.Register(r)
}

type ControllerDependencies = controllers.Dependencies

var IsValidated = link.IsValidated
var LinkName = types.LinkName

// RegisterControllers registers controllers for the catalog types with
// the given controller Manager.
func RegisterControllers(mgr *controller.Manager, deps ControllerDependencies) {
	controllers.Register(mgr, deps)
}

// Needed for testing
var StatusKey = link.StatusKey
var ConditionValidatedSuccess = link.ConditionValidatedSuccess
var ConditionValidatedFailed = link.ConditionValidatedFailed
