// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package mesh

import (
	"context"
	"testing"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/envoyextensions/xdscommon"
	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/mesh/builder"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
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

	ctl reconciler
	ctx context.Context

	workloadID         *pbresource.ID
	workload           *pbcatalog.Workload
	proxyStateTemplate *pbmesh.ProxyStateTemplate
}

func (suite *meshControllerTestSuite) SetupTest() {
	resourceClient := svctest.RunResourceService(suite.T(), types.Register, catalog.RegisterTypes)
	suite.client = resourcetest.NewClient(resourceClient)
	suite.runtime = controller.Runtime{Client: resourceClient, Logger: testutil.Logger(suite.T())}
	suite.ctx = testutil.TestContext(suite.T())

	suite.workload = &pbcatalog.Workload{
		Identity: "test-identity",
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

	suite.workloadID = resourcetest.Resource(catalog.WorkloadType, "test-workload").
		WithData(suite.T(), suite.workload).
		Write(suite.T(), resourceClient).Id

	identityRef := &pbresource.Reference{
		Name:    suite.workload.Identity,
		Tenancy: suite.workloadID.Tenancy,
	}

	suite.proxyStateTemplate = builder.New(suite.workloadID, identityRef).
		AddInboundListener(xdscommon.PublicListenerName, suite.workload).
		AddInboundRouters(suite.workload).
		AddInboundTLS().
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

	resourcetest.Resource(catalog.WorkloadType, "test-non-mesh-workload").
		WithData(suite.T(), nonMeshWorkload).
		Write(suite.T(), suite.client.ResourceServiceClient)

	err := suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
		ID: resourceID(types.ProxyStateTemplateType, "test-non-mesh-workload"),
	})
	require.NoError(suite.T(), err)

	suite.client.RequireResourceNotFound(suite.T(), resourceID(types.ProxyStateTemplateType, "test-non-mesh-workload"))
}

func (suite *meshControllerTestSuite) TestReconcile_NoExistingProxyStateTemplate() {
	err := suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
		ID: resourceID(types.ProxyStateTemplateType, suite.workloadID.Name),
	})
	require.NoError(suite.T(), err)

	res := suite.client.RequireResourceExists(suite.T(), resourceID(types.ProxyStateTemplateType, suite.workloadID.Name))
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), res.Data)
	prototest.AssertDeepEqual(suite.T(), suite.workloadID, res.Owner)
}

func (suite *meshControllerTestSuite) TestReconcile_ExistingProxyStateTemplate_WithUpdates() {
	// Write the original.
	resourcetest.Resource(types.ProxyStateTemplateType, "test-workload").
		WithData(suite.T(), suite.proxyStateTemplate).
		WithOwner(suite.workloadID).
		Write(suite.T(), suite.client.ResourceServiceClient)

	// Update the workload.
	suite.workload.Ports["mesh"].Port = 21000
	updatedWorkloadID := resourcetest.Resource(catalog.WorkloadType, "test-workload").
		WithData(suite.T(), suite.workload).
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

	// Check that our value is updated.
	inboundListenerPort := updatedProxyStateTemplate.ProxyState.Listeners[0].BindAddress.(*pbmesh.Listener_IpPort).IpPort.Port
	require.Equal(suite.T(), uint32(21000), inboundListenerPort)
}

func (suite *meshControllerTestSuite) TestReconcile_ExistingProxyStateTemplate_NoUpdates() {
	// Write the original
	originalProxyState := resourcetest.Resource(types.ProxyStateTemplateType, "test-workload").
		WithData(suite.T(), suite.proxyStateTemplate).
		WithOwner(suite.workloadID).
		Write(suite.T(), suite.client.ResourceServiceClient)

	// Update the metadata on the workload which should result in no changes.
	updatedWorkloadID := resourcetest.Resource(catalog.WorkloadType, "test-workload").
		WithData(suite.T(), suite.workload).
		WithMeta("some", "meta").
		Write(suite.T(), suite.client.ResourceServiceClient).Id

	err := suite.ctl.Reconcile(context.Background(), suite.runtime, controller.Request{
		ID: resourceID(types.ProxyStateTemplateType, updatedWorkloadID.Name),
	})
	require.NoError(suite.T(), err)

	updatedProxyState := suite.client.RequireResourceExists(suite.T(), resourceID(types.ProxyStateTemplateType, suite.workloadID.Name))
	resourcetest.RequireVersionUnchanged(suite.T(), updatedProxyState, originalProxyState.Version)
}

// delete the workload, check that proxy state gets deleted (?can we check that?)
func (suite *meshControllerTestSuite) TestController() {
	// Run the controller manager
	mgr := controller.NewManager(suite.client, suite.runtime.Logger)
	mgr.Register(Controller())
	mgr.SetRaftLeader(true)
	go mgr.Run(suite.ctx)

	proxyStateTemplateID := resourcetest.Resource(types.ProxyStateTemplateType, "test-workload").ID()
	// Add a mesh workload and check that it gets reconciled.
	resourcetest.Resource(catalog.WorkloadType, "test-workload").
		WithData(suite.T(), suite.workload).
		Write(suite.T(), suite.client.ResourceServiceClient)

	resourcetest.Resource(catalog.ServiceType, "test-service").
		WithData(suite.T(), &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{Names: []string{"test-workload"}},
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			}}).
		Write(suite.T(), suite.client.ResourceServiceClient)

	endpoints := &pbcatalog.ServiceEndpoints{
		Endpoints: []*pbcatalog.Endpoint{
			{
				TargetRef: suite.workloadID,
				Addresses: suite.workload.Addresses,
				Ports:     suite.workload.Ports,
			},
		},
	}
	resourcetest.Resource(catalog.ServiceEndpointsType, "test-service").
		WithData(suite.T(), endpoints).
		Write(suite.T(), suite.client.ResourceServiceClient)

	// Check that proxy state template resource is generated.
	var proxyStateTmpl *pbresource.Resource
	retry.Run(suite.T(), func(r *retry.R) {
		proxyStateTmpl = suite.client.RequireResourceExists(r, proxyStateTemplateID)
	})

	// Delete the proxy state template resource and check that it gets regenerated.
	_, err := suite.client.Delete(suite.ctx, &pbresource.DeleteRequest{Id: proxyStateTemplateID})
	require.NoError(suite.T(), err)

	suite.client.WaitForNewVersion(suite.T(), proxyStateTemplateID, proxyStateTmpl.Version)

	// Update workload and service endpoints to not be on the mesh anymore
	// and check that the proxy state template is deleted.
	delete(suite.workload.Ports, "mesh")
	resourcetest.Resource(catalog.WorkloadType, "test-workload").
		WithData(suite.T(), suite.workload).
		Write(suite.T(), suite.client.ResourceServiceClient)

	delete(endpoints.Endpoints[0].Ports, "mesh")
	resourcetest.Resource(catalog.ServiceEndpointsType, "test-service").
		WithData(suite.T(), endpoints).
		Write(suite.T(), suite.client.ResourceServiceClient)

	retry.Run(suite.T(), func(r *retry.R) {
		suite.client.RequireResourceNotFound(r, proxyStateTemplateID)
	})
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
