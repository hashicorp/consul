// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package gatewayproxy

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/apigateways"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/meshgateways"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/multicluster"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v2"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/sdk/testutil"
)

type gatewayproxyControllerSuite struct {
	suite.Suite

	ctx            context.Context
	client         pbresource.ResourceServiceClient
	resourceClient *resourcetest.Client
	rt             controller.Runtime

	meshGateway *pbresource.Resource
	apiGateway  *pbresource.Resource
	service     *pbresource.Resource

	meshgwWorkload  *types.DecodedWorkload
	apigwWorkload   *types.DecodedWorkload
	serviceWorkload *types.DecodedWorkload

	exportedServicesPeerData     *types.DecodedComputedExportedServices
	exportedServicesPeerResource *pbresource.Resource

	tenancies []*pbresource.Tenancy
}

func (suite *gatewayproxyControllerSuite) SetupTest() {
	suite.ctx = testutil.TestContext(suite.T())
	suite.tenancies = resourcetest.TestTenancies()
	suite.client = svctest.NewResourceServiceBuilder().
		WithRegisterFns(types.Register, catalog.RegisterTypes, multicluster.RegisterTypes).
		WithTenancies(suite.tenancies...).
		Run(suite.T())
	suite.resourceClient = resourcetest.NewClient(suite.client)
	suite.rt = controller.Runtime{
		Client: suite.client,
		Logger: testutil.Logger(suite.T()),
	}
}

func (suite *gatewayproxyControllerSuite) TestReconciler_Reconcile() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		r := reconciler{
			dc:             "dc",
			getTrustDomain: func() (string, error) { return "trust-domain", nil },
		}
		ctx := context.Background()

		testutil.RunStep(suite.T(), "non-gateway workload is reconciled", func(t *testing.T) {
			id := &pbresource.ID{
				Name: "api-1",
				Type: &pbresource.Type{
					Group:        "mesh",
					GroupVersion: "v2beta1",
					Kind:         "ProxyStateTemplate",
				},
				Tenancy: tenancy,
			}

			req := controller.Request{ID: id}
			err := r.Reconcile(ctx, suite.rt, req)

			require.NoError(t, err)

			dec, err := resource.GetDecodedResource[*pbmesh.ProxyStateTemplate](ctx, suite.client, id)
			require.NoError(t, err)
			require.Nil(t, dec)
		})

		testutil.RunStep(suite.T(), "mesh-gateway is reconciled", func(t *testing.T) {
			id := &pbresource.ID{
				Name: "mesh-gateway",
				Type: &pbresource.Type{
					Group:        "mesh",
					GroupVersion: "v2beta1",
					Kind:         "ProxyStateTemplate",
				},
				Tenancy: tenancy,
			}

			expectedWrittenResource := &pbresource.Resource{
				Id:       id,
				Metadata: map[string]string{GatewayKindMetadataKey: meshgateways.GatewayKind},
				Owner:    suite.meshgwWorkload.Id,
			}
			req := controller.Request{ID: id}
			err := r.Reconcile(ctx, suite.rt, req)

			require.NoError(t, err)

			dec, err := resource.GetDecodedResource[*pbmesh.ProxyStateTemplate](ctx, suite.client, id)
			require.NoError(t, err)
			require.NotNil(t, dec)
			require.Equal(t, dec.Id.Name, expectedWrittenResource.Id.Name)
			require.Equal(t, dec.Metadata, expectedWrittenResource.Metadata)
			require.Equal(t, dec.Owner.Name, expectedWrittenResource.Owner.Name)
		})
	})
}

func TestGatewayProxyReconciler(t *testing.T) {
	suite.Run(t, new(gatewayproxyControllerSuite))
}

func (suite *gatewayproxyControllerSuite) appendTenancyInfo(tenancy *pbresource.Tenancy) string {
	return fmt.Sprintf("%s_Namespace_%s_Partition", tenancy.Namespace, tenancy.Partition)
}

