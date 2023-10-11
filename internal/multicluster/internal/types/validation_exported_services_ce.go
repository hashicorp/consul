// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package types

import (
	"fmt"
	"github.com/hashicorp/consul/internal/resource"
	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/go-multierror"
)

func ValidateComputedExportedServices(res *pbresource.Resource) error {
	var computedExportedServices pbmulticluster.ComputedExportedServices

	if err := res.Data.UnmarshalTo(&computedExportedServices); err != nil {
		return resource.NewErrDataParse(&computedExportedServices, err)
	}

	var merr error

	if res.Id.Name != ComputedExportedServicesName {
		merr = multierror.Append(merr, resource.ErrInvalidField{
			Name:    "name",
			Wrapped: fmt.Errorf("name can only be \"global\""),
		})
	}

	var hasPartitionOrNamespaceSet bool

	if res.Id.Tenancy.Namespace != "" || res.Id.Tenancy.Partition != "" {
		hasPartitionOrNamespaceSet = true
	}

	for _, consumer := range computedExportedServices.GetConsumers() {
		for _, computedExportedServiceConsumer := range consumer.GetConsumers() {
			if computedExportedServiceConsumer.GetPartition() != "" || computedExportedServiceConsumer.GetNamespace() != "" {
				hasPartitionOrNamespaceSet = true
				break
			}
		}
	}

	if hasPartitionOrNamespaceSet {
		merr = multierror.Append(merr, resource.ErrInvalidField{
			Name:    "namespace or partition",
			Wrapped: fmt.Errorf("namespace or partition can only be set in Enterprise"),
		})
	}

	return merr
}
