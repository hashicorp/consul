// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package v1compat

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/hashicorp/consul/acl"
	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/multicluster/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v2"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/version/versiontest"
)

type controllerSuite struct {
	suite.Suite
	ctx          context.Context
	ctl          *controller.TestController
	isEnterprise bool
	tenancies    []*pbresource.Tenancy
	config       *MockAggregatedConfig
}

func (suite *controllerSuite) SetupTest() {
	suite.tenancies = rtest.TestTenancies()
	suite.isEnterprise = versiontest.IsEnterprise()
	suite.ctx = testutil.TestContext(suite.T())
	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(types.Register).
		WithTenancies(suite.tenancies...).
		Run(suite.T())

	suite.config = NewMockAggregatedConfig(suite.T())
	suite.config.EXPECT().EventChannel().Return(make(chan controller.Event))
	suite.ctl = controller.NewTestController(
		Controller(suite.config),
		client,
	).WithLogger(testutil.Logger(suite.T()))
}

// Test that we do nothing if V1 exports have not been replicated to v2 resources compatible with this controller
func (suite *controllerSuite) TestReconcile_V1ExportsExist() {
	incompatibleConfig := &structs.ExportedServicesConfigEntry{
		Name: "v1Legacy",
		Meta: map[string]string{controllerMetaKey: "foo-controller"},
	}

	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		entMeta := acl.DefaultEnterpriseMeta()
		entMeta.OverridePartition(tenancy.Partition)

		suite.config.EXPECT().
			GetExportedServicesConfigEntry(suite.ctx, tenancy.Partition, entMeta).
			Return(incompatibleConfig, nil)

		resID := &pbresource.ID{
			Type:    pbmulticluster.ComputedExportedServicesType,
			Tenancy: &pbresource.Tenancy{Partition: tenancy.Partition},
			Name:    types.ComputedExportedServicesName,
		}
		err := suite.ctl.Reconcile(suite.ctx, controller.Request{ID: resID})
		require.NoError(suite.T(), err)
	})
}

// Test that we do not stop reconciler even when we fail to retrieve the config entry
func (suite *controllerSuite) TestReconcile_GetExportedServicesConfigEntry_Error() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		entMeta := acl.DefaultEnterpriseMeta()
		entMeta.OverridePartition(tenancy.Partition)

		suite.config.EXPECT().
			GetExportedServicesConfigEntry(suite.ctx, tenancy.Partition, entMeta).
			Return(nil, fmt.Errorf("failed to retrieve config entry"))

		resID := &pbresource.ID{
			Type:    pbmulticluster.ComputedExportedServicesType,
			Tenancy: &pbresource.Tenancy{Partition: tenancy.Partition},
			Name:    types.ComputedExportedServicesName,
		}
		err := suite.ctl.Reconcile(suite.ctx, controller.Request{ID: resID})
		require.NoError(suite.T(), err)
	})
}

// Delete config entry for case where resources aren't found
func (suite *controllerSuite) TestReconcile_DeleteConfig_MissingResources() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		entMeta := acl.DefaultEnterpriseMeta()
		entMeta.OverridePartition(tenancy.Partition)

		configEntry := &structs.ExportedServicesConfigEntry{
			// v1 exported-services config entries must have a Name that is the partitions name
			Name: tenancy.Partition,
			Meta: map[string]string{
				controllerMetaKey: ControllerName,
			},
			EnterpriseMeta: acl.NewEnterpriseMetaWithPartition(tenancy.Partition, ""),
		}

		suite.config.EXPECT().GetExportedServicesConfigEntry(suite.ctx, tenancy.Partition, entMeta).Return(configEntry, nil)
		suite.config.EXPECT().DeleteExportedServicesConfigEntry(suite.ctx, tenancy.Partition, entMeta).Return(nil)

		resID := &pbresource.ID{
			Type:    pbmulticluster.ComputedExportedServicesType,
			Tenancy: &pbresource.Tenancy{Partition: tenancy.Partition},
			Name:    types.ComputedExportedServicesName,
		}
		err := suite.ctl.Reconcile(suite.ctx, controller.Request{ID: resID})
		require.NoError(suite.T(), err)
	})
}

