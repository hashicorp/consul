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
	"github.com/hashicorp/consul/internal/controller/controllertest"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
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

	ctx    context.Context
	client *rtest.Client
	rt     controller.Runtime

	ctl       *controller.TestController
	tenancies []*pbresource.Tenancy

	workload    *pbcatalog.Workload
	workloadRes *pbresource.Resource

	proxyCfg1 *pbmesh.ProxyConfiguration
	proxyCfg2 *pbmesh.ProxyConfiguration
	proxyCfg3 *pbmesh.ProxyConfiguration

	expComputedProxyCfg *pbmesh.ComputedProxyConfiguration
}

func (suite *controllerTestSuite) SetupTest() {
	suite.tenancies = rtest.TestTenancies()

	suite.ctx = testutil.TestContext(suite.T())
	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(types.Register, catalog.RegisterTypes).
		WithTenancies(suite.tenancies...).
		Run(suite.T())

	suite.ctl = controller.NewTestController(Controller(), client).
		WithLogger(testutil.Logger(suite.T()))
	suite.rt = suite.ctl.Runtime()
	suite.client = rtest.NewClient(suite.rt.Client)
}

func (suite *controllerTestSuite) TestReconcile_NoWorkload() {
	// This test ensures that removed workloads are ignored and don't result
	// in the creation of the proxy state template.
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		id := rtest.Resource(pbmesh.ComputedProxyConfigurationType, "not-found").WithTenancy(tenancy).ID()
		suite.reconcileOnce(id)

		suite.client.RequireResourceNotFound(suite.T(), id)
	})
}

func (suite *controllerTestSuite) TestReconcile_NonMeshWorkload() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		rtest.Resource(pbcatalog.WorkloadType, "non-mesh").
			WithTenancy(tenancy).
			WithData(suite.T(), &pbcatalog.Workload{
				Addresses: []*pbcatalog.WorkloadAddress{{Host: "1.1.1.1"}},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"tcp": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
				},
			}).
			Write(suite.T(), suite.client)

		cpcID := rtest.Resource(pbmesh.ComputedProxyConfigurationType, "non-mesh").
			WithTenancy(tenancy).
			Write(suite.T(), suite.client).Id

		suite.reconcileOnce(cpcID)

		suite.client.RequireResourceNotFound(suite.T(), cpcID)
	})
}

func (suite *controllerTestSuite) TestReconcile_HappyPath() {
	// Write all three proxy cfgs.
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		pcID1 := rtest.Resource(pbmesh.ProxyConfigurationType, "cfg1").
			WithTenancy(tenancy).
			WithData(suite.T(), suite.proxyCfg1).
			Write(suite.T(), suite.client).
			Id

		pcID2 := rtest.Resource(pbmesh.ProxyConfigurationType, "cfg2").
			WithTenancy(tenancy).
			WithData(suite.T(), suite.proxyCfg2).
			Write(suite.T(), suite.client).
			Id

		pcID3 := rtest.Resource(pbmesh.ProxyConfigurationType, "cfg3").
			WithTenancy(tenancy).
			WithData(suite.T(), suite.proxyCfg3).
			Write(suite.T(), suite.client).
			Id

		cpcID := resource.ReplaceType(pbmesh.ComputedProxyConfigurationType, suite.workloadRes.Id)
		suite.reconcileOnce(cpcID)

		suite.expComputedProxyCfg.BoundReferences = []*pbresource.Reference{
			resource.Reference(pcID1, ""),
			resource.Reference(pcID2, ""),
			resource.Reference(pcID3, ""),
		}

		suite.requireComputedProxyConfiguration(suite.T(), cpcID)
	})
}

func (suite *controllerTestSuite) TestReconcile_NoProxyConfigs() {
	// Create a proxy cfg and map it so that it gets saved to cache.
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		rtest.Resource(pbmesh.ProxyConfigurationType, "cfg1").
			WithTenancy(tenancy).
			WithData(suite.T(), suite.proxyCfg1).
			Build()

		cpcID := rtest.Resource(pbmesh.ComputedProxyConfigurationType, suite.workloadRes.Id.Name).
			WithTenancy(tenancy).
			Write(suite.T(), suite.client).Id
		suite.reconcileOnce(cpcID)

		suite.client.RequireResourceNotFound(suite.T(), cpcID)
	})
}

