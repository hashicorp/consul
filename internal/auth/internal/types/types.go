// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/internal/resource"
)

const (
	GroupName      = "auth"
	VersionV2Beta1 = "v2beta1"
	CurrentVersion = VersionV2Beta1
)

func Register(r resource.Registry) {
	RegisterWorkloadIdentity(r)
	RegisterTrafficPermissions(r)
	RegisterComputedTrafficPermission(r)
}
