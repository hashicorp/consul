// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	UpstreamsKind = "Upstreams"
)

var (
	UpstreamsV2Beta1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV2beta1,
		Kind:         UpstreamsKind,
	}

	UpstreamsType = UpstreamsV2Beta1Type
)

func RegisterUpstreams(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     UpstreamsV2Beta1Type,
		Proto:    &pbmesh.Upstreams{},
		Scope:    resource.ScopeNamespace,
		Validate: nil,
	})
}
