// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package expander

import (
	"github.com/hashicorp/consul/internal/auth/internal/controllers/trafficpermissions/expander/expander_ce"
)

func GetSamenessGroupExpander() *expander_ce.SamenessGroupExpander {
	return expander_ce.New()
}
