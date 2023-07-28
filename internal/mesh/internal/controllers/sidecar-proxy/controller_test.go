// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package sidecar_proxy

import (
	"context"
	"testing"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecar-proxy/builder"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecar-proxy/cache"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecar-proxy/mapper"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecar-proxy/status"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1/pbproxystate"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type meshControllerTestSuite struct {
	suite.Suite

	client  *resourcetest.Client
	runtime controller.Runtime

	ctl *reconciler
	ctx context.Context

	apiWorkloadID      *pbresource.ID
	apiWorkload        *pbcatalog.Workload
	apiService         *pbresource.Resource
	apiEndpoints       *pbresource.Resource
	apiEndpointsData   *pbcatalog.ServiceEndpoints
	webWorkload        *pbresource.Resource
	proxyStateTemplate *pbmesh.ProxyStateTemplate
}

func (suite *meshControllerTestSuite) SetupTest() {
	resourceClient := svctest.RunResourceService(suite.T(), types.Register, catalog.RegisterTypes)
	suite.client = resourcetest.NewClient(resourceClient)
	suite.runtime = controller.Runtime{Client: resourceClient, Logger: testutil.Logger(suite.T())}
	suite.ctx = testutil.TestContext(suite.T())

	suite.ctl = &reconciler{
		cache: cache.New(),
		getTrustDomain: func() (string, error) {
			return "test.consul", nil
		},
	}

	suite.apiWorkload = &pbcatalog.Workload{
		Identity: "api-identity",
		Addresses: []*pbcatalog.WorkloadAddress{
			{
				Host: "10.0.0.1",
			},
		},
		Ports: map[string]*pbcatalog.WorkloadPort{
			"tcp":  {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
			"mesh": {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
		},
	}

	suite.apiWorkloadID = resourcetest.Resource(catalog.WorkloadType, "api-abc").
		WithData(suite.T(), suite.apiWorkload).
		Write(suite.T(), resourceClient).Id

	suite.apiService = resourcetest.Resource(catalog.ServiceType, "api-service").
		WithData(suite.T(), &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{Names: []string{"api-abc"}},
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "tcp", Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
			}}).
		Write(suite.T(), suite.client.ResourceServiceClient)

	suite.apiEndpointsData = &pbcatalog.ServiceEndpoints{
		Endpoints: []*pbcatalog.Endpoint{
			{
				TargetRef: suite.apiWorkloadID,
				Addresses: suite.apiWorkload.Addresses,
				Ports:     suite.apiWorkload.Ports,
				Identity:  "api-identity",
			},
		},
	}
	suite.apiEndpoints = resourcetest.Resource(catalog.ServiceEndpointsType, "api-service").
		WithData(suite.T(), suite.apiEndpointsData).
		Write(suite.T(), suite.client.ResourceServiceClient)

	webWorkloadData := &pbcatalog.Workload{
		Identity: "web-identity",
		Addresses: []*pbcatalog.WorkloadAddress{
			{
				Host: "10.0.0.2",
			},
		},
		Ports: map[string]*pbcatalog.WorkloadPort{
			"tcp":  {Port: 8081, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
			"mesh": {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
		},
	}
	suite.webWorkload = resourcetest.Resource(catalog.WorkloadType, "web-def").
		WithData(suite.T(), webWorkloadData).
		Write(suite.T(), suite.client)

	resourcetest.Resource(catalog.ServiceType, "web").
		WithData(suite.T(), &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{Names: []string{"web-def"}},
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "tcp", Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
				{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
			}}).
		Write(suite.T(), suite.client)

	resourcetest.Resource(catalog.ServiceEndpointsType, "web").
		WithData(suite.T(), &pbcatalog.ServiceEndpoints{
			Endpoints: []*pbcatalog.Endpoint{
				{
					TargetRef: suite.webWorkload.Id,
					Addresses: webWorkloadData.Addresses,
					Ports:     webWorkloadData.Ports,
					Identity:  "web-identity",
				},
			},
		}).Write(suite.T(), suite.client)

	identityRef := &pbresource.Reference{
		Name:    suite.apiWorkload.Identity,
		Tenancy: suite.apiWorkloadID.Tenancy,
	}

	suite.proxyStateTemplate = builder.New(suite.apiWorkloadID, identityRef, "test.consul").
		BuildLocalApp(suite.apiWorkload).
		Build()
}

func (suite *meshControllerTestSuite) TestReconcile_NoWorkload() {
	// This test ensures that removed workloads are ignored and don't result
	// in the creation of the proxy state template.
	err := suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
		ID: resourceID(types.ProxyStateTemplateType, "not-found"),
	})
	require.NoError(suite.T(), err)

	suite.client.RequireResourceNotFound(suite.T(), resourceID(types.ProxyStateTemplateType, "not-found"))
}

