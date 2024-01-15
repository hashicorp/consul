// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package controllers

import (
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/tenancy/internal/controllers/namespace"
)

func Register(mgr *controller.Manager, deps Dependencies) {
	mgr.Register(namespace.Controller(deps.Registry))
}
