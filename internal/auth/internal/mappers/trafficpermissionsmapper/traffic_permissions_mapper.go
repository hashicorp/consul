// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package trafficpermissionsmapper

import (
	"context"

	"github.com/hashicorp/consul/internal/auth/internal/types"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func MapTrafficPermissions(_ context.Context, _ controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	tp, err := resource.Decode[*pbauth.TrafficPermissions](res)
	if err != nil {
		return nil, err
	}

	if tp.Data.Destination.IdentityName == "" {
		return nil, types.ErrWildcardNotSupported
	}

	return []controller.Request{{ID: &pbresource.ID{
		Type:    pbauth.ComputedTrafficPermissionsType,
		Tenancy: tp.Resource.Id.Tenancy,
		Name:    tp.Data.Destination.IdentityName,
	}}}, nil
}
