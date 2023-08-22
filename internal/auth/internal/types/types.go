// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package types

import (
	"github.com/hashicorp/consul/internal/resource"
)

const (
	GroupName       = "auth"
	VersionV1Alpha1 = "v1alpha1"
	CurrentVersion  = VersionV1Alpha1

	ActionAllow = "allow"
	ActionDeny  = "deny"
)

func Register(r resource.Registry) {
	RegisterWorkloadIdentity(r)
	RegisterTrafficPermission(r)
	RegisterComputedTrafficPermission(r)
}
