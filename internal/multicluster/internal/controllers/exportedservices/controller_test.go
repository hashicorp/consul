// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package exportedservices

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/multicluster/internal/controllers/exportedservices/expander"
	"github.com/hashicorp/consul/internal/multicluster/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v2"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/version/versiontest"
)

type controllerSuite struct {
	suite.Suite
	ctx                   context.Context
	client                *rtest.Client
	rt                    controller.Runtime
	isEnterprise          bool
	reconciler            *reconciler
	samenessGroupExpander ExportedServicesSamenessGroupExpander
	tenancies             []*pbresource.Tenancy
}

func (suite *controllerSuite) SetupTest() {
	suite.ctx = testutil.TestContext(suite.T())
	suite.tenancies = rtest.TestTenancies()
	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(types.Register, catalog.RegisterTypes).
		WithTenancies(suite.tenancies...).
		WithTenancies(rtest.Tenancy("default.app"), rtest.Tenancy("foo.app")).
		Run(suite.T())

	suite.samenessGroupExpander = expander.New()

	suite.client = rtest.NewClient(client)
	suite.rt = controller.Runtime{
		Client: suite.client,
		Logger: testutil.Logger(suite.T()),
	}
	suite.reconciler = &reconciler{
		samenessGroupExpander: suite.samenessGroupExpander,
	}
	suite.isEnterprise = versiontest.IsEnterprise()
}

func TestController(t *testing.T) {
	suite.Run(t, new(controllerSuite))
}

func (suite *controllerSuite) TestReconcile_DeleteOldCES_NoExportedServices() {
	// This test's purpose is to ensure that we delete the
	// already existing CES when no exported service resources
	// are found.

	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		oldCESData := &pbmulticluster.ComputedExportedServices{
			Services: []*pbmulticluster.ComputedExportedService{
				{
					TargetRef: &pbresource.Reference{
						Type:    pbcatalog.ServiceType,
						Tenancy: tenancy,
						Name:    "svc0",
					},
					Consumers: []*pbmulticluster.ComputedExportedServiceConsumer{
						suite.constructConsumer("test-peer", "peer"),
					},
				},
			},
		}

		if suite.isEnterprise {
			oldCESData.Services[0].Consumers = append(oldCESData.Services[0].Consumers, suite.constructConsumer("part-n", "partition"))
		}

		oldCES := rtest.Resource(pbmulticluster.ComputedExportedServicesType, "global").
			WithData(suite.T(), oldCESData).
			WithTenancy(&pbresource.Tenancy{Partition: tenancy.Partition}).
			Write(suite.T(), suite.client)
		require.NotNil(suite.T(), oldCES)

		err := suite.reconciler.Reconcile(suite.ctx, suite.rt, controller.Request{ID: oldCES.Id})
		require.NoError(suite.T(), err)

		suite.client.RequireResourceNotFound(suite.T(), oldCES.Id)
	})
}

func (suite *controllerSuite) TestReconcile_DeleteOldCES_NoMatchingServices() {
	// This test's purpose is to ensure that we delete the
	// already existing CES when the exported services configs
	// don't cover any services present in the partition.

	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		oldCESData := &pbmulticluster.ComputedExportedServices{
			Services: []*pbmulticluster.ComputedExportedService{
				{
					TargetRef: &pbresource.Reference{
						Type:    pbcatalog.ServiceType,
						Tenancy: tenancy,
						Name:    "svc0",
					},
					Consumers: []*pbmulticluster.ComputedExportedServiceConsumer{
						suite.constructConsumer("test-peer", "peer"),
					},
				},
			},
		}

		if suite.isEnterprise {
			oldCESData.Services[0].Consumers = append(oldCESData.Services[0].Consumers, suite.constructConsumer("part-n", "partition"))
		}

		oldCES := rtest.Resource(pbmulticluster.ComputedExportedServicesType, "global").
			WithData(suite.T(), oldCESData).
			WithTenancy(&pbresource.Tenancy{Partition: tenancy.Partition}).
			Write(suite.T(), suite.client)
		require.NotNil(suite.T(), oldCES)

		exportedSvcData := &pbmulticluster.ExportedServices{
			Services: []string{"random-service-1", "random-service-2"},
			Consumers: []*pbmulticluster.ExportedServicesConsumer{
				{ConsumerTenancy: &pbmulticluster.ExportedServicesConsumer_Peer{Peer: "peer-1"}},
			},
		}
		if suite.isEnterprise {
			exportedSvcData.Consumers = append(exportedSvcData.Consumers, &pbmulticluster.ExportedServicesConsumer{
				ConsumerTenancy: &pbmulticluster.ExportedServicesConsumer_Partition{
					Partition: "part-n",
				},
			})
		}
		suite.writeExportedService("exported-svcs", tenancy, exportedSvcData)

		if suite.isEnterprise {
			nesData := &pbmulticluster.NamespaceExportedServices{
				Consumers: []*pbmulticluster.ExportedServicesConsumer{
					{ConsumerTenancy: &pbmulticluster.ExportedServicesConsumer_Partition{Partition: "part-n"}},
					{ConsumerTenancy: &pbmulticluster.ExportedServicesConsumer_Peer{Peer: "peer-2"}},
				},
			}
			suite.writeNamespaceExportedService("nes", tenancy, nesData)
		}

		err := suite.reconciler.Reconcile(suite.ctx, suite.rt, controller.Request{ID: oldCES.Id})
		require.NoError(suite.T(), err)

		suite.client.RequireResourceNotFound(suite.T(), oldCES.Id)
	})
}

