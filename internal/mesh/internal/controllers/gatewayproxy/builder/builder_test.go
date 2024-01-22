// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package builder

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/hashicorp/consul/agent/connect"
	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/envoyextensions/xdscommon"
	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/gatewayproxy/fetcher"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecarproxy/cache"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/multicluster"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbmesh/v2beta1/pbproxystate"
	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/version/versiontest"
)

type proxyStateTemplateBuilderSuite struct {
	suite.Suite

	ctx            context.Context
	client         pbresource.ResourceServiceClient
	resourceClient *resourcetest.Client
	rt             controller.Runtime

	workloadWithAddressPorts          *types.DecodedWorkload
	workloadWithOutAddressPorts       *types.DecodedWorkload
	apiService                        *pbresource.Resource
	exportedServicesPartitionData     *types.DecodedComputedExportedServices
	exportedServicesPartitionResource *pbresource.Resource
	exportedServicesPeerData          *types.DecodedComputedExportedServices
	exportedServicesPeerResource      *pbresource.Resource

	tenancies []*pbresource.Tenancy
}

func (suite *proxyStateTemplateBuilderSuite) SetupTest() {
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

func (suite *proxyStateTemplateBuilderSuite) setupWithTenancy(tenancy *pbresource.Tenancy) {
	suite.workloadWithAddressPorts = &types.DecodedWorkload{
		Data: &pbcatalog.Workload{
			Identity: "test",
			Addresses: []*pbcatalog.WorkloadAddress{
				// we want to test that the first address is used
				{
					Host:     "testhostname",
					Ports:    []string{"wan"},
					External: false,
				},
			},
			Ports: map[string]*pbcatalog.WorkloadPort{
				"wan": {
					Port:     443,
					Protocol: 0,
				},
			},
		},
		Resource: &pbresource.Resource{
			Id: &pbresource.ID{
				Name:    "test",
				Tenancy: tenancy,
			},
		},
	}

	suite.workloadWithOutAddressPorts = &types.DecodedWorkload{
		Data: &pbcatalog.Workload{
			Identity: "test",
			Addresses: []*pbcatalog.WorkloadAddress{
				// we want to test that the first address is used
				{
					Host:     "testhostname",
					Ports:    []string{},
					External: false,
				},
			},
			Ports: map[string]*pbcatalog.WorkloadPort{
				"wan": {
					Port:     443,
					Protocol: 0,
				},
			},
		},
		Resource: &pbresource.Resource{
			Id: &pbresource.ID{
				Name:    "test",
				Tenancy: tenancy,
			},
		},
	}

	// write the service to export
	suite.apiService = resourcetest.Resource(pbcatalog.ServiceType, "api-1").
		WithTenancy(tenancy).
		WithData(suite.T(), &pbcatalog.Service{
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "tcp", VirtualPort: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
				{TargetPort: "mesh", VirtualPort: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
			},
		}).Write(suite.T(), suite.client)

	consumers := []*pbmulticluster.ComputedExportedServiceConsumer{
		{
			Tenancy: &pbmulticluster.ComputedExportedServiceConsumer_Partition{Partition: tenancy.Partition},
		},
	}

	if !versiontest.IsEnterprise() {
		consumers = []*pbmulticluster.ComputedExportedServiceConsumer{}
	}

	suite.exportedServicesPartitionData = &types.DecodedComputedExportedServices{
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
					Consumers: consumers,
				},
			},
		},
	}

	suite.exportedServicesPartitionResource = resourcetest.Resource(pbmulticluster.ComputedExportedServicesType, "global").
		WithData(suite.T(), suite.exportedServicesPartitionData.Data).
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
		WithData(suite.T(), suite.exportedServicesPartitionData.Data).
		Write(suite.T(), suite.client)
}

