// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/internal/resource"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
)

func RegisterWorkloadIdentity(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     pbauth.WorkloadIdentityType,
		Proto:    &pbauth.WorkloadIdentity{},
		Scope:    resource.ScopeNamespace,
		Validate: nil,
	})
}
