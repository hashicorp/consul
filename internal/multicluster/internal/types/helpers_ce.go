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

func ValidateExportedServicesConsumersEnterprise(consumers []*multiclusterv1alpha1.ExportedServicesConsumer) error {
	var merr error

	for indx, consumer := range consumers {
		merr = validateExportedServicesConsumer(consumer, merr, indx)
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
					Name:    "Partition",
					Index:   indx,
					Wrapped: fmt.Errorf("can only be set in Enterprise"),
				})
			}
		}
	}

	return merr
}

func MutateComputedExportedServices(res *pbresource.Resource) error {
	var ces multiclusterv1alpha1.ComputedExportedServices

	if err := res.Data.UnmarshalTo(&ces); err != nil {
		return err
	}

	var changed bool

	for _, cesConsumer := range ces.GetConsumers() {
		for _, consumer := range cesConsumer.GetConsumers() {
			switch t := consumer.GetConsumerTenancy().(type) {
			case *multiclusterv1alpha1.ComputedExportedServicesConsumer_Partition:
				if t.Partition == "" {
					changed = true
					t.Partition = resource.DefaultPartitionName
				}
			}
		}
	}

	if !changed {
		return nil
	}

	return res.Data.MarshalFrom(&ces)
}

func updatePartitionInConsumers(exportedServiceConsumers []*multiclusterv1alpha1.ExportedServicesConsumer) bool {
	var changed bool

	for _, consumer := range exportedServiceConsumers {
		changed = changed || updatePartitionIfNotSet(consumer)
	}

	if !changed {
		return true
	}
	return false
}

func MutateExportedServices(res *pbresource.Resource) error {
	var es multiclusterv1alpha1.ExportedServices

	if err := res.Data.UnmarshalTo(&es); err != nil {
		return err
	}

	notChanged := updatePartitionInConsumers(es.Consumers)

	if notChanged {
		return nil
	}

	return res.Data.MarshalFrom(&es)
}

func MutateNamespaceExportedServices(res *pbresource.Resource) error {
	var nes multiclusterv1alpha1.NamespaceExportedServices

	if err := res.Data.UnmarshalTo(&nes); err != nil {
		return err
	}

	notChanged := updatePartitionInConsumers(nes.Consumers)

	if notChanged {
		return nil
	}

	return res.Data.MarshalFrom(&nes)
}

func MutatePartitionExportedServices(res *pbresource.Resource) error {
	var pes multiclusterv1alpha1.PartitionExportedServices

	if err := res.Data.UnmarshalTo(&pes); err != nil {
		return err
	}

	notChanged := updatePartitionInConsumers(pes.Consumers)

	if notChanged {
		return nil
	}

	return res.Data.MarshalFrom(&pes)
}

func updatePartitionIfNotSet(consumer *multiclusterv1alpha1.ExportedServicesConsumer) bool {
	var updated bool

	switch t := consumer.GetConsumerTenancy().(type) {
	case *multiclusterv1alpha1.ExportedServicesConsumer_Partition:
		if t.Partition == "" {
			updated = true
			t.Partition = resource.DefaultPartitionName
		}
	}
	return updated
}
