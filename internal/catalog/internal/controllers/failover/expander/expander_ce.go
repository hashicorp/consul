// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package expander

import "github.com/hashicorp/consul/internal/catalog/internal/controllers/failover/expander/expander_ce"

func GetSamenessGroupExpander() *expander_ce.SamenessGroupExpander {
	return expander_ce.New()
}
