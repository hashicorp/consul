// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package types

import (
	"fmt"
	"github.com/hashicorp/consul/internal/resource"
	multiclusterv1alpha1 "github.com/hashicorp/consul/proto-public/pbmulticluster/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/go-multierror"
	"strings"
)

func ValidateExportedServicesEnterprise(res *pbresource.Resource, exportedService *multiclusterv1alpha1.ExportedServices) error {
	var hasSetEnterpriseFeatures bool

	invalidFields := make([]string, 0)

	if res.Id != nil && res.Id.Tenancy != nil && (res.Id.Tenancy.Namespace != "default" || res.Id.Tenancy.Partition != "default") {
		if res.Id.Tenancy.Namespace != "default" {
			invalidFields = append(invalidFields, "namespace")
		}
		if res.Id.Tenancy.Partition != "default" {
			invalidFields = append(invalidFields, "partition")
		}
		hasSetEnterpriseFeatures = true
	}

	for _, consumer := range exportedService.Consumers {
		if consumer.GetPartition() != "" || consumer.GetSamenessGroup() != "" {
			hasSetEnterpriseFeatures = true
		}
	}

	var merr error

	if hasSetEnterpriseFeatures {
		merr = multierror.Append(merr, resource.ErrInvalidField{
			Name:    strings.Join(invalidFields, ","),
			Wrapped: fmt.Errorf("namespace and partition can only be set in Enterprise"),
		})
	}

	return merr
}

func ValidateNamespaceExportedServicesEnterprise(res *pbresource.Resource, exportedService *multiclusterv1alpha1.NamespaceExportedServices) error {
	var merr error

	var hasSetEnterpriseFeatures bool

	invalidFields := make([]string, 0)

	if res.Id != nil && res.Id.Tenancy != nil && (res.Id.Tenancy.Namespace != "default" || res.Id.Tenancy.Partition != "default") {
		if res.Id.Tenancy.Namespace != "default" {
			invalidFields = append(invalidFields, "namespace")
		}
		if res.Id.Tenancy.Partition != "default" {
			invalidFields = append(invalidFields, "partition")
		}
		hasSetEnterpriseFeatures = true
	}

	for _, consumer := range exportedService.Consumers {
		if consumer.GetPartition() != "default" || consumer.GetSamenessGroup() != "default" {
			if consumer.GetPartition() != "default" {
				invalidFields = append(invalidFields, "namespace")
			}
			if res.Id.Tenancy.Partition != "default" {
				invalidFields = append(invalidFields, "partition")
			}
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

func ValidateComputedExportedServicesEnterprise(res *pbresource.Resource, computedExportedServices *multiclusterv1alpha1.ComputedExportedServices) error {
	var hasSetEnterpriseFeatures bool

	if res.Id != nil && res.Id.Tenancy != nil && (res.Id.Tenancy.Namespace != "default" || res.Id.Tenancy.Partition != "default") {
		hasSetEnterpriseFeatures = true
	}

	for _, consumer := range computedExportedServices.GetConsumers() {
		for _, computedExportedServiceConsumer := range consumer.GetConsumers() {
			if computedExportedServiceConsumer.GetPartition() != "" {
				hasSetEnterpriseFeatures = true
				break
			}
		}
	}

	var merr error

	if hasSetEnterpriseFeatures {
		merr = multierror.Append(merr, resource.ErrInvalidField{
			Name:    "namespace or partition",
			Wrapped: fmt.Errorf("namespace or partition can only be set in Enterprise"),
		})
	}

	return merr
}
