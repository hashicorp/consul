// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package controllers

import (
	"github.com/hashicorp/consul/internal/catalog/internal/controllers/nodehealth"
	"github.com/hashicorp/consul/internal/controller"
)

func Register(mgr *controller.Manager) {
	mgr.Register(nodehealth.NodeHealthController())
}