func (suite *gatewayproxyControllerSuite) setupSuiteWithTenancy(tenancy *pbresource.Tenancy) {
	suite.apiGateway = resourcetest.Resource(pbmesh.APIGatewayType, "api-gateway").
		WithData(suite.T(), &pbmesh.APIGateway{
			GatewayClassName: "consul",
			Listeners: []*pbmesh.APIGatewayListener{
				{
					Name:     "http-listener",
					Port:     9090,
					Protocol: "http",
				},
				{
					Name:     "tcp-listener",
					Port:     8080,
					Protocol: "tcp",
				},
			},
		}).
		WithTenancy(tenancy).
		Write(suite.T(), suite.client)

	suite.apigwWorkload = &types.DecodedWorkload{
		Data: &pbcatalog.Workload{
			Identity: "api-gateway",
			Addresses: []*pbcatalog.WorkloadAddress{
				{
					Host:     "testhostname",
					Ports:    []string{"http-listener", "tcp-listener"},
					External: false,
				},
			},
			Ports: map[string]*pbcatalog.WorkloadPort{
				"http-listener": {
					Port:     9090,
					Protocol: pbcatalog.Protocol_PROTOCOL_HTTP,
				},
				"tcp-listener": {
					Port:     8080,
					Protocol: pbcatalog.Protocol_PROTOCOL_TCP,
				},
			},
		},
		Resource: &pbresource.Resource{
			Id: &pbresource.ID{
				Name:    "mesh-gateway",
				Tenancy: tenancy,
			},
			Metadata: map[string]string{
				GatewayKindMetadataKey: apigateways.GatewayKind,
			},
		},
	}

	resourcetest.Resource(pbcatalog.WorkloadType, "api-gateway").
		WithData(suite.T(), suite.apigwWorkload.Data).
		WithTenancy(tenancy).
		WithMeta(GatewayKindMetadataKey, apigateways.GatewayKind).
		Write(suite.T(), suite.client)

	meshGWTenancy := resourcetest.ToPartitionScoped(tenancy)

	suite.meshGateway = resourcetest.Resource(pbmesh.MeshGatewayType, "mesh-gateway").
		WithData(suite.T(), &pbmesh.MeshGateway{
			GatewayClassName: "consul",
			Listeners: []*pbmesh.MeshGatewayListener{
				{
					Name:     "wan",
					Port:     443,
					Protocol: "tcp",
				},
			},
		}).
		WithTenancy(meshGWTenancy).
		Write(suite.T(), suite.client)

	suite.meshgwWorkload = &types.DecodedWorkload{
		Data: &pbcatalog.Workload{
			Identity: "mesh-gateway",
			Addresses: []*pbcatalog.WorkloadAddress{
				{
					Host:     "testhostname",
					Ports:    []string{meshgateways.WANPortName},
					External: false,
				},
			},
			Ports: map[string]*pbcatalog.WorkloadPort{
				meshgateways.WANPortName: {
					Port:     443,
					Protocol: 0,
				},
			},
		},
		Resource: &pbresource.Resource{
			Id: &pbresource.ID{
				Name:    "mesh-gateway",
				Tenancy: tenancy,
			},
			Metadata: map[string]string{
				GatewayKindMetadataKey: meshgateways.GatewayKind,
			},
		},
	}

	resourcetest.Resource(pbcatalog.WorkloadType, "mesh-gateway").
		WithData(suite.T(), suite.meshgwWorkload.Data).
		WithTenancy(tenancy).
		WithMeta(GatewayKindMetadataKey, meshgateways.GatewayKind).
		Write(suite.T(), suite.client)

	suite.service = resourcetest.Resource(pbcatalog.ServiceType, "api-1").
		WithTenancy(tenancy).
		WithData(suite.T(), &pbcatalog.Service{
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "tcp", VirtualPort: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
				{TargetPort: "mesh", VirtualPort: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
			},
		}).Write(suite.T(), suite.client)

	suite.serviceWorkload = &types.DecodedWorkload{
		Data: &pbcatalog.Workload{
			Identity: "api-1",
			Addresses: []*pbcatalog.WorkloadAddress{
				{
					Host:     "testhostname",
					Ports:    []string{"tcp", "mesh"},
					External: false,
				},
			},
			Ports: map[string]*pbcatalog.WorkloadPort{
				"tcp": {
					Port:     8080,
					Protocol: pbcatalog.Protocol_PROTOCOL_TCP,
				},
				"mesh": {
					Port:     20000,
					Protocol: pbcatalog.Protocol_PROTOCOL_MESH,
				},
			},
		},
		Resource: &pbresource.Resource{
			Id: &pbresource.ID{
				Name:    "api-1",
				Tenancy: tenancy,
			},
		},
	}

	resourcetest.Resource(pbcatalog.WorkloadType, "api-1").
		WithData(suite.T(), suite.serviceWorkload.Data).
		WithTenancy(tenancy).
		Write(suite.T(), suite.client)

	suite.exportedServicesPeerData = &types.DecodedComputedExportedServices{
		Resource: &pbresource.Resource{},
		Data: &pbmulticluster.ComputedExportedServices{
			Services: []*pbmulticluster.ComputedExportedService{
				{
					TargetRef: &pbresource.Reference{
						Type:    pbcatalog.ServiceType,
						Tenancy: tenancy,
						Name:    "api-1",
						Section: "",
					},
					Consumers: []*pbmulticluster.ComputedExportedServiceConsumer{
						{
							Tenancy: &pbmulticluster.ComputedExportedServiceConsumer_Peer{Peer: "api-1"},
						},
					},
				},
			},
		},
	}

	suite.exportedServicesPeerResource = resourcetest.Resource(pbmulticluster.ComputedExportedServicesType, "global").
		WithData(suite.T(), suite.exportedServicesPeerData.Data).
		Write(suite.T(), suite.client)
}

func (suite *gatewayproxyControllerSuite) runTestCaseWithTenancies(t func(*pbresource.Tenancy)) {
	for _, tenancy := range suite.tenancies {
		suite.Run(suite.appendTenancyInfo(tenancy), func() {
			suite.setupSuiteWithTenancy(tenancy)
			t(tenancy)
		})
	}
}
