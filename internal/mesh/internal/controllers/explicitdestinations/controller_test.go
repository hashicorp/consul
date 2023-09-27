// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package explicitdestinations

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/proto"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/routes/routestest"
	"github.com/hashicorp/consul/internal/mesh/internal/mappers/workloadselectionmapper"
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

	expComputedDest *pbmesh.ComputedDestinations
}

func (suite *controllerTestSuite) SetupTest() {
	resourceClient := svctest.RunResourceService(suite.T(), types.Register, catalog.RegisterTypes)
	suite.client = resourcetest.NewClient(resourceClient)
	suite.runtime = controller.Runtime{Client: resourceClient, Logger: testutil.Logger(suite.T())}
	suite.ctx = testutil.TestContext(suite.T())

	suite.ctl = &reconciler{
		destinations: workloadselectionmapper.New[*pbmesh.Destinations](pbmesh.ComputedDestinationsType),
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
		Write(suite.T(), suite.client)

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
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(suite.T(), suite.serviceData).
		Build()
	suite.destService2 = resourcetest.Resource(pbcatalog.ServiceType, "dest-service-2").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(suite.T(), suite.serviceData).
		Build()
	suite.destService3 = resourcetest.Resource(pbcatalog.ServiceType, "dest-service-3").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(suite.T(), suite.serviceData).
		Build()

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

	suite.expComputedDest = &pbmesh.ComputedDestinations{
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
	id := resourcetest.Resource(pbmesh.ComputedProxyConfigurationType, "not-found").ID()
	err := suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
		ID: id,
	})
	require.NoError(suite.T(), err)

	suite.client.RequireResourceNotFound(suite.T(), id)
}

func (suite *controllerTestSuite) TestReconcile_NonMeshWorkload() {
	resourcetest.Resource(pbcatalog.WorkloadType, "non-mesh").
		WithData(suite.T(), &pbcatalog.Workload{
			Addresses: []*pbcatalog.WorkloadAddress{{Host: "1.1.1.1"}},
			Ports: map[string]*pbcatalog.WorkloadPort{
				"tcp": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
			},
		}).
		Write(suite.T(), suite.client)

	cdID := resourcetest.Resource(pbmesh.ComputedDestinationsType, "non-mesh").
		Write(suite.T(), suite.client).Id

	err := suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
		ID: cdID,
	})
	require.NoError(suite.T(), err)

	suite.client.RequireResourceNotFound(suite.T(), cdID)
}

func (suite *controllerTestSuite) writeServicesAndComputedRoutes(t *testing.T) {
	// Write all services.
	resourcetest.Resource(pbcatalog.ServiceType, suite.destService1Ref.Name).
		WithData(t, suite.serviceData).
		Write(t, suite.client)
	resourcetest.Resource(pbcatalog.ServiceType, suite.destService2Ref.Name).
		WithData(t, suite.serviceData).
		Write(t, suite.client)
	resourcetest.Resource(pbcatalog.ServiceType, suite.destService3Ref.Name).
		WithData(t, suite.serviceData).
		Write(t, suite.client)

	// Write computed routes
	resourcetest.Resource(pbmesh.ComputedRoutesType, suite.destService1Ref.Name).
		WithData(t, suite.destService1Routes).
		Write(t, suite.client)
	resourcetest.Resource(pbmesh.ComputedRoutesType, suite.destService2Ref.Name).
		WithData(t, suite.destService2Routes).
		Write(t, suite.client)
	resourcetest.Resource(pbmesh.ComputedRoutesType, suite.destService3Ref.Name).
		WithData(t, suite.destService3Routes).
		Write(t, suite.client)
}

func (suite *controllerTestSuite) TestReconcile_HappyPath() {
	d1 := resourcetest.Resource(pbmesh.DestinationsType, "dest1").
		WithData(suite.T(), suite.dest1).
		Write(suite.T(), suite.client)
	_, err := suite.ctl.destinations.MapToComputedType(suite.ctx, suite.runtime, d1)
	require.NoError(suite.T(), err)

	d2 := resourcetest.Resource(pbmesh.DestinationsType, "dest2").
		WithData(suite.T(), suite.dest2).
		Write(suite.T(), suite.client)
	_, err = suite.ctl.destinations.MapToComputedType(suite.ctx, suite.runtime, d2)
	require.NoError(suite.T(), err)

	suite.writeServicesAndComputedRoutes(suite.T())

	cdID := resource.ReplaceType(pbmesh.ComputedDestinationsType, suite.workloadRes.Id)
	err = suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
		ID: cdID,
	})

	require.NoError(suite.T(), err)

	suite.requireComputedDestinations(suite.T(), cdID)
	suite.client.RequireStatusCondition(suite.T(), d1.Id, ControllerName, ConditionDestinationsAccepted())
}

