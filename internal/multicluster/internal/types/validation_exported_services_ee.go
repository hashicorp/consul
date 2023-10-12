// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build consulent

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

	if len(exportedService.Services) == 0 {
		merr = multierror.Append(merr, resource.ErrInvalidField{
			Name:    "services",
			Wrapped: fmt.Errorf("at least one service must be set"),
		})
	}

	return merr
}

func ValidateNamespaceExportedServices(res *pbresource.Resource) error {
	// No-op
	return nil
}

func ValidatePartitionExportedServices(res *pbresource.Resource) error {
	// No-op
	return nil
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

	return merr
}
