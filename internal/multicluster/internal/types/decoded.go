// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/internal/resource"
	v2 "github.com/hashicorp/consul/proto-public/pbmulticluster/v2"
)

type (
	DecodedExportedServices          = resource.DecodedResource[*v2.ExportedServices]
	DecodedNamespaceExportedServices = resource.DecodedResource[*v2.NamespaceExportedServices]
	DecodedPartitionExportedServices = resource.DecodedResource[*v2.PartitionExportedServices]
)