func (suite *controllerSuite) TestReconcile_SkipWritingNewCES() {
	// This test's purpose is to ensure that we skip
	// writing the new CES when there are no changes to
	// the existing one

	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		oldCESData := &pbmulticluster.ComputedExportedServices{
			Services: []*pbmulticluster.ComputedExportedService{
				{
					TargetRef: &pbresource.Reference{
						Type:    pbcatalog.ServiceType,
						Tenancy: tenancy,
						Name:    "svc-0",
					},
					Consumers: []*pbmulticluster.ComputedExportedServiceConsumer{
						suite.constructConsumer("peer-1", "peer"),
					},
				},
			},
		}

		if suite.isEnterprise {
			oldCESData.Services[0].Consumers = append(oldCESData.Services[0].Consumers, suite.constructConsumer("part-n", "partition"))
		}

		oldStatus := &pbresource.Status{
			Conditions: []*pbresource.Condition{
				conditionComputed(),
			},
		}
		oldCES := rtest.Resource(pbmulticluster.ComputedExportedServicesType, "global").
			WithData(suite.T(), oldCESData).
			WithTenancy(&pbresource.Tenancy{Partition: tenancy.Partition}).
			WithStatus(statusKey, oldStatus).
			Write(suite.T(), suite.client)
		require.NotNil(suite.T(), oldCES)

		// Export the svc-0 service to just a peer
		exportedSvcData := &pbmulticluster.ExportedServices{
			Services: []string{"svc-0"},
			Consumers: []*pbmulticluster.ExportedServicesConsumer{
				{ConsumerTenancy: &pbmulticluster.ExportedServicesConsumer_Peer{Peer: "peer-1"}},
			},
		}
		_ = rtest.Resource(pbmulticluster.ExportedServicesType, "exported-svcs").
			WithData(suite.T(), exportedSvcData).
			WithTenancy(tenancy).
			Write(suite.T(), suite.client)

		if suite.isEnterprise {
			// Export all services in a given partition to `part-n` partition
			pesData := &pbmulticluster.PartitionExportedServices{
				Consumers: []*pbmulticluster.ExportedServicesConsumer{
					{ConsumerTenancy: &pbmulticluster.ExportedServicesConsumer_Partition{Partition: "part-n"}},
				},
			}
			_ = rtest.Resource(pbmulticluster.PartitionExportedServicesType, "pes").
				WithData(suite.T(), pesData).
				WithTenancy(&pbresource.Tenancy{Partition: tenancy.Partition}).
				Write(suite.T(), suite.client)
		}

		svcData := &pbcatalog.Service{
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			},
		}
		_ = rtest.Resource(pbcatalog.ServiceType, "svc-0").
			WithData(suite.T(), svcData).
			WithTenancy(tenancy).
			Write(suite.T(), suite.client)

		err := suite.reconciler.Reconcile(suite.ctx, suite.rt, controller.Request{ID: oldCES.Id})
		require.NoError(suite.T(), err)

		// Checking no-op with version
		suite.client.RequireVersionUnchanged(suite.T(), oldCES.Id, oldCES.Version)
	})
}

