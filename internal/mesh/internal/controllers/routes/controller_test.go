// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package routes

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
)

type controllerSuite struct {
	suite.Suite

	ctx    context.Context
	client *rtest.Client
	rt     controller.Runtime
}

func (suite *controllerSuite) SetupTest() {
	suite.ctx = testutil.TestContext(suite.T())
	client := svctest.RunResourceService(suite.T(), types.Register, catalog.RegisterTypes)
	suite.rt = controller.Runtime{
		Client: client,
		Logger: testutil.Logger(suite.T()),
	}
	suite.client = rtest.NewClient(client)
}

func (suite *controllerSuite) TestController() {
	// TODO: tidy comment
	//
	// This test's purpose is to exercise the controller in a halfway realistic
	// way.

	// Run the controller manager
	mgr := controller.NewManager(suite.client, suite.rt.Logger)
	mgr.Register(Controller())
	mgr.SetRaftLeader(true)
	go mgr.Run(suite.ctx)

	// Start out by creating a single port service and let it create the
	// default mesh config for tcp.

	serviceData := &pbcatalog.Service{
		Workloads: &pbcatalog.WorkloadSelector{
			Prefixes: []string{"api-"},
		},
		Ports: []*pbcatalog.ServicePort{
			{TargetPort: "tcp", Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
			{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
			// {TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
		},
	}

	_ = rtest.Resource(catalog.ServiceType, "api").
		WithData(suite.T(), serviceData).
		Write(suite.T(), suite.client)

	testutil.RunStep(suite.T(), "default tcp route", func(t *testing.T) {
		// Check that the mesh config resource exists and it has one port that is the default.
		computedRoutesID := rtest.Resource(types.ComputedRoutesType, "api").ID()
		computedRoutes := suite.client.WaitForNewVersion(suite.T(), computedRoutesID, "")

		apiServiceRef := rtest.Resource(catalog.ServiceType, "api").Reference("")

		expect := &pbmesh.ComputedRoutes{
			PortedConfigs: map[string]*pbmesh.ComputedPortRoutes{
				"tcp": {
					Config: &pbmesh.ComputedPortRoutes_Tcp{
						Tcp: &pbmesh.InterpretedTCPRoute{
							ParentRef: newParentRef(apiServiceRef, "tcp"),
							Rules: []*pbmesh.InterpretedTCPRouteRule{{
								BackendRefs: []*pbmesh.InterpretedTCPBackendRef{{
									BackendTarget: "catalog.v1alpha1.Service/default.local.default/api?port=tcp",
								}},
							}},
						},
					},
					Targets: map[string]*pbmesh.BackendTargetDetails{
						"catalog.v1alpha1.Service/default.local.default/api?port=tcp": {
							BackendRef: newBackendRef(apiServiceRef, "tcp", ""),
							Service:    serviceData,
						},
					},
				},
			},
		}

		suite.requireComputedRoutes(expect, computedRoutes)
	})

}

func newParentRef(ref *pbresource.Reference, port string) *pbmesh.ParentReference {
	return &pbmesh.ParentReference{
		Ref:  ref,
		Port: port,
	}
}

func newBackendRef(ref *pbresource.Reference, port string, datacenter string) *pbmesh.BackendReference {
	return &pbmesh.BackendReference{
		Ref:        ref,
		Port:       port,
		Datacenter: datacenter,
	}
}

func (suite *controllerSuite) requireComputedRoutes(expected *pbmesh.ComputedRoutes, resource *pbresource.Resource) {
	suite.T().Helper()
	var mc pbmesh.ComputedRoutes
	require.NoError(suite.T(), resource.Data.UnmarshalTo(&mc))
	prototest.AssertDeepEqual(suite.T(), expected, &mc)
}

func TestController(t *testing.T) {
	suite.Run(t, new(controllerSuite))
}