func (suite *controllerTestSuite) TestController() {
	clientRaw := controllertest.NewControllerTestBuilder().
		WithTenancies(suite.tenancies...).
		WithResourceRegisterFns(types.Register, catalog.RegisterTypes).
		WithControllerRegisterFns(func(mgr *controller.Manager) {
			mgr.Register(Controller())
		}).
		Run(suite.T())

	suite.client = rtest.NewClient(clientRaw)

	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		// Write proxy configs.
		pCfg1 := rtest.Resource(pbmesh.ProxyConfigurationType, "cfg1").
			WithTenancy(tenancy).
			WithData(suite.T(), suite.proxyCfg1).
			Write(suite.T(), suite.client)

		pCfg2 := rtest.Resource(pbmesh.ProxyConfigurationType, "cfg2").
			WithTenancy(tenancy).
			WithData(suite.T(), suite.proxyCfg2).
			Write(suite.T(), suite.client)

		pCfg3 := rtest.Resource(pbmesh.ProxyConfigurationType, "cfg3").
			WithTenancy(tenancy).
			WithData(suite.T(), suite.proxyCfg3).
			Write(suite.T(), suite.client)

		suite.expComputedProxyCfg.BoundReferences = []*pbresource.Reference{
			resource.Reference(pCfg1.Id, ""),
			resource.Reference(pCfg2.Id, ""),
			resource.Reference(pCfg3.Id, ""),
		}

		cpcID := resource.ReplaceType(pbmesh.ComputedProxyConfigurationType, suite.workloadRes.Id)
		testutil.RunStep(suite.T(), "computed proxy config generation", func(t *testing.T) {
			retry.Run(t, func(r *retry.R) {
				suite.client.RequireResourceExists(r, cpcID)
				suite.requireComputedProxyConfiguration(r, cpcID)
			})
		})

		testutil.RunStep(suite.T(), "add another workload", func(t *testing.T) {
			// Create another workload that will match only proxyCfg2.
			matchingWorkload := rtest.Resource(pbcatalog.WorkloadType, "test-extra-workload").WithTenancy(tenancy).
				WithData(t, suite.workload).
				Write(t, suite.client)
			matchingWorkloadCPCID := resource.ReplaceType(pbmesh.ComputedProxyConfigurationType, matchingWorkload.Id)

			retry.Run(t, func(r *retry.R) {
				suite.client.RequireResourceExists(r, cpcID)
				suite.requireComputedProxyConfiguration(r, cpcID)

				matchingWorkloadCPC := suite.client.RequireResourceExists(r, matchingWorkloadCPCID)
				dec := rtest.MustDecode[*pbmesh.ComputedProxyConfiguration](r, matchingWorkloadCPC)
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

			matchingWorkload := rtest.Resource(pbcatalog.WorkloadType, "test-extra-workload").
				WithTenancy(tenancy).
				WithData(t, suite.workload).
				Write(t, suite.client)
			matchingWorkloadCPCID := resource.ReplaceType(pbmesh.ComputedProxyConfigurationType, matchingWorkload.Id)
			rtest.Resource(pbmesh.ProxyConfigurationType, "cfg2").
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
				dec := rtest.MustDecode[*pbmesh.ComputedProxyConfiguration](r, res)
				prototest.AssertDeepEqual(r, expProxyCfg.GetDynamicConfig(), dec.GetData().GetDynamicConfig())
				prototest.AssertDeepEqual(r, expProxyCfg.GetBootstrapConfig(), dec.GetData().GetBootstrapConfig())

				matchingWorkloadCPC := suite.client.RequireResourceExists(r, matchingWorkloadCPCID)
				dec = rtest.MustDecode[*pbmesh.ComputedProxyConfiguration](r, matchingWorkloadCPC)
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

func (suite *controllerTestSuite) requireComputedProxyConfiguration(t rtest.T, id *pbresource.ID) {
	cpcRes := suite.client.RequireResourceExists(t, id)
	decCPC := rtest.MustDecode[*pbmesh.ComputedProxyConfiguration](t, cpcRes)
	prototest.AssertDeepEqual(t, suite.expComputedProxyCfg, decCPC.Data)
	rtest.RequireOwner(t, cpcRes, resource.ReplaceType(pbcatalog.WorkloadType, id), true)
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

	suite.workloadRes = rtest.Resource(pbcatalog.WorkloadType, "test-workload").
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

func (suite *controllerTestSuite) reconcileOnce(id *pbresource.ID) {
	err := suite.ctl.Reconcile(suite.ctx, controller.Request{ID: id})
	require.NoError(suite.T(), err)
	suite.T().Cleanup(func() {
		suite.client.CleanupDelete(suite.T(), id)
	})
}
