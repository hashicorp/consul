// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/internal/resource"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	WorkloadIdentityKind = "WorkloadIdentity"
)

var (
	WorkloadIdentityV2Beta1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV2Beta1,
		Kind:         WorkloadIdentityKind,
	}

	WorkloadIdentityType = WorkloadIdentityV2Beta1Type
)

func RegisterWorkloadIdentity(r resource.Registry) {
	r.Register(resource.Registration{
		Proto:    &pbauth.WorkloadIdentity{},
		Validate: nil,
	})
}
