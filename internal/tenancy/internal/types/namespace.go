// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
	tenancyv1alpha1 "github.com/hashicorp/consul/proto-public/pbtenancy/v1alpha1"
)

const (
	NamespaceKind = "Namespace"
)

var (
	NamespaceV1Alpha1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV1Alpha1,
		Kind:         NamespaceKind,
	}

	TrafficPermissionsType = NamespaceV1Alpha1Type
)

func RegisterNamespace(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     NamespaceV1Alpha1Type,
		Proto:    &tenancyv1alpha1.Namespace{},
		Scope:    resource.ScopePartition,
		Validate: ValidateNamespace,
	})
}

func ValidateNamespace(res *pbresource.Resource) error {
	var ns tenancyv1alpha1.Namespace

	if err := res.Data.UnmarshalTo(&ns); err != nil {
		return resource.NewErrDataParse(&ns, err)
	}
	if res.Id.Name == resource.DefaultNamespacedTenancy().Namespace {
		return errInvalidName
	}
	return nil
}