func (suite *controllerSuite) TestReconcile_SkipWritingNewCES_WithStatusUpdate() {
	// This test's purpose is to ensure that we skip
	// writing the new CES when there are no changes to
	// the existing one but write the status if there
	// is a mismatch

	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		oldCESData := &pbmulticluster.ComputedExportedServices{
			Services: []*pbmulticluster.ComputedExportedService{
				{
					TargetRef: &pbresource.Reference{
						Type:    pbcatalog.ServiceType,
						Tenancy: tenancy,
						Name:    "svc-0",
					},
					Consumers: []*pbmulticluster.ComputedExportedServiceConsumer{
						suite.constructConsumer("peer-1", "peer"),
					},
				},
			},
		}

		if suite.isEnterprise {
			oldCESData.Services[0].Consumers = append(oldCESData.Services[0].Consumers, suite.constructConsumer("part-n", "partition"))
		}

		oldCES := rtest.Resource(pbmulticluster.ComputedExportedServicesType, "global").
			WithData(suite.T(), oldCESData).
			WithTenancy(&pbresource.Tenancy{Partition: tenancy.Partition}).
			Write(suite.T(), suite.client)
		require.NotNil(suite.T(), oldCES)

		// Export the svc-0 service to just a peer
		exportedSvcData := &pbmulticluster.ExportedServices{
			Services: []string{"svc-0"},
			Consumers: []*pbmulticluster.ExportedServicesConsumer{
				{ConsumerTenancy: &pbmulticluster.ExportedServicesConsumer_Peer{Peer: "peer-1"}},
			},
		}
		_ = rtest.Resource(pbmulticluster.ExportedServicesType, "exported-svcs").
			WithData(suite.T(), exportedSvcData).
			WithTenancy(tenancy).
			Write(suite.T(), suite.client)

		if suite.isEnterprise {
			// Export all services in a given partition to `part-n` partition
			pesData := &pbmulticluster.PartitionExportedServices{
				Consumers: []*pbmulticluster.ExportedServicesConsumer{
					{ConsumerTenancy: &pbmulticluster.ExportedServicesConsumer_Partition{Partition: "part-n"}},
				},
			}
			_ = rtest.Resource(pbmulticluster.PartitionExportedServicesType, "pes").
				WithData(suite.T(), pesData).
				WithTenancy(&pbresource.Tenancy{Partition: tenancy.Partition}).
				Write(suite.T(), suite.client)
		}

		svcData := &pbcatalog.Service{
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			},
		}
		_ = rtest.Resource(pbcatalog.ServiceType, "svc-0").
			WithData(suite.T(), svcData).
			WithTenancy(tenancy).
			Write(suite.T(), suite.client)

		passThroughClient := newPassThroughResourceClient(suite.client)
		rt := suite.controllerRuntimeWithPassThroughClient(passThroughClient)
		err := suite.reconciler.Reconcile(suite.ctx, rt, controller.Request{ID: oldCES.Id})
		require.NoError(suite.T(), err)

		// Checking version change to ensure that the status gets updated
		newCES := suite.client.RequireVersionChanged(suite.T(), oldCES.Id, oldCES.Version)
		rtest.RequireStatusCondition(suite.T(), newCES, statusKey, conditionComputed())
		require.Equal(suite.T(), 0, passThroughClient.writesCount)
	})
}

func (suite *controllerSuite) TestReconcile_ComputeCES() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		suite.writeService("svc-0", tenancy)

		if suite.isEnterprise {
			suite.writeService("svc-1", tenancy)
			suite.writeService("svc-2", tenancy)
		}

		// Export the svc-0 service to just peers
		exportedSvcData := &pbmulticluster.ExportedServices{
			Services: []string{"svc-0"},
			Consumers: []*pbmulticluster.ExportedServicesConsumer{
				{ConsumerTenancy: &pbmulticluster.ExportedServicesConsumer_Peer{Peer: "peer-1"}},
				{ConsumerTenancy: &pbmulticluster.ExportedServicesConsumer_Peer{Peer: "peer-2"}},
			},
		}
		suite.writeExportedService("exp-svc-0", tenancy, exportedSvcData)

		if suite.isEnterprise {
			nesData := &pbmulticluster.NamespaceExportedServices{
				Consumers: []*pbmulticluster.ExportedServicesConsumer{
					{ConsumerTenancy: &pbmulticluster.ExportedServicesConsumer_Peer{Peer: "peer-2"}},
					{ConsumerTenancy: &pbmulticluster.ExportedServicesConsumer_Partition{Partition: "part-1"}},
				},
			}
			suite.writeNamespaceExportedService("nes", tenancy, nesData)

			// Export all services in a given partition to `part-n` partition
			pesData := &pbmulticluster.PartitionExportedServices{
				Consumers: []*pbmulticluster.ExportedServicesConsumer{
					{ConsumerTenancy: &pbmulticluster.ExportedServicesConsumer_Partition{Partition: "part-n"}},
				},
			}
			suite.writePartitionedExportedService("pes", tenancy, pesData)
		}

		id := rtest.Resource(pbmulticluster.ComputedExportedServicesType, "global").WithTenancy(&pbresource.Tenancy{Partition: tenancy.Partition}).ID()
		require.NotNil(suite.T(), id)

		err := suite.reconciler.Reconcile(suite.ctx, suite.rt, controller.Request{ID: id})
		require.NoError(suite.T(), err)

		res := suite.client.RequireResourceExists(suite.T(), id)
		computedCES := suite.getComputedExportedSvcData(res)
		rtest.RequireStatusCondition(suite.T(), res, statusKey, conditionComputed())

		var expectedCES *pbmulticluster.ComputedExportedServices
		if suite.isEnterprise {
			expectedCES = &pbmulticluster.ComputedExportedServices{
				Services: []*pbmulticluster.ComputedExportedService{
					{
						TargetRef: &pbresource.Reference{
							Type:    pbcatalog.ServiceType,
							Tenancy: tenancy,
							Name:    "svc-0",
						},
						Consumers: []*pbmulticluster.ComputedExportedServiceConsumer{
							suite.constructConsumer("peer-1", "peer"),
							suite.constructConsumer("peer-2", "peer"),
							suite.constructConsumer("part-1", "partition"),
							suite.constructConsumer("part-n", "partition"),
						},
					},
					{
						TargetRef: &pbresource.Reference{
							Type:    pbcatalog.ServiceType,
							Tenancy: tenancy,
							Name:    "svc-1",
						},
						Consumers: []*pbmulticluster.ComputedExportedServiceConsumer{
							suite.constructConsumer("peer-2", "peer"),
							suite.constructConsumer("part-1", "partition"),
							suite.constructConsumer("part-n", "partition"),
						},
					},
					{
						TargetRef: &pbresource.Reference{
							Type:    pbcatalog.ServiceType,
							Tenancy: tenancy,
							Name:    "svc-2",
						},
						Consumers: []*pbmulticluster.ComputedExportedServiceConsumer{
							suite.constructConsumer("peer-2", "peer"),
							suite.constructConsumer("part-1", "partition"),
							suite.constructConsumer("part-n", "partition"),
						},
					},
				},
			}
		} else {
			expectedCES = &pbmulticluster.ComputedExportedServices{
				Services: []*pbmulticluster.ComputedExportedService{
					{
						TargetRef: &pbresource.Reference{
							Type:    pbcatalog.ServiceType,
							Tenancy: resource.DefaultNamespacedTenancy(),
							Name:    "svc-0",
						},
						Consumers: []*pbmulticluster.ComputedExportedServiceConsumer{
							suite.constructConsumer("peer-1", "peer"),
							suite.constructConsumer("peer-2", "peer"),
						},
					},
				},
			}
		}

		prototest.AssertDeepEqual(suite.T(), expectedCES, computedCES)
	})
}

