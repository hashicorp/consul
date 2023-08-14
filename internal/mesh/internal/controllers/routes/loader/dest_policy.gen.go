// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1
package loader

// Code generated by gen_memoizer_funcs.sh. DO NOT EDIT.

import (
	"context"
	"fmt"

	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// Avoid unused imports in generated code.
var _ *pbmesh.ParentReference

var _ *pbcatalog.Service

var _ = types.HTTPRouteType

var _ = catalog.ServiceType

func (m *memoizingLoader) GetDestinationPolicy(ctx context.Context, id *pbresource.ID) (*types.DecodedDestinationPolicy, error) {
	if !resource.EqualType(id.Type, types.DestinationPolicyType) {
		return nil, fmt.Errorf("expected *pbmesh.DestinationPolicy, not %s", resource.TypeToString(id.Type))
	}

	rk := resource.NewReferenceKey(id)

	if cached, ok := m.mapDestinationPolicy[rk]; ok {
		return cached, nil // cached value may be nil
	}

	dec, err := resource.GetDecodedResource[pbmesh.DestinationPolicy, *pbmesh.DestinationPolicy](ctx, m.client, id)
	if err != nil {
		return nil, err
	}

	m.mapDestinationPolicy[rk] = dec
	return dec, nil
}