func (suite *controllerTestSuite) TestReconcile_NoDestinations() {
	dest := resourcetest.Resource(pbmesh.DestinationsType, "dest").
		WithData(suite.T(), suite.dest1).
		Build()
	_, err := suite.ctl.destinations.MapToComputedType(suite.ctx, suite.runtime, dest)
	require.NoError(suite.T(), err)

	cdID := resourcetest.Resource(pbmesh.ComputedDestinationsType, suite.workloadRes.Id.Name).
		Write(suite.T(), suite.client).Id
	err = suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
		ID: cdID,
	})

	require.NoError(suite.T(), err)

	suite.client.RequireResourceNotFound(suite.T(), cdID)
}

func (suite *controllerTestSuite) TestReconcile_AllDestinationsInvalid() {
	// We add a destination with services refs that don't exist which should result
	// in computed destinations being deleted because all destinations are invalid.
	dest := resourcetest.Resource(pbmesh.DestinationsType, "dest").
		WithData(suite.T(), suite.dest1).
		Write(suite.T(), suite.client)
	_, err := suite.ctl.destinations.MapToComputedType(suite.ctx, suite.runtime, dest)
	require.NoError(suite.T(), err)

	cdID := resourcetest.Resource(pbmesh.ComputedDestinationsType, suite.workloadRes.Id.Name).
		Write(suite.T(), suite.client).Id
	err = suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
		ID: cdID,
	})

	require.NoError(suite.T(), err)

	suite.client.RequireResourceNotFound(suite.T(), cdID)
}

func (suite *controllerTestSuite) TestReconcile_StatusUpdate_NoService() {
	dest := resourcetest.Resource(pbmesh.DestinationsType, "dest").
		WithData(suite.T(), suite.dest2).
		Write(suite.T(), suite.client)
	_, err := suite.ctl.destinations.MapToComputedType(suite.ctx, suite.runtime, dest)
	require.NoError(suite.T(), err)
	cdID := resourcetest.Resource(pbmesh.ComputedDestinationsType, suite.workloadRes.Id.Name).
		Write(suite.T(), suite.client).Id
	err = suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
		ID: cdID,
	})
	require.NoError(suite.T(), err)
	suite.client.RequireResourceNotFound(suite.T(), cdID)

	suite.client.RequireStatusCondition(suite.T(), dest.Id, ControllerName,
		ConditionDestinationServiceNotFound(resource.ReferenceToString(suite.destService3Ref)))
}

func (suite *controllerTestSuite) TestReconcile_StatusUpdate_ServiceNotOnMesh() {
	dest := resourcetest.Resource(pbmesh.DestinationsType, "dest").
		WithData(suite.T(), suite.dest2).
		Write(suite.T(), suite.client)
	_, err := suite.ctl.destinations.MapToComputedType(suite.ctx, suite.runtime, dest)
	require.NoError(suite.T(), err)

	resourcetest.Resource(pbcatalog.ServiceType, suite.destService3Ref.Name).
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

	cdID := resourcetest.Resource(pbmesh.ComputedDestinationsType, suite.workloadRes.Id.Name).
		Write(suite.T(), suite.client).Id

	err = suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
		ID: cdID,
	})
	require.NoError(suite.T(), err)
	suite.client.RequireResourceNotFound(suite.T(), cdID)

	suite.client.RequireStatusCondition(suite.T(), dest.Id, ControllerName,
		ConditionMeshProtocolNotFound(resource.ReferenceToString(suite.destService3Ref)))
}

func (suite *controllerTestSuite) TestReconcile_StatusUpdate_DestinationPortIsMesh() {
	dest := resourcetest.Resource(pbmesh.DestinationsType, "dest").
		WithData(suite.T(), suite.dest2).
		Write(suite.T(), suite.client)
	_, err := suite.ctl.destinations.MapToComputedType(suite.ctx, suite.runtime, dest)
	require.NoError(suite.T(), err)

	resourcetest.Resource(pbcatalog.ServiceType, suite.destService3Ref.Name).
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

	cdID := resourcetest.Resource(pbmesh.ComputedDestinationsType, suite.workloadRes.Id.Name).
		Write(suite.T(), suite.client).Id

	err = suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
		ID: cdID,
	})
	require.NoError(suite.T(), err)
	suite.client.RequireResourceNotFound(suite.T(), cdID)

	suite.client.RequireStatusCondition(suite.T(), dest.Id, ControllerName,
		ConditionMeshProtocolDestinationPort(resource.ReferenceToString(suite.destService3Ref), "tcp"))
}

func (suite *controllerTestSuite) TestReconcile_StatusUpdate_ComputedRoutesNotFound() {
	dest := resourcetest.Resource(pbmesh.DestinationsType, "dest").
		WithData(suite.T(), suite.dest2).
		Write(suite.T(), suite.client)
	_, err := suite.ctl.destinations.MapToComputedType(suite.ctx, suite.runtime, dest)
	require.NoError(suite.T(), err)

	resourcetest.Resource(pbcatalog.ServiceType, suite.destService3Ref.Name).
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

	cdID := resourcetest.Resource(pbmesh.ComputedDestinationsType, suite.workloadRes.Id.Name).
		Write(suite.T(), suite.client).Id

	err = suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
		ID: cdID,
	})
	require.NoError(suite.T(), err)
	suite.client.RequireResourceNotFound(suite.T(), cdID)

	suite.client.RequireStatusCondition(suite.T(), dest.Id, ControllerName,
		ConditionDestinationComputedRoutesNotFound(resource.ReferenceToString(suite.destService3Ref)))
}

