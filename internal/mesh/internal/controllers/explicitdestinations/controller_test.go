// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package explicitdestinations

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/proto"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/explicitdestinations/mapper"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/routes/routestest"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

type controllerTestSuite struct {
	suite.Suite

	client  *resourcetest.Client
	runtime controller.Runtime

	ctl *reconciler
	ctx context.Context

	workload    *pbcatalog.Workload
	workloadRes *pbresource.Resource

	dest1 *pbmesh.Destinations
	dest2 *pbmesh.Destinations

	destService1 *pbresource.Resource
	destService2 *pbresource.Resource
	destService3 *pbresource.Resource

	destService1Ref *pbresource.Reference
	destService2Ref *pbresource.Reference
	destService3Ref *pbresource.Reference

	serviceData *pbcatalog.Service

	destService1Routes *pbmesh.ComputedRoutes
	destService2Routes *pbmesh.ComputedRoutes
	destService3Routes *pbmesh.ComputedRoutes

	expComputedDest *pbmesh.ComputedExplicitDestinations
	tenancies       []*pbresource.Tenancy
}

func TestFindDuplicates(t *testing.T) {
	// Create some conflicting destinations.
	resourcetest.RunWithTenancies(func(tenancy *pbresource.Tenancy) {
		dest1 := &pbmesh.Destinations{
			Workloads: &pbcatalog.WorkloadSelector{
				Names: []string{"foo"},
			},
			Destinations: []*pbmesh.Destination{
				{
					ListenAddr: &pbmesh.Destination_IpPort{
						IpPort: &pbmesh.IPPortAddress{
							Ip:   "127.0.0.1",
							Port: 1000,
						},
					},
				},
				{
					ListenAddr: &pbmesh.Destination_IpPort{
						IpPort: &pbmesh.IPPortAddress{
							Ip:   "127.0.0.1",
							Port: 2000,
						},
					},
				},
			},
		}
		dest2 := &pbmesh.Destinations{
			Workloads: &pbcatalog.WorkloadSelector{
				Names: []string{"foo"},
			},
			Destinations: []*pbmesh.Destination{
				{
					ListenAddr: &pbmesh.Destination_IpPort{
						IpPort: &pbmesh.IPPortAddress{
							Ip:   "127.0.0.1",
							Port: 1000,
						},
					},
				},
			},
		}
		dest3 := &pbmesh.Destinations{
			Workloads: &pbcatalog.WorkloadSelector{
				Names: []string{"foo"},
			},
			Destinations: []*pbmesh.Destination{
				{
					ListenAddr: &pbmesh.Destination_Unix{
						Unix: &pbmesh.UnixSocketAddress{
							Path: "/foo/bar",
						},
					},
				},
			},
		}
		dest4 := &pbmesh.Destinations{
			Workloads: &pbcatalog.WorkloadSelector{
				Names: []string{"foo"},
			},
			Destinations: []*pbmesh.Destination{
				{
					ListenAddr: &pbmesh.Destination_Unix{
						Unix: &pbmesh.UnixSocketAddress{
							Path: "/foo/bar",
						},
					},
				},
			},
		}
		destNonConflicting := &pbmesh.Destinations{
			Workloads: &pbcatalog.WorkloadSelector{
				Names: []string{"foo"},
			},
			Destinations: []*pbmesh.Destination{
				{
					ListenAddr: &pbmesh.Destination_IpPort{
						IpPort: &pbmesh.IPPortAddress{
							Ip:   "127.0.0.1",
							Port: 3000,
						},
					},
				},
				{
					ListenAddr: &pbmesh.Destination_Unix{
						Unix: &pbmesh.UnixSocketAddress{
							Path: "/baz/bar",
						},
					},
				},
			},
		}

		var destinations []*types.DecodedDestinations
		dest1Res := resourcetest.Resource(pbmesh.DestinationsType, "dest1").
			WithData(t, dest1).
			WithTenancy(tenancy).
			Build()
		destinations = append(destinations, resourcetest.MustDecode[*pbmesh.Destinations](t, dest1Res))
		dest2Res := resourcetest.Resource(pbmesh.DestinationsType, "dest2").
			WithData(t, dest2).
			WithTenancy(tenancy).
			Build()
		destinations = append(destinations, resourcetest.MustDecode[*pbmesh.Destinations](t, dest2Res))
		dest3Res := resourcetest.Resource(pbmesh.DestinationsType, "dest3").
			WithData(t, dest3).
			WithTenancy(tenancy).
			Build()
		destinations = append(destinations, resourcetest.MustDecode[*pbmesh.Destinations](t, dest3Res))
		dest4Res := resourcetest.Resource(pbmesh.DestinationsType, "dest4").
			WithData(t, dest4).
			WithTenancy(tenancy).
			Build()
		destinations = append(destinations, resourcetest.MustDecode[*pbmesh.Destinations](t, dest4Res))
		nonConflictingDestRes := resourcetest.Resource(pbmesh.DestinationsType, "nonConflictingDest").
			WithData(t, destNonConflicting).
			WithTenancy(tenancy).
			Build()
		destinations = append(destinations, resourcetest.MustDecode[*pbmesh.Destinations](t, nonConflictingDestRes))

		duplicates := findConflicts(destinations)

		require.Contains(t, duplicates, resource.NewReferenceKey(dest1Res.Id))
		require.Contains(t, duplicates, resource.NewReferenceKey(dest2Res.Id))
		require.Contains(t, duplicates, resource.NewReferenceKey(dest3Res.Id))
		require.Contains(t, duplicates, resource.NewReferenceKey(dest4Res.Id))
		require.NotContains(t, duplicates, resource.NewReferenceKey(nonConflictingDestRes.Id))
	}, t)
}

