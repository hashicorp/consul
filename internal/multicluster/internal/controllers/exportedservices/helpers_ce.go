// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package exportedservices

import (
	"github.com/hashicorp/consul/internal/controller"
)

func registerEnterpriseResourcesWatchers(controller *controller.Controller) *controller.Controller {
	return controller
}
