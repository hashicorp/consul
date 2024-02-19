// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package expander_ce

import (
	"context"

	"github.com/hashicorp/consul/internal/auth/internal/types"
	"github.com/hashicorp/consul/internal/controller"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v2beta1"
)

type XTrafficPermissions interface {
	GetAction() pbauth.Action
	GetPermissions() []*pbauth.Permission
}

type SamenessGroupExpander struct{}

func New() *SamenessGroupExpander {
	return &SamenessGroupExpander{}
}

func (sgE *SamenessGroupExpander) Expand(xtp types.XTrafficPermissions,
	_ map[string][]*pbmulticluster.SamenessGroupMember) ([]*pbauth.Permission, []string) {
	return xtp.GetPermissions(), nil
}

func (sgE *SamenessGroupExpander) List(_ context.Context, _ controller.Runtime,
	_ controller.Request) (map[string][]*pbmulticluster.SamenessGroupMember, error) {
	// no-op for CE
	return nil, nil
}