func (suite *controllerSuite) TestController() {
	// Run the controller manager
	mgr := controller.NewManager(suite.client, suite.rt.Logger)
	mgr.Register(Controller(suite.samenessGroupExpander))
	mgr.SetRaftLeader(true)
	go mgr.Run(suite.ctx)

	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		id := rtest.Resource(pbmulticluster.ComputedExportedServicesType, "global").WithTenancy(&pbresource.Tenancy{
			Partition: tenancy.Partition,
		}).ID()
		require.NotNil(suite.T(), id)

		svc1 := suite.writeService("svc1", tenancy)
		exportedSvcData := &pbmulticluster.ExportedServices{
			Services: []string{"svc1", "svcx"},
			Consumers: []*pbmulticluster.ExportedServicesConsumer{{ConsumerTenancy: &pbmulticluster.ExportedServicesConsumer_Peer{
				Peer: "peer-0",
			}}},
		}

		if suite.isEnterprise {
			exportedSvcData.Consumers = append(exportedSvcData.Consumers, &pbmulticluster.ExportedServicesConsumer{ConsumerTenancy: &pbmulticluster.ExportedServicesConsumer_Partition{
				Partition: "part-0",
			}})
		}

		expSvc := suite.writeExportedService("expsvc", tenancy, exportedSvcData)
		require.NotNil(suite.T(), expSvc)

		res := suite.client.WaitForResourceExists(suite.T(), id)
		computedCES := suite.getComputedExportedSvcData(res)
		rtest.RequireStatusCondition(suite.T(), res, statusKey, conditionComputed())

		expectedComputedExportedService := constructComputedExportedServices(
			constructComputedExportedService(
				constructSvcReference("svc1", tenancy),
				[]*pbmulticluster.ComputedExportedServiceConsumer{
					suite.constructConsumer("peer-0", "peer"),
					suite.constructConsumer("part-0", "partition"),
				}),
		)

		prototest.AssertDeepEqual(suite.T(), expectedComputedExportedService, computedCES)

		svc2 := suite.writeService("svc2", tenancy)
		svc0 := suite.writeService("svc0", &pbresource.Tenancy{Partition: tenancy.Partition, Namespace: "app"})

		exportedNamespaceSvcData := &pbmulticluster.NamespaceExportedServices{
			Consumers: []*pbmulticluster.ExportedServicesConsumer{{ConsumerTenancy: &pbmulticluster.ExportedServicesConsumer_Peer{
				Peer: "peer-1",
			}}},
		}
		namespaceExportedSvc := suite.writeNamespaceExportedService("namesvc", tenancy, exportedNamespaceSvcData)

		res = suite.client.WaitForNewVersion(suite.T(), id, res.Version)
		computedCES = suite.getComputedExportedSvcData(res)
		rtest.RequireStatusCondition(suite.T(), res, statusKey, conditionComputed())

		expectedComputedExportedService = constructComputedExportedServices(
			constructComputedExportedService(
				constructSvcReference("svc1", tenancy),
				[]*pbmulticluster.ComputedExportedServiceConsumer{
					suite.constructConsumer("peer-0", "peer"),
					suite.constructConsumer("peer-1", "peer"),
					suite.constructConsumer("part-0", "partition"),
				},
			),
			constructComputedExportedService(
				constructSvcReference("svc2", tenancy),
				[]*pbmulticluster.ComputedExportedServiceConsumer{
					suite.constructConsumer("peer-1", "peer"),
				},
			),
		)

		prototest.AssertDeepEqual(suite.T(), expectedComputedExportedService, computedCES)

		svc3 := suite.writeService("svc3", tenancy)

		res = suite.client.WaitForNewVersion(suite.T(), id, res.Version)
		computedCES = suite.getComputedExportedSvcData(res)
		rtest.RequireStatusCondition(suite.T(), res, statusKey, conditionComputed())
		expectedComputedExportedService = constructComputedExportedServices(
			constructComputedExportedService(
				constructSvcReference("svc1", tenancy),
				[]*pbmulticluster.ComputedExportedServiceConsumer{
					suite.constructConsumer("peer-0", "peer"),
					suite.constructConsumer("peer-1", "peer"),
					suite.constructConsumer("part-0", "partition"),
				},
			),
			constructComputedExportedService(
				constructSvcReference("svc2", tenancy),
				[]*pbmulticluster.ComputedExportedServiceConsumer{
					suite.constructConsumer("peer-1", "peer"),
				},
			),
			constructComputedExportedService(
				constructSvcReference("svc3", tenancy),
				[]*pbmulticluster.ComputedExportedServiceConsumer{
					suite.constructConsumer("peer-1", "peer"),
				},
			),
		)

		prototest.AssertDeepEqual(suite.T(), expectedComputedExportedService, computedCES)

		suite.client.MustDelete(suite.T(), svc3.Id)

		res = suite.client.WaitForNewVersion(suite.T(), id, res.Version)
		computedCES = suite.getComputedExportedSvcData(res)
		rtest.RequireStatusCondition(suite.T(), res, statusKey, conditionComputed())
		expectedComputedExportedService = constructComputedExportedServices(
			constructComputedExportedService(
				constructSvcReference("svc1", tenancy),
				[]*pbmulticluster.ComputedExportedServiceConsumer{
					suite.constructConsumer("peer-0", "peer"),
					suite.constructConsumer("peer-1", "peer"),
					suite.constructConsumer("part-0", "partition"),
				},
			),
			constructComputedExportedService(
				constructSvcReference("svc2", tenancy),
				[]*pbmulticluster.ComputedExportedServiceConsumer{
					suite.constructConsumer("peer-1", "peer"),
				},
			),
		)

		prototest.AssertDeepEqual(suite.T(), expectedComputedExportedService, computedCES)

		partitionedExportedSvcData := &pbmulticluster.PartitionExportedServices{
			Consumers: []*pbmulticluster.ExportedServicesConsumer{{ConsumerTenancy: &pbmulticluster.ExportedServicesConsumer_Peer{
				Peer: "peer-1",
			}}, {ConsumerTenancy: &pbmulticluster.ExportedServicesConsumer_Peer{
				Peer: "peer-2",
			}}},
		}
		partExpService := suite.writePartitionedExportedService("partsvc", tenancy, partitionedExportedSvcData)

		res = suite.client.WaitForNewVersion(suite.T(), id, res.Version)
		computedCES = suite.getComputedExportedSvcData(res)
		rtest.RequireStatusCondition(suite.T(), res, statusKey, conditionComputed())
		expectedComputedExportedService = constructComputedExportedServices(
			constructComputedExportedService(
				constructSvcReference("svc0", &pbresource.Tenancy{Partition: tenancy.Partition, Namespace: "app"}),
				[]*pbmulticluster.ComputedExportedServiceConsumer{
					suite.constructConsumer("peer-1", "peer"),
					suite.constructConsumer("peer-2", "peer"),
				},
			),
			constructComputedExportedService(
				constructSvcReference("svc1", tenancy),
				[]*pbmulticluster.ComputedExportedServiceConsumer{
					suite.constructConsumer("peer-0", "peer"),
					suite.constructConsumer("peer-1", "peer"),
					suite.constructConsumer("peer-2", "peer"),
					suite.constructConsumer("part-0", "partition"),
				},
			),
			constructComputedExportedService(
				constructSvcReference("svc2", tenancy),
				[]*pbmulticluster.ComputedExportedServiceConsumer{
					suite.constructConsumer("peer-1", "peer"),
					suite.constructConsumer("peer-2", "peer"),
				},
			),
		)
		prototest.AssertDeepEqual(suite.T(), expectedComputedExportedService, computedCES)

		svc4 := suite.writeService("svc4", &pbresource.Tenancy{Partition: tenancy.Partition, Namespace: "app"})

		res = suite.client.WaitForNewVersion(suite.T(), id, res.Version)
		computedCES = suite.getComputedExportedSvcData(res)
		rtest.RequireStatusCondition(suite.T(), res, statusKey, conditionComputed())
		expectedComputedExportedService = constructComputedExportedServices(
			constructComputedExportedService(
				constructSvcReference("svc0", &pbresource.Tenancy{Partition: tenancy.Partition, Namespace: "app"}),
				[]*pbmulticluster.ComputedExportedServiceConsumer{
					suite.constructConsumer("peer-1", "peer"),
					suite.constructConsumer("peer-2", "peer"),
				},
			),
			constructComputedExportedService(
				constructSvcReference("svc4", &pbresource.Tenancy{Partition: tenancy.Partition, Namespace: "app"}),
				[]*pbmulticluster.ComputedExportedServiceConsumer{
					suite.constructConsumer("peer-1", "peer"),
					suite.constructConsumer("peer-2", "peer"),
				},
			),
			constructComputedExportedService(
				constructSvcReference("svc1", tenancy),
				[]*pbmulticluster.ComputedExportedServiceConsumer{
					suite.constructConsumer("peer-0", "peer"),
					suite.constructConsumer("peer-1", "peer"),
					suite.constructConsumer("peer-2", "peer"),
					suite.constructConsumer("part-0", "partition"),
				},
			),
			constructComputedExportedService(
				constructSvcReference("svc2", tenancy),
				[]*pbmulticluster.ComputedExportedServiceConsumer{
					suite.constructConsumer("peer-1", "peer"),
					suite.constructConsumer("peer-2", "peer"),
				},
			),
		)
		prototest.AssertDeepEqual(suite.T(), expectedComputedExportedService, computedCES)

		suite.client.MustDelete(suite.T(), svc4.Id)

		res = suite.client.WaitForNewVersion(suite.T(), id, res.Version)
		computedCES = suite.getComputedExportedSvcData(res)
		rtest.RequireStatusCondition(suite.T(), res, statusKey, conditionComputed())
		expectedComputedExportedService = constructComputedExportedServices(
			constructComputedExportedService(
				constructSvcReference("svc0", &pbresource.Tenancy{Partition: tenancy.Partition, Namespace: "app"}),
				[]*pbmulticluster.ComputedExportedServiceConsumer{
					suite.constructConsumer("peer-1", "peer"),
					suite.constructConsumer("peer-2", "peer"),
				},
			),
			constructComputedExportedService(
				constructSvcReference("svc1", tenancy),
				[]*pbmulticluster.ComputedExportedServiceConsumer{
					suite.constructConsumer("peer-0", "peer"),
					suite.constructConsumer("peer-1", "peer"),
					suite.constructConsumer("peer-2", "peer"),
					suite.constructConsumer("part-0", "partition"),
				},
			),
			constructComputedExportedService(
				constructSvcReference("svc2", tenancy),
				[]*pbmulticluster.ComputedExportedServiceConsumer{
					suite.constructConsumer("peer-1", "peer"),
					suite.constructConsumer("peer-2", "peer"),
				},
			),
		)

		prototest.AssertDeepEqual(suite.T(), expectedComputedExportedService, computedCES)

		suite.writeService("svc5", tenancy)

		res = suite.client.WaitForNewVersion(suite.T(), id, res.Version)
		computedCES = suite.getComputedExportedSvcData(res)
		rtest.RequireStatusCondition(suite.T(), res, statusKey, conditionComputed())
		expectedComputedExportedService = constructComputedExportedServices(
			constructComputedExportedService(
				constructSvcReference("svc0", &pbresource.Tenancy{Partition: tenancy.Partition, Namespace: "app"}),
				[]*pbmulticluster.ComputedExportedServiceConsumer{
					suite.constructConsumer("peer-1", "peer"),
					suite.constructConsumer("peer-2", "peer"),
				},
			),
			constructComputedExportedService(
				constructSvcReference("svc1", tenancy),
				[]*pbmulticluster.ComputedExportedServiceConsumer{
					suite.constructConsumer("peer-0", "peer"),
					suite.constructConsumer("peer-1", "peer"),
					suite.constructConsumer("peer-2", "peer"),
					suite.constructConsumer("part-0", "partition"),
				},
			),
			constructComputedExportedService(
				constructSvcReference("svc2", tenancy),
				[]*pbmulticluster.ComputedExportedServiceConsumer{
					suite.constructConsumer("peer-1", "peer"),
					suite.constructConsumer("peer-2", "peer"),
				},
			),
			constructComputedExportedService(
				constructSvcReference("svc5", tenancy),
				[]*pbmulticluster.ComputedExportedServiceConsumer{
					suite.constructConsumer("peer-1", "peer"),
					suite.constructConsumer("peer-2", "peer"),
				},
			),
		)
		prototest.AssertDeepEqual(suite.T(), expectedComputedExportedService, computedCES)

		suite.client.MustDelete(suite.T(), partExpService.Id)

		res = suite.client.WaitForNewVersion(suite.T(), id, res.Version)
		computedCES = suite.getComputedExportedSvcData(res)
		rtest.RequireStatusCondition(suite.T(), res, statusKey, conditionComputed())
		expectedComputedExportedService = constructComputedExportedServices(
			constructComputedExportedService(
				constructSvcReference("svc1", tenancy),
				[]*pbmulticluster.ComputedExportedServiceConsumer{
					suite.constructConsumer("peer-0", "peer"),
					suite.constructConsumer("peer-1", "peer"),
					suite.constructConsumer("part-0", "partition"),
				},
			),
			constructComputedExportedService(
				constructSvcReference("svc2", tenancy),
				[]*pbmulticluster.ComputedExportedServiceConsumer{
					suite.constructConsumer("peer-1", "peer"),
				},
			),
			constructComputedExportedService(
				constructSvcReference("svc5", tenancy),
				[]*pbmulticluster.ComputedExportedServiceConsumer{
					suite.constructConsumer("peer-1", "peer"),
				},
			),
		)
		prototest.AssertDeepEqual(suite.T(), expectedComputedExportedService, computedCES)

		suite.client.MustDelete(suite.T(), namespaceExportedSvc.Id)
		res = suite.client.WaitForNewVersion(suite.T(), id, res.Version)
		computedCES = suite.getComputedExportedSvcData(res)
		rtest.RequireStatusCondition(suite.T(), res, statusKey, conditionComputed())
		expectedComputedExportedService = constructComputedExportedServices(
			constructComputedExportedService(
				constructSvcReference("svc1", tenancy),
				[]*pbmulticluster.ComputedExportedServiceConsumer{
					suite.constructConsumer("peer-0", "peer"),
					suite.constructConsumer("part-0", "partition"),
				},
			),
		)
		prototest.AssertDeepEqual(suite.T(), expectedComputedExportedService, computedCES)

		suite.client.MustDelete(suite.T(), expSvc.Id)
		suite.client.WaitForDeletion(suite.T(), res.Id)

		namespaceExportedSvc = suite.writeNamespaceExportedService("namesvc1", &pbresource.Tenancy{Partition: tenancy.Partition, Namespace: "app"}, exportedNamespaceSvcData)

		res = suite.client.WaitForResourceExists(suite.T(), id)
		computedCES = suite.getComputedExportedSvcData(res)
		rtest.RequireStatusCondition(suite.T(), res, statusKey, conditionComputed())
		expectedComputedExportedService = constructComputedExportedServices(
			constructComputedExportedService(
				constructSvcReference("svc0", &pbresource.Tenancy{Partition: tenancy.Partition, Namespace: "app"}),
				[]*pbmulticluster.ComputedExportedServiceConsumer{
					suite.constructConsumer("peer-1", "peer"),
				},
			),
		)
		prototest.AssertDeepEqual(suite.T(), expectedComputedExportedService, computedCES)

		expSvc = suite.writeExportedService("expsvc1", tenancy, exportedSvcData)
		res = suite.client.WaitForNewVersion(suite.T(), id, res.Version)
		computedCES = suite.getComputedExportedSvcData(res)
		rtest.RequireStatusCondition(suite.T(), res, statusKey, conditionComputed())
		expectedComputedExportedService = constructComputedExportedServices(
			constructComputedExportedService(
				constructSvcReference("svc0", &pbresource.Tenancy{Partition: tenancy.Partition, Namespace: "app"}),
				[]*pbmulticluster.ComputedExportedServiceConsumer{
					suite.constructConsumer("peer-1", "peer"),
				},
			),
			constructComputedExportedService(
				constructSvcReference("svc1", tenancy),
				[]*pbmulticluster.ComputedExportedServiceConsumer{
					suite.constructConsumer("peer-0", "peer"),
					suite.constructConsumer("part-0", "partition"),
				},
			),
		)
		prototest.AssertDeepEqual(suite.T(), expectedComputedExportedService, computedCES)

		suite.client.MustDelete(suite.T(), svc0.Id)
		suite.client.MustDelete(suite.T(), svc1.Id)
		suite.client.WaitForDeletion(suite.T(), res.Id)

		// Delete other resources
		suite.client.MustDelete(suite.T(), svc2.Id)
		suite.client.MustDelete(suite.T(), svc3.Id)
		suite.client.MustDelete(suite.T(), expSvc.Id)
		suite.client.MustDelete(suite.T(), namespaceExportedSvc.Id)
		suite.client.MustDelete(suite.T(), partExpService.Id)
	})
}

