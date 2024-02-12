// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package trafficpermissions

import (
	"github.com/hashicorp/consul/internal/auth/internal/types"
	"github.com/hashicorp/consul/internal/controller/cache/index"
	"github.com/hashicorp/consul/internal/controller/cache/indexers"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	TenancyIndexName   = "tenancy"
	BoundRefsIndexName = "bound-references"
)

func indexNtpByTenancy() *index.Index {
	return indexers.DecodedSingleIndexer(
		TenancyIndexName,
		index.SingleValueFromArgs(func(t *pbresource.Tenancy) ([]byte, error) {
			return index.IndexFromTenancy(t), nil
		}),
		func(r *types.DecodedNamespaceTrafficPermissions) (bool, []byte, error) {
			return true, index.IndexFromTenancy(r.Id.Tenancy), nil
		},
	)
}

func indexPtpByTenancy() *index.Index {
	return indexers.DecodedSingleIndexer(
		TenancyIndexName,
		index.SingleValueFromArgs(func(t *pbresource.Tenancy) ([]byte, error) {
			return index.IndexFromTenancy(t), nil
		}),
		func(r *types.DecodedPartitionTrafficPermissions) (bool, []byte, error) {
			return true, index.IndexFromTenancy(r.Id.Tenancy), nil
		},
	)
}

var boundRefsIndex = indexers.BoundRefsIndex[*pbauth.ComputedTrafficPermissions](BoundRefsIndexName)