func (suite *controllerTestSuite) SetupTest() {
	suite.tenancies = resourcetest.TestTenancies()
	resourceClient := svctest.NewResourceServiceBuilder().
		WithRegisterFns(types.Register, catalog.RegisterTypes).
		WithTenancies(suite.tenancies...).
		Run(suite.T())
	suite.runtime = controller.Runtime{Client: resourceClient, Logger: testutil.Logger(suite.T())}
	suite.ctx = testutil.TestContext(suite.T())
	suite.client = resourcetest.NewClient(resourceClient)
}

func (suite *controllerTestSuite) setupWithTenancy(tenancy *pbresource.Tenancy) {

	suite.ctl = &reconciler{
		mapper: mapper.New(),
	}

	suite.workload = &pbcatalog.Workload{
		Addresses: []*pbcatalog.WorkloadAddress{{Host: "1.1.1.1"}},
		Ports: map[string]*pbcatalog.WorkloadPort{
			"tcp":  {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
			"mesh": {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
		},
		Identity: "test",
	}

	suite.workloadRes = resourcetest.Resource(pbcatalog.WorkloadType, "test-workload").
		WithData(suite.T(), suite.workload).
		WithTenancy(tenancy).
		Write(suite.T(), suite.client)

	suite.T().Cleanup(func() {
		suite.client.MustDelete(suite.T(), suite.workloadRes.Id)
	})

	suite.serviceData = &pbcatalog.Service{
		Workloads: &pbcatalog.WorkloadSelector{Names: []string{"service-1-workloads"}},
		Ports: []*pbcatalog.ServicePort{
			{
				TargetPort: "tcp",
				Protocol:   pbcatalog.Protocol_PROTOCOL_TCP,
			},
			{
				TargetPort: "admin",
				Protocol:   pbcatalog.Protocol_PROTOCOL_TCP,
			},
			{
				TargetPort: "mesh",
				Protocol:   pbcatalog.Protocol_PROTOCOL_MESH,
			},
		},
	}
	suite.destService1 = resourcetest.Resource(pbcatalog.ServiceType, "dest-service-1").
		WithTenancy(tenancy).
		WithData(suite.T(), suite.serviceData).
		Build()

	suite.T().Cleanup(func() {
		suite.client.MustDelete(suite.T(), suite.destService1.Id)
	})

	suite.destService2 = resourcetest.Resource(pbcatalog.ServiceType, "dest-service-2").
		WithTenancy(tenancy).
		WithData(suite.T(), suite.serviceData).
		Build()
	suite.T().Cleanup(func() {
		suite.client.MustDelete(suite.T(), suite.destService2.Id)
	})

	suite.destService3 = resourcetest.Resource(pbcatalog.ServiceType, "dest-service-3").
		WithTenancy(tenancy).
		WithData(suite.T(), suite.serviceData).
		Build()

	suite.T().Cleanup(func() {
		suite.client.MustDelete(suite.T(), suite.destService3.Id)
	})

	suite.destService1Ref = resource.Reference(suite.destService1.Id, "")
	suite.destService2Ref = resource.Reference(suite.destService2.Id, "")
	suite.destService3Ref = resource.Reference(suite.destService3.Id, "")

	suite.destService1Routes = routestest.BuildComputedRoutes(suite.T(), resource.ReplaceType(pbmesh.ComputedRoutesType, suite.destService1.Id),
		resourcetest.MustDecode[*pbcatalog.Service](suite.T(), suite.destService1),
	).GetData()

	suite.destService2Routes = routestest.BuildComputedRoutes(suite.T(), resource.ReplaceType(pbmesh.ComputedRoutesType, suite.destService2.Id),
		resourcetest.MustDecode[*pbcatalog.Service](suite.T(), suite.destService2),
	).GetData()

	suite.destService3Routes = routestest.BuildComputedRoutes(suite.T(), resource.ReplaceType(pbmesh.ComputedRoutesType, suite.destService3.Id),
		resourcetest.MustDecode[*pbcatalog.Service](suite.T(), suite.destService3),
	).GetData()

	suite.dest1 = &pbmesh.Destinations{
		Workloads: &pbcatalog.WorkloadSelector{
			Names: []string{suite.workloadRes.Id.Name},
		},
		Destinations: []*pbmesh.Destination{
			{
				DestinationRef:  suite.destService1Ref,
				DestinationPort: "tcp",
				ListenAddr: &pbmesh.Destination_IpPort{
					IpPort: &pbmesh.IPPortAddress{
						Ip:   "127.0.0.1",
						Port: 1000,
					},
				},
			},
			{
				DestinationRef:  suite.destService2Ref,
				DestinationPort: "tcp",
				ListenAddr: &pbmesh.Destination_IpPort{
					IpPort: &pbmesh.IPPortAddress{
						Ip:   "127.0.0.1",
						Port: 2000,
					},
				},
			},
		},
	}

	suite.dest2 = &pbmesh.Destinations{
		Workloads: &pbcatalog.WorkloadSelector{
			Prefixes: []string{"test-"},
		},
		Destinations: []*pbmesh.Destination{
			{
				DestinationRef:  suite.destService3Ref,
				DestinationPort: "tcp",
				ListenAddr: &pbmesh.Destination_IpPort{
					IpPort: &pbmesh.IPPortAddress{
						Ip:   "127.0.0.1",
						Port: 3000,
					},
				},
			},
			{
				DestinationRef:  suite.destService2Ref,
				DestinationPort: "admin",
				ListenAddr: &pbmesh.Destination_IpPort{
					IpPort: &pbmesh.IPPortAddress{
						Ip:   "127.0.0.1",
						Port: 4000,
					},
				},
			},
		},
	}

	suite.expComputedDest = &pbmesh.ComputedExplicitDestinations{
		Destinations: []*pbmesh.Destination{
			{
				DestinationRef:  suite.destService1Ref,
				DestinationPort: "tcp",
				ListenAddr: &pbmesh.Destination_IpPort{
					IpPort: &pbmesh.IPPortAddress{
						Ip:   "127.0.0.1",
						Port: 1000,
					},
				},
			},
			{
				DestinationRef:  suite.destService2Ref,
				DestinationPort: "tcp",
				ListenAddr: &pbmesh.Destination_IpPort{
					IpPort: &pbmesh.IPPortAddress{
						Ip:   "127.0.0.1",
						Port: 2000,
					},
				},
			},
			{
				DestinationRef:  suite.destService3Ref,
				DestinationPort: "tcp",
				ListenAddr: &pbmesh.Destination_IpPort{
					IpPort: &pbmesh.IPPortAddress{
						Ip:   "127.0.0.1",
						Port: 3000,
					},
				},
			},
			{
				DestinationRef:  suite.destService2Ref,
				DestinationPort: "admin",
				ListenAddr: &pbmesh.Destination_IpPort{
					IpPort: &pbmesh.IPPortAddress{
						Ip:   "127.0.0.1",
						Port: 4000,
					},
				},
			},
		},
	}
}

func (suite *controllerTestSuite) TestReconcile_NoWorkload() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		id := resourcetest.Resource(pbmesh.ComputedExplicitDestinationsType, "not-found").WithTenancy(tenancy).ID()
		dest := resourcetest.Resource(pbmesh.DestinationsType, "dest1").
			WithTenancy(tenancy).
			WithData(suite.T(), suite.dest1).
			Build()
		decDest := resourcetest.MustDecode[*pbmesh.Destinations](suite.T(), dest)
		suite.ctl.mapper.TrackDestinations(id, []*types.DecodedDestinations{decDest})

		err := suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
			ID: id,
		})
		require.NoError(suite.T(), err)

		suite.client.RequireResourceNotFound(suite.T(), id)

		// Check that we're not tracking services for this workload anymore.
		reqs, err := suite.ctl.mapper.MapService(context.TODO(), controller.Runtime{}, suite.destService1)
		require.NoError(suite.T(), err)
		require.Nil(suite.T(), reqs)

		reqs, err = suite.ctl.mapper.MapService(context.TODO(), controller.Runtime{}, suite.destService2)
		require.NoError(suite.T(), err)
		require.Nil(suite.T(), reqs)
	})
}