func (suite *controllerSuite) runTestCaseWithTenancies(testFunc func(*pbresource.Tenancy)) {
	for _, tenancy := range suite.tenancies {
		suite.Run(suite.appendTenancyInfo(tenancy), func() {
			testFunc(tenancy)
		})
	}
}

func (suite *controllerSuite) appendTenancyInfo(tenancy *pbresource.Tenancy) string {
	return fmt.Sprintf("%s_Namespace_%s_Partition", tenancy.Namespace, tenancy.Partition)
}

func (suite *controllerSuite) getComputedExportedSvcData(ces *pbresource.Resource) *pbmulticluster.ComputedExportedServices {
	decodedComputedExportedService := rtest.MustDecode[*pbmulticluster.ComputedExportedServices](suite.T(), ces)
	return decodedComputedExportedService.Data
}

func (suite *controllerSuite) writeService(name string, tenancy *pbresource.Tenancy) *pbresource.Resource {
	return rtest.Resource(pbcatalog.ServiceType, name).
		WithData(suite.T(), &pbcatalog.Service{
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			},
		}).
		WithTenancy(tenancy).
		Write(suite.T(), suite.client)
}

func (suite *controllerSuite) writeExportedService(name string, tenancy *pbresource.Tenancy, data *pbmulticluster.ExportedServices) *pbresource.Resource {
	return rtest.Resource(pbmulticluster.ExportedServicesType, name).
		WithData(suite.T(), data).
		WithTenancy(tenancy).
		Write(suite.T(), suite.client)
}

