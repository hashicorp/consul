// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package controllers

import (
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/multicluster/internal/controllers/v1compat"
)

type CompatDependencies struct {
	ConfigEntryExports v1compat.AggregatedConfig
}

func RegisterCompat(mgr *controller.Manager, deps CompatDependencies) {
	mgr.Register(v1compat.Controller(deps.ConfigEntryExports))
}
