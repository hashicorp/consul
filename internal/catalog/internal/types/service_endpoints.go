// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package types

import (
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	ServiceEndpointsKind = "ServiceEndpoints"
)

var (
	ServiceEndpointsV1Alpha1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV1Alpha1,
		Kind:         ServiceEndpointsKind,
	}

	ServiceEndpointsType = ServiceEndpointsV1Alpha1Type
)

func RegisterServiceEndpoints(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     ServiceEndpointsV1Alpha1Type,
		Proto:    &pbcatalog.ServiceEndpoints{},
		Validate: nil,
	})
}
