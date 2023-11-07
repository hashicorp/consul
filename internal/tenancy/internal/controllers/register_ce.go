// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package controllers

import (
	"github.com/hashicorp/consul/internal/controller"
)

func Register(mgr *controller.Manager, deps Dependencies) {
	//mgr.Register(namespace.NamespaceController())
}
