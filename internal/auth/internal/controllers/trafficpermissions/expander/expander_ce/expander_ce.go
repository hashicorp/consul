// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package expander_ce

import (
	"context"

	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v2beta1"

	"github.com/hashicorp/consul/internal/controller"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
)

type SamenessGroupExpander struct{}

func New() *SamenessGroupExpander {
	return &SamenessGroupExpander{}
}

func (sgE *SamenessGroupExpander) Expand(_ *pbauth.TrafficPermissions,
	_ map[string][]*pbmulticluster.SamenessGroupMember) []string {
	//no-op for CE
	return nil
}

func (sgE *SamenessGroupExpander) List(_ context.Context, _ controller.Runtime,
	_ controller.Request) (map[string][]*pbmulticluster.SamenessGroupMember, error) {
	//no-op for CE
	return nil, nil
}
