// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package types

import (
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	UpstreamsKind = "Upstreams"
)

var (
	UpstreamsV1Alpha1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: CurrentVersion,
		Kind:         UpstreamsKind,
	}

	UpstreamsType = UpstreamsV1Alpha1Type
)

func RegisterUpstreams(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     UpstreamsV1Alpha1Type,
		Proto:    &pbmesh.Upstreams{},
		Validate: nil,
	})
}