func (suite *controllerTestSuite) TestReconcile_NonMeshWorkload() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		resourcetest.Resource(pbcatalog.WorkloadType, "non-mesh").
			WithTenancy(tenancy).
			WithData(suite.T(), &pbcatalog.Workload{
				Addresses: []*pbcatalog.WorkloadAddress{{Host: "1.1.1.1"}},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"tcp": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
				},
			}).
			Write(suite.T(), suite.client)

		cdID := resourcetest.Resource(pbmesh.ComputedExplicitDestinationsType, "non-mesh").
			WithTenancy(tenancy).
			Write(suite.T(), suite.client).Id

		dest := resourcetest.Resource(pbmesh.DestinationsType, "dest1").
			WithTenancy(tenancy).
			WithData(suite.T(), suite.dest1).
			Build()
		decDest := resourcetest.MustDecode[*pbmesh.Destinations](suite.T(), dest)
		suite.ctl.mapper.TrackDestinations(cdID, []*types.DecodedDestinations{decDest})

		err := suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
			ID: cdID,
		})
		require.NoError(suite.T(), err)

		suite.client.RequireResourceNotFound(suite.T(), cdID)

		// Check that we're not tracking services for this workload anymore.
		reqs, err := suite.ctl.mapper.MapService(context.TODO(), controller.Runtime{}, suite.destService1)
		require.NoError(suite.T(), err)
		require.Nil(suite.T(), reqs)

		reqs, err = suite.ctl.mapper.MapService(context.TODO(), controller.Runtime{}, suite.destService2)
		require.NoError(suite.T(), err)
		require.Nil(suite.T(), reqs)
	})
}