func (suite *controllerSuite) writeNamespaceExportedService(name string, tenancy *pbresource.Tenancy, data *pbmulticluster.NamespaceExportedServices) *pbresource.Resource {
	return rtest.Resource(pbmulticluster.NamespaceExportedServicesType, name).
		WithData(suite.T(), data).
		WithTenancy(tenancy).
		Write(suite.T(), suite.client)
}

func (suite *controllerSuite) writePartitionedExportedService(name string, tenancy *pbresource.Tenancy, data *pbmulticluster.PartitionExportedServices) *pbresource.Resource {
	return rtest.Resource(pbmulticluster.PartitionExportedServicesType, name).
		WithData(suite.T(), data).
		WithTenancy(&pbresource.Tenancy{Partition: tenancy.Partition}).
		Write(suite.T(), suite.client)
}

func (suite *controllerSuite) constructConsumer(name, consumerType string) *pbmulticluster.ComputedExportedServiceConsumer {
	if consumerType == "peer" {
		return &pbmulticluster.ComputedExportedServiceConsumer{
			Tenancy: &pbmulticluster.ComputedExportedServiceConsumer_Peer{
				Peer: name,
			},
		}
	}

	if !suite.isEnterprise {
		return nil
	}

	return &pbmulticluster.ComputedExportedServiceConsumer{
		Tenancy: &pbmulticluster.ComputedExportedServiceConsumer_Partition{
			Partition: name,
		},
	}
}

