// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package expander

import "github.com/hashicorp/consul/internal/multicluster/internal/controllers/exportedservices/expander/expander_ce"

func New() *expander_ce.SamenessGroupExpander {
	return expander_ce.New()
}