func (suite *controllerSuite) TestReconcile_NewExport_PartitionExport() {
	if !suite.isEnterprise {
		suite.T().Skip("this test should only run against the enterprise build")
	}

	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		entMeta := acl.DefaultEnterpriseMeta()
		entMeta.OverridePartition(tenancy.Partition)

		// used as a return value for GetExportedServicesConfigEntry
		existingCE := &structs.ExportedServicesConfigEntry{
			// v1 exported-services config entries must have a Name that is the partitions name
			Name: tenancy.Partition,
			Meta: map[string]string{
				controllerMetaKey: ControllerName,
			},
			EnterpriseMeta: acl.NewEnterpriseMetaWithPartition(tenancy.Partition, ""),
		}
		suite.config.EXPECT().GetExportedServicesConfigEntry(suite.ctx, tenancy.Partition, entMeta).Return(existingCE, nil)

		// expected config entry to be written by reconcile
		expectedCE := &structs.ExportedServicesConfigEntry{
			Name: tenancy.Partition,
			Services: []structs.ExportedService{
				{
					Name:      "s1",
					Namespace: resource.DefaultNamespaceName,
					Consumers: []structs.ServiceConsumer{
						{
							Partition: "p1",
						},
					},
				},
			},
			Meta: map[string]string{
				controllerMetaKey: ControllerName,
			},
			EnterpriseMeta: *entMeta,
		}
		suite.config.EXPECT().WriteExportedServicesConfigEntry(suite.ctx, expectedCE).Return(nil)

		name := "s1"
		expSv := &pbmulticluster.ExportedServices{
			Services: []string{name},
			Consumers: []*pbmulticluster.ExportedServicesConsumer{
				{ConsumerTenancy: &pbmulticluster.ExportedServicesConsumer_Partition{Partition: "p1"}},
			},
		}
		rtest.Resource(pbmulticluster.ExportedServicesType, "exported-svcs").
			WithData(suite.T(), expSv).
			WithTenancy(&pbresource.Tenancy{Partition: tenancy.Partition}).
			Write(suite.T(), suite.ctl.Runtime().Client)
		cesID := &pbresource.ID{
			Type:    pbmulticluster.ComputedExportedServicesType,
			Tenancy: &pbresource.Tenancy{Partition: tenancy.Partition},
			Name:    types.ComputedExportedServicesName,
		}
		err := suite.ctl.Reconcile(suite.ctx, controller.Request{ID: cesID})
		require.NoError(suite.T(), err)
	})
}

func (suite *controllerSuite) TestReconcile_NewExport_PeerExport() {
	if !suite.isEnterprise {
		suite.T().Skip("this test should only run against the enterprise build")
	}

	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		entMeta := acl.DefaultEnterpriseMeta()
		entMeta.OverridePartition(tenancy.Partition)

		// used as a return value for GetExportedServicesConfigEntry
		existingCE := &structs.ExportedServicesConfigEntry{
			// v1 exported-services config entries must have a Name that is the partitions name
			Name: tenancy.Partition,
			Meta: map[string]string{
				controllerMetaKey: ControllerName,
			},
			EnterpriseMeta: acl.NewEnterpriseMetaWithPartition(tenancy.Partition, ""),
		}
		suite.config.EXPECT().GetExportedServicesConfigEntry(suite.ctx, tenancy.Partition, entMeta).Return(existingCE, nil)

		// expected config entry to be written by reconcile
		expectedCE := &structs.ExportedServicesConfigEntry{
			Name: tenancy.Partition,
			Services: []structs.ExportedService{
				{
					Name:      "s1",
					Namespace: resource.DefaultNamespaceName,
					Consumers: []structs.ServiceConsumer{
						{
							Peer: "peer1",
						},
					},
				},
			},
			Meta: map[string]string{
				controllerMetaKey: ControllerName,
			},
			EnterpriseMeta: *entMeta,
		}
		suite.config.EXPECT().WriteExportedServicesConfigEntry(suite.ctx, expectedCE).Return(nil)

		name := "s1"
		expSv := &pbmulticluster.ExportedServices{
			Services: []string{name},
			Consumers: []*pbmulticluster.ExportedServicesConsumer{
				{ConsumerTenancy: &pbmulticluster.ExportedServicesConsumer_Peer{Peer: "peer1"}},
			},
		}
		rtest.Resource(pbmulticluster.ExportedServicesType, "exported-svcs").
			WithData(suite.T(), expSv).
			WithTenancy(&pbresource.Tenancy{Partition: tenancy.Partition}).
			Write(suite.T(), suite.ctl.Runtime().Client)
		cesID := &pbresource.ID{
			Type:    pbmulticluster.ComputedExportedServicesType,
			Tenancy: &pbresource.Tenancy{Partition: tenancy.Partition},
			Name:    types.ComputedExportedServicesName,
		}
		err := suite.ctl.Reconcile(suite.ctx, controller.Request{ID: cesID})
		require.NoError(suite.T(), err)
	})
}

