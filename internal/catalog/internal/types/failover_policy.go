// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package types

import (
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	FailoverPolicyKind = "FailoverPolicy"
)

var (
	FailoverPolicyV1Alpha1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV1Alpha1,
		Kind:         FailoverPolicyKind,
	}

	FailoverPolicyType = FailoverPolicyV1Alpha1Type
)

func RegisterFailoverPolicy(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     FailoverPolicyV1Alpha1Type,
		Proto:    &pbcatalog.FailoverPolicy{},
		Validate: nil,
		Mutate:   nil,
	})
}
