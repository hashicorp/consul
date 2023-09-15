// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	UpstreamsConfigurationKind = "UpstreamsConfiguration"
)

var (
	UpstreamsConfigurationV1Alpha1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV1Alpha1,
		Kind:         UpstreamsConfigurationKind,
	}

	UpstreamsConfigurationType = UpstreamsConfigurationV1Alpha1Type
)

func RegisterUpstreamsConfiguration(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     UpstreamsConfigurationV1Alpha1Type,
		Proto:    &pbmesh.UpstreamsConfiguration{},
		Scope:    resource.ScopeNamespace,
		Validate: nil,
	})
}
