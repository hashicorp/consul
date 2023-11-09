package exportedservices

import (
	"context"
	"fmt"
	"testing"

	svc "github.com/hashicorp/consul/agent/grpc-external/services/resource"
	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/agent/structs"
	cat "github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/multicluster/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type controllerSuite struct {
	suite.Suite
	ctx          context.Context
	client       *rtest.Client
	rt           controller.Runtime
	isEnterprise bool
	reconciler   *reconciler
	tenancies    []*pbresource.Tenancy
}

func (suite *controllerSuite) SetupTest() {
	suite.ctx = testutil.TestContext(suite.T())
	suite.tenancies = rtest.TestTenancies()
	mockTenancyBridge := &svc.MockTenancyBridge{}
	for _, tenancy := range suite.tenancies {
		mockTenancyBridge.On("PartitionExists", tenancy.Partition).Return(true, nil)
		mockTenancyBridge.On("IsPartitionMarkedForDeletion", tenancy.Partition).Return(false, nil)
		mockTenancyBridge.On("NamespaceExists", tenancy.Partition, tenancy.Namespace).Return(true, nil)
		mockTenancyBridge.On("IsNamespaceMarkedForDeletion", tenancy.Partition, tenancy.Namespace).Return(false, nil)
		mockTenancyBridge.On("NamespaceExists", tenancy.Partition, "app").Return(true, nil)
		mockTenancyBridge.On("IsNamespaceMarkedForDeletion", tenancy.Partition, "app").Return(false, nil)
	}

	config := svc.Config{
		TenancyBridge: mockTenancyBridge,
	}
	client := svctest.RunResourceServiceWithConfig(suite.T(), config, types.Register, cat.RegisterTypes)
	suite.client = rtest.NewClient(client)
	suite.rt = controller.Runtime{
		Client: suite.client,
		Logger: testutil.Logger(suite.T()),
	}
	suite.reconciler = &reconciler{}
	suite.isEnterprise = (structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty() == "default")
}

func (suite *controllerSuite) TestReconcile() {
	suite.runTestCaseWithTenancies(suite.reconcileTest)
}