func (suite *controllerTestSuite) writeServices(t *testing.T, tenancy *pbresource.Tenancy) {
	// Write all services.
	resourcetest.Resource(pbcatalog.ServiceType, suite.destService1Ref.Name).
		WithTenancy(tenancy).
		WithData(t, suite.serviceData).
		Write(t, suite.client)
	resourcetest.Resource(pbcatalog.ServiceType, suite.destService2Ref.Name).
		WithTenancy(tenancy).
		WithData(t, suite.serviceData).
		Write(t, suite.client)
	resourcetest.Resource(pbcatalog.ServiceType, suite.destService3Ref.Name).
		WithTenancy(tenancy).
		WithData(t, suite.serviceData).
		Write(t, suite.client)
}

func (suite *controllerTestSuite) writeComputedRoutes(t *testing.T, tenancy *pbresource.Tenancy) {
	// Write computed routes
	resourcetest.Resource(pbmesh.ComputedRoutesType, suite.destService1Ref.Name).
		WithTenancy(tenancy).
		WithData(t, suite.destService1Routes).
		Write(t, suite.client)
	resourcetest.Resource(pbmesh.ComputedRoutesType, suite.destService2Ref.Name).
		WithTenancy(tenancy).
		WithData(t, suite.destService2Routes).
		Write(t, suite.client)
	resourcetest.Resource(pbmesh.ComputedRoutesType, suite.destService3Ref.Name).
		WithTenancy(tenancy).
		WithData(t, suite.destService3Routes).
		Write(t, suite.client)
}

func (suite *controllerTestSuite) TestReconcile_HappyPath() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		// Add configs in reverse alphabetical order.
		d2 := resourcetest.Resource(pbmesh.DestinationsType, "dest2").
			WithTenancy(tenancy).
			WithData(suite.T(), suite.dest2).
			Write(suite.T(), suite.client)
		_, err := suite.ctl.mapper.MapDestinations(suite.ctx, suite.runtime, d2)
		require.NoError(suite.T(), err)

		d1 := resourcetest.Resource(pbmesh.DestinationsType, "dest1").
			WithTenancy(tenancy).
			WithData(suite.T(), suite.dest1).
			Write(suite.T(), suite.client)
		_, err = suite.ctl.mapper.MapDestinations(suite.ctx, suite.runtime, d1)
		require.NoError(suite.T(), err)

		suite.writeServices(suite.T(), tenancy)
		suite.writeComputedRoutes(suite.T(), tenancy)

		cdID := resource.ReplaceType(pbmesh.ComputedExplicitDestinationsType, suite.workloadRes.Id)
		err = suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
			ID: cdID,
		})

		require.NoError(suite.T(), err)

		suite.requireComputedDestinations(suite.T(), cdID)
		suite.client.RequireStatusCondition(suite.T(), d1.Id, ControllerName, ConditionDestinationsAccepted())
	})
}

func (suite *controllerTestSuite) TestReconcile_NoDestinations() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		dest := resourcetest.Resource(pbmesh.DestinationsType, "dest").
			WithTenancy(tenancy).
			WithData(suite.T(), suite.dest1).
			Build()
		_, err := suite.ctl.mapper.MapDestinations(suite.ctx, suite.runtime, dest)
		require.NoError(suite.T(), err)

		cdID := resourcetest.Resource(pbmesh.ComputedExplicitDestinationsType, suite.workloadRes.Id.Name).
			WithTenancy(tenancy).
			Write(suite.T(), suite.client).Id
		err = suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
			ID: cdID,
		})

		require.NoError(suite.T(), err)

		suite.client.RequireResourceNotFound(suite.T(), cdID)
	})
}

