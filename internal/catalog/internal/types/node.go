// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package types

import (
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	NodeKind = "Node"
)

var (
	NodeV1Alpha1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV1Alpha1,
		Kind:         NodeKind,
	}

	NodeType = NodeV1Alpha1Type
)

func RegisterNode(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     NodeV1Alpha1Type,
		Proto:    &pbcatalog.Node{},
		Validate: nil,
	})
}
