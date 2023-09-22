// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	UpstreamsConfigurationKind = "UpstreamsConfiguration"
)

var (
	UpstreamsConfigurationV2Beta1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV2beta1,
		Kind:         UpstreamsConfigurationKind,
	}

	UpstreamsConfigurationType = UpstreamsConfigurationV2Beta1Type
)

func RegisterUpstreamsConfiguration(r resource.Registry) {
	r.Register(resource.Registration{
		Proto:    &pbmesh.UpstreamsConfiguration{},
		Validate: nil,
	})
}
