// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	UpstreamsKind = "Destinations"
)

var (
	UpstreamsV1Alpha1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV1Alpha1,
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