func (suite *controllerTestSuite) TestReconcile_AllDestinationsInvalid() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		// We add a destination with services refs that don't exist which should result
		// in computed mapper being deleted because all mapper are invalid.
		dest := resourcetest.Resource(pbmesh.DestinationsType, "dest").
			WithTenancy(tenancy).
			WithData(suite.T(), suite.dest1).
			Write(suite.T(), suite.client)
		_, err := suite.ctl.mapper.MapDestinations(suite.ctx, suite.runtime, dest)
		require.NoError(suite.T(), err)

		cdID := resourcetest.Resource(pbmesh.ComputedExplicitDestinationsType, suite.workloadRes.Id.Name).
			WithTenancy(tenancy).
			Write(suite.T(), suite.client).Id
		err = suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
			ID: cdID,
		})

		require.NoError(suite.T(), err)

		suite.client.RequireResourceNotFound(suite.T(), cdID)
	})
}

func (suite *controllerTestSuite) TestReconcile_StatusUpdate_ConflictingDestination() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		dest1 := resourcetest.Resource(pbmesh.DestinationsType, "dest1").
			WithTenancy(tenancy).
			WithData(suite.T(), suite.dest1).
			Write(suite.T(), suite.client)
		_, err := suite.ctl.mapper.MapDestinations(suite.ctx, suite.runtime, dest1)
		require.NoError(suite.T(), err)

		// Write a conflicting destinations resource.
		destData := proto.Clone(suite.dest2).(*pbmesh.Destinations)
		destData.Destinations[0] = suite.dest1.Destinations[0]

		dest2 := resourcetest.Resource(pbmesh.DestinationsType, "dest2").
			WithTenancy(tenancy).
			WithData(suite.T(), destData).
			Write(suite.T(), suite.client)
		_, err = suite.ctl.mapper.MapDestinations(suite.ctx, suite.runtime, dest2)
		require.NoError(suite.T(), err)

		cdID := resourcetest.Resource(pbmesh.ComputedExplicitDestinationsType, suite.workloadRes.Id.Name).
			WithTenancy(tenancy).
			Write(suite.T(), suite.client).Id
		err = suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
			ID: cdID,
		})
		require.NoError(suite.T(), err)
		suite.client.RequireResourceNotFound(suite.T(), cdID)

		// Expect that the status on both resource is updated showing conflict.
		suite.client.RequireStatusCondition(suite.T(), dest1.Id, ControllerName,
			ConditionConflictFound(suite.workloadRes.Id))
		suite.client.RequireStatusCondition(suite.T(), dest2.Id, ControllerName,
			ConditionConflictFound(suite.workloadRes.Id))

		// Update dest2 back to have non-conflicting data.
		dest2 = resourcetest.Resource(pbmesh.DestinationsType, "dest2").
			WithTenancy(tenancy).
			WithData(suite.T(), suite.dest2).
			Write(suite.T(), suite.client)
		_, err = suite.ctl.mapper.MapDestinations(suite.ctx, suite.runtime, dest2)
		require.NoError(suite.T(), err)

		err = suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
			ID: cdID,
		})
		require.NoError(suite.T(), err)

		// Expect status on both to be updated to say that there's no conflict.
		suite.client.RequireStatusCondition(suite.T(), dest1.Id, ControllerName,
			ConditionConflictNotFound)
		suite.client.RequireStatusCondition(suite.T(), dest2.Id, ControllerName,
			ConditionConflictNotFound)
	})
}

func (suite *controllerTestSuite) TestReconcile_StatusUpdate_NoService() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		dest := resourcetest.Resource(pbmesh.DestinationsType, "dest").
			WithTenancy(tenancy).
			WithData(suite.T(), suite.dest2).
			Write(suite.T(), suite.client)
		_, err := suite.ctl.mapper.MapDestinations(suite.ctx, suite.runtime, dest)
		require.NoError(suite.T(), err)
		cdID := resourcetest.Resource(pbmesh.ComputedExplicitDestinationsType, suite.workloadRes.Id.Name).
			WithTenancy(tenancy).
			Write(suite.T(), suite.client).Id
		err = suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
			ID: cdID,
		})
		require.NoError(suite.T(), err)
		suite.client.RequireResourceNotFound(suite.T(), cdID)

		suite.client.RequireStatusCondition(suite.T(), dest.Id, ControllerName,
			ConditionDestinationServiceNotFound(resource.ReferenceToString(suite.destService3Ref)))
	})
}

