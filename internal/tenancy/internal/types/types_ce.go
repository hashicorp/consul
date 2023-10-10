// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package types

import "github.com/hashicorp/consul/internal/resource"

func Register(r resource.Registry) {
	RegisterNamespace(r)
}