func (suite *meshControllerTestSuite) TestReconcile_NonMeshWorkload() {
	// This test ensures that non-mesh workloads are ignored by the controller.

	nonMeshWorkload := &pbcatalog.Workload{
		Addresses: []*pbcatalog.WorkloadAddress{
			{
				Host: "10.0.0.1",
			},
		},
		Ports: map[string]*pbcatalog.WorkloadPort{
			"tcp": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
		},
	}

	resourcetest.Resource(catalog.WorkloadType, "test-non-mesh-api-workload").
		WithData(suite.T(), nonMeshWorkload).
		Write(suite.T(), suite.client.ResourceServiceClient)

	err := suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
		ID: resourceID(types.ProxyStateTemplateType, "test-non-mesh-api-workload"),
	})

	require.NoError(suite.T(), err)
	suite.client.RequireResourceNotFound(suite.T(), resourceID(types.ProxyStateTemplateType, "test-non-mesh-api-workload"))
}

func (suite *meshControllerTestSuite) TestReconcile_NoExistingProxyStateTemplate() {
	err := suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
		ID: resourceID(types.ProxyStateTemplateType, suite.apiWorkloadID.Name),
	})
	require.NoError(suite.T(), err)

	res := suite.client.RequireResourceExists(suite.T(), resourceID(types.ProxyStateTemplateType, suite.apiWorkloadID.Name))
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), res.Data)
	prototest.AssertDeepEqual(suite.T(), suite.apiWorkloadID, res.Owner)
}

func (suite *meshControllerTestSuite) TestReconcile_ExistingProxyStateTemplate_WithUpdates() {
	// This test ensures that we write a new proxy state template when there are changes.

	// Write the original.
	resourcetest.Resource(types.ProxyStateTemplateType, "api-abc").
		WithData(suite.T(), suite.proxyStateTemplate).
		WithOwner(suite.apiWorkloadID).
		Write(suite.T(), suite.client.ResourceServiceClient)

	// Update the apiWorkload.
	suite.apiWorkload.Ports["mesh"].Port = 21000
	updatedWorkloadID := resourcetest.Resource(catalog.WorkloadType, "api-abc").
		WithData(suite.T(), suite.apiWorkload).
		Write(suite.T(), suite.client.ResourceServiceClient).Id

	err := suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
		ID: resourceID(types.ProxyStateTemplateType, updatedWorkloadID.Name),
	})
	require.NoError(suite.T(), err)

	res := suite.client.RequireResourceExists(suite.T(), resourceID(types.ProxyStateTemplateType, updatedWorkloadID.Name))
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), res.Data)
	prototest.AssertDeepEqual(suite.T(), updatedWorkloadID, res.Owner)

	var updatedProxyStateTemplate pbmesh.ProxyStateTemplate
	err = res.Data.UnmarshalTo(&updatedProxyStateTemplate)
	require.NoError(suite.T(), err)

	// Check that our value is updated in the proxy state template.
	inboundListenerPort := updatedProxyStateTemplate.ProxyState.Listeners[0].
		BindAddress.(*pbproxystate.Listener_HostPort).HostPort.Port
	require.Equal(suite.T(), uint32(21000), inboundListenerPort)
}

func (suite *meshControllerTestSuite) TestReconcile_ExistingProxyStateTemplate_NoUpdates() {
	// This test ensures that we skip writing of the proxy state template when there are no changes to it.

	// Write the original.
	originalProxyState := resourcetest.Resource(types.ProxyStateTemplateType, "api-abc").
		WithData(suite.T(), suite.proxyStateTemplate).
		WithOwner(suite.apiWorkloadID).
		Write(suite.T(), suite.client.ResourceServiceClient)

	// Update the metadata on the apiWorkload which should result in no changes.
	updatedWorkloadID := resourcetest.Resource(catalog.WorkloadType, "api-abc").
		WithData(suite.T(), suite.apiWorkload).
		WithMeta("some", "meta").
		Write(suite.T(), suite.client.ResourceServiceClient).Id

	err := suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
		ID: resourceID(types.ProxyStateTemplateType, updatedWorkloadID.Name),
	})
	require.NoError(suite.T(), err)

	updatedProxyState := suite.client.RequireResourceExists(suite.T(), resourceID(types.ProxyStateTemplateType, suite.apiWorkloadID.Name))
	resourcetest.RequireVersionUnchanged(suite.T(), updatedProxyState, originalProxyState.Version)
}

