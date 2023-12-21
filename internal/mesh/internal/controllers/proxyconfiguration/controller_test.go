// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package proxyconfiguration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/mappers/workloadselectionmapper"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/iptables"
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

	proxyCfg1 *pbmesh.ProxyConfiguration
	proxyCfg2 *pbmesh.ProxyConfiguration
	proxyCfg3 *pbmesh.ProxyConfiguration

	expComputedProxyCfg *pbmesh.ComputedProxyConfiguration

	tenancies []*pbresource.Tenancy
}

func (suite *controllerTestSuite) SetupTest() {
	suite.tenancies = resourcetest.TestTenancies()
	resourceClient := svctest.NewResourceServiceBuilder().
		WithRegisterFns(types.Register, catalog.RegisterTypes).
		WithTenancies(suite.tenancies...).
		Run(suite.T())
	suite.client = resourcetest.NewClient(resourceClient)
	suite.runtime = controller.Runtime{Client: resourceClient, Logger: testutil.Logger(suite.T())}
	suite.ctx = testutil.TestContext(suite.T())

	suite.ctl = &reconciler{
		proxyConfigMapper: workloadselectionmapper.New[*pbmesh.ProxyConfiguration](pbmesh.ComputedProxyConfigurationType),
	}
}

func (suite *controllerTestSuite) TestReconcile_NoWorkload() {
	// This test ensures that removed workloads are ignored and don't result
	// in the creation of the proxy state template.
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		id := resourcetest.Resource(pbmesh.ComputedProxyConfigurationType, "not-found").WithTenancy(tenancy).ID()
		err := suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
			ID: id,
		})
		require.NoError(suite.T(), err)

		suite.client.RequireResourceNotFound(suite.T(), id)
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

		cpcID := resourcetest.Resource(pbmesh.ComputedProxyConfigurationType, "non-mesh").
			WithTenancy(tenancy).
			Write(suite.T(), suite.client).Id

		err := suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
			ID: cpcID,
		})
		require.NoError(suite.T(), err)

		suite.client.RequireResourceNotFound(suite.T(), cpcID)
	})
}

func (suite *controllerTestSuite) TestReconcile_HappyPath() {
	// Write all three proxy cfgs.
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		pCfg1 := resourcetest.Resource(pbmesh.ProxyConfigurationType, "cfg1").
			WithTenancy(tenancy).
			WithData(suite.T(), suite.proxyCfg1).
			Write(suite.T(), suite.client)
		_, err := suite.ctl.proxyConfigMapper.MapToComputedType(suite.ctx, suite.runtime, pCfg1)
		require.NoError(suite.T(), err)

		pCfg2 := resourcetest.Resource(pbmesh.ProxyConfigurationType, "cfg2").
			WithTenancy(tenancy).
			WithData(suite.T(), suite.proxyCfg2).
			Write(suite.T(), suite.client)
		_, err = suite.ctl.proxyConfigMapper.MapToComputedType(suite.ctx, suite.runtime, pCfg2)
		require.NoError(suite.T(), err)

		pCfg3 := resourcetest.Resource(pbmesh.ProxyConfigurationType, "cfg3").
			WithTenancy(tenancy).
			WithData(suite.T(), suite.proxyCfg3).
			Write(suite.T(), suite.client)
		_, err = suite.ctl.proxyConfigMapper.MapToComputedType(suite.ctx, suite.runtime, pCfg3)
		require.NoError(suite.T(), err)

		cpcID := resource.ReplaceType(pbmesh.ComputedProxyConfigurationType, suite.workloadRes.Id)
		err = suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
			ID: cpcID,
		})

		require.NoError(suite.T(), err)

		suite.requireComputedProxyConfiguration(suite.T(), cpcID)
	})
}

func (suite *controllerTestSuite) TestReconcile_NoProxyConfigs() {
	// Create a proxy cfg and map it so that it gets saved to cache.
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		pCfg1 := resourcetest.Resource(pbmesh.ProxyConfigurationType, "cfg1").
			WithTenancy(tenancy).
			WithData(suite.T(), suite.proxyCfg1).
			Build()
		_, err := suite.ctl.proxyConfigMapper.MapToComputedType(suite.ctx, suite.runtime, pCfg1)
		require.NoError(suite.T(), err)

		cpcID := resourcetest.Resource(pbmesh.ComputedProxyConfigurationType, suite.workloadRes.Id.Name).
			WithTenancy(tenancy).
			Write(suite.T(), suite.client).Id
		err = suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
			ID: cpcID,
		})

		require.NoError(suite.T(), err)

		suite.client.RequireResourceNotFound(suite.T(), cpcID)
	})
}

