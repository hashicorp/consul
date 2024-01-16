// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/internal/resource"
)

const (
	GroupName      = "multicluster"
	VersionV2Beta1 = "v2beta1"
	CurrentVersion = VersionV2Beta1
)

func Register(r resource.Registry) {
	RegisterExportedServices(r)
	RegisterNamespaceExportedServices(r)
	RegisterPartitionExportedServices(r)
	RegisterComputedExportedServices(r)

	RegisterEnterpriseTypes(r)
}