func (suite *controllerSuite) TestReconcile_NewExport_SamenessGroupsExport() {
	if !suite.isEnterprise {
		suite.T().Skip("this test should only run against the enterprise build")
	}

	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		entMeta := acl.DefaultEnterpriseMeta()
		entMeta.OverridePartition(tenancy.Partition)

		// used as a return value for GetExportedServicesConfigEntry
		existingCE := &structs.ExportedServicesConfigEntry{
			// v1 exported-services config entries must have a Name that is the partitions name
			Name: tenancy.Partition,
			Meta: map[string]string{
				controllerMetaKey: ControllerName,
			},
			EnterpriseMeta: acl.NewEnterpriseMetaWithPartition(tenancy.Partition, ""),
		}
		suite.config.EXPECT().GetExportedServicesConfigEntry(suite.ctx, tenancy.Partition, entMeta).Return(existingCE, nil)

		// expected config entry to be written by reconcile
		expectedCE := &structs.ExportedServicesConfigEntry{
			Name: tenancy.Partition,
			Services: []structs.ExportedService{
				{
					Name:      "s1",
					Namespace: resource.DefaultNamespaceName,
					Consumers: []structs.ServiceConsumer{
						{
							SamenessGroup: "sg1",
						},
					},
				},
			},
			Meta: map[string]string{
				controllerMetaKey: ControllerName,
			},
			EnterpriseMeta: *entMeta,
		}
		suite.config.EXPECT().WriteExportedServicesConfigEntry(suite.ctx, expectedCE).Return(nil)

		name := "s1"
		expSv := &pbmulticluster.ExportedServices{
			Services: []string{name},
			Consumers: []*pbmulticluster.ExportedServicesConsumer{
				{ConsumerTenancy: &pbmulticluster.ExportedServicesConsumer_SamenessGroup{SamenessGroup: "sg1"}},
			},
		}
		rtest.Resource(pbmulticluster.ExportedServicesType, "exported-svcs").
			WithData(suite.T(), expSv).
			WithTenancy(&pbresource.Tenancy{Partition: tenancy.Partition}).
			Write(suite.T(), suite.ctl.Runtime().Client)
		cesID := &pbresource.ID{
			Type:    pbmulticluster.ComputedExportedServicesType,
			Tenancy: &pbresource.Tenancy{Partition: tenancy.Partition},
			Name:    types.ComputedExportedServicesName,
		}
		err := suite.ctl.Reconcile(suite.ctx, controller.Request{ID: cesID})
		require.NoError(suite.T(), err)
	})
}