func (suite *controllerTestSuite) TestController() {
	// Run the controller manager
	mgr := controller.NewManager(suite.client, suite.runtime.Logger)

	m := workloadselectionmapper.New[*pbmesh.ProxyConfiguration](pbmesh.ComputedProxyConfigurationType)
	mgr.Register(Controller(m))
	mgr.SetRaftLeader(true)
	go mgr.Run(suite.ctx)

	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		// Write proxy configs.
		pCfg1 := resourcetest.Resource(pbmesh.ProxyConfigurationType, "cfg1").
			WithTenancy(tenancy).
			WithData(suite.T(), suite.proxyCfg1).
			Write(suite.T(), suite.client)

		pCfg2 := resourcetest.Resource(pbmesh.ProxyConfigurationType, "cfg2").
			WithTenancy(tenancy).
			WithData(suite.T(), suite.proxyCfg2).
			Write(suite.T(), suite.client)

		pCfg3 := resourcetest.Resource(pbmesh.ProxyConfigurationType, "cfg3").
			WithTenancy(tenancy).
			WithData(suite.T(), suite.proxyCfg3).
			Write(suite.T(), suite.client)

		cpcID := resource.ReplaceType(pbmesh.ComputedProxyConfigurationType, suite.workloadRes.Id)
		testutil.RunStep(suite.T(), "computed proxy config generation", func(t *testing.T) {
			retry.Run(t, func(r *retry.R) {
				suite.client.RequireResourceExists(r, cpcID)
				suite.requireComputedProxyConfiguration(r, cpcID)
			})
		})

		testutil.RunStep(suite.T(), "add another workload", func(t *testing.T) {
			// Create another workload that will match only proxyCfg2.
			matchingWorkload := resourcetest.Resource(pbcatalog.WorkloadType, "test-extra-workload").WithTenancy(tenancy).
				WithData(t, suite.workload).
				Write(t, suite.client)
			matchingWorkloadCPCID := resource.ReplaceType(pbmesh.ComputedProxyConfigurationType, matchingWorkload.Id)

			retry.Run(t, func(r *retry.R) {
				suite.client.RequireResourceExists(r, cpcID)
				suite.requireComputedProxyConfiguration(r, cpcID)

				matchingWorkloadCPC := suite.client.RequireResourceExists(r, matchingWorkloadCPCID)
				dec := resourcetest.MustDecode[*pbmesh.ComputedProxyConfiguration](r, matchingWorkloadCPC)
				prototest.AssertDeepEqual(r, suite.proxyCfg2.GetDynamicConfig(), dec.GetData().GetDynamicConfig())
				prototest.AssertDeepEqual(r, suite.proxyCfg2.GetBootstrapConfig(), dec.GetData().GetBootstrapConfig())
			})
		})

		testutil.RunStep(suite.T(), "update proxy config selector", func(t *testing.T) {
			// Update proxy config selector to no longer select "test-workload"
			updatedProxyCfg := proto.Clone(suite.proxyCfg2).(*pbmesh.ProxyConfiguration)
			updatedProxyCfg.Workloads = &pbcatalog.WorkloadSelector{
				Names: []string{"test-extra-workload"},
			}

			matchingWorkload := resourcetest.Resource(pbcatalog.WorkloadType, "test-extra-workload").
				WithTenancy(tenancy).
				WithData(t, suite.workload).
				Write(t, suite.client)
			matchingWorkloadCPCID := resource.ReplaceType(pbmesh.ComputedProxyConfigurationType, matchingWorkload.Id)
			resourcetest.Resource(pbmesh.ProxyConfigurationType, "cfg2").
				WithTenancy(tenancy).
				WithData(suite.T(), updatedProxyCfg).
				Write(suite.T(), suite.client)

			retry.Run(t, func(r *retry.R) {
				res := suite.client.RequireResourceExists(r, cpcID)

				// The "test-workload" computed proxy configurations should now be updated to use only proxy cfg 1 and 3.
				expProxyCfg := &pbmesh.ComputedProxyConfiguration{
					DynamicConfig: &pbmesh.DynamicConfig{
						Mode:             pbmesh.ProxyMode_PROXY_MODE_TRANSPARENT,
						TransparentProxy: &pbmesh.TransparentProxy{OutboundListenerPort: iptables.DefaultTProxyOutboundPort},
					},
					BootstrapConfig: &pbmesh.BootstrapConfig{
						PrometheusBindAddr: "0.0.0.0:9000",
					},
				}
				dec := resourcetest.MustDecode[*pbmesh.ComputedProxyConfiguration](r, res)
				prototest.AssertDeepEqual(r, expProxyCfg.GetDynamicConfig(), dec.GetData().GetDynamicConfig())
				prototest.AssertDeepEqual(r, expProxyCfg.GetBootstrapConfig(), dec.GetData().GetBootstrapConfig())

				matchingWorkloadCPC := suite.client.RequireResourceExists(r, matchingWorkloadCPCID)
				dec = resourcetest.MustDecode[*pbmesh.ComputedProxyConfiguration](r, matchingWorkloadCPC)
				prototest.AssertDeepEqual(r, suite.proxyCfg2.GetDynamicConfig(), dec.GetData().GetDynamicConfig())
				prototest.AssertDeepEqual(r, suite.proxyCfg2.GetBootstrapConfig(), dec.GetData().GetBootstrapConfig())
			})
		})

		// Delete all proxy cfgs.
		suite.client.MustDelete(suite.T(), pCfg1.Id)
		suite.client.MustDelete(suite.T(), pCfg2.Id)
		suite.client.MustDelete(suite.T(), pCfg3.Id)

		testutil.RunStep(suite.T(), "all proxy configs are deleted", func(t *testing.T) {
			retry.Run(t, func(r *retry.R) {
				suite.client.RequireResourceNotFound(r, cpcID)
			})
		})
	})
}