func TestController(t *testing.T) {
	suite.Run(t, new(controllerSuite))
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

func removeService(consumers []*pbmulticluster.ComputedExportedService, ref *pbresource.Reference) []*pbmulticluster.ComputedExportedService {
	newConsumers := []*pbmulticluster.ComputedExportedService{}
	for _, consumer := range consumers {
		if !proto.Equal(consumer.TargetRef, ref) {
			newConsumers = append(newConsumers, consumer)
		}
	}
	return newConsumers
}

func (suite *controllerSuite) getComputedExportedSvc(id *pbresource.ID) *pbmulticluster.ComputedExportedServices {
	computedExportedService := suite.client.RequireResourceExists(suite.T(), id)
	decodedComputedExportedService := rtest.MustDecode[*pbmulticluster.ComputedExportedServices](suite.T(), computedExportedService)
	return decodedComputedExportedService.Data
}

var svc0, svc2, svc3, svc4, svc5 *pbresource.Resource

func (suite *controllerSuite) reconcileTest(tenancy *pbresource.Tenancy) {
	id := rtest.Resource(pbmulticluster.ComputedExportedServicesType, "global").WithTenancy(&pbresource.Tenancy{
		Partition: tenancy.Partition,
	}).ID()
	require.NotNil(suite.T(), id)

	rtest.Resource(pbcatalog.ServiceType, "svc1").
		WithData(suite.T(), &pbcatalog.Service{
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			},
		}).
		WithTenancy(tenancy).
		Write(suite.T(), suite.client)

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
	expSvc := rtest.Resource(pbmulticluster.ExportedServicesType, "expsvc").WithData(suite.T(), exportedSvcData).WithTenancy(tenancy).Write(suite.T(), suite.client)
	require.NotNil(suite.T(), expSvc)

	err := suite.reconciler.Reconcile(suite.ctx, suite.rt, controller.Request{ID: id})
	require.NoError(suite.T(), err)
	actualComputedExportedService := suite.getComputedExportedSvc(id)
	expectedComputedExportedService := getExpectation(tenancy, suite.isEnterprise, 0)
	prototest.AssertDeepEqual(suite.T(), expectedComputedExportedService, actualComputedExportedService)

	svc2 = rtest.Resource(pbcatalog.ServiceType, "svc2").
		WithData(suite.T(), &pbcatalog.Service{
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			},
		}).
		WithTenancy(tenancy).
		Write(suite.T(), suite.client)

	svc0 = rtest.Resource(pbcatalog.ServiceType, "svc0").
		WithData(suite.T(), &pbcatalog.Service{
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			},
		}).
		WithTenancy(&pbresource.Tenancy{
			Partition: tenancy.Partition,
			Namespace: "app",
		}).
		Write(suite.T(), suite.client)

	exportedNamespaceSvcData := &pbmulticluster.NamespaceExportedServices{
		Consumers: []*pbmulticluster.ExportedServicesConsumer{{ConsumerTenancy: &pbmulticluster.ExportedServicesConsumer_Peer{
			Peer: "peer-1",
		}}},
	}

	nameexpSvc := rtest.Resource(pbmulticluster.NamespaceExportedServicesType, "namesvc").WithData(suite.T(), exportedNamespaceSvcData).WithTenancy(tenancy).Write(suite.T(), suite.client)
	require.NotNil(suite.T(), nameexpSvc)
	err = suite.reconciler.Reconcile(suite.ctx, suite.rt, controller.Request{ID: id})
	require.NoError(suite.T(), err)
	actualComputedExportedService = suite.getComputedExportedSvc(id)
	expectedComputedExportedService = getExpectation(tenancy, suite.isEnterprise, 1)
	prototest.AssertDeepEqual(suite.T(), expectedComputedExportedService, actualComputedExportedService)

	svc3 = rtest.Resource(pbcatalog.ServiceType, "svc3").
		WithData(suite.T(), &pbcatalog.Service{
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			},
		}).
		WithTenancy(tenancy).
		Write(suite.T(), suite.client)

	err = suite.reconciler.Reconcile(suite.ctx, suite.rt, controller.Request{ID: id})
	require.NoError(suite.T(), err)
	actualComputedExportedService = suite.getComputedExportedSvc(id)
	expectedComputedExportedService = getExpectation(tenancy, suite.isEnterprise, 2)
	prototest.AssertDeepEqual(suite.T(), expectedComputedExportedService, actualComputedExportedService)

	suite.client.MustDelete(suite.T(), svc3.Id)
	err = suite.reconciler.Reconcile(suite.ctx, suite.rt, controller.Request{ID: id})
	require.NoError(suite.T(), err)

	actualComputedExportedService = suite.getComputedExportedSvc(id)
	expectedComputedExportedService = getExpectation(tenancy, suite.isEnterprise, 3)
	prototest.AssertDeepEqual(suite.T(), expectedComputedExportedService, actualComputedExportedService)

	partitionedExportedSvcData := &pbmulticluster.PartitionExportedServices{
		Consumers: []*pbmulticluster.ExportedServicesConsumer{{ConsumerTenancy: &pbmulticluster.ExportedServicesConsumer_Peer{
			Peer: "peer-1",
		}}, {ConsumerTenancy: &pbmulticluster.ExportedServicesConsumer_Peer{
			Peer: "peer-2",
		}}},
	}

	partexpSvc := rtest.Resource(pbmulticluster.PartitionExportedServicesType, "partsvc").WithData(suite.T(), partitionedExportedSvcData).WithTenancy(&pbresource.Tenancy{Partition: tenancy.Partition}).Write(suite.T(), suite.client)
	require.NotNil(suite.T(), partexpSvc)

	err = suite.reconciler.Reconcile(suite.ctx, suite.rt, controller.Request{ID: id})
	require.NoError(suite.T(), err)
	actualComputedExportedService = suite.getComputedExportedSvc(id)
	expectedComputedExportedService = getExpectation(tenancy, suite.isEnterprise, 4)
	prototest.AssertDeepEqual(suite.T(), expectedComputedExportedService, actualComputedExportedService)

	svc4 = rtest.Resource(pbcatalog.ServiceType, "svc4").
		WithData(suite.T(), &pbcatalog.Service{
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			},
		}).
		WithTenancy(&pbresource.Tenancy{
			Partition: tenancy.Partition,
			Namespace: "app",
		}).
		Write(suite.T(), suite.client)

	err = suite.reconciler.Reconcile(suite.ctx, suite.rt, controller.Request{ID: id})
	require.NoError(suite.T(), err)
	actualComputedExportedService = suite.getComputedExportedSvc(id)
	expectedComputedExportedService = getExpectation(tenancy, suite.isEnterprise, 5)
	prototest.AssertDeepEqual(suite.T(), expectedComputedExportedService, actualComputedExportedService)

	suite.client.MustDelete(suite.T(), svc4.Id)
	err = suite.reconciler.Reconcile(suite.ctx, suite.rt, controller.Request{ID: id})
	require.NoError(suite.T(), err)

	actualComputedExportedService = suite.getComputedExportedSvc(id)
	expectedComputedExportedService = getExpectation(tenancy, suite.isEnterprise, 6)
	prototest.AssertDeepEqual(suite.T(), expectedComputedExportedService, actualComputedExportedService)

	svc5 = rtest.Resource(pbcatalog.ServiceType, "svc5").
		WithData(suite.T(), &pbcatalog.Service{
			Ports: []*pbcatalog.ServicePort{
				{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			},
		}).
		WithTenancy(tenancy).
		Write(suite.T(), suite.client)

	err = suite.reconciler.Reconcile(suite.ctx, suite.rt, controller.Request{ID: id})
	require.NoError(suite.T(), err)

	actualComputedExportedService = suite.getComputedExportedSvc(id)
	expectedComputedExportedService = getExpectation(tenancy, suite.isEnterprise, 7)
	prototest.AssertDeepEqual(suite.T(), expectedComputedExportedService, actualComputedExportedService)

	suite.client.MustDelete(suite.T(), partexpSvc.Id)
	err = suite.reconciler.Reconcile(suite.ctx, suite.rt, controller.Request{ID: id})
	require.NoError(suite.T(), err)

	actualComputedExportedService = suite.getComputedExportedSvc(id)
	expectedComputedExportedService = getExpectation(tenancy, suite.isEnterprise, 8)

	prototest.AssertDeepEqual(suite.T(), expectedComputedExportedService, actualComputedExportedService)

	suite.client.MustDelete(suite.T(), nameexpSvc.Id)
	err = suite.reconciler.Reconcile(suite.ctx, suite.rt, controller.Request{ID: id})
	require.NoError(suite.T(), err)

	actualComputedExportedService = suite.getComputedExportedSvc(id)

	expectedComputedExportedService = getExpectation(tenancy, suite.isEnterprise, 9)
	prototest.AssertDeepEqual(suite.T(), expectedComputedExportedService, actualComputedExportedService)

	suite.client.MustDelete(suite.T(), expSvc.Id)
	err = suite.reconciler.Reconcile(suite.ctx, suite.rt, controller.Request{ID: id})
	require.NoError(suite.T(), err)

	suite.client.RequireResourceNotFound(suite.T(), id)

}