func (suite *controllerTestSuite) TestReconcile_StatusUpdate_ServiceNotOnMesh() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		dest := resourcetest.Resource(pbmesh.DestinationsType, "dest").
			WithTenancy(tenancy).
			WithData(suite.T(), suite.dest2).
			Write(suite.T(), suite.client)
		_, err := suite.ctl.mapper.MapDestinations(suite.ctx, suite.runtime, dest)
		require.NoError(suite.T(), err)

		resourcetest.Resource(pbcatalog.ServiceType, suite.destService3Ref.Name).
			WithTenancy(tenancy).
			WithData(suite.T(), &pbcatalog.Service{
				Workloads: &pbcatalog.WorkloadSelector{Names: []string{suite.workloadRes.Id.Name}},
				Ports: []*pbcatalog.ServicePort{
					{
						TargetPort: "tcp",
						Protocol:   pbcatalog.Protocol_PROTOCOL_TCP,
					},
				},
			}).
			Write(suite.T(), suite.client)

		cdID := resourcetest.Resource(pbmesh.ComputedExplicitDestinationsType, suite.workloadRes.Id.Name).
			WithTenancy(tenancy).
			Write(suite.T(), suite.client).Id

		err = suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
			ID: cdID,
		})
		require.NoError(suite.T(), err)
		suite.client.RequireResourceNotFound(suite.T(), cdID)

		suite.client.RequireStatusCondition(suite.T(), dest.Id, ControllerName,
			ConditionMeshProtocolNotFound(resource.ReferenceToString(suite.destService3Ref)))
	})
}

func (suite *controllerTestSuite) TestReconcile_StatusUpdate_DestinationPortIsMesh() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		dest := resourcetest.Resource(pbmesh.DestinationsType, "dest").
			WithTenancy(tenancy).
			WithData(suite.T(), suite.dest2).
			Write(suite.T(), suite.client)
		_, err := suite.ctl.mapper.MapDestinations(suite.ctx, suite.runtime, dest)
		require.NoError(suite.T(), err)

		resourcetest.Resource(pbcatalog.ServiceType, suite.destService3Ref.Name).
			WithTenancy(tenancy).
			WithData(suite.T(), &pbcatalog.Service{
				Workloads: &pbcatalog.WorkloadSelector{Names: []string{suite.workloadRes.Id.Name}},
				Ports: []*pbcatalog.ServicePort{
					{
						TargetPort: "tcp",
						Protocol:   pbcatalog.Protocol_PROTOCOL_MESH,
					},
				},
			}).
			Write(suite.T(), suite.client)

		cdID := resourcetest.Resource(pbmesh.ComputedExplicitDestinationsType, suite.workloadRes.Id.Name).
			WithTenancy(tenancy).
			Write(suite.T(), suite.client).Id

		err = suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
			ID: cdID,
		})
		require.NoError(suite.T(), err)
		suite.client.RequireResourceNotFound(suite.T(), cdID)

		suite.client.RequireStatusCondition(suite.T(), dest.Id, ControllerName,
			ConditionMeshProtocolDestinationPort(resource.ReferenceToString(suite.destService3Ref), "tcp"))
	})
}

func (suite *controllerTestSuite) TestReconcile_StatusUpdate_ComputedRoutesNotFound() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		dest := resourcetest.Resource(pbmesh.DestinationsType, "dest").
			WithTenancy(tenancy).
			WithData(suite.T(), suite.dest2).
			Write(suite.T(), suite.client)
		_, err := suite.ctl.mapper.MapDestinations(suite.ctx, suite.runtime, dest)
		require.NoError(suite.T(), err)

		resourcetest.Resource(pbcatalog.ServiceType, suite.destService3Ref.Name).
			WithTenancy(tenancy).
			WithData(suite.T(), &pbcatalog.Service{
				Workloads: &pbcatalog.WorkloadSelector{Names: []string{suite.workloadRes.Id.Name}},
				Ports: []*pbcatalog.ServicePort{
					{
						TargetPort: "tcp",
						Protocol:   pbcatalog.Protocol_PROTOCOL_TCP,
					},
					{
						TargetPort: "mesh",
						Protocol:   pbcatalog.Protocol_PROTOCOL_MESH,
					},
				},
			}).
			Write(suite.T(), suite.client)

		cdID := resourcetest.Resource(pbmesh.ComputedExplicitDestinationsType, suite.workloadRes.Id.Name).
			WithTenancy(tenancy).
			Write(suite.T(), suite.client).Id

		err = suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
			ID: cdID,
		})
		require.NoError(suite.T(), err)
		suite.client.RequireResourceNotFound(suite.T(), cdID)

		suite.client.RequireStatusCondition(suite.T(), dest.Id, ControllerName,
			ConditionDestinationComputedRoutesNotFound(resource.ReferenceToString(suite.destService3Ref)))
	})
}