func TestControllerSuite(t *testing.T) {
	suite.Run(t, new(controllerTestSuite))
}

func (suite *controllerTestSuite) requireComputedProxyConfiguration(t resourcetest.T, id *pbresource.ID) {
	cpcRes := suite.client.RequireResourceExists(t, id)
	decCPC := resourcetest.MustDecode[*pbmesh.ComputedProxyConfiguration](t, cpcRes)
	prototest.AssertDeepEqual(t, suite.expComputedProxyCfg, decCPC.Data)
	resourcetest.RequireOwner(t, cpcRes, resource.ReplaceType(pbcatalog.WorkloadType, id), true)
}

func (suite *controllerTestSuite) setupResourcesWithTenancy(tenancy *pbresource.Tenancy) {
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

	suite.proxyCfg1 = &pbmesh.ProxyConfiguration{
		Workloads: &pbcatalog.WorkloadSelector{
			Names: []string{suite.workloadRes.Id.Name},
		},
		DynamicConfig: &pbmesh.DynamicConfig{
			Mode: pbmesh.ProxyMode_PROXY_MODE_TRANSPARENT,
		},
	}

	suite.proxyCfg2 = &pbmesh.ProxyConfiguration{
		Workloads: &pbcatalog.WorkloadSelector{
			Prefixes: []string{"test-"},
		},
		DynamicConfig: &pbmesh.DynamicConfig{
			Mode: pbmesh.ProxyMode_PROXY_MODE_DIRECT, // this setting should be overridden by proxycfg1
			LocalConnection: map[string]*pbmesh.ConnectionConfig{
				"tcp": {ConnectTimeout: durationpb.New(2 * time.Second)},
			},
		},
	}

	suite.proxyCfg3 = &pbmesh.ProxyConfiguration{
		Workloads: &pbcatalog.WorkloadSelector{
			Prefixes: []string{"test-wor"},
		},
		BootstrapConfig: &pbmesh.BootstrapConfig{
			PrometheusBindAddr: "0.0.0.0:9000",
		},
	}

	suite.expComputedProxyCfg = &pbmesh.ComputedProxyConfiguration{
		DynamicConfig: &pbmesh.DynamicConfig{
			Mode:             pbmesh.ProxyMode_PROXY_MODE_TRANSPARENT,
			TransparentProxy: &pbmesh.TransparentProxy{OutboundListenerPort: iptables.DefaultTProxyOutboundPort},
			LocalConnection: map[string]*pbmesh.ConnectionConfig{
				"tcp": {ConnectTimeout: durationpb.New(2 * time.Second)},
			},
		},
		BootstrapConfig: &pbmesh.BootstrapConfig{
			PrometheusBindAddr: "0.0.0.0:9000",
		},
	}
}

func (suite *controllerTestSuite) cleanupResources() {
	suite.client.MustDelete(suite.T(), suite.workloadRes.Id)
}

func (suite *controllerTestSuite) runTestCaseWithTenancies(testFunc func(*pbresource.Tenancy)) {
	for _, tenancy := range suite.tenancies {
		suite.Run(suite.appendTenancyInfo(tenancy), func() {
			suite.setupResourcesWithTenancy(tenancy)
			testFunc(tenancy)
			suite.T().Cleanup(suite.cleanupResources)
		})
	}
}

func (suite *controllerTestSuite) appendTenancyInfo(tenancy *pbresource.Tenancy) string {
	return fmt.Sprintf("%s_Namespace_%s_Partition", tenancy.Namespace, tenancy.Partition)
}
