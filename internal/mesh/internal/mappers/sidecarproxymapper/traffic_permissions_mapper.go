// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package sidecarproxymapper

import (
	"context"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func (m *Mapper) MapComputedTrafficPermissionsToProxyStateTemplate(_ context.Context, _ controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	var ctp pbauth.ComputedTrafficPermissions
	err := res.Data.UnmarshalTo(&ctp)
	if err != nil {
		return nil, err
	}

	pid := resource.ReplaceType(pbauth.WorkloadIdentityType, res.Id)
	ids := m.identitiesCache.ProxyIDsByWorkloadIdentity(pid)

	requests := make([]controller.Request, 0, len(ids))
	for _, id := range ids {
		requests = append(requests, controller.Request{
			ID: resource.ReplaceType(pbmesh.ProxyStateTemplateType, id)},
		)
	}

	return requests, nil
}