func (suite *controllerTestSuite) TestReconcile_StatusUpdate_ComputedRoutesPortNotFound() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		dest := resourcetest.Resource(pbmesh.DestinationsType, "dest").
			WithTenancy(tenancy).
			WithData(suite.T(), suite.dest2).
			Write(suite.T(), suite.client)
		_, err := suite.ctl.mapper.MapDestinations(suite.ctx, suite.runtime, dest)
		require.NoError(suite.T(), err)

		destService := resourcetest.Resource(pbcatalog.ServiceType, suite.destService3Ref.Name).
			WithTenancy(tenancy).
			WithData(suite.T(), &pbcatalog.Service{
				Workloads: &pbcatalog.WorkloadSelector{Names: []string{suite.workloadRes.Id.Name}},
				Ports: []*pbcatalog.ServicePort{
					{
						TargetPort: "tcp",
						Protocol:   pbcatalog.Protocol_PROTOCOL_TCP,
					},
					{
						TargetPort: "mesh",
						Protocol:   pbcatalog.Protocol_PROTOCOL_MESH,
					},
				},
			}).
			Write(suite.T(), suite.client)

		resourcetest.Resource(pbmesh.ComputedRoutesType, destService.Id.Name).
			WithTenancy(tenancy).
			WithData(suite.T(), &pbmesh.ComputedRoutes{
				PortedConfigs: map[string]*pbmesh.ComputedPortRoutes{
					"some-other-port": {
						Protocol: pbcatalog.Protocol_PROTOCOL_HTTP,
						Config: &pbmesh.ComputedPortRoutes_Http{
							Http: &pbmesh.ComputedHTTPRoute{},
						},
					},
				},
			}).
			Write(suite.T(), suite.client)

		cdID := resourcetest.Resource(pbmesh.ComputedExplicitDestinationsType, suite.workloadRes.Id.Name).
			WithTenancy(tenancy).
			Write(suite.T(), suite.client).Id

		err = suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
			ID: cdID,
		})
		require.NoError(suite.T(), err)
		suite.client.RequireResourceNotFound(suite.T(), cdID)

		suite.client.RequireStatusCondition(suite.T(), dest.Id, ControllerName,
			ConditionDestinationComputedRoutesPortNotFound(resource.ReferenceToString(suite.destService3Ref), "tcp"))
	})
}

