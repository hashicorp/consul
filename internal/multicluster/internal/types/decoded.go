// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/internal/resource"
	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v2beta1"
)

type (
	DecodedExportedServices          = resource.DecodedResource[*pbmulticluster.ExportedServices]
	DecodedNamespaceExportedServices = resource.DecodedResource[*pbmulticluster.NamespaceExportedServices]
	DecodedPartitionExportedServices = resource.DecodedResource[*pbmulticluster.PartitionExportedServices]

	DecodedSamenessGroup = resource.DecodedResource[*pbmulticluster.SamenessGroup]
)