func (suite *controllerSuite) controllerRuntimeWithPassThroughClient(client *passThroughResourceClient) controller.Runtime {
	return controller.Runtime{
		Cache:  suite.rt.Cache,
		Logger: suite.rt.Logger,
		Client: client,
	}
}

func constructComputedExportedService(ref *pbresource.Reference, consumers []*pbmulticluster.ComputedExportedServiceConsumer) *pbmulticluster.ComputedExportedService {
	finalConsumers := make([]*pbmulticluster.ComputedExportedServiceConsumer, 0)
	for _, c := range consumers {
		if c == nil {
			continue
		}

		finalConsumers = append(finalConsumers, c)
	}

	return &pbmulticluster.ComputedExportedService{
		TargetRef: ref,
		Consumers: finalConsumers,
	}
}

func constructComputedExportedServices(services ...*pbmulticluster.ComputedExportedService) *pbmulticluster.ComputedExportedServices {
	return &pbmulticluster.ComputedExportedServices{
		Services: services,
	}
}

func constructSvcReference(name string, tenancy *pbresource.Tenancy) *pbresource.Reference {
	return &pbresource.Reference{
		Type:    pbcatalog.ServiceType,
		Tenancy: tenancy,
		Name:    name,
	}
}

type passThroughResourceClient struct {
	client pbresource.ResourceServiceClient

	writesCount      int
	writeStatusCount int
}

