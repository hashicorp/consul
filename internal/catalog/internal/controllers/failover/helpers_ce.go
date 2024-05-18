// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package failover

import (
	"github.com/hashicorp/consul/internal/controller"
)

func registerEnterpriseControllerWatchers(ctrl *controller.Controller) *controller.Controller {
	return ctrl
}