func getExpectation(tenancy *pbresource.Tenancy, isEnterprise bool, testCase int) *pbmulticluster.ComputedExportedServices {
	makeCES := func(consumers ...*pbmulticluster.ComputedExportedService) *pbmulticluster.ComputedExportedServices {
		return &pbmulticluster.ComputedExportedServices{
			Consumers: consumers,
		}
	}
	makeConsumer := func(ref *pbresource.Reference, consumers ...*pbmulticluster.ComputedExportedServicesConsumer) *pbmulticluster.ComputedExportedService {
		var actual []*pbmulticluster.ComputedExportedServicesConsumer
		for _, c := range consumers {
			_, ok := c.ConsumerTenancy.(*pbmulticluster.ComputedExportedServicesConsumer_Partition)
			if (isEnterprise && ok) || !ok {
				actual = append(actual, c)
			}
		}

		return &pbmulticluster.ComputedExportedService{
			TargetRef: ref,
			Consumers: actual,
		}
	}
	svc0Ref := &pbresource.Reference{
		Type: pbcatalog.ServiceType,
		Tenancy: &pbresource.Tenancy{
			Partition: tenancy.Partition,
			Namespace: "app",
			PeerName:  resource.DefaultPeerName,
		},
		Name: "svc0",
	}
	svc1Ref := &pbresource.Reference{
		Type:    pbcatalog.ServiceType,
		Tenancy: tenancy,
		Name:    "svc1",
	}
	svc2Ref := &pbresource.Reference{
		Type:    pbcatalog.ServiceType,
		Tenancy: tenancy,
		Name:    "svc2",
	}
	svc3Ref := &pbresource.Reference{
		Type:    pbcatalog.ServiceType,
		Tenancy: tenancy,
		Name:    "svc3",
	}
	svc4Ref := &pbresource.Reference{
		Type: pbcatalog.ServiceType,
		Tenancy: &pbresource.Tenancy{
			Partition: tenancy.Partition,
			Namespace: "app",
			PeerName:  resource.DefaultPeerName,
		},
		Name: "svc4",
	}
	svc5Ref := &pbresource.Reference{
		Type:    pbcatalog.ServiceType,
		Tenancy: tenancy,
		Name:    "svc5",
	}

	peer0Consumer := &pbmulticluster.ComputedExportedServicesConsumer{
		ConsumerTenancy: &pbmulticluster.ComputedExportedServicesConsumer_Peer{
			Peer: "peer-0",
		},
	}
	peer1Consumer := &pbmulticluster.ComputedExportedServicesConsumer{
		ConsumerTenancy: &pbmulticluster.ComputedExportedServicesConsumer_Peer{
			Peer: "peer-1",
		},
	}
	peer2Consumer := &pbmulticluster.ComputedExportedServicesConsumer{
		ConsumerTenancy: &pbmulticluster.ComputedExportedServicesConsumer_Peer{
			Peer: "peer-2",
		},
	}

	part0Consumer := &pbmulticluster.ComputedExportedServicesConsumer{
		ConsumerTenancy: &pbmulticluster.ComputedExportedServicesConsumer_Partition{
			Partition: "part-0",
		},
	}

	switch testCase {
	case 0:
		return makeCES(makeConsumer(svc1Ref, peer0Consumer, part0Consumer))
	case 1:
		return makeCES(
			makeConsumer(svc1Ref, peer0Consumer, peer1Consumer, part0Consumer),
			makeConsumer(svc2Ref, peer1Consumer),
		)
	case 2:
		return makeCES(
			makeConsumer(svc1Ref, peer0Consumer, peer1Consumer, part0Consumer),
			makeConsumer(svc2Ref, peer1Consumer),
			makeConsumer(svc3Ref, peer1Consumer),
		)
	case 3:
		return makeCES(
			makeConsumer(svc1Ref, peer0Consumer, peer1Consumer, part0Consumer),
			makeConsumer(svc2Ref, peer1Consumer),
		)
	case 4:
		return makeCES(
			makeConsumer(svc0Ref, peer1Consumer, peer2Consumer),
			makeConsumer(svc1Ref, peer0Consumer, peer1Consumer, peer2Consumer, part0Consumer),
			makeConsumer(svc2Ref, peer1Consumer, peer2Consumer),
		)
	case 5:
		return makeCES(
			makeConsumer(svc0Ref, peer1Consumer, peer2Consumer),
			makeConsumer(svc4Ref, peer1Consumer, peer2Consumer),
			makeConsumer(svc1Ref, peer0Consumer, peer1Consumer, peer2Consumer, part0Consumer),
			makeConsumer(svc2Ref, peer1Consumer, peer2Consumer),
		)
	case 6:
		return makeCES(
			makeConsumer(svc0Ref, peer1Consumer, peer2Consumer),
			makeConsumer(svc1Ref, peer0Consumer, peer1Consumer, peer2Consumer, part0Consumer),
			makeConsumer(svc2Ref, peer1Consumer, peer2Consumer),
		)
	case 7:
		return makeCES(
			makeConsumer(svc0Ref, peer1Consumer, peer2Consumer),
			makeConsumer(svc1Ref, peer0Consumer, peer1Consumer, peer2Consumer, part0Consumer),
			makeConsumer(svc2Ref, peer1Consumer, peer2Consumer),
			makeConsumer(svc5Ref, peer1Consumer, peer2Consumer),
		)
	case 8:
		return makeCES(
			makeConsumer(svc1Ref, peer0Consumer, peer1Consumer, part0Consumer),
			makeConsumer(svc2Ref, peer1Consumer),
			makeConsumer(svc5Ref, peer1Consumer),
		)
	case 9:
		return makeCES(
			makeConsumer(svc1Ref, peer0Consumer, part0Consumer),
		)
	}

	return nil
}
