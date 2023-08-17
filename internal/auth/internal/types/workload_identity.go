// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package types

import (
	"github.com/hashicorp/consul/internal/catalog/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	WorkloadIdentityKind = "WorkloadIdentity"
)

var (
	WorkloadIdentityV1Alpha1Type = &pbresource.Type{
		Group:        types.GroupName,
		GroupVersion: types.VersionV1Alpha1,
		Kind:         WorkloadIdentityKind,
	}

	WorkloadIdentityType = WorkloadIdentityV1Alpha1Type
)

func RegisterWorkloadIdentity(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     WorkloadIdentityV1Alpha1Type,
		Proto:    &pbcatalog.Workload{},
		Validate: types.ValidateWorkload,
	})
}

func ValidateWorkloadIdentity(res *pbresource.Resource) error {
	var workloadIdentity pbcatalog.WorkloadIdentity

	// TODO: check some things?

	return nil
}
