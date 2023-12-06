// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package workloadselectionmapper

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
)

func TestMapToComputedType(t *testing.T) {
	resourceClient := svctest.NewResourceServiceBuilder().
		WithRegisterFns(types.Register, catalog.RegisterTypes).
		Run(t)

	mapper := New[*pbmesh.ProxyConfiguration](pbmesh.ComputedProxyConfigurationType)

	workloadData := &pbcatalog.Workload{
		Addresses: []*pbcatalog.WorkloadAddress{{Host: "1.1.1.1"}},
		Ports: map[string]*pbcatalog.WorkloadPort{
			"tcp":  {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
			"mesh": {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
		},
		Identity: "test",
	}
	wID1 := resourcetest.Resource(pbcatalog.WorkloadType, "api-1").
		WithData(t, workloadData).
		Write(t, resourceClient).GetId()
	wID2 := resourcetest.Resource(pbcatalog.WorkloadType, "api-2").
		WithData(t, workloadData).
		Write(t, resourceClient).GetId()
	wID3 := resourcetest.Resource(pbcatalog.WorkloadType, "api-abc").
		WithData(t, workloadData).
		Write(t, resourceClient).GetId()
	wID4 := resourcetest.Resource(pbcatalog.WorkloadType, "foo").
		WithData(t, workloadData).
		Write(t, resourceClient).GetId()

	pCfg1 := resourcetest.Resource(pbmesh.ProxyConfigurationType, "cfg1").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, &pbmesh.ProxyConfiguration{
			Workloads: &pbcatalog.WorkloadSelector{
				Names:    []string{"foo", "api-1"},
				Prefixes: []string{"api-a"},
			},
			DynamicConfig: &pbmesh.DynamicConfig{
				Mode: pbmesh.ProxyMode_PROXY_MODE_TRANSPARENT,
			},
		}).Build()
	reqs, err := mapper.MapToComputedType(context.Background(), controller.Runtime{Client: resourceClient}, pCfg1)
	require.NoError(t, err)
	prototest.AssertElementsMatch(t,
		controller.MakeRequests(pbmesh.ComputedProxyConfigurationType, []*pbresource.ID{wID1, wID3, wID4}),
		reqs)

	pCfg2 := resourcetest.Resource(pbmesh.ProxyConfigurationType, "cfg2").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, &pbmesh.ProxyConfiguration{
			Workloads: &pbcatalog.WorkloadSelector{
				Prefixes: []string{"api-"},
				Names:    []string{"foo"},
			},
			BootstrapConfig: &pbmesh.BootstrapConfig{
				PrometheusBindAddr: "0.0.0.0:9000",
			},
		}).Build()

	reqs, err = mapper.MapToComputedType(context.Background(), controller.Runtime{Client: resourceClient}, pCfg2)
	require.NoError(t, err)
	prototest.AssertElementsMatch(t,
		controller.MakeRequests(pbmesh.ComputedProxyConfigurationType, []*pbresource.ID{wID1, wID2, wID3, wID4}),
		reqs)

	// Check mapper state for each workload.
	ids := mapper.IDsForWorkload(wID1)
	prototest.AssertElementsMatch(t, []*pbresource.ID{pCfg1.Id, pCfg2.Id}, ids)

	ids = mapper.IDsForWorkload(wID2)
	prototest.AssertElementsMatch(t, []*pbresource.ID{pCfg2.Id}, ids)

	ids = mapper.IDsForWorkload(wID3)
	prototest.AssertElementsMatch(t, []*pbresource.ID{pCfg1.Id, pCfg2.Id}, ids)

	ids = mapper.IDsForWorkload(wID4)
	prototest.AssertElementsMatch(t, []*pbresource.ID{pCfg1.Id, pCfg2.Id}, ids)

	// Update pCfg2's selector and check that we generate requests for previous and new selector.
	pCfg2 = resourcetest.Resource(pbmesh.ProxyConfigurationType, "cfg2").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, &pbmesh.ProxyConfiguration{
			Workloads: &pbcatalog.WorkloadSelector{
				Names: []string{"foo"},
			},
			BootstrapConfig: &pbmesh.BootstrapConfig{
				PrometheusBindAddr: "0.0.0.0:9000",
			},
		}).Build()

	reqs, err = mapper.MapToComputedType(context.Background(), controller.Runtime{Client: resourceClient}, pCfg2)
	require.NoError(t, err)
	prototest.AssertElementsMatch(t,
		controller.MakeRequests(pbmesh.ComputedProxyConfigurationType, []*pbresource.ID{wID4, wID1, wID2, wID3, wID4}),
		reqs)

	// Check mapper state for each workload.
	ids = mapper.IDsForWorkload(wID1)
	prototest.AssertElementsMatch(t, []*pbresource.ID{pCfg1.Id}, ids)

	ids = mapper.IDsForWorkload(wID2)
	prototest.AssertElementsMatch(t, []*pbresource.ID{}, ids)

	ids = mapper.IDsForWorkload(wID3)
	prototest.AssertElementsMatch(t, []*pbresource.ID{pCfg1.Id}, ids)

	ids = mapper.IDsForWorkload(wID4)
	prototest.AssertElementsMatch(t, []*pbresource.ID{pCfg1.Id, pCfg2.Id}, ids)

	// Untrack one of the proxy cfgs and check that mapper is updated.
	mapper.UntrackID(pCfg1.Id)

	ids = mapper.IDsForWorkload(wID1)
	prototest.AssertElementsMatch(t, []*pbresource.ID{}, ids)

	ids = mapper.IDsForWorkload(wID2)
	prototest.AssertElementsMatch(t, []*pbresource.ID{}, ids)

	ids = mapper.IDsForWorkload(wID3)
	prototest.AssertElementsMatch(t, []*pbresource.ID{}, ids)

	ids = mapper.IDsForWorkload(wID4)
	prototest.AssertElementsMatch(t, []*pbresource.ID{pCfg2.Id}, ids)
}
