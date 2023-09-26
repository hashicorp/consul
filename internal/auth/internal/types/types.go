// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/internal/resource"
)

func Register(r resource.Registry) {
	RegisterWorkloadIdentity(r)
	RegisterTrafficPermissions(r)
	RegisterComputedTrafficPermission(r)
}
