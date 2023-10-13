// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build consulent

package types

import (
	multiclusterv1alpha1 "github.com/hashicorp/consul/proto-public/pbmulticluster/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func ValidateExportedServicesEnterprise(res *pbresource.Resource, exportedService *multiclusterv1alpha1.ExportedServices) error {
	// no op for ENT
	return nil
}

func ValidateNamespaceExportedServicesEnterprise(res *pbresource.Resource, exportedService *multiclusterv1alpha1.NamespaceExportedServices) error {
	// no op for ENT
	return nil
}

func ValidateComputedExportedServicesEnterprise(res *pbresource.Resource, exportedService *multiclusterv1alpha1.ComputedExportedService) error {
	// no op for ENT
	return nil
}
