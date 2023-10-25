// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package types

import (
	"fmt"
	"github.com/hashicorp/consul/internal/resource"
	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v2beta1"
	"github.com/hashicorp/go-multierror"
)

func validateExportedServicesConsumer(consumer *pbmulticluster.ExportedServicesConsumer, indx int) error {
	switch consumer.GetConsumerTenancy().(type) {
	case *pbmulticluster.ExportedServicesConsumer_Partition:
		return resource.ErrInvalidListElement{
			Name:    "partition",
			Index:   indx,
			Wrapped: fmt.Errorf("can only be set in Enterprise"),
		}
	case *pbmulticluster.ExportedServicesConsumer_SamenessGroup:
		return resource.ErrInvalidListElement{
			Name:    "sameness_group",
			Index:   indx,
			Wrapped: fmt.Errorf("can only be set in Enterprise"),
		}
	}
	return nil
}

func ValidateComputedExportedServicesEnterprise(computedExportedServices *pbmulticluster.ComputedExportedServices) error {

	var merr error

	for indx, consumer := range computedExportedServices.GetConsumers() {
		for _, computedExportedServiceConsumer := range consumer.GetConsumers() {
			switch computedExportedServiceConsumer.GetConsumerTenancy().(type) {
			case *pbmulticluster.ComputedExportedServicesConsumer_Partition:
				merr = multierror.Append(merr, resource.ErrInvalidListElement{
					Name:    "partition",
					Index:   indx,
					Wrapped: fmt.Errorf("can only be set in Enterprise"),
				})
				if computedExportedServiceConsumer.GetPartition() == "" {
					merr = multierror.Append(merr, resource.ErrInvalidListElement{
						Name:    "partition",
						Index:   indx,
						Wrapped: fmt.Errorf("can not be empty"),
					})
				}
			case *pbmulticluster.ComputedExportedServicesConsumer_Peer:
				if computedExportedServiceConsumer.GetPeer() == "" {
					merr = multierror.Append(merr, resource.ErrInvalidListElement{
						Name:    "peer",
						Index:   indx,
						Wrapped: fmt.Errorf("can not be empty"),
					})
				}
			}
		}
	}

	return merr
}
