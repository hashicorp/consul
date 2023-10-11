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

func ValidateExportedServices(res *pbresource.Resource) error {
	var exportedService pbmulticluster.ExportedServices

	if err := res.Data.UnmarshalTo(&exportedService); err != nil {
		return resource.NewErrDataParse(&exportedService, err)
	}

	var merr error

	if exportedService.Services == nil || len(exportedService.Services) == 0 {
		merr = multierror.Append(merr, resource.ErrInvalidField{
			Name:    "services",
			Wrapped: fmt.Errorf("at least one service must be set"),
		})
	}

	var hasSetEnterpriseFeatures bool

	if res.Id != nil && res.Id.Tenancy != nil && (res.Id.Tenancy.Namespace != "" || res.Id.Tenancy.Partition != "") {
		hasSetEnterpriseFeatures = true
	}

	for _, consumer := range exportedService.Consumers {
		if consumer.GetPartition() != "" || consumer.GetSamenessGroup() != "" {
			hasSetEnterpriseFeatures = true
		}
	}

	if hasSetEnterpriseFeatures {
		merr = multierror.Append(merr, resource.ErrInvalidField{
			Name:    "namespace or partition",
			Wrapped: fmt.Errorf("namespace or partition can only be set in Enterprise"),
		})
	}

	return merr
}

func ValidateNamespaceExportedServices(res *pbresource.Resource) error {
	var exportedService pbmulticluster.NamespaceExportedServices

	if err := res.Data.UnmarshalTo(&exportedService); err != nil {
		return resource.NewErrDataParse(&exportedService, err)
	}

	var merr error

	var hasSetEnterpriseFeatures bool

	if res.Id != nil && res.Id.Tenancy != nil && (res.Id.Tenancy.Namespace != "" || res.Id.Tenancy.Partition != "") {
		hasSetEnterpriseFeatures = true
	}

	for _, consumer := range exportedService.Consumers {
		if consumer.GetPartition() != "" || consumer.GetSamenessGroup() != "" {
			hasSetEnterpriseFeatures = true
		}
	}

	if hasSetEnterpriseFeatures {
		merr = multierror.Append(merr, resource.ErrInvalidField{
			Name:    "partition or sameness group",
			Wrapped: fmt.Errorf("partition or sameness group can only be set in Enterprise"),
		})
	}

	return merr
}

func ValidatePartitionExportedServices(res *pbresource.Resource) error {
	var exportedService pbmulticluster.PartitionExportedServices

	if err := res.Data.UnmarshalTo(&exportedService); err != nil {
		return resource.NewErrDataParse(&exportedService, err)
	}

	var merr error

	var hasSetEnterpriseFeatures bool

	if res.Id != nil && res.Id.Tenancy != nil && (res.Id.Tenancy.Namespace != "" || res.Id.Tenancy.Partition != "") {
		hasSetEnterpriseFeatures = true
	}

	for _, consumer := range exportedService.Consumers {
		if consumer.GetPartition() != "" || consumer.GetSamenessGroup() != "" {
			hasSetEnterpriseFeatures = true
		}
	}

	if hasSetEnterpriseFeatures {
		merr = multierror.Append(merr, resource.ErrInvalidField{
			Name:    "partition or sameness group",
			Wrapped: fmt.Errorf("partition or sameness group can only be set in Enterprise"),
		})
	}

	return merr
}

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

	var hasSetEnterpriseFeatures bool

	if res.Id != nil && res.Id.Tenancy != nil && (res.Id.Tenancy.Namespace != "" || res.Id.Tenancy.Partition != "") {
		hasSetEnterpriseFeatures = true
	}

	for _, consumer := range computedExportedServices.GetConsumers() {
		for _, computedExportedServiceConsumer := range consumer.GetConsumers() {
			if computedExportedServiceConsumer.GetPartition() != "" || computedExportedServiceConsumer.GetNamespace() != "" {
				hasSetEnterpriseFeatures = true
				break
			}
		}
	}

	if hasSetEnterpriseFeatures {
		merr = multierror.Append(merr, resource.ErrInvalidField{
			Name:    "namespace or partition",
			Wrapped: fmt.Errorf("namespace or partition can only be set in Enterprise"),
		})
	}

	return merr
}