func (suite *controllerTestSuite) TestReconcile_StatusUpdate_ComputedRoutesPortNotFound() {
	dest := resourcetest.Resource(pbmesh.DestinationsType, "dest").
		WithData(suite.T(), suite.dest2).
		Write(suite.T(), suite.client)
	_, err := suite.ctl.destinations.MapToComputedType(suite.ctx, suite.runtime, dest)
	require.NoError(suite.T(), err)

	destService := resourcetest.Resource(pbcatalog.ServiceType, suite.destService3Ref.Name).
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

	cdID := resourcetest.Resource(pbmesh.ComputedDestinationsType, suite.workloadRes.Id.Name).
		Write(suite.T(), suite.client).Id

	err = suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
		ID: cdID,
	})
	require.NoError(suite.T(), err)
	suite.client.RequireResourceNotFound(suite.T(), cdID)

	suite.client.RequireStatusCondition(suite.T(), dest.Id, ControllerName,
		ConditionDestinationComputedRoutesPortNotFound(resource.ReferenceToString(suite.destService3Ref), "tcp"))
}

func (suite *controllerTestSuite) TestController() {
	mgr := controller.NewManager(suite.client, suite.runtime.Logger)

	m := workloadselectionmapper.New[*pbmesh.Destinations](pbmesh.ComputedDestinationsType)
	mgr.Register(Controller(m))
	mgr.SetRaftLeader(true)
	go mgr.Run(suite.ctx)

	dest1 := resourcetest.Resource(pbmesh.DestinationsType, "dest1").
		WithData(suite.T(), suite.dest1).
		Write(suite.T(), suite.client)

	dest2 := resourcetest.Resource(pbmesh.DestinationsType, "dest2").
		WithData(suite.T(), suite.dest2).
		Write(suite.T(), suite.client)

	suite.writeServicesAndComputedRoutes(suite.T())

	cdID := resource.ReplaceType(pbmesh.ComputedDestinationsType, suite.workloadRes.Id)
	testutil.RunStep(suite.T(), "computed destinations generation", func(t *testing.T) {
		retry.Run(t, func(r *retry.R) {
			suite.client.RequireResourceExists(r, cdID)
			suite.requireComputedDestinations(r, cdID)
		})
	})

	testutil.RunStep(suite.T(), "add another workload", func(t *testing.T) {
		// Create another workload that will match only dest2.
		matchingWorkload := resourcetest.Resource(pbcatalog.WorkloadType, "test-extra-workload").
			WithData(t, suite.workload).
			Write(t, suite.client)
		matchingWorkloadCDID := resource.ReplaceType(pbmesh.ComputedDestinationsType, matchingWorkload.Id)

		retry.Run(t, func(r *retry.R) {
			suite.client.RequireResourceExists(r, cdID)
			suite.requireComputedDestinations(r, cdID)

			matchingWorkloadCD := suite.client.RequireResourceExists(r, matchingWorkloadCDID)
			dec := resourcetest.MustDecode[*pbmesh.ComputedDestinations](r, matchingWorkloadCD)
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
			Write(t, suite.client)
		matchingWorkloadCDID := resource.ReplaceType(pbmesh.ComputedDestinationsType, matchingWorkload.Id)
		resourcetest.Resource(pbmesh.DestinationsType, "dest2").
			WithData(suite.T(), updatedDestinations).
			Write(suite.T(), suite.client)

		retry.Run(t, func(r *retry.R) {
			res := suite.client.RequireResourceExists(r, cdID)

			// The "test-workload" computed destinations should now be updated to use only proxy dest1.
			expDest := &pbmesh.ComputedDestinations{
				Destinations: suite.dest1.Destinations,
			}
			dec := resourcetest.MustDecode[*pbmesh.ComputedDestinations](t, res)
			prototest.AssertDeepEqual(r, expDest.GetDestinations(), dec.GetData().GetDestinations())

			matchingWorkloadCD := suite.client.RequireResourceExists(r, matchingWorkloadCDID)
			dec = resourcetest.MustDecode[*pbmesh.ComputedDestinations](r, matchingWorkloadCD)
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
}

func TestControllerSuite(t *testing.T) {
	suite.Run(t, new(controllerTestSuite))
}

func (suite *controllerTestSuite) requireComputedDestinations(t resourcetest.T, id *pbresource.ID) {
	cdRes := suite.client.RequireResourceExists(t, id)
	decCD := resourcetest.MustDecode[*pbmesh.ComputedDestinations](t, cdRes)
	prototest.AssertElementsMatch(t, suite.expComputedDest.GetDestinations(), decCD.Data.GetDestinations())
	resourcetest.RequireOwner(t, cdRes, resource.ReplaceType(pbcatalog.WorkloadType, id), true)
}
