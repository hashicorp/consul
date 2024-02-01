// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"fmt"

	"github.com/hashicorp/consul/internal/resource"
	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/go-multierror"
)

func validateExportedServiceConsumerCommon(consumer *pbmulticluster.ExportedServicesConsumer, indx int) error {
	switch consumer.GetConsumerTenancy().(type) {
	case *pbmulticluster.ExportedServicesConsumer_Peer:
		{
			if consumer.GetPeer() == "" || consumer.GetPeer() == resource.DefaultPeerName {
				return resource.ErrInvalidListElement{
					Name:    "peer",
					Index:   indx,
					Wrapped: fmt.Errorf("can not be empty or local"),
				}
			}
		}
	case *pbmulticluster.ExportedServicesConsumer_Partition:
		{
			if consumer.GetPartition() == "" {
				return resource.ErrInvalidListElement{
					Name:    "partition",
					Index:   indx,
					Wrapped: fmt.Errorf("can not be empty"),
				}
			}
		}
	case *pbmulticluster.ExportedServicesConsumer_SamenessGroup:
		{
			if consumer.GetSamenessGroup() == "" {
				return resource.ErrInvalidListElement{
					Name:    "sameness_group",
					Index:   indx,
					Wrapped: fmt.Errorf("can not be empty"),
				}
			}
		}
	}
	return nil
}

func validateExportedServicesConsumersEnterprise(consumers []*pbmulticluster.ExportedServicesConsumer) error {
	var merr error

	for indx, consumer := range consumers {
		vmerr := validateExportedServiceConsumerCommon(consumer, indx)
		if vmerr != nil {
			merr = multierror.Append(merr, vmerr)
		}
		vmerr = validateExportedServicesConsumer(consumer, indx)
		if vmerr != nil {
			merr = multierror.Append(merr, vmerr)
		}
	}

	return merr
}

func ValidateExportedServices(res *pbresource.Resource) error {
	var exportedService pbmulticluster.ExportedServices

	if err := res.Data.UnmarshalTo(&exportedService); err != nil {
		return resource.NewErrDataParse(&exportedService, err)
	}

	var merr error

	if len(exportedService.Services) == 0 {
		merr = multierror.Append(merr, resource.ErrInvalidField{
			Name:    "services",
			Wrapped: fmt.Errorf("at least one service must be set"),
		})
	}

	vmerr := validateExportedServicesConsumersEnterprise(exportedService.Consumers)

	if vmerr != nil {
		merr = multierror.Append(merr, vmerr)
	}

	return merr
}

func ValidateNamespaceExportedServices(res *pbresource.Resource) error {
	var exportedService pbmulticluster.NamespaceExportedServices

	if err := res.Data.UnmarshalTo(&exportedService); err != nil {
		return resource.NewErrDataParse(&exportedService, err)
	}

	return validateExportedServicesConsumersEnterprise(exportedService.Consumers)
}

func ValidatePartitionExportedServices(res *pbresource.Resource) error {
	var exportedService pbmulticluster.PartitionExportedServices

	if err := res.Data.UnmarshalTo(&exportedService); err != nil {
		return resource.NewErrDataParse(&exportedService, err)
	}

	return validateExportedServicesConsumersEnterprise(exportedService.Consumers)
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

	vmerr := ValidateComputedExportedServicesEnterprise(&computedExportedServices)

	if vmerr != nil {
		merr = multierror.Append(merr, vmerr)
	}

	return merr
}