func (suite *proxyStateTemplateBuilderSuite) TestProxyStateTemplateBuilder_BuildForPeeredExportedServices() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		c := cache.New()
		f := fetcher.New(suite.client, c)
		dc := "dc"
		trustDomain := "trustDomain"
		logger := testutil.Logger(suite.T())

		for name, workload := range map[string]*types.DecodedWorkload{
			"with address ports":    suite.workloadWithAddressPorts,
			"without address ports": suite.workloadWithOutAddressPorts,
		} {
			testutil.RunStep(suite.T(), name, func(t *testing.T) {
				builder := NewProxyStateTemplateBuilder(workload, suite.exportedServicesPeerData.Data.Services, logger, f, dc, trustDomain)
				expectedProxyStateTemplate := &pbmesh.ProxyStateTemplate{
					ProxyState: &pbmesh.ProxyState{
						Identity: &pbresource.Reference{
							Name:    "test",
							Tenancy: tenancy,
							Type:    pbauth.WorkloadIdentityType,
						},
						Listeners: []*pbproxystate.Listener{
							{
								Name:      xdscommon.PublicListenerName,
								Direction: pbproxystate.Direction_DIRECTION_INBOUND,
								BindAddress: &pbproxystate.Listener_HostPort{
									HostPort: &pbproxystate.HostPortAddress{
										Host: "testhostname",
										Port: 443,
									},
								},
								Capabilities: []pbproxystate.Capability{
									pbproxystate.Capability_CAPABILITY_L4_TLS_INSPECTION,
								},
								DefaultRouter: &pbproxystate.Router{
									Destination: &pbproxystate.Router_L4{
										L4: &pbproxystate.L4Destination{
											Destination: &pbproxystate.L4Destination_Cluster{
												Cluster: &pbproxystate.DestinationCluster{
													Name: nullRouteClusterName,
												},
											},
											StatPrefix: "prefix",
										},
									},
								},
								Routers: []*pbproxystate.Router{
									{
										Match: &pbproxystate.Match{
											AlpnProtocols: []string{"consul~tcp"},
											ServerNames:   []string{connect.PeeredServiceSNI("api-1", tenancy.Namespace, tenancy.Partition, "api-1", "trustDomain")},
										},
										Destination: &pbproxystate.Router_L4{
											L4: &pbproxystate.L4Destination{
												Destination: &pbproxystate.L4Destination_Cluster{
													Cluster: &pbproxystate.DestinationCluster{
														Name: fmt.Sprintf("tcp.%s", connect.PeeredServiceSNI("api-1", tenancy.Namespace, tenancy.Partition, "api-1", "trustDomain")),
													},
												},
												StatPrefix: "prefix",
											},
										},
									},
									{
										Match: &pbproxystate.Match{
											AlpnProtocols: []string{"consul~mesh"},
											ServerNames:   []string{connect.PeeredServiceSNI("api-1", tenancy.Namespace, tenancy.Partition, "api-1", "trustDomain")},
										},
										Destination: &pbproxystate.Router_L4{
											L4: &pbproxystate.L4Destination{
												Destination: &pbproxystate.L4Destination_Cluster{
													Cluster: &pbproxystate.DestinationCluster{
														Name: fmt.Sprintf("mesh.%s", connect.PeeredServiceSNI("api-1", tenancy.Namespace, tenancy.Partition, "api-1", "trustDomain")),
													},
												},
												StatPrefix: "prefix",
											},
										},
									},
								},
							},
						},
						Clusters: map[string]*pbproxystate.Cluster{
							nullRouteClusterName: {
								Name: nullRouteClusterName,
								Group: &pbproxystate.Cluster_EndpointGroup{
									EndpointGroup: &pbproxystate.EndpointGroup{
										Group: &pbproxystate.EndpointGroup_Static{
											Static: &pbproxystate.StaticEndpointGroup{
												Config: &pbproxystate.StaticEndpointGroupConfig{
													ConnectTimeout: durationpb.New(10 * time.Second),
												},
											},
										},
									},
								},
								Protocol: pbproxystate.Protocol_PROTOCOL_TCP,
							},
							fmt.Sprintf("mesh.%s", connect.PeeredServiceSNI("api-1", tenancy.Namespace, tenancy.Partition, "api-1", "trustDomain")): {
								Name: fmt.Sprintf("mesh.%s", connect.PeeredServiceSNI("api-1", tenancy.Namespace, tenancy.Partition, "api-1", "trustDomain")),
								Group: &pbproxystate.Cluster_EndpointGroup{
									EndpointGroup: &pbproxystate.EndpointGroup{
										Group: &pbproxystate.EndpointGroup_Dynamic{},
									},
								},
								AltStatName: "prefix",
								Protocol:    pbproxystate.Protocol_PROTOCOL_TCP, // TODO
							},
							fmt.Sprintf("tcp.%s", connect.PeeredServiceSNI("api-1", tenancy.Namespace, tenancy.Partition, "api-1", "trustDomain")): {
								Name: fmt.Sprintf("tcp.%s", connect.PeeredServiceSNI("api-1", tenancy.Namespace, tenancy.Partition, "api-1", "trustDomain")),
								Group: &pbproxystate.Cluster_EndpointGroup{
									EndpointGroup: &pbproxystate.EndpointGroup{
										Group: &pbproxystate.EndpointGroup_Dynamic{},
									},
								},
								AltStatName: "prefix",
								Protocol:    pbproxystate.Protocol_PROTOCOL_TCP, // TODO
							},
						},
					},
					RequiredEndpoints: map[string]*pbproxystate.EndpointRef{
						fmt.Sprintf("mesh.%s", connect.PeeredServiceSNI("api-1", tenancy.Namespace, tenancy.Partition, "api-1", "trustDomain")): {
							Id: &pbresource.ID{
								Name:    "api-1",
								Type:    pbcatalog.ServiceEndpointsType,
								Tenancy: tenancy,
							},
							Port: "mesh",
						},
						fmt.Sprintf("tcp.%s", connect.PeeredServiceSNI("api-1", tenancy.Namespace, tenancy.Partition, "api-1", "trustDomain")): {
							Id: &pbresource.ID{
								Name:    "api-1",
								Type:    pbcatalog.ServiceEndpointsType,
								Tenancy: tenancy,
							},
							Port: "tcp",
						},
					},
					RequiredLeafCertificates: make(map[string]*pbproxystate.LeafCertificateRef),
					RequiredTrustBundles:     make(map[string]*pbproxystate.TrustBundleRef),
				}

				require.Equal(t, expectedProxyStateTemplate, builder.Build())
			})
		}
	})
}

