// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package types

import (
	"fmt"
	"github.com/hashicorp/consul/internal/resource"
	multiclusterv1alpha1 "github.com/hashicorp/consul/proto-public/pbmulticluster/v1alpha1"
	"github.com/hashicorp/go-multierror"
)

func validateExportedServicesConsumer(consumer *multiclusterv1alpha1.ExportedServicesConsumer, merr error, indx int) error {
	switch consumer.GetConsumerTenancy().(type) {
	case *multiclusterv1alpha1.ExportedServicesConsumer_Partition:
		merr = multierror.Append(merr, resource.ErrInvalidListElement{
			Name:    "partition",
			Index:   indx,
			Wrapped: fmt.Errorf("can only be set in Enterprise"),
		})
	case *multiclusterv1alpha1.ExportedServicesConsumer_SamenessGroup:
		merr = multierror.Append(merr, resource.ErrInvalidListElement{
			Name:    "sameness_group",
			Index:   indx,
			Wrapped: fmt.Errorf("can only be set in Enterprise"),
		})
	}
	return merr
}

func ValidateComputedExportedServicesEnterprise(computedExportedServices *multiclusterv1alpha1.ComputedExportedServices) error {

	var merr error

	for indx, consumer := range computedExportedServices.GetConsumers() {
		for _, computedExportedServiceConsumer := range consumer.GetConsumers() {
			switch computedExportedServiceConsumer.GetConsumerTenancy().(type) {
			case *multiclusterv1alpha1.ComputedExportedServicesConsumer_Partition:
				merr = multierror.Append(merr, resource.ErrInvalidListElement{
					Name:    "partition",
					Index:   indx,
					Wrapped: fmt.Errorf("can only be set in Enterprise"),
				})
			}
		}
	}

	return merr
}
