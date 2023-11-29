// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package controllers

import (
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/hcp/internal/controllers/hcclink"
)

func Register(mgr *controller.Manager) {
	mgr.Register(hcclink.HCCLinkController())
}