func version(partitionName, partitionOrPeer string) string {
	if partitionOrPeer == "peer" {
		return "external"
	}
	if partitionName == "default" {
		return "internal"
	}
	return "internal-v1"
}

func withPartition(partition string) string {
	if partition == "default" {
		return ""
	}
	return "." + partition
}

func TestProxyStateTemplateBuilder(t *testing.T) {
	suite.Run(t, new(proxyStateTemplateBuilderSuite))
}

func (suite *proxyStateTemplateBuilderSuite) appendTenancyInfo(tenancy *pbresource.Tenancy) string {
	return fmt.Sprintf("%s_Namespace_%s_Partition", tenancy.Namespace, tenancy.Partition)
}

func (suite *proxyStateTemplateBuilderSuite) runTestCaseWithTenancies(t func(*pbresource.Tenancy)) {
	for _, tenancy := range suite.tenancies {
		suite.Run(suite.appendTenancyInfo(tenancy), func() {
			suite.setupWithTenancy(tenancy)
			suite.T().Cleanup(func() {
				suite.cleanUpNodes()
			})
			t(tenancy)
		})
	}
}

func (suite *proxyStateTemplateBuilderSuite) cleanUpNodes() {
	suite.resourceClient.MustDelete(suite.T(), suite.exportedServicesPartitionResource.Id)
	suite.resourceClient.MustDelete(suite.T(), suite.exportedServicesPeerResource.Id)
	suite.resourceClient.MustDelete(suite.T(), suite.apiService.Id)
}