func (suite *controllerTestSuite) TestController() {
	mgr := controller.NewManager(suite.client, suite.runtime.Logger)

	m := mapper.New()
	mgr.Register(Controller(m))
	mgr.SetRaftLeader(true)
	go mgr.Run(suite.ctx)

	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		cdID := resource.ReplaceType(pbmesh.ComputedExplicitDestinationsType, suite.workloadRes.Id)

		dest1 := resourcetest.Resource(pbmesh.DestinationsType, "dest1").
			WithTenancy(tenancy).
			WithData(suite.T(), suite.dest1).
			Write(suite.T(), suite.client)

		// At this point, none of the services or routes yet exist and so we should see the status of the destinations
		// resource to reflect that. The CED resource should not be created in this case.
		testutil.RunStep(suite.T(), "check that destinations status is updated", func(t *testing.T) {
			retry.Run(t, func(r *retry.R) {
				serviceRef := resource.IDToString(suite.destService1.Id)
				suite.client.WaitForStatusCondition(r, dest1.Id, ControllerName, ConditionDestinationServiceNotFound(serviceRef))

				suite.client.RequireResourceNotFound(r, cdID)
			})
		})

		dest2 := resourcetest.Resource(pbmesh.DestinationsType, "dest2").
			WithTenancy(tenancy).
			WithData(suite.T(), suite.dest2).
			Write(suite.T(), suite.client)

		suite.writeServices(suite.T(), tenancy)

		// After we write services, we expect another reconciliation to be kicked off to validate and find that there are no computed routes.
		testutil.RunStep(suite.T(), "check that destinations status says that there are no computed routes", func(t *testing.T) {
			retry.Run(t, func(r *retry.R) {
				suite.client.WaitForStatusCondition(r, dest1.Id, ControllerName,
					ConditionDestinationComputedRoutesNotFound(resource.IDToString(suite.destService1.Id)))
				suite.client.WaitForStatusCondition(r, dest2.Id, ControllerName,
					ConditionDestinationComputedRoutesNotFound(resource.IDToString(suite.destService3.Id)))

				suite.client.RequireResourceNotFound(r, cdID)
			})
		})

		// Now write computed routes to get a computed resource.
		suite.writeComputedRoutes(suite.T(), tenancy)

		testutil.RunStep(suite.T(), "computed destinations generation", func(t *testing.T) {
			retry.Run(t, func(r *retry.R) {
				suite.client.RequireResourceExists(r, cdID)
				suite.requireComputedDestinations(r, cdID)
			})
		})

		testutil.RunStep(suite.T(), "add another workload", func(t *testing.T) {
			// Create another workload that will match only dest2.
			matchingWorkload := resourcetest.Resource(pbcatalog.WorkloadType, "test-extra-workload").
				WithTenancy(tenancy).
				WithData(t, suite.workload).
				Write(t, suite.client)
			matchingWorkloadCDID := resource.ReplaceType(pbmesh.ComputedExplicitDestinationsType, matchingWorkload.Id)

			retry.Run(t, func(r *retry.R) {
				suite.client.RequireResourceExists(r, cdID)
				suite.requireComputedDestinations(r, cdID)

				matchingWorkloadCD := suite.client.RequireResourceExists(r, matchingWorkloadCDID)
				dec := resourcetest.MustDecode[*pbmesh.ComputedExplicitDestinations](r, matchingWorkloadCD)
				prototest.AssertDeepEqual(r, suite.dest2.GetDestinations(), dec.GetData().GetDestinations())
			})
		})

		testutil.RunStep(suite.T(), "update workload selector", func(t *testing.T) {
			// Update workload selector to no point to some non-existing workload
			updatedDestinations := proto.Clone(suite.dest2).(*pbmesh.Destinations)
			updatedDestinations.Workloads = &pbcatalog.WorkloadSelector{
				Names: []string{"other-workload"},
			}

			matchingWorkload := resourcetest.Resource(pbcatalog.WorkloadType, "other-workload").
				WithData(t, suite.workload).
				WithTenancy(tenancy).
				Write(t, suite.client)
			matchingWorkloadCDID := resource.ReplaceType(pbmesh.ComputedExplicitDestinationsType, matchingWorkload.Id)
			resourcetest.Resource(pbmesh.DestinationsType, "dest2").
				WithTenancy(tenancy).
				WithData(suite.T(), updatedDestinations).
				Write(suite.T(), suite.client)

			retry.Run(t, func(r *retry.R) {
				res := suite.client.RequireResourceExists(r, cdID)

				// The "test-workload" computed destinations should now be updated to use only proxy dest1.
				expDest := &pbmesh.ComputedExplicitDestinations{
					Destinations: suite.dest1.Destinations,
				}
				dec := resourcetest.MustDecode[*pbmesh.ComputedExplicitDestinations](r, res)
				prototest.AssertDeepEqual(r, expDest.GetDestinations(), dec.GetData().GetDestinations())

				matchingWorkloadCD := suite.client.RequireResourceExists(r, matchingWorkloadCDID)
				dec = resourcetest.MustDecode[*pbmesh.ComputedExplicitDestinations](r, matchingWorkloadCD)
				prototest.AssertDeepEqual(r, suite.dest2.GetDestinations(), dec.GetData().GetDestinations())
			})
		})

		// Delete all destinations.
		suite.client.MustDelete(suite.T(), dest1.Id)
		suite.client.MustDelete(suite.T(), dest2.Id)

		testutil.RunStep(suite.T(), "all destinations are deleted", func(t *testing.T) {
			retry.Run(t, func(r *retry.R) {
				suite.client.RequireResourceNotFound(r, cdID)
			})
		})
	})
}

func TestControllerSuite(t *testing.T) {
	suite.Run(t, new(controllerTestSuite))
}

func (suite *controllerTestSuite) requireComputedDestinations(t resourcetest.T, id *pbresource.ID) {
	cdRes := suite.client.RequireResourceExists(t, id)
	decCD := resourcetest.MustDecode[*pbmesh.ComputedExplicitDestinations](t, cdRes)
	prototest.AssertElementsMatch(t, suite.expComputedDest.GetDestinations(), decCD.Data.GetDestinations())
	resourcetest.RequireOwner(t, cdRes, resource.ReplaceType(pbcatalog.WorkloadType, id), true)
}

func (suite *controllerTestSuite) appendTenancyInfo(tenancy *pbresource.Tenancy) string {
	return fmt.Sprintf("%s_Namespace_%s_Partition", tenancy.Namespace, tenancy.Partition)
}

func (suite *controllerTestSuite) runTestCaseWithTenancies(t func(*pbresource.Tenancy)) {
	for _, tenancy := range suite.tenancies {
		suite.Run(suite.appendTenancyInfo(tenancy), func() {
			suite.setupWithTenancy(tenancy)
			t(tenancy)
		})
	}
}