// newPassThroughResourceClient returns a client that implements pbresource.ResourceServiceClient
// It can be used to keep track of operations happening within a controller
func newPassThroughResourceClient(client *rtest.Client) *passThroughResourceClient {
	return &passThroughResourceClient{
		client: client,
	}
}

func (pc *passThroughResourceClient) resetCounters() {
	pc.writeStatusCount = 0
	pc.writesCount = 0
}

func (pc *passThroughResourceClient) Read(ctx context.Context, in *pbresource.ReadRequest, opts ...grpc.CallOption) (*pbresource.ReadResponse, error) {
	return pc.client.Read(ctx, in, opts...)
}

func (pc *passThroughResourceClient) Write(ctx context.Context, in *pbresource.WriteRequest, opts ...grpc.CallOption) (*pbresource.WriteResponse, error) {
	pc.writesCount++
	return pc.client.Write(ctx, in, opts...)
}

func (pc *passThroughResourceClient) WriteStatus(ctx context.Context, in *pbresource.WriteStatusRequest, opts ...grpc.CallOption) (*pbresource.WriteStatusResponse, error) {
	pc.writeStatusCount++
	return pc.client.WriteStatus(ctx, in, opts...)
}

func (pc *passThroughResourceClient) Delete(ctx context.Context, in *pbresource.DeleteRequest, opts ...grpc.CallOption) (*pbresource.DeleteResponse, error) {
	return pc.client.Delete(ctx, in, opts...)
}

func (pc *passThroughResourceClient) List(ctx context.Context, in *pbresource.ListRequest, opts ...grpc.CallOption) (*pbresource.ListResponse, error) {
	return pc.client.List(ctx, in, opts...)
}

func (pc *passThroughResourceClient) ListByOwner(ctx context.Context, in *pbresource.ListByOwnerRequest, opts ...grpc.CallOption) (*pbresource.ListByOwnerResponse, error) {
	return pc.client.ListByOwner(ctx, in, opts...)
}

func (pc *passThroughResourceClient) WatchList(ctx context.Context, in *pbresource.WatchListRequest, opts ...grpc.CallOption) (pbresource.ResourceService_WatchListClient, error) {
	return pc.client.WatchList(ctx, in, opts...)
}

func (pc *passThroughResourceClient) MutateAndValidate(ctx context.Context, in *pbresource.MutateAndValidateRequest, opts ...grpc.CallOption) (*pbresource.MutateAndValidateResponse, error) {
	return pc.client.MutateAndValidate(ctx, in, opts...)
}