func (suite *meshControllerTestSuite) TestController() {
	// This is a comprehensive test that checks the overall controller behavior as various resources change state.
	// This should test interactions between the reconciler, the mappers, and the cache to ensure they work
	// together and produce expected result.

	// Run the controller manager
	mgr := controller.NewManager(suite.client, suite.runtime.Logger)
	c := cache.New()
	m := mapper.New(c)

	mgr.Register(Controller(c, m, func() (string, error) {
		return "test.consul", nil
	}))
	mgr.SetRaftLeader(true)
	go mgr.Run(suite.ctx)

	// Create proxy state template IDs to check against in this test.
	apiProxyStateTemplateID := resourcetest.Resource(types.ProxyStateTemplateType, "api-abc").ID()
	webProxyStateTemplateID := resourcetest.Resource(types.ProxyStateTemplateType, "web-def").ID()

	// Check that proxy state template resource is generated for both the api and web workloads.
	var webProxyStateTemplate *pbresource.Resource
	retry.Run(suite.T(), func(r *retry.R) {
		suite.client.RequireResourceExists(r, apiProxyStateTemplateID)
		webProxyStateTemplate = suite.client.RequireResourceExists(r, webProxyStateTemplateID)
	})

	// Add a source service and check that a new proxy state is generated.
	webDestinations := resourcetest.Resource(types.UpstreamsType, "web-destinations").
		WithData(suite.T(), &pbmesh.Upstreams{
			Workloads: &pbcatalog.WorkloadSelector{Names: []string{"web-def"}},
			Upstreams: []*pbmesh.Upstream{
				{
					DestinationRef:  resource.Reference(suite.apiService.Id, ""),
					DestinationPort: "tcp",
				},
			},
		}).Write(suite.T(), suite.client)
	webProxyStateTemplate = suite.client.WaitForNewVersion(suite.T(), webProxyStateTemplateID, webProxyStateTemplate.Version)

	// Update destination's service apiEndpoints and workload to be non-mesh
	// and check that:
	// * api's proxy state template is deleted
	// * we get a new web proxy resource re-generated
	// * the status on Upstreams resource is updated with a validation error
	nonMeshPorts := map[string]*pbcatalog.WorkloadPort{
		"tcp": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
	}
	resourcetest.Resource(catalog.ServiceEndpointsType, "api-service").
		WithData(suite.T(), &pbcatalog.ServiceEndpoints{
			Endpoints: []*pbcatalog.Endpoint{
				{
					TargetRef: suite.apiWorkloadID,
					Addresses: suite.apiWorkload.Addresses,
					Ports:     nonMeshPorts,
					Identity:  "api-identity",
				},
			},
		}).
		Write(suite.T(), suite.client.ResourceServiceClient)

	resourcetest.Resource(catalog.WorkloadType, "api-abc").
		WithData(suite.T(), &pbcatalog.Workload{
			Identity:  "api-identity",
			Addresses: suite.apiWorkload.Addresses,
			Ports:     nonMeshPorts}).
		Write(suite.T(), suite.client)

	// Check that api proxy template is gone.
	retry.Run(suite.T(), func(r *retry.R) {
		suite.client.RequireResourceNotFound(r, apiProxyStateTemplateID)
	})

	// Check status on the pbmesh.Upstreams resource.
	serviceRef := cache.KeyFromRefAndPort(resource.Reference(suite.apiService.Id, ""), "tcp")
	suite.client.WaitForStatusCondition(suite.T(), webDestinations.Id, ControllerName,
		status.ConditionNonMeshDestination(serviceRef))

	// We should get a new web proxy template resource because this destination should be removed.
	webProxyStateTemplate = suite.client.WaitForNewVersion(suite.T(), webProxyStateTemplateID, webProxyStateTemplate.Version)

	// Update destination's service apiEndpoints back to mesh and check that we get a new web proxy resource re-generated
	// and that the status on Upstreams resource is updated to be empty.
	resourcetest.Resource(catalog.ServiceEndpointsType, "api-service").
		WithData(suite.T(), suite.apiEndpointsData).
		Write(suite.T(), suite.client.ResourceServiceClient)

	suite.client.WaitForStatusCondition(suite.T(), webDestinations.Id, ControllerName,
		status.ConditionMeshDestination(serviceRef))

	// We should also get a new web proxy template resource as this destination should be added again.
	webProxyStateTemplate = suite.client.WaitForNewVersion(suite.T(), webProxyStateTemplateID, webProxyStateTemplate.Version)

	// Delete the proxy state template resource and check that it gets regenerated.
	_, err := suite.client.Delete(suite.ctx, &pbresource.DeleteRequest{Id: webProxyStateTemplateID})
	require.NoError(suite.T(), err)

	suite.client.WaitForNewVersion(suite.T(), webProxyStateTemplateID, webProxyStateTemplate.Version)
}

func TestMeshController(t *testing.T) {
	suite.Run(t, new(meshControllerTestSuite))
}

func resourceID(rtype *pbresource.Type, name string) *pbresource.ID {
	return &pbresource.ID{
		Type: rtype,
		Tenancy: &pbresource.Tenancy{
			Partition: "default",
			Namespace: "default",
			PeerName:  "local",
		},
		Name: name,
	}
}
