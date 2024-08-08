// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/internal/resource"
)

func Register(r resource.Registry) {
	RegisterExportedServices(r)
	RegisterNamespaceExportedServices(r)
	RegisterPartitionExportedServices(r)
	RegisterComputedExportedServices(r)
}