func (suite *controllerSuite) TestReconcile_MultipleExports() {
	if !suite.isEnterprise {
		suite.T().Skip("this test should only run against the enterprise build")
	}

	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		entMeta := acl.DefaultEnterpriseMeta()
		entMeta.OverridePartition(tenancy.Partition)
		configCE := &structs.ExportedServicesConfigEntry{
			// v1 exported-services config entries must have a Name that is the partitions name
			Name: tenancy.Partition,
			Meta: map[string]string{
				controllerMetaKey: ControllerName,
			},
			EnterpriseMeta: acl.NewEnterpriseMetaWithPartition(tenancy.Partition, ""),
		}

		suite.config.EXPECT().
			GetExportedServicesConfigEntry(suite.ctx, tenancy.Partition, entMeta).
			Return(configCE, nil)

		expSv1 := &pbmulticluster.ExportedServices{
			Services: []string{"s1"},
			Consumers: []*pbmulticluster.ExportedServicesConsumer{
				{ConsumerTenancy: &pbmulticluster.ExportedServicesConsumer_Partition{Partition: "p1"}},
				{ConsumerTenancy: &pbmulticluster.ExportedServicesConsumer_Partition{Partition: "p4"}},
			},
		}
		expSv2 := &pbmulticluster.ExportedServices{
			Services: []string{"s2"},
			Consumers: []*pbmulticluster.ExportedServicesConsumer{
				{ConsumerTenancy: &pbmulticluster.ExportedServicesConsumer_Partition{Partition: "p2"}},
				{ConsumerTenancy: &pbmulticluster.ExportedServicesConsumer_Peer{Peer: "peer1"}},
			},
		}
		expSv3 := &pbmulticluster.ExportedServices{
			Services: []string{"s1", "s3"},
			Consumers: []*pbmulticluster.ExportedServicesConsumer{
				{ConsumerTenancy: &pbmulticluster.ExportedServicesConsumer_Partition{Partition: "p3"}},
				{ConsumerTenancy: &pbmulticluster.ExportedServicesConsumer_SamenessGroup{SamenessGroup: "sg1"}},
			},
		}

		for i, s := range []*pbmulticluster.ExportedServices{expSv1, expSv2, expSv3} {
			rtest.Resource(pbmulticluster.ExportedServicesType, fmt.Sprintf("exported-svcs-%d", i)).
				WithData(suite.T(), s).
				WithTenancy(&pbresource.Tenancy{Partition: tenancy.Partition}).
				Write(suite.T(), suite.ctl.Runtime().Client)
		}

		cesID := &pbresource.ID{
			Type:    pbmulticluster.ComputedExportedServicesType,
			Tenancy: &pbresource.Tenancy{Partition: tenancy.Partition},
			Name:    types.ComputedExportedServicesName,
		}

		// expected computed config entry to be written by reconcile
		computedConfigEntry := &structs.ExportedServicesConfigEntry{
			Name: tenancy.Partition,
			Meta: map[string]string{
				controllerMetaKey: ControllerName,
			},
			Services: []structs.ExportedService{
				{
					Name:      "s3",
					Namespace: resource.DefaultNamespaceName,
					Consumers: []structs.ServiceConsumer{
						{Partition: "p3"},
						{SamenessGroup: "sg1"},
					},
				},
				{
					Name:      "s2",
					Namespace: resource.DefaultNamespaceName,
					Consumers: []structs.ServiceConsumer{
						{Partition: "p2"},
						{Peer: "peer1"},
					},
				},
				{
					Name:      "s1",
					Namespace: resource.DefaultNamespaceName,
					Consumers: []structs.ServiceConsumer{
						{Partition: "p1"},
						{Partition: "p3"},
						{Partition: "p4"},
						{SamenessGroup: "sg1"},
					},
				},
			},
			EnterpriseMeta: acl.NewEnterpriseMetaWithPartition(tenancy.Partition, resource.DefaultNamespaceName),
		}
		suite.config.EXPECT().
			WriteExportedServicesConfigEntry(suite.ctx, computedConfigEntry).
			Return(nil)

		err := suite.ctl.Reconcile(suite.ctx, controller.Request{ID: cesID})
		require.NoError(suite.T(), err)
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

func TestController(t *testing.T) {
	suite.Run(t, new(controllerSuite))
}
