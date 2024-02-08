// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package authv2beta1

import (
	pbresource "github.com/hashicorp/consul/proto-public/pbresource"
)

func (src *Source) GetWorkloadIdentityReference() *pbresource.Reference {
	return &pbresource.Reference{
		Type: WorkloadIdentityType,
		Name: src.IdentityName,
		Tenancy: &pbresource.Tenancy{
			Partition: src.Partition,
			Namespace: src.Namespace,
		},
	}
}
