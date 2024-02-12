// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package expander

import (
	"context"

	"github.com/hashicorp/consul/internal/auth/internal/types"
	"github.com/hashicorp/consul/internal/controller"
	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v2beta1"
)

// SamenessGroupExpander is used to expand sameness group for a ComputedTrafficPermission resource
type SamenessGroupExpander interface {
	Expand(types.XTrafficPermissions, map[string][]*pbmulticluster.SamenessGroupMember) []string
	List(context.Context, controller.Runtime, controller.Request) (map[string][]*pbmulticluster.SamenessGroupMember, error)
}
